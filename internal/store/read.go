package store

import (
	"context"
	"math"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

const attachmentCols = `id, entity_type, entity_id, file_name, content_type, size_bytes, sha256, uploaded_by, created_at`

func scanAttachment(row scannable) (domain.Attachment, error) {
	var v domain.Attachment
	var uploaded *uuid.UUID
	err := row.Scan(&v.ID, &v.EntityType, &v.EntityID, &v.FileName, &v.ContentType, &v.SizeBytes, &v.SHA256, &uploaded, &v.CreatedAt)
	v.UploadedBy = uuidOrNil(uploaded)
	return v, err
}

// Ping checks that PostgreSQL answers.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// InsertAttachment stores file metadata. The bytes stay on disk under storageName.
func (s *Store) InsertAttachment(ctx context.Context, actor domain.Actor, a domain.Attachment, storageName string) (domain.Attachment, error) {
	if _, ok := attachmentTable(a.EntityType); !ok {
		return domain.Attachment{}, invalid("entity type is not valid")
	}
	exists, err := s.EntityExists(ctx, a.EntityType, a.EntityID)
	if err != nil {
		return domain.Attachment{}, err
	}
	if !exists {
		return domain.Attachment{}, invalid("the record this file belongs to does not exist")
	}
	var uploaded any
	if actor.ID != uuid.Nil {
		uploaded = actor.ID
	}
	out, err := scanAttachment(s.pool.QueryRow(ctx, `
		INSERT INTO attachments (id, entity_type, entity_id, file_name, content_type, size_bytes, sha256, storage_name, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+attachmentCols,
		a.ID, a.EntityType, a.EntityID, a.FileName, a.ContentType, a.SizeBytes, a.SHA256, storageName, uploaded))
	if err != nil {
		return domain.Attachment{}, mapErr(err)
	}
	return out, s.writeAudit(ctx, s.pool, actor, "attachment.create", "attachment", out.ID, "Attached "+out.FileName, nil, out)
}

// ListAttachments returns files for one parent record.
func (s *Store) ListAttachments(ctx context.Context, entityType string, entityID uuid.UUID) ([]domain.Attachment, error) {
	if _, ok := attachmentTable(entityType); !ok {
		return nil, invalid("entity type is not valid")
	}
	rows, err := s.pool.Query(ctx, `SELECT `+attachmentCols+` FROM attachments WHERE entity_type = $1 AND entity_id = $2 ORDER BY created_at`, entityType, entityID)
	if err != nil {
		return nil, mapErr(err)
	}
	return collect(rows, scanAttachment)
}

// GetAttachment returns metadata and the on-disk name.
func (s *Store) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, string, error) {
	var storage string
	row := s.pool.QueryRow(ctx, `SELECT `+attachmentCols+`, storage_name FROM attachments WHERE id = $1`, id)
	var v domain.Attachment
	var uploaded *uuid.UUID
	err := row.Scan(&v.ID, &v.EntityType, &v.EntityID, &v.FileName, &v.ContentType, &v.SizeBytes, &v.SHA256, &uploaded, &v.CreatedAt, &storage)
	if err != nil {
		return domain.Attachment{}, "", mapErr(err)
	}
	v.UploadedBy = uuidOrNil(uploaded)
	return v, storage, nil
}

// DeleteAttachment removes the metadata row and returns the on-disk name.
func (s *Store) DeleteAttachment(ctx context.Context, actor domain.Actor, id uuid.UUID) (string, error) {
	a, storage, err := s.GetAttachment(ctx, id)
	if err != nil {
		return "", err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM attachments WHERE id = $1`, id)
	if err != nil {
		return "", mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	if err := s.writeAudit(ctx, s.pool, actor, "attachment.delete", "attachment", id, "Removed "+a.FileName, a, nil); err != nil {
		return storage, err
	}
	return storage, nil
}

// Dashboard returns the plant snapshot.
func (s *Store) Dashboard(ctx context.Context) (domain.Dashboard, error) {
	var d domain.Dashboard
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM inspections WHERE status = 'in_progress'),
			(SELECT count(*) FROM lots WHERE status = 'on_hold'),
			(SELECT count(*) FROM nonconformances WHERE status IN ('open', 'under_review', 'dispositioned')),
			(SELECT count(*) FROM corrective_actions WHERE status IN ('open', 'in_progress', 'verification')),
			(SELECT count(*) FROM corrective_actions WHERE status IN ('open', 'in_progress', 'verification') AND due_date < CURRENT_DATE),
			(SELECT count(*) FROM inspections WHERE status = 'completed' AND result = 'pass' AND completed_at >= now() - interval '30 days'),
			(SELECT count(*) FROM inspections WHERE status = 'completed' AND result = 'fail' AND completed_at >= now() - interval '30 days'),
			(SELECT count(*) FROM inspections WHERE status = 'completed' AND result = 'conditional' AND completed_at >= now() - interval '30 days')
	`).Scan(&d.OpenInspections, &d.LotsOnHold, &d.OpenNonconformances, &d.OpenCorrectiveActions, &d.OverdueCorrectiveActions, &d.PassLast30Days, &d.FailLast30Days, &d.ConditionalLast30Days)
	if err != nil {
		return domain.Dashboard{}, mapErr(err)
	}
	d.InspectionsLast30Days = d.PassLast30Days + d.FailLast30Days + d.ConditionalLast30Days
	if d.InspectionsLast30Days > 0 {
		rate := math.Round((float64(d.PassLast30Days)/float64(d.InspectionsLast30Days))*10000) / 10000
		d.PassRateLast30Days = &rate
	}
	rows, err := s.pool.Query(ctx, `
		SELECT d.severity, COALESCE(sum(d.quantity), 0)::int
		FROM inspection_defects d
		JOIN inspections i ON i.id = d.inspection_id
		WHERE i.status = 'completed' AND i.completed_at >= now() - interval '30 days'
		GROUP BY d.severity`)
	if err != nil {
		return domain.Dashboard{}, mapErr(err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var sev string
		var n int
		if err := rows.Scan(&sev, &n); err != nil {
			return domain.Dashboard{}, mapErr(err)
		}
		counts[sev] = n
	}
	if err := rows.Err(); err != nil {
		return domain.Dashboard{}, mapErr(err)
	}
	d.DefectsBySeverity = []domain.SeverityCount{
		{Severity: domain.SeverityMinor, Count: counts[domain.SeverityMinor]},
		{Severity: domain.SeverityMajor, Count: counts[domain.SeverityMajor]},
		{Severity: domain.SeverityCritical, Count: counts[domain.SeverityCritical]},
	}
	return d, nil
}

type MeasurementFilter struct {
	ProductID *uuid.UUID
	Code      string
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

// Measurements returns completed variable readings for charting.
func (s *Store) Measurements(ctx context.Context, f MeasurementFilter) ([]domain.Measurement, int, error) {
	f.Code = domain.Trim(f.Code)
	if f.ProductID == nil && f.Code == "" && f.From == nil {
		return nil, 0, invalid("filter by product_id, characteristic code, or from")
	}
	if f.Code != "" {
		if err := domain.CheckLabel(f.Code, "characteristic_code", 40); err != nil {
			return nil, 0, wrapInvalid(err)
		}
	}
	if f.Limit <= 0 {
		f.Limit = 500
	}
	if f.Limit > 2000 {
		f.Limit = 2000
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	var c clause
	c.add("i.status = 'completed'")
	c.add("r.characteristic_type = 'variable'")
	c.add("r.measured_value IS NOT NULL")
	if f.ProductID != nil {
		c.add("l.product_id = ?", *f.ProductID)
	}
	if f.Code != "" {
		c.add("lower(r.characteristic_code) = lower(?)", f.Code)
	}
	if f.From != nil {
		c.add("r.recorded_at >= ?", *f.From)
	}
	if f.To != nil {
		c.add("r.recorded_at < ?", *f.To)
	}
	from := ` FROM inspection_results r JOIN inspections i ON i.id = r.inspection_id JOIN lots l ON l.id = i.lot_id`
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+from+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, `
		SELECT r.recorded_at, l.lot_number, i.inspection_number, r.characteristic_code, r.characteristic_name,
			r.sample_index, r.measured_value, r.in_spec, r.unit, r.nominal, r.lower_limit, r.upper_limit, r.is_critical`+
		from+c.where()+` ORDER BY r.recorded_at ASC, r.sample_index ASC LIMIT $`+itoa(len(c.args)+1)+` OFFSET $`+itoa(len(c.args)+2), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := []domain.Measurement{}
	for rows.Next() {
		var m domain.Measurement
		if err := rows.Scan(&m.RecordedAt, &m.LotNumber, &m.InspectionNumber, &m.CharacteristicCode, &m.CharacteristicName, &m.SampleIndex, &m.MeasuredValue, &m.InSpec, &m.Unit, &m.Nominal, &m.LowerLimit, &m.UpperLimit, &m.Critical); err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, mapErr(err)
	}
	return out, total, nil
}

type AuditFilter struct {
	EntityType string
	EntityID   *uuid.UUID
	Limit      int
	Offset     int
}

// ListAudit returns the mutation log, newest first.
func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]domain.AuditEntry, int, error) {
	f.EntityType = domain.Trim(f.EntityType)
	if len(f.EntityType) > 80 {
		return nil, 0, invalid("entity type is too long")
	}
	f.Limit, f.Offset = clamp(f.Limit, f.Offset, 200)
	var c clause
	if f.EntityType != "" {
		c.add("entity_type = ?", f.EntityType)
	}
	if f.EntityID != nil {
		c.add("entity_id = ?", *f.EntityID)
	}
	cols := `id, actor_id, action, entity_type, entity_id, summary, before_data, after_data, ip, request_id, created_at`
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`+c.where(), c.args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args := append([]any{}, c.args...)
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, `SELECT `+cols+` FROM audit_log`+c.where()+` ORDER BY created_at DESC, id DESC LIMIT $`+itoa(len(c.args)+1)+` OFFSET $`+itoa(len(c.args)+2), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := []domain.AuditEntry{}
	for rows.Next() {
		var e domain.AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.Action, &e.EntityType, &e.EntityID, &e.Summary, &e.Before, &e.After, &e.IP, &e.RequestID, &e.CreatedAt); err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, mapErr(err)
	}
	return out, total, nil
}
