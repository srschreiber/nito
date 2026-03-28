package database

import (
	"context"
	"fmt"

	dbtypes "github.com/srschreiber/nito/broker/database/types"
)

// CreateUser inserts a new user with an optional public key used for encrypting room keys.
func CreateUser(ctx context.Context, conn Conn, username string, password, publicKey *string) (*dbtypes.User, error) {
	row := conn.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, public_key)
		VALUES ($1, crypt($2, gen_salt('bf', 12)), $3)
		RETURNING id, username, public_key, updated_at, created_at
	`, username, password, publicKey)

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

// GetCoMemberUserIDs returns UUIDs of all joined members (excluding userID) who share
// at least one joined room with userID. Used to fan out presence change notifications.
func GetCoMemberUserIDs(ctx context.Context, conn Conn, userID string) ([]string, error) {
	rows, err := conn.Query(ctx, `
		SELECT DISTINCT rm2.user_id
		FROM room_members rm1
		JOIN room_members rm2 ON rm1.room_id = rm2.room_id
		WHERE rm1.user_id      = $1
		  AND rm1.joined_at IS NOT NULL
		  AND rm2.joined_at IS NOT NULL
		  AND rm2.user_id     != $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get co-member user ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan co-member user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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

func GetUserIDByUsername(ctx context.Context, conn Conn, username string) (string, error) {
	var userID string
	err := conn.QueryRow(ctx, `
		SELECT id FROM users WHERE username = $1`, username).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("get user id by username: %w", err)
	}
	return userID, nil
}
