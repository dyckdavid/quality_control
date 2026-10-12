package store

import (
	"context"
	"math"
	"strings"
	"time"

	"quality_control/internal/domain"
	"quality_control/internal/qc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const inspectionCols = `id, inspection_number, lot_id, plan_id, station_id, inspector_id, status, result, sample_size, quantity_inspected, quantity_accepted, quantity_rejected, notes, started_at, completed_at, created_by, created_at, updated_at`

func scanInspection(row scannable) (domain.Inspection, error) {
	var v domain.Inspection
	var createdBy *uuid.UUID
	err := row.Scan(&v.ID, &v.InspectionNumber, &v.LotID, &v.PlanID, &v.StationID, &v.InspectorID, &v.Status, &v.Result, &v.SampleSize, &v.QuantityInspected, &v.QuantityAccepted, &v.QuantityRejected, &v.Notes, &v.StartedAt, &v.CompletedAt, &createdBy, &v.CreatedAt, &v.UpdatedAt)
	v.CreatedBy = uuidOrNil(createdBy)
	return v, err
}

const resultCols = `id, inspection_id, characteristic_id, sample_index, measured_value, attribute_result, in_spec, calibration_warning, is_critical, nominal, lower_limit, upper_limit, unit, characteristic_code, characteristic_name, characteristic_type, gauge_id, notes, recorded_by, recorded_at, updated_at`

func scanResult(row scannable) (domain.InspectionResult, error) {
	var v domain.InspectionResult
	var recordedBy *uuid.UUID
	err := row.Scan(&v.ID, &v.InspectionID, &v.CharacteristicID, &v.SampleIndex, &v.MeasuredValue, &v.AttributeResult, &v.InSpec, &v.CalibrationWarning, &v.Critical, &v.Nominal, &v.LowerLimit, &v.UpperLimit, &v.Unit, &v.CharacteristicCode, &v.CharacteristicName, &v.CharacteristicType, &v.GaugeID, &v.Notes, &recordedBy, &v.RecordedAt, &v.UpdatedAt)
	v.RecordedBy = uuidOrNil(recordedBy)
	return v, err
}

const inspDefectCols = `id, inspection_id, defect_code_id, code, name, severity, quantity, notes, recorded_by, created_at`

func scanInspDefect(row scannable) (domain.InspectionDefect, error) {
	var v domain.InspectionDefect
	var recordedBy *uuid.UUID
	err := row.Scan(&v.ID, &v.InspectionID, &v.DefectCodeID, &v.Code, &v.Name, &v.Severity, &v.Quantity, &v.Notes, &recordedBy, &v.CreatedAt)
	v.RecordedBy = uuidOrNil(recordedBy)
	return v, err
}

type InspectionFilter struct {
	Status      string
	Result      string
	LotID       *uuid.UUID
	InspectorID *uuid.UUID
	From        *time.Time
	To          *time.Time
	Limit       int
	Offset      int
}

func (s *Store) ListInspections(ctx context.Context, f InspectionFilter) ([]domain.Inspection, int, error) {
	if f.Status != "" {
		if err := domain.CheckOneOf(f.Status, "status", []string{domain.InspInProgress, domain.InspCompleted, domain.InspCancelled}); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	if f.Result != "" {
		if err := domain.CheckOneOf(f.Result, "result", []string{domain.ResultPending, domain.ResultPass, domain.ResultFail, domain.ResultConditional}); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	f.Limit, f.Offset = clamp(f.Limit, f.Offset, 200)
	var c clause
	if f.Status != "" {
		c.add("status = ?", f.Status)
	}
	if f.Result != "" {
		c.add("result = ?", f.Result)
	}
	if f.LotID != nil {
		c.add("lot_id = ?", *f.LotID)
	}
	if f.InspectorID != nil {
		c.add("inspector_id = ?", *f.InspectorID)
	}
	if f.From != nil {
		c.add("started_at >= ?", *f.From)
	}
	if f.To != nil {
		c.add("started_at < ?", *f.To)
	}
	return listRows(ctx, s.pool, "inspections", inspectionCols, "created_at DESC, id DESC", c, f.Limit, f.Offset, scanInspection)
}

func (s *Store) GetInspection(ctx context.Context, id uuid.UUID) (domain.InspectionDetail, error) {
	return s.inspectionDetail(ctx, s.pool, id)
}

func (s *Store) inspectionDetail(ctx context.Context, q querier, id uuid.UUID) (domain.InspectionDetail, error) {
	insp, err := scanInspection(q.QueryRow(ctx, `SELECT `+inspectionCols+` FROM inspections WHERE id = $1`, id))
	if err != nil {
		return domain.InspectionDetail{}, mapErr(err)
	}
	results, err := s.resultsFor(ctx, q, id)
	if err != nil {
		return domain.InspectionDetail{}, err
	}
	defects, err := s.defectsFor(ctx, q, id)
	if err != nil {
		return domain.InspectionDetail{}, err
	}
	chars, err := s.characteristics(ctx, q, insp.PlanID)
	if err != nil {
		return domain.InspectionDetail{}, err
	}
	needs := make([]qc.Need, len(chars))
	for i, ch := range chars {
		needs[i] = qc.Need{ID: ch.ID, Code: ch.Code, Type: ch.CharacteristicType}
	}
	readings := make([]qc.Reading, len(results))
	for i, r := range results {
		readings[i] = qc.Reading{CharacteristicID: r.CharacteristicID, SampleIndex: r.SampleIndex}
	}
	return domain.InspectionDetail{
		Inspection:             insp,
		Results:                results,
		Defects:                defects,
		MissingCharacteristics: qc.MissingCoverage(needs, readings, insp.SampleSize),
	}, nil
}

func (s *Store) resultsFor(ctx context.Context, q querier, inspectionID uuid.UUID) ([]domain.InspectionResult, error) {
	rows, err := q.Query(ctx, `SELECT `+resultCols+` FROM inspection_results WHERE inspection_id = $1 ORDER BY characteristic_code, sample_index`, inspectionID)
	if err != nil {
		return nil, mapErr(err)
	}
	return collect(rows, scanResult)
}

func (s *Store) defectsFor(ctx context.Context, q querier, inspectionID uuid.UUID) ([]domain.InspectionDefect, error) {
	rows, err := q.Query(ctx, `SELECT `+inspDefectCols+` FROM inspection_defects WHERE inspection_id = $1 ORDER BY created_at`, inspectionID)
	if err != nil {
		return nil, mapErr(err)
	}
	return collect(rows, scanInspDefect)
}

func (s *Store) CreateInspection(ctx context.Context, actor domain.Actor, in domain.NewInspection) (domain.InspectionDetail, error) {
	in.Notes = domain.Trim(in.Notes)
	if err := domain.CheckOptional(in.Notes, "notes", 8000); err != nil {
		return domain.InspectionDetail{}, wrapInvalid(err)
	}
	if in.SampleSize != nil && (*in.SampleSize < 1 || *in.SampleSize > 500) {
		return domain.InspectionDetail{}, invalid("sample size must be from 1 to 500")
	}
	var out domain.InspectionDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		lot, err := scanLot(tx.QueryRow(ctx, `SELECT `+lotCols+` FROM lots WHERE id = $1 FOR UPDATE`, in.LotID))
		if err != nil {
			return mapErr(err)
		}
		if lot.Status == domain.LotClosed {
			return invalid("reopen the lot before inspecting it")
		}
		if lot.Status == domain.LotOnHold {
			return invalid("a supervisor must release the hold before this lot is inspected again")
		}
		var plan domain.InspectionPlan
		if in.PlanID != nil {
			plan, err = scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1`, *in.PlanID))
			if err != nil {
				if isNoRows(err) {
					return invalid("inspection plan does not exist")
				}
				return mapErr(err)
			}
		} else {
			plan, err = scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE product_id = $1 AND status = 'active'`, lot.ProductID))
			if err != nil {
				if isNoRows(err) {
					return invalid("no active inspection plan for this product")
				}
				return mapErr(err)
			}
		}
		if plan.Status != domain.PlanActive {
			return invalid("inspection plan is not active")
		}
		if plan.ProductID != lot.ProductID {
			return invalid("inspection plan is for a different product")
		}
		if in.StationID != nil {
			var siteID uuid.UUID
			var active bool
			err := tx.QueryRow(ctx, `SELECT site_id, active FROM stations WHERE id = $1`, *in.StationID).Scan(&siteID, &active)
			if err != nil {
				if isNoRows(err) {
					return invalid("station does not exist")
				}
				return mapErr(err)
			}
			if !active {
				return invalid("station is not active")
			}
			if siteID != lot.SiteID {
				return invalid("station is not at the lot's site")
			}
		}
		inspector := actor.ID
		if in.InspectorID != nil {
			inspector = *in.InspectorID
		}
		if inspector == uuid.Nil {
			return invalid("inspector is required")
		}
		if err := s.requireUser(ctx, tx, inspector, domain.RoleInspector); err != nil {
			return err
		}
		sample := 1
		if in.SampleSize != nil {
			sample = *in.SampleSize
		} else if plan.SampleSize != nil {
			sample = *plan.SampleSize
		}
		number, err := nextNumber(ctx, tx, "INS")
		if err != nil {
			return err
		}
		var createdBy any
		if actor.ID != uuid.Nil {
			createdBy = actor.ID
		}
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO inspections (id, inspection_number, lot_id, plan_id, station_id, inspector_id, sample_size, notes, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			id, number, lot.ID, plan.ID, in.StationID, inspector, sample, in.Notes, createdBy); err != nil {
			return mapErr(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE lots SET status = 'in_inspection' WHERE id = $1`, lot.ID); err != nil {
			return mapErr(err)
		}
		out, err = s.inspectionDetail(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "inspection.create", "inspection", id, "Started inspection "+number, nil, out.Inspection)
	})
	return out, err
}

func (s *Store) RecordResults(ctx context.Context, actor domain.Actor, inspectionID uuid.UUID, batch domain.ResultBatch) ([]domain.InspectionResult, error) {
	if len(batch.Results) == 0 {
		return nil, invalid("at least one result is required")
	}
	if len(batch.Results) > 500 {
		return nil, invalid("too many results in one request")
	}
	var out []domain.InspectionResult
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		insp, err := s.lockInspection(ctx, tx, inspectionID)
		if err != nil {
			return err
		}
		if insp.Status != domain.InspInProgress {
			return invalid("results can only be recorded on an in-progress inspection")
		}
		seen := map[string]bool{}
		out = make([]domain.InspectionResult, 0, len(batch.Results))
		for _, in := range batch.Results {
			index := in.SampleIndex
			if index == 0 {
				index = 1
			}
			if index < 1 || index > insp.SampleSize {
				return invalid("sample index is outside this inspection's sample size")
			}
			key := in.CharacteristicID.String() + "#" + itoa(index)
			if seen[key] {
				return invalid("duplicate sample in the batch")
			}
			seen[key] = true
			ch, err := scanChar(tx.QueryRow(ctx, `SELECT `+charCols+` FROM characteristics WHERE id = $1 AND plan_id = $2`, in.CharacteristicID, insp.PlanID))
			if err != nil {
				if isNoRows(err) {
					return invalid("characteristic is not on this inspection plan")
				}
				return mapErr(err)
			}
			notes := domain.Trim(in.Notes)
			if err := domain.CheckOptional(notes, "notes", 8000); err != nil {
				return wrapInvalid(err)
			}
			var attribute *string
			if in.AttributeResult != nil {
				v := strings.ToLower(domain.Trim(*in.AttributeResult))
				attribute = &v
			}
			inSpec, err := qc.InSpec(ch.CharacteristicType, in.MeasuredValue, attribute, ch.LowerLimit, ch.UpperLimit)
			if err != nil {
				return invalid(ch.Code + ": " + err.Error())
			}
			var warning bool
			if in.GaugeID != nil {
				var active bool
				var due *time.Time
				err := tx.QueryRow(ctx, `SELECT active, calibration_due FROM gauges WHERE id = $1`, *in.GaugeID).Scan(&active, &due)
				if err != nil {
					if isNoRows(err) {
						return invalid("gauge does not exist")
					}
					return mapErr(err)
				}
				if !active {
					return invalid("gauge is not active")
				}
				today := time.Now().UTC().Truncate(24 * time.Hour)
				warning = due != nil && due.Before(today)
			}
			var recordedBy any
			if actor.ID != uuid.Nil {
				recordedBy = actor.ID
			}
			row, err := scanResult(tx.QueryRow(ctx, `
				INSERT INTO inspection_results (
					id, inspection_id, characteristic_id, sample_index, measured_value, attribute_result,
					in_spec, calibration_warning, is_critical, nominal, lower_limit, upper_limit, unit,
					characteristic_code, characteristic_name, characteristic_type, gauge_id, notes, recorded_by)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
				ON CONFLICT (inspection_id, characteristic_id, sample_index) DO UPDATE SET
					measured_value = EXCLUDED.measured_value,
					attribute_result = EXCLUDED.attribute_result,
					in_spec = EXCLUDED.in_spec,
					calibration_warning = EXCLUDED.calibration_warning,
					is_critical = EXCLUDED.is_critical,
					nominal = EXCLUDED.nominal,
					lower_limit = EXCLUDED.lower_limit,
					upper_limit = EXCLUDED.upper_limit,
					unit = EXCLUDED.unit,
					characteristic_code = EXCLUDED.characteristic_code,
					characteristic_name = EXCLUDED.characteristic_name,
					characteristic_type = EXCLUDED.characteristic_type,
					gauge_id = EXCLUDED.gauge_id,
					notes = EXCLUDED.notes,
					recorded_by = EXCLUDED.recorded_by,
					recorded_at = now()
				RETURNING `+resultCols,
				uuid.New(), inspectionID, ch.ID, index, in.MeasuredValue, attribute, inSpec, warning, ch.Critical,
				ch.Nominal, ch.LowerLimit, ch.UpperLimit, ch.Unit, ch.Code, ch.Name, ch.CharacteristicType, in.GaugeID, notes, recordedBy))
			if err != nil {
				return mapErr(err)
			}
			out = append(out, row)
		}
		return s.writeAudit(ctx, tx, actor, "inspection.record", "inspection", inspectionID, "Recorded results", nil, map[string]int{"count": len(out)})
	})
	return out, err
}

func (s *Store) DeleteResult(ctx context.Context, actor domain.Actor, id uuid.UUID) error {
	return s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var inspectionID uuid.UUID
		err := tx.QueryRow(ctx, `SELECT inspection_id FROM inspection_results WHERE id = $1`, id).Scan(&inspectionID)
		if err != nil {
			return mapErr(err)
		}
		insp, err := s.lockInspection(ctx, tx, inspectionID)
		if err != nil {
			return err
		}
		if insp.Status != domain.InspInProgress {
			return invalid("results can only be removed from an in-progress inspection")
		}
		tag, err := tx.Exec(ctx, `DELETE FROM inspection_results WHERE id = $1`, id)
		if err != nil {
			return mapErr(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return s.writeAudit(ctx, tx, actor, "inspection_result.delete", "inspection_result", id, "Removed a result from "+insp.InspectionNumber, nil, nil)
	})
}

func (s *Store) AddInspectionDefect(ctx context.Context, actor domain.Actor, inspectionID uuid.UUID, in domain.NewInspectionDefect) (domain.InspectionDefect, error) {
	in.Notes = domain.Trim(in.Notes)
	if in.Quantity < 1 || in.Quantity > 1000000 {
		return domain.InspectionDefect{}, invalid("quantity must be at least 1")
	}
	if err := domain.CheckOptional(in.Notes, "notes", 8000); err != nil {
		return domain.InspectionDefect{}, wrapInvalid(err)
	}
	var out domain.InspectionDefect
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		insp, err := s.lockInspection(ctx, tx, inspectionID)
		if err != nil {
			return err
		}
		if insp.Status != domain.InspInProgress {
			return invalid("defects can only be added to an in-progress inspection")
		}
		code, err := scanDefect(tx.QueryRow(ctx, `SELECT `+defectCols+` FROM defect_codes WHERE id = $1`, in.DefectCodeID))
		if err != nil {
			if isNoRows(err) {
				return invalid("defect code does not exist")
			}
			return mapErr(err)
		}
		if !code.Active {
			return invalid("defect code is not active")
		}
		var recordedBy any
		if actor.ID != uuid.Nil {
			recordedBy = actor.ID
		}
		out, err = scanInspDefect(tx.QueryRow(ctx, `
			INSERT INTO inspection_defects (id, inspection_id, defect_code_id, code, name, severity, quantity, notes, recorded_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+inspDefectCols,
			uuid.New(), inspectionID, code.ID, code.Code, code.Name, code.Severity, in.Quantity, in.Notes, recordedBy))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "inspection_defect.create", "inspection_defect", out.ID, "Recorded defect "+out.Code, nil, out)
	})
	return out, err
}

