package store

import (
	"context"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const ncrCols = `id, ncr_number, inspection_id, lot_id, product_id, title, description, severity, status, disposition, quantity_affected, assigned_to, opened_by, disposed_by, closed_by, disposition_notes, opened_at, disposed_at, closed_at, created_at, updated_at`

func scanNCR(row scannable) (domain.Nonconformance, error) {
	var v domain.Nonconformance
	var opened, disposed, closed *uuid.UUID
	err := row.Scan(&v.ID, &v.NCRNumber, &v.InspectionID, &v.LotID, &v.ProductID, &v.Title, &v.Description, &v.Severity, &v.Status, &v.Disposition, &v.QuantityAffected, &v.AssignedTo, &opened, &disposed, &closed, &v.DispositionNotes, &v.OpenedAt, &v.DisposedAt, &v.ClosedAt, &v.CreatedAt, &v.UpdatedAt)
	v.OpenedBy, v.DisposedBy, v.ClosedBy = uuidOrNil(opened), disposed, closed
	return v, err
}

const capaCols = `id, ca_number, nonconformance_id, title, root_cause, action_plan, status, owner_id, COALESCE(due_date::text, ''), verification_notes, effectiveness_notes, opened_by, verified_by, closed_by, opened_at, verified_at, closed_at, created_at, updated_at`

func scanCAPA(row scannable) (domain.CorrectiveAction, error) {
	var v domain.CorrectiveAction
	var opened, verified, closed *uuid.UUID
	err := row.Scan(&v.ID, &v.CANumber, &v.NonconformanceID, &v.Title, &v.RootCause, &v.ActionPlan, &v.Status, &v.OwnerID, &v.DueDate, &v.VerificationNotes, &v.EffectivenessNotes, &opened, &verified, &closed, &v.OpenedAt, &v.VerifiedAt, &v.ClosedAt, &v.CreatedAt, &v.UpdatedAt)
	v.OpenedBy, v.VerifiedBy, v.ClosedBy = uuidOrNil(opened), verified, closed
	return v, err
}

type NCRFilter struct {
	Status    string
	Severity  string
	LotID     *uuid.UUID
	ProductID *uuid.UUID
	Limit     int
	Offset    int
}

func (s *Store) ListNCRs(ctx context.Context, f NCRFilter) ([]domain.Nonconformance, int, error) {
	if f.Status != "" {
		if err := domain.CheckOneOf(f.Status, "status", ncrStatuses()); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	if f.Severity != "" {
		if err := domain.CheckOneOf(f.Severity, "severity", severities()); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	f.Limit, f.Offset = clamp(f.Limit, f.Offset, 200)
	var c clause
	if f.Status != "" {
		c.add("status = ?", f.Status)
	}
	if f.Severity != "" {
		c.add("severity = ?", f.Severity)
	}
	if f.LotID != nil {
		c.add("lot_id = ?", *f.LotID)
	}
	if f.ProductID != nil {
		c.add("product_id = ?", *f.ProductID)
	}
	return listRows(ctx, s.pool, "nonconformances", ncrCols, "opened_at DESC, id DESC", c, f.Limit, f.Offset, scanNCR)
}

func (s *Store) GetNCR(ctx context.Context, id uuid.UUID) (domain.NCRDetail, error) {
	ncr, err := scanNCR(s.pool.QueryRow(ctx, `SELECT `+ncrCols+` FROM nonconformances WHERE id = $1`, id))
	if err != nil {
		return domain.NCRDetail{}, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT `+capaCols+` FROM corrective_actions WHERE nonconformance_id = $1 ORDER BY opened_at`, id)
	if err != nil {
		return domain.NCRDetail{}, mapErr(err)
	}
	actions, err := collect(rows, scanCAPA)
	if err != nil {
		return domain.NCRDetail{}, mapErr(err)
	}
	return domain.NCRDetail{Nonconformance: ncr, CorrectiveActions: actions}, nil
}

func (s *Store) CreateNCR(ctx context.Context, actor domain.Actor, in domain.NewNonconformance) (domain.Nonconformance, error) {
	var out domain.Nonconformance
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if in.LotID != nil {
			var product uuid.UUID
			err := tx.QueryRow(ctx, `SELECT product_id FROM lots WHERE id = $1`, *in.LotID).Scan(&product)
			if err != nil {
				if isNoRows(err) {
					return invalid("lot does not exist")
				}
				return mapErr(err)
			}
			if product != in.ProductID {
				return invalid("lot is for a different product")
			}
		}
		if in.InspectionID != nil {
			var lotID uuid.UUID
			err := tx.QueryRow(ctx, `SELECT lot_id FROM inspections WHERE id = $1`, *in.InspectionID).Scan(&lotID)
			if err != nil {
				if isNoRows(err) {
					return invalid("inspection does not exist")
				}
				return mapErr(err)
			}
			if in.LotID != nil && *in.LotID != lotID {
				return invalid("inspection is for a different lot")
			}
			in.LotID = &lotID
			var product uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT product_id FROM lots WHERE id = $1`, lotID).Scan(&product); err != nil {
				return mapErr(err)
			}
			if product != in.ProductID {
				return invalid("inspection is for a different product")
			}
		}
		var err error
		out, err = s.insertNCR(ctx, tx, actor, in)
		return err
	})
	return out, err
}

