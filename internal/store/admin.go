package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"quality_control/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type deletableKind struct {
	table, entity, label, numberCol, attachmentType string
}

// Deletable maps the URL segment an administrator deletes through to its table.
// Rows still referenced by other records are refused by the foreign keys, so a
// delete can never orphan inspection history.
var deletable = map[string]deletableKind{
	"products":           {"products", "product", "product", "part_number", ""},
	"sites":              {"sites", "site", "site", "code", ""},
	"stations":           {"stations", "station", "station", "code", ""},
	"suppliers":          {"suppliers", "supplier", "supplier", "code", ""},
	"gauges":             {"gauges", "gauge", "gauge", "code", ""},
	"defect-codes":       {"defect_codes", "defect_code", "defect code", "code", ""},
	"lots":               {"lots", "lot", "lot", "lot_number", "lot"},
	"checklists":         {"checklists", "checklist", "checklist", "code", ""},
	"webhooks":           {"webhooks", "webhook", "webhook", "name", ""},
	"users":              {"users", "user", "user", "username", ""},
	"inspections":        {"inspections", "inspection", "inspection", "inspection_number", "inspection"},
	"nonconformances":    {"nonconformances", "nonconformance", "alert", "ncr_number", "nonconformance"},
	"corrective-actions": {"corrective_actions", "corrective_action", "corrective action", "ca_number", "corrective_action"},
}

// DeletableKinds lists the URL segments DeleteRecord accepts.
func DeletableKinds() []string {
	out := make([]string, 0, len(deletable))
	for k := range deletable {
		out = append(out, k)
	}
	return out
}

var referenceNames = map[string]string{
	"lots":                "lots",
	"inspections":         "inspections",
	"inspection_plans":    "inspection plans",
	"inspection_results":  "recorded readings",
	"inspection_defects":  "recorded defects",
	"nonconformances":     "alerts",
	"corrective_actions":  "corrective actions",
	"stations":            "stations",
	"checklists":          "checklists",
	"checklist_runs":      "filled checklists",
	"checklist_run_items": "filled checklists",
	"audit_log":           "the audit history",
	"attachments":         "attachments",
	"products":            "products",
}

// DeleteRecord removes one row for an administrator and returns the storage names of
// any attachment files that went with it, so the caller can remove them from disk.
func (s *Store) DeleteRecord(ctx context.Context, actor domain.Actor, kind string, id uuid.UUID) ([]string, error) {
	k, ok := deletable[kind]
	if !ok {
		return nil, ErrNotFound
	}
	var files []string
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT to_jsonb(t) - 'password_hash' - 'secret' FROM `+k.table+` t WHERE id = $1 FOR UPDATE`, id).Scan(&raw)
		if err != nil {
			return mapErr(err)
		}
		var before map[string]any
		_ = json.Unmarshal(raw, &before)
		number, _ := before[k.numberCol].(string)

		var lotID uuid.UUID
		switch kind {
		case "users":
			if id == actor.ID {
				return invalid("You can't delete the account you are signed in with.")
			}
			if before["role"] == domain.RoleAdmin && before["active"] == true {
				if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE role = 'admin' AND active ORDER BY id FOR UPDATE`); err != nil {
					return mapErr(err)
				}
				var others int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin' AND active AND id <> $1`, id).Scan(&others); err != nil {
					return mapErr(err)
				}
				if others == 0 {
					return invalid("The last active administrator can't be deleted.")
				}
			}
		case "inspections":
			if s, _ := before["lot_id"].(string); s != "" {
				lotID, _ = uuid.Parse(s)
			}
			if lotID != uuid.Nil {
				if _, err := tx.Exec(ctx, `SELECT 1 FROM lots WHERE id = $1 FOR UPDATE`, lotID); err != nil {
					return mapErr(err)
				}
			}
		}

		if k.attachmentType != "" {
			rows, err := tx.Query(ctx, `DELETE FROM attachments WHERE entity_type = $1 AND entity_id = $2 RETURNING storage_name`, k.attachmentType, id)
			if err != nil {
				return mapErr(err)
			}
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					rows.Close()
					return mapErr(err)
				}
				files = append(files, name)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return mapErr(err)
			}
		}

		if _, err := tx.Exec(ctx, `DELETE FROM `+k.table+` WHERE id = $1`, id); err != nil {
			return inUse(err, k.label, number)
		}
		if lotID != uuid.Nil {
			if err := s.syncLot(ctx, tx, lotID); err != nil {
				return err
			}
		}
		summary := "Deleted " + k.label
		if number != "" {
			summary += " " + number
		}
		return s.writeAudit(ctx, tx, actor, k.entity+".delete", k.entity, id, summary, json.RawMessage(raw), nil)
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func inUse(err error, label, number string) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		what := referenceNames[pg.TableName]
		if what == "" {
			what = "other records"
		}
		name := strings.ToUpper(label[:1]) + label[1:]
		if number != "" {
			name += " " + number
		}
		hint := " Remove those first, or turn it off instead."
		if pg.TableName == "audit_log" {
			hint = " Turn it off instead so the history stays intact."
		}
		return fail(ErrConflict, name+" is still used by "+what+"."+hint)
	}
	return mapErr(err)
}

