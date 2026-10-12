package domain

import "github.com/google/uuid"

// NewUser is an account to create.
type NewUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

// PatchUser changes account fields. Username is permanent.
type PatchUser struct {
	Email    *string `json:"email"`
	FullName *string `json:"full_name"`
	Role     *string `json:"role"`
	Active   *bool   `json:"active"`
}

// PasswordChange is a self-service password update.
type PasswordChange struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// PasswordReset is an administrator setting a new password.
type PasswordReset struct {
	Password string `json:"password"`
}

// NewSite is a plant.
type NewSite struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// PatchSite updates a plant.
type PatchSite struct {
	Code    *string `json:"code"`
	Name    *string `json:"name"`
	Address *string `json:"address"`
	Active  *bool   `json:"active"`
}

// NewStation is a work center.
type NewStation struct {
	SiteID      uuid.UUID `json:"site_id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
}

// PatchStation updates a work center.
type PatchStation struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Active      *bool   `json:"active"`
}

// NewSupplier is a vendor.
type NewSupplier struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	ContactName string `json:"contact_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

// PatchSupplier updates a vendor.
type PatchSupplier struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	ContactName *string `json:"contact_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	Active      *bool   `json:"active"`
}

// NewGauge is an instrument.
type NewGauge struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	SerialNumber   string `json:"serial_number"`
	CalibrationDue string `json:"calibration_due"`
}

// PatchGauge updates an instrument.
type PatchGauge struct {
	Code           *string `json:"code"`
	Name           *string `json:"name"`
	SerialNumber   *string `json:"serial_number"`
	CalibrationDue *string `json:"calibration_due"`
	Active         *bool   `json:"active"`
}

// NewProduct is a part.
type NewProduct struct {
	PartNumber  string `json:"part_number"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Unit        string `json:"unit"`
}

