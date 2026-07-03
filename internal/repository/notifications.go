package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type NotificationRepository struct {
	db *sql.DB
}

func (r *NotificationRepository) ListChannels(ctx context.Context) ([]model.NotificationChannel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, type, enabled, config_json, events_json, printer_ids_json, min_severity, created_at, updated_at FROM notification_channels ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	channels := []model.NotificationChannel{}
	for rows.Next() {
		channel, err := scanNotificationChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (r *NotificationRepository) GetChannel(ctx context.Context, id uuid.UUID) (*model.NotificationChannel, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, type, enabled, config_json, events_json, printer_ids_json, min_severity, created_at, updated_at FROM notification_channels WHERE id = ?`, id)
	channel, err := scanNotificationChannel(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &channel, nil
}

func (r *NotificationRepository) CreateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	channel.ID = uuid.New()
	now := time.Now().UTC()
	channel.CreatedAt = now
	channel.UpdatedAt = now
	configJSON, eventsJSON, printerIDsJSON := marshalNotificationChannel(channel)
	_, err := r.db.ExecContext(ctx, `INSERT INTO notification_channels (id, name, type, enabled, config_json, events_json, printer_ids_json, min_severity, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, channel.ID, channel.Name, channel.Type, channel.Enabled, configJSON, eventsJSON, printerIDsJSON, channel.MinSeverity, channel.CreatedAt, channel.UpdatedAt)
	return err
}

func (r *NotificationRepository) UpdateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	channel.UpdatedAt = time.Now().UTC()
	configJSON, eventsJSON, printerIDsJSON := marshalNotificationChannel(channel)
	res, err := r.db.ExecContext(ctx, `UPDATE notification_channels SET name = ?, type = ?, enabled = ?, config_json = ?, events_json = ?, printer_ids_json = ?, min_severity = ?, updated_at = ? WHERE id = ?`, channel.Name, channel.Type, channel.Enabled, configJSON, eventsJSON, printerIDsJSON, channel.MinSeverity, channel.UpdatedAt, channel.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *NotificationRepository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id = ?`, id)
	return err
}

func (r *NotificationRepository) CreateDelivery(ctx context.Context, delivery *model.NotificationDelivery) error {
	delivery.ID = uuid.New()
	delivery.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO notification_deliveries (id, channel_id, event_type, severity, status, attempts, last_error, payload_json, sent_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, delivery.ID, delivery.ChannelID, delivery.EventType, delivery.Severity, delivery.Status, delivery.Attempts, delivery.LastError, delivery.Payload, delivery.SentAt, delivery.CreatedAt)
	return err
}

func (r *NotificationRepository) ListDeliveries(ctx context.Context, channelID *uuid.UUID, limit int) ([]model.NotificationDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT id, channel_id, event_type, severity, status, attempts, last_error, payload_json, sent_at, created_at FROM notification_deliveries`
	args := []any{}
	if channelID != nil {
		query += ` WHERE channel_id = ?`
		args = append(args, *channelID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	deliveries := []model.NotificationDelivery{}
	for rows.Next() {
		var delivery model.NotificationDelivery
		var id, channelID, createdAt string
		var sentAt sql.NullString
		if err := rows.Scan(&id, &channelID, &delivery.EventType, &delivery.Severity, &delivery.Status, &delivery.Attempts, &delivery.LastError, &delivery.Payload, &sentAt, &createdAt); err != nil {
			return nil, err
		}
		deliveryID, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		parsedChannelID, err := uuid.Parse(channelID)
		if err != nil {
			return nil, err
		}
		delivery.ID = deliveryID
		delivery.ChannelID = parsedChannelID
		delivery.CreatedAt, _ = parseTime(createdAt)
		if sentAt.Valid {
			parsedSentAt, err := parseTime(sentAt.String)
			if err == nil {
				delivery.SentAt = &parsedSentAt
			}
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

type notificationScanner interface {
	Scan(dest ...any) error
}

func scanNotificationChannel(row notificationScanner) (model.NotificationChannel, error) {
	var channel model.NotificationChannel
	var id, createdAt, updatedAt string
	var configJSON, eventsJSON, printerIDsJSON []byte
	if err := row.Scan(&id, &channel.Name, &channel.Type, &channel.Enabled, &configJSON, &eventsJSON, &printerIDsJSON, &channel.MinSeverity, &createdAt, &updatedAt); err != nil {
		return channel, err
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return channel, err
	}
	channel.ID = parsedID
	channel.CreatedAt, _ = parseTime(createdAt)
	channel.UpdatedAt, _ = parseTime(updatedAt)
	_ = json.Unmarshal(configJSON, &channel.Config)
	_ = json.Unmarshal(eventsJSON, &channel.Events)
	var printerIDs []string
	_ = json.Unmarshal(printerIDsJSON, &printerIDs)
	channel.PrinterIDs = []uuid.UUID{}
	for _, id := range printerIDs {
		parsed, err := uuid.Parse(id)
		if err == nil {
			channel.PrinterIDs = append(channel.PrinterIDs, parsed)
		}
	}
	if channel.Config == nil {
		channel.Config = map[string]any{}
	}
	if channel.Events == nil {
		channel.Events = []string{}
	}
	return channel, nil
}

func marshalNotificationChannel(channel *model.NotificationChannel) ([]byte, []byte, []byte) {
	configJSON, _ := json.Marshal(channel.Config)
	eventsJSON, _ := json.Marshal(channel.Events)
	printerIDs := make([]string, 0, len(channel.PrinterIDs))
	for _, id := range channel.PrinterIDs {
		printerIDs = append(printerIDs, id.String())
	}
	printerIDsJSON, _ := json.Marshal(printerIDs)
	return configJSON, eventsJSON, printerIDsJSON
}

func (r *NotificationRepository) ListTemplates(ctx context.Context, channelID *uuid.UUID) ([]model.NotificationTemplate, error) {
	query := `SELECT id, channel_id, event_type, format, title_template, body_template, payload_template, enabled, created_at, updated_at FROM notification_templates`
	args := []any{}
	if channelID != nil {
		query += ` WHERE channel_id = ?`
		args = append(args, *channelID)
	}
	query += ` ORDER BY event_type ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	templates := []model.NotificationTemplate{}
	for rows.Next() {
		template, err := scanNotificationTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, rows.Err()
}

func (r *NotificationRepository) GetTemplate(ctx context.Context, channelID uuid.UUID, eventType string) (*model.NotificationTemplate, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, channel_id, event_type, format, title_template, body_template, payload_template, enabled, created_at, updated_at FROM notification_templates WHERE channel_id = ? AND event_type = ?`, channelID, eventType)
	template, err := scanNotificationTemplate(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

func (r *NotificationRepository) UpsertTemplate(ctx context.Context, template *model.NotificationTemplate) error {
	if template.ID == uuid.Nil {
		template.ID = uuid.New()
	}
	now := time.Now().UTC()
	if template.CreatedAt.IsZero() {
		template.CreatedAt = now
	}
	template.UpdatedAt = now
	var channelID any
	if template.ChannelID != nil {
		channelID = *template.ChannelID
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO notification_templates (id, channel_id, event_type, format, title_template, body_template, payload_template, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(channel_id, event_type) DO UPDATE SET format = excluded.format, title_template = excluded.title_template, body_template = excluded.body_template, payload_template = excluded.payload_template, enabled = excluded.enabled, updated_at = excluded.updated_at`, template.ID, channelID, template.EventType, template.Format, template.TitleTemplate, template.BodyTemplate, template.PayloadTemplate, template.Enabled, template.CreatedAt, template.UpdatedAt)
	return err
}

func (r *NotificationRepository) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM notification_templates WHERE id = ?`, id)
	return err
}

func scanNotificationTemplate(row notificationScanner) (model.NotificationTemplate, error) {
	var template model.NotificationTemplate
	var id, createdAt, updatedAt string
	var channelID sql.NullString
	if err := row.Scan(&id, &channelID, &template.EventType, &template.Format, &template.TitleTemplate, &template.BodyTemplate, &template.PayloadTemplate, &template.Enabled, &createdAt, &updatedAt); err != nil {
		return template, err
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return template, err
	}
	template.ID = parsedID
	if channelID.Valid && channelID.String != "" {
		parsedChannelID, err := uuid.Parse(channelID.String)
		if err != nil {
			return template, err
		}
		template.ChannelID = &parsedChannelID
	}
	template.CreatedAt, _ = parseTime(createdAt)
	template.UpdatedAt, _ = parseTime(updatedAt)
	return template, nil
}

// ProjectSupplyRepository handles project supply database operations.
