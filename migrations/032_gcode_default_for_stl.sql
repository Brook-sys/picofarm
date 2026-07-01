ALTER TABLE gcode_files ADD COLUMN default_for_stl BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE gcode_files
SET default_for_stl = TRUE
WHERE parent_stl_id IS NOT NULL
  AND id IN (
    SELECT id FROM (
      SELECT id, parent_stl_id, ROW_NUMBER() OVER (PARTITION BY parent_stl_id ORDER BY created_at ASC) AS rn
      FROM gcode_files
      WHERE parent_stl_id IS NOT NULL
    ) ranked
    WHERE rn = 1
  );
