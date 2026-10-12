package httpapi

import (
	"net/http"

	"quality_control/internal/domain"
	"quality_control/internal/store"
)

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	product, ok := queryUUID(w, r, "product_id")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListPlans(r.Context(), product, r.URL.Query().Get("status"), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	var in domain.NewPlan
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreatePlan(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/plans/", v.ID, v, err)
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetPlan(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchPlan
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdatePlan(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) deletePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeletePlan(r.Context(), s.actor(r), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) activatePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.ActivatePlan(r.Context(), s.actor(r), id)
	s.one(w, r, v, err)
}

func (s *Server) obsoletePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.ObsoletePlan(r.Context(), s.actor(r), id)
	s.one(w, r, v, err)
}

func (s *Server) revisePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.RevisePlan
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.RevisePlan(r.Context(), s.actor(r), id, in.Revision)
	s.created(w, r, "/api/v1/plans/", v.ID, v, err)
}

func (s *Server) addCharacteristic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.NewCharacteristic
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.AddCharacteristic(r.Context(), s.actor(r), id, in)
	s.created(w, r, "/api/v1/characteristics/", v.ID, v, err)
}

func (s *Server) updateCharacteristic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchCharacteristic
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateCharacteristic(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) deleteCharacteristic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteCharacteristic(r.Context(), s.actor(r), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listLots(w http.ResponseWriter, r *http.Request) {
	product, ok := queryUUID(w, r, "product_id")
	if !ok {
		return
	}
	site, ok := queryUUID(w, r, "site_id")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListLots(r.Context(), store.LotFilter{
		Query: r.URL.Query().Get("q"), Status: r.URL.Query().Get("status"), Source: r.URL.Query().Get("source"),
		ProductID: product, SiteID: site, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createLot(w http.ResponseWriter, r *http.Request) {
	var in domain.NewLot
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateLot(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/lots/", v.ID, v, err)
}

func (s *Server) getLot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetLot(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateLot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchLot
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateLot(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) setLotStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.LotStatusChange
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.SetLotStatus(r.Context(), s.actor(r), id, in.Status)
	if err == nil {
		s.enqueueEvent(domain.EventLotStatusChanged, map[string]any{
			"lot_id":     v.ID,
			"lot_number": v.LotNumber,
			"status":     v.Status,
		})
	}
	s.one(w, r, v, err)
}

func (s *Server) listInspections(w http.ResponseWriter, r *http.Request) {
	lot, ok := queryUUID(w, r, "lot_id")
	if !ok {
		return
	}
	inspector, ok := queryUUID(w, r, "inspector_id")
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
	rows, total, err := s.store.ListInspections(r.Context(), store.InspectionFilter{
		Status: r.URL.Query().Get("status"), Result: r.URL.Query().Get("result"),
		LotID: lot, InspectorID: inspector, From: from, To: to, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createInspection(w http.ResponseWriter, r *http.Request) {
	var in domain.NewInspection
	if !decode(w, r, &in) {
		return
	}
	p, _ := principal(r)
	if in.InspectorID != nil && *in.InspectorID != p.ID && !domain.AtLeast(p.Role, domain.RoleSupervisor) {
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to that action.")
		return
	}
	v, err := s.store.CreateInspection(r.Context(), p.Actor(), in)
	s.created(w, r, "/api/v1/inspections/", v.ID, v, err)
}

func (s *Server) getInspection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetInspection(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) recordResults(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ResultBatch
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.RecordResults(r.Context(), s.actor(r), id, in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, v)
}

func (s *Server) deleteResult(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteResult(r.Context(), s.actor(r), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addDefect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.NewInspectionDefect
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.AddInspectionDefect(r.Context(), s.actor(r), id, in)
	s.created(w, r, "/api/v1/inspection-defects/", v.ID, v, err)
}

func (s *Server) deleteDefect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteInspectionDefect(r.Context(), s.actor(r), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) completeInspection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.CompleteInspection
	if !decodeOptional(w, r, &in) {
		return
	}
	v, err := s.store.CompleteInspection(r.Context(), s.actor(r), id, in)
	if err == nil {
		s.enqueueEvent(domain.EventInspectionCompleted, map[string]any{
			"inspection_id":     v.Inspection.ID,
			"inspection_number": v.Inspection.InspectionNumber,
			"lot_id":            v.Inspection.LotID,
			"result":            v.Inspection.Result,
		})
		if v.Nonconformance != nil {
			s.enqueueEvent(domain.EventNCRCreated, ncrEvent(*v.Nonconformance))
		}
	}
	s.one(w, r, v, err)
}

func (s *Server) cancelInspection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.CancelInspection(r.Context(), s.actor(r), id)
	s.one(w, r, v, err)
}
