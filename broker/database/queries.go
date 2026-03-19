package database

import (
	"context"
	"fmt"

	dbtypes "github.com/srschreiber/nito/broker/database/types"
)

// CreateUser inserts a new user with an optional public key used for encrypting room keys.
func CreateUser(ctx context.Context, conn Conn, username string, publicKey *string) (*dbtypes.User, error) {
	row := conn.QueryRow(ctx, `
		INSERT INTO users (username, public_key)
		VALUES ($1, $2)
		RETURNING id, username, public_key, updated_at, created_at
	`, username, publicKey)

	user, err := ScanRow[dbtypes.User](row)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &user, nil
}

// GetUserPublicKey returns the public key for a user, used by callers to encrypt room keys before sending.
func GetUserPublicKey(ctx context.Context, conn Conn, userID string) (*string, error) {
	var publicKey *string
	err := conn.QueryRow(ctx, `
		SELECT public_key FROM users WHERE id = $1
	`, userID).Scan(&publicKey)
	if err != nil {
		return nil, fmt.Errorf("get user public key: %w", err)
	}
	return publicKey, nil
}

// GetUserPublicKeyByUsername returns the public key for a user looked up by username.
func GetUserPublicKeyByUsername(ctx context.Context, conn Conn, username string) (*string, error) {
	var publicKey *string
	err := conn.QueryRow(ctx, `
		SELECT public_key FROM users WHERE username = $1
	`, username).Scan(&publicKey)
	if err != nil {
		return nil, fmt.Errorf("get user public key by username: %w", err)
	}
	return publicKey, nil
}