func (s *Store) insertNCR(ctx context.Context, q querier, actor domain.Actor, in domain.NewNonconformance) (domain.Nonconformance, error) {
	in.Title, in.Description, in.Severity = domain.Trim(in.Title), domain.Trim(in.Description), domain.Trim(in.Severity)
	if err := domain.CheckLabel(in.Title, "title", 200); err != nil {
		return domain.Nonconformance{}, wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.Description, "description", 8000); err != nil {
		return domain.Nonconformance{}, wrapInvalid(err)
	}
	if err := domain.CheckOneOf(in.Severity, "severity", severities()); err != nil {
		return domain.Nonconformance{}, wrapInvalid(err)
	}
	if err := domain.CheckNonNegative(in.QuantityAffected, "quantity_affected"); err != nil {
		return domain.Nonconformance{}, wrapInvalid(err)
	}
	if in.AssignedTo != nil {
		if err := s.requireUser(ctx, q, *in.AssignedTo, domain.RoleOperator); err != nil {
			return domain.Nonconformance{}, err
		}
	}
	number, err := nextNumber(ctx, q, "NCR")
	if err != nil {
		return domain.Nonconformance{}, err
	}
	status := domain.NCROpen
	if in.AssignedTo != nil {
		status = domain.NCRUnderReview
	}
	var opened any
	if actor.ID != uuid.Nil {
		opened = actor.ID
	}
	ncr, err := scanNCR(q.QueryRow(ctx, `
		INSERT INTO nonconformances (id, ncr_number, inspection_id, lot_id, product_id, title, description, severity, status, quantity_affected, assigned_to, opened_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+ncrCols,
		uuid.New(), number, in.InspectionID, in.LotID, in.ProductID, in.Title, in.Description, in.Severity, status, in.QuantityAffected, in.AssignedTo, opened))
	if err != nil {
		return domain.Nonconformance{}, mapErr(err)
	}
	return ncr, s.writeAudit(ctx, q, actor, "ncr.create", "nonconformance", ncr.ID, "Opened "+ncr.NCRNumber, nil, ncr)
}

func (s *Store) UpdateNCR(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchNonconformance) (domain.Nonconformance, error) {
	var out domain.Nonconformance
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanNCR(tx.QueryRow(ctx, `SELECT `+ncrCols+` FROM nonconformances WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if cur.Status != domain.NCROpen && cur.Status != domain.NCRUnderReview {
			return invalid("this nonconformance can no longer be edited")
		}
		title, desc, sev := cur.Title, cur.Description, cur.Severity
		qty := cur.QuantityAffected
		assigned := cur.AssignedTo
		if in.Title != nil {
			title = domain.Trim(*in.Title)
		}
		if in.Description != nil {
			desc = domain.Trim(*in.Description)
		}
		if in.Severity != nil {
			sev = domain.Trim(*in.Severity)
		}
		if in.QuantityAffected != nil {
			qty = *in.QuantityAffected
		}
		if in.AssignedTo != nil {
			assigned = in.AssignedTo
			if err := s.requireUser(ctx, tx, *assigned, domain.RoleOperator); err != nil {
				return err
			}
		}
		if err := domain.CheckLabel(title, "title", 200); err != nil {
			return wrapInvalid(err)
		}
		if err := domain.CheckLabel(desc, "description", 8000); err != nil {
			return wrapInvalid(err)
		}
		if err := domain.CheckOneOf(sev, "severity", severities()); err != nil {
			return wrapInvalid(err)
		}
		if err := domain.CheckNonNegative(qty, "quantity_affected"); err != nil {
			return wrapInvalid(err)
		}
		status := cur.Status
		if assigned != nil && status == domain.NCROpen {
			status = domain.NCRUnderReview
		}
		var set setter
		if in.Title != nil {
			set.add("title", title)
		}
		if in.Description != nil {
			set.add("description", desc)
		}
		if in.Severity != nil {
			set.add("severity", sev)
		}
		if in.QuantityAffected != nil {
			set.add("quantity_affected", qty)
		}
		if in.AssignedTo != nil {
			set.add("assigned_to", *assigned)
		}
		if status != cur.Status {
			set.add("status", status)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "nonconformances", ncrCols, set, id, scanNCR)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "ncr.update", "nonconformance", id, "Updated "+out.NCRNumber, cur, out)
	})
	return out, err
}

