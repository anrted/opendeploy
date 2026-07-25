// Package migrations manages database schema migrations for OpenDeploy.
//
// SQL files are embedded into the binary, so no external files are required
// at runtime. Migrations are applied in filename order and tracked in the
// schema_migrations table to ensure idempotent execution.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.sql
var sqlFiles embed.FS

// Run applies all pending SQL migration files in ascending filename order.
// Each migration is applied in its own transaction and recorded in the
// schema_migrations table.
func Run(db *sql.DB) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	entries, err := listMigrationFiles()
	if err != nil {
		return err
	}

	for _, name := range entries {
		applied, err := isMigrationApplied(db, name)
		if err != nil {
			return fmt.Errorf("check migration %q: %w", name, err)
		}
		if applied {
			continue
		}

		if err := applyMigration(db, name); err != nil {
			return fmt.Errorf("apply migration %q: %w", name, err)
		}
	}

	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	return err
}

func listMigrationFiles() ([]string, error) {
	var names []string

	err := fs.WalkDir(sqlFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk migrations: %w", err)
	}

	sort.Strings(names)
	return names, nil
}

func isMigrationApplied(db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&count)
	return count > 0, err
}

func applyMigration(db *sql.DB, name string) error {
	content, err := sqlFiles.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // error checked via commit

	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("execute sql: %w", err)
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}
