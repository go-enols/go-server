//! Error types for the go-server Rust SDK

use thiserror::Error;

/// Result type alias for SDK operations
pub type Result<T> = std::result::Result<T, SdkError>;

/// Main error type for the SDK
#[derive(Error, Debug)]
pub enum SdkError {
    /// HTTP request errors
    #[error("HTTP request failed: {0}")]
    Http(#[from] reqwest::Error),

    /// WebSocket connection errors
    #[error("WebSocket error: {0}")]
    WebSocket(Box<tokio_tungstenite::tungstenite::Error>),

    /// JSON serialization/deserialization errors
    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    /// Encryption/decryption errors
    #[error("Crypto error: {0}")]
    Crypto(String),

    /// Task execution errors
    #[error("Task error: {0}")]
    Task(String),

    /// Connection errors
    #[error("Connection error: {0}")]
    Connection(String),

    /// Timeout errors
    #[error("Operation timed out")]
    Timeout,

    /// Method not found errors
    #[error("Method '{0}' not found")]
    MethodNotFound(String),

    /// Invalid configuration errors
    #[error("Invalid configuration: {0}")]
    InvalidConfig(String),

    /// URL parsing errors
    #[error("URL parse error: {0}")]
    UrlParse(#[from] url::ParseError),

    /// Generic errors
    #[error("Error: {0}")]
    Generic(String),
}

impl From<String> for SdkError {
    fn from(s: String) -> Self {
        SdkError::Generic(s)
    }
}

impl From<&str> for SdkError {
    fn from(s: &str) -> Self {
        SdkError::Generic(s.to_string())
    }
}

impl From<aes_gcm::Error> for SdkError {
    fn from(e: aes_gcm::Error) -> Self {
        SdkError::Crypto(format!("AES-GCM error: {:?}", e))
    }
}

impl From<base64::DecodeError> for SdkError {
    fn from(e: base64::DecodeError) -> Self {
        SdkError::Crypto(format!("Base64 decode error: {}", e))
    }
}

impl From<tokio_tungstenite::tungstenite::Error> for SdkError {
    fn from(e: tokio_tungstenite::tungstenite::Error) -> Self {
        SdkError::WebSocket(Box::new(e))
    }
}