# Go-Server Python SDK

Python SDK for the go-server distributed task scheduling system. This SDK provides both client and worker libraries for interacting with the go-server scheduler.

## Features

- 🚀 **Easy Integration**: Simple APIs for both clients and workers
- 🔄 **Automatic Retry**: Built-in retry mechanisms for robust operation
- 📡 **WebSocket Support**: Real-time communication with the scheduler
- 🛡️ **Type Safety**: Full type hints support for better development experience
- 📚 **Method Documentation**: Support for documenting registered methods
- ⚖️ **Load Balancing**: Automatic task distribution across workers

## Prerequisites

### Starting the Scheduler

**Important**: Before using this Python SDK, you must have the go-server scheduler running. The scheduler is the core component that manages and distributes tasks to workers.

#### Quick Start with go-server

1. **Clone the go-server repository**:
   ```bash
   git clone https://github.com/go-enols/go-server.git
   cd go-server
   ```

2. **Build and run the scheduler**:
   ```bash
   # Build the project
   go build -o scheduler ./go-sdk/examples/scheduler/scheduler
   
   # Run the scheduler (default port: 8080)
   ./scheduler
   ```

For detailed configuration options and advanced setup, please refer to the [go-server documentation](https://github.com/go-enols/go-server).

**Note**: The scheduler must be running and accessible before you can use any of the SDK features. All examples in this documentation assume the scheduler is running on `http://localhost:8080`.

## Installation

install from requirements.txt:

```bash
pip install -r requirements.txt
```

install from source:

```bash
python setup.py install
```

## Quick Start

### Client Usage

```python
from python_sdk.scheduler import SchedulerClient

# Create a client
client = SchedulerClient("http://localhost:8080")

# Execute a task synchronously
result = client.execute_sync("add", {"a": 1, "b": 2}, timeout=30.0)
print(f"Result: {result.result}")  # Output: Result: 3

# Execute a task asynchronously
response = client.execute("add", {"a": 5, "b": 3})
task_id = response.task_id

# Get the result later
result = client.get_result(task_id)
print(f"Async result: {result.result}")  # Output: Async result: 8
```

### Worker Usage

```python
from python_sdk.worker import Worker, Config
import time

# Define your methods
def add_numbers(params):
    """Add two numbers"""
    return params["a"] + params["b"]

def multiply_numbers(params):
    """Multiply two numbers"""
    return params["a"] * params["b"]

def long_running_task(params):
    """Simulate a long-running task"""
    time.sleep(params.get("duration", 5))
    return {"status": "completed", "message": "Task finished"}

# Create worker configuration
config = Config(
    scheduler_url="http://localhost:8080",
    worker_group="math_workers",
    max_retry=3,
    ping_interval=30
)

# Create and configure worker
worker = Worker(config)

# Register methods with documentation
worker.register_method(
    "add", 
    add_numbers,
    "Add two numbers",
    "Parameters: {\"a\": number, \"b\": number}",
    "Returns: number"
)

worker.register_method(
    "multiply", 
    multiply_numbers,
    "Multiply two numbers",
    "Parameters: {\"a\": number, \"b\": number}",
    "Returns: number"
)

worker.register_method(
    "long_task",
    long_running_task,
    "Execute a long-running task",
    "Parameters: {\"duration\": number (optional, default: 5)}",
    "Returns: {\"status\": string, \"message\": string}"
)

# Start the worker
try:
    worker.start()
    print("Worker started. Press Ctrl+C to stop.")
    
    # Keep the worker running
    import signal
    import sys
    
    def signal_handler(sig, frame):
        print("\nStopping worker...")
        worker.stop()
        sys.exit(0)
    
    signal.signal(signal.SIGINT, signal_handler)
    signal.pause()
    
except Exception as e:
    print(f"Error starting worker: {e}")
    worker.stop()
```

### Simple Call Function

For quick one-off calls, you can use the simple `call` function:

```python
from python_sdk.worker import call

# Simple synchronous call
result = call("http://localhost:8080", "add", {"a": 1, "b": 2})
print(f"Result: {result}")  # Output: Result: 3

# Call with type hint
result: int = call("http://localhost:8080", "add", {"a": 1, "b": 2}, int)

# Async call
from python_sdk.worker import call_async, get_result

task_id = call_async("http://localhost:8080", "long_task", {"duration": 10})
print(f"Task submitted: {task_id}")

# Get result later
result = get_result("http://localhost:8080", task_id)
print(f"Task result: {result}")
```

### Retry Client

For more robust operation, use the retry client:

```python
from python_sdk.scheduler import RetryClient

# Create retry client with custom settings
client = RetryClient(
    base_url="http://localhost:8080",
    max_retries=5,
    retry_delay=2.0,
    timeout=30
)

# Execute with automatic retry
result = client.execute_with_retry("add", {"a": 1, "b": 2})
print(f"Result: {result.result}")

# Synchronous execution with retry
result = client.execute_sync_with_retry("multiply", {"a": 3, "b": 4}, timeout=60.0)
print(f"Result: {result.result}")
```

## API Reference

### SchedulerClient

- `execute(method, params)`: Execute a task asynchronously
- `get_result(task_id)`: Get result for a task (with polling)
- `execute_sync(method, params, timeout)`: Execute a task synchronously

### RetryClient

- `execute_with_retry(method, params)`: Execute with automatic retry
- `execute_sync_with_retry(method, params, timeout)`: Synchronous execution with retry

### Worker

- `register_method(name, handler, *docs)`: Register a method handler
- `start()`: Start the worker
- `stop()`: Stop the worker

### Configuration

```python
config = Config(
    scheduler_url="http://localhost:8080",  # Scheduler URL
    worker_group="my_workers",              # Worker group name
    max_retry=3,                            # Connection retry attempts
    ping_interval=30                        # Heartbeat interval (seconds)
)
```

## Error Handling

The SDK provides comprehensive error handling:

```python
from python_sdk.scheduler import SchedulerClient
import requests

client = SchedulerClient("http://localhost:8080")

try:
    result = client.execute_sync("nonexistent_method", {})
except requests.RequestException as e:
    print(f"Network error: {e}")
except ValueError as e:
    print(f"Invalid response: {e}")
except RuntimeError as e:
    print(f"Task execution error: {e}")
except TimeoutError as e:
    print(f"Timeout: {e}")
```

## Development

### Running Tests

```bash
pip install -e ".[dev]"
pytest
```

### Code Formatting

```bash
black python_sdk/
flake8 python_sdk/
mypy python_sdk/
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.