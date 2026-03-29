// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

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
