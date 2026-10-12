// Package httpapi is the JSON API for the quality-control server.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"quality_control/internal/auth"
	"quality_control/internal/config"
	"quality_control/internal/domain"
	"quality_control/internal/store"
)

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxRequest
)

// Server is the HTTP API.
type Server struct {
	cfg     config.Config
	store   *store.Store
	tokens  *auth.Manager
	limiter *loginLimiter
	metrics *metricsState
}

// New builds the API. store is used for every request except health and version.
func New(cfg config.Config, st *store.Store, tokens *auth.Manager) *Server {
	return &Server{cfg: cfg, store: st, tokens: tokens, limiter: newLoginLimiter(), metrics: newMetricsState()}
}

// Handler is the full middleware stack and router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routes(mux)
	// requestAudit and metrics only observe; they do not alter responses.
	return s.recover(s.requestID(s.secure(s.log(s.cors(s.metricsMiddleware(s.requestAuditMiddleware(mux)))))))
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.prometheusMetrics)
	mux.HandleFunc("GET /api/v1/version", s.version)
	mux.HandleFunc("GET /api/v1/openapi.json", s.openapiJSON)
	mux.HandleFunc("GET /api/v1/openapi.yaml", s.openapiYAML)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)

	read := domain.RoleViewer
	operator := domain.RoleOperator
	inspector := domain.RoleInspector
	supervisor := domain.RoleSupervisor
	manager := domain.RoleQualityManager
	admin := domain.RoleAdmin

	s.handle(mux, "GET /api/v1/me", read, s.me)
	s.handle(mux, "POST /api/v1/me/password", read, s.changePassword)
	s.handle(mux, "GET /api/v1/enums", read, s.enums)
	s.handle(mux, "GET /api/v1/dashboard", read, s.dashboard)
	s.handle(mux, "GET /api/v1/measurements", read, s.measurements)
	s.handle(mux, "GET /api/v1/audit", supervisor, s.audit)
	s.handle(mux, "GET /api/v1/metrics", read, s.jsonMetrics)
	s.handle(mux, "GET /api/v1/search", read, s.search)
	s.handle(mux, "GET /api/v1/request-audit", supervisor, s.listRequestAudit)

	s.handle(mux, "GET /api/v1/stats/defect-rates", read, s.statsDefectRates)
	s.handle(mux, "GET /api/v1/stats/ncr-aging", read, s.statsNCRAging)
	s.handle(mux, "GET /api/v1/stats/capa-aging", read, s.statsCAPAAging)
	s.handle(mux, "GET /api/v1/stats/trends", read, s.statsTrends)

	s.handle(mux, "GET /api/v1/reports/inspections.csv", read, s.reportInspectionsCSV)
	s.handle(mux, "GET /api/v1/reports/nonconformances.csv", read, s.reportNCRSCSV)
	s.handle(mux, "GET /api/v1/reports/corrective-actions.csv", read, s.reportCAPAsCSV)
	s.handle(mux, "GET /api/v1/reports/inspections.pdf", read, s.reportInspectionsPDF)
	s.handle(mux, "GET /api/v1/reports/summary.pdf", read, s.reportSummaryPDF)

	s.handle(mux, "GET /api/v1/checklists", read, s.listChecklists)
	s.handle(mux, "POST /api/v1/checklists", manager, s.createChecklist)
	s.handle(mux, "GET /api/v1/checklists/{id}", read, s.getChecklist)
	s.handle(mux, "PATCH /api/v1/checklists/{id}", manager, s.updateChecklist)
	s.handle(mux, "POST /api/v1/checklists/{id}/items", manager, s.addChecklistItem)
	s.handle(mux, "PATCH /api/v1/checklist-items/{id}", manager, s.updateChecklistItem)
	s.handle(mux, "DELETE /api/v1/checklist-items/{id}", manager, s.deleteChecklistItem)
	s.handle(mux, "GET /api/v1/inspections/{id}/checklists", read, s.listInspectionChecklistRuns)
	s.handle(mux, "POST /api/v1/inspections/{id}/checklists", inspector, s.startInspectionChecklist)
	s.handle(mux, "GET /api/v1/checklist-runs/{id}", read, s.getChecklistRun)
	s.handle(mux, "POST /api/v1/checklist-runs/{id}/items/{itemID}", inspector, s.answerChecklistRunItem)
	s.handle(mux, "POST /api/v1/checklist-runs/{id}/complete", inspector, s.completeChecklistRun)

	s.handle(mux, "GET /api/v1/webhooks/events", supervisor, s.listWebhookEvents)
	s.handle(mux, "GET /api/v1/webhooks", supervisor, s.listWebhooks)
	s.handle(mux, "POST /api/v1/webhooks", supervisor, s.createWebhook)
	s.handle(mux, "GET /api/v1/webhooks/{id}", supervisor, s.getWebhook)
	s.handle(mux, "PATCH /api/v1/webhooks/{id}", supervisor, s.updateWebhook)
	s.handle(mux, "GET /api/v1/webhook-deliveries", supervisor, s.listWebhookDeliveries)
	s.handle(mux, "GET /api/v1/notifications", supervisor, s.listNotifications)
	s.handle(mux, "POST /api/v1/notifications/email", supervisor, s.enqueueEmail)

	s.handle(mux, "POST /api/v1/admin/seed-demo", admin, s.seedDemo)

	s.handle(mux, "GET /api/v1/users", admin, s.listUsers)
	s.handle(mux, "POST /api/v1/users", admin, s.createUser)
	s.handle(mux, "GET /api/v1/users/{id}", admin, s.getUser)
	s.handle(mux, "PATCH /api/v1/users/{id}", admin, s.updateUser)
	s.handle(mux, "POST /api/v1/users/{id}/password", admin, s.resetPassword)

	s.handle(mux, "GET /api/v1/sites", read, s.listSites)
	s.handle(mux, "POST /api/v1/sites", supervisor, s.createSite)
	s.handle(mux, "GET /api/v1/sites/{id}", read, s.getSite)
	s.handle(mux, "PATCH /api/v1/sites/{id}", supervisor, s.updateSite)

	s.handle(mux, "GET /api/v1/stations", read, s.listStations)
	s.handle(mux, "POST /api/v1/stations", supervisor, s.createStation)
	s.handle(mux, "GET /api/v1/stations/{id}", read, s.getStation)
	s.handle(mux, "PATCH /api/v1/stations/{id}", supervisor, s.updateStation)

	s.handle(mux, "GET /api/v1/suppliers", read, s.listSuppliers)
	s.handle(mux, "POST /api/v1/suppliers", manager, s.createSupplier)
	s.handle(mux, "GET /api/v1/suppliers/{id}", read, s.getSupplier)
	s.handle(mux, "PATCH /api/v1/suppliers/{id}", manager, s.updateSupplier)

	s.handle(mux, "GET /api/v1/gauges", read, s.listGauges)
	s.handle(mux, "POST /api/v1/gauges", manager, s.createGauge)
	s.handle(mux, "GET /api/v1/gauges/{id}", read, s.getGauge)
	s.handle(mux, "PATCH /api/v1/gauges/{id}", manager, s.updateGauge)

	s.handle(mux, "GET /api/v1/products", read, s.listProducts)
	s.handle(mux, "POST /api/v1/products", manager, s.createProduct)
	s.handle(mux, "GET /api/v1/products/{id}", read, s.getProduct)
	s.handle(mux, "PATCH /api/v1/products/{id}", manager, s.updateProduct)

	s.handle(mux, "GET /api/v1/defect-codes", read, s.listDefects)
	s.handle(mux, "POST /api/v1/defect-codes", manager, s.createDefect)
	s.handle(mux, "GET /api/v1/defect-codes/{id}", read, s.getDefect)
	s.handle(mux, "PATCH /api/v1/defect-codes/{id}", manager, s.updateDefect)

	s.handle(mux, "GET /api/v1/plans", read, s.listPlans)
	s.handle(mux, "POST /api/v1/plans", manager, s.createPlan)
	s.handle(mux, "GET /api/v1/plans/{id}", read, s.getPlan)
	s.handle(mux, "PATCH /api/v1/plans/{id}", manager, s.updatePlan)
	s.handle(mux, "DELETE /api/v1/plans/{id}", manager, s.deletePlan)
	s.handle(mux, "POST /api/v1/plans/{id}/activate", manager, s.activatePlan)
	s.handle(mux, "POST /api/v1/plans/{id}/obsolete", manager, s.obsoletePlan)
	s.handle(mux, "POST /api/v1/plans/{id}/revise", manager, s.revisePlan)
	s.handle(mux, "POST /api/v1/plans/{id}/characteristics", manager, s.addCharacteristic)
	s.handle(mux, "PATCH /api/v1/characteristics/{id}", manager, s.updateCharacteristic)
	s.handle(mux, "DELETE /api/v1/characteristics/{id}", manager, s.deleteCharacteristic)

	s.handle(mux, "GET /api/v1/lots", read, s.listLots)
	s.handle(mux, "POST /api/v1/lots", operator, s.createLot)
	s.handle(mux, "GET /api/v1/lots/{id}", read, s.getLot)
	s.handle(mux, "PATCH /api/v1/lots/{id}", operator, s.updateLot)
	s.handle(mux, "POST /api/v1/lots/{id}/status", supervisor, s.setLotStatus)

	s.handle(mux, "GET /api/v1/inspections", read, s.listInspections)
	s.handle(mux, "POST /api/v1/inspections", inspector, s.createInspection)
	s.handle(mux, "GET /api/v1/inspections/{id}", read, s.getInspection)
	s.handle(mux, "POST /api/v1/inspections/{id}/results", operator, s.recordResults)
	s.handle(mux, "POST /api/v1/inspections/{id}/defects", inspector, s.addDefect)
	s.handle(mux, "POST /api/v1/inspections/{id}/complete", inspector, s.completeInspection)
	s.handle(mux, "POST /api/v1/inspections/{id}/cancel", inspector, s.cancelInspection)
	s.handle(mux, "DELETE /api/v1/inspection-results/{id}", operator, s.deleteResult)
	s.handle(mux, "DELETE /api/v1/inspection-defects/{id}", inspector, s.deleteDefect)

	s.handle(mux, "GET /api/v1/nonconformances", read, s.listNCRs)
	s.handle(mux, "POST /api/v1/nonconformances", inspector, s.createNCR)
	s.handle(mux, "GET /api/v1/nonconformances/{id}", read, s.getNCR)
	s.handle(mux, "PATCH /api/v1/nonconformances/{id}", inspector, s.updateNCR)
	s.handle(mux, "POST /api/v1/nonconformances/{id}/disposition", supervisor, s.disposeNCR)
	s.handle(mux, "POST /api/v1/nonconformances/{id}/close", supervisor, s.closeNCR)
	s.handle(mux, "POST /api/v1/nonconformances/{id}/void", supervisor, s.voidNCR)

	s.handle(mux, "GET /api/v1/corrective-actions", read, s.listCAPAs)
	s.handle(mux, "POST /api/v1/corrective-actions", supervisor, s.createCAPA)
	s.handle(mux, "GET /api/v1/corrective-actions/{id}", read, s.getCAPA)
	s.handle(mux, "PATCH /api/v1/corrective-actions/{id}", supervisor, s.updateCAPA)
	s.handle(mux, "POST /api/v1/corrective-actions/{id}/start", supervisor, s.startCAPA)
	s.handle(mux, "POST /api/v1/corrective-actions/{id}/verify", supervisor, s.verifyCAPA)
	s.handle(mux, "POST /api/v1/corrective-actions/{id}/close", manager, s.closeCAPA)
	s.handle(mux, "POST /api/v1/corrective-actions/{id}/reopen", supervisor, s.reopenCAPA)
	s.handle(mux, "POST /api/v1/corrective-actions/{id}/cancel", supervisor, s.cancelCAPA)

	s.handle(mux, "GET /api/v1/attachments", read, s.listAttachments)
	s.handle(mux, "POST /api/v1/attachments", operator, s.uploadAttachment)
	s.handle(mux, "GET /api/v1/attachments/{id}", read, s.downloadAttachment)
	s.handle(mux, "DELETE /api/v1/attachments/{id}", supervisor, s.deleteAttachment)
}

