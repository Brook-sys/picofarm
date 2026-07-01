-- 019_maintenance_mode.sql
-- Add maintenance_mode flag to printers for temporary out-of-service state

ALTER TABLE printers ADD COLUMN maintenance_mode BOOLEAN NOT NULL DEFAULT FALSE;

-- Index for quick filtering of active printers
CREATE INDEX IF NOT EXISTS idx_printers_maintenance_mode ON printers(maintenance_mode);