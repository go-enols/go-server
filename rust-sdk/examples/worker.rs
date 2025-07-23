//! Worker example demonstrating how to create and run a distributed task worker

use go_server_rust_sdk::worker::{Worker, Config};
use serde_json::{json, Value};
use std::error::Error;

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    // Initialize logger
    env_logger::init();

    // Create worker configuration
    let config = Config {
        scheduler_url: "ws://localhost:8080/api/worker/connect/123456".to_string(),
        worker_group: "math".to_string(),
        max_retry: 5,
        ping_interval: 5,
    };

    // Create worker
    let mut worker = Worker::new(config);

    // Register the "add" method
    worker.register_method(
        "add",
        |params: Value| {
            println!("Received add request: {}", params);
            
            // Extract parameters
            let a = params["a"].as_f64().unwrap_or(0.0);
            let b = params["b"].as_f64().unwrap_or(0.0);
            
            // Perform calculation
            let result = a + b;
            
            println!("Calculated: {} + {} = {}", a, b, result);
            
            // Return result
            Ok(json!(result))
        },
        vec![
            "Add two numbers".to_string(),
            "Parameters: {a: number, b: number}".to_string(),
            "Returns: number".to_string(),
        ],
    );

    // Register the "multiply" method
    worker.register_method(
        "multiply",
        |params: Value| {
            println!("Received multiply request: {}", params);
            
            // Extract parameters
            let a = params["a"].as_f64().unwrap_or(0.0);
            let b = params["b"].as_f64().unwrap_or(0.0);
            
            // Perform calculation
            let result = a * b;
            
            println!("Calculated: {} * {} = {}", a, b, result);
            
            // Return result
            Ok(json!(result))
        },
        vec![
            "Multiply two numbers".to_string(),
            "Parameters: {a: number, b: number}".to_string(),
            "Returns: number".to_string(),
        ],
    );

    // Register a more complex method that demonstrates error handling
    worker.register_method(
        "divide",
        |params: Value| {
            println!("Received divide request: {}", params);
            
            // Extract parameters
            let a = params["a"].as_f64().unwrap_or(0.0);
            let b = params["b"].as_f64().unwrap_or(0.0);
            
            // Check for division by zero
            if b == 0.0 {
                return Err(go_server_rust_sdk::error::SdkError::Generic(
                    "Division by zero is not allowed".to_string()
                ));
            }
            
            // Perform calculation
            let result = a / b;
            
            println!("Calculated: {} / {} = {}", a, b, result);
            
            // Return result
            Ok(json!(result))
        },
        vec![
            "Divide two numbers".to_string(),
            "Parameters: {a: number, b: number}".to_string(),
            "Returns: number".to_string(),
            "Throws error if b is zero".to_string(),
        ],
    );

    println!("Starting worker with group: math");
    println!("Registered methods: add, multiply, divide");
    println!("Press Ctrl+C to stop the worker");

    // Set up graceful shutdown
    let worker_clone = std::sync::Arc::new(tokio::sync::Mutex::new(worker));
    let worker_for_signal = worker_clone.clone();
    
    // Handle Ctrl+C
    tokio::spawn(async move {
        tokio::signal::ctrl_c().await.expect("Failed to listen for ctrl+c");
        println!("\nReceived Ctrl+C, shutting down worker...");
        let mut worker = worker_for_signal.lock().await;
        worker.stop().await;
    });

    // Start the worker
    let mut worker = worker_clone.lock().await;
    if let Err(e) = worker.start().await {
        eprintln!("Worker failed: {}", e);
    }

    println!("Worker stopped");
    Ok(())
}