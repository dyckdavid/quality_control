package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) {
	active, ok := queryBool(w, r, "active")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListWebhooks(r.Context(), active, limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for i := range rows {
		if rows[i].Secret != "" {
			rows[i].Secret = "***"
		}
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) {
	var in domain.NewWebhook
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateWebhook(r.Context(), s.actor(r), in)
	if err == nil && v.Secret != "" {
		v.Secret = "***"
	}
	s.created(w, r, "/api/v1/webhooks/", v.ID, v, err)
}

func (s *Server) getWebhook(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetWebhook(r.Context(), id)
	if err == nil && v.Secret != "" {
		v.Secret = "***"
	}
	s.one(w, r, v, err)
}

func (s *Server) updateWebhook(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchWebhook
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateWebhook(r.Context(), s.actor(r), id, in)
	if err == nil && v.Secret != "" {
		v.Secret = "***"
	}
	s.one(w, r, v, err)
}

func (s *Server) listWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	webhookID, ok := queryUUID(w, r, "webhook_id")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListWebhookDeliveries(r.Context(), webhookID, r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) enqueueEmail(w http.ResponseWriter, r *http.Request) {
	var in domain.NewEmailNotification
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.EnqueueEmailNotification(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/notifications/", v.ID, v, err)
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListNotificationOutbox(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("channel"), limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) listWebhookEvents(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, domain.WebhookEventTypes())
}

// enqueueEvent fans out a domain event to matching webhooks asynchronously.
// It never changes the HTTP response of the caller.
func (s *Server) enqueueEvent(eventType string, payload map[string]any) {
	if s.store == nil {
		return
	}
	go s.deliverEvent(eventType, payload)
}

func (s *Server) deliverEvent(eventType string, payload map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	hooks, err := s.store.ListActiveWebhooksForEvent(ctx, eventType)
	if err != nil {
		slog.Warn("webhook lookup failed", "event", eventType, "err", err)
		return
	}
	body := map[string]any{
		"event":      eventType,
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
		"data":       payload,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	for _, hook := range hooks {
		delivery, err := s.store.CreateWebhookDelivery(ctx, hook.ID, eventType, json.RawMessage(raw))
		if err != nil {
			slog.Warn("webhook delivery create failed", "webhook_id", hook.ID, "err", err)
			continue
		}
		s.postWebhook(ctx, hook, delivery.ID, raw)
	}
}

func (s *Server) postWebhook(ctx context.Context, hook domain.Webhook, deliveryID uuid.UUID, raw []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(raw))
	if err != nil {
		_, _ = s.store.MarkWebhookDelivery(ctx, deliveryID, domain.WebhookFailed, nil, "", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "quality-control-webhook/1")
	req.Header.Set("X-QC-Event", "")
	if hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		_, _ = mac.Write(raw)
		req.Header.Set("X-QC-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		_, _ = s.store.MarkWebhookDelivery(ctx, deliveryID, domain.WebhookFailed, nil, "", err.Error())
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
	code := resp.StatusCode
	status := domain.WebhookDelivered
	errMsg := ""
	if code < 200 || code >= 300 {
		status = domain.WebhookFailed
		errMsg = "non-2xx response"
	}
	_, _ = s.store.MarkWebhookDelivery(ctx, deliveryID, status, &code, string(respBody), errMsg)
}

func (s *Server) seedDemo(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.SeedDemo(r.Context(), s.actor(r))
	s.created(w, r, "/api/v1/admin/seed-demo/", v.SiteID, v, err)
}
