package store

import (
	"context"
	"errors"
	"strings"

	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const checklistCols = `id, code, name, description, product_id, active, created_by, created_at, updated_at`
const checklistItemCols = `id, checklist_id, sequence, prompt, item_type, required, help_text, created_at, updated_at`
const checklistRunCols = `id, checklist_id, inspection_id, status, notes, started_by, completed_by, started_at, completed_at, created_at, updated_at`
const checklistRunItemCols = `id, run_id, item_id, sequence, prompt, item_type, required, result, numeric_value, notes, answered_by, answered_at, created_at, updated_at`

func scanChecklist(row scannable) (domain.Checklist, error) {
	var v domain.Checklist
	var product *uuid.UUID
	var created *uuid.UUID
	err := row.Scan(&v.ID, &v.Code, &v.Name, &v.Description, &product, &v.Active, &created, &v.CreatedAt, &v.UpdatedAt)
	v.ProductID = product
	v.CreatedBy = uuidOrNil(created)
	return v, err
}

func scanChecklistItem(row scannable) (domain.ChecklistItem, error) {
	var v domain.ChecklistItem
	err := row.Scan(&v.ID, &v.ChecklistID, &v.Sequence, &v.Prompt, &v.ItemType, &v.Required, &v.HelpText, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func scanChecklistRun(row scannable) (domain.ChecklistRun, error) {
	var v domain.ChecklistRun
	var started, completed *uuid.UUID
	err := row.Scan(&v.ID, &v.ChecklistID, &v.InspectionID, &v.Status, &v.Notes, &started, &completed, &v.StartedAt, &v.CompletedAt, &v.CreatedAt, &v.UpdatedAt)
	v.StartedBy = uuidOrNil(started)
	v.CompletedBy = completed
	return v, err
}

func scanChecklistRunItem(row scannable) (domain.ChecklistRunItem, error) {
	var v domain.ChecklistRunItem
	var answered *uuid.UUID
	err := row.Scan(&v.ID, &v.RunID, &v.ItemID, &v.Sequence, &v.Prompt, &v.ItemType, &v.Required, &v.Result, &v.NumericValue, &v.Notes, &answered, &v.AnsweredAt, &v.CreatedAt, &v.UpdatedAt)
	v.AnsweredBy = answered
	return v, err
}

// ListChecklists returns templates with optional filters.
func (s *Store) ListChecklists(ctx context.Context, query string, productID *uuid.UUID, active *bool, limit, offset int) ([]domain.Checklist, int, error) {
	query, err := cleanQuery(query)
	if err != nil {
		return nil, 0, err
	}
	limit, offset = clamp(limit, offset, 200)
	var c clause
	if query != "" {
		c.add("(code ILIKE ? ESCAPE '\\' OR name ILIKE ? ESCAPE '\\')", likeContains(query), likeContains(query))
	}
	if productID != nil {
		c.add("product_id = ?", *productID)
	}
	if active != nil {
		c.add("active = ?", *active)
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM checklists`+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `SELECT `+checklistCols+` FROM checklists`+c.where()+` ORDER BY code ASC LIMIT $`+itoa(len(c.args)+1)+` OFFSET $`+itoa(len(c.args)+2), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	out, err := collect(rows, scanChecklist)
	return out, total, err
}

// GetChecklist returns a template with items.
func (s *Store) GetChecklist(ctx context.Context, id uuid.UUID) (domain.ChecklistDetail, error) {
	v, err := scanChecklist(s.pool.QueryRow(ctx, `SELECT `+checklistCols+` FROM checklists WHERE id = $1`, id))
	if err != nil {
		return domain.ChecklistDetail{}, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT `+checklistItemCols+` FROM checklist_items WHERE checklist_id = $1 ORDER BY sequence`, id)
	if err != nil {
		return domain.ChecklistDetail{}, mapErr(err)
	}
	items, err := collect(rows, scanChecklistItem)
	if err != nil {
		return domain.ChecklistDetail{}, err
	}
	return domain.ChecklistDetail{Checklist: v, Items: items}, nil
}

// CreateChecklist inserts a template and optional items.
func (s *Store) CreateChecklist(ctx context.Context, actor domain.Actor, in domain.NewChecklist) (domain.ChecklistDetail, error) {
	in.Code = domain.Trim(in.Code)
	in.Name = domain.Trim(in.Name)
	in.Description = domain.Trim(in.Description)
	if err := domain.CheckLabel(in.Code, "code", 40); err != nil {
		return domain.ChecklistDetail{}, wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.Name, "name", 120); err != nil {
		return domain.ChecklistDetail{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Description, "description", 2000); err != nil {
		return domain.ChecklistDetail{}, wrapInvalid(err)
	}
	var out domain.ChecklistDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if in.ProductID != nil {
			if err := s.requireActiveRow(ctx, tx, "products", *in.ProductID, "product"); err != nil {
				return err
			}
		}
		id := uuid.New()
		var created any
		if actor.ID != uuid.Nil {
			created = actor.ID
		}
		cl, err := scanChecklist(tx.QueryRow(ctx, `
			INSERT INTO checklists (id, code, name, description, product_id, created_by)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+checklistCols, id, in.Code, in.Name, in.Description, in.ProductID, created))
		if err != nil {
			return mapErr(err)
		}
		items := make([]domain.ChecklistItem, 0, len(in.Items))
		for _, item := range in.Items {
			it, err := s.insertChecklistItem(ctx, tx, id, item)
			if err != nil {
				return err
			}
			items = append(items, it)
		}
		out = domain.ChecklistDetail{Checklist: cl, Items: items}
		return s.writeAudit(ctx, tx, actor, "checklist.create", "checklist", cl.ID, "Created checklist "+cl.Code, nil, out)
	})
	return out, err
}

func (s *Store) insertChecklistItem(ctx context.Context, q querier, checklistID uuid.UUID, in domain.NewChecklistItem) (domain.ChecklistItem, error) {
	in.Prompt = domain.Trim(in.Prompt)
	in.HelpText = domain.Trim(in.HelpText)
	in.ItemType = domain.Trim(in.ItemType)
	if in.ItemType == "" {
		in.ItemType = domain.ChecklistItemPassFail
	}
	if err := domain.CheckLabel(in.Prompt, "prompt", 500); err != nil {
		return domain.ChecklistItem{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.HelpText, "help_text", 1000); err != nil {
		return domain.ChecklistItem{}, wrapInvalid(err)
	}
	if err := domain.CheckOneOf(in.ItemType, "item_type", domain.ChecklistItemTypes()); err != nil {
		return domain.ChecklistItem{}, wrapInvalid(err)
	}
	if in.Sequence < 1 || in.Sequence > 10000 {
		return domain.ChecklistItem{}, invalid("sequence must be between 1 and 10000")
	}
	required := true
	if in.Required != nil {
		required = *in.Required
	}
	return scanChecklistItem(q.QueryRow(ctx, `
		INSERT INTO checklist_items (id, checklist_id, sequence, prompt, item_type, required, help_text)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+checklistItemCols,
		uuid.New(), checklistID, in.Sequence, in.Prompt, in.ItemType, required, in.HelpText))
}

// UpdateChecklist patches a template header.
func (s *Store) UpdateChecklist(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchChecklist) (domain.Checklist, error) {
	before, err := s.GetChecklist(ctx, id)
	if err != nil {
		return domain.Checklist{}, err
	}
	set := setter{}
	if in.Code != nil {
		code := domain.Trim(*in.Code)
		if err := domain.CheckLabel(code, "code", 40); err != nil {
			return domain.Checklist{}, wrapInvalid(err)
		}
		set.add("code", code)
	}
	if in.Name != nil {
		name := domain.Trim(*in.Name)
		if err := domain.CheckLabel(name, "name", 120); err != nil {
			return domain.Checklist{}, wrapInvalid(err)
		}
		set.add("name", name)
	}
	if in.Description != nil {
		desc := domain.Trim(*in.Description)
		if err := domain.CheckOptional(desc, "description", 2000); err != nil {
			return domain.Checklist{}, wrapInvalid(err)
		}
		set.add("description", desc)
	}
	if in.ProductID != nil {
		if *in.ProductID == uuid.Nil {
			set.add("product_id", nil)
		} else {
			if err := s.requireActiveRow(ctx, s.pool, "products", *in.ProductID, "product"); err != nil {
				return domain.Checklist{}, err
			}
			set.add("product_id", *in.ProductID)
		}
	}
	if in.Active != nil {
		set.add("active", *in.Active)
	}
	if set.empty() {
		return before.Checklist, nil
	}
	args := append(set.args, id)
	out, err := scanChecklist(s.pool.QueryRow(ctx, `UPDATE checklists SET `+strings.Join(set.sets, ", ")+` WHERE id = $`+itoa(len(args))+` RETURNING `+checklistCols, args...))
	if err != nil {
		return domain.Checklist{}, mapErr(err)
	}
	return out, s.writeAudit(ctx, s.pool, actor, "checklist.update", "checklist", id, "Updated checklist "+out.Code, before.Checklist, out)
}

// AddChecklistItem appends a step.
func (s *Store) AddChecklistItem(ctx context.Context, actor domain.Actor, checklistID uuid.UUID, in domain.NewChecklistItemBody) (domain.ChecklistItem, error) {
	if _, err := s.GetChecklist(ctx, checklistID); err != nil {
		return domain.ChecklistItem{}, err
	}
	item, err := s.insertChecklistItem(ctx, s.pool, checklistID, domain.NewChecklistItem{
		Sequence: in.Sequence, Prompt: in.Prompt, ItemType: in.ItemType, Required: in.Required, HelpText: in.HelpText,
	})
	if err != nil {
		return domain.ChecklistItem{}, mapErr(err)
	}
	return item, s.writeAudit(ctx, s.pool, actor, "checklist_item.create", "checklist", checklistID, "Added checklist item", nil, item)
}

// UpdateChecklistItem patches a step.
func (s *Store) UpdateChecklistItem(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchChecklistItem) (domain.ChecklistItem, error) {
	before, err := scanChecklistItem(s.pool.QueryRow(ctx, `SELECT `+checklistItemCols+` FROM checklist_items WHERE id = $1`, id))
	if err != nil {
		return domain.ChecklistItem{}, mapErr(err)
	}
	set := setter{}
	if in.Sequence != nil {
		if *in.Sequence < 1 || *in.Sequence > 10000 {
			return domain.ChecklistItem{}, invalid("sequence must be between 1 and 10000")
		}
		set.add("sequence", *in.Sequence)
	}
	if in.Prompt != nil {
		p := domain.Trim(*in.Prompt)
		if err := domain.CheckLabel(p, "prompt", 500); err != nil {
			return domain.ChecklistItem{}, wrapInvalid(err)
		}
		set.add("prompt", p)
	}
	if in.ItemType != nil {
		t := domain.Trim(*in.ItemType)
		if err := domain.CheckOneOf(t, "item_type", domain.ChecklistItemTypes()); err != nil {
			return domain.ChecklistItem{}, wrapInvalid(err)
		}
		set.add("item_type", t)
	}
	if in.Required != nil {
		set.add("required", *in.Required)
	}
	if in.HelpText != nil {
		h := domain.Trim(*in.HelpText)
		if err := domain.CheckOptional(h, "help_text", 1000); err != nil {
			return domain.ChecklistItem{}, wrapInvalid(err)
		}
		set.add("help_text", h)
	}
	if set.empty() {
		return before, nil
	}
	args := append(set.args, id)
	out, err := scanChecklistItem(s.pool.QueryRow(ctx, `UPDATE checklist_items SET `+strings.Join(set.sets, ", ")+` WHERE id = $`+itoa(len(args))+` RETURNING `+checklistItemCols, args...))
	if err != nil {
		return domain.ChecklistItem{}, mapErr(err)
	}
	return out, s.writeAudit(ctx, s.pool, actor, "checklist_item.update", "checklist_item", id, "Updated checklist item", before, out)
}

// DeleteChecklistItem removes a template step.
func (s *Store) DeleteChecklistItem(ctx context.Context, actor domain.Actor, id uuid.UUID) error {
	before, err := scanChecklistItem(s.pool.QueryRow(ctx, `SELECT `+checklistItemCols+` FROM checklist_items WHERE id = $1`, id))
	if err != nil {
		return mapErr(err)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM checklist_items WHERE id = $1`, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return s.writeAudit(ctx, s.pool, actor, "checklist_item.delete", "checklist_item", id, "Deleted checklist item", before, nil)
}

// StartChecklistRun begins a run for an inspection.
func (s *Store) StartChecklistRun(ctx context.Context, actor domain.Actor, inspectionID uuid.UUID, in domain.StartChecklistRun) (domain.ChecklistRunDetail, error) {
	in.Notes = domain.Trim(in.Notes)
	if err := domain.CheckOptional(in.Notes, "notes", 2000); err != nil {
		return domain.ChecklistRunDetail{}, wrapInvalid(err)
	}
	var out domain.ChecklistRunDetail
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var inspStatus string
		err := tx.QueryRow(ctx, `SELECT status FROM inspections WHERE id = $1`, inspectionID).Scan(&inspStatus)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return invalid("inspection does not exist")
			}
			return mapErr(err)
		}
		if inspStatus == domain.InspCancelled {
			return invalid("cannot start a checklist on a cancelled inspection")
		}
		cl, err := s.GetChecklist(ctx, in.ChecklistID)
		if err != nil {
			return err
		}
		if !cl.Active {
			return invalid("checklist is not active")
		}
		if len(cl.Items) == 0 {
			return invalid("checklist has no items")
		}
		runID := uuid.New()
		var started any
		if actor.ID != uuid.Nil {
			started = actor.ID
		}
		run, err := scanChecklistRun(tx.QueryRow(ctx, `
			INSERT INTO checklist_runs (id, checklist_id, inspection_id, notes, started_by)
			VALUES ($1,$2,$3,$4,$5) RETURNING `+checklistRunCols, runID, in.ChecklistID, inspectionID, in.Notes, started))
		if err != nil {
			return mapErr(err)
		}
		items := make([]domain.ChecklistRunItem, 0, len(cl.Items))
		for _, item := range cl.Items {
			ri, err := scanChecklistRunItem(tx.QueryRow(ctx, `
				INSERT INTO checklist_run_items (id, run_id, item_id, sequence, prompt, item_type, required)
				VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+checklistRunItemCols,
				uuid.New(), runID, item.ID, item.Sequence, item.Prompt, item.ItemType, item.Required))
			if err != nil {
				return mapErr(err)
			}
			items = append(items, ri)
		}
		out = domain.ChecklistRunDetail{ChecklistRun: run, Items: items}
		return s.writeAudit(ctx, tx, actor, "checklist_run.start", "checklist_run", runID, "Started checklist run", nil, out)
	})
	return out, err
}

