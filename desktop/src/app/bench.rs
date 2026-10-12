//! Floor home. A register of lots plus the work that needs a person.
//! An empty floor is still a finished screen: metrics, a clear next step, and the alert rail.

use crate::api::{Dashboard, Lot, Nonconformance};
use eframe::egui::{self, Color32, FontId, Rect, Rounding, Sense, Stroke, Vec2};

use super::chrome::{self, Btn};

pub enum BenchAction {
    Inspect(uuid::Uuid),
    OpenNcr(uuid::Uuid),
    Alerts,
    NewLot,
    NewCheck,
}

pub fn draw_today(
    ui: &mut egui::Ui,
    dash: &Dashboard,
    lots: &[Lot],
    ncrs: &[Nonconformance],
) -> Option<BenchAction> {
    let p = chrome::palette(ui);
    let mut action = None;

    ui.label(
        egui::RichText::new(chrono::Local::now().format("%A, %B %d").to_string())
            .color(p.muted)
            .size(13.0),
    );
    ui.add_space(12.0);

    let gap = 12.0;
    ui.spacing_mut().item_spacing.x = gap;
    let width = ((ui.available_width() - gap * 3.0) / 4.0).max(140.0);
    let pass = dash
        .pass_rate_last_30_days
        .map(|r| format!("{:.0}%", r * 100.0))
        .unwrap_or_else(|| "—".into());
    ui.horizontal(|ui| {
        ui.spacing_mut().item_spacing.x = gap;
        for (label, value, accent) in [
            ("Open checks", dash.open_inspections.to_string(), p.teal),
            ("Lots on hold", dash.lots_on_hold.to_string(), p.amber),
            ("Open alerts", dash.open_nonconformances.to_string(), p.copper),
            ("Pass, 30 days", pass, p.ink),
        ] {
            ui.allocate_ui(Vec2::new(width, 108.0), |ui| {
                chrome::metric(ui, label, &value, accent);
            });
        }
    });

    ui.add_space(8.0);
    ui.label(
        egui::RichText::new(format!(
            "Last 30 days  ·  {} checks  ·  {} pass  ·  {} fail  ·  {} conditional  ·  {} overdue actions",
            dash.inspections_last_30_days,
            dash.pass_last_30_days,
            dash.fail_last_30_days,
            dash.conditional_last_30_days,
            dash.overdue_corrective_actions
        ))
        .color(p.muted)
        .size(13.0),
    );
    ui.add_space(14.0);

    let body_h = ui.available_height().max(360.0);
    let right_w = 320.0_f32.min(ui.available_width() * 0.32);
    let left_w = (ui.available_width() - right_w - gap).max(280.0);
    ui.spacing_mut().item_spacing.x = gap;
    ui.horizontal(|ui| {
        ui.spacing_mut().item_spacing.x = gap;
        ui.allocate_ui_with_layout(
            Vec2::new(left_w, body_h),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                if let Some(id) = lots_panel(ui, dash, lots, body_h) {
                    action = Some(BenchAction::Inspect(id));
                }
            },
        );
        ui.allocate_ui_with_layout(
            Vec2::new(right_w, body_h),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                let rail = rail(ui, dash, ncrs, body_h);
                if action.is_none() {
                    action = rail;
                }
            },
        );
    });
    action
}

