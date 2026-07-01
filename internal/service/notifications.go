package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
)

var severityRank = map[string]int{"info": 0, "success": 1, "warning": 2, "error": 3, "critical": 4}
var templateVarPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

var defaultNotificationTemplates = map[string]model.NotificationTemplate{
	"test":            {EventType: "test", Format: "text", TitleTemplate: "{{title}}", BodyTemplate: "{{message}}", Enabled: true},
	"print.started":   {EventType: "print.started", Format: "text", TitleTemplate: "Print started: {{file_name}}", BodyTemplate: "Printer: {{printer_name}}\nFile: {{file_name}}\nStatus: {{status}}", Enabled: true},
	"print.completed": {EventType: "print.completed", Format: "text", TitleTemplate: "Print completed: {{file_name}}", BodyTemplate: "Printer: {{printer_name}}\nFile: {{file_name}}\nFilament: {{filament_grams}}g\nWasted: {{wasted_grams}}g", Enabled: true},
	"print.failed":    {EventType: "print.failed", Format: "text", TitleTemplate: "Print failed: {{file_name}}", BodyTemplate: "Printer: {{printer_name}}\nFile: {{file_name}}\nProgress: {{progress}}%\nNotes: {{notes}}", Enabled: true},
	"print.cancelled": {EventType: "print.cancelled", Format: "text", TitleTemplate: "Print cancelled: {{file_name}}", BodyTemplate: "Printer: {{printer_name}}\nFile: {{file_name}}\nProgress: {{progress}}%\nNotes: {{notes}}", Enabled: true},
	"printer.offline": {EventType: "printer.offline", Format: "text", TitleTemplate: "Printer offline", BodyTemplate: "Printer: {{printer_name}}\nStatus: {{status}}", Enabled: true},
	"printer.online":  {EventType: "printer.online", Format: "text", TitleTemplate: "Printer online", BodyTemplate: "Printer: {{printer_name}}\nStatus: {{status}}", Enabled: true},
	"printer.error":   {EventType: "printer.error", Format: "text", TitleTemplate: "Printer error", BodyTemplate: "Printer: {{printer_name}}\nStatus: {{status}}", Enabled: true},
	"emergency.stop":  {EventType: "emergency.stop", Format: "text", TitleTemplate: "Emergency stop", BodyTemplate: "{{message}}", Enabled: true},
	"queue.blocked":   {EventType: "queue.blocked", Format: "text", TitleTemplate: "Queue blocked", BodyTemplate: "{{message}}", Enabled: true},
	"spool.low":       {EventType: "spool.low", Format: "text", TitleTemplate: "Spool low", BodyTemplate: "{{message}}", Enabled: true},
}

type NotificationService struct {
	repo   *repository.NotificationRepository
	client *http.Client
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo, client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *NotificationService) ListChannels(ctx context.Context) ([]model.NotificationChannel, error) {
	channels, err := s.repo.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	for i := range channels {
		maskNotificationConfig(channels[i].Config)
	}
	return channels, nil
}

func (s *NotificationService) GetChannel(ctx context.Context, id uuid.UUID) (*model.NotificationChannel, error) {
	channel, err := s.repo.GetChannel(ctx, id)
	if err != nil || channel == nil {
		return channel, err
	}
	maskNotificationConfig(channel.Config)
	return channel, nil
}

func (s *NotificationService) CreateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	normalizeNotificationChannel(channel)
	if err := validateNotificationChannel(channel); err != nil {
		return err
	}
	return s.repo.CreateChannel(ctx, channel)
}

func (s *NotificationService) UpdateChannel(ctx context.Context, channel *model.NotificationChannel) error {
	existing, err := s.repo.GetChannel(ctx, channel.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("notification channel not found")
	}
	mergeSecretConfig(existing.Config, channel.Config)
	normalizeNotificationChannel(channel)
	if err := validateNotificationChannel(channel); err != nil {
		return err
	}
	return s.repo.UpdateChannel(ctx, channel)
}

func (s *NotificationService) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteChannel(ctx, id)
}

func (s *NotificationService) ListDeliveries(ctx context.Context, channelID *uuid.UUID, limit int) ([]model.NotificationDelivery, error) {
	return s.repo.ListDeliveries(ctx, channelID, limit)
}

