package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"quality_control/internal/auth"
	"quality_control/internal/config"
	"quality_control/internal/db"
	"quality_control/internal/domain"
	"quality_control/internal/store"

	"github.com/google/uuid"
)

func TestOpenAPIHealthMetricsUnauthenticated(t *testing.T) {
	s := New(config.Config{}, nil, auth.NewManager("0123456789abcdef0123456789abcdef", time.Minute, time.Hour))
	h := s.Handler()

	for _, path := range []string{"/healthz", "/metrics", "/api/v1/openapi.yaml", "/api/v1/openapi.json"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil))
	if !strings.Contains(rr.Body.String(), "openapi: 3.0.3") {
		t.Fatalf("openapi body: %s", rr.Body.String()[:min(200, rr.Body.Len())])
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rr.Body.String(), "qc_http_requests_total") {
		t.Fatalf("metrics: %s", rr.Body.String())
	}
}

func TestExtensionsFlow(t *testing.T) {
	dsn := os.Getenv("QC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("QC_TEST_DATABASE_URL is not set")
	}
	if !strings.Contains(dsn, "test") {
		t.Fatal("refusing to run against a database whose name does not contain test")
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			demo_seed_runs, notification_outbox, webhook_deliveries, webhooks,
			checklist_run_items, checklist_runs, checklist_items, checklists, request_audit_log,
			audit_log, attachments, corrective_actions, nonconformances,
			inspection_defects, inspection_results, inspections, lots,
			characteristics, inspection_plans, defect_codes, gauges, products,
			suppliers, stations, sites, refresh_tokens, users, document_counters
		RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	if err := st.EnsureAdmin(ctx, "admin", "admin@localhost", "Administrator", "integration-secret"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cfg := config.Config{DataDir: dir, JWTSecret: "0123456789abcdef0123456789abcdef"}
	api := &client{h: New(cfg, st, auth.NewManager(cfg.JWTSecret, 15*time.Minute, 24*time.Hour)).Handler()}

	var session tokenResponse
	api.call(t, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "integration-secret"}, &session, http.StatusOK)
	api.token = session.AccessToken

	var seed domain.DemoSeedResult
	api.call(t, http.MethodPost, "/api/v1/admin/seed-demo", map[string]any{}, &seed, http.StatusCreated)
	if seed.InspectionID == uuid.Nil || seed.ChecklistID == uuid.Nil {
		t.Fatalf("seed incomplete: %+v", seed)
	}

	var checklist domain.ChecklistDetail
	api.call(t, http.MethodGet, "/api/v1/checklists/"+seed.ChecklistID.String(), nil, &checklist, http.StatusOK)
	if len(checklist.Items) < 2 {
		t.Fatalf("checklist items: %+v", checklist.Items)
	}

	hookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(hookServer.Close)

	var hook domain.Webhook
	api.call(t, http.MethodPost, "/api/v1/webhooks", domain.NewWebhook{
		Name: "test-hook", URL: hookServer.URL, Secret: "s3cret", Events: []string{domain.EventChecklistCompleted},
	}, &hook, http.StatusCreated)

	var run domain.ChecklistRunDetail
	api.call(t, http.MethodPost, "/api/v1/inspections/"+seed.InspectionID.String()+"/checklists", domain.StartChecklistRun{
		ChecklistID: seed.ChecklistID, Notes: "floor run",
	}, &run, http.StatusCreated)
	for _, item := range run.Items {
		if !item.Required {
			continue
		}
		body := domain.AnswerChecklistItem{Notes: "ok"}
		switch item.ItemType {
		case domain.ChecklistItemYesNo:
			body.Result = "yes"
		case domain.ChecklistItemPassFail:
			body.Result = "pass"
		case domain.ChecklistItemText:
			body.Result = "looks good"
		}
		api.call(t, http.MethodPost, "/api/v1/checklist-runs/"+run.ID.String()+"/items/"+item.ID.String(), body, &domain.ChecklistRunItem{}, http.StatusOK)
	}
	api.call(t, http.MethodPost, "/api/v1/checklist-runs/"+run.ID.String()+"/complete", map[string]any{}, &run, http.StatusOK)
	if run.Status != domain.ChecklistRunCompleted {
		t.Fatalf("run status %s", run.Status)
	}

	var rates []domain.DefectRateRow
	api.call(t, http.MethodGet, "/api/v1/stats/defect-rates?group_by=product", nil, &rates, http.StatusOK)
	var aging domain.AgingReport
	api.call(t, http.MethodGet, "/api/v1/stats/ncr-aging", nil, &aging, http.StatusOK)
	api.call(t, http.MethodGet, "/api/v1/stats/capa-aging", nil, &aging, http.StatusOK)
	var trends domain.TrendReport
	api.call(t, http.MethodGet, "/api/v1/stats/trends", nil, &trends, http.StatusOK)
	if len(trends.Points) == 0 {
		t.Fatal("expected trend points")
	}

	var hits []domain.SearchHit
	api.call(t, http.MethodGet, "/api/v1/search?q=DEMO", nil, &hits, http.StatusOK)
	if len(hits) == 0 {
		t.Fatal("expected search hits")
	}

	for _, path := range []string{
		"/api/v1/reports/inspections.csv",
		"/api/v1/reports/nonconformances.csv",
		"/api/v1/reports/corrective-actions.csv",
		"/api/v1/reports/inspections.pdf",
		"/api/v1/reports/summary.pdf",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+api.token)
		rr := httptest.NewRecorder()
		api.h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
		}
		if rr.Body.Len() == 0 {
			t.Fatalf("%s empty body", path)
		}
	}

	api.call(t, http.MethodPost, "/api/v1/notifications/email", domain.NewEmailNotification{
		Destination: "qc@example.com", Subject: "NCR digest", Body: "Open NCRs need review.", EventType: domain.EventNCRCreated,
	}, &domain.NotificationOutboxItem{}, http.StatusCreated)

	var metrics map[string]any
	api.call(t, http.MethodGet, "/api/v1/metrics", nil, &metrics, http.StatusOK)
	if metrics["requests_total"] == nil {
		t.Fatalf("metrics: %+v", metrics)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw := api.raw(t, http.MethodGet, "/api/v1/request-audit?limit=5", nil, http.StatusOK)
		var wrap struct {
			Data []domain.RequestAuditEntry `json:"data"`
		}
		_ = json.Unmarshal(raw, &wrap)
		if len(wrap.Data) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	var deliveries []domain.WebhookDelivery
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw := api.raw(t, http.MethodGet, "/api/v1/webhook-deliveries?limit=10", nil, http.StatusOK)
		var wrap struct {
			Data []domain.WebhookDelivery `json:"data"`
		}
		_ = json.Unmarshal(raw, &wrap)
		deliveries = wrap.Data
		if len(deliveries) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(deliveries) == 0 {
		t.Fatal("expected webhook delivery after checklist complete")
	}
}

func (c *client) raw(t *testing.T, method, path string, body any, want int) []byte {
	t.Helper()
	var buf io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		buf = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	rr := httptest.NewRecorder()
	c.h.ServeHTTP(rr, req)
	if rr.Code != want {
		t.Fatalf("%s %s: got %d want %d body %s", method, path, rr.Code, want, rr.Body.String())
	}
	return rr.Body.Bytes()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