// GetChecklistRun returns a run with answers.
func (s *Store) GetChecklistRun(ctx context.Context, id uuid.UUID) (domain.ChecklistRunDetail, error) {
	run, err := scanChecklistRun(s.pool.QueryRow(ctx, `SELECT `+checklistRunCols+` FROM checklist_runs WHERE id = $1`, id))
	if err != nil {
		return domain.ChecklistRunDetail{}, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT `+checklistRunItemCols+` FROM checklist_run_items WHERE run_id = $1 ORDER BY sequence`, id)
	if err != nil {
		return domain.ChecklistRunDetail{}, mapErr(err)
	}
	items, err := collect(rows, scanChecklistRunItem)
	if err != nil {
		return domain.ChecklistRunDetail{}, err
	}
	return domain.ChecklistRunDetail{ChecklistRun: run, Items: items}, nil
}

// ListChecklistRunsForInspection lists runs for one inspection.
func (s *Store) ListChecklistRunsForInspection(ctx context.Context, inspectionID uuid.UUID) ([]domain.ChecklistRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+checklistRunCols+` FROM checklist_runs WHERE inspection_id = $1 ORDER BY started_at DESC`, inspectionID)
	if err != nil {
		return nil, mapErr(err)
	}
	return collect(rows, scanChecklistRun)
}

// AnswerChecklistRunItem records one answer.
func (s *Store) AnswerChecklistRunItem(ctx context.Context, actor domain.Actor, runID, itemID uuid.UUID, in domain.AnswerChecklistItem) (domain.ChecklistRunItem, error) {
	run, err := s.GetChecklistRun(ctx, runID)
	if err != nil {
		return domain.ChecklistRunItem{}, err
	}
	if run.Status != domain.ChecklistRunInProgress {
		return domain.ChecklistRunItem{}, invalid("checklist run is not in progress")
	}
	var target *domain.ChecklistRunItem
	for i := range run.Items {
		if run.Items[i].ID == itemID {
			target = &run.Items[i]
			break
		}
	}
	if target == nil {
		return domain.ChecklistRunItem{}, ErrNotFound
	}
	in.Result = domain.Trim(in.Result)
	in.Notes = domain.Trim(in.Notes)
	if err := domain.CheckOptional(in.Notes, "notes", 1000); err != nil {
		return domain.ChecklistRunItem{}, wrapInvalid(err)
	}
	switch target.ItemType {
	case domain.ChecklistItemPassFail:
		if err := domain.CheckOneOf(in.Result, "result", []string{"pass", "fail", "na"}); err != nil {
			return domain.ChecklistRunItem{}, wrapInvalid(err)
		}
	case domain.ChecklistItemYesNo:
		if err := domain.CheckOneOf(in.Result, "result", []string{"yes", "no", "na"}); err != nil {
			return domain.ChecklistRunItem{}, wrapInvalid(err)
		}
	case domain.ChecklistItemText:
		if err := domain.CheckLabel(in.Result, "result", 1000); err != nil {
			return domain.ChecklistRunItem{}, wrapInvalid(err)
		}
	case domain.ChecklistItemNumber:
		if in.NumericValue == nil {
			return domain.ChecklistRunItem{}, invalid("numeric_value is required")
		}
		if err := domain.CheckFinite(in.NumericValue, "numeric_value"); err != nil {
			return domain.ChecklistRunItem{}, wrapInvalid(err)
		}
		in.Result = ""
	}
	var answered any
	if actor.ID != uuid.Nil {
		answered = actor.ID
	}
	out, err := scanChecklistRunItem(s.pool.QueryRow(ctx, `
		UPDATE checklist_run_items
		SET result = $1, numeric_value = $2, notes = $3, answered_by = $4, answered_at = now()
		WHERE id = $5 AND run_id = $6
		RETURNING `+checklistRunItemCols, in.Result, in.NumericValue, in.Notes, answered, itemID, runID))
	if err != nil {
		return domain.ChecklistRunItem{}, mapErr(err)
	}
	return out, s.writeAudit(ctx, s.pool, actor, "checklist_run.answer", "checklist_run_item", out.ID, "Answered checklist item", target, out)
}

// CompleteChecklistRun finishes a run when required items are answered.
func (s *Store) CompleteChecklistRun(ctx context.Context, actor domain.Actor, runID uuid.UUID) (domain.ChecklistRunDetail, error) {
	detail, err := s.GetChecklistRun(ctx, runID)
	if err != nil {
		return domain.ChecklistRunDetail{}, err
	}
	if detail.Status != domain.ChecklistRunInProgress {
		return domain.ChecklistRunDetail{}, invalid("checklist run is not in progress")
	}
	for _, item := range detail.Items {
		if item.Required && item.AnsweredAt == nil {
			return domain.ChecklistRunDetail{}, invalid("required checklist items are unanswered")
		}
	}
	var completed any
	if actor.ID != uuid.Nil {
		completed = actor.ID
	}
	run, err := scanChecklistRun(s.pool.QueryRow(ctx, `
		UPDATE checklist_runs SET status = 'completed', completed_by = $1, completed_at = now()
		WHERE id = $2 RETURNING `+checklistRunCols, completed, runID))
	if err != nil {
		return domain.ChecklistRunDetail{}, mapErr(err)
	}
	out := domain.ChecklistRunDetail{ChecklistRun: run, Items: detail.Items}
	if err := s.writeAudit(ctx, s.pool, actor, "checklist_run.complete", "checklist_run", runID, "Completed checklist run", detail.ChecklistRun, run); err != nil {
		return out, err
	}
	return out, nil
}
