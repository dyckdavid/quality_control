//! Shared visual language for the desktop app.
//! Inter is bundled under the SIL Open Font License (see assets/Inter-LICENSE.txt).

use eframe::egui::{
    self, Color32, FontFamily, FontId, Rect, Rounding, Sense, Stroke, UiBuilder, Vec2,
};

#[derive(Clone, Copy)]
pub struct Palette {
    pub canvas: Color32,
    pub card: Color32,
    pub ink: Color32,
    pub muted: Color32,
    pub line: Color32,
    pub hover: Color32,
    pub sidebar: Color32,
    pub sidebar_ink: Color32,
    pub sidebar_muted: Color32,
    pub sidebar_hover: Color32,
    pub sidebar_selected: Color32,
    pub teal: Color32,
    pub teal_hover: Color32,
    pub teal_soft: Color32,
    pub copper: Color32,
    pub copper_soft: Color32,
    pub amber: Color32,
    pub amber_soft: Color32,
    pub blue: Color32,
    pub blue_soft: Color32,
}

pub fn colors(dark: bool) -> Palette {
    if dark {
        Palette {
            canvas: Color32::from_rgb(14, 16, 20),
            card: Color32::from_rgb(24, 27, 34),
            ink: Color32::from_rgb(244, 245, 247),
            muted: Color32::from_rgb(154, 160, 171),
            line: Color32::from_rgb(46, 51, 61),
            hover: Color32::from_rgb(32, 36, 46),
            sidebar: Color32::from_rgb(8, 9, 12),
            sidebar_ink: Color32::from_rgb(244, 245, 247),
            sidebar_muted: Color32::from_rgb(154, 160, 171),
            sidebar_hover: Color32::from_rgb(22, 25, 32),
            sidebar_selected: Color32::from_rgb(28, 33, 42),
            teal: Color32::from_rgb(45, 168, 148),
            teal_hover: Color32::from_rgb(62, 184, 164),
            teal_soft: Color32::from_rgb(18, 48, 44),
            copper: Color32::from_rgb(242, 125, 78),
            copper_soft: Color32::from_rgb(58, 32, 24),
            amber: Color32::from_rgb(232, 176, 74),
            amber_soft: Color32::from_rgb(54, 42, 18),
            blue: Color32::from_rgb(126, 168, 232),
            blue_soft: Color32::from_rgb(24, 36, 58),
        }
    } else {
        Palette {
            canvas: Color32::from_rgb(244, 245, 247),
            card: Color32::WHITE,
            ink: Color32::from_rgb(17, 19, 24),
            muted: Color32::from_rgb(92, 99, 112),
            line: Color32::from_rgb(226, 228, 233),
            hover: Color32::from_rgb(246, 247, 249),
            sidebar: Color32::from_rgb(16, 18, 22),
            sidebar_ink: Color32::from_rgb(246, 247, 249),
            sidebar_muted: Color32::from_rgb(156, 163, 175),
            sidebar_hover: Color32::from_rgb(28, 31, 38),
            sidebar_selected: Color32::from_rgb(34, 39, 48),
            teal: Color32::from_rgb(15, 110, 98),
            teal_hover: Color32::from_rgb(12, 92, 82),
            teal_soft: Color32::from_rgb(226, 242, 238),
            copper: Color32::from_rgb(180, 62, 28),
            copper_soft: Color32::from_rgb(252, 236, 228),
            amber: Color32::from_rgb(161, 98, 7),
            amber_soft: Color32::from_rgb(252, 243, 220),
            blue: Color32::from_rgb(37, 84, 158),
            blue_soft: Color32::from_rgb(230, 238, 250),
        }
    }
}

pub fn palette(ui: &egui::Ui) -> Palette {
    colors(ui.visuals().dark_mode)
}

