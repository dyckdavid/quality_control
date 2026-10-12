package store

import (
	"context"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

// SeedDemo loads the shop catalog: cattle equipment and hearth products,
// the checks for each, and open builds. Running it again does not duplicate rows.
func (s *Store) SeedDemo(ctx context.Context, actor domain.Actor) (domain.DemoSeedResult, error) {
	site, err := s.ensureSite(ctx, actor, "SHOP", "Shop", "")
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	if _, err := s.ensureStation(ctx, actor, site.ID, "FIT", "Fit and weld", "Frames, gates, panels, fireboxes."); err != nil {
		return domain.DemoSeedResult{}, err
	}
	if _, err := s.ensureStation(ctx, actor, site.ID, "PAINT", "Paint and assembly", "Finish, hardware, and glass."); err != nil {
		return domain.DemoSeedResult{}, err
	}
	final, err := s.ensureStation(ctx, actor, site.ID, "FINAL", "Final inspection", "Last check before a build ships.")
	if err != nil {
		return domain.DemoSeedResult{}, err
	}

	for _, g := range []domain.NewGauge{
		{Code: "TAPE-25", Name: "25 ft tape"},
		{Code: "CAL-6", Name: "6 in caliper"},
		{Code: "SQ-12", Name: "12 in square"},
		{Code: "WELD-GA", Name: "Weld fillet gauge"},
	} {
		if _, err := s.ensureGauge(ctx, actor, g); err != nil {
			return domain.DemoSeedResult{}, err
		}
	}

	for _, d := range shopDefects() {
		if _, err := s.ensureDefect(ctx, actor, d); err != nil {
			return domain.DemoSeedResult{}, err
		}
	}

	type built struct {
		product domain.NewProduct
		plan    string
		chars   []domain.NewCharacteristic
		lot     string
		qty     float64
		notes   string
	}
	builds := []built{
		{
			product: domain.NewProduct{PartNumber: "CHUTE-SQ", Name: "Squeeze chute", Unit: "ea",
				Description: "Cattle squeeze chute. Head gate, squeeze sides, and the handler side."},
			plan: "Squeeze chute final", chars: cattleChecks(true),
			lot: "CHUTE-100", qty: 1, notes: "Squeeze chute at final inspection.",
		},
		{
			product: domain.NewProduct{PartNumber: "GATE-COR", Name: "Corral gate", Unit: "ea",
				Description: "Corral gate. Hinges, chain, and latch."},
			plan: "Corral gate final", chars: cattleChecks(false),
			lot: "GATE-100", qty: 6, notes: "Corral gates in fit and weld.",
		},
		{
			product: domain.NewProduct{PartNumber: "PANEL-COR", Name: "Corral panel", Unit: "ea",
				Description: "Corral panel. Frame, connectors, and feet."},
			plan: "Corral panel final", chars: cattleChecks(false),
			lot: "PANEL-100", qty: 10, notes: "Corral panels in fit and weld.",
		},
		{
			product: domain.NewProduct{PartNumber: "STOVE-WD", Name: "Wood stove", Unit: "ea",
				Description: "Wood stove. Firebox, door, flue collar, and outer skin."},
			plan: "Wood stove final", chars: hearthChecks(true),
			lot: "STOVE-100", qty: 2, notes: "Wood stoves in assembly.",
		},
		{
			product: domain.NewProduct{PartNumber: "FP-WD", Name: "Wood fireplace", Unit: "ea",
				Description: "Wood fireplace. Firebox opening, flue collar, front, and outer finish."},
			plan: "Wood fireplace final", chars: hearthChecks(false),
			lot: "FP-100", qty: 1, notes: "Fireplace in fit and weld.",
		},
	}

	var firstProduct domain.Product
	var firstPlan domain.PlanDetail
	var firstLot domain.Lot
	for i, b := range builds {
		product, err := s.ensureProduct(ctx, actor, b.product)
		if err != nil {
			return domain.DemoSeedResult{}, err
		}
		plan, err := s.ensureActivePlan(ctx, actor, product.ID, b.plan, b.chars)
		if err != nil {
			return domain.DemoSeedResult{}, err
		}
		lot, err := s.ensureLot(ctx, actor, domain.NewLot{
			LotNumber: b.lot, ProductID: product.ID, SiteID: site.ID,
			Quantity: b.qty, Unit: "ea", Source: domain.SourceInProcess, Notes: b.notes,
		})
		if err != nil {
			return domain.DemoSeedResult{}, err
		}
		if i == 0 {
			firstProduct, firstPlan, firstLot = product, plan, lot
		}
	}

	one := 1
	inspID, err := s.ensureInspection(ctx, actor, firstLot.ID, firstPlan.ID, final.ID, &one, "Final check of the squeeze chute.")
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	cattleList, err := s.ensureChecklist(ctx, actor, domain.NewChecklist{
		Code: "CATTLE-FINAL", Name: "Cattle equipment final",
		Description: "Squeeze chute, corral gate, or corral panel before it ships.",
		Items: []domain.NewChecklistItem{
			{Sequence: 1, Prompt: "Welds complete, no crack, no missed joint?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 2, Prompt: "Latches, hinges, and the squeeze travel freely?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 3, Prompt: "No sharp edge where an animal or a person puts a hand?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 4, Prompt: "Paint covers the finished surfaces?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 5, Prompt: "Pins, chains, and the hardware kit are with the unit?", ItemType: domain.ChecklistItemYesNo},
		},
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	if _, err := s.ensureChecklist(ctx, actor, domain.NewChecklist{
		Code: "HEARTH-FINAL", Name: "Wood stove and fireplace final",
		Description: "Wood stove or wood fireplace before it ships.",
		Items: []domain.NewChecklistItem{
			{Sequence: 1, Prompt: "Firebox welds complete?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 2, Prompt: "Door or front closes and latches?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 3, Prompt: "Gasket is continuous, or the unit has no gasket?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 4, Prompt: "Glass is intact, or the unit has no glass?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 5, Prompt: "Rating plate is installed and readable?", ItemType: domain.ChecklistItemYesNo},
		},
	}); err != nil {
		return domain.DemoSeedResult{}, err
	}

	summary := "Shop catalog: squeeze chute, corral gate, corral panel, wood stove, wood fireplace. Builds CHUTE-100, GATE-100, PANEL-100, STOVE-100, FP-100."
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO demo_seed_runs (id, seeded_by, summary) VALUES ($1,$2,$3)`,
		uuid.New(), actor.ID, summary); err != nil {
		return domain.DemoSeedResult{}, mapErr(err)
	}
	return domain.DemoSeedResult{
		SiteID: site.ID, ProductID: firstProduct.ID, PlanID: firstPlan.ID, LotID: firstLot.ID,
		InspectionID: inspID, ChecklistID: cattleList.ID, Summary: summary,
	}, nil
}

func cattleChecks(chute bool) []domain.NewCharacteristic {
	latch := "Latches and hinges operate through full travel."
	if chute {
		latch = "Head gate, squeeze, and latches operate through full travel."
	}
	return []domain.NewCharacteristic{
		dimCheck(1, "WIDTH", "Overall width"),
		dimCheck(2, "HEIGHT", "Overall height"),
		dimCheck(3, "LENGTH", "Overall length"),
		attrCheck(4, "WELD", "Welds", "Welds complete. No crack, no missed joint.", true),
		attrCheck(5, "LATCH", "Latches and travel", latch, true),
		attrCheck(6, "EDGE", "Sharp edges", "No burr or sharp edge on animal or handler contact.", true),
		attrCheck(7, "FINISH", "Finish", "Paint covers the finished surfaces.", false),
		attrCheck(8, "HDW", "Hardware kit", "Pins, chains, and fasteners called out for this unit are present.", false),
	}
}

func hearthChecks(stove bool) []domain.NewCharacteristic {
	door := "Front seats and closes."
	if stove {
		door = "Door closes and latches."
	}
	return []domain.NewCharacteristic{
		dimCheck(1, "OPEN-W", "Opening width"),
		dimCheck(2, "OPEN-H", "Opening height"),
		dimCheck(3, "COLLAR", "Flue collar diameter"),
		attrCheck(4, "WELD", "Firebox welds", "Firebox welds complete. No crack, no missed joint.", true),
		attrCheck(5, "DOOR", "Door or front", door, true),
		attrCheck(6, "GASKET", "Door gasket", "Gasket is continuous and seated. Pass if this unit has no gasket.", true),
		attrCheck(7, "GLASS", "Glass", "Glass is intact. Pass if this unit has no glass.", true),
		attrCheck(8, "PLATE", "Rating plate", "Rating plate is installed and readable.", true),
	}
}

func shopDefects() []domain.NewDefectCode {
	return []domain.NewDefectCode{
		{Code: "WELD-MISS", Name: "Missed or incomplete weld", Severity: domain.SeverityMajor, Category: "weld"},
		{Code: "WELD-CRACK", Name: "Crack in a weld or in the steel", Severity: domain.SeverityCritical, Category: "weld"},
		{Code: "EDGE-BURR", Name: "Sharp edge or burr", Severity: domain.SeverityMajor, Category: "fit"},
		{Code: "LATCH-OP", Name: "Latch, hinge, or squeeze does not travel", Severity: domain.SeverityMajor, Category: "function"},
		{Code: "DIM-OUT", Name: "Dimension out of print", Severity: domain.SeverityMajor, Category: "dimension"},
		{Code: "FIT-GAP", Name: "Fit-up out of print", Severity: domain.SeverityMajor, Category: "fit"},
		{Code: "PAINT-THIN", Name: "Thin or missed finish", Severity: domain.SeverityMinor, Category: "finish"},
		{Code: "PAINT-RUN", Name: "Paint run or sag", Severity: domain.SeverityMinor, Category: "finish"},
		{Code: "HDW-MISS", Name: "Missing pin, bolt, or chain", Severity: domain.SeverityMinor, Category: "hardware"},
		{Code: "GLASS-CRACK", Name: "Cracked glass", Severity: domain.SeverityCritical, Category: "hearth"},
		{Code: "GASKET-GAP", Name: "Gasket gap", Severity: domain.SeverityMajor, Category: "hearth"},
		{Code: "PLATE-MISS", Name: "Rating plate missing or unreadable", Severity: domain.SeverityCritical, Category: "hearth"},
	}
}

func dimCheck(seq int, code, name string) domain.NewCharacteristic {
	return domain.NewCharacteristic{
		Sequence: seq, Code: code, Name: name, CharacteristicType: domain.CharVariable,
		Method: "Record the measurement in inches and compare it to the print for this build.",
		Unit:   "in",
	}
}

func attrCheck(seq int, code, name, method string, critical bool) domain.NewCharacteristic {
	return domain.NewCharacteristic{
		Sequence: seq, Code: code, Name: name, CharacteristicType: domain.CharAttribute,
		Method: method, Critical: critical,
	}
}

func (s *Store) lookupID(ctx context.Context, query string, args ...any) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, query, args...).Scan(&id)
	if isNoRows(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, mapErr(err)
	}
	return id, true, nil
}

func (s *Store) ensureSite(ctx context.Context, actor domain.Actor, code, name, address string) (domain.Site, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM sites WHERE lower(code) = lower($1)`, code); err != nil || ok {
		if err != nil {
			return domain.Site{}, err
		}
		return s.GetSite(ctx, id)
	}
	return s.CreateSite(ctx, actor, domain.NewSite{Code: code, Name: name, Address: address})
}

func (s *Store) ensureStation(ctx context.Context, actor domain.Actor, siteID uuid.UUID, code, name, description string) (domain.Station, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM stations WHERE site_id = $1 AND lower(code) = lower($2)`, siteID, code); err != nil || ok {
		if err != nil {
			return domain.Station{}, err
		}
		return s.GetStation(ctx, id)
	}
	return s.CreateStation(ctx, actor, domain.NewStation{SiteID: siteID, Code: code, Name: name, Description: description})
}

func (s *Store) ensureGauge(ctx context.Context, actor domain.Actor, in domain.NewGauge) (domain.Gauge, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM gauges WHERE lower(code) = lower($1)`, in.Code); err != nil || ok {
		if err != nil {
			return domain.Gauge{}, err
		}
		return s.GetGauge(ctx, id)
	}
	return s.CreateGauge(ctx, actor, in)
}

func (s *Store) ensureDefect(ctx context.Context, actor domain.Actor, in domain.NewDefectCode) (domain.DefectCode, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM defect_codes WHERE lower(code) = lower($1)`, in.Code); err != nil || ok {
		if err != nil {
			return domain.DefectCode{}, err
		}
		return s.GetDefectCode(ctx, id)
	}
	return s.CreateDefectCode(ctx, actor, in)
}

func (s *Store) ensureProduct(ctx context.Context, actor domain.Actor, in domain.NewProduct) (domain.Product, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM products WHERE lower(part_number) = lower($1)`, in.PartNumber); err != nil || ok {
		if err != nil {
			return domain.Product{}, err
		}
		return s.GetProduct(ctx, id)
	}
	return s.CreateProduct(ctx, actor, in)
}

