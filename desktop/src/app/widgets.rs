use eframe::egui;

pub fn loading_spinner(ui: &mut egui::Ui) {
    ui.horizontal(|ui| {
        ui.spinner();
        ui.label("Loading…");
    });
}

pub fn empty_state(ui: &mut egui::Ui, title: &str, hint: &str) {
    ui.vertical_centered(|ui| {
        ui.add_space(40.0);
        ui.label(egui::RichText::new(title).heading().weak());
        ui.label(egui::RichText::new(hint).weak());
    });
}

pub fn error_banner(ui: &mut egui::Ui, message: &str) {
    ui.colored_label(egui::Color32::from_rgb(200, 60, 60), message);
}

pub fn searchable_field(ui: &mut egui::Ui, search: &mut String) {
    ui.label("Search");
    ui.add(
        egui::TextEdit::singleline(search)
            .hint_text("Filter table…")
            .desired_width(220.0),
    );
}

pub fn status_badge(ui: &mut egui::Ui, status: &str) {
    let color = match status {
        "pass" | "completed" | "closed" | "accepted" => egui::Color32::from_rgb(46, 125, 80),
        "fail" | "critical" | "rejected" | "void" | "cancelled" => {
            egui::Color32::from_rgb(180, 50, 50)
        }
        "major" | "conditional" | "on_hold" | "overdue" | "verification" => {
            egui::Color32::from_rgb(180, 120, 30)
        }
        "open" | "in_progress" | "under_review" | "pending" | "minor" => {
            egui::Color32::from_rgb(50, 100, 180)
        }
        _ => egui::Color32::GRAY,
    };
    ui.colored_label(color, status);
}
