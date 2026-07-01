-- Migration 022: Queue items (G-code based, decoupled from projects/tasks)
CREATE TABLE IF NOT EXISTS queue_items (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL DEFAULT 'upload', -- upload | print_job | manual
    source_id TEXT, -- optional reference to original print_job/design/file
    file_id TEXT NOT NULL REFERENCES files(id),
    file_name TEXT NOT NULL,
    display_name TEXT,
    status TEXT NOT NULL DEFAULT 'queued', -- draft | queued | ready | blocked | printing | paused | done | failed | cancelled
    priority INTEGER NOT NULL DEFAULT 0,
    assigned_printer_id TEXT REFERENCES printers(id),
    assigned_spool_id TEXT REFERENCES material_spools(id),
    material_type TEXT,
    material_color TEXT,
    filament_grams REAL,
    estimated_seconds INTEGER,
    layer_height REAL,
    nozzle_diameter REAL,
    bed_temp REAL,
    nozzle_temp REAL,
    thumbnail_file_id TEXT REFERENCES files(id),
    metadata_json TEXT,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_queue_items_status ON queue_items(status);
CREATE INDEX IF NOT EXISTS idx_queue_items_priority ON queue_items(priority DESC);
CREATE INDEX IF NOT EXISTS idx_queue_items_printer ON queue_items(assigned_printer_id);
CREATE INDEX IF NOT EXISTS idx_queue_items_spool ON queue_items(assigned_spool_id);
CREATE INDEX IF NOT EXISTS idx_queue_items_source ON queue_items(source_type, source_id);
