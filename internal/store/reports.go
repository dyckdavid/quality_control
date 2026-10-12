package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ReportInspectionRow is one inspection export line.
type ReportInspectionRow struct {
	InspectionNumber string
	LotNumber        string
	PartNumber       string
	SiteCode         string
	Inspector        string
	Status           string
	Result           string
	SampleSize       int
	QtyInspected     float64
	QtyAccepted      float64
	QtyRejected      float64
	StartedAt        time.Time
	CompletedAt      *time.Time
}

// ReportNCRRow is one NCR export line.
type ReportNCRRow struct {
	NCRNumber   string
	Title       string
	Severity    string
	Status      string
	Disposition string
	PartNumber  string
	LotNumber   string
	OpenedAt    time.Time
	ClosedAt    *time.Time
}

// ReportCAPARow is one CAPA export line.
type ReportCAPARow struct {
	CANumber    string
	Title       string
	Status      string
	NCRNumber   string
	Owner       string
	DueDate     *string
	OpenedAt    time.Time
	ClosedAt    *time.Time
	RootCause   string
	ActionPlan  string
}

// ReportFilter limits export queries.
type ReportFilter struct {
	From      *time.Time
	To        *time.Time
	ProductID *uuid.UUID
	SiteID    *uuid.UUID
	Status    string
	Limit     int
}

func (f *ReportFilter) clamp() {
	if f.Limit <= 0 {
		f.Limit = 5000
	}
	if f.Limit > 20000 {
		f.Limit = 20000
	}
}

// ExportInspections returns rows for CSV/PDF inspection reports.
func (s *Store) ExportInspections(ctx context.Context, f ReportFilter) ([]ReportInspectionRow, error) {
	f.clamp()
	var c clause
	c.add("1=1")
	if f.From != nil {
		c.add("i.created_at >= ?", *f.From)
	}
	if f.To != nil {
		c.add("i.created_at < ?", *f.To)
	}
	if f.ProductID != nil {
		c.add("l.product_id = ?", *f.ProductID)
	}
	if f.SiteID != nil {
		c.add("l.site_id = ?", *f.SiteID)
	}
	if f.Status != "" {
		c.add("i.status = ?", f.Status)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT i.inspection_number, l.lot_number, p.part_number, si.code, u.username,
			i.status, i.result, i.sample_size, i.quantity_inspected, i.quantity_accepted, i.quantity_rejected,
			i.started_at, i.completed_at
		FROM inspections i
		JOIN lots l ON l.id = i.lot_id
		JOIN products p ON p.id = l.product_id
		JOIN sites si ON si.id = l.site_id
		JOIN users u ON u.id = i.inspector_id
		`+c.where()+`
		ORDER BY i.created_at DESC
		LIMIT $`+itoa(len(c.args)+1), append(c.args, f.Limit)...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []ReportInspectionRow{}
	for rows.Next() {
		var r ReportInspectionRow
		if err := rows.Scan(&r.InspectionNumber, &r.LotNumber, &r.PartNumber, &r.SiteCode, &r.Inspector,
			&r.Status, &r.Result, &r.SampleSize, &r.QtyInspected, &r.QtyAccepted, &r.QtyRejected,
			&r.StartedAt, &r.CompletedAt); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExportNCRs returns rows for NCR reports.
func (s *Store) ExportNCRs(ctx context.Context, f ReportFilter) ([]ReportNCRRow, error) {
	f.clamp()
	var c clause
	c.add("1=1")
	if f.From != nil {
		c.add("n.opened_at >= ?", *f.From)
	}
	if f.To != nil {
		c.add("n.opened_at < ?", *f.To)
	}
	if f.ProductID != nil {
		c.add("n.product_id = ?", *f.ProductID)
	}
	if f.Status != "" {
		c.add("n.status = ?", f.Status)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT n.ncr_number, n.title, n.severity, n.status, n.disposition, p.part_number,
			COALESCE(l.lot_number, ''), n.opened_at, n.closed_at
		FROM nonconformances n
		JOIN products p ON p.id = n.product_id
		LEFT JOIN lots l ON l.id = n.lot_id
		`+c.where()+`
		ORDER BY n.opened_at DESC
		LIMIT $`+itoa(len(c.args)+1), append(c.args, f.Limit)...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []ReportNCRRow{}
	for rows.Next() {
		var r ReportNCRRow
		if err := rows.Scan(&r.NCRNumber, &r.Title, &r.Severity, &r.Status, &r.Disposition, &r.PartNumber, &r.LotNumber, &r.OpenedAt, &r.ClosedAt); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExportCAPAs returns rows for CAPA reports.
func (s *Store) ExportCAPAs(ctx context.Context, f ReportFilter) ([]ReportCAPARow, error) {
	f.clamp()
	var c clause
	c.add("1=1")
	if f.From != nil {
		c.add("c.opened_at >= ?", *f.From)
	}
	if f.To != nil {
		c.add("c.opened_at < ?", *f.To)
	}
	if f.Status != "" {
		c.add("c.status = ?", f.Status)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.ca_number, c.title, c.status, COALESCE(n.ncr_number, ''), COALESCE(u.username, ''),
			to_char(c.due_date, 'YYYY-MM-DD'), c.opened_at, c.closed_at, c.root_cause, c.action_plan
		FROM corrective_actions c
		LEFT JOIN nonconformances n ON n.id = c.nonconformance_id
		LEFT JOIN users u ON u.id = c.owner_id
		`+c.where()+`
		ORDER BY c.opened_at DESC
		LIMIT $`+itoa(len(c.args)+1), append(c.args, f.Limit)...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []ReportCAPARow{}
	for rows.Next() {
		var r ReportCAPARow
		if err := rows.Scan(&r.CANumber, &r.Title, &r.Status, &r.NCRNumber, &r.Owner, &r.DueDate, &r.OpenedAt, &r.ClosedAt, &r.RootCause, &r.ActionPlan); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
