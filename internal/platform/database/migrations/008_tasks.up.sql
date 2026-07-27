PRAGMA foreign_keys=OFF;
CREATE TABLE jobs_v2 (
    id          TEXT NOT NULL PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    payload     TEXT NOT NULL DEFAULT '{}',
    state       TEXT NOT NULL DEFAULT 'pending'
                    CHECK (state IN ('pending', 'running', 'success', 'error', 'canceled')),
    progress    INTEGER NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    output      TEXT,
    error       TEXT,
    created_at  TEXT NOT NULL,
    started_at  TEXT,
    finished_at TEXT
);
INSERT INTO jobs_v2 (id, name, type, payload, state, progress, output, error, created_at, started_at, finished_at)
SELECT id, type, type, payload, state,
       CASE WHEN state = 'success' THEN 100 WHEN state = 'running' THEN 10 ELSE 0 END,
       output, error, created_at, started_at, finished_at FROM jobs;
DROP TABLE jobs;
ALTER TABLE jobs_v2 RENAME TO jobs;
CREATE INDEX idx_jobs_state ON jobs(state);
CREATE INDEX idx_jobs_created_at ON jobs(created_at);
PRAGMA foreign_keys=ON;
