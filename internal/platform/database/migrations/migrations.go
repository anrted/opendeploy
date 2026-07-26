package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var sqlFiles embed.FS

// Run applies all pending SQL migration files using golang-migrate.
func Run(db *sql.DB) error {
	if err := migrateLegacyMetadata(db); err != nil {
		return fmt.Errorf("migrate legacy metadata: %w", err)
	}

	d, err := iofs.New(sqlFiles, ".")
	if err != nil {
		return fmt.Errorf("create iofs: %w", err)
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("create sqlite3 driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}

// migrateLegacyMetadata upgrades the migration table used by early OpenDeploy
// releases (name, applied_at) to the format expected by golang-migrate
// (version, dirty). Application tables and data are not modified.
func migrateLegacyMetadata(db *sql.DB) error {
	hasName, hasVersion, err := migrationMetadataColumns(db)
	if err != nil {
		return err
	}
	if hasVersion {
		return repairConvertedMetadata(db)
	}
	if !hasName {
		return nil
	}

	names, err := legacyMigrationNames(db)
	if err != nil {
		return err
	}

	currentVersion := latestLegacyVersion(names)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin legacy migration metadata transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`ALTER TABLE schema_migrations RENAME TO schema_migrations_legacy`); err != nil {
		return fmt.Errorf("preserve legacy migration metadata: %w", err)
	}
	if _, err := tx.Exec(`CREATE TABLE schema_migrations (version uint64 NOT NULL PRIMARY KEY, dirty bool NOT NULL)`); err != nil {
		return fmt.Errorf("create migration metadata: %w", err)
	}
	if currentVersion > 0 {
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, dirty) VALUES (?, 0)`, currentVersion); err != nil {
			return fmt.Errorf("record migrated version %d: %w", currentVersion, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy migration metadata: %w", err)
	}
	return nil
}

func migrationMetadataColumns(db *sql.DB) (bool, bool, error) {
	rows, err := db.Query(`PRAGMA table_info(schema_migrations)`)
	if err != nil {
		return false, false, fmt.Errorf("inspect schema_migrations: %w", err)
	}

	hasName := false
	hasVersion := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return false, false, fmt.Errorf("scan schema_migrations column: %w", err)
		}
		hasName = hasName || name == "name"
		hasVersion = hasVersion || name == "version"
	}
	if err := rows.Close(); err != nil {
		return false, false, fmt.Errorf("close schema_migrations columns: %w", err)
	}
	if err := rows.Err(); err != nil {
		return false, false, fmt.Errorf("iterate schema_migrations columns: %w", err)
	}
	return hasName, hasVersion, nil
}

func legacyMigrationNames(db *sql.DB) ([]string, error) {
	var names []string
	nameRows, err := db.Query(`SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read legacy migration names: %w", err)
	}
	for nameRows.Next() {
		var name string
		if err := nameRows.Scan(&name); err != nil {
			_ = nameRows.Close()
			return nil, fmt.Errorf("scan legacy migration name: %w", err)
		}
		names = append(names, name)
	}
	if err := nameRows.Close(); err != nil {
		return nil, fmt.Errorf("close legacy migration names: %w", err)
	}
	if err := nameRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate legacy migration names: %w", err)
	}
	return names, nil
}

func repairConvertedMetadata(db *sql.DB) error {
	var hasLegacyName bool
	rows, err := db.Query(`PRAGMA table_info(schema_migrations_legacy)`)
	if err != nil {
		return fmt.Errorf("inspect legacy migration metadata: %w", err)
	}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy migration metadata: %w", err)
		}
		hasLegacyName = hasLegacyName || name == "name"
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy migration metadata: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy migration metadata: %w", err)
	}
	if !hasLegacyName {
		return nil
	}

	var names []string
	nameRows, err := db.Query(`SELECT name FROM schema_migrations_legacy`)
	if err != nil {
		return fmt.Errorf("read preserved legacy migration names: %w", err)
	}
	for nameRows.Next() {
		var name string
		if err := nameRows.Scan(&name); err != nil {
			_ = nameRows.Close()
			return fmt.Errorf("scan preserved legacy migration name: %w", err)
		}
		names = append(names, name)
	}
	if err := nameRows.Close(); err != nil {
		return fmt.Errorf("close preserved legacy migration names: %w", err)
	}
	if err := nameRows.Err(); err != nil {
		return fmt.Errorf("iterate preserved legacy migration names: %w", err)
	}

	legacyVersion := latestLegacyVersion(names)
	if legacyVersion == 0 {
		return nil
	}

	var version uint64
	var dirty bool
	if err := db.QueryRow(`SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty); err != nil {
		return fmt.Errorf("read converted migration metadata: %w", err)
	}
	if version >= legacyVersion && !dirty {
		return nil
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET version = ?, dirty = 0`, legacyVersion); err != nil {
		return fmt.Errorf("repair converted migration metadata: %w", err)
	}
	return nil
}

func latestLegacyVersion(names []string) uint64 {
	var currentVersion uint64
	for _, name := range names {
		if !strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".down.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		version, err := strconv.ParseUint(prefix, 10, 64)
		if err == nil && version > currentVersion {
			currentVersion = version
		}
	}
	return currentVersion
}
