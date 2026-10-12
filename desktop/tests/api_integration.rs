//! Integration tests against a running quality-control API.
//!
//! Requires:
//!   QC_API_BASE_URL   (default http://127.0.0.1:8080)
//!   QC_API_USERNAME   (default admin)
//!   QC_API_PASSWORD   (default change-me-now)
//!
//! Skips cleanly when the server is unreachable.

use qc_desktop::api::{
    ApiClient, NewCorrectiveAction, NewLot, NewNonconformance, NewProduct, NewSite,
};
use std::time::{SystemTime, UNIX_EPOCH};

fn env_or(key: &str, default: &str) -> String {
    std::env::var(key).unwrap_or_else(|_| default.to_string())
}

fn client_or_skip() -> Option<ApiClient> {
    let base = env_or("QC_API_BASE_URL", "http://127.0.0.1:8080");
    let client = ApiClient::new(base);
    match client.healthz() {
        Ok(()) => Some(client),
        Err(e) => {
            eprintln!("skipping integration tests: API unreachable ({e})");
            None
        }
    }
}

fn unique(prefix: &str) -> String {
    let n = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_millis();
    format!("{prefix}-{n}")
}

#[test]
fn health_and_version() {
    let Some(client) = client_or_skip() else {
        return;
    };
    client.healthz().expect("healthz");
    let v = client.version().expect("version");
    assert_eq!(v.name, "quality-control");
    assert!(!v.version.is_empty());
}

#[test]
fn login_me_dashboard() {
    let Some(mut client) = client_or_skip() else {
        return;
    };
    let user = env_or("QC_API_USERNAME", "admin");
    let pass = env_or("QC_API_PASSWORD", "change-me-now");
    let session = client.login(&user, &pass).expect("login");
    assert!(!session.access_token.is_empty());
    assert_eq!(session.user.username, user);
    assert_eq!(session.token_type, "Bearer");

    let me = client.me().expect("me");
    assert_eq!(me.username, user);

    let dash = client.dashboard().expect("dashboard");
    assert!(dash.open_inspections >= 0);
    assert!(dash.inspections_last_30_days >= 0);
}

#[test]
fn list_inspections_ncrs_capas() {
    let Some(mut client) = client_or_skip() else {
        return;
    };
    let user = env_or("QC_API_USERNAME", "admin");
    let pass = env_or("QC_API_PASSWORD", "change-me-now");
    client.login(&user, &pass).expect("login");

    let (inspections, page) = client.list_inspections(None, 50, 0).expect("inspections");
    assert!(page.limit >= inspections.len() as i32 || page.total >= 0);

    let (ncrs, _) = client.list_ncrs(None, 50, 0).expect("ncrs");
    let (capas, _) = client.list_capas(None, 50, 0).expect("capas");
    // Lists may be empty on a fresh DB; just ensure decode succeeded.
    let _ = (ncrs, capas);
}

#[test]
fn create_ncr_and_capa_roundtrip() {
    let Some(mut client) = client_or_skip() else {
        return;
    };
    let user = env_or("QC_API_USERNAME", "admin");
    let pass = env_or("QC_API_PASSWORD", "change-me-now");
    client.login(&user, &pass).expect("login");

    // Ensure a product exists for NCR creation.
    let part = unique("PN");
    let product = client
        .create_product(&NewProduct {
            part_number: part.clone(),
            name: format!("Part {part}"),
            description: "integration".into(),
            unit: "ea".into(),
        })
        .expect("create product");

    let ncr = client
        .create_ncr(&NewNonconformance {
            inspection_id: None,
            lot_id: None,
            product_id: product.id,
            title: format!("NCR for {part}"),
            description: "desktop integration test".into(),
            severity: "minor".into(),
            quantity_affected: 1.0,
            assigned_to: None,
        })
        .expect("create ncr");
    assert!(ncr.ncr_number.starts_with("NCR-") || !ncr.ncr_number.is_empty());
    assert_eq!(ncr.title, format!("NCR for {part}"));

    let detail = client.get_ncr(ncr.id).expect("get ncr");
    assert_eq!(detail.ncr.id, ncr.id);

    let due = chrono::Utc::now().date_naive().to_string();
    let capa = client
        .create_capa(&NewCorrectiveAction {
            nonconformance_id: Some(ncr.id),
            title: format!("CAPA for {part}"),
            root_cause: "process drift".into(),
            action_plan: "retrain operators".into(),
            owner_id: None,
            due_date: due,
        })
        .expect("create capa");
    assert!(!capa.ca_number.is_empty());

    let fetched = client.get_capa(capa.id).expect("get capa");
    assert_eq!(fetched.id, capa.id);
    assert_eq!(fetched.title, capa.title);
}

#[test]
fn create_site_and_lot_for_inspection_path() {
    let Some(mut client) = client_or_skip() else {
        return;
    };
    let user = env_or("QC_API_USERNAME", "admin");
    let pass = env_or("QC_API_PASSWORD", "change-me-now");
    client.login(&user, &pass).expect("login");

    let code = unique("SITE");
    let site = client
        .create_site(&NewSite {
            code: code.clone(),
            name: format!("Plant {code}"),
            address: "1 Test Way".into(),
        })
        .expect("create site");

    let part = unique("LOT-PN");
    let product = client
        .create_product(&NewProduct {
            part_number: part.clone(),
            name: format!("Lot part {part}"),
            description: String::new(),
            unit: "ea".into(),
        })
        .expect("product");

    let lot = client
        .create_lot(&NewLot {
            lot_number: unique("LOT"),
            product_id: product.id,
            site_id: site.id,
            quantity: 5.0,
            unit: "ea".into(),
            source: "incoming".into(),
            notes: "integration".into(),
        })
        .expect("lot");
    assert_eq!(lot.product_id, product.id);
    assert_eq!(lot.site_id, site.id);

    let (lots, _) = client.list_lots(Some(&lot.lot_number)).expect("list lots");
    assert!(lots.iter().any(|l| l.id == lot.id));
}

#[test]
fn unauthorized_without_token() {
    let Some(client) = client_or_skip() else {
        return;
    };
    // Fresh client with no token
    let bare = ApiClient::new(client.base_url());
    let err = bare.dashboard().expect_err("should be unauthorized");
    assert!(err.is_unauthorized() || err.message().contains("Sign in") || err.message().contains("unauthorized") || err.message().contains("401"));
}
