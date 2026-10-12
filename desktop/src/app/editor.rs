//! Edit and delete for the selected row on any catalog page.

use super::chrome::{self, Btn};
use super::records::parse_measure;
use super::{NavPage, QcApp};
use eframe::egui;
use poll_promise::Promise;
use serde_json::{json, Map, Value};
use uuid::Uuid;

#[derive(Clone, Copy, PartialEq)]
pub(super) enum Kind {
    Text,
    Long,
    Number,
    Whole,
    Date,
    Toggle,
    Secret,
    Events,
    Product,
    Supplier,
    Choice(&'static [(&'static str, &'static str)]),
}

pub(super) struct EditSpec {
    pub fields: &'static [(&'static str, Kind)],
    /// Lowest role that may save changes. The server enforces the same rule.
    pub role: &'static str,
    /// Whether an administrator may delete the row from this page.
    pub deletable: bool,
}

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

pub(super) fn edit_spec(page: NavPage) -> Option<EditSpec> {
    use Kind::*;
    Some(match page {
        NavPage::Lots => EditSpec {
            fields: &[
                ("lot_number", Text),
                ("quantity", Number),
                ("unit", Text),
                ("supplier_id", Supplier),
                ("supplier_lot", Text),
                ("notes", Long),
            ],
            role: "operator",
            deletable: true,
        },
        NavPage::Products => EditSpec {
            fields: &[
                ("part_number", Text),
                ("name", Text),
                ("unit", Text),
                ("description", Long),
                ("active", Toggle),
            ],
            role: "quality_manager",
            deletable: true,
        },
        NavPage::Plans => EditSpec {
            fields: &[
                ("name", Text),
                ("sampling_type", Choice(SAMPLING)),
                ("sample_size", Whole),
                ("notes", Long),
            ],
            role: "quality_manager",
            deletable: false,
        },
        NavPage::Sites => EditSpec {
            fields: &[("code", Text), ("name", Text), ("address", Long), ("active", Toggle)],
            role: "supervisor",
            deletable: true,
        },
        NavPage::Stations => EditSpec {
            fields: &[("code", Text), ("name", Text), ("description", Long), ("active", Toggle)],
            role: "supervisor",
            deletable: true,
        },
        NavPage::Suppliers => EditSpec {
            fields: &[
                ("code", Text),
                ("name", Text),
                ("contact_name", Text),
                ("email", Text),
                ("phone", Text),
                ("active", Toggle),
            ],
            role: "quality_manager",
            deletable: true,
        },
        NavPage::Gauges => EditSpec {
            fields: &[
                ("code", Text),
                ("name", Text),
                ("serial_number", Text),
                ("calibration_due", Date),
                ("active", Toggle),
            ],
            role: "quality_manager",
            deletable: true,
        },
        NavPage::Defects => EditSpec {
            fields: &[
                ("code", Text),
                ("name", Text),
                ("severity", Choice(SEVERITIES)),
                ("category", Text),
                ("active", Toggle),
            ],
            role: "quality_manager",
            deletable: true,
        },
        NavPage::Checklists => EditSpec {
            fields: &[
                ("code", Text),
                ("name", Text),
                ("product_id", Product),
                ("description", Long),
                ("active", Toggle),
            ],
            role: "quality_manager",
            deletable: true,
        },
        NavPage::Users => EditSpec {
            fields: &[
                ("full_name", Text),
                ("email", Text),
                ("role", Choice(ROLES)),
                ("active", Toggle),
            ],
            role: "admin",
            deletable: true,
        },
        NavPage::Webhooks => EditSpec {
            fields: &[
                ("name", Text),
                ("url", Text),
                ("events", Events),
                ("secret", Secret),
                ("description", Long),
                ("active", Toggle),
            ],
            role: "supervisor",
            deletable: true,
        },
        _ => return None,
    })
}

/// Which row fields go on a QR label: the code, the name line, and a small line under it.
fn label_fields(page: NavPage) -> Option<(&'static str, &'static str, &'static str)> {
    match page {
        NavPage::Lots => Some(("lot_number", "", "")),
        NavPage::Products => Some(("part_number", "name", "unit")),
        NavPage::Gauges => Some(("code", "name", "serial_number")),
        NavPage::Stations => Some(("code", "name", "description")),
        _ => None,
    }
}

/// The edit form for one selected row.
pub(super) struct RecordEdit {
    id: Uuid,
    page: NavPage,
    source: Value,
    values: Vec<(String, String)>,
    original: Vec<(String, String)>,
    password: String,
    confirm_delete: bool,
    /// A save was sent, so the next reload of the row replaces the form.
    saved: bool,
}

