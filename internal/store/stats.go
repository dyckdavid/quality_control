package store

import (
	"context"
	"math"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

// StatsFilter is a common date / dimension filter for analytics endpoints.
type StatsFilter struct {
	From      *time.Time
	To        *time.Time
	ProductID *uuid.UUID
	SiteID    *uuid.UUID
	Inspector *uuid.UUID
	Limit     int
}

func (f *StatsFilter) normalizeWindow() {
	now := time.Now().UTC()
	if f.To == nil {
		t := now
		f.To = &t
	}
	if f.From == nil {
		t := f.To.AddDate(0, 0, -30)
		f.From = &t
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// DefectRatesByProduct returns fail/defect rates grouped by product.
func (s *Store) DefectRatesByProduct(ctx context.Context, f StatsFilter) ([]domain.DefectRateRow, error) {
	f.normalizeWindow()
	rows, err := s.pool.Query(ctx, `
		SELECT p.part_number, p.name,
			count(i.id)::int AS inspections,
			count(*) FILTER (WHERE i.result = 'fail')::int AS failed,
			count(*) FILTER (WHERE i.result = 'conditional')::int AS conditional,
			COALESCE((
				SELECT sum(d.quantity)::int FROM inspection_defects d
				JOIN inspections ii ON ii.id = d.inspection_id
				JOIN lots ll ON ll.id = ii.lot_id
				WHERE ll.product_id = p.id AND ii.status = 'completed'
				  AND ii.completed_at >= $1 AND ii.completed_at < $2
			), 0) AS defects
		FROM products p
		LEFT JOIN lots l ON l.product_id = p.id
		LEFT JOIN inspections i ON i.lot_id = l.id AND i.status = 'completed'
			AND i.completed_at >= $1 AND i.completed_at < $2
		WHERE ($3::uuid IS NULL OR p.id = $3)
		GROUP BY p.id, p.part_number, p.name
		HAVING count(i.id) > 0
		ORDER BY failed DESC, inspections DESC
		LIMIT $4`,
		*f.From, *f.To, f.ProductID, f.Limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.DefectRateRow{}
	for rows.Next() {
		var r domain.DefectRateRow
		if err := rows.Scan(&r.Key, &r.Label, &r.Inspections, &r.Failed, &r.Conditional, &r.DefectQuantity); err != nil {
			return nil, mapErr(err)
		}
		if r.Inspections > 0 {
			r.FailRate = round4(float64(r.Failed) / float64(r.Inspections))
			r.DefectRate = round4(float64(r.DefectQuantity) / float64(r.Inspections))
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DefectRatesBySite returns fail/defect rates grouped by site.
func (s *Store) DefectRatesBySite(ctx context.Context, f StatsFilter) ([]domain.DefectRateRow, error) {
	f.normalizeWindow()
	rows, err := s.pool.Query(ctx, `
		SELECT s.code, s.name,
			count(i.id)::int,
			count(*) FILTER (WHERE i.result = 'fail')::int,
			count(*) FILTER (WHERE i.result = 'conditional')::int,
			COALESCE(sum(def.qty), 0)::int
		FROM sites s
		LEFT JOIN lots l ON l.site_id = s.id
		LEFT JOIN inspections i ON i.lot_id = l.id AND i.status = 'completed'
			AND i.completed_at >= $1 AND i.completed_at < $2
		LEFT JOIN LATERAL (
			SELECT sum(d.quantity) AS qty FROM inspection_defects d WHERE d.inspection_id = i.id
		) def ON true
		WHERE ($3::uuid IS NULL OR s.id = $3)
		GROUP BY s.id, s.code, s.name
		HAVING count(i.id) > 0
		ORDER BY 4 DESC, 3 DESC
		LIMIT $4`,
		*f.From, *f.To, f.SiteID, f.Limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.DefectRateRow{}
	for rows.Next() {
		var r domain.DefectRateRow
		if err := rows.Scan(&r.Key, &r.Label, &r.Inspections, &r.Failed, &r.Conditional, &r.DefectQuantity); err != nil {
			return nil, mapErr(err)
		}
		if r.Inspections > 0 {
			r.FailRate = round4(float64(r.Failed) / float64(r.Inspections))
			r.DefectRate = round4(float64(r.DefectQuantity) / float64(r.Inspections))
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DefectRatesByInspector returns fail rates grouped by inspector.
func (s *Store) DefectRatesByInspector(ctx context.Context, f StatsFilter) ([]domain.DefectRateRow, error) {
	f.normalizeWindow()
	rows, err := s.pool.Query(ctx, `
		SELECT u.username, u.full_name,
			count(i.id)::int,
			count(*) FILTER (WHERE i.result = 'fail')::int,
			count(*) FILTER (WHERE i.result = 'conditional')::int,
			COALESCE(sum(def.qty), 0)::int
		FROM users u
		JOIN inspections i ON i.inspector_id = u.id AND i.status = 'completed'
			AND i.completed_at >= $1 AND i.completed_at < $2
		LEFT JOIN LATERAL (
			SELECT sum(d.quantity) AS qty FROM inspection_defects d WHERE d.inspection_id = i.id
		) def ON true
		WHERE ($3::uuid IS NULL OR u.id = $3)
		GROUP BY u.id, u.username, u.full_name
		ORDER BY 4 DESC, 3 DESC
		LIMIT $4`,
		*f.From, *f.To, f.Inspector, f.Limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.DefectRateRow{}
	for rows.Next() {
		var r domain.DefectRateRow
		if err := rows.Scan(&r.Key, &r.Label, &r.Inspections, &r.Failed, &r.Conditional, &r.DefectQuantity); err != nil {
			return nil, mapErr(err)
		}
		if r.Inspections > 0 {
			r.FailRate = round4(float64(r.Failed) / float64(r.Inspections))
			r.DefectRate = round4(float64(r.DefectQuantity) / float64(r.Inspections))
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NCRAging returns open nonconformance aging buckets and items.
func (s *Store) NCRAging(ctx context.Context, limit int) (domain.AgingReport, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var report domain.AgingReport
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM nonconformances
		WHERE status IN ('open', 'under_review', 'dispositioned')`).Scan(&report.Total)
	if err != nil {
		return report, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT bucket, count(*)::int FROM (
			SELECT CASE
				WHEN opened_at >= now() - interval '7 days' THEN '0-7d'
				WHEN opened_at >= now() - interval '30 days' THEN '8-30d'
				WHEN opened_at >= now() - interval '90 days' THEN '31-90d'
				ELSE '90d+'
			END AS bucket
			FROM nonconformances
			WHERE status IN ('open', 'under_review', 'dispositioned')
		) t GROUP BY bucket`)
	if err != nil {
		return report, mapErr(err)
	}
	counts := map[string]int{"0-7d": 0, "8-30d": 0, "31-90d": 0, "90d+": 0}
	for rows.Next() {
		var b string
		var n int
		if err := rows.Scan(&b, &n); err != nil {
			rows.Close()
			return report, mapErr(err)
		}
		counts[b] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return report, mapErr(err)
	}
	report.Buckets = []domain.AgingBucket{
		{Bucket: "0-7d", Count: counts["0-7d"]},
		{Bucket: "8-30d", Count: counts["8-30d"]},
		{Bucket: "31-90d", Count: counts["31-90d"]},
		{Bucket: "90d+", Count: counts["90d+"]},
	}
	itemRows, err := s.pool.Query(ctx, `
		SELECT id, ncr_number, title, status, severity, opened_at,
			GREATEST(0, floor(extract(epoch FROM (now() - opened_at)) / 86400))::int AS age_days
		FROM nonconformances
		WHERE status IN ('open', 'under_review', 'dispositioned')
		ORDER BY opened_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return report, mapErr(err)
	}
	defer itemRows.Close()
	report.Items = []domain.AgingItem{}
	for itemRows.Next() {
		var it domain.AgingItem
		if err := itemRows.Scan(&it.ID, &it.Number, &it.Title, &it.Status, &it.Severity, &it.OpenedAt, &it.AgeDays); err != nil {
			return report, mapErr(err)
		}
		it.Overdue = it.AgeDays > 30
		report.Items = append(report.Items, it)
	}
	return report, itemRows.Err()
}

// CAPAAging returns corrective-action aging buckets and items.
func (s *Store) CAPAAging(ctx context.Context, limit int) (domain.AgingReport, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var report domain.AgingReport
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM corrective_actions
		WHERE status IN ('open', 'in_progress', 'verification')`).Scan(&report.Total)
	if err != nil {
		return report, mapErr(err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT bucket, count(*)::int FROM (
			SELECT CASE
				WHEN opened_at >= now() - interval '7 days' THEN '0-7d'
				WHEN opened_at >= now() - interval '30 days' THEN '8-30d'
				WHEN opened_at >= now() - interval '90 days' THEN '31-90d'
				ELSE '90d+'
			END AS bucket
			FROM corrective_actions
			WHERE status IN ('open', 'in_progress', 'verification')
		) t GROUP BY bucket`)
	if err != nil {
		return report, mapErr(err)
	}
	counts := map[string]int{"0-7d": 0, "8-30d": 0, "31-90d": 0, "90d+": 0}
	for rows.Next() {
		var b string
		var n int
		if err := rows.Scan(&b, &n); err != nil {
			rows.Close()
			return report, mapErr(err)
		}
		counts[b] = n
	}
	rows.Close()
	report.Buckets = []domain.AgingBucket{
		{Bucket: "0-7d", Count: counts["0-7d"]},
		{Bucket: "8-30d", Count: counts["8-30d"]},
		{Bucket: "31-90d", Count: counts["31-90d"]},
		{Bucket: "90d+", Count: counts["90d+"]},
	}
	itemRows, err := s.pool.Query(ctx, `
		SELECT id, ca_number, title, status, opened_at,
			GREATEST(0, floor(extract(epoch FROM (now() - opened_at)) / 86400))::int,
			to_char(due_date, 'YYYY-MM-DD'),
			(due_date IS NOT NULL AND due_date < CURRENT_DATE)
		FROM corrective_actions
		WHERE status IN ('open', 'in_progress', 'verification')
		ORDER BY COALESCE(due_date, '9999-12-31'::date) ASC, opened_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return report, mapErr(err)
	}
	defer itemRows.Close()
	report.Items = []domain.AgingItem{}
	for itemRows.Next() {
		var it domain.AgingItem
		var due *string
		if err := itemRows.Scan(&it.ID, &it.Number, &it.Title, &it.Status, &it.OpenedAt, &it.AgeDays, &due, &it.Overdue); err != nil {
			return report, mapErr(err)
		}
		it.DueDate = due
		report.Items = append(report.Items, it)
	}
	return report, itemRows.Err()
}

// Trends returns daily inspection / NCR / defect counts.
func (s *Store) Trends(ctx context.Context, f StatsFilter) (domain.TrendReport, error) {
	f.normalizeWindow()
	report := domain.TrendReport{From: *f.From, To: *f.To, Points: []domain.TrendPoint{}}
	rows, err := s.pool.Query(ctx, `
		WITH days AS (
			SELECT generate_series(date_trunc('day', $1::timestamptz), date_trunc('day', $2::timestamptz - interval '1 second'), interval '1 day') AS day
		)
		SELECT to_char(d.day, 'YYYY-MM-DD'),
			COALESCE(i.inspections, 0), COALESCE(i.pass, 0), COALESCE(i.fail, 0), COALESCE(i.conditional, 0),
			COALESCE(n.ncrs, 0), COALESCE(def.defects, 0)
		FROM days d
		LEFT JOIN LATERAL (
			SELECT count(*)::int AS inspections,
				count(*) FILTER (WHERE result = 'pass')::int AS pass,
				count(*) FILTER (WHERE result = 'fail')::int AS fail,
				count(*) FILTER (WHERE result = 'conditional')::int AS conditional
			FROM inspections insp
			JOIN lots l ON l.id = insp.lot_id
			WHERE insp.status = 'completed'
			  AND insp.completed_at >= d.day AND insp.completed_at < d.day + interval '1 day'
			  AND ($3::uuid IS NULL OR l.product_id = $3)
			  AND ($4::uuid IS NULL OR l.site_id = $4)
			  AND ($5::uuid IS NULL OR insp.inspector_id = $5)
		) i ON true
		LEFT JOIN LATERAL (
			SELECT count(*)::int AS ncrs FROM nonconformances ncr
			WHERE ncr.opened_at >= d.day AND ncr.opened_at < d.day + interval '1 day'
			  AND ($3::uuid IS NULL OR ncr.product_id = $3)
		) n ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(sum(dd.quantity), 0)::int AS defects
			FROM inspection_defects dd
			JOIN inspections insp ON insp.id = dd.inspection_id
			JOIN lots l ON l.id = insp.lot_id
			WHERE insp.status = 'completed'
			  AND insp.completed_at >= d.day AND insp.completed_at < d.day + interval '1 day'
			  AND ($3::uuid IS NULL OR l.product_id = $3)
			  AND ($4::uuid IS NULL OR l.site_id = $4)
			  AND ($5::uuid IS NULL OR insp.inspector_id = $5)
		) def ON true
		ORDER BY d.day`,
		*f.From, *f.To, f.ProductID, f.SiteID, f.Inspector)
	if err != nil {
		return report, mapErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.TrendPoint
		if err := rows.Scan(&p.Date, &p.Inspections, &p.Pass, &p.Fail, &p.Conditional, &p.NCROpened, &p.Defects); err != nil {
			return report, mapErr(err)
		}
		report.Points = append(report.Points, p)
	}
	return report, rows.Err()
}

// SearchRecords finds lots, inspections, NCRs, and CAPAs by free text.
func (s *Store) SearchRecords(ctx context.Context, q string, limit int) ([]domain.SearchHit, error) {
	q, err := cleanQuery(q)
	if err != nil {
		return nil, err
	}
	if q == "" {
		return nil, invalid("q is required")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	pattern := likeContains(q)
	rows, err := s.pool.Query(ctx, `
		(
			SELECT 'lot', id, lot_number, coalesce(notes, ''), status, lot_number || ' ' || coalesce(notes, ''), created_at
			FROM lots
			WHERE lot_number ILIKE $1 ESCAPE '\' OR notes ILIKE $1 ESCAPE '\' OR supplier_lot ILIKE $1 ESCAPE '\'
			ORDER BY created_at DESC LIMIT $2
		)
		UNION ALL
		(
			SELECT 'inspection', id, inspection_number, coalesce(notes, ''), status, inspection_number || ' ' || coalesce(notes, ''), created_at
			FROM inspections
			WHERE inspection_number ILIKE $1 ESCAPE '\' OR notes ILIKE $1 ESCAPE '\'
			ORDER BY created_at DESC LIMIT $2
		)
		UNION ALL
		(
			SELECT 'nonconformance', id, ncr_number, title, status, ncr_number || ' ' || title || ' ' || description, created_at
			FROM nonconformances
			WHERE ncr_number ILIKE $1 ESCAPE '\' OR title ILIKE $1 ESCAPE '\' OR description ILIKE $1 ESCAPE '\'
			ORDER BY created_at DESC LIMIT $2
		)
		UNION ALL
		(
			SELECT 'corrective_action', id, ca_number, title, status, ca_number || ' ' || title || ' ' || root_cause, created_at
			FROM corrective_actions
			WHERE ca_number ILIKE $1 ESCAPE '\' OR title ILIKE $1 ESCAPE '\' OR root_cause ILIKE $1 ESCAPE '\' OR action_plan ILIKE $1 ESCAPE '\'
			ORDER BY created_at DESC LIMIT $2
		)
		ORDER BY created_at DESC
		LIMIT $2`, pattern, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.SearchHit{}
	for rows.Next() {
		var h domain.SearchHit
		if err := rows.Scan(&h.EntityType, &h.ID, &h.Number, &h.Title, &h.Status, &h.Snippet, &h.CreatedAt); err != nil {
			return nil, mapErr(err)
		}
		h.Snippet = clip(h.Snippet, 160)
		out = append(out, h)
	}
	return out, rows.Err()
}
