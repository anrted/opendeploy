package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRunUpgradesLegacyMigrationMetadataWithoutLosingData(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
		CREATE TABLE schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL
		);
		INSERT INTO schema_migrations (name, applied_at) VALUES
			('001_init.sql', CURRENT_TIMESTAMP),
			('002_modules.sql', CURRENT_TIMESTAMP),
			('003_sites.sql', CURRENT_TIMESTAMP),
			('004_dashboard.sql', CURRENT_TIMESTAMP),
			('005_services.sql', CURRENT_TIMESTAMP),
			('006_sites_v2.sql', CURRENT_TIMESTAMP);
		CREATE TABLE preserved_data (value TEXT NOT NULL);
		INSERT INTO preserved_data (value) VALUES ('keep me');
	`)
	if err != nil {
		t.Fatalf("create legacy database: %v", err)
	}

	if err := migrateLegacyMetadata(db); err != nil {
		t.Fatalf("migrateLegacyMetadata() returned error: %v", err)
	}

	var version uint64
	var dirty bool
	if err := db.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("read migrated metadata: %v", err)
	}
	if version != 6 || dirty {
		t.Fatalf("migration metadata = (%d, %t), want (6, false)", version, dirty)
	}

	var value string
	if err := db.QueryRow(`SELECT value FROM preserved_data`).Scan(&value); err != nil {
		t.Fatalf("read preserved data: %v", err)
	}
	if value != "keep me" {
		t.Fatalf("preserved data = %q, want %q", value, "keep me")
	}

	var legacyCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations_legacy`).Scan(&legacyCount); err != nil {
		t.Fatalf("read legacy metadata: %v", err)
	}
	if legacyCount != 6 {
		t.Fatalf("legacy migration count = %d, want 6", legacyCount)
	}
}

func TestRunRepairsDirtyMetadataLeftByLegacyConversion(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
		CREATE TABLE schema_migrations (version uint64 NOT NULL PRIMARY KEY, dirty bool NOT NULL);
		INSERT INTO schema_migrations (version, dirty) VALUES (3, 1);
		CREATE TABLE schema_migrations_legacy (
			name TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL
		);
		INSERT INTO schema_migrations_legacy (name, applied_at) VALUES
			('001_init.sql', CURRENT_TIMESTAMP),
			('002_modules.sql', CURRENT_TIMESTAMP),
			('003_sites.sql', CURRENT_TIMESTAMP),
			('004_dashboard.sql', CURRENT_TIMESTAMP),
			('005_services.sql', CURRENT_TIMESTAMP),
			('006_sites_v2.sql', CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("create dirty converted database: %v", err)
	}

	if err := migrateLegacyMetadata(db); err != nil {
		t.Fatalf("migrateLegacyMetadata() returned error: %v", err)
	}

	var version uint64
	var dirty bool
	if err := db.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("read repaired metadata: %v", err)
	}
	if version != 6 || dirty {
		t.Fatalf("migration metadata = (%d, %t), want (6, false)", version, dirty)
	}
}

func TestRunAddsRemoteAPIVersionColumn(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	if err := Run(db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('servers') WHERE name='api_version'
	`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("api_version column count = %d", count)
	}
}
