use crate::api::{CorrectiveAction, Inspection, Nonconformance};
use std::path::Path;

/// Stops spreadsheet apps from running typed text as a formula. Plain numbers pass through.
fn safe(s: &str) -> String {
    match s.as_bytes().first() {
        Some(b'=' | b'+' | b'-' | b'@' | b'\t' | b'\r') if s.parse::<f64>().is_err() => {
            format!("'{s}")
        }
        _ => s.to_string(),
    }
}

pub fn export_inspections(path: &Path, rows: &[Inspection]) -> Result<(), String> {
    let mut wtr = csv::Writer::from_path(path).map_err(|e| e.to_string())?;
    wtr.write_record([
        "inspection_number",
        "status",
        "result",
        "lot_id",
        "sample_size",
        "quantity_inspected",
        "quantity_accepted",
        "quantity_rejected",
        "started_at",
        "completed_at",
        "notes",
    ])
    .map_err(|e| e.to_string())?;
    for r in rows {
        wtr.write_record([
            r.inspection_number.as_str(),
            r.status.as_str(),
            r.result.as_str(),
            &r.lot_id.to_string(),
            &r.sample_size.to_string(),
            &r.quantity_inspected.to_string(),
            &r.quantity_accepted.to_string(),
            &r.quantity_rejected.to_string(),
            &r.started_at.to_rfc3339(),
            &r.completed_at
                .map(|t| t.to_rfc3339())
                .unwrap_or_default(),
            r.notes.as_str(),
        ].map(safe))
        .map_err(|e| e.to_string())?;
    }
    wtr.flush().map_err(|e| e.to_string())
}

pub fn export_ncrs(path: &Path, rows: &[Nonconformance]) -> Result<(), String> {
    let mut wtr = csv::Writer::from_path(path).map_err(|e| e.to_string())?;
    wtr.write_record([
        "ncr_number",
        "title",
        "severity",
        "status",
        "disposition",
        "quantity_affected",
        "product_id",
        "opened_at",
        "description",
    ])
    .map_err(|e| e.to_string())?;
    for r in rows {
        wtr.write_record([
            r.ncr_number.as_str(),
            r.title.as_str(),
            r.severity.as_str(),
            r.status.as_str(),
            r.disposition.as_str(),
            &r.quantity_affected.to_string(),
            &r.product_id.to_string(),
            &r.opened_at.to_rfc3339(),
            r.description.as_str(),
        ].map(safe))
        .map_err(|e| e.to_string())?;
    }
    wtr.flush().map_err(|e| e.to_string())
}

pub fn export_capas(path: &Path, rows: &[CorrectiveAction]) -> Result<(), String> {
    let mut wtr = csv::Writer::from_path(path).map_err(|e| e.to_string())?;
    wtr.write_record([
        "ca_number",
        "title",
        "status",
        "due_date",
        "root_cause",
        "action_plan",
        "opened_at",
    ])
    .map_err(|e| e.to_string())?;
    for r in rows {
        wtr.write_record([
            r.ca_number.as_str(),
            r.title.as_str(),
            r.status.as_str(),
            r.due_date.as_str(),
            r.root_cause.as_str(),
            r.action_plan.as_str(),
            &r.opened_at.to_rfc3339(),
        ].map(safe))
        .map_err(|e| e.to_string())?;
    }
    wtr.flush().map_err(|e| e.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::Utc;
    use uuid::Uuid;

    #[test]
    fn safe_escapes_formulas_not_numbers() {
        assert_eq!(safe("=1+1"), "'=1+1");
        assert_eq!(safe("@SUM(A1)"), "'@SUM(A1)");
        assert_eq!(safe("-2.5"), "-2.5");
        assert_eq!(safe("Weld miss"), "Weld miss");
    }

    #[test]
    fn export_inspections_writes_header() {
        let dir = std::env::temp_dir().join(format!("qc-csv-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let path = dir.join("inspections.csv");
        let row = Inspection {
            id: Uuid::new_v4(),
            inspection_number: "INS-1".into(),
            lot_id: Uuid::new_v4(),
            plan_id: Uuid::new_v4(),
            station_id: None,
            inspector_id: Uuid::new_v4(),
            status: "completed".into(),
            result: "pass".into(),
            sample_size: 1,
            quantity_inspected: 10.0,
            quantity_accepted: 10.0,
            quantity_rejected: 0.0,
            notes: String::new(),
            started_at: Utc::now(),
            completed_at: Some(Utc::now()),
            created_by: Uuid::new_v4(),
            created_at: Utc::now(),
            updated_at: Utc::now(),
        };
        export_inspections(&path, &[row]).unwrap();
        let text = std::fs::read_to_string(&path).unwrap();
        assert!(text.starts_with("inspection_number,"));
        assert!(text.contains("INS-1"));
        let _ = std::fs::remove_dir_all(&dir);
    }
}
