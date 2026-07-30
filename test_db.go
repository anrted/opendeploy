//go:build ignore

package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	
	// Pre-013 schema
	db.Exec(`CREATE TABLE sites (
		id TEXT PRIMARY KEY,
		server_id TEXT NOT NULL,
		name TEXT NOT NULL,
		module_id TEXT NOT NULL,
		root_path TEXT NOT NULL,
		status TEXT NOT NULL,
		owner_id TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	
	db.Exec(`INSERT INTO sites (id, server_id, name, module_id, root_path, status, owner_id, created_at, updated_at) VALUES ('1', 'local', 'test', 'nginx', '/var/www', 'active', NULL, '2022-01-01', '2022-01-01')`)
	
	// Apply 013 migration
	_, err = db.Exec(`ALTER TABLE sites ADD COLUMN proxy_enabled BOOLEAN NOT NULL DEFAULT 0`)
	if err != nil { fmt.Println("migration 1:", err) }
	_, err = db.Exec(`ALTER TABLE sites ADD COLUMN proxy_host TEXT NOT NULL DEFAULT '127.0.0.1'`)
	if err != nil { fmt.Println("migration 2:", err) }
	_, err = db.Exec(`ALTER TABLE sites ADD COLUMN proxy_port INTEGER NOT NULL DEFAULT 0`)
	if err != nil { fmt.Println("migration 3:", err) }

	// Try query
	rows, err := db.QueryContext(context.Background(), `SELECT id, name, module_id, root_path, status, owner_id, proxy_enabled, proxy_host, proxy_port, created_at, updated_at FROM sites WHERE server_id=?`, "local")
	if err != nil {
		fmt.Println("query error:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, moduleID, rootPath, status, created, updated string
		var ownerID *string
		var proxyEnabled bool
		var proxyHost string
		var proxyPort int
		err := rows.Scan(&id, &name, &moduleID, &rootPath, &status, &ownerID, &proxyEnabled, &proxyHost, &proxyPort, &created, &updated)
		if err != nil {
			fmt.Println("scan error:", err)
			return
		}
		fmt.Printf("scanned: proxyHost=%v, proxyPort=%v, proxyEnabled=%v, ownerID=%v\n", proxyHost, proxyPort, proxyEnabled, ownerID)
	}
}
