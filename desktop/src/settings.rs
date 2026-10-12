use directories::{ProjectDirs, UserDirs};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct AppSettings {
    pub server_url: String,
    pub theme: ThemePreference,
    pub last_username: String,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ThemePreference {
    Light,
    Dark,
    System,
}

impl Default for AppSettings {
    fn default() -> Self {
        Self {
            server_url: "http://127.0.0.1:8080".to_string(),
            theme: ThemePreference::System,
            last_username: String::new(),
        }
    }
}

impl AppSettings {
    pub fn load() -> Self {
        let path = settings_path();
        match fs::read_to_string(&path) {
            Ok(text) => serde_json::from_str(&text).unwrap_or_default(),
            Err(_) => Self::default(),
        }
    }

    pub fn save(&self) -> Result<(), String> {
        let path = settings_path();
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        let text = serde_json::to_string_pretty(self).map_err(|e| e.to_string())?;
        fs::write(&path, text).map_err(|e| e.to_string())
    }

    pub fn path_display() -> String {
        settings_path().display().to_string()
    }
}

fn settings_path() -> PathBuf {
    if let Some(dirs) = ProjectDirs::from("com", "QualityControl", "qc-desktop") {
        dirs.config_dir().join("settings.json")
    } else if let Some(user) = UserDirs::new() {
        user.home_dir().join(".qc-desktop").join("settings.json")
    } else {
        PathBuf::from("qc-desktop-settings.json")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_server_url() {
        let s = AppSettings::default();
        assert!(s.server_url.contains("8080"));
        assert_eq!(s.theme, ThemePreference::System);
    }

    #[test]
    fn roundtrip_json() {
        let s = AppSettings {
            server_url: "http://example:9000".into(),
            theme: ThemePreference::Dark,
            last_username: "admin".into(),
        };
        let text = serde_json::to_string(&s).unwrap();
        let back: AppSettings = serde_json::from_str(&text).unwrap();
        assert_eq!(s, back);
    }
}
