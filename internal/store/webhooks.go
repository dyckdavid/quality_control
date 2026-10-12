package store

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

const webhookCols = `id, name, url, secret, events, active, description, created_by, created_at, updated_at`
const deliveryCols = `id, webhook_id, event_type, payload, status, attempt_count, response_code, response_body, error_message, created_at, delivered_at`
const outboxCols = `id, channel, event_type, destination, subject, body, payload, status, error_message, created_at, sent_at`

func scanWebhook(row scannable) (domain.Webhook, error) {
	var v domain.Webhook
	var created *uuid.UUID
	err := row.Scan(&v.ID, &v.Name, &v.URL, &v.Secret, &v.Events, &v.Active, &v.Description, &created, &v.CreatedAt, &v.UpdatedAt)
	v.CreatedBy = uuidOrNil(created)
	return v, err
}

func scanDelivery(row scannable) (domain.WebhookDelivery, error) {
	var v domain.WebhookDelivery
	err := row.Scan(&v.ID, &v.WebhookID, &v.EventType, &v.Payload, &v.Status, &v.AttemptCount, &v.ResponseCode, &v.ResponseBody, &v.ErrorMessage, &v.CreatedAt, &v.DeliveredAt)
	return v, err
}

func scanOutbox(row scannable) (domain.NotificationOutboxItem, error) {
	var v domain.NotificationOutboxItem
	err := row.Scan(&v.ID, &v.Channel, &v.EventType, &v.Destination, &v.Subject, &v.Body, &v.Payload, &v.Status, &v.ErrorMessage, &v.CreatedAt, &v.SentAt)
	return v, err
}

func validateWebhookURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if len(raw) > 2000 {
		return invalid("url is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return invalid("url must be an absolute http or https URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return invalid("url must use http or https")
	}
	return nil
}

