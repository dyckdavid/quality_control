//! Integration tests against a running quality-control API.
//!
//! Requires:
//!   QC_API_BASE_URL   (default http://127.0.0.1:8080)
//!   QC_API_USERNAME   (default admin)
//!   QC_API_PASSWORD   (default change-me-now)
//!
//! Skips cleanly when the server is unreachable.
//!
//! Tests that create records only run with QC_API_ALLOW_WRITES=1. Point them at a
//! scratch server, never the shop's real database.

use qc_desktop::api::{
    ApiClient, CompleteInspection, DispositionRequest, NewCorrectiveAction, NewInspection,
    NewInspectionDefect, NewLot, NewNonconformance, NewProduct, NewSite, ResultBatch, ResultInput,
};
use std::time::{SystemTime, UNIX_EPOCH};

fn env_or(key: &str, default: &str) -> String {
    std::env::var(key).unwrap_or_else(|_| default.to_string())
}

fn writes_allowed() -> bool {
    if std::env::var("QC_API_ALLOW_WRITES").as_deref() == Ok("1") {
        return true;
    }
    eprintln!("skipping write test: set QC_API_ALLOW_WRITES=1 against a scratch server");
    false
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
    if !writes_allowed() {
        return;
    }
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
    if !writes_allowed() {
        return;
    }
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

/// The whole shop loop the desktop drives: build, check, fail, decide, fix, close.
#[test]
fn shop_floor_workflow() {
    if !writes_allowed() {
        return;
    }
    let Some(mut client) = client_or_skip() else {
        return;
    };
    let user = env_or("QC_API_USERNAME", "admin");
    let pass = env_or("QC_API_PASSWORD", "change-me-now");
    let session = client.login(&user, &pass).expect("login");
    client
        .post_json("/api/v1/admin/seed-demo", &serde_json::json!({}))
        .expect("seed shop");

    let (products, _) = client.list_products(Some("GATE-COR")).expect("products");
    let gate = products.iter().find(|p| p.part_number == "GATE-COR").expect("gate product");
    let (sites, _) = client.list_sites().expect("sites");
    let shop = sites.iter().find(|s| s.code == "SHOP").expect("shop site");
    let lot = client
        .create_lot(&NewLot {
            lot_number: unique("GATE"),
            product_id: gate.id,
            site_id: shop.id,
            quantity: 2.0,
            unit: "ea".into(),
            source: "in_process".into(),
            notes: "workflow test".into(),
        })
        .expect("lot");

    let started = client
        .create_inspection(&NewInspection {
            lot_id: lot.id,
            plan_id: None,
            station_id: None,
            inspector_id: None,
            sample_size: None,
            notes: String::new(),
        })
        .expect("inspection");
    let id = started.inspection.id;
    let plan = client.get_plan(started.inspection.plan_id).expect("plan");
    assert!(!plan.characteristics.is_empty());

    let samples = started.inspection.sample_size.max(1);
    let mut results = Vec::new();
    for ch in &plan.characteristics {
        if ch.is_variable() {
            for s in 1..=samples {
                results.push(ResultInput {
                    characteristic_id: ch.id,
                    sample_index: s,
                    measured_value: Some(48.5),
                    attribute_result: None,
                    gauge_id: None,
                    notes: String::new(),
                });
            }
        } else {
            let verdict = if ch.code == "WELD" { "fail" } else { "pass" };
            results.push(ResultInput {
                characteristic_id: ch.id,
                sample_index: 1,
                measured_value: None,
                attribute_result: Some(verdict.into()),
                gauge_id: None,
                notes: String::new(),
            });
        }
    }
    client.record_results(id, &ResultBatch { results }).expect("record results");
    let after = client.get_inspection(id).expect("reload");
    assert!(after.missing_characteristics.is_empty(), "missing {:?}", after.missing_characteristics);

    let codes = client.list_defect_codes().expect("defect codes");
    let burr = codes.iter().find(|d| d.code == "EDGE-BURR").expect("EDGE-BURR");
    client
        .add_defect(id, &NewInspectionDefect { defect_code_id: burr.id, quantity: 1, notes: "top rail".into() })
        .expect("defect");

    let done = client
        .complete_inspection(
            id,
            &CompleteInspection {
                quantity_inspected: Some(2.0),
                quantity_accepted: Some(1.0),
                quantity_rejected: Some(1.0),
                notes: "weld miss on hinge side".into(),
                open_ncr: true,
            },
        )
        .expect("complete");
    assert_eq!(done.inspection.result, "fail");
    let ncr = done.nonconformance.expect("failed inspection opens an alert");

    let closing_early = client.close_ncr(ncr.id).expect_err("close needs a decision first");
    assert!(closing_early.message().to_lowercase().contains("disposition"));
    let decided = client
        .dispose_ncr(ncr.id, &DispositionRequest { disposition: "rework".into(), notes: "reweld hinge".into() })
        .expect("dispose");
    assert_eq!(decided.status, "dispositioned");
    assert_eq!(client.close_ncr(ncr.id).expect("close").status, "closed");

    let capa = client
        .create_capa(&NewCorrectiveAction {
            nonconformance_id: Some(ncr.id),
            title: "Weld fixture for hinge side".into(),
            root_cause: "Hinge side is hard to reach".into(),
            action_plan: "Add a turning fixture".into(),
            owner_id: None,
            due_date: chrono::Utc::now().date_naive().to_string(),
        })
        .expect("capa");
    assert_eq!(client.move_capa(capa.id, "start", "").expect("start").status, "in_progress");
    client.move_capa(capa.id, "verify", "").expect_err("verify needs notes");
    assert_eq!(
        client.move_capa(capa.id, "verify", "Next 5 gates checked").expect("verify").status,
        "verification"
    );
    assert_eq!(client.move_capa(capa.id, "close", "No misses since").expect("close").status, "closed");

    let lot_now = client.get_lot(lot.id).expect("lot");
    assert_eq!(lot_now.status, "rejected");
    assert_eq!(client.set_lot_status(lot.id, "on_hold").expect("hold").status, "on_hold");

    let renewed = client.refresh(&session.refresh_token).expect("refresh");
    assert_ne!(renewed.refresh_token, session.refresh_token);
    client.set_access_token(Some(renewed.access_token.clone()));
    client.dashboard().expect("works with renewed token");
    client.refresh(&session.refresh_token).expect_err("old refresh token is spent");
    client.logout(&renewed.refresh_token).expect("logout");
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
