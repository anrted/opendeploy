ALTER TABLE sites ADD COLUMN server_id TEXT NOT NULL DEFAULT 'local';
CREATE INDEX idx_sites_server_id ON sites(server_id);

PRAGMA foreign_keys=OFF;
CREATE TABLE site_domains_v3 (
    id          TEXT NOT NULL PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    server_id   TEXT NOT NULL DEFAULT 'local',
    domain      TEXT NOT NULL,
    domain_type TEXT NOT NULL DEFAULT 'primary' CHECK (domain_type IN ('primary', 'alias', 'redirect')),
    created_at  TEXT NOT NULL,
    UNIQUE(server_id, domain)
);
INSERT INTO site_domains_v3 (id,site_id,server_id,domain,domain_type,created_at)
SELECT id,site_id,'local',domain,domain_type,created_at FROM site_domains;
DROP TABLE site_domains;
ALTER TABLE site_domains_v3 RENAME TO site_domains;
CREATE INDEX idx_site_domains_site_id ON site_domains(site_id);
CREATE INDEX idx_site_domains_server_domain ON site_domains(server_id,domain);
PRAGMA foreign_keys=ON;

CREATE TABLE managed_services_v2 (
    id TEXT NOT NULL PRIMARY KEY,
    server_id TEXT NOT NULL DEFAULT 'local',
    name TEXT NOT NULL,
    unit TEXT NOT NULL,
    description TEXT,
    autostart INTEGER NOT NULL DEFAULT 1,
    state TEXT NOT NULL DEFAULT 'unknown' CHECK (state IN ('running','stopped','failed','unknown')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(server_id,name),
    UNIQUE(server_id,unit)
);
INSERT INTO managed_services_v2 (id,server_id,name,unit,description,autostart,state,created_at,updated_at)
SELECT id,'local',name,unit,description,autostart,state,created_at,updated_at FROM managed_services;
DROP TABLE managed_services;
ALTER TABLE managed_services_v2 RENAME TO managed_services;
CREATE INDEX idx_managed_services_server_id ON managed_services(server_id);
CREATE INDEX idx_managed_services_state ON managed_services(state);

CREATE TABLE modules_v2 (
    id TEXT NOT NULL,
    server_id TEXT NOT NULL DEFAULT 'local',
    name TEXT NOT NULL,
    version TEXT,
    state TEXT NOT NULL DEFAULT 'available' CHECK (state IN ('available','installing','installed','enabled','disabled','removing','error')),
    config TEXT,
    installed_at TEXT,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(id,server_id)
);
INSERT INTO modules_v2 (id,server_id,name,version,state,config,installed_at,updated_at)
SELECT id,'local',name,version,state,config,installed_at,updated_at FROM modules;
DROP TABLE modules;
ALTER TABLE modules_v2 RENAME TO modules;
CREATE INDEX idx_modules_server_id ON modules(server_id);

ALTER TABLE jobs ADD COLUMN server_id TEXT NOT NULL DEFAULT 'local';
CREATE INDEX idx_jobs_server_id ON jobs(server_id);

ALTER TABLE audit_log ADD COLUMN server_id TEXT NOT NULL DEFAULT 'local';
CREATE INDEX idx_audit_log_server_id ON audit_log(server_id);

ALTER TABLE system_logs ADD COLUMN server_id TEXT NOT NULL DEFAULT 'local';
CREATE INDEX idx_system_logs_server_id ON system_logs(server_id);

ALTER TABLE stats_snapshots ADD COLUMN server_id TEXT NOT NULL DEFAULT 'local';
CREATE INDEX idx_stats_snapshots_server_id ON stats_snapshots(server_id);

CREATE TABLE settings_v2 (
    key TEXT NOT NULL,
    server_id TEXT NOT NULL DEFAULT 'local',
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(key,server_id)
);
INSERT INTO settings_v2 (key,server_id,value,updated_at)
SELECT key,'local',value,updated_at FROM settings;
DROP TABLE settings;
ALTER TABLE settings_v2 RENAME TO settings;
CREATE INDEX idx_settings_server_id ON settings(server_id);
