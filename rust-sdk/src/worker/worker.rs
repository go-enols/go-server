//! Worker implementation for handling distributed tasks

use futures_util::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::{mpsc, RwLock};
use tokio::time::{interval, sleep, timeout};
use tokio_tungstenite::{connect_async, tungstenite::Message};
use url::Url;
use crate::crypto::{encrypt_data, decrypt_data, unsalt_key};
use crate::error::{Result, SdkError};
use super::{TASK_STATUS_ERROR};

/// Worker configuration
#[derive(Debug, Clone)]
pub struct Config {
    /// Scheduler WebSocket URL
    pub scheduler_url: String,
    /// Worker group name
    pub worker_group: String,
    /// Maximum connection retry attempts
    pub max_retry: usize,
    /// Ping interval in seconds
    pub ping_interval: u64,
}

/// Method handler function type
pub type MethodHandler = Box<dyn Fn(Value) -> Result<Value> + Send + Sync>;

/// Method definition with handler and documentation
#[derive(Clone)]
pub struct Method {
    pub name: String,
    pub handler: Arc<MethodHandler>,
    pub docs: Vec<String>,
}

/// WebSocket message types
#[derive(Deserialize, Debug)]
#[serde(tag = "type")]
enum IncomingMessage {
    #[serde(rename = "task")]
    Task {
        #[serde(rename = "taskId")]
        task_id: String,
        method: String,
        params: Value,
    },
    #[serde(rename = "encrypted_task")]
    EncryptedTask {
        #[serde(rename = "taskId")]
        task_id: String,
        method: String,
        params: Value,
        key: String,
        crypto: String,
    },
    #[serde(rename = "ping")]
    Ping,
}

#[derive(Serialize, Debug)]
#[serde(tag = "type")]
enum OutgoingMessage {
    #[serde(rename = "result")]
    Result {
        #[serde(rename = "taskId")]
        task_id: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        result: Option<Value>,
        #[serde(skip_serializing_if = "Option::is_none")]
        error: Option<String>,
    },
    #[serde(rename = "pong")]
    Pong,
}

#[derive(Serialize, Debug)]
struct RegistrationMessage {
    group: String,
    methods: Vec<MethodInfo>,
}

#[derive(Serialize, Debug)]
struct MethodInfo {
    name: String,
    docs: Vec<String>,
}

/// Distributed task worker
pub struct Worker {
    config: Config,
    methods: Arc<RwLock<HashMap<String, Method>>>,
    running: Arc<RwLock<bool>>,
    shutdown_tx: Option<mpsc::Sender<()>>,
}

impl Worker {
    /// Creates a new worker with the given configuration
    /// 
    /// # Arguments
    /// 
    /// * `config` - Worker configuration
    /// 
    /// # Example
    /// 
    /// ```rust
    /// use go_server_rust_sdk::worker::{Worker, Config};
    /// 
    /// let config = Config {
    ///     scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
    ///     worker_group: "math".to_string(),
    ///     max_retry: 5,
    ///     ping_interval: 5,
    /// };
    /// 
    /// let worker = Worker::new(config);
    /// ```
    pub fn new(config: Config) -> Self {
        Self {
            config,
            methods: Arc::new(RwLock::new(HashMap::new())),
            running: Arc::new(RwLock::new(false)),
            shutdown_tx: None,
        }
    }

    /// Registers a method handler with the worker
    /// 
    /// # Arguments
    /// 
    /// * `name` - Method name
    /// * `handler` - Function that handles the method call
    /// * `docs` - Documentation strings for the method
    /// 
    /// # Example
    /// 
    /// ```rust
    /// use go_server_rust_sdk::worker::{Worker, Config};
    /// use serde_json::{json, Value};
    /// 
    /// # let config = Config {
    /// #     scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
    /// #     worker_group: "math".to_string(),
    /// #     max_retry: 5,
    /// #     ping_interval: 5,
    /// # };
    /// let mut worker = Worker::new(config);
    /// 
    /// worker.register_method("add", |params: Value| {
    ///     let a = params["a"].as_f64().unwrap_or(0.0);
    ///     let b = params["b"].as_f64().unwrap_or(0.0);
    ///     Ok(json!(a + b))
    /// }, vec!["Add two numbers".to_string()]);
    /// ```
    pub fn register_method<F>(&mut self, name: impl Into<String>, handler: F, docs: Vec<String>)
    where
        F: Fn(Value) -> Result<Value> + Send + Sync + 'static,
    {
        let method = Method {
            name: name.into(),
            handler: Arc::new(Box::new(handler)),
            docs,
        };
        
        // We need to use a blocking approach here since this is a sync method
        let methods = self.methods.clone();
        tokio::spawn(async move {
            let mut methods_guard = methods.write().await;
            methods_guard.insert(method.name.clone(), method);
        });
    }

