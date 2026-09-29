package httpapi

import (
	"net/http"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	active, ok := queryBool(w, r, "active")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListUsers(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("role"), active, limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var in domain.NewUser
	if !decode(w, r, &in) {
		return
	}
	user, err := s.store.CreateUser(r.Context(), s.actor(r), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/users/"+user.ID.String())
	writeData(w, http.StatusCreated, user)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	user, err := s.store.GetUser(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, user)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchUser
	if !decode(w, r, &in) {
		return
	}
	p, _ := principal(r)
	if in.Active != nil && !*in.Active && id == p.ID {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "You cannot deactivate your own account.")
		return
	}
	user, err := s.store.UpdateUser(r.Context(), p.Actor(), id, in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, user)
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PasswordReset
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.ResetPassword(r.Context(), s.actor(r), id, in.Password); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	s.listActive(w, r, func(q string, active *bool, limit, offset int) (any, int, error) {
		return s.store.ListSites(r.Context(), q, active, limit, offset)
	})
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var in domain.NewSite
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateSite(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/sites/", v.ID, v, err)
}

func (s *Server) getSite(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetSite(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateSite(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchSite
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateSite(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) listStations(w http.ResponseWriter, r *http.Request) {
	site, ok := queryUUID(w, r, "site_id")
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
	rows, total, err := s.store.ListStations(r.Context(), site, r.URL.Query().Get("q"), active, limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createStation(w http.ResponseWriter, r *http.Request) {
	var in domain.NewStation
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateStation(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/stations/", v.ID, v, err)
}

func (s *Server) getStation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetStation(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateStation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchStation
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateStation(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) listSuppliers(w http.ResponseWriter, r *http.Request) {
	s.listActive(w, r, func(q string, active *bool, limit, offset int) (any, int, error) {
		return s.store.ListSuppliers(r.Context(), q, active, limit, offset)
	})
}

func (s *Server) createSupplier(w http.ResponseWriter, r *http.Request) {
	var in domain.NewSupplier
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateSupplier(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/suppliers/", v.ID, v, err)
}

func (s *Server) getSupplier(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetSupplier(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateSupplier(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchSupplier
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateSupplier(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) listGauges(w http.ResponseWriter, r *http.Request) {
	s.listActive(w, r, func(q string, active *bool, limit, offset int) (any, int, error) {
		return s.store.ListGauges(r.Context(), q, active, limit, offset)
	})
}

func (s *Server) createGauge(w http.ResponseWriter, r *http.Request) {
	var in domain.NewGauge
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateGauge(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/gauges/", v.ID, v, err)
}

func (s *Server) getGauge(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetGauge(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateGauge(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchGauge
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateGauge(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	s.listActive(w, r, func(q string, active *bool, limit, offset int) (any, int, error) {
		return s.store.ListProducts(r.Context(), q, active, limit, offset)
	})
}

func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) {
	var in domain.NewProduct
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateProduct(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/products/", v.ID, v, err)
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetProduct(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchProduct
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateProduct(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) listDefects(w http.ResponseWriter, r *http.Request) {
	active, ok := queryBool(w, r, "active")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := s.store.ListDefectCodes(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("severity"), active, limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) createDefect(w http.ResponseWriter, r *http.Request) {
	var in domain.NewDefectCode
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.CreateDefectCode(r.Context(), s.actor(r), in)
	s.created(w, r, "/api/v1/defect-codes/", v.ID, v, err)
}

func (s *Server) getDefect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.store.GetDefectCode(r.Context(), id)
	s.one(w, r, v, err)
}

func (s *Server) updateDefect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.PatchDefectCode
	if !decode(w, r, &in) {
		return
	}
	v, err := s.store.UpdateDefectCode(r.Context(), s.actor(r), id, in)
	s.one(w, r, v, err)
}

func (s *Server) listActive(w http.ResponseWriter, r *http.Request, fn func(string, *bool, int, int) (any, int, error)) {
	active, ok := queryBool(w, r, "active")
	if !ok {
		return
	}
	limit, offset, ok := queryPage(w, r)
	if !ok {
		return
	}
	rows, total, err := fn(r.URL.Query().Get("q"), active, limit, offset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writePage(w, rows, limit, offset, total)
}

func (s *Server) one(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeData(w, http.StatusOK, v)
}

func (s *Server) created(w http.ResponseWriter, r *http.Request, prefix string, id uuid.UUID, v any, err error) {
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Location", prefix+id.String())
	writeData(w, http.StatusCreated, v)
}
