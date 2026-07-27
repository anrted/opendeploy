PRAGMA foreign_keys=OFF;
CREATE TABLE jobs_v1 (
    id TEXT NOT NULL PRIMARY KEY, type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}',
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','running','success','error')),
    output TEXT, error TEXT, created_at TEXT NOT NULL, started_at TEXT, finished_at TEXT
);
INSERT INTO jobs_v1 SELECT id, type, payload, CASE WHEN state='canceled' THEN 'error' ELSE state END, output, error, created_at, started_at, finished_at FROM jobs;
DROP TABLE jobs;
ALTER TABLE jobs_v1 RENAME TO jobs;
CREATE INDEX idx_jobs_state ON jobs(state);
PRAGMA foreign_keys=ON;