fn lots_panel(ui: &mut egui::Ui, dash: &Dashboard, lots: &[Lot], height: f32) -> Option<uuid::Uuid> {
    let mut opened = None;
    chrome::surface(ui, "lots-panel", height, |ui| {
        let p = chrome::palette(ui);
        ui.horizontal(|ui| {
            ui.label(chrome::semi("Lots", 16.0, p.ink));
            ui.label(egui::RichText::new(lots.len().to_string()).color(p.muted));
            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                ui.label(
                    egui::RichText::new(format!("{} on hold", dash.lots_on_hold))
                        .color(p.muted)
                        .size(12.5),
                );
            });
        });
        ui.add_space(6.0);
        if !lots.is_empty() {
            ui.label(
                egui::RichText::new("Click a lot to start a check.")
                    .color(p.muted)
                    .size(12.5),
            );
            ui.add_space(10.0);
        }

        if lots.is_empty() {
            ui.add_space((ui.available_height() * 0.22).max(8.0));
            ui.label(chrome::semi("Nothing is on the floor", 20.0, p.ink));
            ui.add_space(6.0);
            ui.label(
                egui::RichText::new(
                    "Open a build: a squeeze chute, a corral gate, a panel, a wood stove, or a fireplace.",
                )
                .color(p.muted)
                .size(14.0),
            );
            return;
        }

        let groups: [(&str, &[&str]); 4] = [
            ("Waiting", &["open"]),
            ("On the bench", &["in_inspection"]),
            ("Hold", &["on_hold"]),
            ("Released", &["accepted", "rejected", "closed"]),
        ];
        egui::ScrollArea::vertical()
            .id_salt("lot-scroll")
            .show(ui, |ui| {
                for (title, statuses) in groups {
                    let rows: Vec<&Lot> = lots
                        .iter()
                        .filter(|lot| statuses.contains(&lot.status.as_str()))
                        .collect();
                    if rows.is_empty() {
                        continue;
                    }
                    ui.add_space(8.0);
                    ui.label(chrome::semi(
                        format!("{title}  {}", rows.len()),
                        12.0,
                        p.muted,
                    ));
                    ui.add_space(6.0);
                    for lot in rows {
                        if lot_row(ui, lot).clicked() {
                            opened = Some(lot.id);
                        }
                        ui.add_space(6.0);
                    }
                }
                let known = ["open", "in_inspection", "on_hold", "accepted", "rejected", "closed"];
                let other: Vec<&Lot> = lots
                    .iter()
                    .filter(|lot| !known.contains(&lot.status.as_str()))
                    .collect();
                if !other.is_empty() {
                    ui.add_space(8.0);
                    ui.label(chrome::semi("Other", 12.0, p.muted));
                    ui.add_space(6.0);
                    for lot in other {
                        if lot_row(ui, lot).clicked() {
                            opened = Some(lot.id);
                        }
                        ui.add_space(6.0);
                    }
                }
            });
    });
    opened
}

fn rail(
    ui: &mut egui::Ui,
    dash: &Dashboard,
    ncrs: &[Nonconformance],
    height: f32,
) -> Option<BenchAction> {
    let mut action = None;
    let gap = 12.0;
    let work_h = 236.0;
    let alert_h = (height - work_h - gap).max(160.0);
    chrome::surface(ui, "alert-rail", alert_h, |ui| {
        let p = chrome::palette(ui);
        ui.horizontal(|ui| {
            ui.label(chrome::semi("Needs a person", 16.0, p.ink));
            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                chrome::pill(ui, &dash.open_nonconformances.to_string());
            });
        });
        ui.add_space(4.0);
        ui.label(
            egui::RichText::new(format!(
                "{} overdue actions",
                dash.overdue_corrective_actions
            ))
            .color(p.muted)
            .size(12.5),
        );
        ui.add_space(10.0);
        let open: Vec<&Nonconformance> = ncrs
            .iter()
            .filter(|n| matches!(n.status.as_str(), "open" | "under_review" | "dispositioned"))
            .collect();
        if open.is_empty() && dash.defects_by_severity.is_empty() {
            ui.add_space(18.0);
            ui.label(chrome::semi("The floor is quiet", 16.0, p.ink));
            ui.add_space(4.0);
            ui.label(
                egui::RichText::new("Open alerts and defect counts show up here.")
                    .color(p.muted)
                    .size(13.0),
            );
        }
        egui::ScrollArea::vertical()
            .id_salt("alert-scroll")
            .show(ui, |ui| {
                for ncr in open.into_iter().take(8) {
                    if ncr_row(ui, ncr).clicked() {
                        action = Some(BenchAction::OpenNcr(ncr.id));
                    }
                    ui.add_space(6.0);
                }
                if !dash.defects_by_severity.is_empty() {
                    ui.add_space(8.0);
                    ui.label(chrome::semi("Defects by severity", 12.0, p.muted));
                    ui.add_space(6.0);
                    for item in &dash.defects_by_severity {
                        ui.horizontal(|ui| {
                            chrome::pill(ui, &item.severity);
                            ui.label(chrome::semi(item.count.to_string(), 14.0, p.ink));
                        });
                    }
                }
            });
    });
    ui.add_space(gap);
    chrome::surface(ui, "work-rail", work_h, |ui| {
        let p = chrome::palette(ui);
        ui.label(chrome::semi("Start work", 16.0, p.ink));
        ui.add_space(4.0);
        ui.label(
            egui::RichText::new("The next chute, gate, panel, stove, or fireplace.")
                .color(p.muted)
                .size(12.5),
        );
        ui.add_space(12.0);
        if chrome::button_fill(ui, "New lot", Btn::Primary).clicked() {
            action = Some(BenchAction::NewLot);
        }
        ui.add_space(8.0);
        if chrome::button_fill(ui, "New inspection", Btn::Quiet).clicked() {
            action = Some(BenchAction::NewCheck);
        }
        ui.add_space(8.0);
        if chrome::button_fill(ui, "Open alerts", Btn::Quiet).clicked() {
            action = Some(BenchAction::Alerts);
        }
    });
    action
}

