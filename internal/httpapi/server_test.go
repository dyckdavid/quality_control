package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quality_control/internal/auth"
	"quality_control/internal/config"
)

func TestHealthAndUnauthorized(t *testing.T) {
	s := New(config.Config{}, nil, auth.NewManager("0123456789abcdef0123456789abcdef", time.Minute, time.Hour))
	h := s.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("health: %d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("users: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready: %d", rr.Code)
	}
}
