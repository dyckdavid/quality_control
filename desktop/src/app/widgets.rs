use super::chrome::{self, Btn};
use eframe::egui;

pub fn loading_spinner(ui: &mut egui::Ui) {
    let p = chrome::palette(ui);
    ui.horizontal(|ui| {
        ui.spinner();
        ui.label(egui::RichText::new("Loading").color(p.muted).size(13.0));
    });
}

pub fn empty_state(ui: &mut egui::Ui, title: &str, hint: &str) {
    let p = chrome::palette(ui);
    ui.add_space(12.0);
    ui.label(chrome::semi(title, 18.0, p.ink));
    ui.add_space(4.0);
    ui.label(egui::RichText::new(hint).color(p.muted).size(14.0));
}

pub fn error_banner(ui: &mut egui::Ui, message: &str) {
    chrome::banner(ui, message, true);
}

pub fn searchable_field(ui: &mut egui::Ui, search: &mut String) {
    ui.add(
        egui::TextEdit::singleline(search)
            .hint_text("Filter this list")
            .desired_width(260.0)
            .margin(egui::Margin::symmetric(10.0, 8.0)),
    );
}

pub fn status_badge(ui: &mut egui::Ui, status: &str) {
    chrome::pill(ui, status);
}

pub fn quiet_button(ui: &mut egui::Ui, label: &str) -> egui::Response {
    chrome::button(ui, label, Btn::Quiet)
}
