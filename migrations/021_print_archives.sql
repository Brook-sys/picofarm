-- 021_print_archives.sql

CREATE TABLE IF NOT EXISTS print_archives (
    id TEXT PRIMARY KEY,
    job_id TEXT REFERENCES print_jobs(id) ON DELETE SET NULL,
    printer_id TEXT REFERENCES printers(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    start_time TEXT,
    end_time TEXT,
    duration_seconds INTEGER DEFAULT 0,
    filament_used_grams REAL DEFAULT 0,
    cost_cents INTEGER DEFAULT 0,
    thumbnail_file_id TEXT REFERENCES files(id) ON DELETE SET NULL,
    notes TEXT DEFAULT '',
    tags TEXT DEFAULT '[]',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_print_archives_printer_id ON print_archives(printer_id);
CREATE INDEX IF NOT EXISTS idx_print_archives_status ON print_archives(status);
CREATE INDEX IF NOT EXISTS idx_print_archives_job_id ON print_archives(job_id);

CREATE TABLE IF NOT EXISTS print_archive_events (
    id TEXT PRIMARY KEY,
    archive_id TEXT REFERENCES print_archives(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    data TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_print_archive_events_archive_id ON print_archive_events(archive_id);