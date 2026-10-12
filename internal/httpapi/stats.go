package httpapi

import (
	"net/http"
	"time"

	"quality_control/internal/store"
)

func (s *Server) statsFilter(w http.ResponseWriter, r *http.Request) (store.StatsFilter, bool) {
	from, ok := queryTime(w, r, "from")
	if !ok {
		return store.StatsFilter{}, false
	}
	to, ok := queryTime(w, r, "to")
	if !ok {
		return store.StatsFilter{}, false
	}
	product, ok := queryUUID(w, r, "product_id")
	if !ok {
		return store.StatsFilter{}, false
	}
	site, ok := queryUUID(w, r, "site_id")
	if !ok {
		return store.StatsFilter{}, false
	}
	inspector, ok := queryUUID(w, r, "inspector_id")
	if !ok {
		return store.StatsFilter{}, false
	}
	limit, _, ok := queryPage(w, r)
	if !ok {
		return store.StatsFilter{}, false
	}
	if from != nil {
		end := time.Now()
		if to != nil {
			end = *to
		}
		if !end.After(*from) {
			writeError(w, http.StatusBadRequest, "invalid", "The end date must be after the start date.")
			return store.StatsFilter{}, false
		}
		if end.Sub(*from) > 367*24*time.Hour {
			writeError(w, http.StatusBadRequest, "invalid", "Pick a date range of one year or less.")
			return store.StatsFilter{}, false
		}
	}
	return store.StatsFilter{From: from, To: to, ProductID: product, SiteID: site, Inspector: inspector, Limit: limit}, true
}

func (s *Server) statsDefectRates(w http.ResponseWriter, r *http.Request) {
	f, ok := s.statsFilter(w, r)
	if !ok {
		return
	}
	group := r.URL.Query().Get("group_by")
	if group == "" {
		group = "product"
	}
	var (
		rows any
		err  error
	)
	switch group {
	case "product":
		rows, err = s.store.DefectRatesByProduct(r.Context(), f)
	case "site":
		rows, err = s.store.DefectRatesBySite(r.Context(), f)
	case "inspector":
		rows, err = s.store.DefectRatesByInspector(r.Context(), f)
	default:
		writeError(w, http.StatusBadRequest, "invalid", "group_by must be product, site, or inspector.")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *Server) statsNCRAging(w http.ResponseWriter, r *http.Request) {
	limit, _, ok := queryPage(w, r)
	if !ok {
		return
	}
	v, err := s.store.NCRAging(r.Context(), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, v)
}

func (s *Server) statsCAPAAging(w http.ResponseWriter, r *http.Request) {
	limit, _, ok := queryPage(w, r)
	if !ok {
		return
	}
	v, err := s.store.CAPAAging(r.Context(), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, v)
}

func (s *Server) statsTrends(w http.ResponseWriter, r *http.Request) {
	f, ok := s.statsFilter(w, r)
	if !ok {
		return
	}
	v, err := s.store.Trends(r.Context(), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, v)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	limit, _, ok := queryPage(w, r)
	if !ok {
		return
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.store.SearchRecords(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *Server) listRequestAudit(w http.ResponseWriter, r *http.Request) {
	actor, ok := queryUUID(w, r, "actor_id")
	if !ok {
		return
	}
	from, ok := queryTime(w, r, "from")
	if !ok {
		return
	}
	to, ok := queryTime(w, r, "to")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListRequestAudit(r.Context(), store.RequestAuditFilter{
		ActorID: actor, Method: r.URL.Query().Get("method"), Path: r.URL.Query().Get("path"),
		From: from, To: to, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}
