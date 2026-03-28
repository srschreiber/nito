package database

import (
	"context"
	"fmt"

	dbtypes "github.com/srschreiber/nito/broker/database/types"
)

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

func GetAllUserRoomKeys(ctx context.Context, conn Conn, userID, roomID string) ([]dbtypes.UserRoomKey, error) {
	rows, err := conn.Query(ctx, `
		SELECT user_id, room_id, room_key_version_num, encrypted_room_key, updated_at, created_at
		FROM user_room_keys
		WHERE user_id = $1 AND room_id = $2
		ORDER BY room_key_version_num ASC
	`, userID, roomID)

	if err != nil {
		return nil, fmt.Errorf("get all user room keys: %w", err)
	}
	defer rows.Close()

	keys, err := ScanRows[dbtypes.UserRoomKey](rows)
	if err != nil {
		return nil, fmt.Errorf("get all user room keys: %w", err)
	}
	return keys, nil
}
