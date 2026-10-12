// Package domain is the quality-control vocabulary shared by storage and HTTP.
// JSON field names are the API contract.
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	RoleViewer         = "viewer"
	RoleOperator       = "operator"
	RoleInspector      = "inspector"
	RoleSupervisor     = "supervisor"
	RoleQualityManager = "quality_manager"
	RoleAdmin          = "admin"
)

const (
	SourceIncoming       = "incoming"
	SourceInProcess      = "in_process"
	SourceFinal          = "final"
	SourceCustomerReturn = "customer_return"

	LotOpen         = "open"
	LotInInspection = "in_inspection"
	LotAccepted     = "accepted"
	LotRejected     = "rejected"
	LotOnHold       = "on_hold"
	LotClosed       = "closed"

	PlanDraft    = "draft"
	PlanActive   = "active"
	PlanObsolete = "obsolete"

	SampleHundred = "hundred_percent"
	SampleFixed   = "fixed"
	SampleAQL     = "aql"

	CharVariable  = "variable"
	CharAttribute = "attribute"

	InspInProgress = "in_progress"
	InspCompleted  = "completed"
	InspCancelled  = "cancelled"

	ResultPending     = "pending"
	ResultPass        = "pass"
	ResultFail        = "fail"
	ResultConditional = "conditional"

	SeverityMinor    = "minor"
	SeverityMajor    = "major"
	SeverityCritical = "critical"

	NCROpen          = "open"
	NCRUnderReview   = "under_review"
	NCRDispositioned = "dispositioned"
	NCRClosed        = "closed"
	NCRVoid          = "void"

	DispPending          = "pending"
	DispUseAsIs          = "use_as_is"
	DispRework           = "rework"
	DispRepair           = "repair"
	DispScrap            = "scrap"
	DispReturnToSupplier = "return_to_supplier"
	DispSort             = "sort"

	CAPAOpen         = "open"
	CAPAInProgress   = "in_progress"
	CAPAVerification = "verification"
	CAPAClosed       = "closed"
	CAPACancelled    = "cancelled"

	AttachLot            = "lot"
	AttachInspection     = "inspection"
	AttachNonconformance = "nonconformance"
	AttachCAPA           = "corrective_action"
)

// Rank orders roles. A higher rank includes every lower rank's writes.
func Rank(role string) int {
	switch role {
	case RoleOperator:
		return 1
	case RoleInspector:
		return 2
	case RoleSupervisor:
		return 3
	case RoleQualityManager:
		return 4
	case RoleAdmin:
		return 5
	default:
		return 0
	}
}

// AtLeast reports whether role is allowed to perform an action whose
// minimum role is min.
func AtLeast(role, min string) bool {
	return Rank(role) >= Rank(min)
}

// Actor is the person and request behind a write, copied into the audit log.
type Actor struct {
	ID        uuid.UUID
	Role      string
	IP        string
	RequestID string
}

// IsAdmin reports whether the caller may override status locks on records.
func (a Actor) IsAdmin() bool { return a.Role == RoleAdmin }

// Principal is the authenticated caller for one request.
type Principal struct {
	ID        uuid.UUID
	Username  string
	Role      string
	IP        string
	RequestID string
}

// Actor returns the audit identity for this caller.
func (p Principal) Actor() Actor {
	return Actor{ID: p.ID, Role: p.Role, IP: p.IP, RequestID: p.RequestID}
}

