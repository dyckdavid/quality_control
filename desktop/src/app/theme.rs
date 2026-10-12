use crate::settings::ThemePreference;
use eframe::egui::{Color32, Rounding, Stroke, Vec2, Visuals};

use super::chrome::{self, colors};

pub fn apply_theme(ctx: &egui::Context, pref: ThemePreference) {
    let dark = match pref {
        ThemePreference::Dark => true,
        ThemePreference::Light => false,
        ThemePreference::System => ctx.system_theme() == Some(egui::Theme::Dark),
    };
    let p = colors(dark);
    let mut style = (*ctx.style()).clone();
    style.visuals = plant_visuals(dark, &p);
    style.spacing.item_spacing = Vec2::new(8.0, 8.0);
    style.spacing.button_padding = Vec2::new(12.0, 8.0);
    style.spacing.interact_size = Vec2::new(40.0, 34.0);
    style.spacing.menu_margin = egui::Margin::same(8.0);
    style.text_styles.insert(
        egui::TextStyle::Small,
        egui::FontId::proportional(12.0),
    );
    style.text_styles.insert(
        egui::TextStyle::Body,
        egui::FontId::proportional(14.0),
    );
    style.text_styles.insert(
        egui::TextStyle::Button,
        chrome::semibold_id(13.5),
    );
    style.text_styles.insert(
        egui::TextStyle::Heading,
        chrome::semibold_id(22.0),
    );
    style.text_styles.insert(
        egui::TextStyle::Monospace,
        egui::FontId::monospace(13.0),
    );
    ctx.set_style(style);
}

fn plant_visuals(dark: bool, p: &chrome::Palette) -> Visuals {
    let mut visuals = if dark {
        Visuals::dark()
    } else {
        Visuals::light()
    };
    visuals.window_fill = p.card;
    visuals.panel_fill = p.canvas;
    visuals.extreme_bg_color = if dark {
        Color32::from_rgb(16, 18, 24)
    } else {
        Color32::from_rgb(248, 249, 251)
    };
    visuals.faint_bg_color = if dark {
        Color32::from_rgb(30, 34, 42)
    } else {
        Color32::from_rgb(247, 248, 250)
    };
    visuals.code_bg_color = visuals.extreme_bg_color;
    visuals.hyperlink_color = p.teal;
    visuals.selection.bg_fill = p.teal_soft;
    visuals.selection.stroke = Stroke::new(1.0_f32, p.teal);
    visuals.warn_fg_color = p.amber;
    visuals.error_fg_color = p.copper;
    visuals.window_rounding = Rounding::same(12.0);
    visuals.menu_rounding = Rounding::same(10.0);
    visuals.window_shadow = egui::epaint::Shadow {
        offset: Vec2::new(0.0, 10.0),
        blur: 28.0,
        spread: 0.0,
        color: Color32::from_black_alpha(if dark { 80 } else { 28 }),
    };

    for widget in [
        &mut visuals.widgets.noninteractive,
        &mut visuals.widgets.inactive,
        &mut visuals.widgets.hovered,
        &mut visuals.widgets.active,
        &mut visuals.widgets.open,
    ] {
        widget.rounding = Rounding::same(8.0);
        widget.expansion = 0.0;
    }
    visuals.widgets.noninteractive.bg_stroke = Stroke::new(1.0_f32, p.line);
    visuals.widgets.noninteractive.fg_stroke = Stroke::new(1.0_f32, p.ink);
    visuals.widgets.inactive.bg_fill = p.card;
    visuals.widgets.inactive.bg_stroke = Stroke::new(1.0_f32, p.line);
    visuals.widgets.inactive.fg_stroke = Stroke::new(1.0_f32, p.ink);
    visuals.widgets.inactive.weak_bg_fill = p.hover;
    visuals.widgets.hovered.bg_fill = p.hover;
    visuals.widgets.hovered.bg_stroke = Stroke::new(1.0_f32, p.teal);
    visuals.widgets.hovered.fg_stroke = Stroke::new(1.0_f32, p.ink);
    visuals.widgets.active.bg_fill = p.teal;
    visuals.widgets.active.bg_stroke = Stroke::NONE;
    visuals.widgets.active.fg_stroke = Stroke::new(1.0_f32, Color32::WHITE);
    visuals.widgets.open.bg_fill = p.card;
    visuals.widgets.open.bg_stroke = Stroke::new(1.0_f32, p.teal);
    visuals.widgets.open.fg_stroke = Stroke::new(1.0_f32, p.ink);
    visuals
}
