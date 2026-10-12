//! Tables and create forms for every shop record the API stores.

use super::chrome::{self, Btn};
use super::records::parse_measure;
use super::{FormMode, NavPage, QcApp};
use crate::api::ApiError;
use eframe::egui;
use poll_promise::Promise;
use serde_json::{json, Value};
use uuid::Uuid;

pub struct CatalogSpec {
    pub path: &'static str,
    pub columns: &'static [&'static str],
    pub create_path: Option<&'static str>,
    /// Field key and the example shown as a hint.
    pub fields: &'static [(&'static str, &'static str)],
    pub hint: &'static str,
}

/// A draft check for the selected plan.
pub(super) struct CharForm {
    measured: bool,
    code: String,
    name: String,
    unit: String,
    lower: String,
    upper: String,
    critical: bool,
    /// The check being edited; `None` adds a new one.
    editing: Option<Uuid>,
}

/// A draft step for the selected checklist.
pub(super) struct ItemForm {
    prompt: String,
    item_type: String,
    required: bool,
    help: String,
    editing: Option<Uuid>,
}

impl Default for ItemForm {
    fn default() -> Self {
        Self {
            prompt: String::new(),
            item_type: "pass_fail".into(),
            required: true,
            help: String::new(),
            editing: None,
        }
    }
}

const ITEM_TYPES: &[(&str, &str)] = &[
    ("pass_fail", "Pass or fail"),
    ("yes_no", "Yes or no"),
    ("number", "Number"),
    ("text", "Note"),
];

impl Default for CharForm {
    fn default() -> Self {
        Self {
            editing: None,
            measured: true,
            code: String::new(),
            name: String::new(),
            unit: "in".into(),
            lower: String::new(),
            upper: String::new(),
            critical: false,
        }
    }
}

const SOURCES: &[(&str, &str)] = &[
    ("in_process", "Built here"),
    ("final", "Final"),
    ("incoming", "Bought in"),
    ("customer_return", "Customer return"),
];
const SEVERITIES: &[(&str, &str)] = &[("minor", "Minor"), ("major", "Major"), ("critical", "Critical")];
const ROLES: &[(&str, &str)] = &[
    ("viewer", "Viewer"),
    ("operator", "Operator"),
    ("inspector", "Inspector"),
    ("supervisor", "Supervisor"),
    ("quality_manager", "Quality manager"),
    ("admin", "Admin"),
];
const SAMPLING: &[(&str, &str)] = &[
    ("hundred_percent", "Every piece"),
    ("fixed", "Fixed count"),
    ("aql", "AQL"),
];

fn choices(key: &str) -> Option<&'static [(&'static str, &'static str)]> {
    match key {
        "source" => Some(SOURCES),
        "severity" => Some(SEVERITIES),
        "role" => Some(ROLES),
        "sampling_type" => Some(SAMPLING),
        _ => None,
    }
}

fn field_default(key: &str) -> &'static str {
    match key {
        "source" => "in_process",
        "severity" => "major",
        "role" => "inspector",
        "sampling_type" => "hundred_percent",
        "unit" => "ea",
        "revision" => "A",
        _ => "",
    }
}

/// Manual lot moves the server allows, mirrored so only valid buttons show.
fn lot_moves(from: &str) -> &'static [(&'static str, &'static str)] {
    match from {
        "open" => &[("on_hold", "Put on hold"), ("closed", "Close")],
        "on_hold" => &[
            ("open", "Release hold"),
            ("accepted", "Accept"),
            ("rejected", "Reject"),
            ("closed", "Close"),
        ],
        "accepted" | "rejected" => &[("on_hold", "Put on hold"), ("closed", "Close")],
        "closed" => &[("open", "Reopen")],
        _ => &[],
    }
}