func (s *NotificationService) ListTemplates(ctx context.Context, channelID *uuid.UUID) ([]model.NotificationTemplate, error) {
	return s.repo.ListTemplates(ctx, channelID)
}

func (s *NotificationService) UpsertTemplate(ctx context.Context, template *model.NotificationTemplate) error {
	normalizeNotificationTemplate(template)
	if err := validateNotificationTemplate(template); err != nil {
		return err
	}
	return s.repo.UpsertTemplate(ctx, template)
}

func (s *NotificationService) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteTemplate(ctx, id)
}

func (s *NotificationService) PreviewTemplate(ctx context.Context, template model.NotificationTemplate) (model.NotificationPreview, error) {
	normalizeNotificationTemplate(&template)
	if err := validateNotificationTemplate(&template); err != nil {
		return model.NotificationPreview{}, err
	}
	event := sampleNotificationEvent(template.EventType)
	return renderNotification(template, event), nil
}

func (s *NotificationService) SendTest(ctx context.Context, id uuid.UUID) error {
	channel, err := s.repo.GetChannel(ctx, id)
	if err != nil {
		return err
	}
	if channel == nil {
		return fmt.Errorf("notification channel not found")
	}
	event := model.NotificationEvent{Type: "test", Severity: "info", Title: "Daedalus test notification", Message: "This notification channel is working.", Timestamp: time.Now().UTC(), Data: map[string]any{"source": "test"}}
	return s.sendToChannel(ctx, *channel, event)
}

func (s *NotificationService) Dispatch(ctx context.Context, event model.NotificationEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	channels, err := s.repo.ListChannels(ctx)
	if err != nil {
		return
	}
	for _, channel := range channels {
		if !notificationChannelMatches(channel, event) {
			continue
		}
		go s.sendToChannel(context.Background(), channel, event)
	}
}

func (s *NotificationService) sendToChannel(ctx context.Context, channel model.NotificationChannel, event model.NotificationEvent) error {
	payload, _ := json.Marshal(event)
	delivery := &model.NotificationDelivery{ChannelID: channel.ID, EventType: event.Type, Severity: event.Severity, Status: "pending", Attempts: 1, Payload: string(payload)}
	err := s.send(ctx, channel, event)
	if err != nil {
		delivery.Status = "failed"
		delivery.LastError = err.Error()
		_ = s.repo.CreateDelivery(ctx, delivery)
		return err
	}
	now := time.Now().UTC()
	delivery.Status = "sent"
	delivery.SentAt = &now
	return s.repo.CreateDelivery(ctx, delivery)
}

func (s *NotificationService) send(ctx context.Context, channel model.NotificationChannel, event model.NotificationEvent) error {
	switch channel.Type {
	case "telegram":
		return s.sendTelegram(ctx, channel, event)
	case "discord":
		return s.sendDiscord(ctx, channel, event)
	case "webhook":
		return s.sendWebhook(ctx, channel, event)
	default:
		return fmt.Errorf("unsupported notification channel type")
	}
}

func (s *NotificationService) sendTelegram(ctx context.Context, channel model.NotificationChannel, event model.NotificationEvent) error {
	botToken := stringConfig(channel.Config, "bot_token")
	chatID := stringConfig(channel.Config, "chat_id")
	if botToken == "" || chatID == "" {
		return fmt.Errorf("telegram bot token and chat id are required")
	}
	rendered := s.renderForChannel(ctx, channel, event)
	text := fmt.Sprintf("<b>%s</b>\n\n%s", html.EscapeString(rendered.Title), html.EscapeString(rendered.Body))
	body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML"})
	return s.postJSON(ctx, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken), body, nil)
}

func (s *NotificationService) sendDiscord(ctx context.Context, channel model.NotificationChannel, event model.NotificationEvent) error {
	webhookURL := stringConfig(channel.Config, "webhook_url")
	if webhookURL == "" {
		return fmt.Errorf("discord webhook url is required")
	}
	rendered := s.renderForChannel(ctx, channel, event)
	body, _ := json.Marshal(map[string]any{"embeds": []map[string]any{{"title": rendered.Title, "description": rendered.Body, "color": discordColor(event.Severity), "timestamp": event.Timestamp.Format(time.RFC3339)}}})
	return s.postJSON(ctx, webhookURL, body, nil)
}