func (s *Store) ensureActivePlan(ctx context.Context, actor domain.Actor, productID uuid.UUID, name string, chars []domain.NewCharacteristic) (domain.PlanDetail, error) {
	active, ok, err := s.lookupID(ctx, `SELECT id FROM inspection_plans WHERE product_id = $1 AND status = 'active'`, productID)
	if err != nil {
		return domain.PlanDetail{}, err
	}
	if ok {
		return s.GetPlan(ctx, active)
	}
	id, ok, err := s.lookupID(ctx, `
		SELECT id FROM inspection_plans
		WHERE product_id = $1 AND lower(revision) = 'a'`, productID)
	if err != nil {
		return domain.PlanDetail{}, err
	}
	if !ok {
		plan, err := s.CreatePlan(ctx, actor, domain.NewPlan{
			ProductID: productID, Revision: "A", Name: name,
			SamplingType:    domain.SampleHundred,
			Notes:           "Check every unit in the lot. A dimension with no stored limit is compared to the print for that build.",
			Characteristics: chars,
		})
		if err != nil {
			return domain.PlanDetail{}, err
		}
		return s.ActivatePlan(ctx, actor, plan.ID)
	}
	plan, err := s.GetPlan(ctx, id)
	if err != nil {
		return domain.PlanDetail{}, err
	}
	if plan.Status == domain.PlanDraft {
		return s.ActivatePlan(ctx, actor, plan.ID)
	}
	return plan, nil
}

