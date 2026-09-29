package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
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

func TestInspectionFlow(t *testing.T) {
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
	if session.User.Role != domain.RoleAdmin || session.RefreshToken == "" {
		t.Fatalf("session: %+v", session)
	}

	api.call(t, http.MethodPost, "/api/v1/users", domain.NewUser{
		Username: "viewer1", Email: "viewer@localhost", Password: "viewer-password", FullName: "View Only", Role: domain.RoleViewer,
	}, &domain.User{}, http.StatusCreated)
	viewer := &client{h: api.h}
	var viewerSession tokenResponse
	viewer.call(t, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "viewer1", "password": "viewer-password"}, &viewerSession, http.StatusOK)
	viewer.token = viewerSession.AccessToken
	viewer.call(t, http.MethodPost, "/api/v1/products", domain.NewProduct{PartNumber: "NOPE", Name: "Nope"}, nil, http.StatusForbidden)

	var site domain.Site
	api.call(t, http.MethodPost, "/api/v1/sites", domain.NewSite{Code: "MAIN", Name: "Main plant"}, &site, http.StatusCreated)
	var station domain.Station
	api.call(t, http.MethodPost, "/api/v1/stations", domain.NewStation{SiteID: site.ID, Code: "IN-1", Name: "Incoming bench"}, &station, http.StatusCreated)
	var supplier domain.Supplier
	api.call(t, http.MethodPost, "/api/v1/suppliers", domain.NewSupplier{Code: "ACME", Name: "Acme Steel"}, &supplier, http.StatusCreated)
	var product domain.Product
	api.call(t, http.MethodPost, "/api/v1/products", domain.NewProduct{PartNumber: "BRK-100", Name: "Bracket", Unit: "ea"}, &product, http.StatusCreated)
	api.call(t, http.MethodPost, "/api/v1/products", domain.NewProduct{PartNumber: "BRK-100", Name: "Duplicate"}, nil, http.StatusConflict)
	due := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	var gauge domain.Gauge
	api.call(t, http.MethodPost, "/api/v1/gauges", domain.NewGauge{Code: "CAL-1", Name: "Caliper", CalibrationDue: due}, &gauge, http.StatusCreated)
	var defect domain.DefectCode
	api.call(t, http.MethodPost, "/api/v1/defect-codes", domain.NewDefectCode{Code: "VIS-SCR", Name: "Scratch", Severity: "minor", Category: "visual"}, &defect, http.StatusCreated)

	var plan domain.PlanDetail
	api.call(t, http.MethodPost, "/api/v1/plans", domain.NewPlan{
		ProductID: product.ID, Revision: "A", Name: "Incoming", SamplingType: domain.SampleFixed, SampleSize: intPtr(1),
		Characteristics: []domain.NewCharacteristic{
			{Sequence: 1, Code: "LEN", Name: "Length", CharacteristicType: domain.CharVariable, Nominal: floatPtr(10), LowerLimit: floatPtr(9.9), UpperLimit: floatPtr(10.1), Unit: "mm", Critical: true},
			{Sequence: 2, Code: "VIS", Name: "Visual", CharacteristicType: domain.CharAttribute},
		},
	}, &plan, http.StatusCreated)
	if len(plan.Characteristics) != 2 {
		t.Fatalf("characteristics: %+v", plan.Characteristics)
	}
	api.call(t, http.MethodPost, "/api/v1/plans/"+plan.ID.String()+"/activate", map[string]any{}, &plan, http.StatusOK)
	if plan.Status != domain.PlanActive {
		t.Fatalf("plan status %s", plan.Status)
	}

	var lot domain.Lot
	api.call(t, http.MethodPost, "/api/v1/lots", domain.NewLot{
		LotNumber: "LOT-1", ProductID: product.ID, SiteID: site.ID, SupplierID: &supplier.ID, Quantity: 20, Source: domain.SourceIncoming,
	}, &lot, http.StatusCreated)
	var insp domain.InspectionDetail
	api.call(t, http.MethodPost, "/api/v1/inspections", domain.NewInspection{LotID: lot.ID, StationID: &station.ID, SampleSize: intPtr(1)}, &insp, http.StatusCreated)
	if insp.InspectionNumber == "" || !strings.HasPrefix(insp.InspectionNumber, "INS-") {
		t.Fatalf("number %s", insp.InspectionNumber)
	}
	api.call(t, http.MethodPost, "/api/v1/inspections/"+insp.ID.String()+"/complete", map[string]any{}, nil, http.StatusUnprocessableEntity)

	lenID, visID := characteristicIDs(t, plan.Characteristics)
	var recorded []domain.InspectionResult
	api.call(t, http.MethodPost, "/api/v1/inspections/"+insp.ID.String()+"/results", domain.ResultBatch{Results: []domain.ResultInput{
		{CharacteristicID: lenID, MeasuredValue: floatPtr(10)},
		{CharacteristicID: visID, AttributeResult: strPtr("pass"), GaugeID: &gauge.ID},
	}}, &recorded, http.StatusOK)
	if len(recorded) != 2 || !recorded[0].InSpec || !recorded[1].CalibrationWarning {
		t.Fatalf("results: %+v", recorded)
	}
	var done domain.CompletedInspection
	api.call(t, http.MethodPost, "/api/v1/inspections/"+insp.ID.String()+"/complete", domain.CompleteInspection{
		QuantityInspected: floatPtr(20), QuantityAccepted: floatPtr(20), QuantityRejected: floatPtr(0),
	}, &done, http.StatusOK)
	if done.Inspection.Result != domain.ResultPass || done.Nonconformance != nil {
		t.Fatalf("complete: %+v", done)
	}
	api.call(t, http.MethodGet, "/api/v1/lots/"+lot.ID.String(), nil, &lot, http.StatusOK)
	if lot.Status != domain.LotAccepted {
		t.Fatalf("lot status %s", lot.Status)
	}

	var lot2 domain.Lot
	api.call(t, http.MethodPost, "/api/v1/lots", domain.NewLot{
		LotNumber: "LOT-2", ProductID: product.ID, SiteID: site.ID, Quantity: 8, Source: domain.SourceIncoming,
	}, &lot2, http.StatusCreated)
	var insp2 domain.InspectionDetail
	api.call(t, http.MethodPost, "/api/v1/inspections", domain.NewInspection{LotID: lot2.ID}, &insp2, http.StatusCreated)
	api.call(t, http.MethodPost, "/api/v1/inspections/"+insp2.ID.String()+"/results", domain.ResultBatch{Results: []domain.ResultInput{
		{CharacteristicID: lenID, MeasuredValue: floatPtr(9)},
		{CharacteristicID: visID, AttributeResult: strPtr("pass")},
	}}, &recorded, http.StatusOK)
	api.call(t, http.MethodPost, "/api/v1/inspections/"+insp2.ID.String()+"/defects", domain.NewInspectionDefect{DefectCodeID: defect.ID, Quantity: 2}, &domain.InspectionDefect{}, http.StatusCreated)
	var failed domain.CompletedInspection
	api.call(t, http.MethodPost, "/api/v1/inspections/"+insp2.ID.String()+"/complete", map[string]any{}, &failed, http.StatusOK)
	if failed.Inspection.Result != domain.ResultFail || failed.Nonconformance == nil {
		t.Fatalf("failed complete: %+v", failed)
	}
	api.call(t, http.MethodGet, "/api/v1/lots/"+lot2.ID.String(), nil, &lot2, http.StatusOK)
	if lot2.Status != domain.LotRejected {
		t.Fatalf("rejected lot status %s", lot2.Status)
	}

	var dash domain.Dashboard
	api.call(t, http.MethodGet, "/api/v1/dashboard", nil, &dash, http.StatusOK)
	if dash.FailLast30Days != 1 || dash.PassLast30Days != 1 || dash.OpenNonconformances != 1 {
		t.Fatalf("dashboard: %+v", dash)
	}
	var measurements []domain.Measurement
	api.call(t, http.MethodGet, "/api/v1/measurements?product_id="+product.ID.String(), nil, &measurements, http.StatusOK)
	if len(measurements) != 2 {
		t.Fatalf("measurements: %+v", measurements)
	}

	var ncr domain.Nonconformance
	api.call(t, http.MethodPost, "/api/v1/nonconformances/"+failed.Nonconformance.ID.String()+"/disposition", domain.DispositionRequest{Disposition: domain.DispScrap, Notes: "Scrap the short pieces"}, &ncr, http.StatusOK)
	if ncr.Status != domain.NCRDispositioned {
		t.Fatalf("ncr %s", ncr.Status)
	}
	api.call(t, http.MethodPost, "/api/v1/nonconformances/"+ncr.ID.String()+"/close", map[string]any{}, &ncr, http.StatusOK)

	var capa domain.CorrectiveAction
	api.call(t, http.MethodPost, "/api/v1/corrective-actions", domain.NewCorrectiveAction{
		NonconformanceID: &ncr.ID, Title: "Calibrate incoming gauge", ActionPlan: "Check the fixture", DueDate: time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02"),
	}, &capa, http.StatusCreated)
	api.call(t, http.MethodPost, "/api/v1/corrective-actions/"+capa.ID.String()+"/start", map[string]any{}, &capa, http.StatusOK)
	api.call(t, http.MethodPost, "/api/v1/corrective-actions/"+capa.ID.String()+"/verify", domain.NoteBody{Notes: "Fixture was worn"}, &capa, http.StatusOK)
	api.call(t, http.MethodPost, "/api/v1/corrective-actions/"+capa.ID.String()+"/close", domain.NoteBody{Notes: "Next lot passed"}, &capa, http.StatusOK)
	if capa.Status != domain.CAPAClosed {
		t.Fatalf("capa %s", capa.Status)
	}

	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	if err := form.WriteField("entity_type", domain.AttachInspection); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("entity_id", insp.ID.String()); err != nil {
		t.Fatal(err)
	}
	part, err := form.CreateFormFile("file", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "heat lot 42\n"); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+api.token)
	rr := httptest.NewRecorder()
	api.h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rr.Code, rr.Body.String())
	}
	var uploaded domain.Attachment
	decodeData(t, rr.Body.Bytes(), &uploaded)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/attachments/"+uploaded.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+api.token)
	rr = httptest.NewRecorder()
	api.h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "heat lot 42\n" {
		t.Fatalf("download: %d %q", rr.Code, rr.Body.String())
	}

	var revised domain.PlanDetail
	api.call(t, http.MethodPost, "/api/v1/plans/"+plan.ID.String()+"/revise", domain.RevisePlan{Revision: "B"}, &revised, http.StatusCreated)
	api.call(t, http.MethodPost, "/api/v1/plans/"+revised.ID.String()+"/activate", map[string]any{}, &revised, http.StatusOK)
	var original domain.PlanDetail
	api.call(t, http.MethodGet, "/api/v1/plans/"+plan.ID.String(), nil, &original, http.StatusOK)
	if original.Status != domain.PlanObsolete || revised.Status != domain.PlanActive || len(revised.Characteristics) != 2 {
		t.Fatalf("revise original=%s revised=%s chars=%d", original.Status, revised.Status, len(revised.Characteristics))
	}

	var audits []domain.AuditEntry
	api.call(t, http.MethodGet, "/api/v1/audit?limit=5", nil, &audits, http.StatusOK)
	if len(audits) == 0 {
		t.Fatal("expected audit rows")
	}
}

type client struct {
	h     http.Handler
	token string
}

func (c *client) call(t *testing.T, method, path string, body any, dest any, want int) {
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
	if dest != nil {
		decodeData(t, rr.Body.Bytes(), dest)
	}
}

func decodeData(t *testing.T, raw []byte, dest any) {
	t.Helper()
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		t.Fatalf("envelope: %v body %s", err, raw)
	}
	if err := json.Unmarshal(wrap.Data, dest); err != nil {
		t.Fatalf("data: %v body %s", err, raw)
	}
}

func characteristicIDs(t *testing.T, chars []domain.Characteristic) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var length, visual uuid.UUID
	for _, ch := range chars {
		switch ch.Code {
		case "LEN":
			length = ch.ID
		case "VIS":
			visual = ch.ID
		}
	}
	if length == uuid.Nil || visual == uuid.Nil {
		t.Fatalf("missing characteristics: %+v", chars)
	}
	return length, visual
}

func intPtr(n int) *int           { return &n }
func floatPtr(n float64) *float64 { return &n }
func strPtr(s string) *string     { return &s }
