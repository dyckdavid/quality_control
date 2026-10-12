package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"quality_control/internal/auth"
	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const userCols = `id, username, email, full_name, role, active, last_login_at, created_at, updated_at`

func scanUser(row scannable) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.FullName, &u.Role, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// GetUser loads an account by id.
func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return u, nil
}

// ListUsers returns accounts matching the optional search, role, and active flag.
func (s *Store) ListUsers(ctx context.Context, query, role string, active *bool, limit, offset int) ([]domain.User, int, error) {
	query, err := cleanQuery(query)
	if err != nil {
		return nil, 0, err
	}
	if role != "" {
		if err := domain.CheckRole(role); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	limit, offset = clamp(limit, offset, 200)
	var c clause
	if query != "" {
		like := likeContains(query)
		c.add("(username ILIKE ? ESCAPE '\\' OR email ILIKE ? ESCAPE '\\' OR full_name ILIKE ? ESCAPE '\\')", like, like, like)
	}
	if role != "" {
		c.add("role = ?", role)
	}
	if active != nil {
		c.add("active = ?", *active)
	}
	return listRows(ctx, s.pool, `users`, userCols, `lower(username)`, c, limit, offset, scanUser)
}

// EnsureAdmin creates the first administrator when the users table is empty.
func (s *Store) EnsureAdmin(ctx context.Context, username, email, fullName, password string) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		return mapErr(err)
	}
	if n > 0 {
		return nil
	}
	if strings.TrimSpace(password) == "" {
		return invalid("QC_ADMIN_PASSWORD is required to create the first administrator")
	}
	_, err := s.insertUser(ctx, s.pool, domain.Actor{}, domain.NewUser{
		Username: username,
		Email:    email,
		Password: password,
		FullName: fullName,
		Role:     domain.RoleAdmin,
	})
	return err
}

// CreateUser adds an account.
func (s *Store) CreateUser(ctx context.Context, actor domain.Actor, in domain.NewUser) (domain.User, error) {
	return s.insertUser(ctx, s.pool, actor, in)
}

func (s *Store) insertUser(ctx context.Context, q querier, actor domain.Actor, in domain.NewUser) (domain.User, error) {
	in.Username = domain.Trim(in.Username)
	in.Email = domain.Trim(in.Email)
	in.FullName = domain.Trim(in.FullName)
	if err := domain.CheckUsername(in.Username); err != nil {
		return domain.User{}, wrapInvalid(err)
	}
	if err := domain.CheckEmail(in.Email); err != nil {
		return domain.User{}, wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.FullName, "full_name", 200); err != nil {
		return domain.User{}, wrapInvalid(err)
	}
	if err := domain.CheckRole(in.Role); err != nil {
		return domain.User{}, wrapInvalid(err)
	}
	if err := domain.CheckPassword(in.Password, in.Username); err != nil {
		return domain.User{}, wrapInvalid(err)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return domain.User{}, err
	}
	id := uuid.New()
	u, err := scanUser(q.QueryRow(ctx, `
		INSERT INTO users (id, username, email, password_hash, full_name, role)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+userCols, id, in.Username, in.Email, hash, in.FullName, in.Role))
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	if err := s.writeAudit(ctx, q, actor, "user.create", "user", u.ID, "Created user "+u.Username, nil, u); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// UpdateUser changes email, name, role, or active. Username stays put.
func (s *Store) UpdateUser(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchUser) (domain.User, error) {
	var updated domain.User
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		current, err := scanUser(tx.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if in.Email == nil && in.FullName == nil && in.Role == nil && in.Active == nil {
			return invalid("no fields to update")
		}
		nextRole, nextActive := current.Role, current.Active
		var set setter
		if in.Email != nil {
			email := domain.Trim(*in.Email)
			if err := domain.CheckEmail(email); err != nil {
				return wrapInvalid(err)
			}
			set.add("email", email)
		}
		if in.FullName != nil {
			name := domain.Trim(*in.FullName)
			if err := domain.CheckLabel(name, "full_name", 200); err != nil {
				return wrapInvalid(err)
			}
			set.add("full_name", name)
		}
		if in.Role != nil {
			if err := domain.CheckRole(*in.Role); err != nil {
				return wrapInvalid(err)
			}
			nextRole = *in.Role
			set.add("role", nextRole)
		}
		if in.Active != nil {
			nextActive = *in.Active
			set.add("active", nextActive)
		}
		if current.Role == domain.RoleAdmin && current.Active && (nextRole != domain.RoleAdmin || !nextActive) {
			var admins int
			// Lock every admin row so two admins cannot demote each other at once.
			if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE role = 'admin' AND active ORDER BY id FOR UPDATE`); err != nil {
				return mapErr(err)
			}
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin' AND active AND id <> $1`, id).Scan(&admins); err != nil {
				return mapErr(err)
			}
			if admins == 0 {
				return invalid("the last active administrator cannot be removed")
			}
		}
		set.args = append(set.args, id)
		updated, err = scanUser(tx.QueryRow(ctx, `UPDATE users SET `+strings.Join(set.sets, ", ")+` WHERE id = $`+itoa(len(set.args))+` RETURNING `+userCols, set.args...))
		if err != nil {
			return mapErr(err)
		}
		if !updated.Active {
			if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
				return mapErr(err)
			}
		}
		return s.writeAudit(ctx, tx, actor, "user.update", "user", id, "Updated user "+updated.Username, current, updated)
	})
	return updated, err
}

