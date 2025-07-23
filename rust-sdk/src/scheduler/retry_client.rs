//! Retry client implementation for the scheduler

use serde_json::Value;
use std::time::Duration;
use tokio::time::sleep;
use crate::error::{Result, SdkError};
use super::client::{Client, ResultResponse};

/// Retry client that wraps the base client with retry functionality
#[derive(Clone)]
pub struct RetryClient {
    client: Client,
    max_retries: usize,
    retry_delay: Duration,
}

impl RetryClient {
    /// Creates a new retry client
    /// 
    /// # Arguments
    /// 
    /// * `base_url` - The base URL of the scheduler server
    /// * `max_retries` - Maximum number of retry attempts
    /// * `retry_delay` - Delay between retry attempts
    /// 
    /// # Example
    /// 
    /// ```rust
    /// use go_server_rust_sdk::scheduler::RetryClient;
    /// use std::time::Duration;
    /// 
    /// let client = RetryClient::new(
    ///     "http://localhost:8080",
    ///     3,
    ///     Duration::from_secs(1)
    /// );
    /// ```
    pub fn new(
        base_url: impl Into<String>,
        max_retries: usize,
        retry_delay: Duration,
    ) -> Self {
        Self {
            client: Client::new(base_url),
            max_retries,
            retry_delay,
        }
    }

