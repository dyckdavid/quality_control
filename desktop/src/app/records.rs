use super::chrome::{self, Btn};
use super::widgets::{empty_state, error_banner, loading_spinner, quiet_button};
use super::{DetailView, FormMode, MutatePayload, NavPage, QcApp};
use crate::api::{
    ApiClient, ApiError, CompleteInspection, DispositionRequest, InspectionDetail,
    InspectionResult, Lot, NewInspectionDefect, PlanDetail, ResultBatch, ResultInput,
};
use eframe::egui;
use std::collections::HashMap;
use uuid::Uuid;

pub(super) struct InspectionBundle {
    pub detail: InspectionDetail,
    pub plan: Option<PlanDetail>,
    pub lot: Option<Lot>,
}

/// Everything typed on an open inspection. Kept across reloads only until the server answers.
pub(super) struct Workbench {
    pub detail: InspectionDetail,
    pub plan: Option<PlanDetail>,
    pub lot: Option<Lot>,
    values: HashMap<(Uuid, i32), String>,
    attrs: HashMap<Uuid, String>,
    gauge: Option<Uuid>,
    defect: Option<Uuid>,
    defect_qty: String,
    defect_notes: String,
    qty_inspected: String,
    qty_rejected: String,
    notes: String,
    open_ncr: bool,
    confirm_cancel: bool,
    /// The inspection notes as typed; saved on their own with PATCH.
    header_notes: String,
}

/// Drafts for the status buttons on an alert or corrective action.
#[derive(Default)]
pub(super) struct ActionDraft {
    pub disposition: String,
    pub notes: String,
    pub confirm: Option<&'static str>,
}

enum WorkAction {
    Save(Vec<ResultInput>),
    AddDefect(NewInspectionDefect),
    RemoveDefect(Uuid),
    Complete(CompleteInspection),
    Cancel,
    OpenNcr(Uuid),
    SaveNotes(String),
}

const DISPOSITIONS: &[(&str, &str, &str)] = &[
    ("rework", "Rework", "Fix it in the shop and check it again."),
    ("repair", "Repair", "Fix it so it works, even if it is not to print."),
    ("use_as_is", "Use as is", "Ship it. The miss does not matter for this build."),
    ("scrap", "Scrap", "Cut it up or throw it out."),
    ("return_to_supplier", "Return to supplier", "Bad steel, glass, or hardware. Send it back."),
    ("sort", "Sort", "Check every piece and pull the bad ones."),
];

pub(super) fn role_rank(role: &str) -> u8 {
    match role {
        "operator" => 1,
        "inspector" => 2,
        "supervisor" => 3,
        "quality_manager" => 4,
        "admin" => 5,
        _ => 0,
    }
}

/// Reads a tape measurement: `36.5`, `36 1/2`, `36-1/2`, `1/2`, or `36 1/2"`.
pub(super) fn parse_measure(text: &str) -> Option<f64> {
    let t = text.trim().trim_end_matches('"').trim();
    if t.is_empty() {
        return None;
    }
    if let Ok(v) = t.parse::<f64>() {
        return v.is_finite().then_some(v);
    }
    let (whole, frac) = match t.rsplit_once([' ', '-']) {
        Some((w, f)) if f.contains('/') => (w.trim(), f),
        _ if t.contains('/') => ("", t),
        _ => return None,
    };
    let (num, den) = frac.split_once('/')?;
    let num: f64 = num.trim().parse().ok()?;
    let den: f64 = den.trim().parse().ok()?;
    if den == 0.0 {
        return None;
    }
    let negative = if whole.is_empty() { t.starts_with('-') } else { whole.starts_with('-') };
    let whole: f64 = if whole.is_empty() { 0.0 } else { whole.parse().ok()? };
    let part = num / den;
    let v = if negative { whole - part } else { whole + part };
    v.is_finite().then_some(v)
}

fn fmt_num(v: f64) -> String {
    if v.fract() == 0.0 {
        format!("{v:.0}")
    } else {
        let s = format!("{v:.4}");
        s.trim_end_matches('0').trim_end_matches('.').to_string()
    }
}

fn spec_text(ch: &crate::api::Characteristic) -> String {
    let unit = if ch.unit.is_empty() { String::new() } else { format!(" {}", ch.unit) };
    match (ch.lower_limit, ch.upper_limit) {
        (Some(lo), Some(hi)) => format!("{} to {}{unit}", fmt_num(lo), fmt_num(hi)),
        (Some(lo), None) => format!("at least {}{unit}", fmt_num(lo)),
        (None, Some(hi)) => format!("at most {}{unit}", fmt_num(hi)),
        (None, None) if ch.is_variable() => format!("Compare to the print{unit}"),
        (None, None) => "Pass or fail".to_string(),
    }
}

impl Workbench {
    pub fn new(b: InspectionBundle) -> Self {
        let mut values = HashMap::new();
        let mut attrs = HashMap::new();
        for r in &b.detail.results {
            if let Some(v) = r.measured_value {
                values.insert((r.characteristic_id, r.sample_index), fmt_num(v));
            }
            if let Some(a) = &r.attribute_result {
                attrs.insert(r.characteristic_id, a.clone());
            }
        }
        let qty = b
            .lot
            .as_ref()
            .map(|l| l.quantity)
            .unwrap_or(b.detail.inspection.sample_size as f64);
        let gauge = b.detail.results.iter().find_map(|r| r.gauge_id);
        Self {
            values,
            attrs,
            gauge,
            defect: None,
            defect_qty: "1".into(),
            defect_notes: String::new(),
            qty_inspected: fmt_num(qty),
            qty_rejected: "0".into(),
            notes: String::new(),
            open_ncr: true,
            confirm_cancel: false,
            header_notes: b.detail.inspection.notes.clone(),
            detail: b.detail,
            plan: b.plan,
            lot: b.lot,
        }
    }

