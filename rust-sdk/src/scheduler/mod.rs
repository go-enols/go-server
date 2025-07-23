//! Scheduler client functionality for interacting with the go-server scheduler

mod client;
mod retry_client;

pub use client::{Client, ExecuteRequest, ExecuteEncryptedRequest, ResultResponse};
pub use retry_client::RetryClient;

/// Task status constants
pub const TASK_STATUS_ERROR: &str = "error";
pub const TASK_STATUS_DONE: &str = "done";
pub const TASK_STATUS_PENDING: &str = "pending";
pub const TASK_STATUS_PROCESSING: &str = "processing";