impl NavPage {
    pub fn catalog(self) -> Option<CatalogSpec> {
        Some(match self {
            NavPage::Lots => CatalogSpec {
                path: "/api/v1/lots",
                columns: &["lot_number", "product_id", "status", "source", "quantity", "unit"],
                create_path: Some("/api/v1/lots"),
                fields: &[
                    ("lot_number", "CHUTE-101"),
                    ("product_id", ""),
                    ("site_id", ""),
                    ("quantity", "1"),
                    ("unit", "ea"),
                    ("source", ""),
                    ("notes", "Customer, color, or options"),
                ],
                hint: "A lot is one build or one batch. Click a row to inspect it or change its status.",
            },
            NavPage::Products => CatalogSpec {
                path: "/api/v1/products",
                columns: &["part_number", "name", "unit", "active"],
                create_path: Some("/api/v1/products"),
                fields: &[
                    ("part_number", "CHUTE-SQ"),
                    ("name", "Squeeze chute"),
                    ("description", ""),
                    ("unit", "ea"),
                ],
                hint: "What the shop builds. Each product needs an active plan before it can be inspected.",
            },
            NavPage::Plans => CatalogSpec {
                path: "/api/v1/plans",
                columns: &["name", "product_id", "revision", "status", "sampling_type"],
                create_path: Some("/api/v1/plans"),
                fields: &[
                    ("product_id", ""),
                    ("name", "Final"),
                    ("revision", "A"),
                    ("sampling_type", ""),
                    ("sample_size", "Only for fixed count"),
                ],
                hint: "Click a plan to see its checks, add more, and make it active.",
            },
            NavPage::Sites => CatalogSpec {
                path: "/api/v1/sites",
                columns: &["code", "name", "address", "active"],
                create_path: Some("/api/v1/sites"),
                fields: &[("code", "SHOP"), ("name", "Shop"), ("address", "")],
                hint: "The shop, and any yard or second building.",
            },
            NavPage::Stations => CatalogSpec {
                path: "/api/v1/stations",
                columns: &["code", "name", "site_id", "active"],
                create_path: Some("/api/v1/stations"),
                fields: &[
                    ("site_id", ""),
                    ("code", "FINAL"),
                    ("name", "Final inspection"),
                    ("description", ""),
                ],
                hint: "Where in the shop a check happens.",
            },
            NavPage::Suppliers => CatalogSpec {
                path: "/api/v1/suppliers",
                columns: &["code", "name", "email", "phone", "active"],
                create_path: Some("/api/v1/suppliers"),
                fields: &[
                    ("code", "STEEL1"),
                    ("name", "Steel supplier"),
                    ("contact_name", ""),
                    ("email", ""),
                    ("phone", ""),
                ],
                hint: "Steel, glass, gaskets, and hardware vendors.",
            },
            NavPage::Gauges => CatalogSpec {
                path: "/api/v1/gauges",
                columns: &["code", "name", "serial_number", "calibration_due", "active"],
                create_path: Some("/api/v1/gauges"),
                fields: &[
                    ("code", "TAPE-25"),
                    ("name", "25 ft tape"),
                    ("serial_number", ""),
                    ("calibration_due", "2027-01-31"),
                ],
                hint: "Calibration due is YYYY-MM-DD. An overdue gauge still works, but its readings are flagged.",
            },
            NavPage::Defects => CatalogSpec {
                path: "/api/v1/defect-codes",
                columns: &["code", "name", "severity", "category", "active"],
                create_path: Some("/api/v1/defect-codes"),
                fields: &[
                    ("code", "WELD-MISS"),
                    ("name", "Missed or short weld"),
                    ("severity", ""),
                    ("category", "weld"),
                ],
                hint: "Major and critical defects fail a build. Minor ones pass it with a note.",
            },
            NavPage::Checklists => CatalogSpec {
                path: "/api/v1/checklists",
                columns: &["code", "name", "active"],
                create_path: Some("/api/v1/checklists"),
                fields: &[("code", "CATTLE-FINAL"), ("name", "Cattle equipment final"), ("description", "")],
                hint: "Templates a quality manager builds.",
            },
            NavPage::Measurements => CatalogSpec {
                path: "/api/v1/measurements",
                columns: &[
                    "characteristic_code",
                    "measured_value",
                    "unit",
                    "in_spec",
                    "lot_number",
                    "inspection_number",
                ],
                create_path: None,
                fields: &[("characteristic_code", "WIDTH, HEIGHT, LENGTH, COLLAR")],
                hint: "Type a check code and press Show.",
            },
            NavPage::Users => CatalogSpec {
                path: "/api/v1/users",
                columns: &["username", "full_name", "email", "role", "active"],
                create_path: Some("/api/v1/users"),
                fields: &[
                    ("username", "jsmith"),
                    ("full_name", "John Smith"),
                    ("email", "john@shop.local"),
                    ("password", "At least 10 characters"),
                    ("role", ""),
                ],
                hint: "Admin only.",
            },
            NavPage::Audit => CatalogSpec {
                path: "/api/v1/audit",
                columns: &["action", "summary", "entity_type", "created_at"],
                create_path: None,
                fields: &[],
                hint: "Supervisor and above.",
            },
            NavPage::Webhooks => CatalogSpec {
                path: "/api/v1/webhooks",
                columns: &["name", "url", "active"],
                create_path: Some("/api/v1/webhooks"),
                fields: &[
                    ("name", "Shop events"),
                    ("url", "https://example.com/qc"),
                    ("secret", "Shared secret"),
                    ("description", ""),
                    ("events", "inspection.completed, nonconformance.opened"),
                ],
                hint: "Comma-separated event names. Supervisor and above.",
            },
            NavPage::Search => CatalogSpec {
                path: "/api/v1/search",
                columns: &["entity_type", "number", "title", "status"],
                create_path: None,
                fields: &[("q", "CHUTE, GATE-100, NCR-")],
                hint: "Lots, inspections, alerts, and corrective actions.",
            },
            _ => return None,
        })
    }
}

impl QcApp {
    fn reset_catalog_form(&mut self) {
        let Some(spec) = self.page.catalog() else {
            return;
        };
        self.catalog_form = spec
            .fields
            .iter()
            .map(|(k, _)| ((*k).to_string(), field_default(k).to_string()))
            .collect();
        self.catalog_form_page = Some(self.page);
    }

    pub(super) fn spawn_catalog(&mut self) {
        let Some(spec) = self.page.catalog() else {
            return;
        };
        if self.catalog_form_page != Some(self.page) {
            self.reset_catalog_form();
        }
        let query_key = match self.page {
            NavPage::Measurements => Some("characteristic_code"),
            NavPage::Search => Some("q"),
            _ => None,
        };
        let query_val = self
            .catalog_form
            .first()
            .map(|(_, v)| v.trim().to_string())
            .unwrap_or_default();
        if query_key.is_some() && query_val.is_empty() {
            self.catalog_rows.clear();
            return;
        }
        let client = self.authed_client();
        let path = spec.path.to_string();
        let search = self.page == NavPage::Search;
        self.loading = true;
        self.list_error = None;
        self.catalog_promise = Some(Promise::spawn_thread("catalog", move || {
            match query_key {
                Some(key) if search => client.get_list_envelope(&path, &[(key, query_val)]),
                Some(key) => client.list_json_query(&path, &[(key, query_val)]),
                None => client.list_json(&path),
            }
        }));
    }

    fn spawn_plan(&mut self, id: Uuid) {
        let client = self.authed_client();
        self.plan_promise = Some(Promise::spawn_thread("plan", move || client.get_plan(id)));
    }

    pub(super) fn poll_plan(&mut self) {
        if self.plan_promise.as_ref().is_some_and(|p| p.ready().is_some()) {
            if let Some(p) = self.plan_promise.take() {
                match p.block_and_take() {
                    Ok(plan) if self.catalog_selected == Some(plan.id) => self.catalog_plan = Some(plan),
                    Ok(_) => {}
                    Err(e) => self.list_error = Some(e.message()),
                }
            }
        }
    }