func (s *Store) ensureLot(ctx context.Context, actor domain.Actor, in domain.NewLot) (domain.Lot, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM lots WHERE lower(lot_number) = lower($1)`, in.LotNumber); err != nil || ok {
		if err != nil {
			return domain.Lot{}, err
		}
		return s.GetLot(ctx, id)
	}
	return s.CreateLot(ctx, actor, in)
}

func (s *Store) ensureInspection(ctx context.Context, actor domain.Actor, lotID, planID, stationID uuid.UUID, sample *int, notes string) (uuid.UUID, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM inspections WHERE lot_id = $1 ORDER BY created_at LIMIT 1`, lotID); err != nil || ok {
		return id, err
	}
	detail, err := s.CreateInspection(ctx, actor, domain.NewInspection{
		LotID: lotID, PlanID: &planID, StationID: &stationID, SampleSize: sample, Notes: notes,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return detail.Inspection.ID, nil
}

func (s *Store) ensureChecklist(ctx context.Context, actor domain.Actor, in domain.NewChecklist) (domain.ChecklistDetail, error) {
	if id, ok, err := s.lookupID(ctx, `SELECT id FROM checklists WHERE lower(code) = lower($1)`, in.Code); err != nil || ok {
		if err != nil {
			return domain.ChecklistDetail{}, err
		}
		return s.GetChecklist(ctx, id)
	}
	return s.CreateChecklist(ctx, actor, in)
}
