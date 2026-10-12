package store

import (
	"context"

	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const planCols = `id, product_id, revision, name, status, sampling_type, aql, sample_size, notes, created_by, approved_by, approved_at, created_at, updated_at`

func scanPlan(row scannable) (domain.InspectionPlan, error) {
	var v domain.InspectionPlan
	var createdBy *uuid.UUID
	err := row.Scan(&v.ID, &v.ProductID, &v.Revision, &v.Name, &v.Status, &v.SamplingType, &v.AQL, &v.SampleSize, &v.Notes, &createdBy, &v.ApprovedBy, &v.ApprovedAt, &v.CreatedAt, &v.UpdatedAt)
	v.CreatedBy = uuidOrNil(createdBy)
	return v, err
}

const charCols = `id, plan_id, sequence, code, name, characteristic_type, method, nominal, lower_limit, upper_limit, unit, critical, created_at, updated_at`

func scanChar(row scannable) (domain.Characteristic, error) {
	var v domain.Characteristic
	err := row.Scan(&v.ID, &v.PlanID, &v.Sequence, &v.Code, &v.Name, &v.CharacteristicType, &v.Method, &v.Nominal, &v.LowerLimit, &v.UpperLimit, &v.Unit, &v.Critical, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (s *Store) ListPlans(ctx context.Context, productID *uuid.UUID, status, query string, limit, offset int) ([]domain.InspectionPlan, int, error) {
	query, err := cleanQuery(query)
	if err != nil {
		return nil, 0, err
	}
	if status != "" {
		if err := domain.CheckOneOf(status, "status", []string{domain.PlanDraft, domain.PlanActive, domain.PlanObsolete}); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	limit, offset = clamp(limit, offset, 200)
	var c clause
	if productID != nil {
		c.add("product_id = ?", *productID)
	}
	if status != "" {
		c.add("status = ?", status)
	}
	if query != "" {
		like := likeContains(query)
		c.add("(revision ILIKE ? ESCAPE '\\' OR name ILIKE ? ESCAPE '\\')", like, like)
	}
	return listRows(ctx, s.pool, "inspection_plans", planCols, "created_at DESC, id DESC", c, limit, offset, scanPlan)
}

func (s *Store) GetPlan(ctx context.Context, id uuid.UUID) (domain.PlanDetail, error) {
	return s.planDetail(ctx, s.pool, id)
}

func (s *Store) planDetail(ctx context.Context, q querier, id uuid.UUID) (domain.PlanDetail, error) {
	plan, err := scanPlan(q.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1`, id))
	if err != nil {
		return domain.PlanDetail{}, mapErr(err)
	}
	chars, err := s.characteristics(ctx, q, id)
	if err != nil {
		return domain.PlanDetail{}, err
	}
	return domain.PlanDetail{InspectionPlan: plan, Characteristics: chars}, nil
}

func (s *Store) characteristics(ctx context.Context, q querier, planID uuid.UUID) ([]domain.Characteristic, error) {
	rows, err := q.Query(ctx, `SELECT `+charCols+` FROM characteristics WHERE plan_id = $1 ORDER BY sequence`, planID)
	if err != nil {
		return nil, mapErr(err)
	}
	return collect(rows, scanChar)
}

func (s *Store) CreatePlan(ctx context.Context, actor domain.Actor, in domain.NewPlan) (domain.PlanDetail, error) {
	if err := cleanPlanHeader(&in.Revision, &in.Name, &in.SamplingType, &in.AQL, &in.Notes, in.SampleSize); err != nil {
		return domain.PlanDetail{}, err
	}
	for i := range in.Characteristics {
		if err := cleanCharacteristic(&in.Characteristics[i]); err != nil {
			return domain.PlanDetail{}, err
		}
	}
	var out domain.PlanDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.requireActiveRow(ctx, tx, "products", in.ProductID, "product"); err != nil {
			return err
		}
		id := uuid.New()
		var createdBy any
		if actor.ID != uuid.Nil {
			createdBy = actor.ID
		}
		if _, err := scanPlan(tx.QueryRow(ctx, `
			INSERT INTO inspection_plans (id, product_id, revision, name, sampling_type, aql, sample_size, notes, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+planCols,
			id, in.ProductID, in.Revision, in.Name, in.SamplingType, in.AQL, in.SampleSize, in.Notes, createdBy)); err != nil {
			return mapErr(err)
		}
		for _, ch := range in.Characteristics {
			if err := insertCharacteristic(ctx, tx, id, ch); err != nil {
				return err
			}
		}
		detail, err := s.planDetail(ctx, tx, id)
		if err != nil {
			return err
		}
		out = detail
		return s.writeAudit(ctx, tx, actor, "plan.create", "inspection_plan", id, "Created plan "+detail.Revision, nil, detail.InspectionPlan)
	})
	return out, err
}

func (s *Store) UpdatePlan(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchPlan) (domain.PlanDetail, error) {
	var out domain.PlanDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if cur.Status != domain.PlanDraft && !actor.IsAdmin() {
			return invalid("only a draft plan can be edited")
		}
		if in.ClearSample && in.SampleSize != nil {
			return invalid("send sample_size or clear_sample_size, not both")
		}
		name, sampling, aql, notes := cur.Name, cur.SamplingType, cur.AQL, cur.Notes
		sample := cur.SampleSize
		if in.Name != nil {
			name = *in.Name
		}
		if in.SamplingType != nil {
			sampling = *in.SamplingType
		}
		if in.AQL != nil {
			aql = *in.AQL
		}
		if in.Notes != nil {
			notes = *in.Notes
		}
		if in.ClearSample {
			sample = nil
		} else if in.SampleSize != nil {
			sample = in.SampleSize
		}
		revision := cur.Revision
		if err := cleanPlanHeader(&revision, &name, &sampling, &aql, &notes, sample); err != nil {
			return err
		}
		var set setter
		if in.Name != nil {
			set.add("name", name)
		}
		if in.SamplingType != nil {
			set.add("sampling_type", sampling)
		}
		if in.AQL != nil {
			set.add("aql", aql)
		}
		if in.Notes != nil {
			set.add("notes", notes)
		}
		if in.ClearSample || in.SampleSize != nil {
			set.add("sample_size", sample)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		if _, err := updateReturning(ctx, tx, "inspection_plans", planCols, set, id, scanPlan); err != nil {
			return err
		}
		out, err = s.planDetail(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "plan.update", "inspection_plan", id, "Updated plan "+out.Revision, cur, out.InspectionPlan)
	})
	return out, err
}

func (s *Store) AddCharacteristic(ctx context.Context, actor domain.Actor, planID uuid.UUID, in domain.NewCharacteristic) (domain.Characteristic, error) {
	if err := cleanCharacteristic(&in); err != nil {
		return domain.Characteristic{}, err
	}
	var out domain.Characteristic
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		plan, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1 FOR UPDATE`, planID))
		if err != nil {
			return mapErr(err)
		}
		if plan.Status != domain.PlanDraft {
			return invalid("characteristics can only be added to a draft plan")
		}
		id := uuid.New()
		out, err = scanChar(tx.QueryRow(ctx, `
			INSERT INTO characteristics (id, plan_id, sequence, code, name, characteristic_type, method, nominal, lower_limit, upper_limit, unit, critical)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+charCols,
			id, planID, in.Sequence, in.Code, in.Name, in.CharacteristicType, in.Method, in.Nominal, in.LowerLimit, in.UpperLimit, in.Unit, in.Critical))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "characteristic.create", "characteristic", out.ID, "Added characteristic "+out.Code, nil, out)
	})
	return out, err
}

