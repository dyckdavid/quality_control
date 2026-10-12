package store

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalid      = errors.New("invalid")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
)

// Error is a safe message for an API client, wrapped around a sentinel.
type Error struct {
	Kind error
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func (e *Error) Unwrap() error { return e.Kind }

func fail(kind error, msg string) error {
	return &Error{Kind: kind, Msg: msg}
}

func invalid(msg string) error { return fail(ErrInvalid, msg) }

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) || errors.Is(err, ErrInvalid) || errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return fail(ErrConflict, conflictMessage(pg.ConstraintName))
		case "23503":
			return invalid("Referenced record does not exist.")
		case "23514":
			return invalid("A value failed validation.")
		}
	}
	return err
}

func conflictMessage(constraint string) string {
	switch constraint {
	case "users_username_idx":
		return "That username is already in use."
	case "users_email_idx":
		return "That email is already in use."
	case "sites_code_idx":
		return "That site code is already in use."
	case "stations_site_code_idx":
		return "That station code is already in use at this site."
	case "suppliers_code_idx":
		return "That supplier code is already in use."
	case "gauges_code_idx":
		return "That gauge code is already in use."
	case "products_part_number_idx":
		return "That part number is already in use."
	case "inspection_plans_revision_idx":
		return "That revision already exists for this product."
	case "inspection_plans_one_active_idx":
		return "This product already has an active inspection plan."
	case "characteristics_code_idx":
		return "That characteristic code is already on this plan."
	case "characteristics_sequence_idx":
		return "That sequence number is already on this plan."
	case "defect_codes_code_idx":
		return "That defect code is already in use."
	case "lots_number_idx":
		return "That lot number is already in use."
	case "checklists_code_idx":
		return "That checklist code is already in use."
	case "checklist_items_sequence_idx":
		return "That sequence number is already on this checklist."
	case "checklist_runs_checklist_id_inspection_id_key":
		return "That checklist is already started on this inspection."
	default:
		return "A record with this identifier already exists."
	}
}

func message(err error) string {
	var app *Error
	if errors.As(err, &app) && app.Msg != "" {
		return app.Msg
	}
	return ""
}

// Message returns the client-safe text for an error, if there is one.
func Message(err error) string { return message(err) }

func wrapInvalid(err error) error {
	if err == nil {
		return nil
	}
	var app *Error
	if errors.As(err, &app) {
		return err
	}
	return invalid(err.Error())
}
