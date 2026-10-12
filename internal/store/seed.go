package store

import (
	"context"
	"fmt"
	"time"

	"quality_control/internal/domain"

	"github.com/google/uuid"
)

// SeedDemo creates a coherent demo plant dataset for local demos and UI work.
// It is additive and does not delete existing records.
func (s *Store) SeedDemo(ctx context.Context, actor domain.Actor) (domain.DemoSeedResult, error) {
	suffix := time.Now().UTC().Format("150405")
	site, err := s.CreateSite(ctx, actor, domain.NewSite{
		Code: "DEMO-" + suffix, Name: "Demo Plant", Address: "1 Quality Way",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	station, err := s.CreateStation(ctx, actor, domain.NewStation{
		SiteID: site.ID, Code: "QC-1", Name: "Incoming inspection", Description: "Demo station",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	supplier, err := s.CreateSupplier(ctx, actor, domain.NewSupplier{
		Code: "DEMO-SUP-" + suffix, Name: "Demo Metals", ContactName: "Pat Supplier", Email: "supplier@example.com",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	product, err := s.CreateProduct(ctx, actor, domain.NewProduct{
		PartNumber: "DEMO-BRACKET-" + suffix, Name: "Demo Bracket", Description: "Seeded demo part", Unit: "ea",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	due := time.Now().UTC().AddDate(0, 3, 0).Format("2006-01-02")
	if _, err := s.CreateGauge(ctx, actor, domain.NewGauge{
		Code: "DEMO-CAL-" + suffix, Name: "Demo Caliper", SerialNumber: "SN-DEMO", CalibrationDue: due,
	}); err != nil {
		return domain.DemoSeedResult{}, err
	}
	defect, err := s.CreateDefectCode(ctx, actor, domain.NewDefectCode{
		Code: "DEMO-SCR-" + suffix, Name: "Demo scratch", Severity: domain.SeverityMinor, Category: "visual",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	sample := 2
	plan, err := s.CreatePlan(ctx, actor, domain.NewPlan{
		ProductID: product.ID, Revision: "A", Name: "Demo incoming plan",
		SamplingType: domain.SampleFixed, SampleSize: &sample,
		Characteristics: []domain.NewCharacteristic{
			{Sequence: 1, Code: "LEN", Name: "Length", CharacteristicType: domain.CharVariable,
				Nominal: floatPtr(10), LowerLimit: floatPtr(9.8), UpperLimit: floatPtr(10.2), Unit: "mm", Critical: true},
			{Sequence: 2, Code: "VIS", Name: "Visual", CharacteristicType: domain.CharAttribute},
		},
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	plan, err = s.ActivatePlan(ctx, actor, plan.ID)
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	lot, err := s.CreateLot(ctx, actor, domain.NewLot{
		LotNumber: "DEMO-LOT-" + suffix, ProductID: product.ID, SiteID: site.ID, SupplierID: &supplier.ID,
		Quantity: 100, Unit: "ea", Source: domain.SourceIncoming, SupplierLot: "SUP-42", Notes: "Seeded demo lot",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	inspSample := 2
	insp, err := s.CreateInspection(ctx, actor, domain.NewInspection{
		LotID: lot.ID, PlanID: &plan.ID, StationID: &station.ID, InspectorID: &actor.ID, SampleSize: &inspSample, Notes: "Seeded inspection",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	var lengthID, visualID uuid.UUID
	for _, ch := range plan.Characteristics {
		switch ch.Code {
		case "LEN":
			lengthID = ch.ID
		case "VIS":
			visualID = ch.ID
		}
	}
	if _, err := s.RecordResults(ctx, actor, insp.ID, domain.ResultBatch{Results: []domain.ResultInput{
		{CharacteristicID: lengthID, SampleIndex: 1, MeasuredValue: floatPtr(10.0)},
		{CharacteristicID: lengthID, SampleIndex: 2, MeasuredValue: floatPtr(10.05)},
		{CharacteristicID: visualID, SampleIndex: 1, AttributeResult: strPtr("pass")},
		{CharacteristicID: visualID, SampleIndex: 2, AttributeResult: strPtr("pass")},
	}}); err != nil {
		return domain.DemoSeedResult{}, err
	}
	if _, err := s.AddInspectionDefect(ctx, actor, insp.ID, domain.NewInspectionDefect{
		DefectCodeID: defect.ID, Quantity: 1, Notes: "Cosmetic mark on edge",
	}); err != nil {
		return domain.DemoSeedResult{}, err
	}
	openNCR := true
	qtyInsp, qtyAcc, qtyRej := 2.0, 1.0, 1.0
	completed, err := s.CompleteInspection(ctx, actor, insp.ID, domain.CompleteInspection{
		QuantityInspected: &qtyInsp, QuantityAccepted: &qtyAcc, QuantityRejected: &qtyRej,
		OpenNCR: &openNCR, Notes: "Seeded complete with minor defect",
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	ncrID := uuid.Nil
	if completed.Nonconformance != nil {
		ncrID = completed.Nonconformance.ID
	} else {
		ncr, err := s.CreateNCR(ctx, actor, domain.NewNonconformance{
			InspectionID: &insp.ID, LotID: &lot.ID, ProductID: product.ID,
			Title: "Demo nonconformance", Description: "Seeded NCR for CAPA demo",
			Severity: domain.SeverityMinor, QuantityAffected: 1,
		})
		if err != nil {
			return domain.DemoSeedResult{}, err
		}
		ncrID = ncr.ID
	}
	dueDate := time.Now().UTC().AddDate(0, 0, 14).Format("2006-01-02")
	capa, err := s.CreateCAPA(ctx, actor, domain.NewCorrectiveAction{
		NonconformanceID: &ncrID, Title: "Demo corrective action",
		RootCause: "Handling damage at supplier", ActionPlan: "Add edge protection in packaging",
		OwnerID: &actor.ID, DueDate: dueDate,
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	optional := false
	checklist, err := s.CreateChecklist(ctx, actor, domain.NewChecklist{
		Code: "DEMO-CL-" + suffix, Name: "Demo visual checklist", Description: "Seeded checklist",
		ProductID: &product.ID,
		Items: []domain.NewChecklistItem{
			{Sequence: 1, Prompt: "Part number label present?", ItemType: domain.ChecklistItemYesNo},
			{Sequence: 2, Prompt: "Surface finish acceptable?", ItemType: domain.ChecklistItemPassFail},
			{Sequence: 3, Prompt: "Notes", ItemType: domain.ChecklistItemText, Required: &optional},
		},
	})
	if err != nil {
		return domain.DemoSeedResult{}, err
	}
	summary := fmt.Sprintf("Seeded demo site %s, product %s, lot %s, inspection %s, NCR, CAPA, checklist %s",
		site.Code, product.PartNumber, lot.LotNumber, insp.InspectionNumber, checklist.Code)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO demo_seed_runs (id, seeded_by, summary) VALUES ($1,$2,$3)`,
		uuid.New(), nullUUID(actor.ID), summary); err != nil {
		return domain.DemoSeedResult{}, mapErr(err)
	}
	return domain.DemoSeedResult{
		SiteID: site.ID, ProductID: product.ID, PlanID: plan.ID, LotID: lot.ID,
		InspectionID: insp.ID, NCRID: ncrID, CAPAID: capa.ID, ChecklistID: checklist.ID,
		Summary: summary,
	}, nil
}

func floatPtr(n float64) *float64 { return &n }
func strPtr(s string) *string     { return &s }

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