func (s *Store) UpdateCharacteristic(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchCharacteristic) (domain.Characteristic, error) {
	var out domain.Characteristic
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanChar(tx.QueryRow(ctx, `SELECT `+charCols+` FROM characteristics WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		plan, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1`, cur.PlanID))
		if err != nil {
			return mapErr(err)
		}
		if plan.Status != domain.PlanDraft && !actor.IsAdmin() {
			return invalid("characteristics can only be edited on a draft plan")
		}
		next := domain.NewCharacteristic{
			Sequence: cur.Sequence, Code: cur.Code, Name: cur.Name, CharacteristicType: cur.CharacteristicType,
			Method: cur.Method, Nominal: cur.Nominal, LowerLimit: cur.LowerLimit, UpperLimit: cur.UpperLimit,
			Unit: cur.Unit, Critical: cur.Critical,
		}
		if in.Sequence != nil {
			next.Sequence = *in.Sequence
		}
		if in.Code != nil {
			next.Code = *in.Code
		}
		if in.Name != nil {
			next.Name = *in.Name
		}
		if in.CharacteristicType != nil {
			if plan.Status != domain.PlanDraft && *in.CharacteristicType != cur.CharacteristicType {
				return invalid("a check on an active plan can't switch between measure and pass or fail; make a new revision")
			}
			next.CharacteristicType = *in.CharacteristicType
		}
		if in.Method != nil {
			next.Method = *in.Method
		}
		if in.ClearNominal {
			next.Nominal = nil
		} else if in.Nominal != nil {
			next.Nominal = in.Nominal
		}
		if in.ClearLower {
			next.LowerLimit = nil
		} else if in.LowerLimit != nil {
			next.LowerLimit = in.LowerLimit
		}
		if in.ClearUpper {
			next.UpperLimit = nil
		} else if in.UpperLimit != nil {
			next.UpperLimit = in.UpperLimit
		}
		if in.Unit != nil {
			next.Unit = *in.Unit
		}
		if in.Critical != nil {
			next.Critical = *in.Critical
		}
		if err := cleanCharacteristic(&next); err != nil {
			return err
		}
		out, err = scanChar(tx.QueryRow(ctx, `
			UPDATE characteristics SET sequence=$2, code=$3, name=$4, characteristic_type=$5, method=$6,
				nominal=$7, lower_limit=$8, upper_limit=$9, unit=$10, critical=$11
			WHERE id=$1 RETURNING `+charCols,
			id, next.Sequence, next.Code, next.Name, next.CharacteristicType, next.Method, next.Nominal, next.LowerLimit, next.UpperLimit, next.Unit, next.Critical))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "characteristic.update", "characteristic", id, "Updated characteristic "+out.Code, cur, out)
	})
	return out, err
}