fn lot_row(ui: &mut egui::Ui, lot: &Lot) -> egui::Response {
    let p = chrome::palette(ui);
    let (rect, response) =
        ui.allocate_exact_size(Vec2::new(ui.available_width(), 58.0), Sense::click());
    let fill = if response.hovered() { p.hover } else { p.canvas };
    ui.painter()
        .rect(rect, Rounding::same(10.0), fill, Stroke::new(1.0_f32, p.line));
    ui.painter().rect_filled(
        Rect::from_min_size(rect.min + Vec2::new(0.0, 10.0), Vec2::new(3.0, rect.height() - 20.0)),
        Rounding::same(2.0),
        mark(&p, &lot.status),
    );
    ui.painter().text(
        rect.min + Vec2::new(16.0, 10.0),
        egui::Align2::LEFT_TOP,
        &lot.lot_number,
        chrome::semibold_id(14.5),
        p.ink,
    );
    ui.painter().text(
        rect.min + Vec2::new(16.0, 32.0),
        egui::Align2::LEFT_TOP,
        format!(
            "{} {}  ·  {}",
            qty(lot.quantity),
            lot.unit,
            lot.source.replace('_', " ")
        ),
        FontId::proportional(12.0),
        p.muted,
    );
    let status = lot.status.replace('_', " ");
    ui.painter().text(
        rect.right_center() + Vec2::new(-14.0, 0.0),
        egui::Align2::RIGHT_CENTER,
        status,
        chrome::semibold_id(12.0),
        mark(&p, &lot.status),
    );
    response.on_hover_text("Start an inspection for this lot")
}

fn ncr_row(ui: &mut egui::Ui, ncr: &Nonconformance) -> egui::Response {
    let p = chrome::palette(ui);
    let (rect, response) =
        ui.allocate_exact_size(Vec2::new(ui.available_width(), 58.0), Sense::click());
    let fill = if response.hovered() {
        p.copper_soft
    } else {
        p.canvas
    };
    ui.painter()
        .rect(rect, Rounding::same(10.0), fill, Stroke::new(1.0_f32, p.line));
    ui.painter().text(
        rect.min + Vec2::new(12.0, 8.0),
        egui::Align2::LEFT_TOP,
        &ncr.ncr_number,
        chrome::semibold_id(12.5),
        p.copper,
    );
    ui.painter().text(
        rect.min + Vec2::new(12.0, 30.0),
        egui::Align2::LEFT_TOP,
        truncate(&ncr.title, 32),
        FontId::proportional(13.0),
        p.ink,
    );
    response.on_hover_text("Open this alert")
}

fn mark(p: &chrome::Palette, status: &str) -> Color32 {
    match status {
        "accepted" | "closed" => p.teal,
        "rejected" => p.copper,
        "on_hold" => p.amber,
        "in_inspection" => p.blue,
        _ => p.muted,
    }
}

fn qty(n: f64) -> String {
    if (n - n.round()).abs() < 0.001 {
        format!("{}", n.round() as i64)
    } else {
        format!("{n:.1}")
    }
}

fn truncate(text: &str, max: usize) -> String {
    if text.chars().count() <= max {
        text.to_string()
    } else {
        let cut: String = text.chars().take(max.saturating_sub(1)).collect();
        format!("{cut}…")
    }
}