pub fn install_fonts(ctx: &egui::Context) {
    let mut fonts = egui::FontDefinitions::default();
    fonts.font_data.insert(
        "inter".to_owned(),
        egui::FontData::from_static(include_bytes!("../../assets/Inter-Regular.ttf")),
    );
    fonts.font_data.insert(
        "inter-semi".to_owned(),
        egui::FontData::from_static(include_bytes!("../../assets/Inter-SemiBold.ttf")),
    );
    let proportional = fonts.families.entry(FontFamily::Proportional).or_default();
    proportional.insert(0, "inter".to_owned());
    // Inter lacks symbols like ⌘ and ✓; fall back to egui's bundled fonts for those.
    let mut semibold = vec!["inter-semi".to_owned()];
    semibold.extend(proportional.iter().skip(1).cloned());
    fonts.families.insert(FontFamily::Name("semibold".into()), semibold);
    ctx.set_fonts(fonts);
}

pub fn semi(text: impl Into<String>, size: f32, color: Color32) -> egui::RichText {
    egui::RichText::new(text)
        .size(size)
        .color(color)
        .family(FontFamily::Name("semibold".into()))
}

pub fn semibold_id(size: f32) -> FontId {
    FontId::new(size, FontFamily::Name("semibold".into()))
}

#[derive(Clone, Copy)]
pub enum Btn {
    Primary,
    Quiet,
    Danger,
}

pub fn button(ui: &mut egui::Ui, label: &str, kind: Btn) -> egui::Response {
    let width = (label.chars().count() as f32) * 7.4 + 28.0;
    paint_button(ui, label, kind, width.max(72.0), 34.0)
}

pub fn button_fill(ui: &mut egui::Ui, label: &str, kind: Btn) -> egui::Response {
    let width = ui.available_width();
    paint_button(ui, label, kind, width, 40.0)
}

fn paint_button(
    ui: &mut egui::Ui,
    label: &str,
    kind: Btn,
    width: f32,
    height: f32,
) -> egui::Response {
    let p = palette(ui);
    let (rect, response) = ui.allocate_exact_size(Vec2::new(width, height), Sense::click());
    let enabled = ui.is_enabled();
    let (fill, text, stroke) = if !enabled {
        (p.hover, p.muted, Stroke::new(1.0_f32, p.line))
    } else {
        match kind {
            Btn::Primary => (
                if response.hovered() { p.teal_hover } else { p.teal },
                Color32::WHITE,
                Stroke::NONE,
            ),
            Btn::Quiet => (
                if response.hovered() { p.hover } else { p.card },
                p.ink,
                Stroke::new(1.0_f32, p.line),
            ),
            Btn::Danger => (
                if response.hovered() {
                    p.copper_soft
                } else {
                    p.card
                },
                p.copper,
                Stroke::new(1.0_f32, p.line),
            ),
        }
    };
    ui.painter()
        .rect(rect, Rounding::same(8.0), fill, stroke);
    ui.painter().text(
        rect.center(),
        egui::Align2::CENTER_CENTER,
        label,
        semibold_id(13.5),
        text,
    );
    response
}

pub fn nav_heading(ui: &mut egui::Ui, title: &str) {
    let p = palette(ui);
    ui.add_space(16.0);
    ui.label(
        semi(title.to_uppercase(), 11.0, p.sidebar_muted).extra_letter_spacing(0.6),
    );
    ui.add_space(4.0);
}

pub fn nav_row(ui: &mut egui::Ui, label: &str, selected: bool) -> egui::Response {
    let p = palette(ui);
    let height = 34.0;
    let (rect, response) =
        ui.allocate_exact_size(Vec2::new(ui.available_width(), height), Sense::click());
    let bg = if selected {
        p.sidebar_selected
    } else if response.hovered() {
        p.sidebar_hover
    } else {
        Color32::TRANSPARENT
    };
    if bg != Color32::TRANSPARENT {
        ui.painter().rect_filled(rect, Rounding::same(8.0), bg);
    }
    if selected {
        let bar = Rect::from_min_size(rect.min + Vec2::new(0.0, 9.0), Vec2::new(3.0, height - 18.0));
        ui.painter()
            .rect_filled(bar, Rounding::same(2.0), p.teal);
    }
    let color = if selected { p.sidebar_ink } else { p.sidebar_muted };
    let font = if selected {
        semibold_id(13.5)
    } else {
        FontId::proportional(13.5)
    };
    ui.painter().text(
        rect.left_center() + Vec2::new(16.0, 0.0),
        egui::Align2::LEFT_CENTER,
        label,
        font,
        color,
    );
    response
}

