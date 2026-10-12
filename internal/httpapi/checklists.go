package httpapi

import (
	"net/http"

	"quality_control/internal/domain"
)

func (s *Server) listChecklists(w http.ResponseWriter, r *http.Request) {
	product, ok := queryUUID(w, r, "product_id")
	if !ok {
		return
	}
	active, ok := queryBool(w, r, "active")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListChecklists(r.Context(), r.URL.Query().Get("q"), product, active, limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createChecklist(w http.ResponseWriter, r *http.Request) {
	var in domain.NewChecklist
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateChecklist(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/checklists/", v.ID, v, err)
}

func (s *Server) getChecklist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetChecklist(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateChecklist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchChecklist
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateChecklist(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) addChecklistItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.NewChecklistItemBody
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.AddChecklistItem(r.Context(), s.actor(r), id, in)
	s.created(w, r, "/api/v1/checklist-items/", v.ID, v, err)
}

func (s *Server) updateChecklistItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchChecklistItem
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateChecklistItem(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) deleteChecklistItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteChecklistItem(r.Context(), s.actor(r), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listInspectionChecklistRuns(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	rows, err := s.store.ListChecklistRunsForInspection(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *Server) startInspectionChecklist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.StartChecklistRun
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.StartChecklistRun(r.Context(), s.actor(r), id, in)
	s.created(w, r, "/api/v1/checklist-runs/", v.ID, v, err)
}

func (s *Server) getChecklistRun(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetChecklistRun(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) answerChecklistRunItem(w http.ResponseWriter, r *http.Request) {
	runID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	itemID, ok := pathID(w, r, "itemID")
	if !ok {
		return
	}
	var in domain.AnswerChecklistItem
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.AnswerChecklistRunItem(r.Context(), s.actor(r), runID, itemID, in)
	s.one(w, r, v, err)
}

func (s *Server) completeChecklistRun(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.CompleteChecklistRun(r.Context(), s.actor(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.enqueueEvent(domain.EventChecklistCompleted, map[string]any{
		"checklist_run_id": v.ID,
		"inspection_id":    v.InspectionID,
		"checklist_id":     v.ChecklistID,
		"status":           v.Status,
	})
	writeData(w, http.StatusOK, v)
}
