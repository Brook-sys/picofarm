CREATE TABLE IF NOT EXISTS notification_templates (
    id TEXT PRIMARY KEY,
    channel_id TEXT REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    format TEXT NOT NULL DEFAULT 'text',
    title_template TEXT NOT NULL DEFAULT '',
    body_template TEXT NOT NULL DEFAULT '',
    payload_template TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_notification_templates_channel ON notification_templates(channel_id);
CREATE INDEX IF NOT EXISTS idx_notification_templates_event ON notification_templates(event_type);
