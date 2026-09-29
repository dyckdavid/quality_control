package store

import (
	"context"
	"strings"

	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const siteCols = `id, code, name, address, active, created_at, updated_at`

func scanSite(row scannable) (domain.Site, error) {
	var v domain.Site
	err := row.Scan(&v.ID, &v.Code, &v.Name, &v.Address, &v.Active, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (s *Store) ListSites(ctx context.Context, query string, active *bool, limit, offset int) ([]domain.Site, int, error) {
	c, limit, offset, err := textActiveFilter(query, active, limit, offset, "code", "name")
	if err != nil {
		return nil, 0, err
	}
	return listRows(ctx, s.pool, "sites", siteCols, "lower(code)", c, limit, offset, scanSite)
}

func (s *Store) GetSite(ctx context.Context, id uuid.UUID) (domain.Site, error) {
	v, err := scanSite(s.pool.QueryRow(ctx, `SELECT `+siteCols+` FROM sites WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateSite(ctx context.Context, actor domain.Actor, in domain.NewSite) (domain.Site, error) {
	in.Code, in.Name, in.Address = domain.Trim(in.Code), domain.Trim(in.Name), domain.Trim(in.Address)
	if err := domain.CheckLabel(in.Code, "code", 40); err != nil {
		return domain.Site{}, wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.Name, "name", 200); err != nil {
		return domain.Site{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Address, "address", 500); err != nil {
		return domain.Site{}, wrapInvalid(err)
	}
	v, err := scanSite(s.pool.QueryRow(ctx, `
		INSERT INTO sites (id, code, name, address) VALUES ($1,$2,$3,$4) RETURNING `+siteCols,
		uuid.New(), in.Code, in.Name, in.Address))
	if err != nil {
		return domain.Site{}, mapErr(err)
	}
	return v, s.writeAudit(ctx, s.pool, actor, "site.create", "site", v.ID, "Created site "+v.Code, nil, v)
}

func (s *Store) UpdateSite(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchSite) (domain.Site, error) {
	var out domain.Site
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanSite(tx.QueryRow(ctx, `SELECT `+siteCols+` FROM sites WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		set, err := siteSetter(in)
		if err != nil {
			return err
		}
		out, err = updateReturning(ctx, tx, "sites", siteCols, set, id, scanSite)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "site.update", "site", id, "Updated site "+out.Code, cur, out)
	})
	return out, err
}

func siteSetter(in domain.PatchSite) (setter, error) {
	var set setter
	if in.Code != nil {
		code := domain.Trim(*in.Code)
		if err := domain.CheckLabel(code, "code", 40); err != nil {
			return set, wrapInvalid(err)
		}
		set.add("code", code)
	}
	if in.Name != nil {
		name := domain.Trim(*in.Name)
		if err := domain.CheckLabel(name, "name", 200); err != nil {
			return set, wrapInvalid(err)
		}
		set.add("name", name)
	}
	if in.Address != nil {
		address := domain.Trim(*in.Address)
		if err := domain.CheckOptional(address, "address", 500); err != nil {
			return set, wrapInvalid(err)
		}
		set.add("address", address)
	}
	if in.Active != nil {
		set.add("active", *in.Active)
	}
	if set.empty() {
		return set, invalid("no fields to update")
	}
	return set, nil
}

const stationCols = `id, site_id, code, name, description, active, created_at, updated_at`

func scanStation(row scannable) (domain.Station, error) {
	var v domain.Station
	err := row.Scan(&v.ID, &v.SiteID, &v.Code, &v.Name, &v.Description, &v.Active, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (s *Store) ListStations(ctx context.Context, siteID *uuid.UUID, query string, active *bool, limit, offset int) ([]domain.Station, int, error) {
	c, limit, offset, err := textActiveFilter(query, active, limit, offset, "code", "name")
	if err != nil {
		return nil, 0, err
	}
	if siteID != nil {
		c.add("site_id = ?", *siteID)
	}
	return listRows(ctx, s.pool, "stations", stationCols, "lower(code)", c, limit, offset, scanStation)
}

func (s *Store) GetStation(ctx context.Context, id uuid.UUID) (domain.Station, error) {
	v, err := scanStation(s.pool.QueryRow(ctx, `SELECT `+stationCols+` FROM stations WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateStation(ctx context.Context, actor domain.Actor, in domain.NewStation) (domain.Station, error) {
	in.Code, in.Name, in.Description = domain.Trim(in.Code), domain.Trim(in.Name), domain.Trim(in.Description)
	if err := domain.CheckLabel(in.Code, "code", 40); err != nil {
		return domain.Station{}, wrapInvalid(err)
	}
	if err := domain.CheckLabel(in.Name, "name", 200); err != nil {
		return domain.Station{}, wrapInvalid(err)
	}
	if err := domain.CheckOptional(in.Description, "description", 2000); err != nil {
		return domain.Station{}, wrapInvalid(err)
	}
	if err := s.requireActiveRow(ctx, s.pool, "sites", in.SiteID, "site"); err != nil {
		return domain.Station{}, err
	}
	v, err := scanStation(s.pool.QueryRow(ctx, `
		INSERT INTO stations (id, site_id, code, name, description) VALUES ($1,$2,$3,$4,$5) RETURNING `+stationCols,
		uuid.New(), in.SiteID, in.Code, in.Name, in.Description))
	if err != nil {
		return domain.Station{}, mapErr(err)
	}
	return v, s.writeAudit(ctx, s.pool, actor, "station.create", "station", v.ID, "Created station "+v.Code, nil, v)
}

func (s *Store) UpdateStation(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchStation) (domain.Station, error) {
	var out domain.Station
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanStation(tx.QueryRow(ctx, `SELECT `+stationCols+` FROM stations WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		var set setter
		if in.Code != nil {
			code := domain.Trim(*in.Code)
			if err := domain.CheckLabel(code, "code", 40); err != nil {
				return wrapInvalid(err)
			}
			set.add("code", code)
		}
		if in.Name != nil {
			name := domain.Trim(*in.Name)
			if err := domain.CheckLabel(name, "name", 200); err != nil {
				return wrapInvalid(err)
			}
			set.add("name", name)
		}
		if in.Description != nil {
			desc := domain.Trim(*in.Description)
			if err := domain.CheckOptional(desc, "description", 2000); err != nil {
				return wrapInvalid(err)
			}
			set.add("description", desc)
		}
		if in.Active != nil {
			set.add("active", *in.Active)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "stations", stationCols, set, id, scanStation)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "station.update", "station", id, "Updated station "+out.Code, cur, out)
	})
	return out, err
}

const supplierCols = `id, code, name, contact_name, email, phone, active, created_at, updated_at`

func scanSupplier(row scannable) (domain.Supplier, error) {
	var v domain.Supplier
	err := row.Scan(&v.ID, &v.Code, &v.Name, &v.ContactName, &v.Email, &v.Phone, &v.Active, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (s *Store) ListSuppliers(ctx context.Context, query string, active *bool, limit, offset int) ([]domain.Supplier, int, error) {
	c, limit, offset, err := textActiveFilter(query, active, limit, offset, "code", "name")
	if err != nil {
		return nil, 0, err
	}
	return listRows(ctx, s.pool, "suppliers", supplierCols, "lower(code)", c, limit, offset, scanSupplier)
}

func (s *Store) GetSupplier(ctx context.Context, id uuid.UUID) (domain.Supplier, error) {
	v, err := scanSupplier(s.pool.QueryRow(ctx, `SELECT `+supplierCols+` FROM suppliers WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateSupplier(ctx context.Context, actor domain.Actor, in domain.NewSupplier) (domain.Supplier, error) {
	if err := cleanSupplier(&in.Code, &in.Name, &in.ContactName, &in.Email, &in.Phone); err != nil {
		return domain.Supplier{}, err
	}
	v, err := scanSupplier(s.pool.QueryRow(ctx, `
		INSERT INTO suppliers (id, code, name, contact_name, email, phone) VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+supplierCols,
		uuid.New(), in.Code, in.Name, in.ContactName, in.Email, in.Phone))
	if err != nil {
		return domain.Supplier{}, mapErr(err)
	}
	return v, s.writeAudit(ctx, s.pool, actor, "supplier.create", "supplier", v.ID, "Created supplier "+v.Code, nil, v)
}

func (s *Store) UpdateSupplier(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchSupplier) (domain.Supplier, error) {
	var out domain.Supplier
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanSupplier(tx.QueryRow(ctx, `SELECT `+supplierCols+` FROM suppliers WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		var set setter
		code, name, contact, email, phone := cur.Code, cur.Name, cur.ContactName, cur.Email, cur.Phone
		if in.Code != nil {
			code = domain.Trim(*in.Code)
		}
		if in.Name != nil {
			name = domain.Trim(*in.Name)
		}
		if in.ContactName != nil {
			contact = domain.Trim(*in.ContactName)
		}
		if in.Email != nil {
			email = domain.Trim(*in.Email)
		}
		if in.Phone != nil {
			phone = domain.Trim(*in.Phone)
		}
		if in.Code != nil || in.Name != nil || in.ContactName != nil || in.Email != nil || in.Phone != nil {
			if err := cleanSupplier(&code, &name, &contact, &email, &phone); err != nil {
				return err
			}
		}
		if in.Code != nil {
			set.add("code", code)
		}
		if in.Name != nil {
			set.add("name", name)
		}
		if in.ContactName != nil {
			set.add("contact_name", contact)
		}
		if in.Email != nil {
			set.add("email", email)
		}
		if in.Phone != nil {
			set.add("phone", phone)
		}
		if in.Active != nil {
			set.add("active", *in.Active)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "suppliers", supplierCols, set, id, scanSupplier)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "supplier.update", "supplier", id, "Updated supplier "+out.Code, cur, out)
	})
	return out, err
}

func cleanSupplier(code, name, contact, email, phone *string) error {
	*code, *name = domain.Trim(*code), domain.Trim(*name)
	*contact, *email, *phone = domain.Trim(*contact), domain.Trim(*email), domain.Trim(*phone)
	if err := domain.CheckLabel(*code, "code", 40); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckLabel(*name, "name", 200); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOptional(*contact, "contact_name", 200); err != nil {
		return wrapInvalid(err)
	}
	if *email != "" {
		if err := domain.CheckEmail(*email); err != nil {
			return wrapInvalid(err)
		}
	}
	if err := domain.CheckOptional(*phone, "phone", 40); err != nil {
		return wrapInvalid(err)
	}
	return nil
}

const gaugeCols = `id, code, name, serial_number, COALESCE(calibration_due::text, ''), active, created_at, updated_at`

func scanGauge(row scannable) (domain.Gauge, error) {
	var v domain.Gauge
	err := row.Scan(&v.ID, &v.Code, &v.Name, &v.SerialNumber, &v.CalibrationDue, &v.Active, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (s *Store) ListGauges(ctx context.Context, query string, active *bool, limit, offset int) ([]domain.Gauge, int, error) {
	c, limit, offset, err := textActiveFilter(query, active, limit, offset, "code", "name")
	if err != nil {
		return nil, 0, err
	}
	return listRows(ctx, s.pool, "gauges", gaugeCols, "lower(code)", c, limit, offset, scanGauge)
}

func (s *Store) GetGauge(ctx context.Context, id uuid.UUID) (domain.Gauge, error) {
	v, err := scanGauge(s.pool.QueryRow(ctx, `SELECT `+gaugeCols+` FROM gauges WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateGauge(ctx context.Context, actor domain.Actor, in domain.NewGauge) (domain.Gauge, error) {
	due, err := cleanGauge(&in.Code, &in.Name, &in.SerialNumber, in.CalibrationDue)
	if err != nil {
		return domain.Gauge{}, err
	}
	v, err := scanGauge(s.pool.QueryRow(ctx, `
		INSERT INTO gauges (id, code, name, serial_number, calibration_due) VALUES ($1,$2,$3,$4,$5) RETURNING `+gaugeCols,
		uuid.New(), in.Code, in.Name, in.SerialNumber, due))
	if err != nil {
		return domain.Gauge{}, mapErr(err)
	}
	return v, s.writeAudit(ctx, s.pool, actor, "gauge.create", "gauge", v.ID, "Created gauge "+v.Code, nil, v)
}

func (s *Store) UpdateGauge(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchGauge) (domain.Gauge, error) {
	var out domain.Gauge
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanGauge(tx.QueryRow(ctx, `SELECT `+gaugeCols+` FROM gauges WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		code, name, serial, dueText := cur.Code, cur.Name, cur.SerialNumber, cur.CalibrationDue
		if in.Code != nil {
			code = *in.Code
		}
		if in.Name != nil {
			name = *in.Name
		}
		if in.SerialNumber != nil {
			serial = *in.SerialNumber
		}
		if in.CalibrationDue != nil {
			dueText = *in.CalibrationDue
		}
		due, err := cleanGauge(&code, &name, &serial, dueText)
		if err != nil {
			return err
		}
		var set setter
		if in.Code != nil {
			set.add("code", code)
		}
		if in.Name != nil {
			set.add("name", name)
		}
		if in.SerialNumber != nil {
			set.add("serial_number", serial)
		}
		if in.CalibrationDue != nil {
			set.add("calibration_due", due)
		}
		if in.Active != nil {
			set.add("active", *in.Active)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "gauges", gaugeCols, set, id, scanGauge)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "gauge.update", "gauge", id, "Updated gauge "+out.Code, cur, out)
	})
	return out, err
}

func cleanGauge(code, name, serial *string, dueText string) (any, error) {
	*code, *name, *serial = domain.Trim(*code), domain.Trim(*name), domain.Trim(*serial)
	if err := domain.CheckLabel(*code, "code", 40); err != nil {
		return nil, wrapInvalid(err)
	}
	if err := domain.CheckLabel(*name, "name", 200); err != nil {
		return nil, wrapInvalid(err)
	}
	if err := domain.CheckOptional(*serial, "serial_number", 80); err != nil {
		return nil, wrapInvalid(err)
	}
	return dateArg(dueText)
}

const productCols = `id, part_number, name, description, unit, active, created_by, created_at, updated_at`

func scanProduct(row scannable) (domain.Product, error) {
	var v domain.Product
	var createdBy *uuid.UUID
	err := row.Scan(&v.ID, &v.PartNumber, &v.Name, &v.Description, &v.Unit, &v.Active, &createdBy, &v.CreatedAt, &v.UpdatedAt)
	v.CreatedBy = uuidOrNil(createdBy)
	return v, err
}

func (s *Store) ListProducts(ctx context.Context, query string, active *bool, limit, offset int) ([]domain.Product, int, error) {
	c, limit, offset, err := textActiveFilter(query, active, limit, offset, "part_number", "name")
	if err != nil {
		return nil, 0, err
	}
	return listRows(ctx, s.pool, "products", productCols, "lower(part_number)", c, limit, offset, scanProduct)
}

func (s *Store) GetProduct(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	v, err := scanProduct(s.pool.QueryRow(ctx, `SELECT `+productCols+` FROM products WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateProduct(ctx context.Context, actor domain.Actor, in domain.NewProduct) (domain.Product, error) {
	if err := cleanProduct(&in.PartNumber, &in.Name, &in.Description, &in.Unit); err != nil {
		return domain.Product{}, err
	}
	var createdBy any
	if actor.ID != uuid.Nil {
		createdBy = actor.ID
	}
	v, err := scanProduct(s.pool.QueryRow(ctx, `
		INSERT INTO products (id, part_number, name, description, unit, created_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+productCols,
		uuid.New(), in.PartNumber, in.Name, in.Description, in.Unit, createdBy))
	if err != nil {
		return domain.Product{}, mapErr(err)
	}
	return v, s.writeAudit(ctx, s.pool, actor, "product.create", "product", v.ID, "Created product "+v.PartNumber, nil, v)
}

func (s *Store) UpdateProduct(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchProduct) (domain.Product, error) {
	var out domain.Product
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanProduct(tx.QueryRow(ctx, `SELECT `+productCols+` FROM products WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		part, name, desc, unit := cur.PartNumber, cur.Name, cur.Description, cur.Unit
		if in.PartNumber != nil {
			part = *in.PartNumber
		}
		if in.Name != nil {
			name = *in.Name
		}
		if in.Description != nil {
			desc = *in.Description
		}
		if in.Unit != nil {
			unit = *in.Unit
		}
		if err := cleanProduct(&part, &name, &desc, &unit); err != nil {
			return err
		}
		var set setter
		if in.PartNumber != nil {
			set.add("part_number", part)
		}
		if in.Name != nil {
			set.add("name", name)
		}
		if in.Description != nil {
			set.add("description", desc)
		}
		if in.Unit != nil {
			set.add("unit", unit)
		}
		if in.Active != nil {
			set.add("active", *in.Active)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "products", productCols, set, id, scanProduct)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "product.update", "product", id, "Updated product "+out.PartNumber, cur, out)
	})
	return out, err
}

func cleanProduct(part, name, desc, unit *string) error {
	*part, *name, *desc, *unit = domain.Trim(*part), domain.Trim(*name), domain.Trim(*desc), domain.Trim(*unit)
	if err := domain.CheckLabel(*part, "part_number", 80); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckLabel(*name, "name", 200); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOptional(*desc, "description", 8000); err != nil {
		return wrapInvalid(err)
	}
	if *unit == "" {
		*unit = "ea"
	}
	if err := domain.CheckLabel(*unit, "unit", 40); err != nil {
		return wrapInvalid(err)
	}
	return nil
}

const defectCols = `id, code, name, severity, category, active, created_at, updated_at`

func scanDefect(row scannable) (domain.DefectCode, error) {
	var v domain.DefectCode
	err := row.Scan(&v.ID, &v.Code, &v.Name, &v.Severity, &v.Category, &v.Active, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (s *Store) ListDefectCodes(ctx context.Context, query, severity string, active *bool, limit, offset int) ([]domain.DefectCode, int, error) {
	c, limit, offset, err := textActiveFilter(query, active, limit, offset, "code", "name")
	if err != nil {
		return nil, 0, err
	}
	if severity != "" {
		if err := domain.CheckOneOf(severity, "severity", []string{domain.SeverityMinor, domain.SeverityMajor, domain.SeverityCritical}); err != nil {
			return nil, 0, wrapInvalid(err)
		}
		c.add("severity = ?", severity)
	}
	return listRows(ctx, s.pool, "defect_codes", defectCols, "lower(code)", c, limit, offset, scanDefect)
}

func (s *Store) GetDefectCode(ctx context.Context, id uuid.UUID) (domain.DefectCode, error) {
	v, err := scanDefect(s.pool.QueryRow(ctx, `SELECT `+defectCols+` FROM defect_codes WHERE id = $1`, id))
	return v, mapErr(err)
}

func (s *Store) CreateDefectCode(ctx context.Context, actor domain.Actor, in domain.NewDefectCode) (domain.DefectCode, error) {
	if err := cleanDefect(&in.Code, &in.Name, &in.Severity, &in.Category); err != nil {
		return domain.DefectCode{}, err
	}
	v, err := scanDefect(s.pool.QueryRow(ctx, `
		INSERT INTO defect_codes (id, code, name, severity, category) VALUES ($1,$2,$3,$4,$5) RETURNING `+defectCols,
		uuid.New(), in.Code, in.Name, in.Severity, in.Category))
	if err != nil {
		return domain.DefectCode{}, mapErr(err)
	}
	return v, s.writeAudit(ctx, s.pool, actor, "defect_code.create", "defect_code", v.ID, "Created defect code "+v.Code, nil, v)
}

func (s *Store) UpdateDefectCode(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchDefectCode) (domain.DefectCode, error) {
	var out domain.DefectCode
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := scanDefect(tx.QueryRow(ctx, `SELECT `+defectCols+` FROM defect_codes WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return mapErr(err)
		}
		code, name, severity, category := cur.Code, cur.Name, cur.Severity, cur.Category
		if in.Code != nil {
			code = *in.Code
		}
		if in.Name != nil {
			name = *in.Name
		}
		if in.Severity != nil {
			severity = *in.Severity
		}
		if in.Category != nil {
			category = *in.Category
		}
		if err := cleanDefect(&code, &name, &severity, &category); err != nil {
			return err
		}
		var set setter
		if in.Code != nil {
			set.add("code", code)
		}
		if in.Name != nil {
			set.add("name", name)
		}
		if in.Severity != nil {
			set.add("severity", severity)
		}
		if in.Category != nil {
			set.add("category", category)
		}
		if in.Active != nil {
			set.add("active", *in.Active)
		}
		if set.empty() {
			return invalid("no fields to update")
		}
		out, err = updateReturning(ctx, tx, "defect_codes", defectCols, set, id, scanDefect)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tx, actor, "defect_code.update", "defect_code", id, "Updated defect code "+out.Code, cur, out)
	})
	return out, err
}

func cleanDefect(code, name, severity, category *string) error {
	*code, *name, *severity, *category = domain.Trim(*code), domain.Trim(*name), domain.Trim(*severity), domain.Trim(*category)
	if err := domain.CheckLabel(*code, "code", 40); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckLabel(*name, "name", 200); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOneOf(*severity, "severity", []string{domain.SeverityMinor, domain.SeverityMajor, domain.SeverityCritical}); err != nil {
		return wrapInvalid(err)
	}
	if err := domain.CheckOptional(*category, "category", 80); err != nil {
		return wrapInvalid(err)
	}
	return nil
}

func textActiveFilter(query string, active *bool, limit, offset int, cols ...string) (clause, int, int, error) {
	query, err := cleanQuery(query)
	if err != nil {
		return clause{}, 0, 0, err
	}
	limit, offset = clamp(limit, offset, 200)
	var c clause
	if query != "" {
		like := likeContains(query)
		parts := make([]string, len(cols))
		args := make([]any, len(cols))
		for i, col := range cols {
			parts[i] = col + " ILIKE ? ESCAPE '\\'"
			args[i] = like
		}
		c.add("("+strings.Join(parts, " OR ")+")", args...)
	}
	if active != nil {
		c.add("active = ?", *active)
	}
	return c, limit, offset, nil
}

func updateReturning[T any](ctx context.Context, q querier, table, cols string, set setter, id uuid.UUID, scan func(scannable) (T, error)) (T, error) {
	var zero T
	set.args = append(set.args, id)
	sql := `UPDATE ` + table + ` SET ` + strings.Join(set.sets, ", ") + ` WHERE id = $` + itoa(len(set.args)) + ` RETURNING ` + cols
	item, err := scan(q.QueryRow(ctx, sql, set.args...))
	if err != nil {
		return zero, mapErr(err)
	}
	return item, nil
}
