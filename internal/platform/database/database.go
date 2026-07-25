// Package database defines the core database abstraction for OpenDeploy.
//
// All repository implementations receive a *sql.DB obtained through this
// package. The abstraction allows switching between SQLite (MVP) and
// PostgreSQL without touching business logic.
package database

import "database/sql"

// Database wraps a *sql.DB with lifecycle management.
// All repositories depend on this type, not on driver-specific types.
type Database struct {
	DB *sql.DB
}

// Close releases the underlying database connection.
func (d *Database) Close() error {
	return d.DB.Close()
}

// Ping checks that the database is reachable.
func (d *Database) Ping() error {
	return d.DB.Ping()
}
