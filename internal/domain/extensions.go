package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ChecklistItemPassFail = "pass_fail"
	ChecklistItemYesNo    = "yes_no"
	ChecklistItemText     = "text"
	ChecklistItemNumber   = "number"

	ChecklistRunInProgress = "in_progress"
	ChecklistRunCompleted  = "completed"
	ChecklistRunCancelled  = "cancelled"

	WebhookPending   = "pending"
	WebhookDelivered = "delivered"
	WebhookFailed    = "failed"
	WebhookSkipped   = "skipped"

	NotifyEmail   = "email"
	NotifyWebhook = "webhook"
	NotifyPending = "pending"
	NotifySent    = "sent"
	NotifyFailed  = "failed"
	NotifySkipped = "skipped"
)

// Known webhook / notification event types.
const (
	EventInspectionCompleted = "inspection.completed"
	EventNCRCreated          = "ncr.created"
	EventNCRDispositioned    = "ncr.dispositioned"
	EventCAPACreated         = "capa.created"
	EventCAPAOverdue         = "capa.overdue"
	EventChecklistCompleted  = "checklist.completed"
	EventLotStatusChanged    = "lot.status_changed"
)

// RequestAuditEntry is one HTTP access record (additive log, separate from audit_log).
type RequestAuditEntry struct {
	ID         int64      `json:"id"`
	ActorID    *uuid.UUID `json:"actor_id,omitempty"`
	Method     string     `json:"method"`
	Path       string     `json:"path"`
	StatusCode int        `json:"status_code"`
	DurationMS int        `json:"duration_ms"`
	IP         string     `json:"ip"`
	RequestID  string     `json:"request_id"`
	UserAgent  string     `json:"user_agent"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Checklist is an inspection checklist template.
type Checklist struct {
	ID          uuid.UUID  `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ProductID   *uuid.UUID `json:"product_id,omitempty"`
	Active      bool       `json:"active"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ChecklistItem is one step on a template.
type ChecklistItem struct {
	ID          uuid.UUID `json:"id"`
	ChecklistID uuid.UUID `json:"checklist_id"`
	Sequence    int       `json:"sequence"`
	Prompt      string    `json:"prompt"`
	ItemType    string    `json:"item_type"`
	Required    bool      `json:"required"`
	HelpText    string    `json:"help_text"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChecklistDetail is a template with its items.
type ChecklistDetail struct {
	Checklist
	Items []ChecklistItem `json:"items"`
}

// NewChecklist creates a template.
type NewChecklist struct {
	Code        string             `json:"code"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	ProductID   *uuid.UUID         `json:"product_id"`
	Items       []NewChecklistItem `json:"items"`
}

// NewChecklistItem is a template step on create.
type NewChecklistItem struct {
	Sequence int    `json:"sequence"`
	Prompt   string `json:"prompt"`
	ItemType string `json:"item_type"`
	Required *bool  `json:"required"`
	HelpText string `json:"help_text"`
}

// PatchChecklist updates a template header.
type PatchChecklist struct {
	Code        *string    `json:"code"`
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	ProductID   *uuid.UUID `json:"product_id"`
	Active      *bool      `json:"active"`
}

// NewChecklistItemBody adds a step to an existing template.
type NewChecklistItemBody struct {
	Sequence int    `json:"sequence"`
	Prompt   string `json:"prompt"`
	ItemType string `json:"item_type"`
	Required *bool  `json:"required"`
	HelpText string `json:"help_text"`
}

// PatchChecklistItem updates a template step.
type PatchChecklistItem struct {
	Sequence *int    `json:"sequence"`
	Prompt   *string `json:"prompt"`
	ItemType *string `json:"item_type"`
	Required *bool   `json:"required"`
	HelpText *string `json:"help_text"`
}

// ChecklistRun is a filled checklist on an inspection.
type ChecklistRun struct {
	ID           uuid.UUID  `json:"id"`
	ChecklistID  uuid.UUID  `json:"checklist_id"`
	InspectionID uuid.UUID  `json:"inspection_id"`
	Status       string     `json:"status"`
	Notes        string     `json:"notes"`
	StartedBy    uuid.UUID  `json:"started_by"`
	CompletedBy  *uuid.UUID `json:"completed_by,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ChecklistRunItem is one answered step.
type ChecklistRunItem struct {
	ID           uuid.UUID  `json:"id"`
	RunID        uuid.UUID  `json:"run_id"`
	ItemID       uuid.UUID  `json:"item_id"`
	Sequence     int        `json:"sequence"`
	Prompt       string     `json:"prompt"`
	ItemType     string     `json:"item_type"`
	Required     bool       `json:"required"`
	Result       string     `json:"result"`
	NumericValue *float64   `json:"numeric_value,omitempty"`
	Notes        string     `json:"notes"`
	AnsweredBy   *uuid.UUID `json:"answered_by,omitempty"`
	AnsweredAt   *time.Time `json:"answered_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ChecklistRunDetail is a run with answers.
type ChecklistRunDetail struct {
	ChecklistRun
	Items []ChecklistRunItem `json:"items"`
}

// StartChecklistRun starts a run for an inspection.
type StartChecklistRun struct {
	ChecklistID uuid.UUID `json:"checklist_id"`
	Notes       string    `json:"notes"`
}

// AnswerChecklistItem records one answer.
type AnswerChecklistItem struct {
	Result       string   `json:"result"`
	NumericValue *float64 `json:"numeric_value"`
	Notes        string   `json:"notes"`
}

// Webhook is an outbound notification subscription.
type Webhook struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Secret      string    `json:"secret,omitempty"`
	Events      []string  `json:"events"`
	Active      bool      `json:"active"`
	Description string    `json:"description"`
	CreatedBy   uuid.UUID `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewWebhook registers a subscription.
type NewWebhook struct {
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Secret      string   `json:"secret"`
	Events      []string `json:"events"`
	Description string   `json:"description"`
}

// PatchWebhook updates a subscription.
type PatchWebhook struct {
	Name        *string  `json:"name"`
	URL         *string  `json:"url"`
	Secret      *string  `json:"secret"`
	Events      []string `json:"events"`
	Active      *bool    `json:"active"`
	Description *string  `json:"description"`
}

// WebhookDelivery is one delivery attempt record.
type WebhookDelivery struct {
	ID           uuid.UUID       `json:"id"`
	WebhookID    uuid.UUID       `json:"webhook_id"`
	EventType    string          `json:"event_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	AttemptCount int             `json:"attempt_count"`
	ResponseCode *int            `json:"response_code,omitempty"`
	ResponseBody string          `json:"response_body,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	DeliveredAt  *time.Time      `json:"delivered_at,omitempty"`
}

// NotificationOutboxItem is a queued email or webhook notification.
type NotificationOutboxItem struct {
	ID           uuid.UUID       `json:"id"`
	Channel      string          `json:"channel"`
	EventType    string          `json:"event_type"`
	Destination  string          `json:"destination"`
	Subject      string          `json:"subject"`
	Body         string          `json:"body"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Status       string          `json:"status"`
	ErrorMessage string          `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	SentAt       *time.Time      `json:"sent_at,omitempty"`
}

// NewEmailNotification queues an email (stored; delivery is out-of-band).
type NewEmailNotification struct {
	Destination string          `json:"destination"`
	Subject     string          `json:"subject"`
	Body        string          `json:"body"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
}

// DefectRateRow is defect rate for one dimension.
type DefectRateRow struct {
	Key            string  `json:"key"`
	Label          string  `json:"label"`
	Inspections    int     `json:"inspections"`
	Failed         int     `json:"failed"`
	Conditional    int     `json:"conditional"`
	DefectQuantity int     `json:"defect_quantity"`
	DefectRate     float64 `json:"defect_rate"`
	FailRate       float64 `json:"fail_rate"`
}

// AgingBucket is a count in an age window.
type AgingBucket struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

// AgingReport is open-item aging.
type AgingReport struct {
	Total   int           `json:"total"`
	Buckets []AgingBucket `json:"buckets"`
	Items   []AgingItem   `json:"items"`
}

// AgingItem is one aged open record.
type AgingItem struct {
	ID       uuid.UUID `json:"id"`
	Number   string    `json:"number"`
	Title    string    `json:"title"`
	Status   string    `json:"status"`
	Severity string    `json:"severity,omitempty"`
	OpenedAt time.Time `json:"opened_at"`
	AgeDays  int       `json:"age_days"`
	DueDate  *string   `json:"due_date,omitempty"`
	Overdue  bool      `json:"overdue"`
}

// TrendPoint is one day in a trend series.
type TrendPoint struct {
	Date        string `json:"date"`
	Inspections int    `json:"inspections"`
	Pass        int    `json:"pass"`
	Fail        int    `json:"fail"`
	Conditional int    `json:"conditional"`
	NCROpened   int    `json:"ncr_opened"`
	Defects     int    `json:"defects"`
}

// TrendReport is daily QC trends.
type TrendReport struct {
	From   time.Time    `json:"from"`
	To     time.Time    `json:"to"`
	Points []TrendPoint `json:"points"`
}

// SearchHit is one unified search result.
type SearchHit struct {
	EntityType string    `json:"entity_type"`
	ID         uuid.UUID `json:"id"`
	Number     string    `json:"number"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Snippet    string    `json:"snippet"`
	CreatedAt  time.Time `json:"created_at"`
}

// DemoSeedResult summarizes seeded demo data.
type DemoSeedResult struct {
	SiteID       uuid.UUID `json:"site_id"`
	ProductID    uuid.UUID `json:"product_id"`
	PlanID       uuid.UUID `json:"plan_id"`
	LotID        uuid.UUID `json:"lot_id"`
	InspectionID uuid.UUID `json:"inspection_id"`
	NCRID        uuid.UUID `json:"ncr_id"`
	CAPAID       uuid.UUID `json:"capa_id"`
	ChecklistID  uuid.UUID `json:"checklist_id"`
	Summary      string    `json:"summary"`
}

// MetricsSnapshot is process counters for JSON metrics.
type MetricsSnapshot struct {
	RequestsTotal    int64            `json:"requests_total"`
	RequestsByStatus map[string]int64 `json:"requests_by_status"`
	UptimeSeconds    int64            `json:"uptime_seconds"`
}

// WebhookEventTypes returns the supported event names.
func WebhookEventTypes() []string {
	return []string{
		EventInspectionCompleted,
		EventNCRCreated,
		EventNCRDispositioned,
		EventCAPACreated,
		EventCAPAOverdue,
		EventChecklistCompleted,
		EventLotStatusChanged,
	}
}

// ChecklistItemTypes returns allowed checklist item types.
func ChecklistItemTypes() []string {
	return []string{ChecklistItemPassFail, ChecklistItemYesNo, ChecklistItemText, ChecklistItemNumber}
}
