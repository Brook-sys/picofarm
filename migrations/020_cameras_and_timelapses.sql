-- 020_cameras_and_timelapses.sql

CREATE TABLE IF NOT EXISTS cameras (
    id TEXT PRIMARY KEY,
    printer_id TEXT REFERENCES printers(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'mjpeg',
    url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    token TEXT DEFAULT '',
    token_expires_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cameras_printer_id ON cameras(printer_id);
CREATE INDEX IF NOT EXISTS idx_cameras_enabled ON cameras(enabled);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cameras_token ON cameras(token) WHERE token != '';

CREATE TABLE IF NOT EXISTS timelapses (
    id TEXT PRIMARY KEY,
    printer_id TEXT REFERENCES printers(id) ON DELETE SET NULL,
    camera_id TEXT REFERENCES cameras(id) ON DELETE SET NULL,
    print_job_id TEXT REFERENCES print_jobs(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    frames_path TEXT DEFAULT '',
    video_path TEXT DEFAULT '',
    frame_count INTEGER NOT NULL DEFAULT 0,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_timelapses_printer_id ON timelapses(printer_id);
CREATE INDEX IF NOT EXISTS idx_timelapses_status ON timelapses(status);