    /// Carries unsaved typing over when the server sends a fresh copy.
    pub fn absorb(&mut self, b: InspectionBundle) {
        let keep_values = std::mem::take(&mut self.values);
        let keep_attrs = std::mem::take(&mut self.attrs);
        let same = self.detail.inspection.id == b.detail.inspection.id;
        let fresh = Workbench::new(b);
        let (gauge, defect, inspected, rejected, notes, open_ncr) = (
            self.gauge,
            self.defect,
            self.qty_inspected.clone(),
            self.qty_rejected.clone(),
            self.notes.clone(),
            self.open_ncr,
        );
        *self = fresh;
        if same {
            for (k, v) in keep_values {
                self.values.entry(k).or_insert(v);
            }
            for (k, v) in keep_attrs {
                self.attrs.entry(k).or_insert(v);
            }
            self.gauge = gauge.or(self.gauge);
            self.defect = defect;
            self.qty_inspected = inspected;
            self.qty_rejected = rejected;
            self.notes = notes;
            self.open_ncr = open_ncr;
        }
    }

    fn in_progress(&self) -> bool {
        self.detail.inspection.status == "in_progress"
    }

    fn saved(&self, ch: Uuid, sample: i32) -> Option<&InspectionResult> {
        self.detail
            .results
            .iter()
            .find(|r| r.characteristic_id == ch && r.sample_index == sample)
    }

    fn samples(&self) -> i32 {
        self.detail.inspection.sample_size.max(1)
    }

    /// Readings typed or changed since the last save.
    fn pending(&self) -> Result<Vec<ResultInput>, String> {
        let Some(plan) = &self.plan else {
            return Ok(Vec::new());
        };
        let mut out = Vec::new();
        for ch in &plan.characteristics {
            if ch.is_variable() {
                for s in 1..=self.samples() {
                    let Some(text) = self.values.get(&(ch.id, s)) else {
                        continue;
                    };
                    let t = text.trim();
                    if t.is_empty() {
                        continue;
                    }
                    let v = parse_measure(t).ok_or_else(|| {
                        format!("{} piece {s}: \"{t}\" is not a measurement.", ch.code)
                    })?;
                    if self.saved(ch.id, s).and_then(|r| r.measured_value) == Some(v) {
                        continue;
                    }
                    out.push(ResultInput {
                        characteristic_id: ch.id,
                        sample_index: s,
                        measured_value: Some(v),
                        attribute_result: None,
                        gauge_id: self.gauge,
                        notes: String::new(),
                    });
                }
            } else if let Some(a) = self.attrs.get(&ch.id) {
                if self.saved(ch.id, 1).and_then(|r| r.attribute_result.as_deref()) == Some(a) {
                    continue;
                }
                out.push(ResultInput {
                    characteristic_id: ch.id,
                    sample_index: 1,
                    measured_value: None,
                    attribute_result: Some(a.clone()),
                    gauge_id: None,
                    notes: String::new(),
                });
            }
        }
        Ok(out)
    }

    fn covered(&self, ch: &crate::api::Characteristic) -> bool {
        if ch.is_variable() {
            (1..=self.samples()).all(|s| self.saved(ch.id, s).is_some())
        } else {
            self.saved(ch.id, 1).is_some()
        }
    }

    fn finish_body(&self) -> Result<CompleteInspection, String> {
        let inspected = parse_measure(&self.qty_inspected)
            .filter(|v| *v >= 0.0)
            .ok_or("Quantity checked must be a number.")?;
        let rejected = parse_measure(&self.qty_rejected)
            .filter(|v| *v >= 0.0)
            .ok_or("Quantity rejected must be a number.")?;
        if rejected > inspected {
            return Err("More rejected than checked.".into());
        }
        Ok(CompleteInspection {
            quantity_inspected: Some(inspected),
            quantity_accepted: Some(inspected - rejected),
            quantity_rejected: Some(rejected),
            notes: self.notes.trim().to_string(),
            open_ncr: self.open_ncr,
        })
    }
}

impl QcApp {
    pub(super) fn role_at_least(&self, role: &str) -> bool {
        self.session
            .as_ref()
            .is_some_and(|s| role_rank(&s.user.role) >= role_rank(role))
    }

    pub(super) fn spawn_mutation(
        &mut self,
        name: &str,
        work: impl FnOnce(ApiClient) -> Result<MutatePayload, ApiError> + Send + 'static,
    ) {
        if self.mutate_promise.is_some() {
            return;
        }
        let client = self.authed_client();
        self.action_error = None;
        self.mutate_promise = Some(poll_promise::Promise::spawn_thread(name, move || work(client)));
    }

    fn product_label(&self, id: Uuid) -> String {
        self.products
            .iter()
            .find(|p| p.id == id)
            .map(|p| format!("{} {}", p.part_number, p.name))
            .unwrap_or_default()
    }