    /// Executes a task with retry logic
    /// 
    /// This method will retry the execution up to `max_retries` times
    /// if the request fails.
    /// 
    /// # Arguments
    /// 
    /// * `method` - The method name to execute
    /// * `params` - The parameters for the method
    /// 
    /// # Returns
    /// 
    /// A `ResultResponse` containing the task ID and initial status
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::scheduler::RetryClient;
    /// use serde_json::json;
    /// use std::time::Duration;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = RetryClient::new(
    ///     "http://localhost:8080",
    ///     3,
    ///     Duration::from_secs(1)
    /// );
    /// let params = json!({"a": 10, "b": 20});
    /// let response = client.execute_with_retry("add", params).await?;
    /// println!("Task ID: {}", response.task_id);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn execute_with_retry(
        &self,
        method: impl Into<String> + Clone,
        params: Value,
    ) -> Result<ResultResponse> {
        let mut last_error = None;
        
        for attempt in 0..=self.max_retries {
            match self.client.execute(method.clone(), params.clone()).await {
                Ok(response) => return Ok(response),
                Err(e) => {
                    last_error = Some(e);
                    if attempt < self.max_retries {
                        sleep(self.retry_delay).await;
                    }
                }
            }
        }
        
        Err(SdkError::Generic(format!(
            "Failed after {} retries: {}",
            self.max_retries,
            last_error.unwrap()
        )))
    }

    /// Executes an encrypted task with retry logic
    /// 
    /// This method will retry the execution up to `max_retries` times
    /// if the request fails.
    /// 
    /// # Arguments
    /// 
    /// * `method` - The method name to execute
    /// * `key` - The encryption key
    /// * `salt` - The salt value for key encryption
    /// * `params` - The parameters for the method
    /// 
    /// # Returns
    /// 
    /// A `ResultResponse` containing the task ID and initial status
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::scheduler::RetryClient;
    /// use serde_json::json;
    /// use std::time::Duration;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = RetryClient::new(
    ///     "http://localhost:8080",
    ///     3,
    ///     Duration::from_secs(1)
    /// );
    /// let params = json!({"a": 10, "b": 20});
    /// let response = client.execute_encrypted_with_retry(
    ///     "add", 
    ///     "my-secret-key", 
    ///     123456, 
    ///     params
    /// ).await?;
    /// println!("Task ID: {}", response.task_id);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn execute_encrypted_with_retry(
        &self,
        method: impl Into<String> + Clone,
        key: &str,
        salt: i32,
        params: Value,
    ) -> Result<ResultResponse> {
        let mut last_error = None;
        
        for attempt in 0..=self.max_retries {
            match self.client.execute_encrypted(method.clone(), key, salt, params.clone()).await {
                Ok(response) => return Ok(response),
                Err(e) => {
                    last_error = Some(e);
                    if attempt < self.max_retries {
                        sleep(self.retry_delay).await;
                    }
                }
            }
        }
        
        Err(SdkError::Generic(format!(
            "Failed after {} retries: {}",
            self.max_retries,
            last_error.unwrap()
        )))
    }

    /// Executes a task synchronously with retry logic and timeout
    /// 
    /// This is a convenience method that combines retry logic with synchronous execution.
    /// 
    /// # Arguments
    /// 
    /// * `method` - The method name to execute
    /// * `params` - The parameters for the method
    /// * `timeout_duration` - Maximum time to wait for completion
    /// 
    /// # Returns
    /// 
    /// A `ResultResponse` containing the final task result
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::scheduler::RetryClient;
    /// use serde_json::json;
    /// use std::time::Duration;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = RetryClient::new(
    ///     "http://localhost:8080",
    ///     3,
    ///     Duration::from_secs(1)
    /// );
    /// let params = json!({"a": 10, "b": 20});
    /// let result = client.execute_sync_with_retry(
    ///     "add", 
    ///     params, 
    ///     Duration::from_secs(30)
    /// ).await?;
    /// println!("Result: {:?}", result.result);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn execute_sync_with_retry(
        &self,
        method: impl Into<String> + Clone,
        params: Value,
        timeout_duration: Duration,
    ) -> Result<ResultResponse> {
        // Submit task with retry
        let exec_response = self.execute_with_retry(method, params).await?;

        // Get result (this already has built-in polling)
        let result = self.client.get_result(&exec_response.task_id).await?;

        Ok(result)
    }

    /// Executes an encrypted task synchronously with retry logic, decryption and timeout
    /// 
    /// This is a convenience method that combines retry logic with synchronous encrypted execution.
    /// 
    /// # Arguments
    /// 
    /// * `method` - The method name to execute
    /// * `key` - The encryption key
    /// * `salt` - The salt value for key encryption
    /// * `params` - The parameters for the method
    /// * `timeout_duration` - Maximum time to wait for completion
    /// 
    /// # Returns
    /// 
    /// A `ResultResponse` containing the decrypted task result
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::scheduler::RetryClient;
    /// use serde_json::json;
    /// use std::time::Duration;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = RetryClient::new(
    ///     "http://localhost:8080",
    ///     3,
    ///     Duration::from_secs(1)
    /// );
    /// let params = json!({"a": 10, "b": 20});
    /// let result = client.execute_sync_encrypted_with_retry(
    ///     "add", 
    ///     "my-secret-key", 
    ///     123456, 
    ///     params, 
    ///     Duration::from_secs(30)
    /// ).await?;
    /// println!("Decrypted result: {:?}", result.result);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn execute_sync_encrypted_with_retry(
        &self,
        method: impl Into<String> + Clone,
        key: &str,
        salt: i32,
        params: Value,
        timeout_duration: Duration,
    ) -> Result<ResultResponse> {
        // Submit encrypted task with retry
        let exec_response = self.execute_encrypted_with_retry(method, key, salt, params).await?;

        // Get and decrypt result (this already has built-in polling)
        let result = self.client.get_result_encrypted(&exec_response.task_id, key, salt).await?;

        Ok(result)
    }

    /// Get access to the underlying client
    /// 
    /// This allows access to methods that don't need retry logic,
    /// such as `get_result` and `get_result_encrypted`.
    /// 
    /// # Returns
    /// 
    /// A reference to the underlying `Client`
    pub fn client(&self) -> &Client {
        &self.client
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    #[test]
    fn test_retry_client_creation() {
        let client = RetryClient::new(
            "http://localhost:8080",
            3,
            Duration::from_millis(500),
        );
        
        assert_eq!(client.max_retries, 3);
        assert_eq!(client.retry_delay, Duration::from_millis(500));
    }

    #[test]
    fn test_client_access() {
        let retry_client = RetryClient::new(
            "http://localhost:8080",
            3,
            Duration::from_millis(500),
        );
        
        let _client = retry_client.client();
        // Should be able to access the underlying client
    }
}