pub fn pill(ui: &mut egui::Ui, status: &str) {
    let p = palette(ui);
    let (bg, fg) = tone(&p, status);
    let label = if status.is_empty() {
        "—".to_string()
    } else {
        status.replace('_', " ")
    };
    let font = semibold_id(11.5);
    let width = (label.chars().count() as f32) * 6.6 + 16.0;
    let (rect, _) = ui.allocate_exact_size(Vec2::new(width.max(36.0), 22.0), Sense::hover());
    ui.painter()
        .rect_filled(rect, Rounding::same(11.0), bg);
    ui.painter().text(
        rect.center(),
        egui::Align2::CENTER_CENTER,
        label,
        font,
        fg,
    );
}

fn tone(p: &Palette, status: &str) -> (Color32, Color32) {
    match status {
        "pass" | "completed" | "closed" | "accepted" | "active" | "true" => (p.teal_soft, p.teal),
        "fail" | "critical" | "rejected" | "void" | "cancelled" | "false" => {
            (p.copper_soft, p.copper)
        }
        "major" | "conditional" | "on_hold" | "overdue" | "verification" | "hold"
        | "dispositioned" => {
            (p.amber_soft, p.amber)
        }
        "open" | "in_progress" | "in_inspection" | "under_review" | "pending" | "minor"
        | "incoming" => (p.blue_soft, p.blue),
        _ => (p.hover, p.muted),
    }
}

pub fn surface<R>(
    ui: &mut egui::Ui,
    id: &str,
    height: f32,
    body: impl FnOnce(&mut egui::Ui) -> R,
) -> R {
    let p = palette(ui);
    let (rect, _) =
        ui.allocate_exact_size(Vec2::new(ui.available_width(), height), Sense::hover());
    ui.painter()
        .rect(rect, Rounding::same(14.0), p.card, Stroke::new(1.0_f32, p.line));
    let inner = rect.shrink(18.0);
    let mut child = ui.new_child(
        UiBuilder::new()
            .id_salt(id)
            .max_rect(inner)
            .layout(egui::Layout::top_down(egui::Align::LEFT)),
    );
    child.set_clip_rect(rect);
    body(&mut child)
}

pub fn form_card<R>(ui: &mut egui::Ui, title: &str, body: impl FnOnce(&mut egui::Ui) -> R) -> R {
    let p = palette(ui);
    let width = ui.available_width().min(680.0);
    egui::Frame::none()
        .fill(p.card)
        .stroke(Stroke::new(1.0_f32, p.line))
        .rounding(Rounding::same(14.0))
        .inner_margin(egui::Margin::same(18.0))
        .show(ui, |ui| {
            ui.set_min_width((width - 36.0).max(240.0));
            ui.set_max_width((width - 36.0).max(240.0));
            ui.label(semi(title, 16.0, p.ink));
            ui.add_space(10.0);
            body(ui)
        })
        .inner
}

/// A card that fills the column it sits in.
pub fn card<R>(ui: &mut egui::Ui, title: &str, body: impl FnOnce(&mut egui::Ui) -> R) -> R {
    let p = palette(ui);
    let width = ui.available_width();
    egui::Frame::none()
        .fill(p.card)
        .stroke(Stroke::new(1.0_f32, p.line))
        .rounding(Rounding::same(14.0))
        .inner_margin(egui::Margin::same(18.0))
        .show(ui, |ui| {
            ui.set_width((width - 36.0).max(200.0));
            if !title.is_empty() {
                ui.label(semi(title, 15.5, p.ink));
                ui.add_space(10.0);
            }
            body(ui)
        })
        .inner
}

