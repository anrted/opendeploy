// Package sqlite provides the SQLite implementation of the OpenDeploy database.
//
// It uses CGO (mattn/go-sqlite3) and applies WAL mode for better concurrent
// read performance with a single writer. Migrations are embedded into the
// binary and run automatically on startup.
package sqlite

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3" // SQLite driver registration

	"github.com/anrted/opendeploy/internal/platform/database"
	"github.com/anrted/opendeploy/internal/platform/database/migrations"
)

// Open creates and configures a SQLite database at the given DSN path,
// applies all pending migrations, and returns a ready-to-use Database.
func Open(dsn string) (*database.Database, error) {
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

	if err := migrations.Run(db); err != nil {
		return nil, fmt.Errorf("sqlite: migrations: %w", err)
	}

	return wrap, nil
}
