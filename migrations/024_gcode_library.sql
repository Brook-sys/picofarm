-- 024_gcode_library.sql
-- Library of reusable .gcode files with metadata and stats

CREATE TABLE IF NOT EXISTS gcode_files (
    id TEXT PRIMARY KEY,
    file_id TEXT NOT NULL REFERENCES files(id),
    display_name TEXT,
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
    print_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_gcode_files_material ON gcode_files(material_type);
CREATE INDEX IF NOT EXISTS idx_gcode_files_created ON gcode_files(created_at DESC);
