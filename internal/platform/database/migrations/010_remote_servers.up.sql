CREATE TABLE servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    hostname TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    machine_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    agent_version TEXT NOT NULL DEFAULT '',
    os TEXT NOT NULL DEFAULT '',
    distribution TEXT NOT NULL DEFAULT '',
    os_version TEXT NOT NULL DEFAULT '',
    kernel TEXT NOT NULL DEFAULT '',
    architecture TEXT NOT NULL DEFAULT '',
    cpu_model TEXT NOT NULL DEFAULT '',
    cpu_cores INTEGER NOT NULL DEFAULT 0,
    ram_total INTEGER NOT NULL DEFAULT 0,
    disk_total INTEGER NOT NULL DEFAULT 0,
    public_ip TEXT NOT NULL DEFAULT '',
    private_ip TEXT NOT NULL DEFAULT '',
    uptime INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    tags TEXT NOT NULL DEFAULT '[]',
    update_channel TEXT NOT NULL DEFAULT 'stable',
    health_status TEXT NOT NULL DEFAULT 'unknown',
    maintenance INTEGER NOT NULL DEFAULT 0,
    last_heartbeat TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_servers_status ON servers(status);
CREATE INDEX idx_servers_last_heartbeat ON servers(last_heartbeat);

CREATE TABLE server_tokens (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    used_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_server_tokens_hash ON server_tokens(token_hash);

CREATE TABLE server_certificates (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL UNIQUE,
    certificate_pem TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE server_heartbeats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    cpu_usage REAL NOT NULL DEFAULT 0,
    memory_usage REAL NOT NULL DEFAULT 0,
    disk_usage REAL NOT NULL DEFAULT 0,
    uptime INTEGER NOT NULL DEFAULT 0,
    running_tasks INTEGER NOT NULL DEFAULT 0,
    agent_version TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_server_heartbeats_server_time ON server_heartbeats(server_id, created_at DESC);

CREATE TABLE server_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    message TEXT NOT NULL,
    details TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);
CREATE INDEX idx_server_events_server_time ON server_events(server_id, created_at DESC);

CREATE TABLE server_tasks (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    state TEXT NOT NULL DEFAULT 'pending',
    output TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    started_at TEXT,
    finished_at TEXT
);
CREATE INDEX idx_server_tasks_server_state ON server_tasks(server_id, state);

CREATE TABLE server_updates (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    from_version TEXT NOT NULL DEFAULT '',
    to_version TEXT NOT NULL,
    channel TEXT NOT NULL,
    state TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    finished_at TEXT
);