func (s *Store) DeleteInspectionDefect(ctx context.Context, actor domain.Actor, id uuid.UUID) error {
	return s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanInspDefect(tx.QueryRow(ctx, `SELECT `+inspDefectCols+` FROM inspection_defects WHERE id = $1`, id))
		if err != nil {
			return mapErr(err)
		}
		insp, err := s.lockInspection(ctx, tx, cur.InspectionID)
		if err != nil {
			return err
		}
		if insp.Status != domain.InspInProgress {
			return invalid("defects can only be removed from an in-progress inspection")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM inspection_defects WHERE id = $1`, id); err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "inspection_defect.delete", "inspection_defect", id, "Removed defect "+cur.Code, cur, nil)
	})
}

func (s *Store) CompleteInspection(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.CompleteInspection) (domain.CompletedInspection, error) {
	var out domain.CompletedInspection
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var lotID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT lot_id FROM inspections WHERE id = $1`, id).Scan(&lotID); err != nil {
			return mapErr(err)
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM lots WHERE id = $1 FOR UPDATE`, lotID); err != nil {
			return mapErr(err)
		}
		insp, err := s.lockInspection(ctx, tx, id)
		if err != nil {
			return err
		}
		if insp.Status != domain.InspInProgress {
			return invalid("only an in-progress inspection can be completed")
		}
		detail, err := s.inspectionDetail(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(detail.MissingCharacteristics) > 0 {
			return invalid("missing results for: " + strings.Join(detail.MissingCharacteristics, ", "))
		}
		qtyInsp, qtyAcc, qtyRej := insp.QuantityInspected, insp.QuantityAccepted, insp.QuantityRejected
		if in.QuantityInspected != nil || in.QuantityAccepted != nil || in.QuantityRejected != nil {
			if in.QuantityInspected == nil || in.QuantityAccepted == nil || in.QuantityRejected == nil {
				return invalid("quantity inspected, accepted, and rejected must be sent together")
			}
			if err := domain.CheckNonNegative(*in.QuantityInspected, "quantity_inspected"); err != nil {
				return wrapInvalid(err)
			}
			if err := domain.CheckNonNegative(*in.QuantityAccepted, "quantity_accepted"); err != nil {
				return wrapInvalid(err)
			}
			if err := domain.CheckNonNegative(*in.QuantityRejected, "quantity_rejected"); err != nil {
				return wrapInvalid(err)
			}
			if math.Abs((*in.QuantityAccepted+*in.QuantityRejected)-*in.QuantityInspected) > 0.000001 {
				return invalid("accepted and rejected quantities must add up to the inspected quantity")
			}
			qtyInsp, qtyAcc, qtyRej = *in.QuantityInspected, *in.QuantityAccepted, *in.QuantityRejected
		}
		samples := make([]qc.Sample, len(detail.Results))
		for i, r := range detail.Results {
			samples[i] = qc.Sample{InSpec: r.InSpec, Critical: r.Critical}
		}
		defects := make([]qc.Defect, len(detail.Defects))
		for i, d := range detail.Defects {
			defects[i] = qc.Defect{Severity: d.Severity, Quantity: d.Quantity}
		}
		decision, err := qc.Evaluate(samples, defects, qtyRej)
		if err != nil {
			return invalid(err.Error())
		}
		notes := insp.Notes
		if extra := domain.Trim(in.Notes); extra != "" {
			if err := domain.CheckOptional(extra, "notes", 8000); err != nil {
				return wrapInvalid(err)
			}
			if notes != "" {
				notes += "\n"
			}
			notes += extra
		}
		now := time.Now().UTC()
		updated, err := scanInspection(tx.QueryRow(ctx, `
			UPDATE inspections SET status = 'completed', result = $2, quantity_inspected = $3, quantity_accepted = $4,
				quantity_rejected = $5, notes = $6, completed_at = $7
			WHERE id = $1 RETURNING `+inspectionCols, id, decision.Result, qtyInsp, qtyAcc, qtyRej, notes, now))
		if err != nil {
			return mapErr(err)
		}
		if err := s.syncLot(ctx, tx, lotID); err != nil {
			return err
		}
		open := decision.Result == domain.ResultFail
		if in.OpenNCR != nil {
			open = *in.OpenNCR
		}
		out.Inspection = updated
		if open {
			sev := decision.Severity
			if sev == "" {
				sev = domain.SeverityMinor
			}
			desc := domain.Trim(in.Notes)
			if desc == "" {
				desc = "Inspection " + updated.InspectionNumber + " result is " + decision.Result + "."
			}
			qty := qtyRej
			if qty == 0 {
				qty = qtyInsp
			}
			lot, err := scanLot(tx.QueryRow(ctx, `SELECT `+lotCols+` FROM lots WHERE id = $1`, lotID))
			if err != nil {
				return mapErr(err)
			}
			ncr, err := s.insertNCR(ctx, tx, actor, domain.NewNonconformance{
				InspectionID:     &id,
				LotID:            &lotID,
				ProductID:        lot.ProductID,
				Title:            "Inspection " + updated.InspectionNumber,
				Description:      desc,
				Severity:         sev,
				QuantityAffected: qty,
			})
			if err != nil {
				return err
			}
			out.Nonconformance = &ncr
		}
		return s.writeAudit(ctx, tx, actor, "inspection.complete", "inspection", id, "Completed inspection "+updated.InspectionNumber+" as "+decision.Result, insp, updated)
	})
	return out, err
}

func (s *Store) CancelInspection(ctx context.Context, actor domain.Actor, id uuid.UUID) (domain.Inspection, error) {
	var out domain.Inspection
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var lotID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT lot_id FROM inspections WHERE id = $1`, id).Scan(&lotID); err != nil {
			return mapErr(err)
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM lots WHERE id = $1 FOR UPDATE`, lotID); err != nil {
			return mapErr(err)
		}
		insp, err := s.lockInspection(ctx, tx, id)
		if err != nil {
			return err
		}
		if insp.Status != domain.InspInProgress {
			return invalid("only an in-progress inspection can be cancelled")
		}
		out, err = scanInspection(tx.QueryRow(ctx, `UPDATE inspections SET status = 'cancelled' WHERE id = $1 RETURNING `+inspectionCols, id))
		if err != nil {
			return mapErr(err)
		}
		if err := s.syncLot(ctx, tx, lotID); err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "inspection.cancel", "inspection", id, "Cancelled inspection "+out.InspectionNumber, insp, out)
	})
	return out, err
}

func (s *Store) lockInspection(ctx context.Context, tx pgx.Tx, id uuid.UUID) (domain.Inspection, error) {
	insp, err := scanInspection(tx.QueryRow(ctx, `SELECT `+inspectionCols+` FROM inspections WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return domain.Inspection{}, mapErr(err)
	}
	return insp, nil
}

func (s *Store) syncLot(ctx context.Context, tx pgx.Tx, lotID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM lots WHERE id = $1 FOR UPDATE`, lotID); err != nil {
		return mapErr(err)
	}
	var inProgress int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM inspections WHERE lot_id = $1 AND status = 'in_progress'`, lotID).Scan(&inProgress); err != nil {
		return mapErr(err)
	}
	next := domain.LotOpen
	if inProgress > 0 {
		next = domain.LotInInspection
	} else {
		var result string
		err := tx.QueryRow(ctx, `
			SELECT result FROM inspections
			WHERE lot_id = $1 AND status = 'completed'
			ORDER BY completed_at DESC LIMIT 1`, lotID).Scan(&result)
		if err != nil {
			if !isNoRows(err) {
				return mapErr(err)
			}
		} else {
			switch result {
			case domain.ResultFail:
				next = domain.LotRejected
			case domain.ResultConditional:
				next = domain.LotOnHold
			default:
				next = domain.LotAccepted
			}
		}
	}
	_, err := tx.Exec(ctx, `UPDATE lots SET status = $2 WHERE id = $1`, lotID, next)
	return mapErr(err)
}