func validateEvents(events []string) ([]string, error) {
	if len(events) == 0 {
		return nil, invalid("events is required")
	}
	if len(events) > 32 {
		return nil, invalid("too many events")
	}
	allowed := map[string]bool{}
	for _, e := range domain.WebhookEventTypes() {
		allowed[e] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(events))
	for _, e := range events {
		e = domain.Trim(e)
		if !allowed[e] {
			return nil, invalid("unknown event type: " + e)
		}
		if seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	return out, nil
}

// ListWebhooks returns webhook subscriptions.
func (s *Store) ListWebhooks(ctx context.Context, active *bool, limit, offset int) ([]domain.Webhook, int, error) {
	limit, offset = clamp(limit, offset, 200)
	var c clause
	if active != nil {
		c.add("active = ?", *active)
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM webhooks`+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `SELECT `+webhookCols+` FROM webhooks`+c.where()+` ORDER BY name ASC LIMIT $`+itoa(len(c.args)+1)+` OFFSET $`+itoa(len(c.args)+2), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	out, err := collect(rows, scanWebhook)
	return out, total, err
}

// GetWebhook returns one subscription.
func (s *Store) GetWebhook(ctx context.Context, id uuid.UUID) (domain.Webhook, error) {
	v, err := scanWebhook(s.pool.QueryRow(ctx, `SELECT `+webhookCols+` FROM webhooks WHERE id = $1`, id))
	return v, mapErr(err)
}

// CreateWebhook registers a subscription.
func (s *Store) CreateWebhook(ctx context.Context, actor domain.Actor, in domain.NewWebhook) (domain.Webhook, error) {
	in.Name = domain.Trim(in.Name)
	in.URL = domain.Trim(in.URL)
	in.Secret = domain.Trim(in.Secret)
	in.Description = domain.Trim(in.Description)
	if err := domain.CheckLabel(in.Name, "name", 120); err != nil {
		return domain.Webhook{}, wrapInvalid(err)
	}
	if err := validateWebhookURL(in.URL); err != nil {
		return domain.Webhook{}, err
	}
	if err := domain.CheckOptional(in.Secret, "secret", 200); err != nil {
		return domain.Webhook{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Description, "description", 1000); err != nil {
		return domain.Webhook{}, wrapInvalid(err)
	}
	events, err := validateEvents(in.Events)
	if err != nil {
		return domain.Webhook{}, err
	}
	var created any
	if actor.ID != uuid.Nil {
		created = actor.ID
	}
	out, err := scanWebhook(s.pool.QueryRow(ctx, `
		INSERT INTO webhooks (id, name, url, secret, events, description, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+webhookCols,
		uuid.New(), in.Name, in.URL, in.Secret, events, in.Description, created))
	if err != nil {
		return domain.Webhook{}, mapErr(err)
	}
	return out, s.writeAudit(ctx, s.pool, actor, "webhook.create", "webhook", out.ID, "Created webhook "+out.Name, nil, redactWebhook(out))
}

// UpdateWebhook patches a subscription.
func (s *Store) UpdateWebhook(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchWebhook) (domain.Webhook, error) {
	before, err := s.GetWebhook(ctx, id)
	if err != nil {
		return domain.Webhook{}, mapErr(err)
	}
	set := setter{}
	if in.Name != nil {
		name := domain.Trim(*in.Name)
		if err := domain.CheckLabel(name, "name", 120); err != nil {
			return domain.Webhook{}, wrapInvalid(err)
		}
		set.add("name", name)
	}
	if in.URL != nil {
		u := domain.Trim(*in.URL)
		if err := validateWebhookURL(u); err != nil {
			return domain.Webhook{}, err
		}
		set.add("url", u)
	}
	if in.Secret != nil {
		sec := domain.Trim(*in.Secret)
		if err := domain.CheckOptional(sec, "secret", 200); err != nil {
			return domain.Webhook{}, wrapInvalid(err)
		}
		set.add("secret", sec)
	}
	if in.Events != nil {
		events, err := validateEvents(in.Events)
		if err != nil {
			return domain.Webhook{}, err
		}
		set.add("events", events)
	}
	if in.Active != nil {
		set.add("active", *in.Active)
	}
	if in.Description != nil {
		d := domain.Trim(*in.Description)
		if err := domain.CheckOptional(d, "description", 1000); err != nil {
			return domain.Webhook{}, wrapInvalid(err)
		}
		set.add("description", d)
	}
	if set.empty() {
		return before, nil
	}
	args := append(set.args, id)
	out, err := scanWebhook(s.pool.QueryRow(ctx, `UPDATE webhooks SET `+strings.Join(set.sets, ", ")+` WHERE id = $`+itoa(len(args))+` RETURNING `+webhookCols, args...))
	if err != nil {
		return domain.Webhook{}, mapErr(err)
	}
	return out, s.writeAudit(ctx, s.pool, actor, "webhook.update", "webhook", id, "Updated webhook "+out.Name, redactWebhook(before), redactWebhook(out))
}

func redactWebhook(w domain.Webhook) domain.Webhook {
	if w.Secret != "" {
		w.Secret = "***"
	}
	return w
}

// ListActiveWebhooksForEvent returns active hooks matching an event.
func (s *Store) ListActiveWebhooksForEvent(ctx context.Context, eventType string) ([]domain.Webhook, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+webhookCols+` FROM webhooks
		WHERE active AND $1 = ANY(events)
		ORDER BY created_at`, eventType)
	if err != nil {
		return nil, mapErr(err)
	}
	return collect(rows, scanWebhook)
}

