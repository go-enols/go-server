//! Scheduler client example demonstrating various ways to execute tasks

use go_server_rust_sdk::scheduler::{Client, RetryClient};
use serde_json::json;
use std::error::Error;

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    // Initialize logger
    env_logger::init();

    let scheduler_url = "http://localhost:8080";
    
    println!("=== Scheduler Client Examples ===");
    
    // Example 1: Basic synchronous execution
    println!("\n1. Basic synchronous execution:");
    let client = Client::new(scheduler_url.to_string());
    
    match client.execute_sync("add", json!({"a": 5, "b": 3}), std::time::Duration::from_secs(30)).await {
        Ok(result) => {
            if let Some(value) = result.result {
                println!("   5 + 3 = {}", value);
            } else {
                println!("   No result returned");
            }
        },
        Err(e) => eprintln!("   Error: {}", e),
    }
    
    // Example 2: Asynchronous execution with manual result polling
    println!("\n2. Asynchronous execution with manual polling:");
    match client.execute("multiply", json!({"a": 4, "b": 7})).await {
        Ok(response) => {
            println!("   Task submitted with ID: {}", response.task_id);
            
            // Poll for result
            loop {
                match client.get_result(&response.task_id).await {
                    Ok(result) => {
                        match result.status.as_str() {
                            "done" => {
                                if let Some(value) = result.result {
                                    println!("   4 * 7 = {}", value);
                                } else {
                                    println!("   No result returned");
                                }
                                break;
                            },
                            "error" => {
                                eprintln!("   Task failed");
                                break;
                            },
                            _ => {
                                println!("   Task still running, waiting...");
                                tokio::time::sleep(tokio::time::Duration::from_millis(500)).await;
                            }
                        }
                    }
                    Err(e) => {
                        eprintln!("   Error getting result: {}", e);
                        break;
                    }
                }
            }
        }
        Err(e) => eprintln!("   Error submitting task: {}", e),
    }
    
    // Example 3: Encrypted execution
    println!("\n3. Encrypted synchronous execution:");
    let encryption_key = "my-secret-key-123";
    
    match client.execute_sync_encrypted("add", encryption_key, rand::random::<i32>(), json!({"a": 10, "b": 15}), std::time::Duration::from_secs(30)).await {
        Ok(result) => {
            if let Some(value) = result.result {
                println!("   10 + 15 = {} (encrypted)", value);
            } else {
                println!("   No result returned");
            }
        },
        Err(e) => eprintln!("   Error: {}", e),
    }
    
    // Example 4: Using RetryClient for automatic retries
    println!("\n4. Using RetryClient with automatic retries:");
    let retry_client = RetryClient::new(scheduler_url.to_string(), 3, std::time::Duration::from_millis(1000));
    
    match retry_client.execute_sync_with_retry("divide", json!({"a": 20, "b": 4}), std::time::Duration::from_secs(30)).await {
        Ok(result) => {
            if let Some(value) = result.result {
                println!("   20 / 4 = {} (with retry support)", value);
            } else {
                println!("   No result returned");
            }
        },
        Err(e) => eprintln!("   Error: {}", e),
    }
    
    // Example 5: Error handling - division by zero
    println!("\n5. Error handling example (division by zero):");
    match client.execute_sync("divide", json!({"a": 10, "b": 0}), std::time::Duration::from_secs(30)).await {
        Ok(result) => {
            if let Some(value) = result.result {
                println!("   Unexpected result: {}", value);
            } else {
                println!("   No result returned");
            }
        },
        Err(e) => println!("   Expected error: {}", e),
    }
    
    // Example 6: Encrypted execution with RetryClient
    println!("\n6. Encrypted execution with RetryClient:");
    match retry_client.execute_sync_encrypted_with_retry(
        "multiply", 
        encryption_key,
        rand::random::<i32>(),
        json!({"a": 6, "b": 9}), 
        std::time::Duration::from_secs(30)
    ).await {
        Ok(result) => {
            if let Some(value) = result.result {
                println!("   6 * 9 = {} (encrypted with retry)", value);
            } else {
                println!("   No result returned");
            }
        },
        Err(e) => eprintln!("   Error: {}", e),
    }
    
    // Example 7: Batch operations
    println!("\n7. Batch operations:");
    let operations = vec![
        ("add", json!({"a": 1, "b": 2})),
        ("multiply", json!({"a": 3, "b": 4})),
        ("add", json!({"a": 5, "b": 6})),
    ];
    
    let mut task_ids = Vec::new();
    
    // Submit all tasks
    for (method, params) in &operations {
        match client.execute(*method, params.clone()).await {
            Ok(response) => {
                println!("   Submitted {}: {}", method, response.task_id);
                task_ids.push(response.task_id);
            }
            Err(e) => eprintln!("   Error submitting {}: {}", method, e),
        }
    }
    
    // Collect all results
    for (i, task_id) in task_ids.iter().enumerate() {
        match client.get_result(task_id).await {
            Ok(result) => {
                match result.status.as_str() {
                    "done" => {
                        let (method, params) = &operations[i];
                        if let Some(value) = result.result {
                            println!("   {} with {} = {}", method, params, value);
                        } else {
                            println!("   {} with {} = No result", method, params);
                        }
                    },
                    "error" => {
                        eprintln!("   Task {} failed", task_id);
                    },
                    _ => {
                        println!("   Task {} still running", task_id);
                    }
                }
            }
            Err(e) => eprintln!("   Error getting result for {}: {}", task_id, e),
        }
    }
    
    println!("\n=== Examples completed ===");
    Ok(())
}