    pub(super) fn ui_catalog(&mut self, ui: &mut egui::Ui) {
        if self.page == NavPage::Reports {
            self.ui_reports(ui);
            return;
        }
        if self.page == NavPage::Stats {
            self.ui_stats_page(ui);
            return;
        }
        let Some(spec) = self.page.catalog() else {
            return;
        };
        if self.catalog_form_page != Some(self.page) {
            self.reset_catalog_form();
        }
        chrome::muted(ui, spec.hint);
        ui.add_space(10.0);
        if spec.create_path.is_some() {
            let open = self.catalog_rows.is_empty() && !self.loading;
            egui::CollapsingHeader::new(chrome::semi("New record", 13.5, chrome::palette(ui).ink))
                .id_salt(("new-record", spec.path))
                .default_open(open)
                .show(ui, |ui| {
                    chrome::form_card(ui, "Create", |ui| {
                        self.draw_fields(ui, spec.fields);
                        ui.add_space(4.0);
                        let busy = self.mutate_promise.is_some();
                        if ui.add_enabled_ui(!busy, |ui| chrome::button(ui, "Save", Btn::Primary).clicked()).inner {
                            self.submit_catalog_create();
                        }
                    });
                });
            ui.add_space(8.0);
        } else if !spec.fields.is_empty() {
            let mut apply = false;
            ui.horizontal(|ui| {
                for ((_, value), (_, hint)) in self.catalog_form.iter_mut().zip(spec.fields) {
                    let r = ui.add(
                        egui::TextEdit::singleline(value)
                            .hint_text(*hint)
                            .desired_width(320.0)
                            .margin(egui::Margin::symmetric(10.0, 8.0)),
                    );
                    if r.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter)) {
                        apply = true;
                    }
                }
                if chrome::button(ui, "Show", Btn::Primary).clicked() {
                    apply = true;
                }
            });
            if apply {
                self.spawn_catalog();
            }
            ui.add_space(10.0);
        }
        self.sync_record_edit();
        match self.page {
            NavPage::Plans => self.ui_plan_panel(ui),
            NavPage::Lots => self.ui_lot_panel(ui),
            _ => {}
        }
        self.ui_record_editor(ui);
        if self.page == NavPage::Checklists {
            self.ui_checklist_panel(ui);
        }
        if self.catalog_selected.is_some() && self.selected_row().is_some() {
            chrome::muted(ui, "Click the row again to close it.");
            ui.add_space(6.0);
        }
        if let Some(id) = self.catalog_table(ui, spec.columns) {
            self.catalog_plan = None;
            self.catalog_extra = None;
            self.record_edit = None;
            self.char_form = CharForm::default();
            self.item_form = ItemForm::default();
            if self.catalog_selected == Some(id) {
                self.catalog_selected = None;
            } else {
                self.catalog_selected = Some(id);
                match self.page {
                    NavPage::Plans => self.spawn_plan(id),
                    NavPage::Checklists => self.spawn_extra(id),
                    _ => {}
                }
            }
        }
    }

    pub(super) fn spawn_extra(&mut self, id: Uuid) {
        let client = self.authed_client();
        self.extra_promise = Some(Promise::spawn_thread("checklist", move || {
            client.get_json(&format!("/api/v1/checklists/{id}"))
        }));
    }

    pub(super) fn poll_extra(&mut self) {
        if self.extra_promise.as_ref().is_some_and(|p| p.ready().is_some()) {
            if let Some(p) = self.extra_promise.take() {
                match p.block_and_take() {
                    Ok(v) if v.get("id").and_then(Value::as_str) == self.catalog_selected.map(|i| i.to_string()).as_deref() => {
                        self.catalog_extra = Some(v)
                    }
                    Ok(_) => {}
                    Err(e) => self.list_error = Some(e.message()),
                }
            }
        }
    }

    fn ui_checklist_panel(&mut self, ui: &mut egui::Ui) {
        let Some(id) = self.catalog_selected else {
            return;
        };
        if self.selected_row().is_none() {
            return;
        }
        let p = chrome::palette(ui);
        let manager = self.role_at_least("quality_manager");
        let busy = self.mutate_promise.is_some();
        let mut remove: Option<Uuid> = None;
        let mut edit: Option<Value> = None;
        let mut save = false;
        chrome::card(ui, "", |ui| {
            ui.label(chrome::semi("Steps", 15.0, p.ink));
            ui.add_space(6.0);
            let items: Vec<Value> = match &self.catalog_extra {
                None => {
                    ui.spinner();
                    return;
                }
                Some(c) => c.get("items").and_then(Value::as_array).cloned().unwrap_or_default(),
            };
            if items.is_empty() {
                chrome::muted(ui, "No steps yet. Add the first one below.");
            } else {
                egui::Grid::new("checklist-items").striped(true).spacing(egui::vec2(18.0, 6.0)).show(ui, |ui| {
                    for h in ["#", "Step", "Answer", "Required", ""] {
                        ui.label(chrome::semi(h, 12.0, p.muted));
                    }
                    ui.end_row();
                    for item in &items {
                        ui.label(cell(item, "sequence"));
                        ui.label(cell(item, "prompt"));
                        let kind = cell(item, "item_type");
                        ui.label(ITEM_TYPES.iter().find(|(k, _)| *k == kind).map(|(_, l)| *l).unwrap_or(&kind));
                        ui.label(if cell(item, "required") == "true" { "Yes" } else { "No" });
                        ui.horizontal(|ui| {
                            if manager && !busy {
                                if chrome::button(ui, "Edit", Btn::Quiet).clicked() {
                                    edit = Some(item.clone());
                                }
                                if chrome::button(ui, "Remove", Btn::Danger).clicked() {
                                    remove = item.get("id").and_then(Value::as_str).and_then(|s| Uuid::parse_str(s).ok());
                                }
                            }
                        });
                        ui.end_row();
                    }
                });
            }
            if !manager {
                return;
            }
            ui.add_space(12.0);
            ui.separator();
            ui.add_space(8.0);
            let f = &mut self.item_form;
            ui.label(chrome::semi(if f.editing.is_some() { "Edit step" } else { "Add a step" }, 13.5, p.ink));
            ui.add_space(6.0);
            ui.horizontal_wrapped(|ui| {
                for (k, label) in ITEM_TYPES {
                    if chrome::choice(ui, label, f.item_type == *k, true).clicked() {
                        f.item_type = (*k).to_string();
                    }
                }
                ui.add_space(8.0);
                ui.checkbox(&mut f.required, "Required");
            });
            ui.add_space(6.0);
            ui.add(
                egui::TextEdit::singleline(&mut f.prompt)
                    .hint_text("What to check, like: Head gate latches both sides")
                    .desired_width(f32::INFINITY)
                    .margin(egui::Margin::symmetric(10.0, 8.0)),
            );
            ui.add_space(4.0);
            ui.add(
                egui::TextEdit::singleline(&mut f.help)
                    .hint_text("Help for the person filling it in (optional)")
                    .desired_width(f32::INFINITY)
                    .margin(egui::Margin::symmetric(10.0, 8.0)),
            );
            ui.add_space(8.0);
            ui.horizontal(|ui| {
                let ready = !f.prompt.trim().is_empty() && !busy;
                let label = if f.editing.is_some() { "Save step" } else { "Add step" };
                if ui.add_enabled_ui(ready, |ui| chrome::button(ui, label, Btn::Primary).clicked()).inner {
                    save = true;
                }
                if f.editing.is_some() && chrome::button(ui, "Cancel", Btn::Quiet).clicked() {
                    *f = ItemForm::default();
                }
            });
        });
        ui.add_space(12.0);
        if let Some(item) = edit {
            self.item_form = ItemForm {
                prompt: cell(&item, "prompt"),
                item_type: cell(&item, "item_type"),
                required: cell(&item, "required") == "true",
                help: cell(&item, "help_text"),
                editing: item.get("id").and_then(Value::as_str).and_then(|s| Uuid::parse_str(s).ok()),
            };
        }
        if let Some(item) = remove {
            let client = self.authed_client();
            self.mutate_promise = Some(Promise::spawn_thread("remove-step", move || {
                client.delete_path(&format!("/api/v1/checklist-items/{item}"))?;
                Ok(super::MutatePayload::Ok("Step removed.".into()))
            }));
        }
        if save {
            let next = self
                .catalog_extra
                .as_ref()
                .and_then(|c| c.get("items"))
                .and_then(Value::as_array)
                .map(|items| items.iter().filter_map(|i| i.get("sequence").and_then(Value::as_i64)).max().unwrap_or(0) + 1)
                .unwrap_or(1);
            let f = std::mem::take(&mut self.item_form);
            let body = json!({
                "prompt": f.prompt.trim(), "item_type": f.item_type, "required": f.required, "help_text": f.help.trim(),
            });
            let client = self.authed_client();
            self.mutate_promise = Some(Promise::spawn_thread("save-step", move || {
                match f.editing {
                    Some(item) => {
                        client.patch_json(&format!("/api/v1/checklist-items/{item}"), &body)?;
                        Ok(super::MutatePayload::Ok("Step saved.".into()))
                    }
                    None => {
                        let mut body = body;
                        body["sequence"] = json!(next);
                        client.post_json(&format!("/api/v1/checklists/{id}/items"), &body)?;
                        Ok(super::MutatePayload::Ok("Step added.".into()))
                    }
                }
            }));
        }
    }

    fn draw_fields(&mut self, ui: &mut egui::Ui, fields: &[(&str, &str)]) {
        let products = self.products.clone();
        let sites = self.sites.clone();
        for ((key, value), (_, hint)) in self.catalog_form.iter_mut().zip(fields) {
            let label = match key.as_str() {
                "product_id" => "Product".to_string(),
                "site_id" => "Site".to_string(),
                other => chrome::pretty_key(other),
            };
            chrome::field_label(ui, &label);
            match key.as_str() {
                "product_id" => {
                    let shown = products
                        .iter()
                        .find(|p| p.id.to_string() == *value)
                        .map(|p| format!("{}  {}", p.part_number, p.name))
                        .unwrap_or_else(|| "Pick a product".into());
                    egui::ComboBox::from_id_salt(("pick", key.as_str()))
                        .width(ui.available_width())
                        .selected_text(shown)
                        .show_ui(ui, |ui| {
                            for p in products.iter().filter(|p| p.active) {
                                ui.selectable_value(value, p.id.to_string(), format!("{}  {}", p.part_number, p.name));
                            }
                        });
                }
                "site_id" => {
                    if value.is_empty() && sites.len() == 1 {
                        *value = sites[0].id.to_string();
                    }
                    let shown = sites
                        .iter()
                        .find(|s| s.id.to_string() == *value)
                        .map(|s| format!("{}  {}", s.code, s.name))
                        .unwrap_or_else(|| "Pick a site".into());
                    egui::ComboBox::from_id_salt(("pick", key.as_str()))
                        .width(ui.available_width())
                        .selected_text(shown)
                        .show_ui(ui, |ui| {
                            for s in sites.iter().filter(|s| s.active) {
                                ui.selectable_value(value, s.id.to_string(), format!("{}  {}", s.code, s.name));
                            }
                        });
                }
                k if choices(k).is_some() => {
                    ui.horizontal_wrapped(|ui| {
                        for (v, text) in choices(k).unwrap_or_default() {
                            if chrome::choice(ui, text, value == v, true).clicked() {
                                *value = (*v).to_string();
                            }
                        }
                    });
                }
                "password" | "secret" => {
                    ui.add(
                        egui::TextEdit::singleline(value)
                            .password(true)
                            .hint_text(*hint)
                            .desired_width(f32::INFINITY)
                            .margin(egui::Margin::symmetric(10.0, 8.0)),
                    );
                }
                _ => {
                    ui.add(
                        egui::TextEdit::singleline(value)
                            .hint_text(*hint)
                            .desired_width(f32::INFINITY)
                            .margin(egui::Margin::symmetric(10.0, 8.0)),
                    );
                }
            }
            ui.add_space(6.0);
        }
    }

    pub(super) fn selected_row(&self) -> Option<&Value> {
        let id = self.catalog_selected?.to_string();
        self.catalog_rows
            .iter()
            .find(|r| r.get("id").and_then(Value::as_str) == Some(id.as_str()))
    }

    fn ui_plan_panel(&mut self, ui: &mut egui::Ui) {
        let Some(row) = self.selected_row().cloned() else {
            return;
        };
        let Some(id) = self.catalog_selected else {
            return;
        };
        let p = chrome::palette(ui);
        let status = cell(&row, "status");
        let manager = self.role_at_least("quality_manager");
        let admin = self.role_at_least("admin");
        let busy = self.mutate_promise.is_some();
        let draft = status == "draft";
        let can_edit_checks = manager && (draft || (admin && status == "active"));
        let mut edit_check: Option<crate::api::Characteristic> = None;
        let mut remove_check: Option<Uuid> = None;
        chrome::card(ui, "", |ui| {
            ui.horizontal(|ui| {
                ui.label(chrome::semi(
                    format!("{} rev {}", cell(&row, "name"), cell(&row, "revision")),
                    16.0,
                    p.ink,
                ));
                chrome::pill(ui, &status);
                ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                    if !manager {
                        chrome::muted(ui, "A quality manager changes plans.");
                        return;
                    }
                    ui.add_enabled_ui(!busy, |ui| {
                        if status == "draft" {
                            if chrome::button(ui, "Delete draft", Btn::Danger).clicked() {
                                self.plan_call(id, None, "Draft deleted.");
                            }
                            if chrome::button(ui, "Make active", Btn::Primary).clicked() {
                                self.plan_call(id, Some("activate"), "Plan is active. New inspections use it.");
                            }
                        } else if status == "active" {
                            if chrome::button(ui, "Retire", Btn::Quiet).clicked() {
                                self.plan_call(id, Some("obsolete"), "Plan retired.");
                            }
                            if chrome::button(ui, "New revision", Btn::Quiet).clicked() {
                                self.plan_call(id, Some("revise"), "New draft revision made. Edit it, then make it active.");
                            }
                        }
                    });
                });
            });
            ui.add_space(8.0);
            match &self.catalog_plan {
                None => {
                    ui.spinner();
                }
                Some(plan) if plan.characteristics.is_empty() => {
                    chrome::muted(ui, "No checks yet. Add the first one below.");
                }
                Some(plan) => {
                    egui::Grid::new("plan-checks").striped(true).spacing(egui::vec2(18.0, 6.0)).show(ui, |ui| {
                        for h in ["#", "Code", "Check", "Kind", "Limits", ""] {
                            ui.label(chrome::semi(h, 12.0, p.muted));
                        }
                        ui.end_row();
                        for c in &plan.characteristics {
                            ui.label(c.sequence.to_string());
                            ui.horizontal(|ui| {
                                ui.label(chrome::semi(&c.code, 13.0, p.ink));
                                if c.critical {
                                    chrome::pill(ui, "critical");
                                }
                            });
                            ui.label(&c.name);
                            ui.label(if c.is_variable() { "Measure" } else { "Pass or fail" });
                            let limits = match (c.lower_limit, c.upper_limit) {
                                (Some(a), Some(b)) => format!("{a} to {b} {}", c.unit),
                                (Some(a), None) => format!("≥ {a} {}", c.unit),
                                (None, Some(b)) => format!("≤ {b} {}", c.unit),
                                (None, None) if c.is_variable() => format!("to print ({})", c.unit),
                                (None, None) => "—".into(),
                            };
                            ui.label(limits);
                            ui.horizontal(|ui| {
                                if can_edit_checks && !busy {
                                    if chrome::button(ui, "Edit", Btn::Quiet).clicked() {
                                        edit_check = Some(c.clone());
                                    }
                                    if draft && chrome::button(ui, "Remove", Btn::Danger).clicked() {
                                        remove_check = Some(c.id);
                                    }
                                }
                            });
                            ui.end_row();
                        }
                    });
                }
            }
            let editing = self.char_form.editing.is_some();
            if !(manager && (draft || (editing && can_edit_checks))) {
                if !draft && manager {
                    ui.add_space(6.0);
                    if admin && status == "active" {
                        chrome::muted(
                            ui,
                            "Active plan. As admin you can fix a check's name or limits; adding or removing checks needs a new revision.",
                        );
                    } else {
                        chrome::muted(ui, "Active plans are locked. Make a new revision to change the checks.");
                    }
                }
                return;
            }
            ui.add_space(12.0);
            ui.separator();
            ui.add_space(8.0);
            ui.label(chrome::semi(if editing { "Edit check" } else { "Add a check" }, 13.5, p.ink));
            ui.add_space(6.0);
            let f = &mut self.char_form;
            ui.horizontal(|ui| {
                ui.add_enabled_ui(draft, |ui| {
                    if chrome::choice(ui, "Measure", f.measured, true).clicked() {
                        f.measured = true;
                    }
                    if chrome::choice(ui, "Pass or fail", !f.measured, true).clicked() {
                        f.measured = false;
                    }
                });
                ui.add_space(8.0);
                ui.checkbox(&mut f.critical, "Critical");
            });
            ui.add_space(6.0);
            ui.horizontal(|ui| {
                ui.add(egui::TextEdit::singleline(&mut f.code).hint_text("Code, like WIDTH").desired_width(140.0));
                ui.add(egui::TextEdit::singleline(&mut f.name).hint_text("What to check").desired_width(220.0));
                if f.measured {
                    ui.add(egui::TextEdit::singleline(&mut f.lower).hint_text("Low").desired_width(70.0));
                    ui.add(egui::TextEdit::singleline(&mut f.upper).hint_text("High").desired_width(70.0));
                    ui.add(egui::TextEdit::singleline(&mut f.unit).hint_text("Unit").desired_width(50.0));
                }
            });
            ui.add_space(4.0);
            if f.measured {
                chrome::muted(ui, "Leave low and high empty to compare against the print by eye.");
            }
            ui.add_space(8.0);
            let ready = !self.char_form.code.trim().is_empty() && !busy;
            ui.horizontal(|ui| {
                let label = if editing { "Save check" } else { "Add check" };
                if ui.add_enabled_ui(ready, |ui| chrome::button(ui, label, Btn::Primary).clicked()).inner {
                    self.add_characteristic(id);
                }
                if editing && chrome::button(ui, "Cancel", Btn::Quiet).clicked() {
                    self.char_form = CharForm::default();
                }
            });
        });
        ui.add_space(12.0);
        if let Some(c) = edit_check {
            let num = |v: Option<f64>| v.map(|n| n.to_string()).unwrap_or_default();
            self.char_form = CharForm {
                measured: c.is_variable(),
                code: c.code.clone(),
                name: c.name.clone(),
                unit: c.unit.clone(),
                lower: num(c.lower_limit),
                upper: num(c.upper_limit),
                critical: c.critical,
                editing: Some(c.id),
            };
        }
        if let Some(cid) = remove_check {
            let client = self.authed_client();
            self.mutate_promise = Some(Promise::spawn_thread("remove-check", move || {
                client.delete_path(&format!("/api/v1/characteristics/{cid}"))?;
                Ok(super::MutatePayload::Ok("Check removed.".into()))
            }));
        }
    }

    fn ui_lot_panel(&mut self, ui: &mut egui::Ui) {
        let Some(row) = self.selected_row().cloned() else {
            return;
        };
        let Some(id) = self.catalog_selected else {
            return;
        };
        let p = chrome::palette(ui);
        let status = cell(&row, "status");
        let sup = self.role_at_least("supervisor");
        let inspector = self.role_at_least("inspector");
        let busy = self.mutate_promise.is_some();
        let product = self.product_cell(&cell(&row, "product_id"));
        let mut inspect = false;
        chrome::card(ui, "", |ui| {
            ui.horizontal(|ui| {
                ui.label(chrome::semi(cell(&row, "lot_number"), 16.0, p.ink));
                chrome::pill(ui, &status);
                chrome::muted(ui, format!("{product}  ·  {} {}", cell(&row, "quantity"), cell(&row, "unit")));
            });
            ui.add_space(10.0);
            ui.horizontal_wrapped(|ui| {
                ui.add_enabled_ui(!busy, |ui| {
                    if inspector
                        && !matches!(status.as_str(), "closed" | "on_hold")
                        && chrome::button(ui, "Inspect this build", Btn::Primary).clicked()
                    {
                        inspect = true;
                    }
                    if sup {
                        for (to, label) in lot_moves(&status) {
                            let kind = if *to == "rejected" { Btn::Danger } else { Btn::Quiet };
                            if chrome::button(ui, label, kind).clicked() {
                                let client = self.authed_client();
                                let to = to.to_string();
                                let label = label.to_string();
                                self.mutate_promise = Some(Promise::spawn_thread("lot-status", move || {
                                    let lot = client.set_lot_status(id, &to)?;
                                    Ok(super::MutatePayload::Ok(format!("{}: {}.", lot.lot_number, label.to_lowercase())))
                                }));
                            }
                        }
                    }
                });
            });
            if status == "in_inspection" {
                ui.add_space(6.0);
                chrome::muted(ui, "Finish or cancel the open inspection before changing this lot.");
            }
            if status == "on_hold" {
                ui.add_space(6.0);
                chrome::muted(ui, "On hold. A supervisor releases it before anyone inspects it again.");
            }
        });
        ui.add_space(12.0);
        if inspect {
            self.insp_form.lot_id = id.to_string();
            self.insp_form.error = None;
            self.goto(NavPage::Inspections);
            self.form = FormMode::CreateInspection;
        }
    }

    fn plan_call(&mut self, id: Uuid, action: Option<&'static str>, msg: &'static str) {
        let client = self.authed_client();
        let body = match action {
            Some("revise") => {
                let cur = self.selected_row().map(|r| cell(r, "revision")).unwrap_or_default();
                json!({ "revision": next_revision(&cur) })
            }
            _ => json!({}),
        };
        self.mutate_promise = Some(Promise::spawn_thread("plan-action", move || {
            match action {
                Some(a) => {
                    client.post_json(&format!("/api/v1/plans/{id}/{a}"), &body)?;
                }
                None => client.delete_path(&format!("/api/v1/plans/{id}"))?,
            }
            Ok(super::MutatePayload::Ok(msg.into()))
        }));
        if action.is_none() || action == Some("revise") {
            self.catalog_selected = None;
            self.catalog_plan = None;
        }
    }

    fn add_characteristic(&mut self, plan_id: Uuid) {
        let f = &self.char_form;
        let lower = if f.lower.trim().is_empty() { None } else { parse_measure(&f.lower) };
        let upper = if f.upper.trim().is_empty() { None } else { parse_measure(&f.upper) };
        if f.measured && ((!f.lower.trim().is_empty() && lower.is_none()) || (!f.upper.trim().is_empty() && upper.is_none())) {
            self.list_error = Some("Low and high must be numbers, like 36 or 36 1/2.".into());
            return;
        }
        let next = self
            .catalog_plan
            .as_ref()
            .map(|p| p.characteristics.iter().map(|c| c.sequence).max().unwrap_or(0) + 1)
            .unwrap_or(1);
        let code = f.code.trim().to_uppercase();
        let name = if f.name.trim().is_empty() { code.clone() } else { f.name.trim().to_string() };
        let body = if f.measured {
            json!({
                "sequence": next, "code": code, "name": name, "characteristic_type": "variable",
                "lower_limit": lower, "upper_limit": upper, "unit": f.unit.trim(), "critical": f.critical,
            })
        } else {
            json!({
                "sequence": next, "code": code, "name": name, "characteristic_type": "attribute",
                "critical": f.critical,
            })
        };
        let client = self.authed_client();
        if let Some(cid) = f.editing {
            let mut body = body;
            if let Some(map) = body.as_object_mut() {
                map.remove("sequence");
                if f.measured {
                    map.insert("clear_lower_limit".into(), json!(lower.is_none()));
                    map.insert("clear_upper_limit".into(), json!(upper.is_none()));
                }
            }
            self.char_form = CharForm::default();
            self.mutate_promise = Some(Promise::spawn_thread("edit-char", move || {
                client.patch_json(&format!("/api/v1/characteristics/{cid}"), &body)?;
                Ok(super::MutatePayload::Ok(format!("Check {code} saved.")))
            }));
            return;
        }
        self.char_form = CharForm { measured: self.char_form.measured, unit: self.char_form.unit.clone(), ..CharForm::default() };
        self.mutate_promise = Some(Promise::spawn_thread("add-char", move || {
            client.post_json(&format!("/api/v1/plans/{plan_id}/characteristics"), &body)?;
            Ok(super::MutatePayload::Ok(format!("Check {code} added.")))
        }));
    }

    fn ui_reports(&mut self, ui: &mut egui::Ui) {
        let reports = [
            ("Inspections", "CSV", "Every inspection, for a spreadsheet.", "/api/v1/reports/inspections.csv"),
            ("Alerts", "CSV", "Nonconformances and what was decided.", "/api/v1/reports/nonconformances.csv"),
            ("Actions", "CSV", "Corrective actions and their due dates.", "/api/v1/reports/corrective-actions.csv"),
            ("Inspections", "PDF", "A printable inspection packet.", "/api/v1/reports/inspections.pdf"),
            ("Summary", "PDF", "One page on how the shop is doing.", "/api/v1/reports/summary.pdf"),
        ];
        ui.horizontal_wrapped(|ui| {
            ui.spacing_mut().item_spacing = egui::vec2(12.0, 12.0);
            for (title, kind, detail, path) in reports {
                ui.allocate_ui(egui::vec2(240.0, 168.0), |ui| {
                    chrome::surface(ui, path, 168.0, |ui| {
                        let p = chrome::palette(ui);
                        chrome::pill(ui, kind);
                        ui.add_space(10.0);
                        ui.label(chrome::semi(title, 18.0, p.ink));
                        ui.add_space(4.0);
                        ui.label(egui::RichText::new(detail).color(p.muted).size(13.0));
                        ui.add_space(12.0);
                        if chrome::button(ui, "Download", Btn::Quiet).clicked() {
                            self.download_report(path, title);
                        }
                    });
                });
            }
        });
    }

    fn ui_stats_page(&mut self, ui: &mut egui::Ui) {
        if chrome::button(ui, "Load statistics", Btn::Primary).clicked() {
            self.spawn_stats();
        }
        ui.add_space(12.0);
        if let Some(text) = &self.stats_text {
            let p = chrome::palette(ui);
            egui::Frame::none()
                .fill(p.card)
                .stroke(egui::Stroke::new(1.0_f32, p.line))
                .rounding(egui::Rounding::same(14.0))
                .inner_margin(egui::Margin::same(16.0))
                .show(ui, |ui| {
                    egui::ScrollArea::vertical().show(ui, |ui| {
                        ui.label(egui::RichText::new(text).monospace().size(12.5));
                    });
                });
        }
    }

    fn product_cell(&self, id: &str) -> String {
        self.products
            .iter()
            .find(|p| p.id.to_string() == id)
            .map(|p| format!("{} {}", p.part_number, p.name))
            .unwrap_or_else(|| id.chars().take(8).collect())
    }

    fn site_cell(&self, id: &str) -> String {
        self.sites
            .iter()
            .find(|s| s.id.to_string() == id)
            .map(|s| s.code.clone())
            .unwrap_or_else(|| id.chars().take(8).collect())
    }

    /// Draws the table. Returns the id of a row that was clicked.
    fn catalog_table(&self, ui: &mut egui::Ui, columns: &[&str]) -> Option<Uuid> {
        use egui_extras::{Column, TableBuilder};
        let rows = &self.catalog_rows;
        if rows.is_empty() && !self.loading {
            let (title, body) = match self.page {
                NavPage::Search | NavPage::Measurements => ("Nothing to show yet.", "Type above and press Show."),
                _ => ("Nothing here yet.", "Create a record above."),
            };
            super::widgets::empty_state(ui, title, body);
            return None;
        }
        let selectable = super::editor::edit_spec(self.page).is_some();
        let selected = self.catalog_selected.map(|id| id.to_string());
        let muted = chrome::palette(ui).muted;
        let mut clicked = None;
        TableBuilder::new(ui)
            .striped(true)
            .resizable(true)
            .sense(if selectable { egui::Sense::click() } else { egui::Sense::hover() })
            .cell_layout(egui::Layout::left_to_right(egui::Align::Center))
            .column(Column::remainder().at_least(140.0))
            .columns(Column::remainder().at_least(90.0), columns.len().saturating_sub(1).max(1))
            .header(32.0, |mut header| {
                for col in columns {
                    header.col(|ui| {
                        let title = match *col {
                            "product_id" => "Product".to_string(),
                            "site_id" => "Site".to_string(),
                            "entity_type" => "Type".to_string(),
                            c => chrome::pretty_key(c),
                        };
                        ui.label(chrome::semi(title, 12.0, muted));
                    });
                }
            })
            .body(|body| {
                body.rows(38.0, rows.len(), |mut row| {
                    let idx = row.index();
                    let item = &rows[idx];
                    let id = item.get("id").and_then(Value::as_str).unwrap_or_default();
                    row.set_selected(selected.as_deref() == Some(id));
                    for col in columns {
                        row.col(|ui| {
                            let raw = cell(item, col);
                            let value = match *col {
                                "product_id" if raw != "—" => self.product_cell(&raw),
                                "site_id" if raw != "—" => self.site_cell(&raw),
                                "source" | "sampling_type" | "role" => choices(col)
                                    .unwrap_or_default()
                                    .iter()
                                    .find(|(k, _)| *k == raw)
                                    .map(|(_, v)| v.to_string())
                                    .unwrap_or(raw),
                                _ => raw,
                            };
                            if matches!(*col, "status" | "severity" | "result" | "disposition" | "active" | "in_spec")
                                && value != "—"
                            {
                                chrome::pill(ui, &value);
                            } else {
                                ui.add(egui::Label::new(value).selectable(false).truncate());
                            }
                        });
                    }
                    if selectable && row.response().clicked() {
                        clicked = Uuid::parse_str(id).ok();
                    }
                });
            });
        clicked
    }

    fn submit_catalog_create(&mut self) {
        if self.mutate_promise.is_some() {
            return;
        }
        let Some(spec) = self.page.catalog() else {
            return;
        };
        let Some(path) = spec.create_path else {
            return;
        };
        let mut body = serde_json::Map::new();
        for (key, value) in &self.catalog_form {
            let value = value.trim();
            if value.is_empty() && !matches!(key.as_str(), "notes" | "description" | "address") {
                continue;
            }
            match key.as_str() {
                "quantity" => match parse_measure(value) {
                    Some(n) => {
                        body.insert(key.clone(), json!(n));
                    }
                    None => {
                        self.list_error = Some("Quantity must be a number.".into());
                        return;
                    }
                },
                "sample_size" => match value.parse::<i64>() {
                    Ok(n) => {
                        body.insert(key.clone(), json!(n));
                    }
                    Err(_) => {
                        self.list_error = Some("Sample size must be a whole number.".into());
                        return;
                    }
                },
                "events" => {
                    let events: Vec<&str> = value.split(',').map(str::trim).filter(|s| !s.is_empty()).collect();
                    body.insert(key.clone(), json!(events));
                }
                _ => {
                    body.insert(key.clone(), json!(value));
                }
            }
        }
        for required in ["product_id", "site_id"] {
            if spec.fields.iter().any(|(k, _)| *k == required) && !body.contains_key(required) {
                self.list_error = Some(format!(
                    "Pick a {}.",
                    if required == "product_id" { "product" } else { "site" }
                ));
                return;
            }
        }
        if self.page == NavPage::Plans && body.get("sampling_type") != Some(&json!("fixed")) {
            body.remove("sample_size");
        }
        if self.page == NavPage::Checklists {
            body.insert("items".into(), json!([]));
        }
        let client = self.authed_client();
        let path = path.to_string();
        let value = Value::Object(body);
        self.list_error = None;
        self.mutate_promise = Some(Promise::spawn_thread("create-catalog", move || {
            client.post_json(&path, &value)?;
            Ok(super::MutatePayload::Ok("Saved.".into()))
        }));
    }

    fn download_report(&mut self, path: &str, label: &str) {
        if self.mutate_promise.is_some() {
            return;
        }
        let client = self.authed_client();
        let path = path.to_string();
        let label = label.to_string();
        self.mutate_promise = Some(Promise::spawn_thread("report", move || {
            let bytes = client.download(&path)?;
            let ext = if path.ends_with(".pdf") { "pdf" } else { "csv" };
            let stamp = chrono::Local::now().format("%Y-%m-%d");
            let name = format!("{label}-{stamp}.{ext}").replace(' ', "-");
            let dest = directories::UserDirs::new()
                .and_then(|dirs| dirs.download_dir().map(|p| p.to_path_buf()))
                .unwrap_or_else(std::env::temp_dir)
                .join(name);
            std::fs::write(&dest, &bytes).map_err(|e| ApiError::Invalid(e.to_string()))?;
            Ok(super::MutatePayload::Ok(format!("Saved {}", dest.display())))
        }));
    }

    pub(super) fn spawn_stats(&mut self) {
        if self.mutate_promise.is_some() {
            return;
        }
        let client = self.authed_client();
        self.mutate_promise = Some(Promise::spawn_thread("stats", move || {
            let defects = client.get_json("/api/v1/stats/defect-rates?group_by=product")?;
            let ncr = client.get_json("/api/v1/stats/ncr-aging")?;
            let capa = client.get_json("/api/v1/stats/capa-aging")?;
            let trends = client.get_json("/api/v1/stats/trends")?;
            let text = format!(
                "Defect rates\n{}\n\nNCR aging\n{}\n\nCAPA aging\n{}\n\nTrends\n{}",
                serde_json::to_string_pretty(&defects).unwrap_or_default(),
                serde_json::to_string_pretty(&ncr).unwrap_or_default(),
                serde_json::to_string_pretty(&capa).unwrap_or_default(),
                serde_json::to_string_pretty(&trends).unwrap_or_default()
            );
            Ok(super::MutatePayload::Stats(text))
        }));
    }
}

