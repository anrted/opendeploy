DROP INDEX IF EXISTS idx_settings_server_id;
DROP INDEX IF EXISTS idx_stats_snapshots_server_id;
DROP INDEX IF EXISTS idx_system_logs_server_id;
DROP INDEX IF EXISTS idx_audit_log_server_id;
DROP INDEX IF EXISTS idx_jobs_server_id;
DROP INDEX IF EXISTS idx_modules_server_id;
DROP INDEX IF EXISTS idx_managed_services_server_id;
DROP INDEX IF EXISTS idx_sites_server_id;
PRAGMA foreign_keys=OFF;
CREATE TABLE site_domains_v2 (
    id TEXT NOT NULL PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    domain TEXT NOT NULL UNIQUE,
    domain_type TEXT NOT NULL DEFAULT 'primary' CHECK (domain_type IN ('primary', 'alias', 'redirect')),
    created_at TEXT NOT NULL
);
INSERT OR IGNORE INTO site_domains_v2 (id,site_id,domain,domain_type,created_at)
SELECT id,site_id,domain,domain_type,created_at FROM site_domains;
DROP TABLE site_domains;
ALTER TABLE site_domains_v2 RENAME TO site_domains;
CREATE INDEX idx_site_domains_site_id ON site_domains(site_id);
PRAGMA foreign_keys=ON;
CREATE TABLE settings_v1 (
    key TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
INSERT OR IGNORE INTO settings_v1 (key,value,updated_at)
SELECT key,value,updated_at FROM settings;
DROP TABLE settings;
ALTER TABLE settings_v1 RENAME TO settings;
ALTER TABLE stats_snapshots DROP COLUMN server_id;
ALTER TABLE system_logs DROP COLUMN server_id;
ALTER TABLE audit_log DROP COLUMN server_id;
ALTER TABLE jobs DROP COLUMN server_id;
CREATE TABLE modules_v1 (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT,
    state TEXT NOT NULL DEFAULT 'available' CHECK (state IN ('available','installing','installed','enabled','disabled','removing','error')),
    config TEXT,
    installed_at TEXT,
    updated_at TEXT NOT NULL
);
INSERT OR IGNORE INTO modules_v1 (id,name,version,state,config,installed_at,updated_at)
SELECT id,name,version,state,config,installed_at,updated_at FROM modules;
DROP TABLE modules;
ALTER TABLE modules_v1 RENAME TO modules;
CREATE TABLE managed_services_v1 (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    unit TEXT NOT NULL UNIQUE,
    description TEXT,
    autostart INTEGER NOT NULL DEFAULT 1,
    state TEXT NOT NULL DEFAULT 'unknown' CHECK (state IN ('running','stopped','failed','unknown')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
INSERT OR IGNORE INTO managed_services_v1 (id,name,unit,description,autostart,state,created_at,updated_at)
SELECT id,name,unit,description,autostart,state,created_at,updated_at FROM managed_services;
DROP TABLE managed_services;
ALTER TABLE managed_services_v1 RENAME TO managed_services;
CREATE INDEX idx_managed_services_state ON managed_services(state);
ALTER TABLE sites DROP COLUMN server_id;