    /// Starts the worker with automatic reconnection support
    /// 
    /// This method will block until the worker is stopped.
    /// 
    /// # Returns
    /// 
    /// A `Result` indicating success or failure
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::worker::{Worker, Config};
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = Config {
    /// #     scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
    /// #     worker_group: "math".to_string(),
    /// #     max_retry: 5,
    /// #     ping_interval: 5,
    /// # };
    /// let mut worker = Worker::new(config);
    /// // Register methods...
    /// worker.start().await?;
    /// # Ok(())
    /// # }
    /// ```
    pub async fn start(&mut self) -> Result<()> {
        *self.running.write().await = true;
        
        let (shutdown_tx, mut shutdown_rx) = mpsc::channel::<()>(1);
        self.shutdown_tx = Some(shutdown_tx);

        log::info!("Worker {} starting", self.config.worker_group);

        loop {
            // Check if we should stop
            if !*self.running.read().await {
                break;
            }

            // Try to connect and run
            match self.connect_and_run(&mut shutdown_rx).await {
                Ok(_) => {
                    log::info!("Worker connection closed normally");
                    break;
                }
                Err(e) => {
                    log::error!("Worker connection failed: {}", e);
                    if *self.running.read().await {
                        log::info!("Retrying connection in 5 seconds...");
                        sleep(Duration::from_secs(5)).await;
                    }
                }
            }
        }

        log::info!("Worker {} stopped", self.config.worker_group);
        Ok(())
    }

    /// Stops the worker
    /// 
    /// This method will signal the worker to stop and close all connections.
    /// 
    /// # Example
    /// 
    /// ```rust,no_run
    /// use go_server_rust_sdk::worker::{Worker, Config};
    /// 
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = Config {
    /// #     scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
    /// #     worker_group: "math".to_string(),
    /// #     max_retry: 5,
    /// #     ping_interval: 5,
    /// # };
    /// let mut worker = Worker::new(config);
    /// 
    /// // In another task or signal handler:
    /// worker.stop().await;
    /// # Ok(())
    /// # }
    /// ```
    pub async fn stop(&mut self) {
        *self.running.write().await = false;
        
        if let Some(tx) = &self.shutdown_tx {
            let _ = tx.send(()).await;
        }
    }

    async fn connect_and_run(&self, shutdown_rx: &mut mpsc::Receiver<()>) -> Result<()> {
        let url = Url::parse(&self.config.scheduler_url)?;
        let (ws_stream, _) = connect_async(url).await?;
        let (mut ws_sender, mut ws_receiver) = ws_stream.split();

        // Send registration message
        let methods = self.get_methods_info().await;
        let registration = RegistrationMessage {
            group: self.config.worker_group.clone(),
            methods,
        };
        
        let registration_msg = serde_json::to_string(&registration)?;
        ws_sender.send(Message::Text(registration_msg)).await?;

        log::info!("Worker {} connected and registered", self.config.worker_group);

        // Start ping interval
        let mut ping_interval = interval(Duration::from_secs(self.config.ping_interval));
        
        loop {
            tokio::select! {
                // Handle shutdown signal
                _ = shutdown_rx.recv() => {
                    log::info!("Received shutdown signal");
                    let _ = ws_sender.close().await;
                    break;
                }
                
                // Handle ping interval
                _ = ping_interval.tick() => {
                    let ping_msg = serde_json::to_string(&OutgoingMessage::Pong)?;
                    if let Err(e) = ws_sender.send(Message::Text(ping_msg)).await {
                        log::error!("Failed to send ping: {}", e);
                        break;
                    }
                }
                
                // Handle incoming messages
                msg = ws_receiver.next() => {
                    match msg {
                        Some(Ok(Message::Text(text))) => {
                            if let Err(e) = self.handle_message(&text, &mut ws_sender).await {
                                log::error!("Error handling message: {}", e);
                            }
                        }
                        Some(Ok(Message::Close(_))) => {
                            log::info!("WebSocket connection closed by server");
                            break;
                        }
                        Some(Err(e)) => {
                            log::error!("WebSocket error: {}", e);
                            break;
                        }
                        None => {
                            log::info!("WebSocket stream ended");
                            break;
                        }
                        _ => {}
                    }
                }
            }
        }

        Ok(())
    }

    async fn handle_message(
        &self,
        text: &str,
        ws_sender: &mut futures_util::stream::SplitSink<
            tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>,
            Message,
        >,
    ) -> Result<()> {
        let message: IncomingMessage = serde_json::from_str(text)?;

        match message {
            IncomingMessage::Task { task_id, method, params } => {
                self.handle_task(task_id, method, params, ws_sender).await?
            }
            IncomingMessage::EncryptedTask { task_id, method, params, key, crypto } => {
                self.handle_encrypted_task(task_id, method, params, key, crypto, ws_sender).await?
            }
            IncomingMessage::Ping => {
                let pong_msg = serde_json::to_string(&OutgoingMessage::Pong)?;
                ws_sender.send(Message::Text(pong_msg)).await?;
            }
        }

        Ok(())
    }

    async fn handle_task(
        &self,
        task_id: String,
        method: String,
        params: Value,
        ws_sender: &mut futures_util::stream::SplitSink<
            tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>,
            Message,
        >,
    ) -> Result<()> {
        let methods = self.methods.read().await;
        let method_handler = methods.get(&method).cloned();
        drop(methods);

        let response = match method_handler {
            Some(handler) => {
                match (handler.handler)(params) {
                    Ok(result) => OutgoingMessage::Result {
                        task_id,
                        result: Some(result),
                        error: None,
                    },
                    Err(e) => OutgoingMessage::Result {
                        task_id,
                        result: None,
                        error: Some(e.to_string()),
                    },
                }
            }
            None => OutgoingMessage::Result {
                task_id,
                result: None,
                error: Some(format!("Method '{}' not found", method)),
            },
        };

        let response_text = serde_json::to_string(&response)?;
        ws_sender.send(Message::Text(response_text)).await?;

        Ok(())
    }

    async fn handle_encrypted_task(
        &self,
        task_id: String,
        method: String,
        encrypted_params: Value,
        salted_key: String,
        crypto: String,
        ws_sender: &mut futures_util::stream::SplitSink<
            tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>,
            Message,
        >,
    ) -> Result<()> {
        let methods = self.methods.read().await;
        let method_handler = methods.get(&method).cloned();
        drop(methods);

        let response = match method_handler {
            Some(handler) => {
                // Decrypt parameters
                match self.decrypt_task_params(encrypted_params, &salted_key, &crypto).await {
                    Ok(params) => {
                        // Execute method
                        match (handler.handler)(params) {
                            Ok(result) => {
                                // Encrypt result
                                match self.encrypt_task_result(result, &salted_key, &crypto).await {
                                    Ok(encrypted_result) => OutgoingMessage::Result {
                                        task_id,
                                        result: Some(encrypted_result),
                                        error: None,
                                    },
                                    Err(e) => OutgoingMessage::Result {
                                        task_id,
                                        result: None,
                                        error: Some(format!("Failed to encrypt result: {}", e)),
                                    },
                                }
                            }
                            Err(e) => OutgoingMessage::Result {
                                task_id,
                                result: None,
                                error: Some(e.to_string()),
                            },
                        }
                    }
                    Err(e) => OutgoingMessage::Result {
                        task_id,
                        result: None,
                        error: Some(format!("Failed to decrypt params: {}", e)),
                    },
                }
            }
            None => OutgoingMessage::Result {
                task_id,
                result: None,
                error: Some(format!("Method '{}' not found", method)),
            },
        };

        let response_text = serde_json::to_string(&response)?;
        ws_sender.send(Message::Text(response_text)).await?;

        Ok(())
    }

    async fn decrypt_task_params(
        &self,
        encrypted_params: Value,
        salted_key: &str,
        crypto: &str,
    ) -> Result<Value> {
        // Extract encrypted string from JSON
        let encrypted_str = encrypted_params
            .as_str()
            .ok_or_else(|| SdkError::Crypto("Invalid encrypted params format".to_string()))?;

        // Parse salt from crypto string
        let salt: i32 = crypto.parse()
            .map_err(|_| SdkError::Crypto("Invalid crypto salt format".to_string()))?;

        // Unsalt the key
        let original_key = unsalt_key(salted_key, salt)?;

        // Decrypt the parameters
        decrypt_data(encrypted_str, &original_key)
    }

    async fn encrypt_task_result(
        &self,
        result: Value,
        salted_key: &str,
        crypto: &str,
    ) -> Result<Value> {
        // Parse salt from crypto string
        let salt: i32 = crypto.parse()
            .map_err(|_| SdkError::Crypto("Invalid crypto salt format".to_string()))?;

        // Unsalt the key
        let original_key = unsalt_key(salted_key, salt)?;

        // Serialize result to JSON string
        let result_str = serde_json::to_string(&result)?;

        // Encrypt the result using the original key
        let encrypted_result = encrypt_data(&Value::String(result_str), &original_key)?;

        Ok(Value::String(encrypted_result))
    }

    async fn get_methods_info(&self) -> Vec<MethodInfo> {
        let methods = self.methods.read().await;
        methods
            .values()
            .map(|method| MethodInfo {
                name: method.name.clone(),
                docs: method.docs.clone(),
            })
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_worker_creation() {
        let config = Config {
            scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
            worker_group: "test".to_string(),
            max_retry: 3,
            ping_interval: 5,
        };

        let worker = Worker::new(config.clone());
        assert_eq!(worker.config.worker_group, "test");
        assert_eq!(worker.config.max_retry, 3);
    }

    #[tokio::test]
    async fn test_method_registration() {
        let config = Config {
            scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
            worker_group: "test".to_string(),
            max_retry: 3,
            ping_interval: 5,
        };

        let mut worker = Worker::new(config);
        
        worker.register_method("test_method", |params: Value| {
            Ok(json!({"received": params}))
        }, vec!["Test method".to_string()]);

        // Give some time for the async registration to complete
        tokio::time::sleep(Duration::from_millis(10)).await;

        let methods = worker.methods.read().await;
        assert!(methods.contains_key("test_method"));
    }
}