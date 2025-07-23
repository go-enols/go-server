//! Scheduler client implementation

use reqwest::Client as HttpClient;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::time::Duration;
use tokio::time::{sleep, timeout};
use crate::crypto::{encrypt_data, salt_key, decrypt_data};
use crate::error::{Result, SdkError};
use super::{TASK_STATUS_ERROR, TASK_STATUS_DONE, TASK_STATUS_PENDING, TASK_STATUS_PROCESSING};

/// Scheduler client for executing tasks
#[derive(Clone)]
pub struct Client {
    base_url: String,
    http_client: HttpClient,
}

/// Task execution request
#[derive(Serialize, Debug)]
pub struct ExecuteRequest {
    pub method: String,
    pub params: Value,
}

/// Encrypted task execution request
#[derive(Serialize, Debug)]
pub struct ExecuteEncryptedRequest {
    pub method: String,
    pub params: String,
    pub key: String,
    pub crypto: String,
}

/// Task result response
#[derive(Deserialize, Debug, Clone)]
pub struct ResultResponse {
    #[serde(rename = "taskId")]
    pub task_id: String,
    pub status: String,
    pub result: Option<Value>,
}

impl std::fmt::Display for ResultResponse {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "Task {} (status: {})", self.task_id, self.status)
    }
}

impl Client {
    /// Creates a new scheduler client
    /// 
    /// # Arguments
    /// 
    /// * `base_url` - The base URL of the scheduler server
    /// 
    /// # Example
    /// 
    /// ```rust
    /// use go_server_rust_sdk::scheduler::Client;
    /// 
    /// let client = Client::new("http://localhost:8080");
    /// ```
    pub fn new(base_url: impl Into<String>) -> Self {
        Self {
            base_url: base_url.into(),
            http_client: HttpClient::builder()
                .timeout(Duration::from_secs(30))
                .build()
                .expect("Failed to create HTTP client"),
        }
    }

    /// Executes a task with the given method and parameters
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
    /// use go_server_rust_sdk::scheduler::Client;
    /// use serde_json::json;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = Client::new("http://localhost:8080");
    /// let params = json!({"a": 10, "b": 20});
    /// let response = client.execute("add", params).await?;
    /// println!("Task ID: {}", response.task_id);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn execute(&self, method: impl Into<String>, params: Value) -> Result<ResultResponse> {
        let request = ExecuteRequest {
            method: method.into(),
            params,
        };

        let url = format!("{}/api/execute", self.base_url);
        let response = self.http_client
            .post(&url)
            .json(&request)
            .send()
            .await?
            .error_for_status()?;

