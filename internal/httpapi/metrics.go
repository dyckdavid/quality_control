package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type metricsState struct {
	started   time.Time
	total     atomic.Int64
	status2xx atomic.Int64
	status3xx atomic.Int64
	status4xx atomic.Int64
	status5xx atomic.Int64
}

func newMetricsState() *metricsState {
	return &metricsState{started: time.Now().UTC()}
}

func (m *metricsState) observe(status int) {
	m.total.Add(1)
	switch {
	case status >= 200 && status < 300:
		m.status2xx.Add(1)
	case status >= 300 && status < 400:
		m.status3xx.Add(1)
	case status >= 400 && status < 500:
		m.status4xx.Add(1)
	default:
		m.status5xx.Add(1)
	}
}

func (s *Server) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw, ok := w.(*statusWriter)
		if !ok {
			sw = &statusWriter{ResponseWriter: w, status: http.StatusOK}
			w = sw
		}
		next.ServeHTTP(w, r)
		if s.metrics != nil {
			s.metrics.observe(sw.status)
		}
	})
}

func (s *Server) requestAuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw, ok := w.(*statusWriter)
		if !ok {
			sw = &statusWriter{ResponseWriter: w, status: http.StatusOK}
			w = sw
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		if s.store == nil {
			return
		}
		path := r.URL.Path
		switch path {
		case "/healthz", "/readyz", "/metrics", "/api/v1/openapi.json", "/api/v1/openapi.yaml":
			return
		}
		var actorID *uuid.UUID
		if p, ok := principal(r); ok {
			id := p.ID
			actorID = &id
		} else if s.tokens != nil {
			if token, ok := cutBearer(r.Header.Get("Authorization")); ok {
				if id, _, err := s.tokens.ParseAccess(token, time.Now()); err == nil {
					actorID = &id
				}
			}
		}
		method := r.Method
		status := sw.status
		dur := int(time.Since(start).Milliseconds())
		ip := clientIP(r, s.cfg.TrustProxy)
		rid := requestID(r)
		ua := r.UserAgent()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = s.store.InsertRequestAudit(ctx, actorID, method, path, status, dur, ip, rid, ua)
		}()
	})
}

func (s *Server) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Metrics are not configured.")
		return
	}
	m := s.metrics
	uptime := time.Since(m.started).Seconds()
	body := fmt.Sprintf(`# HELP qc_http_requests_total Total HTTP requests handled.
# TYPE qc_http_requests_total counter
qc_http_requests_total %d
# HELP qc_http_requests_by_class_total HTTP requests by status class.
# TYPE qc_http_requests_by_class_total counter
qc_http_requests_by_class_total{class="2xx"} %d
qc_http_requests_by_class_total{class="3xx"} %d
qc_http_requests_by_class_total{class="4xx"} %d
qc_http_requests_by_class_total{class="5xx"} %d
# HELP qc_process_uptime_seconds Process uptime in seconds.
# TYPE qc_process_uptime_seconds gauge
qc_process_uptime_seconds %s
`, m.total.Load(), m.status2xx.Load(), m.status3xx.Load(), m.status4xx.Load(), m.status5xx.Load(), strconv.FormatFloat(uptime, 'f', 3, 64))
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (s *Server) jsonMetrics(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Metrics are not configured.")
		return
	}
	m := s.metrics
	writeData(w, http.StatusOK, map[string]any{
		"requests_total": m.total.Load(),
		"requests_by_status": map[string]int64{
			"2xx": m.status2xx.Load(),
			"3xx": m.status3xx.Load(),
			"4xx": m.status4xx.Load(),
			"5xx": m.status5xx.Load(),
		},
		"uptime_seconds": int64(time.Since(m.started).Seconds()),
	})
}
