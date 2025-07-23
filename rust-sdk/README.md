# Go Server Rust SDK

A Rust implementation of the Go Server SDK for distributed task processing.

## Features

- **Scheduler Client**: Execute tasks on remote workers with support for synchronous and asynchronous execution
- **Worker**: Create distributed task workers that can handle method calls
- **Encryption**: Built-in AES-GCM encryption support for secure task execution
- **Retry Logic**: Automatic retry mechanisms for improved reliability
- **WebSocket Support**: Real-time communication between workers and scheduler

## Installation

Add this to your `Cargo.toml`:

```toml
[dependencies]
go-server-rust-sdk = "0.1.0"
tokio = { version = "1.0", features = ["full"] }
serde_json = "1.0"
```

## Quick Start

### Client Usage

```rust
use go_server_rust_sdk::worker::call;
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Call a remote method
    let result = call(
        "http://localhost:8080",
        "add",
        json!({"a": 1, "b": 2})
    ).await?;
    
    println!("Result: {}", result);
    Ok(())
}
```

### Worker Usage

```rust
use go_server_rust_sdk::worker::{Worker, Config};
use serde_json::{json, Value};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let config = Config {
        scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
        worker_group: "math".to_string(),
        max_retry: 5,
        ping_interval: 5,
    };
    
    let mut worker = Worker::new(config);
    
    // Register a method
    worker.register_method("add", |params: Value| {
        let a = params["a"].as_f64().unwrap_or(0.0);
        let b = params["b"].as_f64().unwrap_or(0.0);
        Ok(json!(a + b))
    }, vec!["Add two numbers".to_string()]);
    
    // Start the worker
    worker.start().await?;
    Ok(())
}
```

### Scheduler Client Usage

```rust
use go_server_rust_sdk::scheduler::Client;
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new("http://localhost:8080".to_string());
    
    // Synchronous execution
    let result = client.execute_sync("add", json!({"a": 5, "b": 3})).await?;
    println!("5 + 3 = {}", result);
    
    // Asynchronous execution
    let task_id = client.execute("multiply", json!({"a": 4, "b": 7})).await?;
    let result = client.get_result(&task_id).await?;
    if let Some(result) = result {
        println!("4 * 7 = {}", result);
    }
    
    // Encrypted execution
    let result = client.execute_sync_encrypted(
        "add", 
        json!({"a": 10, "b": 15}), 
        "my-secret-key"
    ).await?;
    println!("10 + 15 = {} (encrypted)", result);
    
    Ok(())
}
```

## API Reference

### Scheduler Client

The `Client` struct provides methods for executing tasks on the scheduler:

- `execute(method, params)` - Execute a task asynchronously
- `execute_sync(method, params)` - Execute a task synchronously with polling
- `execute_encrypted(method, params, key)` - Execute an encrypted task asynchronously
- `execute_sync_encrypted(method, params, key)` - Execute an encrypted task synchronously
- `get_result(task_id)` - Get the result of a task
- `get_result_encrypted(task_id, key)` - Get the encrypted result of a task

### Retry Client

The `RetryClient` wraps the base client with automatic retry functionality:

- `execute_with_retry(method, params)` - Execute with retry logic
- `execute_sync_with_retry(method, params)` - Synchronous execution with retry
- `execute_encrypted_with_retry(method, params, key)` - Encrypted execution with retry
- `execute_sync_encrypted_with_retry(method, params, key)` - Encrypted synchronous execution with retry

### Worker

The `Worker` struct allows you to create distributed task workers:

- `new(config)` - Create a new worker
- `register_method(name, handler, docs)` - Register a method handler
- `start()` - Start the worker (blocks until stopped)
- `stop()` - Stop the worker

### Helper Functions

- `call(scheduler_url, method, params)` - Simple function to call a remote method
- `call_encrypted(scheduler_url, method, params, key)` - Simple function to call an encrypted remote method

## Examples

See the `examples/` directory for complete examples:

- `client.rs` - Simple client example
- `worker.rs` - Worker with multiple methods
- `scheduler.rs` - Advanced scheduler client usage

## Error Handling

The SDK uses a comprehensive error system with the `SdkError` enum that covers:

- HTTP errors
- WebSocket errors
- JSON serialization/deserialization errors
- Cryptographic errors
- Task execution errors
- Connection errors
- Timeout errors

## Encryption

The SDK supports AES-GCM encryption for secure task execution. When using encrypted methods, both the task parameters and results are encrypted using the provided key.

## Compatibility

This Rust SDK is fully compatible with the original Go SDK and can interoperate with Go-based workers and clients.

## License

MIT License - see LICENSE file for details.