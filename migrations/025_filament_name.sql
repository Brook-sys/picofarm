-- 025_filament_name.sql
-- Store slicer filament preset/name as read-only metadata

ALTER TABLE queue_items ADD COLUMN filament_name TEXT;
ALTER TABLE gcode_files ADD COLUMN filament_name TEXT;
