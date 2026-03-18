package database

import (
	"context"
	"embed"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(conn *pgx.Conn) error {
	ctx := context.Background()

	// Get the current version; if migration_version doesn't exist yet, start at 0.
	startVersion := 0
	row := conn.QueryRow(ctx, "SELECT COALESCE(MAX(version_num), 0) FROM migration_version")
	if err := row.Scan(&startVersion); err != nil {
		startVersion = 0
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	type migration struct {
		version int
		name    string
	}
	var migs []migration
	for _, e := range entries {
		parts := strings.SplitN(e.Name(), "_", 2)
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		migs = append(migs, migration{version: v, name: e.Name()})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	lastApplied := startVersion - 1
	for _, m := range migs {
		if m.version < startVersion {
			continue
		}
		if err := applyMigration(ctx, conn, m.version, m.name); err != nil {
			return err
		}
		lastApplied = m.version
	}

	if lastApplied >= startVersion {
		if _, err := conn.Exec(ctx,
			`INSERT INTO migration_version (version_num) VALUES ($1) ON CONFLICT DO NOTHING`,
			lastApplied+1,
		); err != nil {
			return fmt.Errorf("updating migration_version: %w", err)
		}
		log.Printf("migrations up to date at version %d", lastApplied+1)
	} else {
		log.Printf("no new migrations to apply (at version %d)", startVersion)
	}

	return nil
}

func applyMigration(ctx context.Context, conn *pgx.Conn, version int, name string) error {
	content, err := migrationsFS.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("reading migration %s: %w", name, err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction for %s: %w", name, err)
	}
	defer tx.Rollback(ctx)

	log.Printf("applying migration %s", name)
	for _, stmt := range strings.Split(string(content), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migration %s (version %d): %w", name, version, err)
		}
	}

	return tx.Commit(ctx)
}