    fn back_row(&mut self, ui: &mut egui::Ui, label: &str) -> bool {
        let mut back = false;
        let target = match self.detail {
            DetailView::Inspection(id) => Some(("inspections", "inspection", id)),
            DetailView::Ncr(id) => Some(("nonconformances", "alert", id)),
            DetailView::Capa(id) => Some(("corrective-actions", "corrective action", id)),
            DetailView::None => None,
        }
        .filter(|_| self.role_at_least("admin"));
        let busy = self.mutate_promise.is_some();
        let mut delete = false;
        ui.horizontal(|ui| {
            if quiet_button(ui, label).clicked() {
                back = true;
            }
            let Some((_, what, _)) = target else {
                return;
            };
            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                if self.draft.confirm == Some("delete") {
                    if chrome::button(ui, "Keep it", Btn::Quiet).clicked() {
                        self.draft.confirm = None;
                    }
                    if ui.add_enabled_ui(!busy, |ui| chrome::button(ui, "Yes, delete", Btn::Danger).clicked()).inner {
                        delete = true;
                    }
                    chrome::muted(ui, format!("Delete this {what} and everything recorded on it? This can't be undone."));
                } else if chrome::button(ui, "Delete", Btn::Quiet).on_hover_text("Admin only").clicked() {
                    self.draft.confirm = Some("delete");
                }
            });
        });
        ui.add_space(12.0);
        if let (true, Some((path, what, id))) = (delete, target) {
            self.draft.confirm = None;
            let msg = format!("Deleted the {what}.");
            self.spawn_mutation("delete-record", move |c| {
                c.delete_path(&format!("/api/v1/{path}/{id}"))?;
                Ok(MutatePayload::Deleted(msg))
            });
        }
        if back {
            self.detail = DetailView::None;
            self.workbench = None;
            self.ncr_detail = None;
            self.capa_detail = None;
            self.action_error = None;
            self.draft = ActionDraft::default();
            self.refresh_current();
        }
        back
    }

    pub(super) fn ui_inspection_detail(&mut self, ui: &mut egui::Ui) {
        if self.back_row(ui, "← All inspections") {
            return;
        }
        let Some(mut wb) = self.workbench.take() else {
            if self.detail_promise.is_some() {
                loading_spinner(ui);
            } else {
                empty_state(ui, "This inspection did not load.", "Go back and open it again.");
            }
            return;
        };
        let busy = self.mutate_promise.is_some();
        let can_record = self.role_at_least("operator");
        let can_inspect = self.role_at_least("inspector");
        let sup = self.role_at_least("supervisor");
        let product = wb
            .lot
            .as_ref()
            .map(|l| self.product_label(l.product_id))
            .unwrap_or_default();
        let linked_ncr = self
            .ncrs
            .iter()
            .find(|n| n.inspection_id == Some(wb.detail.inspection.id))
            .map(|n| (n.id, n.ncr_number.clone()));
        let action_error = self.action_error.clone();
        let mut action: Option<WorkAction> = None;
        let mut local_error: Option<String> = None;
        let p = chrome::palette(ui);

        egui::ScrollArea::vertical()
            .id_salt("workbench")
            .auto_shrink([false, false])
            .show(ui, |ui| {
                let insp = wb.detail.inspection.clone();
                ui.horizontal(|ui| {
                    ui.label(chrome::semi(&insp.inspection_number, 22.0, p.ink));
                    ui.add_space(6.0);
                    chrome::pill(ui, &insp.status);
                    if !insp.result.is_empty() && insp.result != "pending" {
                        chrome::pill(ui, &insp.result);
                    }
                    if wb.in_progress() && can_inspect {
                        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                            if wb.confirm_cancel {
                                if chrome::button(ui, "Keep it", Btn::Quiet).clicked() {
                                    wb.confirm_cancel = false;
                                }
                                if chrome::button(ui, "Yes, cancel", Btn::Danger).clicked() {
                                    action = Some(WorkAction::Cancel);
                                }
                                chrome::muted(ui, "Cancel this check? Readings stay on record.");
                            } else if chrome::button(ui, "Cancel inspection", Btn::Danger).clicked() {
                                wb.confirm_cancel = true;
                            }
                        });
                    }
                });
                let mut meta = Vec::new();
                if let Some(lot) = &wb.lot {
                    meta.push(format!("Lot {}", lot.lot_number));
                    meta.push(format!("{} {}", fmt_num(lot.quantity), lot.unit));
                }
                if !product.is_empty() {
                    meta.push(product.clone());
                }
                if let Some(plan) = &wb.plan {
                    meta.push(format!("{} rev {}", plan.name, plan.revision));
                }
                meta.push(format!("started {}", insp.started_at.with_timezone(&chrono::Local).format("%b %-d, %H:%M")));
                chrome::muted(ui, meta.join("  ·  "));

                let chars = wb.plan.as_ref().map(|p| p.characteristics.clone()).unwrap_or_default();
                let total = chars.len();
                let done = chars.iter().filter(|c| wb.covered(c)).count();
                ui.add_space(10.0);
                if total > 0 {
                    ui.horizontal(|ui| {
                        ui.label(chrome::semi(format!("{done} of {total} checks recorded"), 13.0, p.ink));
                        ui.add(
                            egui::ProgressBar::new(done as f32 / total as f32)
                                .desired_width(220.0)
                                .desired_height(8.0)
                                .fill(if done == total { p.teal } else { p.blue }),
                        );
                    });
                }
                if let Some(err) = action_error.as_deref() {
                    ui.add_space(8.0);
                    error_banner(ui, err);
                }
                ui.add_space(14.0);

                let rail = 340.0;
                let gap = 16.0;
                let main_w = (ui.available_width() - rail - gap).max(420.0);
                ui.horizontal_top(|ui| {
                    ui.allocate_ui_with_layout(
                        egui::vec2(main_w, 0.0),
                        egui::Layout::top_down(egui::Align::LEFT),
                        |ui| {
                            self.draw_checks(ui, &mut wb, &chars, can_record, busy, &mut action, &mut local_error);
                        },
                    );
                    ui.add_space(gap);
                    ui.allocate_ui_with_layout(
                        egui::vec2(rail, 0.0),
                        egui::Layout::top_down(egui::Align::LEFT),
                        |ui| {
                            self.draw_defects(ui, &mut wb, can_inspect, busy, &mut action, &mut local_error);
                            ui.add_space(12.0);
                            draw_finish(ui, &mut wb, can_inspect, busy, linked_ncr.clone(), &mut action, &mut local_error);
                            ui.add_space(12.0);
                            let may_edit = can_inspect && (wb.in_progress() || sup);
                            draw_header_notes(ui, &mut wb, may_edit, busy, &mut action);
                        },
                    );
                });
            });

        let id = wb.detail.inspection.id;
        self.workbench = Some(wb);
        if let Some(e) = local_error {
            self.action_error = Some(e);
        }
        match action {
            Some(WorkAction::Save(rows)) => {
                let n = rows.len();
                self.spawn_mutation("save-readings", move |c| {
                    c.record_results(id, &ResultBatch { results: rows })?;
                    Ok(MutatePayload::Reload(format!(
                        "Saved {n} reading{}.",
                        if n == 1 { "" } else { "s" }
                    )))
                });
            }
            Some(WorkAction::AddDefect(body)) => {
                self.spawn_mutation("add-defect", move |c| {
                    c.add_defect(id, &body)?;
                    Ok(MutatePayload::Reload("Defect added.".into()))
                });
            }
            Some(WorkAction::RemoveDefect(defect_id)) => {
                self.spawn_mutation("remove-defect", move |c| {
                    c.delete_path(&format!("/api/v1/inspection-defects/{defect_id}"))?;
                    Ok(MutatePayload::Reload("Defect removed.".into()))
                });
            }
            Some(WorkAction::Complete(body)) => {
                self.spawn_mutation("complete", move |c| {
                    Ok(MutatePayload::Completed(Box::new(c.complete_inspection(id, &body)?)))
                });
            }
            Some(WorkAction::Cancel) => {
                self.spawn_mutation("cancel-insp", move |c| {
                    c.cancel_inspection(id)?;
                    Ok(MutatePayload::Reload("Inspection cancelled.".into()))
                });
            }
            Some(WorkAction::OpenNcr(ncr)) => self.open_ncr(ncr),
            Some(WorkAction::SaveNotes(notes)) => {
                self.spawn_mutation("insp-notes", move |c| {
                    c.patch_json(&format!("/api/v1/inspections/{id}"), &serde_json::json!({ "notes": notes }))?;
                    Ok(MutatePayload::Reload("Notes saved.".into()))
                });
            }
            None => {}
        }
    }

    #[allow(clippy::too_many_arguments)]
    fn draw_checks(
        &self,
        ui: &mut egui::Ui,
        wb: &mut Workbench,
        chars: &[crate::api::Characteristic],
        can_record: bool,
        busy: bool,
        action: &mut Option<WorkAction>,
        local_error: &mut Option<String>,
    ) {
        let p = chrome::palette(ui);
        let editable = wb.in_progress() && can_record;
        let dim_out = self.defect_codes.iter().find(|d| d.code == "DIM-OUT" && d.active).map(|d| d.id);
        chrome::card(ui, "Checks", |ui| {
            if wb.plan.is_none() {
                chrome::muted(ui, "The plan for this inspection did not load.");
                return;
            }
            if chars.is_empty() {
                chrome::muted(ui, "This plan has no checks. Add them on the Plans page.");
                return;
            }
            let samples = wb.samples();
            let info_w = 210.0;
            let mark_w = 48.0;
            for (i, ch) in chars.iter().enumerate() {
                if i > 0 {
                    ui.add_space(6.0);
                    ui.separator();
                    ui.add_space(6.0);
                }
                ui.horizontal_top(|ui| {
                        ui.allocate_ui_with_layout(
                            egui::vec2(info_w, 0.0),
                            egui::Layout::top_down(egui::Align::LEFT),
                            |ui| {
                                ui.set_width(info_w);
                                ui.horizontal(|ui| {
                                    ui.label(chrome::semi(&ch.code, 13.5, p.ink));
                                    if ch.critical {
                                        chrome::pill(ui, "critical");
                                    }
                                });
                                ui.label(egui::RichText::new(&ch.name).color(p.ink).size(13.0));
                                ui.label(egui::RichText::new(spec_text(ch)).color(p.muted).size(12.0));
                            },
                        );
                        let entry_w = (ui.available_width() - mark_w - 8.0).max(160.0);
                        ui.allocate_ui_with_layout(
                            egui::vec2(entry_w, 0.0),
                            egui::Layout::left_to_right(egui::Align::Center).with_main_wrap(true),
                            |ui| {
                            ui.set_width(entry_w);
                            ui.add_enabled_ui(editable, |ui| {
                                if ch.is_variable() {
                                    for s in 1..=samples {
                                        let saved = wb.saved(ch.id, s).map(|r| r.in_spec);
                                        let text = wb.values.entry((ch.id, s)).or_default();
                                        let color = match saved {
                                            Some(false) => p.copper,
                                            _ => p.ink,
                                        };
                                        ui.add(
                                            egui::TextEdit::singleline(text)
                                                .desired_width(72.0)
                                                .hint_text(if samples > 1 { format!("#{s}") } else { "value".into() })
                                                .text_color(color)
                                                .margin(egui::Margin::symmetric(8.0, 6.0)),
                                        );
                                    }
                                } else {
                                    let cur = wb.attrs.get(&ch.id).cloned().unwrap_or_default();
                                    if chrome::choice(ui, "Pass", cur == "pass", true).clicked() {
                                        wb.attrs.insert(ch.id, "pass".into());
                                    }
                                    if chrome::choice(ui, "Fail", cur == "fail", false).clicked() {
                                        wb.attrs.insert(ch.id, "fail".into());
                                    }
                                }
                            });
                            if editable && ch.is_variable() && !ch.has_limits() {
                                if let Some(dim) = dim_out {
                                    if quiet_button(ui, "Out of print").on_hover_text(
                                        "Adds a DIM-OUT defect, so this build fails when you finish.",
                                    ).clicked() {
                                        *action = Some(WorkAction::AddDefect(NewInspectionDefect {
                                            defect_code_id: dim,
                                            quantity: 1,
                                            notes: format!("{} {} is off the print.", ch.code, ch.name),
                                        }));
                                    }
                                }
                            }
                        });
                        let (mark, color) = if wb.covered(ch) {
                            let bad = wb
                                .detail
                                .results
                                .iter()
                                .any(|r| r.characteristic_id == ch.id && !r.in_spec);
                            if bad { ("Out", p.copper) } else { ("Done", p.teal) }
                        } else {
                            ("Open", p.muted)
                        };
                        ui.label(chrome::semi(mark, 12.0, color));
                });
            }

            if !editable {
                return;
            }
            ui.add_space(14.0);
            ui.separator();
            ui.add_space(10.0);
            let pending = wb.pending();
            ui.horizontal(|ui| {
                ui.label(chrome::semi("Gauge", 12.0, p.muted));
                let gauge_label = wb
                    .gauge
                    .and_then(|g| self.gauges.iter().find(|x| x.id == g))
                    .map(|g| format!("{} {}", g.code, g.name))
                    .unwrap_or_else(|| "None".into());
                egui::ComboBox::from_id_salt("wb-gauge")
                    .width(200.0)
                    .selected_text(gauge_label)
                    .show_ui(ui, |ui| {
                        ui.selectable_value(&mut wb.gauge, None, "None");
                        for g in self.gauges.iter().filter(|g| g.active) {
                            ui.selectable_value(&mut wb.gauge, Some(g.id), format!("{} {}", g.code, g.name));
                        }
                    });
                ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                    let label = match &pending {
                        Ok(rows) if !rows.is_empty() => format!("Save {} reading{}", rows.len(), if rows.len() == 1 { "" } else { "s" }),
                        _ => "Save readings".into(),
                    };
                    let ready = matches!(&pending, Ok(rows) if !rows.is_empty()) && !busy;
                    let clicked = ui
                        .add_enabled_ui(ready, |ui| chrome::button(ui, &label, Btn::Primary).clicked())
                        .inner;
                    if clicked {
                        match wb.pending() {
                            Ok(rows) => *action = Some(WorkAction::Save(rows)),
                            Err(e) => *local_error = Some(e),
                        }
                    }
                });
            });
            ui.add_space(6.0);
            match pending {
                Err(e) => error_banner(ui, &e),
                Ok(_) => chrome::muted(ui, "Inches can be typed as 36.5, 36 1/2, or 36-1/2."),
            }
        });
    }

    fn draw_defects(
        &self,
        ui: &mut egui::Ui,
        wb: &mut Workbench,
        can_inspect: bool,
        busy: bool,
        action: &mut Option<WorkAction>,
        local_error: &mut Option<String>,
    ) {
        let p = chrome::palette(ui);
        let editable = wb.in_progress() && can_inspect;
        chrome::card(ui, "Defects", |ui| {
            if wb.detail.defects.is_empty() {
                chrome::muted(ui, "None found yet.");
            }
            for d in wb.detail.defects.clone() {
                ui.horizontal(|ui| {
                    chrome::pill(ui, &d.severity);
                    ui.label(chrome::semi(format!("{} × {}", d.code, d.quantity), 13.0, p.ink));
                    if editable {
                        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                            if ui.add(egui::Label::new(egui::RichText::new("Remove").color(p.muted).size(12.0)).sense(egui::Sense::click())).clicked() && !busy {
                                *action = Some(WorkAction::RemoveDefect(d.id));
                            }
                        });
                    }
                });
                let mut line = d.name.clone();
                if !d.notes.is_empty() {
                    line = format!("{line}. {}", d.notes);
                }
                ui.label(egui::RichText::new(line).color(p.muted).size(12.0));
                ui.add_space(6.0);
            }
            if !editable {
                return;
            }
            ui.add_space(6.0);
            ui.separator();
            ui.add_space(8.0);
            chrome::field_label(ui, "Add a defect");
            let picked = wb
                .defect
                .and_then(|id| self.defect_codes.iter().find(|d| d.id == id))
                .map(|d| format!("{} {}", d.code, d.name))
                .unwrap_or_else(|| "Pick a defect".into());
            egui::ComboBox::from_id_salt("wb-defect")
                .width(ui.available_width())
                .selected_text(picked)
                .height(360.0)
                .show_ui(ui, |ui| {
                    for d in self.defect_codes.iter().filter(|d| d.active) {
                        ui.selectable_value(
                            &mut wb.defect,
                            Some(d.id),
                            format!("{}  {}  ({})", d.code, d.name, d.severity),
                        );
                    }
                });
            ui.add_space(6.0);
            ui.horizontal(|ui| {
                ui.label(chrome::semi("How many", 12.0, p.muted));
                ui.add(egui::TextEdit::singleline(&mut wb.defect_qty).desired_width(56.0));
            });
            ui.add_space(6.0);
            ui.add(
                egui::TextEdit::singleline(&mut wb.defect_notes)
                    .hint_text("Where on the build")
                    .desired_width(f32::INFINITY),
            );
            ui.add_space(8.0);
            let ready = wb.defect.is_some() && !busy;
            if ui.add_enabled_ui(ready, |ui| chrome::button_fill(ui, "Add defect", Btn::Quiet).clicked()).inner {
                match wb.defect_qty.trim().parse::<i32>() {
                    Ok(q) if q > 0 => {
                        *action = Some(WorkAction::AddDefect(NewInspectionDefect {
                            defect_code_id: wb.defect.unwrap_or_default(),
                            quantity: q,
                            notes: wb.defect_notes.trim().to_string(),
                        }));
                        wb.defect_notes.clear();
                        wb.defect_qty = "1".into();
                    }
                    _ => *local_error = Some("How many must be a whole number above zero.".into()),
                }
            }
        });
    }

    pub(super) fn open_ncr(&mut self, id: Uuid) {
        self.goto(NavPage::Nonconformances);
        self.detail = DetailView::Ncr(id);
        self.spawn_detail(DetailView::Ncr(id));
    }

    fn open_capa(&mut self, id: Uuid) {
        self.goto(NavPage::CorrectiveActions);
        self.detail = DetailView::Capa(id);
        self.spawn_detail(DetailView::Capa(id));
    }

    pub(super) fn ui_ncr_detail(&mut self, ui: &mut egui::Ui) {
        if self.back_row(ui, "← All alerts") {
            return;
        }
        let Some(d) = self.ncr_detail.clone() else {
            if self.detail_promise.is_some() {
                loading_spinner(ui);
            } else {
                empty_state(ui, "This alert did not load.", "Go back and open it again.");
            }
            return;
        };
        let p = chrome::palette(ui);
        let n = &d.ncr;
        let busy = self.mutate_promise.is_some();
        let can_decide = self.role_at_least("supervisor");
        let mut go_capa: Option<Uuid> = None;
        let mut new_capa = false;
        let mut go_insp: Option<Uuid> = None;
        let mut run: Option<(&'static str, String)> = None;

        egui::ScrollArea::vertical().id_salt("ncr-detail").auto_shrink([false, false]).show(ui, |ui| {
            ui.horizontal(|ui| {
                ui.label(chrome::semi(&n.ncr_number, 22.0, p.ink));
                ui.add_space(6.0);
                chrome::pill(ui, &n.severity);
                chrome::pill(ui, &n.status);
                if n.disposition != "pending" && !n.disposition.is_empty() {
                    chrome::pill(ui, &n.disposition);
                }
            });
            ui.label(chrome::semi(&n.title, 16.0, p.ink));
            let mut meta = vec![self.product_label(n.product_id)];
            if let Some(lot) = n.lot_id.and_then(|id| self.lots.iter().find(|l| l.id == id)) {
                meta.push(format!("Lot {}", lot.lot_number));
            }
            meta.push(format!("{} affected", fmt_num(n.quantity_affected)));
            meta.push(format!("opened {}", n.opened_at.with_timezone(&chrono::Local).format("%b %-d, %Y")));
            chrome::muted(ui, meta.join("  ·  "));
            if let Some(err) = self.action_error.as_deref() {
                ui.add_space(8.0);
                error_banner(ui, err);
            }
            ui.add_space(14.0);

            let rail = 360.0;
            let main_w = (ui.available_width() - rail - 16.0).max(380.0);
            ui.horizontal_top(|ui| {
                ui.allocate_ui_with_layout(egui::vec2(main_w, 0.0), egui::Layout::top_down(egui::Align::LEFT), |ui| {
                    chrome::card(ui, "What happened", |ui| {
                        let text = if n.description.trim().is_empty() { "No description." } else { n.description.as_str() };
                        ui.label(egui::RichText::new(text).color(p.ink).size(14.0));
                        if let Some(insp) = n.inspection_id {
                            ui.add_space(10.0);
                            if quiet_button(ui, "Open the inspection").clicked() {
                                go_insp = Some(insp);
                            }
                        }
                    });
                    if !n.disposition_notes.is_empty() {
                        ui.add_space(12.0);
                        chrome::card(ui, "Decision notes", |ui| {
                            ui.label(egui::RichText::new(&n.disposition_notes).color(p.ink).size(14.0));
                        });
                    }
                    ui.add_space(12.0);
                    chrome::card(ui, "Corrective actions", |ui| {
                        if d.corrective_actions.is_empty() {
                            chrome::muted(ui, "None yet. Open one if this could happen again.");
                        }
                        for c in &d.corrective_actions {
                            ui.horizontal(|ui| {
                                chrome::pill(ui, &c.status);
                                ui.label(chrome::semi(&c.ca_number, 13.0, p.ink));
                                ui.label(egui::RichText::new(&c.title).color(p.muted).size(13.0));
                                ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                                    if quiet_button(ui, "Open").clicked() {
                                        go_capa = Some(c.id);
                                    }
                                });
                            });
                        }
                        if can_decide && n.status != "void" {
                            ui.add_space(10.0);
                            if chrome::button(ui, "Open corrective action", Btn::Quiet).clicked() {
                                new_capa = true;
                            }
                        }
                    });
                });
                ui.add_space(16.0);
                ui.allocate_ui_with_layout(egui::vec2(rail, 0.0), egui::Layout::top_down(egui::Align::LEFT), |ui| {
                    let open = n.status == "open" || n.status == "under_review";
                    chrome::card(ui, "Decide", |ui| {
                        if !can_decide {
                            chrome::muted(ui, "A supervisor decides what happens to the parts.");
                            return;
                        }
                        if n.status == "dispositioned" {
                            chrome::muted(ui, "The decision is in. Close it when the work is done.");
                            ui.add_space(10.0);
                            if ui.add_enabled_ui(!busy, |ui| chrome::button_fill(ui, "Close alert", Btn::Primary).clicked()).inner {
                                run = Some(("close", String::new()));
                            }
                            return;
                        }
                        if !open {
                            chrome::muted(ui, "Nothing left to decide.");
                            return;
                        }
                        for (key, label, help) in DISPOSITIONS {
                            let sel = self.draft.disposition == *key;
                            let r = ui.add(egui::SelectableLabel::new(sel, chrome::semi(*label, 13.5, if sel { p.teal } else { p.ink })));
                            if r.clicked() {
                                self.draft.disposition = (*key).to_string();
                            }
                            ui.label(egui::RichText::new(*help).color(p.muted).size(12.0));
                            ui.add_space(4.0);
                        }
                        ui.add_space(6.0);
                        chrome::field_label(ui, "Notes");
                        ui.add(egui::TextEdit::multiline(&mut self.draft.notes).desired_rows(3).desired_width(f32::INFINITY).hint_text("Who fixes it, and how"));
                        ui.add_space(10.0);
                        let ready = !self.draft.disposition.is_empty() && !busy;
                        if ui.add_enabled_ui(ready, |ui| chrome::button_fill(ui, "Record decision", Btn::Primary).clicked()).inner {
                            run = Some(("dispose", self.draft.notes.clone()));
                        }
                        ui.add_space(8.0);
                        if self.draft.confirm == Some("void") {
                            chrome::muted(ui, "Void means this was never a real problem. The notes above are kept.");
                            ui.horizontal(|ui| {
                                if chrome::button(ui, "Yes, void it", Btn::Danger).clicked() {
                                    run = Some(("void", self.draft.notes.clone()));
                                }
                                if chrome::button(ui, "Keep it", Btn::Quiet).clicked() {
                                    self.draft.confirm = None;
                                }
                            });
                        } else if ui.add_enabled_ui(!busy, |ui| chrome::button_fill(ui, "Void alert", Btn::Danger).clicked()).inner {
                            self.draft.confirm = Some("void");
                        }
                    });
                });
            });
        });

        let id = n.id;
        if let Some((what, notes)) = run {
            let disposition = self.draft.disposition.clone();
            self.spawn_mutation("ncr-move", move |c| {
                let msg = match what {
                    "dispose" => {
                        c.dispose_ncr(id, &DispositionRequest { disposition, notes })?;
                        "Decision recorded."
                    }
                    "close" => {
                        c.close_ncr(id)?;
                        "Alert closed."
                    }
                    _ => {
                        c.void_ncr(id, &notes)?;
                        "Alert voided."
                    }
                };
                Ok(MutatePayload::Reload(msg.into()))
            });
        }
        if new_capa {
            let title = format!("Stop {} from happening again", d.ncr.title.to_lowercase());
            self.goto(NavPage::CorrectiveActions);
            self.capa_form.ncr = Some((id, d.ncr.ncr_number.clone()));
            self.capa_form.title = title.chars().take(200).collect();
            self.capa_form.root_cause.clear();
            self.capa_form.action_plan.clear();
            self.capa_form.error = None;
            self.form = FormMode::CreateCapa;
        }
        if let Some(c) = go_capa {
            self.open_capa(c);
        }
        if let Some(i) = go_insp {
            self.goto(NavPage::Inspections);
            self.detail = DetailView::Inspection(i);
            self.spawn_detail(DetailView::Inspection(i));
        }
    }

    pub(super) fn ui_capa_detail(&mut self, ui: &mut egui::Ui) {
        if self.back_row(ui, "← All actions") {
            return;
        }
        let Some(c) = self.capa_detail.clone() else {
            if self.detail_promise.is_some() {
                loading_spinner(ui);
            } else {
                empty_state(ui, "This action did not load.", "Go back and open it again.");
            }
            return;
        };
        let p = chrome::palette(ui);
        let busy = self.mutate_promise.is_some();
        let sup = self.role_at_least("supervisor");
        let mgr = self.role_at_least("quality_manager");
        let today = chrono::Local::now().date_naive().to_string();
        let overdue = !c.due_date.is_empty()
            && c.due_date.as_str() < today.as_str()
            && !matches!(c.status.as_str(), "closed" | "cancelled");
        let mut run: Option<&'static str> = None;
        let mut go_ncr: Option<Uuid> = None;

        egui::ScrollArea::vertical().id_salt("capa-detail").auto_shrink([false, false]).show(ui, |ui| {
            ui.horizontal(|ui| {
                ui.label(chrome::semi(&c.ca_number, 22.0, p.ink));
                ui.add_space(6.0);
                chrome::pill(ui, &c.status);
                if overdue {
                    chrome::pill(ui, "overdue");
                }
            });
            ui.label(chrome::semi(&c.title, 16.0, p.ink));
            let mut meta = Vec::new();
            if !c.due_date.is_empty() {
                meta.push(format!("due {}", c.due_date));
            }
            meta.push(format!("opened {}", c.opened_at.with_timezone(&chrono::Local).format("%b %-d, %Y")));
            if let Some(v) = c.verified_at {
                meta.push(format!("verified {}", v.with_timezone(&chrono::Local).format("%b %-d")));
            }
            if let Some(v) = c.closed_at {
                meta.push(format!("closed {}", v.with_timezone(&chrono::Local).format("%b %-d")));
            }
            chrome::muted(ui, meta.join("  ·  "));
            if let Some(err) = self.action_error.as_deref() {
                ui.add_space(8.0);
                error_banner(ui, err);
            }
            ui.add_space(14.0);
            let rail = 360.0;
            let main_w = (ui.available_width() - rail - 16.0).max(380.0);
            ui.horizontal_top(|ui| {
                ui.allocate_ui_with_layout(egui::vec2(main_w, 0.0), egui::Layout::top_down(egui::Align::LEFT), |ui| {
                    for (title, body) in [
                        ("Root cause", &c.root_cause),
                        ("Action plan", &c.action_plan),
                        ("Verification", &c.verification_notes),
                        ("Did it work", &c.effectiveness_notes),
                    ] {
                        if body.trim().is_empty() && matches!(title, "Verification" | "Did it work") {
                            continue;
                        }
                        chrome::card(ui, title, |ui| {
                            let text = if body.trim().is_empty() { "Not written yet." } else { body.as_str() };
                            ui.label(egui::RichText::new(text).color(p.ink).size(14.0));
                        });
                        ui.add_space(12.0);
                    }
                    if let Some(ncr) = c.nonconformance_id {
                        if quiet_button(ui, "Open the alert").clicked() {
                            go_ncr = Some(ncr);
                        }
                    }
                });
                ui.add_space(16.0);
                ui.allocate_ui_with_layout(egui::vec2(rail, 0.0), egui::Layout::top_down(egui::Align::LEFT), |ui| {
                    chrome::card(ui, "Next step", |ui| {
                        let (hint, buttons): (&str, &[(&'static str, &str, Btn, bool)]) = match c.status.as_str() {
                            "open" => ("Start when someone owns the fix.", &[
                                ("start", "Start work", Btn::Primary, sup),
                                ("cancel", "Cancel action", Btn::Danger, sup),
                            ]),
                            "in_progress" => ("When the fix is in, say how you checked it.", &[
                                ("verify", "Send to verification", Btn::Primary, sup),
                                ("cancel", "Cancel action", Btn::Danger, sup),
                            ]),
                            "verification" => ("Close it once later builds come out right.", &[
                                ("close", "Close action", Btn::Primary, mgr),
                                ("reopen", "Reopen", Btn::Quiet, sup),
                            ]),
                            _ => ("Nothing left to do here.", &[]),
                        };
                        chrome::muted(ui, hint);
                        if buttons.is_empty() {
                            return;
                        }
                        if !buttons.iter().any(|b| b.3) {
                            ui.add_space(6.0);
                            chrome::muted(ui, "Your role can't move this one.");
                            return;
                        }
                        ui.add_space(10.0);
                        let needs_notes = matches!(c.status.as_str(), "in_progress" | "verification");
                        chrome::field_label(ui, match c.status.as_str() {
                            "in_progress" => "How was the fix checked (required)",
                            "verification" => "Did it work (required to close)",
                            _ => "Notes",
                        });
                        ui.add(egui::TextEdit::multiline(&mut self.draft.notes).desired_rows(3).desired_width(f32::INFINITY));
                        ui.add_space(10.0);
                        for (key, label, kind, allowed) in buttons {
                            let note_ok = !needs_notes || !self.draft.notes.trim().is_empty() || matches!(*key, "cancel" | "reopen");
                            let ready = *allowed && note_ok && !busy;
                            let r = ui.add_enabled_ui(ready, |ui| chrome::button_fill(ui, label, *kind)).inner;
                            let r = if !allowed { r.on_disabled_hover_text("Your role can't do this.") } else { r };
                            if r.clicked() {
                                run = Some(key);
                            }
                            ui.add_space(6.0);
                        }
                    });
                });
            });
        });

        let id = c.id;
        if let Some(step) = run {
            let notes = self.draft.notes.trim().to_string();
            self.spawn_mutation("capa-move", move |cl| {
                cl.move_capa(id, step, &notes)?;
                let msg = match step {
                    "start" => "Work started.",
                    "verify" => "Sent to verification.",
                    "close" => "Corrective action closed.",
                    "reopen" => "Reopened.",
                    _ => "Corrective action cancelled.",
                };
                Ok(MutatePayload::Reload(msg.into()))
            });
        }
        if let Some(n) = go_ncr {
            self.open_ncr(n);
        }
    }
}

fn draw_header_notes(ui: &mut egui::Ui, wb: &mut Workbench, may_edit: bool, busy: bool, action: &mut Option<WorkAction>) {
    chrome::card(ui, "Notes", |ui| {
        ui.add_enabled(
            may_edit,
            egui::TextEdit::multiline(&mut wb.header_notes)
                .desired_rows(3)
                .desired_width(f32::INFINITY)
                .hint_text("Customer, options, or anything the next person should know")
                .margin(egui::Margin::symmetric(10.0, 8.0)),
        );
        if !may_edit && !wb.in_progress() {
            ui.add_space(4.0);
            chrome::muted(ui, "A supervisor edits notes on a finished inspection.");
        }
        let changed = wb.header_notes != wb.detail.inspection.notes;
        if may_edit && changed {
            ui.add_space(6.0);
            if ui.add_enabled_ui(!busy, |ui| chrome::button(ui, "Save notes", Btn::Quiet).clicked()).inner {
                *action = Some(WorkAction::SaveNotes(wb.header_notes.trim().to_string()));
            }
        }
    });
}

fn draw_finish(
    ui: &mut egui::Ui,
    wb: &mut Workbench,
    can_inspect: bool,
    busy: bool,
    linked_ncr: Option<(Uuid, String)>,
    action: &mut Option<WorkAction>,
    local_error: &mut Option<String>,
) {
    let p = chrome::palette(ui);
    let insp = wb.detail.inspection.clone();
    if !wb.in_progress() {
        chrome::card(ui, "Outcome", |ui| {
            ui.horizontal(|ui| {
                chrome::pill(ui, &insp.status);
                if !insp.result.is_empty() {
                    chrome::pill(ui, &insp.result);
                }
            });
            ui.add_space(8.0);
            ui.label(chrome::semi(
                format!(
                    "{} checked · {} good · {} rejected",
                    fmt_num(insp.quantity_inspected),
                    fmt_num(insp.quantity_accepted),
                    fmt_num(insp.quantity_rejected)
                ),
                13.0,
                p.ink,
            ));
            if let Some(at) = insp.completed_at {
                chrome::muted(ui, format!("Finished {}", at.with_timezone(&chrono::Local).format("%b %-d, %H:%M")));
            }
            if !insp.notes.is_empty() {
                ui.add_space(6.0);
                ui.label(egui::RichText::new(&insp.notes).color(p.ink).size(13.0));
            }
            if let Some((id, number)) = linked_ncr {
                ui.add_space(10.0);
                if chrome::button_fill(ui, &format!("Open alert {number}"), Btn::Quiet).clicked() {
                    *action = Some(WorkAction::OpenNcr(id));
                }
            }
        });
        return;
    }
    chrome::card(ui, "Finish", |ui| {
        if !can_inspect {
            chrome::muted(ui, "An inspector signs off the build.");
            return;
        }
        ui.horizontal(|ui| {
            ui.vertical(|ui| {
                chrome::field_label(ui, "Checked");
                ui.add(egui::TextEdit::singleline(&mut wb.qty_inspected).desired_width(90.0));
            });
            ui.vertical(|ui| {
                chrome::field_label(ui, "Rejected");
                ui.add(egui::TextEdit::singleline(&mut wb.qty_rejected).desired_width(90.0));
            });
            ui.vertical(|ui| {
                chrome::field_label(ui, "Good");
                let good = match (parse_measure(&wb.qty_inspected), parse_measure(&wb.qty_rejected)) {
                    (Some(a), Some(b)) if b <= a => fmt_num(a - b),
                    _ => "—".into(),
                };
                ui.add_space(4.0);
                ui.label(chrome::semi(good, 15.0, p.ink));
            });
        });
        ui.add_space(8.0);
        chrome::field_label(ui, "Sign-off notes");
        ui.add(egui::TextEdit::multiline(&mut wb.notes).desired_rows(2).desired_width(f32::INFINITY));
        ui.add_space(6.0);
        ui.checkbox(&mut wb.open_ncr, "Open an alert if it fails");
        ui.add_space(10.0);
        let missing = &wb.detail.missing_characteristics;
        let unsaved = wb.pending().map(|r| !r.is_empty()).unwrap_or(true);
        let ready = missing.is_empty() && !unsaved && !busy;
        if ui.add_enabled_ui(ready, |ui| chrome::button_fill(ui, "Finish inspection", Btn::Primary).clicked()).inner {
            match wb.finish_body() {
                Ok(body) => *action = Some(WorkAction::Complete(body)),
                Err(e) => *local_error = Some(e),
            }
        }
        ui.add_space(6.0);
        if unsaved {
            chrome::muted(ui, "Save the readings first.");
        } else if !missing.is_empty() {
            chrome::muted(ui, format!("Still open: {}", missing.join(", ")));
        } else {
            chrome::muted(ui, "The server decides pass, fail, or conditional from the readings and defects.");
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reads_tape_fractions() {
        assert_eq!(parse_measure("36.5"), Some(36.5));
        assert_eq!(parse_measure("36 1/2"), Some(36.5));
        assert_eq!(parse_measure("36-1/2"), Some(36.5));
        assert_eq!(parse_measure("36 3/8\""), Some(36.375));
        assert_eq!(parse_measure("3/4"), Some(0.75));
        assert_eq!(parse_measure("-2"), Some(-2.0));
        assert_eq!(parse_measure("-2 1/2"), Some(-2.5));
        assert_eq!(parse_measure("abc"), None);
        assert_eq!(parse_measure("1/0"), None);
        assert_eq!(parse_measure(""), None);
    }

    #[test]
    fn ranks_roles() {
        assert!(role_rank("admin") > role_rank("quality_manager"));
        assert!(role_rank("supervisor") > role_rank("inspector"));
        assert_eq!(role_rank("viewer"), 0);
    }

    #[test]
    fn formats_numbers_plainly() {
        assert_eq!(fmt_num(36.0), "36");
        assert_eq!(fmt_num(36.375), "36.375");
    }
}
