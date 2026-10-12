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
    pub fn message(&self) -> String {
        self.to_string()
    }

    pub fn is_unauthorized(&self) -> bool {
        matches!(self, ApiError::Http { status: 401, .. })
    }
}
