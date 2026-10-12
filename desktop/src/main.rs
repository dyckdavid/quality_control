use clap::Parser;
use qc_desktop::app::{LaunchOptions, QcApp};
use qc_desktop::settings::AppSettings;
use std::path::PathBuf;

#[derive(Parser, Debug)]
#[command(name = "qc-desktop", about = "Native quality-control desktop client")]
struct Args {
    /// Override API server URL (otherwise uses persisted settings).
    #[arg(long)]
    server: Option<String>,

    /// Directory to write UI screenshots, then exit (used under Xvfb).
    #[arg(long)]
    screenshot_dir: Option<PathBuf>,

    /// Username for automated login during screenshot capture.
    #[arg(long, env = "QC_DESKTOP_USER")]
    user: Option<String>,

    /// Password for automated login during screenshot capture.
    #[arg(long, env = "QC_DESKTOP_PASSWORD")]
    password: Option<String>,
}

fn main() -> eframe::Result<()> {
    let args = Args::parse();
    let mut settings = AppSettings::load();
    if let Some(server) = args.server {
        settings.server_url = server;
    }

    let opts = LaunchOptions {
        settings,
        screenshot_dir: args.screenshot_dir,
        auto_login_user: args.user,
        auto_login_pass: args.password,
    };

    let native_options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_inner_size([1280.0, 800.0])
            .with_min_inner_size([900.0, 600.0])
            .with_title("Quality Control"),
        ..Default::default()
    };

    eframe::run_native(
        "Quality Control",
        native_options,
        Box::new(|cc| Ok(Box::new(QcApp::new(cc, opts)))),
    )
}