        let result: ResultResponse = response.json().await?;
        Ok(result)
    }

    /// Executes an encrypted task with the given method, key, salt and parameters
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
    /// use go_server_rust_sdk::scheduler::Client;
    /// use serde_json::json;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = Client::new("http://localhost:8080");
    /// let params = json!({"a": 10, "b": 20});
    /// let response = client.execute_encrypted("add", "my-secret-key", 123456, params).await?;
    /// println!("Task ID: {}", response.task_id);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn execute_encrypted(
        &self,
        method: impl Into<String>,
        key: &str,
        salt: i32,
        params: Value,
    ) -> Result<ResultResponse> {
        // Encrypt parameters
        let encrypted_params = encrypt_data(&params, key)?;

        // Salt the key
        let salted_key = salt_key(key, salt)?;

        let request = ExecuteEncryptedRequest {
            method: method.into(),
            params: encrypted_params,
            key: salted_key,
            crypto: salt.to_string(),
        };

        let url = format!("{}/api/encrypted/execute", self.base_url);
        let response = self.http_client
            .post(&url)
            .json(&request)
            .send()
            .await?
            .error_for_status()?;

        let result: ResultResponse = response.json().await?;
        Ok(result)
    }

    /// Retrieves the result of a task by its ID with polling
    /// 
    /// This method will automatically poll the server until the task is complete
    /// or an error occurs.
    /// 
    /// # Arguments
    /// 
    /// * `task_id` - The ID of the task to retrieve
    /// 
    /// # Returns
    /// 
    /// A `ResultResponse` containing the final task result
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::scheduler::Client;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = Client::new("http://localhost:8080");
    /// let result = client.get_result("task-123").await?;
    /// println!("Result: {:?}", result.result);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn get_result(&self, task_id: &str) -> Result<ResultResponse> {
        loop {
            let url = format!("{}/api/result/{}", self.base_url, task_id);
            let response = self.http_client
                .get(&url)
                .send()
                .await?
                .error_for_status()?;

            let result: ResultResponse = response.json().await?;

            match result.status.as_str() {
                TASK_STATUS_PENDING | TASK_STATUS_PROCESSING => {
                    sleep(Duration::from_secs(1)).await;
                    continue;
                }
                TASK_STATUS_ERROR => {
                    let error_msg = result.result
                        .as_ref()
                        .and_then(|v| v.as_str())
                        .unwrap_or("Unknown error");
                    return Err(SdkError::Task(error_msg.to_string()));
                }
                _ => return Ok(result),
            }
        }
    }

    /// Retrieves and decrypts the result of an encrypted task by its ID
    /// 
    /// This method will automatically poll the server until the task is complete,
    /// then decrypt the result using the provided key.
    /// 
    /// # Arguments
    /// 
    /// * `task_id` - The ID of the task to retrieve
    /// * `key` - The decryption key
    /// * `salt` - The salt value used for encryption
    /// 
    /// # Returns
    /// 
    /// A `ResultResponse` containing the decrypted task result
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::scheduler::Client;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = Client::new("http://localhost:8080");
    /// let result = client.get_result_encrypted("task-123", "my-secret-key", 123456).await?;
    /// println!("Decrypted result: {:?}", result.result);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn get_result_encrypted(
        &self,
        task_id: &str,
        key: &str,
        _salt: i32,
    ) -> Result<ResultResponse> {
        loop {
            let url = format!("{}/api/encrypted/result/{}", self.base_url, task_id);
            let response = self.http_client
                .get(&url)
                .send()
                .await?
                .error_for_status()?;

            let mut result: ResultResponse = response.json().await?;

            match result.status.as_str() {
                TASK_STATUS_PENDING | TASK_STATUS_PROCESSING => {
                    sleep(Duration::from_secs(1)).await;
                    continue;
                }
                TASK_STATUS_ERROR => {
                    let error_msg = result.result
                        .as_ref()
                        .and_then(|v| v.as_str())
                        .unwrap_or("Unknown error");
                    return Err(SdkError::Task(error_msg.to_string()));
                }
                TASK_STATUS_DONE => {
                    // Decrypt result data if present
                    if let Some(encrypted_result) = &result.result {
                        if let Some(encrypted_str) = encrypted_result.as_str() {
                            let decrypted_result = decrypt_data(encrypted_str, key)?;
                            result.result = Some(decrypted_result);
                        }
                    }
                    return Ok(result);
                }
                _ => return Ok(result),
            }
        }
    }

    /// Executes a task synchronously with polling and timeout
    /// 
    /// This is a convenience method that combines `execute` and `get_result`
    /// with a timeout.
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
    /// use go_server_rust_sdk::scheduler::Client;
    /// use serde_json::json;
    /// use std::time::Duration;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = Client::new("http://localhost:8080");
    /// let params = json!({"a": 10, "b": 20});
    /// let result = client.execute_sync("add", params, Duration::from_secs(30)).await?;
    /// println!("Result: {:?}", result.result);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn execute_sync(
        &self,
        method: impl Into<String>,
        params: Value,
        timeout_duration: Duration,
    ) -> Result<ResultResponse> {
        // Submit task
        let exec_response = self.execute(method, params).await?;

        // Poll for result with timeout
        let result = timeout(timeout_duration, self.get_result(&exec_response.task_id)).await
            .map_err(|_| SdkError::Timeout)??;

        Ok(result)
    }

    /// Executes an encrypted task synchronously with polling, decryption and timeout
    /// 
    /// This is a convenience method that combines `execute_encrypted` and 
    /// `get_result_encrypted` with a timeout.
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
    /// use go_server_rust_sdk::scheduler::Client;
    /// use serde_json::json;
    /// use std::time::Duration;
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// let client = Client::new("http://localhost:8080");
    /// let params = json!({"a": 10, "b": 20});
    /// let result = client.execute_sync_encrypted(
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
    pub async fn execute_sync_encrypted(
        &self,
        method: impl Into<String>,
        key: &str,
        salt: i32,
        params: Value,
        timeout_duration: Duration,
    ) -> Result<ResultResponse> {
        // Submit encrypted task
        let exec_response = self.execute_encrypted(method, key, salt, params).await?;

        // Poll for result with timeout and decryption
        let result = timeout(
            timeout_duration,
            self.get_result_encrypted(&exec_response.task_id, key, salt),
        )
        .await
        .map_err(|_| SdkError::Timeout)??;

        Ok(result)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_client_creation() {
        let client = Client::new("http://localhost:8080");
        assert_eq!(client.base_url, "http://localhost:8080");
    }

    #[test]
    fn test_execute_request_serialization() {
        let request = ExecuteRequest {
            method: "test_method".to_string(),
            params: json!({"key": "value"}),
        };
        
        let serialized = serde_json::to_string(&request).unwrap();
        assert!(serialized.contains("test_method"));
        assert!(serialized.contains("value"));
    }

    #[test]
    fn test_result_response_deserialization() {
        let json_str = r#"{
            "taskId": "123",
            "status": "done",
            "result": {"answer": 42}
        }"#;
        
        let response: ResultResponse = serde_json::from_str(json_str).unwrap();
        assert_eq!(response.task_id, "123");
        assert_eq!(response.status, "done");
        assert!(response.result.is_some());
    }
}