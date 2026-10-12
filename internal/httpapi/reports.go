package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"quality_control/internal/report"
	"quality_control/internal/store"
)

func (s *Server) reportFilter(w http.ResponseWriter, r *http.Request) (store.ReportFilter, bool) {
	from, ok := queryTime(w, r, "from")
	if !ok {
		return store.ReportFilter{}, false
	}
	to, ok := queryTime(w, r, "to")
	if !ok {
		return store.ReportFilter{}, false
	}
	product, ok := queryUUID(w, r, "product_id")
	if !ok {
		return store.ReportFilter{}, false
	}
	site, ok := queryUUID(w, r, "site_id")
	if !ok {
		return store.ReportFilter{}, false
	}
	limit := 5000
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 20000 {
			writeError(w, http.StatusBadRequest, "invalid", "limit is outside the allowed range.")
			return store.ReportFilter{}, false
		}
		limit = n
	}
	return store.ReportFilter{
		From: from, To: to, ProductID: product, SiteID: site,
		Status: r.URL.Query().Get("status"), Limit: limit,
	}, true
}

func (s *Server) reportInspectionsCSV(w http.ResponseWriter, r *http.Request) {
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	rows, err := s.store.ExportInspections(r.Context(), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	data := make([][]string, 0, len(rows))
	for _, row := range rows {
		data = append(data, []string{
			row.InspectionNumber, row.LotNumber, row.PartNumber, row.SiteCode, row.Inspector,
			row.Status, row.Result, report.FormatInt(row.SampleSize),
			report.FormatFloat(row.QtyInspected), report.FormatFloat(row.QtyAccepted), report.FormatFloat(row.QtyRejected),
			report.FormatTime(row.StartedAt), report.FormatPtrTime(row.CompletedAt),
		})
	}
	s.writeCSV(w, report.Filename("inspections", "csv"), []string{
		"inspection_number", "lot_number", "part_number", "site_code", "inspector",
		"status", "result", "sample_size", "qty_inspected", "qty_accepted", "qty_rejected",
		"started_at", "completed_at",
	}, data)
}

func (s *Server) reportNCRSCSV(w http.ResponseWriter, r *http.Request) {
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	rows, err := s.store.ExportNCRs(r.Context(), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	data := make([][]string, 0, len(rows))
	for _, row := range rows {
		data = append(data, []string{
			row.NCRNumber, row.Title, row.Severity, row.Status, row.Disposition,
			row.PartNumber, row.LotNumber, report.FormatTime(row.OpenedAt), report.FormatPtrTime(row.ClosedAt),
		})
	}
	s.writeCSV(w, report.Filename("nonconformances", "csv"), []string{
		"ncr_number", "title", "severity", "status", "disposition", "part_number", "lot_number", "opened_at", "closed_at",
	}, data)
}

func (s *Server) reportCAPAsCSV(w http.ResponseWriter, r *http.Request) {
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	rows, err := s.store.ExportCAPAs(r.Context(), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	data := make([][]string, 0, len(rows))
	for _, row := range rows {
		due := ""
		if row.DueDate != nil {
			due = *row.DueDate
		}
		data = append(data, []string{
			row.CANumber, row.Title, row.Status, row.NCRNumber, row.Owner, due,
			report.FormatTime(row.OpenedAt), report.FormatPtrTime(row.ClosedAt), row.RootCause, row.ActionPlan,
		})
	}
	s.writeCSV(w, report.Filename("corrective-actions", "csv"), []string{
		"ca_number", "title", "status", "ncr_number", "owner", "due_date", "opened_at", "closed_at", "root_cause", "action_plan",
	}, data)
}

func (s *Server) reportInspectionsPDF(w http.ResponseWriter, r *http.Request) {
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	rows, err := s.store.ExportInspections(r.Context(), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	doc := report.NewPDF("Inspection report")
	doc.AddHeading("Summary")
	doc.AddKV("Rows", fmt.Sprintf("%d", len(rows)))
	doc.AddHeading("Inspections")
	for i, row := range rows {
		if i >= 80 {
			doc.AddLine(fmt.Sprintf("... and %d more", len(rows)-80))
			break
		}
		doc.AddLine(fmt.Sprintf("%s  lot=%s  part=%s  result=%s  inspector=%s",
			row.InspectionNumber, row.LotNumber, row.PartNumber, row.Result, row.Inspector))
	}
	s.writePDF(w, report.Filename("inspections", "pdf"), doc.Bytes())
}

func (s *Server) reportSummaryPDF(w http.ResponseWriter, r *http.Request) {
	dash, err := s.store.Dashboard(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ncr, err := s.store.NCRAging(r.Context(), 10)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	capa, err := s.store.CAPAAging(r.Context(), 10)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	doc := report.NewPDF("Quality summary")
	doc.AddHeading("Plant snapshot")
	doc.AddKV("Open inspections", fmt.Sprintf("%d", dash.OpenInspections))
	doc.AddKV("Lots on hold", fmt.Sprintf("%d", dash.LotsOnHold))
	doc.AddKV("Open NCRs", fmt.Sprintf("%d", dash.OpenNonconformances))
	doc.AddKV("Open CAPAs", fmt.Sprintf("%d", dash.OpenCorrectiveActions))
	doc.AddKV("Overdue CAPAs", fmt.Sprintf("%d", dash.OverdueCorrectiveActions))
	doc.AddKV("Inspections last 30 days", fmt.Sprintf("%d", dash.InspectionsLast30Days))
	if dash.PassRateLast30Days != nil {
		doc.AddKV("Pass rate last 30 days", fmt.Sprintf("%.2f%%", *dash.PassRateLast30Days*100))
	}
	doc.AddHeading("Open NCR aging")
	for _, b := range ncr.Buckets {
		doc.AddKV(b.Bucket, fmt.Sprintf("%d", b.Count))
	}
	doc.AddHeading("Open CAPA aging")
	for _, b := range capa.Buckets {
		doc.AddKV(b.Bucket, fmt.Sprintf("%d", b.Count))
	}
	s.writePDF(w, report.Filename("quality-summary", "pdf"), doc.Bytes())
}

func (s *Server) writeCSV(w http.ResponseWriter, filename string, header []string, rows [][]string) {
	body, err := report.WriteCSV(header, rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not build CSV.")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) writePDF(w http.ResponseWriter, filename string, body []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
