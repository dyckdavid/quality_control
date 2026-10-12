package store

import (
	"context"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

// InsertRequestAudit records one HTTP request. Best-effort callers ignore errors.
func (s *Store) InsertRequestAudit(ctx context.Context, actorID *uuid.UUID, method, path string, status, durationMS int, ip, requestID, userAgent string) error {
	if len(method) > 16 {
		method = method[:16]
	}
	if len(path) > 512 {
		path = path[:512]
	}
	if len(ip) > 80 {
		ip = ip[:80]
	}
	if len(requestID) > 80 {
		requestID = requestID[:80]
	}
	if len(userAgent) > 300 {
		userAgent = userAgent[:300]
	}
	if durationMS < 0 {
		durationMS = 0
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO request_audit_log (actor_id, method, path, status_code, duration_ms, ip, request_id, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		actorID, method, path, status, durationMS, ip, requestID, userAgent)
	return mapErr(err)
}

// RequestAuditFilter filters the HTTP access log.
type RequestAuditFilter struct {
	ActorID *uuid.UUID
	Method  string
	Path    string
	From    *time.Time
	To      *time.Time
	Limit   int
	Offset  int
}

// ListRequestAudit returns HTTP access records, newest first.
func (s *Store) ListRequestAudit(ctx context.Context, f RequestAuditFilter) ([]domain.RequestAuditEntry, int, error) {
	f.Method = domain.Trim(f.Method)
	f.Path = domain.Trim(f.Path)
	f.Limit, f.Offset = clamp(f.Limit, f.Offset, 200)
	var c clause
	if f.ActorID != nil {
		c.add("actor_id = ?", *f.ActorID)
	}
	if f.Method != "" {
		c.add("method = ?", f.Method)
	}
	if f.Path != "" {
		c.add("path ILIKE ? ESCAPE '\\'", likeContains(f.Path))
	}
	if f.From != nil {
		c.add("created_at >= ?", *f.From)
	}
	if f.To != nil {
		c.add("created_at < ?", *f.To)
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM request_audit_log`+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_id, method, path, status_code, duration_ms, ip, request_id, user_agent, created_at
		FROM request_audit_log`+c.where()+` ORDER BY created_at DESC, id DESC LIMIT $`+itoa(len(c.args)+1)+` OFFSET $`+itoa(len(c.args)+2), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := []domain.RequestAuditEntry{}
	for rows.Next() {
		var e domain.RequestAuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.Method, &e.Path, &e.StatusCode, &e.DurationMS, &e.IP, &e.RequestID, &e.UserAgent, &e.CreatedAt); err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
