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