func (s *Store) DeleteCharacteristic(ctx context.Context, actor domain.Actor, id uuid.UUID) error {
	return s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanChar(tx.QueryRow(ctx, `SELECT `+charCols+` FROM characteristics WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM inspection_plans WHERE id = $1`, cur.PlanID).Scan(&status); err != nil {
			return mapErr(err)
		}
		if status != domain.PlanDraft {
			return invalid("characteristics can only be removed from a draft plan")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM characteristics WHERE id = $1`, id); err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "characteristic.delete", "characteristic", id, "Removed characteristic "+cur.Code, cur, nil)
	})
}

func (s *Store) DeletePlan(ctx context.Context, actor domain.Actor, id uuid.UUID) error {
	return s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if cur.Status != domain.PlanDraft {
			return invalid("only a draft plan can be deleted")
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM inspections WHERE plan_id = $1`, id).Scan(&n); err != nil {
			return mapErr(err)
		}
		if n > 0 {
			return invalid("a plan that has been used by an inspection cannot be deleted")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM inspection_plans WHERE id = $1`, id); err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "plan.delete", "inspection_plan", id, "Deleted draft plan "+cur.Revision, cur, nil)
	})
}

func (s *Store) ActivatePlan(ctx context.Context, actor domain.Actor, id uuid.UUID) (domain.PlanDetail, error) {
	var out domain.PlanDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if cur.Status != domain.PlanDraft {
			return invalid("only a draft plan can be activated")
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM characteristics WHERE plan_id = $1`, id).Scan(&n); err != nil {
			return mapErr(err)
		}
		if n == 0 {
			return invalid("add at least one characteristic before activating")
		}
		if err := s.requireActiveRow(ctx, tx, "products", cur.ProductID, "product"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE inspection_plans SET status = 'obsolete' WHERE product_id = $1 AND status = 'active'`, cur.ProductID); err != nil {
			return mapErr(err)
		}
		var approver any
		if actor.ID != uuid.Nil {
			approver = actor.ID
		}
		if _, err := tx.Exec(ctx, `UPDATE inspection_plans SET status = 'active', approved_by = $2, approved_at = now() WHERE id = $1`, id, approver); err != nil {
			return mapErr(err)
		}
		out, err = s.planDetail(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "plan.activate", "inspection_plan", id, "Activated plan "+out.Revision, cur, out.InspectionPlan)
	})
	return out, err
}

func (s *Store) ObsoletePlan(ctx context.Context, actor domain.Actor, id uuid.UUID) (domain.PlanDetail, error) {
	var out domain.PlanDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		if cur.Status != domain.PlanActive {
			return invalid("only an active plan can be marked obsolete")
		}
		if _, err := tx.Exec(ctx, `UPDATE inspection_plans SET status = 'obsolete' WHERE id = $1`, id); err != nil {
			return mapErr(err)
		}
		out, err = s.planDetail(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "plan.obsolete", "inspection_plan", id, "Obsoleted plan "+out.Revision, cur, out.InspectionPlan)
	})
	return out, err
}

func (s *Store) RevisePlan(ctx context.Context, actor domain.Actor, id uuid.UUID, revision string) (domain.PlanDetail, error) {
	revision = domain.Trim(revision)
	if err := domain.CheckLabel(revision, "revision", 40); err != nil {
		return domain.PlanDetail{}, wrapInvalid(err)
	}
	var out domain.PlanDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM inspection_plans WHERE id = $1`, id))
		if err != nil {
			return mapErr(err)
		}
		newID := uuid.New()
		var createdBy any
		if actor.ID != uuid.Nil {
			createdBy = actor.ID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO inspection_plans (id, product_id, revision, name, status, sampling_type, aql, sample_size, notes, created_by)
			VALUES ($1,$2,$3,$4,'draft',$5,$6,$7,$8,$9)`,
			newID, cur.ProductID, revision, cur.Name, cur.SamplingType, cur.AQL, cur.SampleSize, cur.Notes, createdBy); err != nil {
			return mapErr(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO characteristics (id, plan_id, sequence, code, name, characteristic_type, method, nominal, lower_limit, upper_limit, unit, critical)
			SELECT gen_random_uuid(), $2, sequence, code, name, characteristic_type, method, nominal, lower_limit, upper_limit, unit, critical
			FROM characteristics WHERE plan_id = $1`, id, newID); err != nil {
			return mapErr(err)
		}
		out, err = s.planDetail(ctx, tx, newID)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "plan.revise", "inspection_plan", newID, "Revised plan "+cur.Revision+" as "+revision, nil, out.InspectionPlan)
	})
	return out, err
}