func (s *NotificationService) sendWebhook(ctx context.Context, channel model.NotificationChannel, event model.NotificationEvent) error {
	webhookURL := stringConfig(channel.Config, "url")
	if webhookURL == "" {
		return fmt.Errorf("webhook url is required")
	}
	rendered := s.renderForChannel(ctx, channel, event)
	body, _ := json.Marshal(rendered.Payload)
	headers := map[string]string{}
	if raw, ok := channel.Config["headers"].(map[string]any); ok {
		for key, value := range raw {
			headers[key] = fmt.Sprint(value)
		}
	}
	if secret := stringConfig(channel.Config, "secret"); secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		headers["X-Daedalus-Signature"] = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	return s.postJSON(ctx, webhookURL, body, headers)
}

func (s *NotificationService) postJSON(ctx context.Context, url string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Daedalus")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification request failed: %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (s *NotificationService) renderForChannel(ctx context.Context, channel model.NotificationChannel, event model.NotificationEvent) model.NotificationPreview {
	template := defaultTemplateForEvent(event.Type)
	if custom, err := s.repo.GetTemplate(ctx, channel.ID, event.Type); err == nil && custom != nil && custom.Enabled {
		template = *custom
	}
	if template.Format == "" {
		template.Format = defaultFormatForChannel(channel.Type)
	}
	return renderNotification(template, event)
}

func renderNotification(template model.NotificationTemplate, event model.NotificationEvent) model.NotificationPreview {
	values := notificationTemplateValues(event)
	title := renderTemplateString(template.TitleTemplate, values)
	body := renderTemplateString(template.BodyTemplate, values)
	if title == "" {
		title = event.Title
	}
	if body == "" {
		body = event.Message
	}
	payload := map[string]any{}
	if strings.TrimSpace(template.PayloadTemplate) != "" {
		renderedPayload := renderTemplateString(template.PayloadTemplate, values)
		if err := json.Unmarshal([]byte(renderedPayload), &payload); err != nil {
			payload = defaultNotificationPayload(event, title, body)
		}
	} else {
		payload = defaultNotificationPayload(event, title, body)
	}
	return model.NotificationPreview{Title: title, Body: body, Payload: payload}
}

func renderTemplateString(template string, values map[string]string) string {
	return templateVarPattern.ReplaceAllStringFunc(template, func(match string) string {
		parts := templateVarPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return ""
		}
		if value, ok := values[parts[1]]; ok {
			return value
		}
		return "—"
	})
}

func notificationTemplateValues(event model.NotificationEvent) map[string]string {
	values := map[string]string{
		"event":     event.Type,
		"severity":  event.Severity,
		"title":     event.Title,
		"message":   event.Message,
		"timestamp": event.Timestamp.Format(time.RFC3339),
	}
	if event.PrinterID != nil {
		values["printer_id"] = event.PrinterID.String()
	}
	for key, value := range event.Data {
		values[key] = fmt.Sprint(value)
	}
	return values
}

func defaultNotificationPayload(event model.NotificationEvent, title string, body string) map[string]any {
	payload := map[string]any{
		"event":     event.Type,
		"severity":  event.Severity,
		"title":     title,
		"message":   body,
		"timestamp": event.Timestamp.Format(time.RFC3339),
		"data":      event.Data,
	}
	if event.PrinterID != nil {
		payload["printer_id"] = event.PrinterID.String()
	}
	return payload
}

func defaultTemplateForEvent(eventType string) model.NotificationTemplate {
	if template, ok := defaultNotificationTemplates[eventType]; ok {
		return template
	}
	return model.NotificationTemplate{EventType: eventType, Format: "text", TitleTemplate: "{{title}}", BodyTemplate: "{{message}}", Enabled: true}
}

func defaultFormatForChannel(channelType string) string {
	switch channelType {
	case "telegram":
		return "telegram_html"
	case "discord":
		return "discord_embed"
	case "webhook":
		return "json"
	default:
		return "text"
	}
}

func sampleNotificationEvent(eventType string) model.NotificationEvent {
	if eventType == "" {
		eventType = "print.completed"
	}
	printerID := uuid.New()
	return model.NotificationEvent{
		Type:      eventType,
		Severity:  severityForEvent(eventType),
		Title:     defaultTemplateForEvent(eventType).TitleTemplate,
		Message:   "Sample notification message",
		Timestamp: time.Now().UTC(),
		PrinterID: &printerID,
		Data: map[string]any{
			"printer_name":   "Neptune 4 Plus",
			"printer_model":  "Neptune 4 Plus",
			"file_name":      "chain-base.gcode",
			"status":         "done",
			"progress":       "100",
			"duration":       "46m",
			"filament_grams": "18.4",
			"wasted_grams":   "0",
			"notes":          "Sample notes",
		},
	}
}