// UpdateInspection edits the header of an inspection. Finished inspections need a supervisor.
func (s *Store) UpdateInspection(ctx context.Context, actor domain.Actor, id uuid.UUID, in domain.PatchInspection) (domain.Inspection, error) {
	var out domain.Inspection
	err := s.tx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.lockInspection(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.Status != domain.InspInProgress && domain.Rank(actor.Role) < domain.Rank(domain.RoleSupervisor) {
			return fail(ErrForbidden, "Only a supervisor can change a finished inspection.")
		}
		set := setter{}
		if in.Notes != nil {
			if err := domain.CheckOptional(*in.Notes, "notes", 4000); err != nil {
				return wrapInvalid(err)
			}
			set.add("notes", *in.Notes)
		}
		if in.ClearStation {
			set.add("station_id", nil)
		} else if in.StationID != nil {
			set.add("station_id", *in.StationID)
		}
		if set.empty() {
			out = cur
			return nil
		}
		set.args = append(set.args, id)
		out, err = scanInspection(tx.QueryRow(ctx, `UPDATE inspections SET `+strings.Join(set.sets, ", ")+` WHERE id = $`+itoa(len(set.args))+` RETURNING `+inspectionCols, set.args...))
		if err != nil {
			return mapErr(err)
		}
		return s.writeAudit(ctx, tx, actor, "inspection.update", "inspection", id, "Updated inspection "+out.InspectionNumber, cur, out)
	})
	return out, err
}

// Lookup finds records whose number, code, or serial matches a scanned code exactly.
func (s *Store) Lookup(ctx context.Context, code string) ([]domain.SearchHit, error) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 200 {
		return []domain.SearchHit{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT entity_type, id, number, title, status, created_at FROM (
			SELECT 1 AS rank, 'lot' AS entity_type, l.id, l.lot_number AS number, p.name AS title, l.status, l.created_at
			FROM lots l JOIN products p ON p.id = l.product_id
			WHERE lower(l.lot_number) = lower($1) OR (l.supplier_lot <> '' AND lower(l.supplier_lot) = lower($1))
			UNION ALL
			SELECT 2, 'inspection', id, inspection_number, notes, status, created_at FROM inspections WHERE lower(inspection_number) = lower($1)
			UNION ALL
			SELECT 2, 'nonconformance', id, ncr_number, title, status, created_at FROM nonconformances WHERE lower(ncr_number) = lower($1)
			UNION ALL
			SELECT 2, 'corrective_action', id, ca_number, title, status, created_at FROM corrective_actions WHERE lower(ca_number) = lower($1)
			UNION ALL
			SELECT 3, 'gauge', id, code, name, CASE WHEN active THEN 'active' ELSE 'inactive' END, created_at FROM gauges
			WHERE lower(code) = lower($1) OR (serial_number <> '' AND lower(serial_number) = lower($1))
			UNION ALL
			SELECT 4, 'product', id, part_number, name, CASE WHEN active THEN 'active' ELSE 'inactive' END, created_at FROM products WHERE lower(part_number) = lower($1)
			UNION ALL
			SELECT 5, 'station', id, code, name, CASE WHEN active THEN 'active' ELSE 'inactive' END, created_at FROM stations WHERE lower(code) = lower($1)
			UNION ALL
			SELECT 5, 'supplier', id, code, name, CASE WHEN active THEN 'active' ELSE 'inactive' END, created_at FROM suppliers WHERE lower(code) = lower($1)
			UNION ALL
			SELECT 5, 'defect_code', id, code, name, CASE WHEN active THEN 'active' ELSE 'inactive' END, created_at FROM defect_codes WHERE lower(code) = lower($1)
			UNION ALL
			SELECT 6, 'site', id, code, name, CASE WHEN active THEN 'active' ELSE 'inactive' END, created_at FROM sites WHERE lower(code) = lower($1)
		) hits
		ORDER BY rank, created_at DESC
		LIMIT 20`, code)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.SearchHit{}
	for rows.Next() {
		var h domain.SearchHit
		if err := rows.Scan(&h.EntityType, &h.ID, &h.Number, &h.Title, &h.Status, &h.CreatedAt); err != nil {
			return nil, mapErr(err)
		}
		h.Title = clip(h.Title, 160)
		out = append(out, h)
	}
	return out, rows.Err()
}
