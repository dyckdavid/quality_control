use thiserror::Error;

#[derive(Debug, Error)]
pub enum ApiError {
    #[error("HTTP {status}: {message}")]
    Http { status: u16, code: String, message: String },
    #[error("transport: {0}")]
    Transport(String),
    #[error("decode: {0}")]
    Decode(String),
    #[error("invalid request: {0}")]
    Invalid(String),
}

impl ApiError {
    /// Text for people on the floor: the server's own sentence, without status codes.
    pub fn message(&self) -> String {
        match self {
            ApiError::Http { status: 403, .. } => {
                "Your role can't do that. Ask a supervisor or admin.".into()
            }
            ApiError::Http { message, .. } if !message.trim().is_empty() => {
                let mut m = message.trim().to_string();
                if let Some(first) = m.get(0..1) {
                    let upper = first.to_uppercase();
                    m.replace_range(0..1, &upper);
                }
                if !m.ends_with('.') {
                    m.push('.');
                }
                m
            }
            ApiError::Http { status, .. } => format!("The server answered {status}."),
            ApiError::Transport(_) => {
                "Can't reach the server. Check that it is running and the address in Settings.".into()
            }
            ApiError::Decode(e) => format!("The server sent something this app can't read ({e})."),
            ApiError::Invalid(e) => e.clone(),
        }
    }

    pub fn is_unauthorized(&self) -> bool {
        matches!(self, ApiError::Http { status: 401, .. })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn message_drops_status_and_capitalizes() {
        let e = ApiError::Http {
            status: 422,
            code: "invalid".into(),
            message: "record a disposition before closing".into(),
        };
        assert_eq!(e.message(), "Record a disposition before closing.");
    }

    #[test]
    fn forbidden_explains_role() {
        let e = ApiError::Http { status: 403, code: "forbidden".into(), message: "x".into() };
        assert!(e.message().contains("role"));
    }
}
