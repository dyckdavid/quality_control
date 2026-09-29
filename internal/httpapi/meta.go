package httpapi

import (
	"net/http"
	"time"

	"quality_control/internal/buildinfo"
	"quality_control/internal/domain"
	"quality_control/internal/store"

	"github.com/google/uuid"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Database is not configured.")
		return
	}
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Database is not ready.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": "quality-control", "version": buildinfo.Version})
}

func (s *Server) enums(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, domain.Enums())
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.Dashboard(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, d)
}

func (s *Server) measurements(w http.ResponseWriter, r *http.Request) {
	product, ok := queryUUID(w, r, "product_id")
	if !ok {
		return
	}
	from, ok := queryTime(w, r, "from")
	if !ok {
		return
	}
	to, ok := queryTime(w, r, "to")
	if !ok {
		return
	}
	limit, offset, ok := queryPageMax(w, r, 2000, 500)
	if !ok {
		return
	}
	rows, total, err := s.store.Measurements(r.Context(), storeMeasurement(product, r.URL.Query().Get("characteristic_code"), from, to, limit, offset))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	id, ok := queryUUID(w, r, "entity_id")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListAudit(r.Context(), storeAudit(r.URL.Query().Get("entity_type"), id, limit, offset))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	user, err := s.store.GetUser(r.Context(), p.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, user)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
	User         domain.User `json:"user"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if !decode(w, r, &in) {
		return
	}
	ip := clientIP(r, s.cfg.TrustProxy)
	if !s.limiter.allow(ip, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many sign-in attempts. Wait and try again.")
		return
	}
	user, err := s.store.Authenticate(r.Context(), in.Username, in.Password, domain.Actor{IP: ip, RequestID: requestID(r)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.limiter.reset(ip)
	s.writeSession(w, r, user)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var in refreshRequest
	if !decode(w, r, &in) {
		return
	}
	if in.RefreshToken == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "refresh_token is required.")
		return
	}
	user, secret, err := s.store.RotateRefresh(r.Context(), in.RefreshToken, s.tokens.RefreshTTL())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	access, err := s.tokens.SignAccess(user.ID, user.Username, time.Now())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, tokenResponse{
		AccessToken: access, RefreshToken: secret, TokenType: "Bearer",
		ExpiresIn: int(s.tokens.AccessTTL().Seconds()), User: user,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var in refreshRequest
	if !decode(w, r, &in) {
		return
	}
	if in.RefreshToken == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "refresh_token is required.")
		return
	}
	actor := domain.Actor{IP: clientIP(r, s.cfg.TrustProxy), RequestID: requestID(r)}
	if header := r.Header.Get("Authorization"); header != "" {
		if p, ok := s.optionalUser(r); ok {
			actor = p.Actor()
		}
	}
	if err := s.store.RevokeRefresh(r.Context(), actor, in.RefreshToken); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var in domain.PasswordChange
	if !decode(w, r, &in) {
		return
	}
	p, _ := principal(r)
	if err := s.store.ChangePassword(r.Context(), p.Actor(), p.ID, in.CurrentPassword, in.NewPassword); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeSession(w http.ResponseWriter, r *http.Request, user domain.User) {
	secret, err := s.store.IssueRefresh(r.Context(), user.ID, s.tokens.RefreshTTL())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	access, err := s.tokens.SignAccess(user.ID, user.Username, time.Now())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, tokenResponse{
		AccessToken: access, RefreshToken: secret, TokenType: "Bearer",
		ExpiresIn: int(s.tokens.AccessTTL().Seconds()), User: user,
	})
}

func (s *Server) optionalUser(r *http.Request) (domain.Principal, bool) {
	header := r.Header.Get("Authorization")
	token, ok := cutBearer(header)
	if !ok {
		return domain.Principal{}, false
	}
	id, _, err := s.tokens.ParseAccess(token, time.Now())
	if err != nil {
		return domain.Principal{}, false
	}
	user, err := s.store.GetUser(r.Context(), id)
	if err != nil || !user.Active {
		return domain.Principal{}, false
	}
	return domain.Principal{ID: user.ID, Username: user.Username, Role: user.Role, IP: clientIP(r, s.cfg.TrustProxy), RequestID: requestID(r)}, true
}

func cutBearer(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return "", false
	}
	return header[len(prefix):], true
}

func storeMeasurement(product *uuid.UUID, code string, from, to *time.Time, limit, offset int) store.MeasurementFilter {
	return store.MeasurementFilter{ProductID: product, Code: code, From: from, To: to, Limit: limit, Offset: offset}
}

func storeAudit(entityType string, id *uuid.UUID, limit, offset int) store.AuditFilter {
	return store.AuditFilter{EntityType: entityType, EntityID: id, Limit: limit, Offset: offset}
}
