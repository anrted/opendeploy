// Package sqlite provides the SQLite implementation of the OpenDeploy database.
//
// It uses CGO (mattn/go-sqlite3) and applies WAL mode for better concurrent
// read performance with a single writer. Migrations are embedded into the
// binary and run automatically on startup.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // SQLite driver registration

	systembackup "github.com/anrted/opendeploy/internal/backup"
	"github.com/anrted/opendeploy/internal/platform/database"
	"github.com/anrted/opendeploy/internal/platform/database/migrations"
)

// Open creates and configures a SQLite database at the given DSN path,
// applies all pending migrations, and returns a ready-to-use Database.
func Open(dsn string) (*database.Database, error) {
	_, statErr := os.Stat(dsn)
	existingDatabase := statErr == nil
	// Append pragmas to the DSN for a robust, concurrent-safe setup.
	pragmaDSN := fmt.Sprintf(
		"%s?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000",
		dsn,
	)

	db, err := sql.Open("sqlite3", pragmaDSN)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", dsn, err)
	}

	// SQLite supports only one writer at a time; limit pool accordingly.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	wrap := &database.Database{DB: db}

	pending, err := migrations.NeedsMigration(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: inspect migrations: %w", err)
	}
	if existingDatabase && pending {
		backupConfig := systembackup.DefaultConfig()
		backupConfig.BackupDir = "/var/lib/opendeploy/migration-backups"
		backupConfig.StateDir = "/var/lib/opendeploy/migration-backups/.state"
		backupConfig.Sources = []systembackup.Source{{
			ID: "database", Path: dsn, Required: true, Database: true,
		}}
		if directory := os.Getenv("OD_BACKUP_DIR"); directory != "" {
			backupConfig.BackupDir = directory
			backupConfig.StateDir = filepath.Join(directory, ".state")
		}
		if _, _, err := systembackup.NewEngine(backupConfig, "").Create(
			context.Background(), "before-database-migration",
		); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite: mandatory pre-migration backup: %w", err)
		}
	}
	if err := migrations.Run(db); err != nil {
		return nil, fmt.Errorf("sqlite: migrations: %w", err)
	}

	return wrap, nil
}
