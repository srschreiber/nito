package database

import (
	"context"
	"fmt"

	dbtypes "github.com/srschreiber/nito/broker/database/types"
)

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

// PendingInviteRow holds room info for invites the user hasn't accepted yet.
type PendingInviteRow struct {
	RoomID   string
	RoomName string
}

// ListPendingInvites returns all rooms where the user has been invited but not yet joined.
func ListPendingInvites(ctx context.Context, conn Conn, userID string) ([]PendingInviteRow, error) {
	rows, err := conn.Query(ctx, `
		SELECT rm.room_id, r.name
		FROM room_members rm
		JOIN rooms r ON r.id = rm.room_id
		WHERE rm.user_id = $1 AND rm.joined_at IS NULL
		ORDER BY rm.created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list pending invites: %w", err)
	}
	defer rows.Close()

	var result []PendingInviteRow
	for rows.Next() {
		var row PendingInviteRow
		if err := rows.Scan(&row.RoomID, &row.RoomName); err != nil {
			return nil, fmt.Errorf("scan pending invite: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// AcceptRoomInvite sets joined_at = now() for a pending room_members row.
func AcceptRoomInvite(ctx context.Context, conn Conn, userID, roomID string) error {
	_, err := conn.Exec(ctx, `
		UPDATE room_members SET joined_at = now(), updated_at = now()
		WHERE user_id = $1 AND room_id = $2 AND joined_at IS NULL
	`, userID, roomID)
	if err != nil {
		return fmt.Errorf("accept room invite: %w", err)
	}
	return nil
}

// RoomMemberRow holds a joined member's user ID and username.
type RoomMemberRow struct {
	UserID   string
	Username string
}

// ListRoomMembersWithUsernames returns joined members of a room with their usernames.
func ListRoomMembersWithUsernames(ctx context.Context, conn Conn, roomID string) ([]RoomMemberRow, error) {
	rows, err := conn.Query(ctx, `
		SELECT rm.user_id, u.username
		FROM room_members rm
		JOIN users u ON u.id = rm.user_id
		WHERE rm.room_id = $1 AND rm.joined_at IS NOT NULL
		ORDER BY rm.joined_at ASC, rm.created_at ASC
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list room members: %w", err)
	}
	defer rows.Close()

	var result []RoomMemberRow
	for rows.Next() {
		var row RoomMemberRow
		if err := rows.Scan(&row.UserID, &row.Username); err != nil {
			return nil, fmt.Errorf("scan room member: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func ListRoomMembers(ctx context.Context, conn Conn, roomID string) ([]dbtypes.RoomMember, error) {
	rows, err := conn.Query(ctx, `
	SELECT room_id, user_id, invited_by_user_id, joined_at, updated_at, created_at
	FROM room_members
	WHERE room_id = $1 AND joined_at IS NOT NULL
	ORDER BY joined_at ASC, created_at ASC
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list room members: %w", err)
	}
	defer rows.Close()

	var members []dbtypes.RoomMember
	for rows.Next() {
		var m dbtypes.RoomMember
		if err := rows.Scan(&m.RoomID, &m.UserID, &m.InvitedByUserID, &m.JoinedAt, &m.UpdatedAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan room member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
