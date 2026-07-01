-- 023_queue_item_progress.sql
-- Track print progress for manual queue items

ALTER TABLE queue_items ADD COLUMN progress REAL NOT NULL DEFAULT 0;