func (s *Server) handle(mux *http.ServeMux, pattern, minRole string, h http.HandlerFunc) {
	mux.Handle(pattern, s.authenticate(s.requireRank(minRole, h)))
}

func (s *Server) actor(r *http.Request) domain.Actor {
	p, _ := principal(r)
	return p.Actor()
}

func principal(r *http.Request) (domain.Principal, bool) {
	p, ok := r.Context().Value(ctxUser).(domain.Principal)
	return p, ok
}

func requestID(r *http.Request) string {
	if id, ok := r.Context().Value(ctxRequest).(string); ok {
		return id
	}
	return ""
}

func clientIP(r *http.Request, trust bool) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if trust && isLoopback(host) {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			first := strings.TrimSpace(strings.Split(fwd, ",")[0])
			if first != "" {
				return first
			}
		}
	}
	return host
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Sign in is required.")
			return
		}
		id, _, err := s.tokens.ParseAccess(strings.TrimSpace(token), time.Now())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Sign in is required.")
			return
		}
		if s.store == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Sign in is required.")
			return
		}
		user, err := s.store.GetUser(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "unauthorized", "Sign in is required.")
				return
			}
			s.fail(w, r, err)
			return
		}
		if !user.Active {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Sign in is required.")
			return
		}
		p := domain.Principal{
			ID: user.ID, Username: user.Username, Role: user.Role,
			IP: clientIP(r, s.cfg.TrustProxy), RequestID: requestID(r),
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxUser, p)))
	})
}