// CreateWebhookDelivery inserts a pending delivery row.
func (s *Store) CreateWebhookDelivery(ctx context.Context, webhookID uuid.UUID, eventType string, payload any) (domain.WebhookDelivery, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return domain.WebhookDelivery{}, err
	}
	return scanDelivery(s.pool.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, status)
		VALUES ($1,$2,$3,$4,'pending') RETURNING `+deliveryCols, uuid.New(), webhookID, eventType, raw))
}

// MarkWebhookDelivery records the outcome of a delivery attempt.
func (s *Store) MarkWebhookDelivery(ctx context.Context, id uuid.UUID, status string, code *int, body, errMsg string) (domain.WebhookDelivery, error) {
	body = clip(body, 4000)
	errMsg = clip(errMsg, 1000)
	var delivered any
	if status == domain.WebhookDelivered {
		delivered = time.Now().UTC()
	}
	return scanDelivery(s.pool.QueryRow(ctx, `
		UPDATE webhook_deliveries
		SET status = $1, attempt_count = attempt_count + 1, response_code = $2, response_body = $3,
			error_message = $4, delivered_at = COALESCE($5::timestamptz, delivered_at)
		WHERE id = $6 RETURNING `+deliveryCols, status, code, body, errMsg, delivered, id))
}

// ListWebhookDeliveries lists deliveries, optionally for one webhook.
func (s *Store) ListWebhookDeliveries(ctx context.Context, webhookID *uuid.UUID, status string, limit, offset int) ([]domain.WebhookDelivery, int, error) {
	limit, offset = clamp(limit, offset, 200)
	var c clause
	if webhookID != nil {
		c.add("webhook_id = ?", *webhookID)
	}
	status = domain.Trim(status)
	if status != "" {
		c.add("status = ?", status)
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries`+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `SELECT `+deliveryCols+` FROM webhook_deliveries`+c.where()+` ORDER BY created_at DESC LIMIT $`+itoa(len(c.args)+1)+` OFFSET $`+itoa(len(c.args)+2), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	out, err := collect(rows, scanDelivery)
	return out, total, err
}

// EnqueueEmailNotification stores an email in the outbox (delivery is external).
func (s *Store) EnqueueEmailNotification(ctx context.Context, actor domain.Actor, in domain.NewEmailNotification) (domain.NotificationOutboxItem, error) {
	in.Destination = domain.Trim(in.Destination)
	in.Subject = domain.Trim(in.Subject)
	in.Body = domain.Trim(in.Body)
	in.EventType = domain.Trim(in.EventType)
	if err := domain.CheckEmail(in.Destination); err != nil {
		return domain.NotificationOutboxItem{}, wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.Subject, "subject", 200); err != nil {
		return domain.NotificationOutboxItem{}, wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.Body, "body", 10000); err != nil {
		return domain.NotificationOutboxItem{}, wrapInvalid(err)
	}
	if in.EventType == "" {
		in.EventType = "custom"
	}
	if len(in.EventType) > 80 {
		return domain.NotificationOutboxItem{}, invalid("event_type is too long")
	}
	var payload any
	if len(in.Payload) > 0 {
		payload = in.Payload
	}
	out, err := scanOutbox(s.pool.QueryRow(ctx, `
		INSERT INTO notification_outbox (id, channel, event_type, destination, subject, body, payload, status)
		VALUES ($1,'email',$2,$3,$4,$5,$6,'pending') RETURNING `+outboxCols,
		uuid.New(), in.EventType, in.Destination, in.Subject, in.Body, payload))
	if err != nil {
		return domain.NotificationOutboxItem{}, mapErr(err)
	}
	return out, s.writeAudit(ctx, s.pool, actor, "notification.enqueue", "notification", out.ID, "Queued email to "+out.Destination, nil, out)
}

// ListNotificationOutbox lists queued notifications.
func (s *Store) ListNotificationOutbox(ctx context.Context, status, channel string, limit, offset int) ([]domain.NotificationOutboxItem, int, error) {
	limit, offset = clamp(limit, offset, 200)
	var c clause
	if status = domain.Trim(status); status != "" {
		c.add("status = ?", status)
	}
	if channel = domain.Trim(channel); channel != "" {
		c.add("channel = ?", channel)
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM notification_outbox`+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `SELECT `+outboxCols+` FROM notification_outbox`+c.where()+` ORDER BY created_at DESC LIMIT $`+itoa(len(c.args)+1)+` OFFSET $`+itoa(len(c.args)+2), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	out, err := collect(rows, scanOutbox)
	return out, total, err
}

// MarkNotificationSent marks an outbox row sent or failed.
func (s *Store) MarkNotificationSent(ctx context.Context, id uuid.UUID, status, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_outbox SET status = $1, error_message = $2, sent_at = CASE WHEN $1 = 'sent' THEN now() ELSE sent_at END
		WHERE id = $3`, status, errMsg, id)
	return mapErr(err)
}
