-- Migration: 004_dashboard.sql
-- Dashboard statistics snapshots and notification settings.

-- Periodic system stats snapshots (kept for sparkline charts).
-- Purged by a background job; kept for 24 hours by default.
CREATE TABLE IF NOT EXISTS stats_snapshots (
    id           TEXT NOT NULL PRIMARY KEY,
    cpu_percent  REAL NOT NULL DEFAULT 0,
    mem_percent  REAL NOT NULL DEFAULT 0,
    disk_percent REAL NOT NULL DEFAULT 0,
    load_1m      REAL NOT NULL DEFAULT 0,
    load_5m      REAL NOT NULL DEFAULT 0,
    load_15m     REAL NOT NULL DEFAULT 0,
    net_rx_bytes INTEGER NOT NULL DEFAULT 0,
    net_tx_bytes INTEGER NOT NULL DEFAULT 0,
    recorded_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_stats_recorded_at ON stats_snapshots(recorded_at);

-- Per-user notification preferences.
CREATE TABLE IF NOT EXISTS notification_settings (
    user_id    TEXT NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email      TEXT,
    on_job_fail  INTEGER NOT NULL DEFAULT 1,
    on_job_ok    INTEGER NOT NULL DEFAULT 0,
    on_module_change INTEGER NOT NULL DEFAULT 1,
    updated_at TEXT NOT NULL
);