func (s *Server) requireRank(min string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principal(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Sign in is required.")
			return
		}
		if !domain.AtLeast(p.Role, min) {
			writeError(w, http.StatusForbidden, "forbidden", "You do not have access to that action.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	msg := store.Message(err)
	switch {
	case errors.Is(err, store.ErrNotFound):
		if msg == "" {
			msg = "Not found."
		}
		writeError(w, http.StatusNotFound, "not_found", msg)
	case errors.Is(err, store.ErrConflict):
		if msg == "" {
			msg = "That record already exists."
		}
		writeError(w, http.StatusConflict, "conflict", msg)
	case errors.Is(err, store.ErrInvalid):
		if msg == "" {
			msg = "The request is not valid."
		}
		writeError(w, http.StatusUnprocessableEntity, "invalid", msg)
	case errors.Is(err, store.ErrUnauthorized):
		if msg == "" {
			msg = "Sign in is required."
		}
		writeError(w, http.StatusUnauthorized, "unauthorized", msg)
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to that action.")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusServiceUnavailable, "unavailable", "The database took too long.")
	default:
		slog.Error("request failed", "err", err, "request_id", requestID(r), "path", r.URL.Path)
		writeError(w, http.StatusInternalServerError, "internal", "Something went wrong.")
	}
}