// User is an account. Password hashes never appear on this struct.
type User struct {
	ID          uuid.UUID  `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	Role        string     `json:"role"`
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Site is a plant or warehouse.
type Site struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Station is a line, bench, or cell inside a site.
type Station struct {
	ID          uuid.UUID `json:"id"`
	SiteID      uuid.UUID `json:"site_id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Supplier is a vendor for incoming material.
type Supplier struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	ContactName string    `json:"contact_name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Gauge is a measuring instrument. CalibrationDue is YYYY-MM-DD or empty.
type Gauge struct {
	ID             uuid.UUID `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	SerialNumber   string    `json:"serial_number"`
	CalibrationDue string    `json:"calibration_due"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Product is a part that can be inspected.
type Product struct {
	ID          uuid.UUID `json:"id"`
	PartNumber  string    `json:"part_number"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Unit        string    `json:"unit"`
	Active      bool      `json:"active"`
	CreatedBy   uuid.UUID `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DefectCode is the plant defect catalog.
type DefectCode struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Severity  string    `json:"severity"`
	Category  string    `json:"category"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InspectionPlan is a controlled revision of what to check on a product.
type InspectionPlan struct {
	ID           uuid.UUID  `json:"id"`
	ProductID    uuid.UUID  `json:"product_id"`
	Revision     string     `json:"revision"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	SamplingType string     `json:"sampling_type"`
	AQL          string     `json:"aql"`
	SampleSize   *int       `json:"sample_size"`
	Notes        string     `json:"notes"`
	CreatedBy    uuid.UUID  `json:"created_by"`
	ApprovedBy   *uuid.UUID `json:"approved_by"`
	ApprovedAt   *time.Time `json:"approved_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PlanDetail is a plan plus its characteristics.
type PlanDetail struct {
	InspectionPlan
	Characteristics []Characteristic `json:"characteristics"`
}

// Characteristic is one check on a plan. Limits are inclusive.
type Characteristic struct {
	ID                 uuid.UUID `json:"id"`
	PlanID             uuid.UUID `json:"plan_id"`
	Sequence           int       `json:"sequence"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	CharacteristicType string    `json:"characteristic_type"`
	Method             string    `json:"method"`
	Nominal            *float64  `json:"nominal"`
	LowerLimit         *float64  `json:"lower_limit"`
	UpperLimit         *float64  `json:"upper_limit"`
	Unit               string    `json:"unit"`
	Critical           bool      `json:"critical"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Lot is a batch of a product at a site.
type Lot struct {
	ID          uuid.UUID  `json:"id"`
	LotNumber   string     `json:"lot_number"`
	ProductID   uuid.UUID  `json:"product_id"`
	SiteID      uuid.UUID  `json:"site_id"`
	SupplierID  *uuid.UUID `json:"supplier_id"`
	Quantity    float64    `json:"quantity"`
	Unit        string     `json:"unit"`
	Source      string     `json:"source"`
	SupplierLot string     `json:"supplier_lot"`
	Status      string     `json:"status"`
	ReceivedAt  time.Time  `json:"received_at"`
	Notes       string     `json:"notes"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Inspection is one execution of a plan against a lot.
type Inspection struct {
	ID                uuid.UUID  `json:"id"`
	InspectionNumber  string     `json:"inspection_number"`
	LotID             uuid.UUID  `json:"lot_id"`
	PlanID            uuid.UUID  `json:"plan_id"`
	StationID         *uuid.UUID `json:"station_id"`
	InspectorID       uuid.UUID  `json:"inspector_id"`
	Status            string     `json:"status"`
	Result            string     `json:"result"`
	SampleSize        int        `json:"sample_size"`
	QuantityInspected float64    `json:"quantity_inspected"`
	QuantityAccepted  float64    `json:"quantity_accepted"`
	QuantityRejected  float64    `json:"quantity_rejected"`
	Notes             string     `json:"notes"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	CreatedBy         uuid.UUID  `json:"created_by"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// InspectionDetail is the inspection plus the rows needed to finish it.
type InspectionDetail struct {
	Inspection
	Results                []InspectionResult `json:"results"`
	Defects                []InspectionDefect `json:"defects"`
	MissingCharacteristics []string           `json:"missing_characteristics"`
}

// InspectionResult is one sample of one characteristic.
// Limits are copied at record time so later plan edits cannot rewrite history.
type InspectionResult struct {
	ID                 uuid.UUID  `json:"id"`
	InspectionID       uuid.UUID  `json:"inspection_id"`
	CharacteristicID   uuid.UUID  `json:"characteristic_id"`
	SampleIndex        int        `json:"sample_index"`
	MeasuredValue      *float64   `json:"measured_value"`
	AttributeResult    *string    `json:"attribute_result"`
	InSpec             bool       `json:"in_spec"`
	CalibrationWarning bool       `json:"calibration_warning"`
	Critical           bool       `json:"critical"`
	Nominal            *float64   `json:"nominal"`
	LowerLimit         *float64   `json:"lower_limit"`
	UpperLimit         *float64   `json:"upper_limit"`
	Unit               string     `json:"unit"`
	CharacteristicCode string     `json:"characteristic_code"`
	CharacteristicName string     `json:"characteristic_name"`
	CharacteristicType string     `json:"characteristic_type"`
	GaugeID            *uuid.UUID `json:"gauge_id"`
	Notes              string     `json:"notes"`
	RecordedBy         uuid.UUID  `json:"recorded_by"`
	RecordedAt         time.Time  `json:"recorded_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// InspectionDefect is a catalog defect found during an inspection.
// Code, name, and severity are copied from the catalog at record time.
type InspectionDefect struct {
	ID           uuid.UUID `json:"id"`
	InspectionID uuid.UUID `json:"inspection_id"`
	DefectCodeID uuid.UUID `json:"defect_code_id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Severity     string    `json:"severity"`
	Quantity     int       `json:"quantity"`
	Notes        string    `json:"notes"`
	RecordedBy   uuid.UUID `json:"recorded_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// Nonconformance is an NCR.
type Nonconformance struct {
	ID               uuid.UUID  `json:"id"`
	NCRNumber        string     `json:"ncr_number"`
	InspectionID     *uuid.UUID `json:"inspection_id"`
	LotID            *uuid.UUID `json:"lot_id"`
	ProductID        uuid.UUID  `json:"product_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Severity         string     `json:"severity"`
	Status           string     `json:"status"`
	Disposition      string     `json:"disposition"`
	QuantityAffected float64    `json:"quantity_affected"`
	AssignedTo       *uuid.UUID `json:"assigned_to"`
	OpenedBy         uuid.UUID  `json:"opened_by"`
	DisposedBy       *uuid.UUID `json:"disposed_by"`
	ClosedBy         *uuid.UUID `json:"closed_by"`
	DispositionNotes string     `json:"disposition_notes"`
	OpenedAt         time.Time  `json:"opened_at"`
	DisposedAt       *time.Time `json:"disposed_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// NCRDetail is an NCR plus its corrective actions.
type NCRDetail struct {
	Nonconformance
	CorrectiveActions []CorrectiveAction `json:"corrective_actions"`
}

// CorrectiveAction is a CAPA, optionally tied to an NCR.
type CorrectiveAction struct {
	ID                 uuid.UUID  `json:"id"`
	CANumber           string     `json:"ca_number"`
	NonconformanceID   *uuid.UUID `json:"nonconformance_id"`
	Title              string     `json:"title"`
	RootCause          string     `json:"root_cause"`
	ActionPlan         string     `json:"action_plan"`
	Status             string     `json:"status"`
	OwnerID            *uuid.UUID `json:"owner_id"`
	DueDate            string     `json:"due_date"`
	VerificationNotes  string     `json:"verification_notes"`
	EffectivenessNotes string     `json:"effectiveness_notes"`
	OpenedBy           uuid.UUID  `json:"opened_by"`
	VerifiedBy         *uuid.UUID `json:"verified_by"`
	ClosedBy           *uuid.UUID `json:"closed_by"`
	OpenedAt           time.Time  `json:"opened_at"`
	VerifiedAt         *time.Time `json:"verified_at"`
	ClosedAt           *time.Time `json:"closed_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Attachment is metadata for a file stored under the data directory.
type Attachment struct {
	ID          uuid.UUID `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    uuid.UUID `json:"entity_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	UploadedBy  uuid.UUID `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditEntry is one mutation.
type AuditEntry struct {
	ID         int64           `json:"id"`
	ActorID    *uuid.UUID      `json:"actor_id"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   *uuid.UUID      `json:"entity_id"`
	Summary    string          `json:"summary"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
	IP         string          `json:"ip"`
	RequestID  string          `json:"request_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Measurement is one completed variable reading, shaped for charting.
type Measurement struct {
	RecordedAt         time.Time `json:"recorded_at"`
	LotNumber          string    `json:"lot_number"`
	InspectionNumber   string    `json:"inspection_number"`
	CharacteristicCode string    `json:"characteristic_code"`
	CharacteristicName string    `json:"characteristic_name"`
	SampleIndex        int       `json:"sample_index"`
	MeasuredValue      float64   `json:"measured_value"`
	InSpec             bool      `json:"in_spec"`
	Unit               string    `json:"unit"`
	Nominal            *float64  `json:"nominal"`
	LowerLimit         *float64  `json:"lower_limit"`
	UpperLimit         *float64  `json:"upper_limit"`
	Critical           bool      `json:"critical"`
}

// Dashboard is the plant snapshot.
type Dashboard struct {
	OpenInspections          int             `json:"open_inspections"`
	LotsOnHold               int             `json:"lots_on_hold"`
	OpenNonconformances      int             `json:"open_nonconformances"`
	OpenCorrectiveActions    int             `json:"open_corrective_actions"`
	OverdueCorrectiveActions int             `json:"overdue_corrective_actions"`
	InspectionsLast30Days    int             `json:"inspections_last_30_days"`
	PassLast30Days           int             `json:"pass_last_30_days"`
	FailLast30Days           int             `json:"fail_last_30_days"`
	ConditionalLast30Days    int             `json:"conditional_last_30_days"`
	PassRateLast30Days       *float64        `json:"pass_rate_last_30_days"`
	DefectsBySeverity        []SeverityCount `json:"defects_by_severity"`
}

// SeverityCount is a defect tally.
type SeverityCount struct {
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

// Page is a limit/offset window. Total is the unpaged match count.
type Page struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// Enums returns the closed sets a client can use to build forms.
func Enums() map[string][]string {
	return map[string][]string{
		"roles":                {RoleViewer, RoleOperator, RoleInspector, RoleSupervisor, RoleQualityManager, RoleAdmin},
		"lot_sources":          {SourceIncoming, SourceInProcess, SourceFinal, SourceCustomerReturn},
		"lot_statuses":         {LotOpen, LotInInspection, LotAccepted, LotRejected, LotOnHold, LotClosed},
		"plan_statuses":        {PlanDraft, PlanActive, PlanObsolete},
		"sampling_types":       {SampleHundred, SampleFixed, SampleAQL},
		"characteristic_types": {CharVariable, CharAttribute},
		"inspection_statuses":  {InspInProgress, InspCompleted, InspCancelled},
		"inspection_results":   {ResultPending, ResultPass, ResultFail, ResultConditional},
		"severities":           {SeverityMinor, SeverityMajor, SeverityCritical},
		"ncr_statuses":         {NCROpen, NCRUnderReview, NCRDispositioned, NCRClosed, NCRVoid},
		"dispositions":         {DispPending, DispUseAsIs, DispRework, DispRepair, DispScrap, DispReturnToSupplier, DispSort},
		"capa_statuses":        {CAPAOpen, CAPAInProgress, CAPAVerification, CAPAClosed, CAPACancelled},
		"attachment_entities":  {AttachLot, AttachInspection, AttachNonconformance, AttachCAPA},
	}
}
