// Package store is the PostgreSQL persistence layer.
// Business checks that protect the quality record live here, in the same
// transaction as the write they guard.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store talks to PostgreSQL through a connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a store for the pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type scannable interface {
	Scan(dest ...any) error
}

func (s *Store) tx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback(context.Background())
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return mapErr(tx.Commit(ctx))
}

type clause struct {
	conds []string
	args  []any
}

func (c *clause) add(expr string, args ...any) {
	for _, a := range args {
		c.args = append(c.args, a)
		expr = strings.Replace(expr, "?", fmt.Sprintf("$%d", len(c.args)), 1)
	}
	c.conds = append(c.conds, expr)
}

func (c *clause) where() string {
	if len(c.conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(c.conds, " AND ")
}

type setter struct {
	sets []string
	args []any
}

func (s *setter) add(column string, arg any) {
	s.args = append(s.args, arg)
	s.sets = append(s.sets, fmt.Sprintf("%s = $%d", column, len(s.args)))
}

func (s *setter) empty() bool { return len(s.sets) == 0 }

func likeContains(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return "%" + s + "%"
}

func cleanQuery(q string) (string, error) {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) > 100 {
		return "", invalid("search text is too long")
	}
	return q, nil
}

func clamp(limit, offset, max int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > max {
		limit = max
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func dateArg(s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, invalid("date must be YYYY-MM-DD")
	}
	return t, nil
}

func uuidOrNil(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func collect[T any](rows pgx.Rows, scan func(scannable) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) writeAudit(ctx context.Context, q querier, actor domain.Actor, action, entityType string, entityID uuid.UUID, summary string, before, after any) error {
	var beforeJSON, afterJSON []byte
	var err error
	if before != nil {
		beforeJSON, err = json.Marshal(before)
		if err != nil {
			return err
		}
	}
	if after != nil {
		afterJSON, err = json.Marshal(after)
		if err != nil {
			return err
		}
	}
	var actorID any
	if actor.ID != uuid.Nil {
		actorID = actor.ID
	}
	var ent any
	if entityID != uuid.Nil {
		ent = entityID
	}
	_, err = q.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, entity_type, entity_id, summary, before_data, after_data, ip, request_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		actorID, action, entityType, ent, summary, beforeJSON, afterJSON, actor.IP, actor.RequestID)
	return mapErr(err)
}

func nextNumber(ctx context.Context, q querier, docType string) (string, error) {
	year := time.Now().UTC().Year()
	var storedYear, n int
	err := q.QueryRow(ctx, `
		INSERT INTO document_counters (doc_type, year, last_value)
		VALUES ($1, $2, 1)
		ON CONFLICT (doc_type) DO UPDATE SET
			year = EXCLUDED.year,
			last_value = CASE
				WHEN document_counters.year = EXCLUDED.year THEN document_counters.last_value + 1
				ELSE 1
			END
		RETURNING year, last_value`, docType, year).Scan(&storedYear, &n)
	if err != nil {
		return "", mapErr(err)
	}
	return fmt.Sprintf("%s-%04d-%06d", docType, storedYear, n), nil
}

func (s *Store) requireActiveRow(ctx context.Context, q querier, table string, id uuid.UUID, label string) error {
	var active bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s WHERE id = $1`, table), id).Scan(&active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid(label + " does not exist")
		}
		return mapErr(err)
	}
	if !active {
		return invalid(label + " is not active")
	}
	return nil
}

func (s *Store) requireUser(ctx context.Context, q querier, id uuid.UUID, minRole string) error {
	var role string
	var active bool
	err := q.QueryRow(ctx, `SELECT role, active FROM users WHERE id = $1`, id).Scan(&role, &active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid("user does not exist")
		}
		return mapErr(err)
	}
	if !active {
		return invalid("user is not active")
	}
	if minRole != "" && !domain.AtLeast(role, minRole) {
		return invalid("user does not have the required role")
	}
	return nil
}

// EntityExists reports whether an attachment parent row is present.
func (s *Store) EntityExists(ctx context.Context, entityType string, id uuid.UUID) (bool, error) {
	table, ok := attachmentTable(entityType)
	if !ok {
		return false, invalid("entity type is not valid")
	}
	var exists bool
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE id = $1)`, table), id).Scan(&exists)
	return exists, mapErr(err)
}

func attachmentTable(entityType string) (string, bool) {
	switch entityType {
	case domain.AttachLot:
		return "lots", true
	case domain.AttachInspection:
		return "inspections", true
	case domain.AttachNonconformance:
		return "nonconformances", true
	case domain.AttachCAPA:
		return "corrective_actions", true
	default:
		return "", false
	}
}
