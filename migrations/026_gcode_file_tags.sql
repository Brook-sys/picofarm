CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT '#64748b',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gcode_file_tags (
    gcode_file_id TEXT NOT NULL REFERENCES gcode_files(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (gcode_file_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_gcode_file_tags_file ON gcode_file_tags(gcode_file_id);
CREATE INDEX IF NOT EXISTS idx_gcode_file_tags_tag ON gcode_file_tags(tag_id);
