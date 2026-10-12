package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"quality_control/internal/domain"
)

func (s *Server) deleteRecord(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "id")
		if !ok {
			return
		}
		files, err := s.store.DeleteRecord(r.Context(), s.actor(r), kind, id)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		for _, name := range files {
			_ = os.Remove(filepath.Join(s.cfg.DataDir, "attachments", filepath.Base(name)))
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) updateInspection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchInspection
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateInspection(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) lookup(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "invalid", "Scan or type a code.")
		return
	}
	hits, err := s.store.Lookup(r.Context(), code)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, hits)
}
