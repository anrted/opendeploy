CREATE TABLE system_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    level TEXT NOT NULL,
    component TEXT,
    module TEXT,
    error_id TEXT,
    request_id TEXT,
    user_id TEXT,
    duration_ms INTEGER,
    endpoint TEXT,
    method TEXT,
    ip TEXT,
    message TEXT,
    stack_trace TEXT,
    attributes JSON
);

CREATE INDEX idx_system_logs_error_id ON system_logs(error_id);
CREATE INDEX idx_system_logs_timestamp ON system_logs(timestamp);
CREATE INDEX idx_system_logs_level ON system_logs(level);
CREATE INDEX idx_system_logs_module ON system_logs(module);