// CreateRoom creates a room, adds the creator as a joined member with the admin role,
// generates key version 1, and stores the creator's encrypted room key.
func CreateRoom(ctx context.Context, conn Conn, name, creatorUserID, encryptedRoomKey string) (*dbtypes.Room, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO rooms (name, created_by_user_id)
		VALUES ($1, $2)
		RETURNING id, name, created_by_user_id, updated_at, created_at
	`, name, creatorUserID)

	room, err := ScanRow[dbtypes.Room](row)
	if err != nil {
		return nil, fmt.Errorf("insert room: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO room_members (room_id, user_id, joined_at)
		VALUES ($1, $2, now())
	`, room.ID, creatorUserID); err != nil {
		return nil, fmt.Errorf("insert room member: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_room_roles (user_id, room_id, role)
		VALUES ($1, $2, 'admin')
	`, creatorUserID, room.ID); err != nil {
		return nil, fmt.Errorf("insert room role: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO room_key_versions (version_num, room_id, generated_by_user_id)
		VALUES (1, $1, $2)
	`, room.ID, creatorUserID); err != nil {
		return nil, fmt.Errorf("insert room key version: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_room_keys (user_id, room_id, room_key_version_num, encrypted_room_key)
		VALUES ($1, $2, 1, $3)
	`, creatorUserID, room.ID, encryptedRoomKey); err != nil {
		return nil, fmt.Errorf("insert user room key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &room, nil
}

// InviteUserToRoom adds a user to a room as a pending member (joined_at = null) with the member role,
// and stores their encrypted copy of the current room key so they can decrypt messages upon joining.
func InviteUserToRoom(ctx context.Context, conn Conn, roomID, invitedUserID, invitedByUserID, encryptedRoomKey string) (*dbtypes.RoomMember, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get the current key version so we associate the encrypted key with the right version.
	var latestVersion int
	if err := tx.QueryRow(ctx, `
		SELECT MAX(version_num) FROM room_key_versions WHERE room_id = $1
	`, roomID).Scan(&latestVersion); err != nil {
		return nil, fmt.Errorf("get latest key version: %w", err)
	}

	// Add as pending member — joined_at stays null until they accept.
	row := tx.QueryRow(ctx, `
		INSERT INTO room_members (room_id, user_id, invited_by_user_id)
		VALUES ($1, $2, $3)
		RETURNING room_id, user_id, invited_by_user_id, joined_at, updated_at, created_at
	`, roomID, invitedUserID, invitedByUserID)

	member, err := ScanRow[dbtypes.RoomMember](row)
	if err != nil {
		return nil, fmt.Errorf("insert room member: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_room_roles (user_id, room_id, role)
		VALUES ($1, $2, 'member')
	`, invitedUserID, roomID); err != nil {
		return nil, fmt.Errorf("insert member role: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_room_keys (user_id, room_id, room_key_version_num, encrypted_room_key)
		VALUES ($1, $2, $3, $4)
	`, invitedUserID, roomID, latestVersion, encryptedRoomKey); err != nil {
		return nil, fmt.Errorf("insert user room key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &member, nil
}

// CreateRoomKeyVersion inserts a new key version (max + 1) for a room, used when rotating keys.
// Callers must follow up with InsertUserRoomKey for every active member.
func CreateRoomKeyVersion(ctx context.Context, conn Conn, roomID, generatedByUserID string) (*dbtypes.RoomKeyVersion, error) {
	row := conn.QueryRow(ctx, `
		INSERT INTO room_key_versions (version_num, room_id, generated_by_user_id)
		SELECT COALESCE(MAX(version_num), 0) + 1, $1, $2
		FROM room_key_versions
		WHERE room_id = $1
		RETURNING version_num, room_id, generated_by_user_id, updated_at, created_at
	`, roomID, generatedByUserID)

	kv, err := ScanRow[dbtypes.RoomKeyVersion](row)
	if err != nil {
		return nil, fmt.Errorf("create room key version: %w", err)
	}
	return &kv, nil
}

// InsertUserRoomKey stores a user's encrypted copy of the latest room key version.
// The caller is responsible for encrypting the key with the user's public key first.
func InsertUserRoomKey(ctx context.Context, conn Conn, userID, roomID, encryptedRoomKey string) (*dbtypes.UserRoomKey, error) {
	row := conn.QueryRow(ctx, `
		INSERT INTO user_room_keys (user_id, room_id, room_key_version_num, encrypted_room_key)
		SELECT $1, $2, MAX(version_num), $3
		FROM room_key_versions
		WHERE room_id = $2
		RETURNING user_id, room_id, room_key_version_num, encrypted_room_key, updated_at, created_at
	`, userID, roomID, encryptedRoomKey)

	key, err := ScanRow[dbtypes.UserRoomKey](row)
	if err != nil {
		return nil, fmt.Errorf("insert user room key: %w", err)
	}
	return &key, nil
}

// UserRoomRow is a lightweight projection used when listing rooms for a user.
type UserRoomRow struct {
	RoomID   string
	RoomName string
	IsOwner  bool
}

// ListUserRooms returns all rooms the user has joined, along with whether they are the owner.
func ListUserRooms(ctx context.Context, conn Conn, userID string) ([]UserRoomRow, error) {
	rows, err := conn.Query(ctx, `
		SELECT r.id, r.name, (r.created_by_user_id = $1) AS is_owner
		FROM rooms r
		JOIN room_members rm ON rm.room_id = r.id AND rm.user_id = $1
		WHERE rm.joined_at IS NOT NULL
		ORDER BY r.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user rooms: %w", err)
	}
	defer rows.Close()

	var result []UserRoomRow
	for rows.Next() {
		var row UserRoomRow
		if err := rows.Scan(&row.RoomID, &row.RoomName, &row.IsOwner); err != nil {
			return nil, fmt.Errorf("scan room row: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetUserRoomKey returns the user's encrypted room key for the latest key version in the room.
func GetUserRoomKey(ctx context.Context, conn Conn, userID, roomID string) (*dbtypes.UserRoomKey, error) {
	row := conn.QueryRow(ctx, `
		SELECT user_id, room_id, room_key_version_num, encrypted_room_key, updated_at, created_at
		FROM user_room_keys
		WHERE user_id = $1 AND room_id = $2
		ORDER BY room_key_version_num DESC
		LIMIT 1
	`, userID, roomID)

	key, err := ScanRow[dbtypes.UserRoomKey](row)
	if err != nil {
		return nil, fmt.Errorf("get user room key: %w", err)
	}
	return &key, nil
}
