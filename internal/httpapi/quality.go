package httpapi

import (
	"net/http"

	"quality_control/internal/domain"
	"quality_control/internal/store"
)

func (s *Server) listNCRs(w http.ResponseWriter, r *http.Request) {
	lot, ok := queryUUID(w, r, "lot_id")
	if !ok {
		return
	}
	product, ok := queryUUID(w, r, "product_id")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListNCRs(r.Context(), store.NCRFilter{
		Status: r.URL.Query().Get("status"), Severity: r.URL.Query().Get("severity"),
		LotID: lot, ProductID: product, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createNCR(w http.ResponseWriter, r *http.Request) {
	var in domain.NewNonconformance
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateNCR(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/nonconformances/", v.ID, v, err)
}

func (s *Server) getNCR(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetNCR(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateNCR(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchNonconformance
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateNCR(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) disposeNCR(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.DispositionRequest
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.DisposeNCR(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) closeNCR(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.CloseNCR(r.Context(), s.actor(r), id)
	s.one(w, r, v, err)
}

func (s *Server) voidNCR(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.NoteBody
	if !decodeOptional(w, r, &in) {
		return
	}
	v, err := s.store.VoidNCR(r.Context(), s.actor(r), id, in.Notes)
	s.one(w, r, v, err)
}

func (s *Server) listCAPAs(w http.ResponseWriter, r *http.Request) {
	owner, ok := queryUUID(w, r, "owner_id")
	if !ok {
		return
	}
	ncr, ok := queryUUID(w, r, "nonconformance_id")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListCAPAs(r.Context(), store.CAPAFilter{
		Status: r.URL.Query().Get("status"), OwnerID: owner, NonconformanceID: ncr, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createCAPA(w http.ResponseWriter, r *http.Request) {
	var in domain.NewCorrectiveAction
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateCAPA(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/corrective-actions/", v.ID, v, err)
}

func (s *Server) getCAPA(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetCAPA(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateCAPA(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchCorrectiveAction
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateCAPA(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) startCAPA(w http.ResponseWriter, r *http.Request) {
	s.moveCAPA(w, r, domain.CAPAInProgress)
}

func (s *Server) verifyCAPA(w http.ResponseWriter, r *http.Request) {
	s.moveCAPA(w, r, domain.CAPAVerification)
}

func (s *Server) closeCAPA(w http.ResponseWriter, r *http.Request) {
	s.moveCAPA(w, r, domain.CAPAClosed)
}

func (s *Server) reopenCAPA(w http.ResponseWriter, r *http.Request) {
	s.moveCAPA(w, r, domain.CAPAInProgress)
}

func (s *Server) cancelCAPA(w http.ResponseWriter, r *http.Request) {
	s.moveCAPA(w, r, domain.CAPACancelled)
}

func (s *Server) moveCAPA(w http.ResponseWriter, r *http.Request, to string) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.NoteBody
	if !decodeOptional(w, r, &in) {
		return
	}
	v, err := s.store.TransitionCAPA(r.Context(), s.actor(r), id, to, in.Notes)
	s.one(w, r, v, err)
}
