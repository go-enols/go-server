//! # Go-Server Rust SDK
//!
//! This crate provides a Rust SDK for interacting with the go-server distributed task scheduler.
//! It includes both scheduler client functionality and worker implementation.
//!
//! ## Features
//!
//! - **Scheduler Client**: Execute tasks on remote workers
//! - **Worker**: Register and handle distributed tasks
//! - **Encryption**: Support for encrypted task execution
//! - **Retry Logic**: Built-in retry mechanisms for reliability
//!
//! ## Quick Start
//!
//! ### Using the Scheduler Client
//!
//! ```rust,no_run
//! use go_server_rust_sdk::scheduler::Client;
//! use serde_json::json;
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let client = Client::new("http://localhost:8080");
//!     
//!     let params = json!({
//!         "a": 10,
//!         "b": 20
//!     });
//!     
//!     let result = client.execute_sync("add", params, std::time::Duration::from_secs(30)).await?;
//!     println!("Result: {:?}", result);
//!     
//!     Ok(())
//! }
//! ```
//!
//! ### Creating a Worker
//!
//! ```rust,no_run
//! use go_server_rust_sdk::worker::{Worker, Config};
//! use serde_json::{json, Value};
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let config = Config {
//!         scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
//!         worker_group: "math".to_string(),
//!         max_retry: 5,
//!         ping_interval: 5,
//!     };
//!     
//!     let mut worker = Worker::new(config);
//!     
//!     worker.register_method("add", |params: Value| {
//!         let a = params["a"].as_f64().unwrap_or(0.0);
//!         let b = params["b"].as_f64().unwrap_or(0.0);
//!         Ok(json!(a + b))
//!     }, vec!["Add two numbers".to_string()]);
//!     
//!     worker.start().await?;
//!     
//!     Ok(())
//! }
//! ```

pub mod scheduler;
pub mod worker;
pub mod crypto;
pub mod error;

pub use error::{Result, SdkError};

/// Re-export commonly used types
pub mod prelude {
    pub use crate::scheduler::{Client, RetryClient, ResultResponse};
    pub use crate::worker::{Worker, Config as WorkerConfig};
    pub use crate::error::{Result, SdkError};
    pub use serde_json::{json, Value};
}