/// A to B, 3 to 4. Anything else gets a suffix.
fn next_revision(cur: &str) -> String {
    let cur = cur.trim();
    if let Ok(n) = cur.parse::<u32>() {
        return (n + 1).to_string();
    }
    let mut chars = cur.chars();
    if let (Some(c), None) = (chars.next(), chars.next()) {
        if c.is_ascii_alphabetic() && !matches!(c, 'z' | 'Z') {
            return char::from(c.to_ascii_uppercase() as u8 + 1).to_string();
        }
    }
    format!("{cur}-2")
}

fn cell(value: &Value, key: &str) -> String {
    match value.get(key) {
        Some(Value::String(s)) => s.clone(),
        Some(Value::Bool(b)) => b.to_string(),
        Some(Value::Number(n)) => n.to_string(),
        Some(Value::Null) | None => "—".into(),
        Some(other) => other.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn lot_moves_match_server_rules() {
        assert!(lot_moves("open").iter().any(|(to, _)| *to == "on_hold"));
        assert!(!lot_moves("open").iter().any(|(to, _)| *to == "accepted"));
        assert!(lot_moves("in_inspection").is_empty());
        assert_eq!(lot_moves("closed"), &[("open", "Reopen")]);
    }

    #[test]
    fn revisions_step_forward() {
        assert_eq!(next_revision("A"), "B");
        assert_eq!(next_revision("c"), "D");
        assert_eq!(next_revision("3"), "4");
        assert_eq!(next_revision("Z"), "Z-2");
    }

    #[test]
    fn create_fields_start_blank_except_defaults() {
        assert_eq!(field_default("lot_number"), "");
        assert_eq!(field_default("source"), "in_process");
    }
}