/// A chip for pass/fail and similar choices. When selected, `good` picks teal over copper.
pub fn choice(ui: &mut egui::Ui, label: &str, selected: bool, good: bool) -> egui::Response {
    let p = palette(ui);
    let width = (label.chars().count() as f32) * 7.2 + 24.0;
    let (rect, response) = ui.allocate_exact_size(Vec2::new(width.max(56.0), 30.0), Sense::click());
    let enabled = ui.is_enabled();
    let (fill, fg, stroke) = match (selected, good) {
        (true, true) => (p.teal_soft, p.teal, Stroke::new(1.0_f32, p.teal)),
        (true, false) => (p.copper_soft, p.copper, Stroke::new(1.0_f32, p.copper)),
        (false, _) if response.hovered() && enabled => (p.hover, p.ink, Stroke::new(1.0_f32, p.line)),
        (false, _) => (p.card, p.muted, Stroke::new(1.0_f32, p.line)),
    };
    ui.painter().rect(rect, Rounding::same(8.0), fill, stroke);
    ui.painter()
        .text(rect.center(), egui::Align2::CENTER_CENTER, label, semibold_id(13.0), fg);
    response
}

pub fn muted(ui: &mut egui::Ui, text: impl Into<String>) {
    let p = palette(ui);
    ui.label(egui::RichText::new(text).color(p.muted).size(13.0));
}

pub fn metric(ui: &mut egui::Ui, label: &str, value: &str, accent: Color32) {
    let p = palette(ui);
    let (rect, _) =
        ui.allocate_exact_size(Vec2::new(ui.available_width(), 108.0), Sense::hover());
    ui.painter()
        .rect(rect, Rounding::same(14.0), p.card, Stroke::new(1.0_f32, p.line));
    let dot = Rect::from_center_size(rect.min + Vec2::new(22.0, 28.0), Vec2::splat(8.0));
    ui.painter().circle_filled(dot.center(), 4.0, accent);
    ui.painter().text(
        rect.min + Vec2::new(34.0, 20.0),
        egui::Align2::LEFT_TOP,
        label,
        FontId::proportional(13.0),
        p.muted,
    );
    ui.painter().text(
        rect.min + Vec2::new(18.0, 48.0),
        egui::Align2::LEFT_TOP,
        value,
        semibold_id(32.0),
        p.ink,
    );
}

pub fn banner(ui: &mut egui::Ui, message: &str, danger: bool) {
    let p = palette(ui);
    let (fill, fg) = if danger {
        (p.copper_soft, p.copper)
    } else {
        (p.teal_soft, p.teal)
    };
    egui::Frame::none()
        .fill(fill)
        .rounding(Rounding::same(8.0))
        .inner_margin(egui::Margin::symmetric(12.0, 8.0))
        .show(ui, |ui| {
            ui.label(egui::RichText::new(message).color(fg).size(13.5));
        });
}

pub fn field_label(ui: &mut egui::Ui, text: &str) {
    let p = palette(ui);
    ui.label(semi(text, 12.0, p.muted));
    ui.add_space(4.0);
}

pub fn text_line(ui: &mut egui::Ui, value: &mut String) {
    ui.add(
        egui::TextEdit::singleline(value)
            .desired_width(f32::INFINITY)
            .margin(egui::Margin::symmetric(10.0, 8.0)),
    );
}

pub fn pretty_key(key: &str) -> String {
    let mut out = String::new();
    for (i, part) in key.split('_').filter(|p| !p.is_empty()).enumerate() {
        if i > 0 {
            out.push(' ');
        }
        let mut chars = part.chars();
        if let Some(first) = chars.next() {
            out.extend(first.to_uppercase());
            out.push_str(chars.as_str());
        }
    }
    out
}
