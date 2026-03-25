package database

import (
	"context"
)

func ValidateUserPassword(ctx context.Context, conn Conn, username, password string) (bool, error) {
	const sql = `
	SELECT EXISTS (
		SELECT 1 FROM users
		WHERE username = $1 AND password_hash = crypt($2, password_hash)
	)
	`
	var valid bool
	err := conn.QueryRow(ctx, sql, username, password).Scan(&valid)
	if err != nil {
		return false, err
	}
	return valid, nil
}