/// Text form of a row value, as the edit fields show it.
fn as_text(v: Option<&Value>) -> String {
    match v {
        Some(Value::String(s)) => s.clone(),
        Some(Value::Bool(b)) => b.to_string(),
        Some(Value::Number(n)) => n.to_string(),
        Some(Value::Array(items)) => items.iter().filter_map(Value::as_str).collect::<Vec<_>>().join(", "),
        _ => String::new(),
    }
}

impl RecordEdit {
    fn new(page: NavPage, id: Uuid, row: &Value, spec: &EditSpec) -> Self {
        let values: Vec<(String, String)> = spec
            .fields
            .iter()
            .map(|(k, kind)| {
                let text = match kind {
                    Kind::Secret => String::new(),
                    Kind::Date => as_text(row.get(*k)).chars().take(10).collect(),
                    _ => as_text(row.get(*k)),
                };
                ((*k).to_string(), text)
            })
            .collect();
        Self {
            id,
            page,
            source: row.clone(),
            original: values.clone(),
            values,
            password: String::new(),
            confirm_delete: false,
            saved: false,
        }
    }

    fn changed(&self) -> bool {
        self.values != self.original
    }
}

/// Builds the PATCH body from the fields that changed.
pub(super) fn patch_body(
    spec: &EditSpec,
    values: &[(String, String)],
    original: &[(String, String)],
) -> Result<Value, String> {
    let mut body = Map::new();
    let sampling = values.iter().find(|(k, _)| k == "sampling_type").map(|(_, v)| v.as_str());
    for (((key, value), (_, before)), (_, kind)) in values.iter().zip(original).zip(spec.fields) {
        if value == before {
            continue;
        }
        let v = value.trim();
        let label = chrome::pretty_key(key);
        match kind {
            Kind::Toggle => {
                body.insert(key.clone(), json!(v == "true"));
            }
            Kind::Number => match parse_measure(v) {
                Some(n) => {
                    body.insert(key.clone(), json!(n));
                }
                None => return Err(format!("{label} must be a number.")),
            },
            Kind::Whole => {
                if v.is_empty() {
                    body.insert("clear_sample_size".into(), json!(true));
                } else {
                    match v.parse::<i64>() {
                        Ok(n) if n > 0 => {
                            body.insert(key.clone(), json!(n));
                        }
                        _ => return Err(format!("{label} must be a whole number above zero.")),
                    }
                }
            }
            Kind::Events => {
                let events: Vec<&str> = v.split(',').map(str::trim).filter(|s| !s.is_empty()).collect();
                body.insert(key.clone(), json!(events));
            }
            Kind::Secret if v.is_empty() => {}
            Kind::Product | Kind::Supplier => {
                if v.is_empty() {
                    return Err(format!("{label} can't be cleared here; pick another one."));
                }
                body.insert(key.clone(), json!(v));
            }
            Kind::Text if v.is_empty() && matches!(key.as_str(), "code" | "name" | "part_number" | "lot_number" | "url") => {
                return Err(format!("{label} can't be empty."));
            }
            _ => {
                body.insert(key.clone(), json!(v));
            }
        }
    }
    if sampling.is_some_and(|s| s != "fixed") {
        body.remove("sample_size");
        let had_size = original.iter().any(|(k, v)| k == "sample_size" && !v.is_empty());
        if had_size {
            body.insert("clear_sample_size".into(), json!(true));
        } else {
            body.remove("clear_sample_size");
        }
    }
    Ok(Value::Object(body))
}

impl QcApp {
    /// Keeps the edit form in step with the selected row, rebuilding it when the row reloads.
    pub(super) fn sync_record_edit(&mut self) {
        let Some(spec) = edit_spec(self.page) else {
            self.record_edit = None;
            return;
        };
        let (Some(id), Some(row)) = (self.catalog_selected, self.selected_row().cloned()) else {
            self.record_edit = None;
            return;
        };
        let stale = match &self.record_edit {
            Some(e) => e.id != id || e.page != self.page || (e.source != row && (e.saved || !e.changed())),
            None => true,
        };
        if stale {
            self.record_edit = Some(RecordEdit::new(self.page, id, &row, &spec));
        }
    }

