//! Quality-control desktop library: API client, models, settings, and app shell.

pub mod api;
pub mod app;
pub mod csv_export;
pub mod settings;

pub use api::{ApiClient, ApiError};
pub use app::{LaunchOptions, QcApp};
pub use settings::AppSettings;