func insertCharacteristic(ctx context.Context, q querier, planID uuid.UUID, in domain.NewCharacteristic) error {
	_, err := q.Exec(ctx, `
		INSERT INTO characteristics (id, plan_id, sequence, code, name, characteristic_type, method, nominal, lower_limit, upper_limit, unit, critical)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		uuid.New(), planID, in.Sequence, in.Code, in.Name, in.CharacteristicType, in.Method, in.Nominal, in.LowerLimit, in.UpperLimit, in.Unit, in.Critical)
	return mapErr(err)
}

func cleanPlanHeader(revision, name, sampling, aql, notes *string, sample *int) error {
	*revision, *name = domain.Trim(*revision), domain.Trim(*name)
	*sampling, *aql, *notes = domain.Trim(*sampling), domain.Trim(*aql), domain.Trim(*notes)
	if *sampling == "" {
		*sampling = domain.SampleHundred
	}
	if err := domain.CheckLabel(*revision, "revision", 40); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckLabel(*name, "name", 200); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOneOf(*sampling, "sampling_type", []string{domain.SampleHundred, domain.SampleFixed, domain.SampleAQL}); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOptional(*aql, "aql", 40); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOptional(*notes, "notes", 8000); err != nil {
		return wrapInvalid(err)
	}
	if sample != nil && (*sample < 1 || *sample > 500) {
		return invalid("sample size must be from 1 to 500")
	}
	return nil
}

func cleanCharacteristic(in *domain.NewCharacteristic) error {
	in.Code, in.Name = domain.Trim(in.Code), domain.Trim(in.Name)
	in.CharacteristicType, in.Method, in.Unit = domain.Trim(in.CharacteristicType), domain.Trim(in.Method), domain.Trim(in.Unit)
	if in.Sequence < 1 || in.Sequence > 10000 {
		return invalid("sequence must be from 1 to 10000")
	}
	if err := domain.CheckLabel(in.Code, "code", 40); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.Name, "name", 200); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOneOf(in.CharacteristicType, "characteristic_type", []string{domain.CharVariable, domain.CharAttribute}); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Method, "method", 200); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Unit, "unit", 40); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckFinite(in.Nominal, "nominal"); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckFinite(in.LowerLimit, "lower_limit"); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckFinite(in.UpperLimit, "upper_limit"); err != nil {
		return wrapInvalid(err)
	}
	if in.LowerLimit != nil && in.UpperLimit != nil && *in.LowerLimit > *in.UpperLimit {
		return invalid("lower limit cannot be above the upper limit")
	}
	return nil
}