    pub(super) fn ui_record_editor(&mut self, ui: &mut egui::Ui) {
        let Some(spec) = edit_spec(self.page) else {
            return;
        };
        let Some(path) = self.page.catalog().map(|c| c.path) else {
            return;
        };
        if self.record_edit.is_none() {
            return;
        }
        if self.page == NavPage::Plans {
            let status = self.selected_row().map(|r| as_text(r.get("status"))).unwrap_or_default();
            if status != "draft" && !self.role_at_least("admin") {
                return;
            }
        }
        let may_edit = self.role_at_least(spec.role);
        let admin = self.role_at_least("admin");
        let busy = self.mutate_promise.is_some();
        let products = self.products.clone();
        let suppliers = self.suppliers.clone();
        let p = chrome::palette(ui);
        let open = !matches!(self.page, NavPage::Lots | NavPage::Plans);
        let mut save = false;
        let mut delete = false;
        let mut set_password = false;
        let mut save_label = false;
        egui::CollapsingHeader::new(chrome::semi("Edit details", 13.5, p.ink))
            .id_salt(("edit-record", path))
            .default_open(open)
            .show(ui, |ui| {
                let Some(edit) = self.record_edit.as_mut() else {
                    return;
                };
                chrome::card(ui, "", |ui| {
                    if !may_edit {
                        chrome::muted(ui, "Your role can view this but not change it.");
                        ui.add_space(6.0);
                    }
                    ui.add_enabled_ui(may_edit, |ui| {
                        for ((key, value), (_, kind)) in edit.values.iter_mut().zip(spec.fields) {
                            draw_field(ui, key, value, *kind, &products, &suppliers);
                            ui.add_space(6.0);
                        }
                    });
                    ui.add_space(4.0);
                    ui.horizontal_wrapped(|ui| {
                        let can_save = may_edit && !busy && edit.changed();
                        if ui.add_enabled_ui(can_save, |ui| chrome::button(ui, "Save changes", Btn::Primary).clicked()).inner {
                            save = true;
                        }
                        if edit.changed() && chrome::button(ui, "Undo", Btn::Quiet).clicked() {
                            edit.values = edit.original.clone();
                        }
                        if label_fields(edit.page).is_some()
                            && chrome::button(ui, "Save QR label…", Btn::Quiet)
                                .on_hover_text("A PNG to print and stick on it. Scanning it in the iPhone or iPad app opens this record.")
                                .clicked()
                        {
                            save_label = true;
                        }
                        if spec.deletable && admin {
                            ui.add_space(16.0);
                            if edit.confirm_delete {
                                chrome::muted(ui, "Delete for good?");
                                if ui.add_enabled_ui(!busy, |ui| chrome::button(ui, "Yes, delete", Btn::Danger).clicked()).inner {
                                    delete = true;
                                }
                                if chrome::button(ui, "Keep it", Btn::Quiet).clicked() {
                                    edit.confirm_delete = false;
                                }
                            } else if chrome::button(ui, "Delete", Btn::Danger).clicked() {
                                edit.confirm_delete = true;
                            }
                        }
                    });
                    if spec.deletable && admin && edit.confirm_delete {
                        ui.add_space(4.0);
                        chrome::muted(
                            ui,
                            "Records still used elsewhere can't be deleted. The server says what uses it; turn it off instead.",
                        );
                    }
                    if self.page == NavPage::Users && admin {
                        ui.add_space(12.0);
                        chrome::field_label(ui, "Set a new password");
                        ui.horizontal(|ui| {
                            ui.add(
                                egui::TextEdit::singleline(&mut edit.password)
                                    .password(true)
                                    .hint_text("At least 10 characters")
                                    .desired_width(260.0)
                                    .margin(egui::Margin::symmetric(10.0, 8.0)),
                            );
                            let ok = edit.password.chars().count() >= 10 && !busy;
                            if ui.add_enabled_ui(ok, |ui| chrome::button(ui, "Set password", Btn::Quiet).clicked()).inner {
                                set_password = true;
                            }
                        });
                    }
                });
            });
        ui.add_space(12.0);
        if save {
            self.save_record_edit(path, &spec);
        } else if delete {
            self.delete_record(path);
        } else if set_password {
            self.set_user_password();
        } else if save_label {
            self.save_label();
        }
    }

