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
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Conn is satisfied by both *pgx.Conn and *pgxpool.Pool, allowing queries and
// migrations to work with either a single connection or a pool.
type Conn interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

func connString(username, password, host, port, dbName string) string {
	if password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", username, password, host, port, dbName)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s", username, host, port, dbName)
}

// NewPostgres opens a single connection, used for sequential operations like migrations.
func NewPostgres(username, password, host, port, dbName string) (*pgx.Conn, error) {
	return pgx.Connect(context.Background(), connString(username, password, host, port, dbName))
}

// NewPool opens a connection pool, used by the broker for concurrent request handling.
func NewPool(username, password, host, port, dbName string) (*pgxpool.Pool, error) {
	return pgxpool.New(context.Background(), connString(username, password, host, port, dbName))
}
