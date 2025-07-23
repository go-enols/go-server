//! Remote method call functionality for the worker module

use serde_json::Value;
use crate::scheduler::Client;
use crate::error::Result;

/// Calls a remote method on the scheduler
/// 
/// This function provides a simple interface for making remote method calls
/// to the scheduler. It handles task execution and result retrieval automatically.
/// 
/// # Arguments
/// 
/// * `scheduler_url` - The HTTP URL of the scheduler
/// * `method` - The name of the method to call
/// * `params` - The parameters to pass to the method
/// 
/// # Returns
/// 
/// A `Result` containing the method result or an error
/// 
/// # Example
/// 
/// ```rust
/// use go_server_rust_sdk::worker::call;
/// use serde_json::json;
/// 
/// # #[tokio::main]
/// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
/// let result = call(
///     "http://localhost:8080",
///     "add",
///     json!({"a": 1, "b": 2})
/// ).await?;
/// 
/// println!("Result: {}", result);
/// # Ok(())
/// # }
/// ```
pub async fn call(scheduler_url: &str, method: &str, params: Value) -> Result<Value> {
    use std::time::Duration;
    let client = Client::new(scheduler_url.to_string());
    
    // Execute the task synchronously (with polling)
    let result = client.execute_sync(method, params, Duration::from_secs(30)).await?;
    
    // Extract the actual result value from the response
    match result.result {
        Some(value) => Ok(value),
        None => Err(crate::error::SdkError::Task("No result returned".to_string())),
    }
}

/// Calls a remote method on the scheduler with encryption
/// 
/// This function provides a simple interface for making encrypted remote method calls
/// to the scheduler. It handles encryption, task execution, and result decryption automatically.
/// 
/// # Arguments
/// 
/// * `scheduler_url` - The HTTP URL of the scheduler
/// * `method` - The name of the method to call
/// * `params` - The parameters to pass to the method
/// * `key` - The encryption key to use
/// 
/// # Returns
/// 
/// A `Result` containing the method result or an error
/// 
/// # Example
/// 
/// ```rust
/// use go_server_rust_sdk::worker::call_encrypted;
/// use serde_json::json;
/// 
/// # #[tokio::main]
/// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
/// let result = call_encrypted(
///     "http://localhost:8080",
///     "add",
///     json!({"a": 1, "b": 2}),
///     "my-secret-key"
/// ).await?;
/// 
/// println!("Result: {}", result);
/// # Ok(())
/// # }
/// ```
pub async fn call_encrypted(
    scheduler_url: &str,
    method: &str,
    params: Value,
    key: &str,
) -> Result<Value> {
    use std::time::Duration;
    use rand::Rng;
    let client = Client::new(scheduler_url.to_string());
    
    // Generate a random salt for encryption
    let salt = rand::thread_rng().gen::<i32>();
    
    // Execute the encrypted task synchronously (with polling)
    let result = client.execute_sync_encrypted(method, key, salt, params, Duration::from_secs(30)).await?;
    
    // Extract the actual result value from the response
    match result.result {
        Some(value) => Ok(value),
        None => Err(crate::error::SdkError::Task("No result returned".to_string())),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    // Note: These tests would require a running scheduler server
    // They are included for documentation purposes
    
    #[tokio::test]
    #[ignore] // Ignore by default since it requires a running server
    async fn test_call() {
        let result = call(
            "http://localhost:8080",
            "add",
            json!({"a": 1, "b": 2})
        ).await;
        
        // This would pass if the server is running and has an "add" method
        assert!(result.is_ok());
    }
    
    #[tokio::test]
    #[ignore] // Ignore by default since it requires a running server
    async fn test_call_encrypted() {
        let result = call_encrypted(
            "http://localhost:8080",
            "add",
            json!({"a": 1, "b": 2}),
            "test-key"
        ).await;
        
        // This would pass if the server is running and has an "add" method
        assert!(result.is_ok());
    }
}