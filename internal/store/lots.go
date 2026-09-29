package store

import (
	"context"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const lotCols = `id, lot_number, product_id, site_id, supplier_id, quantity, unit, source, supplier_lot, status, received_at, notes, created_by, created_at, updated_at`

func scanLot(row scannable) (domain.Lot, error) {
	var v domain.Lot
	var createdBy *uuid.UUID
	err := row.Scan(&v.ID, &v.LotNumber, &v.ProductID, &v.SiteID, &v.SupplierID, &v.Quantity, &v.Unit, &v.Source, &v.SupplierLot, &v.Status, &v.ReceivedAt, &v.Notes, &createdBy, &v.CreatedAt, &v.UpdatedAt)
	v.CreatedBy = uuidOrNil(createdBy)
	return v, err
}

type LotFilter struct {
	Query     string
	Status    string
	Source    string
	ProductID *uuid.UUID
	SiteID    *uuid.UUID
	Limit     int
	Offset    int
}

func (s *Store) ListLots(ctx context.Context, f LotFilter) ([]domain.Lot, int, error) {
	query, err := cleanQuery(f.Query)
	if err != nil {
		return nil, 0, err
	}
	if f.Status != "" {
		if err := domain.CheckOneOf(f.Status, "status", lotStatuses()); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	if f.Source != "" {
		if err := domain.CheckOneOf(f.Source, "source", lotSources()); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	f.Limit, f.Offset = clamp(f.Limit, f.Offset, 200)
	var c clause
	if query != "" {
		like := likeContains(query)
		c.add("(lot_number ILIKE ? ESCAPE '\\' OR supplier_lot ILIKE ? ESCAPE '\\')", like, like)
	}
	if f.Status != "" {
		c.add("status = ?", f.Status)
	}
	if f.Source != "" {
		c.add("source = ?", f.Source)
	}
	if f.ProductID != nil {
		c.add("product_id = ?", *f.ProductID)
	}
	if f.SiteID != nil {
		c.add("site_id = ?", *f.SiteID)
	}
	return listRows(ctx, s.pool, "lots", lotCols, "created_at DESC, id DESC", c, f.Limit, f.Offset, scanLot)
}

func (s *Store) GetLot(ctx context.Context, id uuid.UUID) (domain.Lot, error) {
	v, err := scanLot(s.pool.QueryRow(ctx, `SELECT `+lotCols+` FROM lots WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateLot(ctx context.Context, actor domain.Actor, in domain.NewLot) (domain.Lot, error) {
	in.LotNumber = domain.Trim(in.LotNumber)
	in.Unit = domain.Trim(in.Unit)
	in.Source = domain.Trim(in.Source)
	in.SupplierLot = domain.Trim(in.SupplierLot)
	in.Notes = domain.Trim(in.Notes)
	if err := domain.CheckLabel(in.LotNumber, "lot_number", 80); err != nil {
		return domain.Lot{}, wrapInvalid(err)
	}
	if err := domain.CheckPositive(in.Quantity, "quantity"); err != nil {
		return domain.Lot{}, wrapInvalid(err)
	}
	if err := domain.CheckOneOf(in.Source, "source", lotSources()); err != nil {
		return domain.Lot{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.SupplierLot, "supplier_lot", 80); err != nil {
		return domain.Lot{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Notes, "notes", 8000); err != nil {
		return domain.Lot{}, wrapInvalid(err)
	}
	var out domain.Lot
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var unit string
		var active bool
		err := tx.QueryRow(ctx, `SELECT unit, active FROM products WHERE id = $1`, in.ProductID).Scan(&unit, &active)
		if err != nil {
			if isNoRows(err) {
				return invalid("product does not exist")
			}
			return mapErr(err)
		}
		if !active {
			return invalid("product is not active")
		}
		if in.Unit == "" {
			in.Unit = unit
		}
		if err := domain.CheckLabel(in.Unit, "unit", 40); err != nil {
			return wrapInvalid(err)
		}
		if err := s.requireActiveRow(ctx, tx, "sites", in.SiteID, "site"); err != nil {
			return err
		}
		if in.SupplierID != nil {
			if err := s.requireActiveRow(ctx, tx, "suppliers", *in.SupplierID, "supplier"); err != nil {
				return err
			}
		}
		var createdBy any
		if actor.ID != uuid.Nil {
			createdBy = actor.ID
		}
		out, err = scanLot(tx.QueryRow(ctx, `
			INSERT INTO lots (id, lot_number, product_id, site_id, supplier_id, quantity, unit, source, supplier_lot, notes, created_by, received_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+lotCols,
			uuid.New(), in.LotNumber, in.ProductID, in.SiteID, in.SupplierID, in.Quantity, in.Unit, in.Source, in.SupplierLot, in.Notes, createdBy, time.Now().UTC()))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "lot.create", "lot", out.ID, "Created lot "+out.LotNumber, nil, out)
	})
	return out, err
}

func (s *Store) UpdateLot(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchLot) (domain.Lot, error) {
	var out domain.Lot
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanLot(tx.QueryRow(ctx, `SELECT `+lotCols+` FROM lots WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if cur.Status == domain.LotClosed {
			return invalid("a closed lot cannot be edited")
		}
		var set setter
		if in.LotNumber != nil {
			if cur.Status != domain.LotOpen {
				return invalid("lot number can only change while the lot is open")
			}
			number := domain.Trim(*in.LotNumber)
			if err := domain.CheckLabel(number, "lot_number", 80); err != nil {
				return wrapInvalid(err)
			}
			set.add("lot_number", number)
		}
		if in.SupplierID != nil {
			if err := s.requireActiveRow(ctx, tx, "suppliers", *in.SupplierID, "supplier"); err != nil {
				return err
			}
			set.add("supplier_id", *in.SupplierID)
		}
		if in.Quantity != nil {
			if err := domain.CheckPositive(*in.Quantity, "quantity"); err != nil {
				return wrapInvalid(err)
			}
			set.add("quantity", *in.Quantity)
		}
		if in.Unit != nil {
			unit := domain.Trim(*in.Unit)
			if err := domain.CheckLabel(unit, "unit", 40); err != nil {
				return wrapInvalid(err)
			}
			set.add("unit", unit)
		}
		if in.SupplierLot != nil {
			lot := domain.Trim(*in.SupplierLot)
			if err := domain.CheckOptional(lot, "supplier_lot", 80); err != nil {
				return wrapInvalid(err)
			}
			set.add("supplier_lot", lot)
		}
		if in.Notes != nil {
			notes := domain.Trim(*in.Notes)
			if err := domain.CheckOptional(notes, "notes", 8000); err != nil {
				return wrapInvalid(err)
			}
			set.add("notes", notes)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "lots", lotCols, set, id, scanLot)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "lot.update", "lot", id, "Updated lot "+out.LotNumber, cur, out)
	})
	return out, err
}

func (s *Store) SetLotStatus(ctx context.Context, actor domain.Actor, id uuid.UUID, status string) (domain.Lot, error) {
	if err := domain.CheckOneOf(status, "status", lotStatuses()); err != nil {
		return domain.Lot{}, wrapInvalid(err)
	}
	var out domain.Lot
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanLot(tx.QueryRow(ctx, `SELECT `+lotCols+` FROM lots WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM inspections WHERE lot_id = $1 AND status = 'in_progress'`, id).Scan(&open); err != nil {
			return mapErr(err)
		}
		if open > 0 {
			return invalid("finish or cancel the open inspection before changing lot status")
		}
		if !domain.CanManualLotTransition(cur.Status, status) {
			return invalid("that lot status change is not allowed")
		}
		out, err = scanLot(tx.QueryRow(ctx, `UPDATE lots SET status = $2 WHERE id = $1 RETURNING `+lotCols, id, status))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "lot.status", "lot", id, "Lot "+out.LotNumber+" moved from "+cur.Status+" to "+status, cur, out)
	})
	return out, err
}

func lotStatuses() []string {
	return []string{domain.LotOpen, domain.LotInInspection, domain.LotAccepted, domain.LotRejected, domain.LotOnHold, domain.LotClosed}
}

func lotSources() []string {
	return []string{domain.SourceIncoming, domain.SourceInProcess, domain.SourceFinal, domain.SourceCustomerReturn}
}