// ChangePassword checks the current password, stores a new one, and revokes refresh tokens.
func (s *Store) ChangePassword(ctx context.Context, actor domain.Actor, id uuid.UUID, current, next string) error {
	var username string
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT username, password_hash FROM users WHERE id = $1 AND active`, id).Scan(&username, &hash)
	if err != nil {
		return mapErr(err)
	}
	if !auth.CheckPassword(hash, current) {
		return fail(ErrUnauthorized, "current password is incorrect")
	}
	return s.setPassword(ctx, actor, id, username, next)
}

// ResetPassword sets a password without the current one. Caller must already be an administrator.
func (s *Store) ResetPassword(ctx context.Context, actor domain.Actor, id uuid.UUID, next string) error {
	var username string
	err := s.pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, id).Scan(&username)
	if err != nil {
		return mapErr(err)
	}
	return s.setPassword(ctx, actor, id, username, next)
}

func (s *Store) setPassword(ctx context.Context, actor domain.Actor, id uuid.UUID, username, next string) error {
	if err := domain.CheckPassword(next, username); err != nil {
		return wrapInvalid(err)
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)
		if err != nil {
			return mapErr(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "user.password", "user", id, "Password changed for "+username, nil, map[string]string{"password": "updated"})
	})
}

// dummyHash is compared against for unknown usernames so they take as long as real ones.
var dummyHash = sync.OnceValue(func() string {
	h, _ := auth.HashPassword("unknown-user-timing-pad")
	return h
})

// Authenticate checks a username and password and records a successful login.
func (s *Store) Authenticate(ctx context.Context, username, password string, actor domain.Actor) (domain.User, error) {
	_, _ = s.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < now() - interval '1 day'`)
	var hash string
	row := s.pool.QueryRow(ctx, `SELECT `+userCols+`, password_hash FROM users WHERE lower(username) = lower($1)`, domain.Trim(username))
	var u domain.User
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.FullName, &u.Role, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt, &hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			auth.CheckPassword(dummyHash(), password)
			return domain.User{}, fail(ErrUnauthorized, "username or password is incorrect")
		}
		return domain.User{}, mapErr(err)
	}
	if !auth.CheckPassword(hash, password) {
		return domain.User{}, fail(ErrUnauthorized, "username or password is incorrect")
	}
	if !u.Active {
		return domain.User{}, fail(ErrUnauthorized, "account is disabled")
	}
	updated, err := scanUser(s.pool.QueryRow(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1 RETURNING `+userCols, u.ID))
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	actor.ID = updated.ID
	if err := s.writeAudit(ctx, s.pool, actor, "auth.login", "user", updated.ID, "Logged in", nil, map[string]string{"username": updated.Username}); err != nil {
		return domain.User{}, err
	}
	return updated, nil
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IssueRefresh stores a new refresh token and returns the client secret.
func (s *Store) IssueRefresh(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	secret, hash, err := auth.NewRefreshToken()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1,$2,$3,$4)`, uuid.New(), userID, hash, time.Now().UTC().Add(ttl))
	if err != nil {
		return "", mapErr(err)
	}
	return secret, nil
}

// RotateRefresh revokes the presented refresh token and stores a new one.
func (s *Store) RotateRefresh(ctx context.Context, presented string, ttl time.Duration) (domain.User, string, error) {
	oldHash := auth.HashRefresh(presented)
	var user domain.User
	var secret string
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var tokenID uuid.UUID
		var userID uuid.UUID
		var active bool
		err := tx.QueryRow(ctx, `
			SELECT t.id, t.user_id, u.active
			FROM refresh_tokens t
			JOIN users u ON u.id = t.user_id
			WHERE t.token_hash = $1 AND t.revoked_at IS NULL AND t.expires_at > now()
			FOR UPDATE OF t`, oldHash).Scan(&tokenID, &userID, &active)
		if err != nil {
			if isNoRows(err) {
				return fail(ErrUnauthorized, "refresh token is not valid")
			}
			return mapErr(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, tokenID); err != nil {
			return mapErr(err)
		}
		if !active {
			return fail(ErrUnauthorized, "account is disabled")
		}
		user, err = scanUser(tx.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, userID))
		if err != nil {
			return mapErr(err)
		}
		var hash string
		secret, hash, err = auth.NewRefreshToken()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
			VALUES ($1,$2,$3,$4)`, uuid.New(), userID, hash, time.Now().UTC().Add(ttl))
		return mapErr(err)
	})
	return user, secret, err
}

// RevokeRefresh marks a refresh token revoked. Missing tokens are not an error.
func (s *Store) RevokeRefresh(ctx context.Context, actor domain.Actor, presented string) error {
	hash := auth.HashRefresh(presented)
	tag, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() > 0 && actor.ID != uuid.Nil {
		return s.writeAudit(ctx, s.pool, actor, "auth.logout", "user", actor.ID, "Logged out", nil, nil)
	}
	return nil
}

func listRows[T any](ctx context.Context, q querier, table, cols, order string, c clause, limit, offset int, scan func(scannable) (T, error)) ([]T, int, error) {
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM `+table+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, limit, offset)
	sql := `SELECT ` + cols + ` FROM ` + table + c.where() + ` ORDER BY ` + order + ` LIMIT $` + itoa(len(c.args)+1) + ` OFFSET $` + itoa(len(c.args)+2)
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	items, err := collect(rows, scan)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	return items, total, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// clip shortens s to at most n bytes without splitting a UTF-8 character,
// dropping invalid bytes Postgres would reject.
func clip(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
