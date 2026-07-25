-- Migration: 003_sites.sql
-- Creates the sites table for web site management.

CREATE TABLE IF NOT EXISTS sites (
    id          TEXT NOT NULL PRIMARY KEY,
    domain      TEXT NOT NULL UNIQUE,
    root_path   TEXT NOT NULL,
    php_version TEXT,
    ssl_enabled INTEGER NOT NULL DEFAULT 0,
    state       TEXT NOT NULL DEFAULT 'active'
                    CHECK (state IN ('active', 'disabled', 'error')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sites_domain ON sites(domain);
CREATE INDEX IF NOT EXISTS idx_sites_state  ON sites(state);
