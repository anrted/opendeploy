-- Migration: 005_services.sql
-- Tables for user-defined system services and site-linked processes.

CREATE TABLE IF NOT EXISTS managed_services (
    id          TEXT NOT NULL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,         -- display name
    unit        TEXT NOT NULL UNIQUE,         -- systemd unit name, e.g. "redis.service"
    description TEXT,
    autostart   INTEGER NOT NULL DEFAULT 1,
    state       TEXT NOT NULL DEFAULT 'unknown'
                    CHECK (state IN ('running', 'stopped', 'failed', 'unknown')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_managed_services_state ON managed_services(state);

-- Extended sites table columns (alter 003 data, not table).
-- Sites need an associated module (nginx/caddy/apache) and optional service.
ALTER TABLE sites ADD COLUMN module_id   TEXT NOT NULL DEFAULT 'nginx';
ALTER TABLE sites ADD COLUMN ssl_cert    TEXT;
ALTER TABLE sites ADD COLUMN ssl_key     TEXT;
ALTER TABLE sites ADD COLUMN created_by  TEXT REFERENCES users(id) ON DELETE SET NULL;