    fn save_label(&mut self) {
        let Some(edit) = &self.record_edit else {
            return;
        };
        let Some((code_key, title_key, sub_key)) = label_fields(edit.page) else {
            return;
        };
        let row = &edit.source;
        let code = as_text(row.get(code_key));
        let title = match edit.page {
            NavPage::Lots => self
                .products
                .iter()
                .find(|p| p.id.to_string() == as_text(row.get("product_id")))
                .map(|p| p.name.clone())
                .unwrap_or_default(),
            _ => as_text(row.get(title_key)),
        };
        let subtitle = match edit.page {
            NavPage::Lots => format!("{} {}", as_text(row.get("quantity")), as_text(row.get("unit"))),
            _ => as_text(row.get(sub_key)),
        };
        let png = match crate::label::render_png(&code, &title, &subtitle) {
            Ok(png) => png,
            Err(e) => {
                self.list_error = Some(format!("Could not make the label: {e}"));
                return;
            }
        };
        let Some(path) = rfd::FileDialog::new()
            .set_file_name(format!("label-{}.png", code.replace(['/', '\\', ' '], "-")))
            .add_filter("PNG image", &["png"])
            .save_file()
        else {
            return;
        };
        match std::fs::write(&path, png) {
            Ok(()) => self.status_msg = Some(format!("Label saved to {}", path.display())),
            Err(e) => self.list_error = Some(format!("Could not save the label: {e}")),
        }
    }

    fn save_record_edit(&mut self, path: &str, spec: &EditSpec) {
        if self.mutate_promise.is_some() {
            return;
        }
        let Some(edit) = &self.record_edit else {
            return;
        };
        let body = match patch_body(spec, &edit.values, &edit.original) {
            Ok(b) => b,
            Err(msg) => {
                self.list_error = Some(msg);
                return;
            }
        };
        let id = edit.id;
        let client = self.authed_client();
        let path = format!("{path}/{id}");
        self.list_error = None;
        self.mutate_promise = Some(Promise::spawn_thread("edit-record", move || {
            client.patch_json(&path, &body)?;
            Ok(super::MutatePayload::Ok("Changes saved.".into()))
        }));
        if let Some(edit) = self.record_edit.as_mut() {
            edit.saved = true;
        }
    }

    fn delete_record(&mut self, path: &str) {
        if self.mutate_promise.is_some() {
            return;
        }
        let Some(edit) = &self.record_edit else {
            return;
        };
        let id = edit.id;
        let client = self.authed_client();
        let path = format!("{path}/{id}");
        self.list_error = None;
        self.mutate_promise = Some(Promise::spawn_thread("delete-record", move || {
            client.delete_path(&path)?;
            Ok(super::MutatePayload::Ok("Deleted.".into()))
        }));
        self.catalog_selected = None;
        self.catalog_plan = None;
        self.catalog_extra = None;
        self.record_edit = None;
    }

    fn set_user_password(&mut self) {
        if self.mutate_promise.is_some() {
            return;
        }
        let Some(edit) = self.record_edit.as_mut() else {
            return;
        };
        let id = edit.id;
        let password = std::mem::take(&mut edit.password);
        let client = self.authed_client();
        self.mutate_promise = Some(Promise::spawn_thread("set-password", move || {
            client.post_json(&format!("/api/v1/users/{id}/password"), &json!({ "password": password }))?;
            Ok(super::MutatePayload::Ok("Password set. Their other sessions were signed out.".into()))
        }));
    }
}

