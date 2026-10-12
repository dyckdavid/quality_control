use crate::settings::ThemePreference;
use eframe::egui::{self, Visuals};

pub fn apply_theme(ctx: &egui::Context, pref: ThemePreference) {
    match pref {
        ThemePreference::Dark => ctx.set_visuals(Visuals::dark()),
        ThemePreference::Light => ctx.set_visuals(Visuals::light()),
        ThemePreference::System => {
            // Prefer dark when the OS theme is unknown (common on headless Xvfb).
            let dark = ctx
                .style()
                .visuals
                .dark_mode;
            // Keep whatever was last applied unless user flips preference;
            // for System default to light on first run unless dark_mode already set.
            if dark {
                ctx.set_visuals(Visuals::dark());
            } else {
                ctx.set_visuals(Visuals::light());
            }
        }
    }
}
