//! Worker functionality for creating and managing distributed task workers

mod client;
mod call;

pub use client::{Worker, Config, Method};
pub use call::call;

/// Task status constants
pub const TASK_STATUS_ERROR: &str = "error";
pub const TASK_STATUS_DONE: &str = "done";
pub const TASK_STATUS_PENDING: &str = "pending";
pub const TASK_STATUS_PROCESSING: &str = "processing";