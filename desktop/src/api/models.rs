use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Page {
    pub limit: i32,
    pub offset: i32,
    pub total: i32,
}

#[derive(Debug, Clone, Deserialize)]
pub struct DataEnvelope<T> {
    pub data: T,
}

#[derive(Debug, Clone, Deserialize)]
pub struct PageEnvelope<T> {
    pub data: T,
    pub page: Page,
}

#[derive(Debug, Clone, Deserialize)]
pub struct ErrorEnvelope {
    pub error: ErrorBody,
}

#[derive(Debug, Clone, Deserialize)]
pub struct ErrorBody {
    pub code: String,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct User {
    pub id: Uuid,
    pub username: String,
    pub email: String,
    pub full_name: String,
    pub role: String,
    pub active: bool,
    pub last_login_at: Option<DateTime<Utc>>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenResponse {
    pub access_token: String,
    pub refresh_token: String,
    pub token_type: String,
    pub expires_in: i32,
    pub user: User,
}

#[derive(Debug, Clone, Serialize)]
pub struct LoginRequest {
    pub username: String,
    pub password: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Dashboard {
    pub open_inspections: i32,
    pub lots_on_hold: i32,
    pub open_nonconformances: i32,
    pub open_corrective_actions: i32,
    pub overdue_corrective_actions: i32,
    pub inspections_last_30_days: i32,
    pub pass_last_30_days: i32,
    pub fail_last_30_days: i32,
    pub conditional_last_30_days: i32,
    pub pass_rate_last_30_days: Option<f64>,
    pub defects_by_severity: Vec<SeverityCount>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SeverityCount {
    pub severity: String,
    pub count: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Inspection {
    pub id: Uuid,
    pub inspection_number: String,
    pub lot_id: Uuid,
    pub plan_id: Uuid,
    pub station_id: Option<Uuid>,
    pub inspector_id: Uuid,
    pub status: String,
    pub result: String,
    pub sample_size: i32,
    pub quantity_inspected: f64,
    pub quantity_accepted: f64,
    pub quantity_rejected: f64,
    pub notes: String,
    pub started_at: DateTime<Utc>,
    pub completed_at: Option<DateTime<Utc>>,
    pub created_by: Uuid,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InspectionDetail {
    #[serde(flatten)]
    pub inspection: Inspection,
    pub results: Vec<InspectionResult>,
    pub defects: Vec<InspectionDefect>,
    pub missing_characteristics: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InspectionResult {
    pub id: Uuid,
    pub inspection_id: Uuid,
    pub characteristic_id: Uuid,
    pub sample_index: i32,
    pub measured_value: Option<f64>,
    pub attribute_result: Option<String>,
    pub in_spec: bool,
    pub calibration_warning: bool,
    pub critical: bool,
    pub nominal: Option<f64>,
    pub lower_limit: Option<f64>,
    pub upper_limit: Option<f64>,
    pub unit: String,
    pub characteristic_code: String,
    pub characteristic_name: String,
    pub characteristic_type: String,
    pub gauge_id: Option<Uuid>,
    pub notes: String,
    pub recorded_by: Uuid,
    pub recorded_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InspectionDefect {
    pub id: Uuid,
    pub inspection_id: Uuid,
    pub defect_code_id: Uuid,
    pub code: String,
    pub name: String,
    pub severity: String,
    pub quantity: i32,
    pub notes: String,
    pub recorded_by: Uuid,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize)]
pub struct NewInspection {
    pub lot_id: Uuid,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub plan_id: Option<Uuid>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub station_id: Option<Uuid>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub inspector_id: Option<Uuid>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_size: Option<i32>,
    pub notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Nonconformance {
    pub id: Uuid,
    pub ncr_number: String,
    pub inspection_id: Option<Uuid>,
    pub lot_id: Option<Uuid>,
    pub product_id: Uuid,
    pub title: String,
    pub description: String,
    pub severity: String,
    pub status: String,
    pub disposition: String,
    pub quantity_affected: f64,
    pub assigned_to: Option<Uuid>,
    pub opened_by: Uuid,
    pub disposed_by: Option<Uuid>,
    pub closed_by: Option<Uuid>,
    pub disposition_notes: String,
    pub opened_at: DateTime<Utc>,
    pub disposed_at: Option<DateTime<Utc>>,
    pub closed_at: Option<DateTime<Utc>>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NcrDetail {
    #[serde(flatten)]
    pub ncr: Nonconformance,
    pub corrective_actions: Vec<CorrectiveAction>,
}

#[derive(Debug, Clone, Serialize)]
pub struct NewNonconformance {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub inspection_id: Option<Uuid>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub lot_id: Option<Uuid>,
    pub product_id: Uuid,
    pub title: String,
    pub description: String,
    pub severity: String,
    pub quantity_affected: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub assigned_to: Option<Uuid>,
}

#[derive(Debug, Clone, Serialize)]
pub struct PatchNonconformance {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub severity: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub quantity_affected: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CorrectiveAction {
    pub id: Uuid,
    pub ca_number: String,
    pub nonconformance_id: Option<Uuid>,
    pub title: String,
    pub root_cause: String,
    pub action_plan: String,
    pub status: String,
    pub owner_id: Option<Uuid>,
    pub due_date: String,
    pub verification_notes: String,
    pub effectiveness_notes: String,
    pub opened_by: Uuid,
    pub verified_by: Option<Uuid>,
    pub closed_by: Option<Uuid>,
    pub opened_at: DateTime<Utc>,
    pub verified_at: Option<DateTime<Utc>>,
    pub closed_at: Option<DateTime<Utc>>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize)]
pub struct NewCorrectiveAction {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub nonconformance_id: Option<Uuid>,
    pub title: String,
    pub root_cause: String,
    pub action_plan: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub owner_id: Option<Uuid>,
    pub due_date: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct PatchCorrectiveAction {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub root_cause: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub action_plan: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub due_date: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Product {
    pub id: Uuid,
    pub part_number: String,
    pub name: String,
    pub description: String,
    pub unit: String,
    pub active: bool,
    pub created_by: Uuid,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Lot {
    pub id: Uuid,
    pub lot_number: String,
    pub product_id: Uuid,
    pub site_id: Uuid,
    pub supplier_id: Option<Uuid>,
    pub quantity: f64,
    pub unit: String,
    pub source: String,
    pub supplier_lot: String,
    pub status: String,
    pub received_at: DateTime<Utc>,
    pub notes: String,
    pub created_by: Uuid,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Site {
    pub id: Uuid,
    pub code: String,
    pub name: String,
    pub address: String,
    pub active: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize)]
pub struct NewSite {
    pub code: String,
    pub name: String,
    pub address: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct NewProduct {
    pub part_number: String,
    pub name: String,
    pub description: String,
    pub unit: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct NewLot {
    pub lot_number: String,
    pub product_id: Uuid,
    pub site_id: Uuid,
    pub quantity: f64,
    pub unit: String,
    pub source: String,
    pub notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VersionInfo {
    pub name: String,
    pub version: String,
}
