CREATE TABLE IF NOT EXISTS stl_file_tags (
    stl_file_id TEXT NOT NULL REFERENCES stl_files(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (stl_file_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_stl_file_tags_file ON stl_file_tags(stl_file_id);
CREATE INDEX IF NOT EXISTS idx_stl_file_tags_tag ON stl_file_tags(tag_id);

DELETE FROM gcode_file_tags
WHERE gcode_file_id IN (
    SELECT id FROM gcode_files WHERE parent_stl_id IS NOT NULL
);