// PatchProduct updates a part.
type PatchProduct struct {
	PartNumber  *string `json:"part_number"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Unit        *string `json:"unit"`
	Active      *bool   `json:"active"`
}

// NewDefectCode is a catalog entry.
type NewDefectCode struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Category string `json:"category"`
}

// PatchDefectCode updates a catalog entry.
type PatchDefectCode struct {
	Code     *string `json:"code"`
	Name     *string `json:"name"`
	Severity *string `json:"severity"`
	Category *string `json:"category"`
	Active   *bool   `json:"active"`
}

// NewCharacteristic is one check, used when creating or revising a plan.
type NewCharacteristic struct {
	Sequence           int      `json:"sequence"`
	Code               string   `json:"code"`
	Name               string   `json:"name"`
	CharacteristicType string   `json:"characteristic_type"`
	Method             string   `json:"method"`
	Nominal            *float64 `json:"nominal"`
	LowerLimit         *float64 `json:"lower_limit"`
	UpperLimit         *float64 `json:"upper_limit"`
	Unit               string   `json:"unit"`
	Critical           bool     `json:"critical"`
}

// NewPlan is a draft inspection plan.
type NewPlan struct {
	ProductID       uuid.UUID           `json:"product_id"`
	Revision        string              `json:"revision"`
	Name            string              `json:"name"`
	SamplingType    string              `json:"sampling_type"`
	AQL             string              `json:"aql"`
	SampleSize      *int                `json:"sample_size"`
	Notes           string              `json:"notes"`
	Characteristics []NewCharacteristic `json:"characteristics"`
}

// PatchPlan edits a draft plan's header.
type PatchPlan struct {
	Name         *string `json:"name"`
	SamplingType *string `json:"sampling_type"`
	AQL          *string `json:"aql"`
	SampleSize   *int    `json:"sample_size"`
	ClearSample  bool    `json:"clear_sample_size"`
	Notes        *string `json:"notes"`
}

// PatchCharacteristic edits one check on a draft plan.
type PatchCharacteristic struct {
	Sequence           *int     `json:"sequence"`
	Code               *string  `json:"code"`
	Name               *string  `json:"name"`
	CharacteristicType *string  `json:"characteristic_type"`
	Method             *string  `json:"method"`
	Nominal            *float64 `json:"nominal"`
	ClearNominal       bool     `json:"clear_nominal"`
	LowerLimit         *float64 `json:"lower_limit"`
	ClearLower         bool     `json:"clear_lower_limit"`
	UpperLimit         *float64 `json:"upper_limit"`
	ClearUpper         bool     `json:"clear_upper_limit"`
	Unit               *string  `json:"unit"`
	Critical           *bool    `json:"critical"`
}

// RevisePlan copies a plan forward as a new draft.
type RevisePlan struct {
	Revision string `json:"revision"`
}

// NewLot is a batch to receive or open.
type NewLot struct {
	LotNumber   string     `json:"lot_number"`
	ProductID   uuid.UUID  `json:"product_id"`
	SiteID      uuid.UUID  `json:"site_id"`
	SupplierID  *uuid.UUID `json:"supplier_id"`
	Quantity    float64    `json:"quantity"`
	Unit        string     `json:"unit"`
	Source      string     `json:"source"`
	SupplierLot string     `json:"supplier_lot"`
	Notes       string     `json:"notes"`
}

// PatchLot edits a lot. Lot number changes are allowed only while the lot is open.
type PatchLot struct {
	LotNumber   *string    `json:"lot_number"`
	SupplierID  *uuid.UUID `json:"supplier_id"`
	Quantity    *float64   `json:"quantity"`
	Unit        *string    `json:"unit"`
	SupplierLot *string    `json:"supplier_lot"`
	Notes       *string    `json:"notes"`
}

// PatchInspection edits the header of an inspection.
type PatchInspection struct {
	Notes        *string    `json:"notes"`
	StationID    *uuid.UUID `json:"station_id"`
	ClearStation bool       `json:"clear_station"`
}

// LotStatusChange is a manual lot move.
type LotStatusChange struct {
	Status string `json:"status"`
}

// NewInspection starts a check against a lot.
type NewInspection struct {
	LotID       uuid.UUID  `json:"lot_id"`
	PlanID      *uuid.UUID `json:"plan_id"`
	StationID   *uuid.UUID `json:"station_id"`
	InspectorID *uuid.UUID `json:"inspector_id"`
	SampleSize  *int       `json:"sample_size"`
	Notes       string     `json:"notes"`
}

// ResultInput is one sample to record. The server decides in_spec.
type ResultInput struct {
	CharacteristicID uuid.UUID  `json:"characteristic_id"`
	SampleIndex      int        `json:"sample_index"`
	MeasuredValue    *float64   `json:"measured_value"`
	AttributeResult  *string    `json:"attribute_result"`
	GaugeID          *uuid.UUID `json:"gauge_id"`
	Notes            string     `json:"notes"`
}

// ResultBatch records many samples in one transaction.
type ResultBatch struct {
	Results []ResultInput `json:"results"`
}

// NewInspectionDefect records a defect against an open inspection.
type NewInspectionDefect struct {
	DefectCodeID uuid.UUID `json:"defect_code_id"`
	Quantity     int       `json:"quantity"`
	Notes        string    `json:"notes"`
}

// CompleteInspection closes an inspection and updates the lot.
// OpenNCR nil means: open an NCR when the result is fail.
type CompleteInspection struct {
	QuantityInspected *float64 `json:"quantity_inspected"`
	QuantityAccepted  *float64 `json:"quantity_accepted"`
	QuantityRejected  *float64 `json:"quantity_rejected"`
	Notes             string   `json:"notes"`
	OpenNCR           *bool    `json:"open_ncr"`
}

// CompletedInspection is the closed inspection and the NCR it opened, if any.
type CompletedInspection struct {
	Inspection     Inspection      `json:"inspection"`
	Nonconformance *Nonconformance `json:"nonconformance"`
}

// NewNonconformance opens an NCR by hand.
type NewNonconformance struct {
	InspectionID     *uuid.UUID `json:"inspection_id"`
	LotID            *uuid.UUID `json:"lot_id"`
	ProductID        uuid.UUID  `json:"product_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Severity         string     `json:"severity"`
	QuantityAffected float64    `json:"quantity_affected"`
	AssignedTo       *uuid.UUID `json:"assigned_to"`
}

// PatchNonconformance edits an NCR that is not closed or void.
type PatchNonconformance struct {
	Title            *string    `json:"title"`
	Description      *string    `json:"description"`
	Severity         *string    `json:"severity"`
	QuantityAffected *float64   `json:"quantity_affected"`
	AssignedTo       *uuid.UUID `json:"assigned_to"`
}

// DispositionRequest records what will happen to the nonconforming material.
type DispositionRequest struct {
	Disposition string `json:"disposition"`
	Notes       string `json:"notes"`
}

// NewCorrectiveAction opens a CAPA.
type NewCorrectiveAction struct {
	NonconformanceID *uuid.UUID `json:"nonconformance_id"`
	Title            string     `json:"title"`
	RootCause        string     `json:"root_cause"`
	ActionPlan       string     `json:"action_plan"`
	OwnerID          *uuid.UUID `json:"owner_id"`
	DueDate          string     `json:"due_date"`
}

// PatchCorrectiveAction edits a CAPA that is still open.
type PatchCorrectiveAction struct {
	Title      *string    `json:"title"`
	RootCause  *string    `json:"root_cause"`
	ActionPlan *string    `json:"action_plan"`
	OwnerID    *uuid.UUID `json:"owner_id"`
	DueDate    *string    `json:"due_date"`
}

// NoteBody carries the text required by a status action.
type NoteBody struct {
	Notes string `json:"notes"`
}
