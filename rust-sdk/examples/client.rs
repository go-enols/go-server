//! Client example demonstrating how to call remote methods

use go_server_rust_sdk::worker::call;
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize logger
    env_logger::init();

    println!("Calling remote method 'add' with parameters a=1, b=2");
    
    // Call the remote method
    match call(
        "http://localhost:8080",
        "add",
        json!({"a": 1, "b": 2})
    ).await {
        Ok(result) => {
            println!("Result: {}", result);
        }
        Err(e) => {
            eprintln!("Error calling remote method: {}", e);
        }
    }

    Ok(())
}