func severityForEvent(eventType string) string {
	switch eventType {
	case "print.completed", "printer.online":
		return "success"
	case "print.failed", "printer.error", "emergency.stop":
		return "error"
	case "print.cancelled", "printer.offline", "queue.blocked", "spool.low":
		return "warning"
	default:
		return "info"
	}
}

func normalizeNotificationTemplate(template *model.NotificationTemplate) {
	template.EventType = strings.TrimSpace(template.EventType)
	template.Format = strings.TrimSpace(template.Format)
	if template.Format == "" {
		template.Format = "text"
	}
}

func validateNotificationTemplate(template *model.NotificationTemplate) error {
	if template.EventType == "" {
		return fmt.Errorf("event type is required")
	}
	switch template.Format {
	case "text", "telegram_html", "discord_embed", "json":
	default:
		return fmt.Errorf("unsupported template format")
	}
	if strings.TrimSpace(template.PayloadTemplate) != "" {
		rendered := renderTemplateString(template.PayloadTemplate, notificationTemplateValues(sampleNotificationEvent(template.EventType)))
		var payload map[string]any
		if err := json.Unmarshal([]byte(rendered), &payload); err != nil {
			return fmt.Errorf("payload template must render valid JSON")
		}
	}
	return nil
}

func normalizeNotificationChannel(channel *model.NotificationChannel) {
	channel.Name = strings.TrimSpace(channel.Name)
	channel.Type = strings.ToLower(strings.TrimSpace(channel.Type))
	channel.MinSeverity = strings.ToLower(strings.TrimSpace(channel.MinSeverity))
	if channel.MinSeverity == "" {
		channel.MinSeverity = "info"
	}
	if channel.Config == nil {
		channel.Config = map[string]any{}
	}
	if channel.Events == nil {
		channel.Events = []string{}
	}
}

func validateNotificationChannel(channel *model.NotificationChannel) error {
	if channel.Name == "" {
		return fmt.Errorf("name is required")
	}
	if channel.Type != "telegram" && channel.Type != "discord" && channel.Type != "webhook" {
		return fmt.Errorf("unsupported notification channel type")
	}
	if _, ok := severityRank[channel.MinSeverity]; !ok {
		return fmt.Errorf("unsupported minimum severity")
	}
	return nil
}

func notificationChannelMatches(channel model.NotificationChannel, event model.NotificationEvent) bool {
	if !channel.Enabled {
		return false
	}
	if severityRank[event.Severity] < severityRank[channel.MinSeverity] {
		return false
	}
	if len(channel.Events) > 0 {
		matched := false
		for _, eventType := range channel.Events {
			if eventType == event.Type || eventType == "*" {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if event.PrinterID != nil && len(channel.PrinterIDs) > 0 {
		for _, id := range channel.PrinterIDs {
			if id == *event.PrinterID {
				return true
			}
		}
		return false
	}
	return true
}

func formatNotificationText(event model.NotificationEvent) string {
	return fmt.Sprintf("<b>%s</b>\n\n%s", event.Title, event.Message)
}

func discordColor(severity string) int {
	switch severity {
	case "success":
		return 5763719
	case "warning":
		return 16705372
	case "error", "critical":
		return 15548997
	default:
		return 5793266
	}
}

func stringConfig(config map[string]any, key string) string {
	if value, ok := config[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func maskNotificationConfig(config map[string]any) {
	for _, key := range []string{"bot_token", "secret", "webhook_url", "url"} {
		if value := stringConfig(config, key); value != "" {
			if len(value) <= 8 {
				config[key] = "********"
			} else {
				config[key] = value[:4] + "..." + value[len(value)-4:]
			}
		}
	}
}

func mergeSecretConfig(existing map[string]any, next map[string]any) {
	for _, key := range []string{"bot_token", "secret", "webhook_url", "url"} {
		value := stringConfig(next, key)
		if value == "" || strings.Contains(value, "...") || value == "********" {
			if existingValue := stringConfig(existing, key); existingValue != "" {
				next[key] = existingValue
			}
		}
	}
}