fn draw_field(
    ui: &mut egui::Ui,
    key: &str,
    value: &mut String,
    kind: Kind,
    products: &[crate::api::Product],
    suppliers: &[Value],
) {
    let label = match key {
        "product_id" => "Product".to_string(),
        "supplier_id" => "Supplier".to_string(),
        "events" => "Events (comma separated)".to_string(),
        "secret" => "Secret (leave blank to keep it)".to_string(),
        "calibration_due" => "Calibration due (YYYY-MM-DD)".to_string(),
        "sample_size" => "Sample size (fixed count only)".to_string(),
        other => chrome::pretty_key(other),
    };
    match kind {
        Kind::Toggle => {
            let mut on = value == "true";
            let text = if on { "Active" } else { "Turned off" };
            if ui.checkbox(&mut on, text).changed() {
                *value = on.to_string();
            }
            return;
        }
        _ => chrome::field_label(ui, &label),
    }
    let line = |ui: &mut egui::Ui, value: &mut String, hint: &str, password: bool| {
        ui.add(
            egui::TextEdit::singleline(value)
                .password(password)
                .hint_text(hint)
                .desired_width(f32::INFINITY)
                .margin(egui::Margin::symmetric(10.0, 8.0)),
        );
    };
    match kind {
        Kind::Choice(options) => {
            ui.horizontal_wrapped(|ui| {
                for (v, text) in options {
                    if chrome::choice(ui, text, value == v, true).clicked() {
                        *value = (*v).to_string();
                    }
                }
            });
        }
        Kind::Long => {
            ui.add(
                egui::TextEdit::multiline(value)
                    .desired_rows(2)
                    .desired_width(f32::INFINITY)
                    .margin(egui::Margin::symmetric(10.0, 8.0)),
            );
        }
        Kind::Product => {
            let shown = products
                .iter()
                .find(|p| p.id.to_string() == *value)
                .map(|p| format!("{}  {}", p.part_number, p.name))
                .unwrap_or_else(|| "Any product".into());
            egui::ComboBox::from_id_salt(("edit-pick", key))
                .width(ui.available_width())
                .selected_text(shown)
                .show_ui(ui, |ui| {
                    for p in products {
                        ui.selectable_value(value, p.id.to_string(), format!("{}  {}", p.part_number, p.name));
                    }
                });
        }
        Kind::Supplier => {
            let name = |s: &Value| format!("{}  {}", as_text(s.get("code")), as_text(s.get("name")));
            let shown = suppliers
                .iter()
                .find(|s| as_text(s.get("id")) == *value)
                .map(name)
                .unwrap_or_else(|| "No supplier".into());
            egui::ComboBox::from_id_salt(("edit-pick", key))
                .width(ui.available_width())
                .selected_text(shown)
                .show_ui(ui, |ui| {
                    for s in suppliers {
                        ui.selectable_value(value, as_text(s.get("id")), name(s));
                    }
                });
        }
        Kind::Secret => line(ui, value, "Unchanged", true),
        Kind::Date => line(ui, value, "2027-01-31", false),
        Kind::Number => line(ui, value, "1", false),
        Kind::Whole => line(ui, value, "Blank for every piece", false),
        _ => line(ui, value, "", false),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pairs(v: &[(&str, &str)]) -> Vec<(String, String)> {
        v.iter().map(|(a, b)| ((*a).to_string(), (*b).to_string())).collect()
    }

    #[test]
    fn only_changed_fields_are_sent() {
        let spec = edit_spec(NavPage::Products).unwrap();
        let before = pairs(&[("part_number", "GATE-12"), ("name", "Gate"), ("unit", "ea"), ("description", ""), ("active", "true")]);
        let mut after = before.clone();
        after[1].1 = "Corral gate 12 ft".into();
        after[4].1 = "false".into();
        let body = patch_body(&spec, &after, &before).unwrap();
        assert_eq!(body, json!({ "name": "Corral gate 12 ft", "active": false }));
    }

    #[test]
    fn numbers_and_events_are_typed() {
        let lots = edit_spec(NavPage::Lots).unwrap();
        let before = pairs(&[("lot_number", "L1"), ("quantity", "4"), ("unit", "ea"), ("supplier_id", ""), ("supplier_lot", ""), ("notes", "")]);
        let mut after = before.clone();
        after[1].1 = "6 1/2".into();
        assert_eq!(patch_body(&lots, &after, &before).unwrap(), json!({ "quantity": 6.5 }));
        after[1].1 = "lots".into();
        assert!(patch_body(&lots, &after, &before).is_err());

        let hooks = edit_spec(NavPage::Webhooks).unwrap();
        let before = pairs(&[("name", "h"), ("url", "https://x"), ("events", "a"), ("secret", ""), ("description", ""), ("active", "true")]);
        let mut after = before.clone();
        after[2].1 = "a, b ,".into();
        assert_eq!(patch_body(&hooks, &after, &before).unwrap(), json!({ "events": ["a", "b"] }));
    }

    #[test]
    fn leaving_fixed_sampling_clears_the_size() {
        let spec = edit_spec(NavPage::Plans).unwrap();
        let before = pairs(&[("name", "Final"), ("sampling_type", "fixed"), ("sample_size", "5"), ("notes", "")]);
        let mut after = before.clone();
        after[1].1 = "hundred_percent".into();
        assert_eq!(
            patch_body(&spec, &after, &before).unwrap(),
            json!({ "sampling_type": "hundred_percent", "clear_sample_size": true })
        );
    }

    #[test]
    fn required_text_cannot_be_blanked() {
        let spec = edit_spec(NavPage::Sites).unwrap();
        let before = pairs(&[("code", "SHOP"), ("name", "Shop"), ("address", ""), ("active", "true")]);
        let mut after = before.clone();
        after[0].1 = " ".into();
        assert!(patch_body(&spec, &after, &before).is_err());
    }
}