func (s *Store) DisposeNCR(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.DispositionRequest) (domain.Nonconformance, error) {
	in.Disposition = domain.Trim(in.Disposition)
	in.Notes = domain.Trim(in.Notes)
	if err := domain.CheckOneOf(in.Disposition, "disposition", []string{domain.DispUseAsIs, domain.DispRework, domain.DispRepair, domain.DispScrap, domain.DispReturnToSupplier, domain.DispSort}); err != nil {
		return domain.Nonconformance{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Notes, "notes", 8000); err != nil {
		return domain.Nonconformance{}, wrapInvalid(err)
	}
	return s.transitionNCR(ctx, actor, id, domain.NCRDispositioned, func(cur domain.Nonconformance) (string, []any) {
		return `UPDATE nonconformances SET status = $2, disposition = $3, disposition_notes = $4, disposed_by = $5, disposed_at = $6 WHERE id = $1 RETURNING ` + ncrCols,
			[]any{id, domain.NCRDispositioned, in.Disposition, in.Notes, nilUUID(actor.ID), time.Now().UTC()}
	})
}

func (s *Store) CloseNCR(ctx context.Context, actor domain.Actor, id uuid.UUID) (domain.Nonconformance, error) {
	return s.transitionNCR(ctx, actor, id, domain.NCRClosed, func(cur domain.Nonconformance) (string, []any) {
		if cur.Disposition == domain.DispPending {
			return "", nil
		}
		return `UPDATE nonconformances SET status = $2, closed_by = $3, closed_at = $4 WHERE id = $1 RETURNING ` + ncrCols,
			[]any{id, domain.NCRClosed, nilUUID(actor.ID), time.Now().UTC()}
	})
}

func (s *Store) VoidNCR(ctx context.Context, actor domain.Actor, id uuid.UUID, notes string) (domain.Nonconformance, error) {
	notes = domain.Trim(notes)
	if err := domain.CheckOptional(notes, "notes", 8000); err != nil {
		return domain.Nonconformance{}, wrapInvalid(err)
	}
	return s.transitionNCR(ctx, actor, id, domain.NCRVoid, func(cur domain.Nonconformance) (string, []any) {
		combined := cur.DispositionNotes
		if notes != "" {
			if combined != "" {
				combined += "\n"
			}
			combined += notes
		}
		return `UPDATE nonconformances SET status = $2, disposition_notes = $3 WHERE id = $1 RETURNING ` + ncrCols,
			[]any{id, domain.NCRVoid, combined}
	})
}

func (s *Store) transitionNCR(ctx context.Context, actor domain.Actor, id uuid.UUID, to string, sqlFor func(domain.Nonconformance) (string, []any)) (domain.Nonconformance, error) {
	var out domain.Nonconformance
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanNCR(tx.QueryRow(ctx, `SELECT `+ncrCols+` FROM nonconformances WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if !domain.CanNCRTransition(cur.Status, to) {
			return invalid("that nonconformance status change is not allowed")
		}
		if to == domain.NCRClosed && cur.Disposition == domain.DispPending {
			return invalid("record a disposition before closing")
		}
		sql, args := sqlFor(cur)
		if sql == "" {
			return invalid("record a disposition before closing")
		}
		out, err = scanNCR(tx.QueryRow(ctx, sql, args...))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "ncr."+to, "nonconformance", id, out.NCRNumber+" is "+to, cur, out)
	})
	return out, err
}

type CAPAFilter struct {
	Status           string
	OwnerID          *uuid.UUID
	NonconformanceID *uuid.UUID
	Limit            int
	Offset           int
}

func (s *Store) ListCAPAs(ctx context.Context, f CAPAFilter) ([]domain.CorrectiveAction, int, error) {
	if f.Status != "" {
		if err := domain.CheckOneOf(f.Status, "status", capaStatuses()); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	f.Limit, f.Offset = clamp(f.Limit, f.Offset, 200)
	var c clause
	if f.Status != "" {
		c.add("status = ?", f.Status)
	}
	if f.OwnerID != nil {
		c.add("owner_id = ?", *f.OwnerID)
	}
	if f.NonconformanceID != nil {
		c.add("nonconformance_id = ?", *f.NonconformanceID)
	}
	return listRows(ctx, s.pool, "corrective_actions", capaCols, "opened_at DESC, id DESC", c, f.Limit, f.Offset, scanCAPA)
}

func (s *Store) GetCAPA(ctx context.Context, id uuid.UUID) (domain.CorrectiveAction, error) {
	v, err := scanCAPA(s.pool.QueryRow(ctx, `SELECT `+capaCols+` FROM corrective_actions WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateCAPA(ctx context.Context, actor domain.Actor, in domain.NewCorrectiveAction) (domain.CorrectiveAction, error) {
	in.Title = domain.Trim(in.Title)
	in.RootCause = domain.Trim(in.RootCause)
	in.ActionPlan = domain.Trim(in.ActionPlan)
	if err := domain.CheckLabel(in.Title, "title", 200); err != nil {
		return domain.CorrectiveAction{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.RootCause, "root_cause", 8000); err != nil {
		return domain.CorrectiveAction{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.ActionPlan, "action_plan", 8000); err != nil {
		return domain.CorrectiveAction{}, wrapInvalid(err)
	}
	due, err := dateArg(in.DueDate)
	if err != nil {
		return domain.CorrectiveAction{}, err
	}
	var out domain.CorrectiveAction
	err = s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if in.NonconformanceID != nil {
			var status string
			err := tx.QueryRow(ctx, `SELECT status FROM nonconformances WHERE id = $1`, *in.NonconformanceID).Scan(&status)
			if err != nil {
				if isNoRows(err) {
					return invalid("nonconformance does not exist")
				}
				return mapErr(err)
			}
			if status == domain.NCRVoid {
				return invalid("a void nonconformance cannot take a corrective action")
			}
		}
		if in.OwnerID != nil {
			if err := s.requireUser(ctx, tx, *in.OwnerID, domain.RoleOperator); err != nil {
				return err
			}
		}
		number, err := nextNumber(ctx, tx, "CA")
		if err != nil {
			return err
		}
		var opened any
		if actor.ID != uuid.Nil {
			opened = actor.ID
		}
		out, err = scanCAPA(tx.QueryRow(ctx, `
			INSERT INTO corrective_actions (id, ca_number, nonconformance_id, title, root_cause, action_plan, owner_id, due_date, opened_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+capaCols,
			uuid.New(), number, in.NonconformanceID, in.Title, in.RootCause, in.ActionPlan, in.OwnerID, due, opened))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "capa.create", "corrective_action", out.ID, "Opened "+out.CANumber, nil, out)
	})
	return out, err
}

func (s *Store) UpdateCAPA(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchCorrectiveAction) (domain.CorrectiveAction, error) {
	var out domain.CorrectiveAction
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanCAPA(tx.QueryRow(ctx, `SELECT `+capaCols+` FROM corrective_actions WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if cur.Status == domain.CAPAClosed || cur.Status == domain.CAPACancelled {
			return invalid("this corrective action can no longer be edited")
		}
		title, cause, plan := cur.Title, cur.RootCause, cur.ActionPlan
		dueText := cur.DueDate
		if in.Title != nil {
			title = domain.Trim(*in.Title)
		}
		if in.RootCause != nil {
			cause = domain.Trim(*in.RootCause)
		}
		if in.ActionPlan != nil {
			plan = domain.Trim(*in.ActionPlan)
		}
		if in.DueDate != nil {
			dueText = domain.Trim(*in.DueDate)
		}
		if err := domain.CheckLabel(title, "title", 200); err != nil {
			return wrapInvalid(err)
		}
		if err := domain.CheckOptional(cause, "root_cause", 8000); err != nil {
			return wrapInvalid(err)
		}
		if err := domain.CheckOptional(plan, "action_plan", 8000); err != nil {
			return wrapInvalid(err)
		}
		due, err := dateArg(dueText)
		if err != nil {
			return err
		}
		if in.OwnerID != nil {
			if err := s.requireUser(ctx, tx, *in.OwnerID, domain.RoleOperator); err != nil {
				return err
			}
		}
		var set setter
		if in.Title != nil {
			set.add("title", title)
		}
		if in.RootCause != nil {
			set.add("root_cause", cause)
		}
		if in.ActionPlan != nil {
			set.add("action_plan", plan)
		}
		if in.OwnerID != nil {
			set.add("owner_id", *in.OwnerID)
		}
		if in.DueDate != nil {
			set.add("due_date", due)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "corrective_actions", capaCols, set, id, scanCAPA)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "capa.update", "corrective_action", id, "Updated "+out.CANumber, cur, out)
	})
	return out, err
}

func (s *Store) TransitionCAPA(ctx context.Context, actor domain.Actor, id uuid.UUID, to, notes string) (domain.CorrectiveAction, error) {
	notes = domain.Trim(notes)
	if err := domain.CheckOptional(notes, "notes", 8000); err != nil {
		return domain.CorrectiveAction{}, wrapInvalid(err)
	}
	if to == domain.CAPAVerification && notes == "" {
		return domain.CorrectiveAction{}, invalid("verification notes are required")
	}
	if to == domain.CAPAClosed && notes == "" {
		return domain.CorrectiveAction{}, invalid("effectiveness notes are required")
	}
	var out domain.CorrectiveAction
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanCAPA(tx.QueryRow(ctx, `SELECT `+capaCols+` FROM corrective_actions WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if !domain.CanCAPATransition(cur.Status, to) {
			return invalid("that corrective action status change is not allowed")
		}
		now := time.Now().UTC()
		switch to {
		case domain.CAPAInProgress:
			out, err = scanCAPA(tx.QueryRow(ctx, `UPDATE corrective_actions SET status = $2 WHERE id = $1 RETURNING `+capaCols, id, to))
		case domain.CAPAVerification:
			out, err = scanCAPA(tx.QueryRow(ctx, `
				UPDATE corrective_actions SET status = $2, verification_notes = $3, verified_by = $4, verified_at = $5
				WHERE id = $1 RETURNING `+capaCols, id, to, notes, nilUUID(actor.ID), now))
		case domain.CAPAClosed:
			out, err = scanCAPA(tx.QueryRow(ctx, `
				UPDATE corrective_actions SET status = $2, effectiveness_notes = $3, closed_by = $4, closed_at = $5
				WHERE id = $1 RETURNING `+capaCols, id, to, notes, nilUUID(actor.ID), now))
		case domain.CAPACancelled:
			text := cur.VerificationNotes
			if notes != "" {
				if text != "" {
					text += "\n"
				}
				text += notes
			}
			out, err = scanCAPA(tx.QueryRow(ctx, `UPDATE corrective_actions SET status = $2, verification_notes = $3 WHERE id = $1 RETURNING `+capaCols, id, to, text))
		default:
			return invalid("that corrective action status change is not allowed")
		}
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "capa."+to, "corrective_action", id, out.CANumber+" is "+to, cur, out)
	})
	return out, err
}

func nilUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func severities() []string {
	return []string{domain.SeverityMinor, domain.SeverityMajor, domain.SeverityCritical}
}

func ncrStatuses() []string {
	return []string{domain.NCROpen, domain.NCRUnderReview, domain.NCRDispositioned, domain.NCRClosed, domain.NCRVoid}
}

func capaStatuses() []string {
	return []string{domain.CAPAOpen, domain.CAPAInProgress, domain.CAPAVerification, domain.CAPAClosed, domain.CAPACancelled}
}
