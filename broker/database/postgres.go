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
