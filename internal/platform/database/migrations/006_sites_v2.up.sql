-- Migration: 006_sites_v2.sql
-- Refactoring sites schema for multi-domain, extensible routing, and robust SSL.

DROP TABLE IF EXISTS sites;

CREATE TABLE sites (
    id          TEXT NOT NULL PRIMARY KEY,
    name        TEXT NOT NULL,
    root_path   TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'error')),
    module_id   TEXT NOT NULL,
    owner_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE site_domains (
    id          TEXT NOT NULL PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    domain      TEXT NOT NULL UNIQUE,
    domain_type TEXT NOT NULL DEFAULT 'primary' CHECK (domain_type IN ('primary', 'alias', 'redirect')),
    created_at  TEXT NOT NULL
);

CREATE INDEX idx_site_domains_site_id ON site_domains(site_id);

CREATE TABLE site_apps (
    site_id       TEXT NOT NULL PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
    app_type      TEXT NOT NULL, -- 'php', 'static', 'proxy'
    app_version   TEXT,
    proxy_target  TEXT,
    custom_config TEXT
);

CREATE TABLE site_ssl (
    id            TEXT NOT NULL PRIMARY KEY,
    site_id       TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    provider      TEXT NOT NULL, -- 'certbot', 'custom'
    cert_path     TEXT,
    key_path      TEXT,
    force_https   INTEGER NOT NULL DEFAULT 0,
    auto_renew    INTEGER NOT NULL DEFAULT 1,
    expires_at    TEXT
);

CREATE INDEX idx_site_ssl_site_id ON site_ssl(site_id);
