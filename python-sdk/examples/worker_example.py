#!/usr/bin/env python3
"""Example worker usage for go-server Python SDK"""

import json
import logging
import os
import random
import signal
import sys
import time
from typing import Any, Dict

# Add the parent directory to the path so we can import the SDK
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from worker import Config, Worker

# Setup logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


# Example method implementations
def add_numbers(params: Dict[str, Any]) -> int:
    """Add two numbers"""
    logger.info(f"Adding {params['a']} + {params['b']}")
    return params["a"] + params["b"]


def multiply_numbers(params: Dict[str, Any]) -> int:
    """Multiply two numbers"""
    logger.info(f"Multiplying {params['a']} * {params['b']}")
    return params["a"] * params["b"]


def subtract_numbers(params: Dict[str, Any]) -> int:
    """Subtract two numbers"""
    logger.info(f"Subtracting {params['a']} - {params['b']}")
    return params["a"] - params["b"]


def divide_numbers(params: Dict[str, Any]) -> float:
    """Divide two numbers"""
    logger.info(f"Dividing {params['a']} / {params['b']}")
    if params["b"] == 0:
        raise ValueError("Division by zero is not allowed")
    return params["a"] / params["b"]


def power_numbers(params: Dict[str, Any]) -> float:
    """Raise a number to a power"""
    base = params["base"]
    exponent = params["exponent"]
    logger.info(f"Calculating {base} ^ {exponent}")
    return base**exponent


def factorial(params: Dict[str, Any]) -> int:
    """Calculate factorial of a number"""
    n = params["n"]
    logger.info(f"Calculating factorial of {n}")

    if n < 0:
        raise ValueError("Factorial is not defined for negative numbers")
    if n > 20:
        raise ValueError("Factorial calculation limited to n <= 20")

    result = 1
    for i in range(1, n + 1):
        result *= i

    return result


def fibonacci(params: Dict[str, Any]) -> int:
    """Calculate nth Fibonacci number"""
    n = params["n"]
    logger.info(f"Calculating {n}th Fibonacci number")

    if n < 0:
        raise ValueError("Fibonacci is not defined for negative numbers")
    if n > 50:
        raise ValueError("Fibonacci calculation limited to n <= 50")

    if n <= 1:
        return n

    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b

    return b


def long_running_task(params: Dict[str, Any]) -> Dict[str, Any]:
    """Simulate a long-running task"""
    duration = params.get("duration", 5)
    task_name = params.get("name", "unnamed_task")

    logger.info(f"Starting long-running task '{task_name}' for {duration} seconds")

    # Simulate work with progress updates
    for i in range(duration):
        time.sleep(1)
        progress = ((i + 1) / duration) * 100
        logger.info(f"Task '{task_name}' progress: {progress:.1f}%")

    result = {
        "status": "completed",
        "task_name": task_name,
        "duration": duration,
        "message": f"Task '{task_name}' completed successfully",
        "timestamp": time.time(),
    }

    logger.info(f"Task '{task_name}' completed")
    return result


def random_number(params: Dict[str, Any]) -> Dict[str, Any]:
    """Generate random numbers"""
    min_val = params.get("min", 0)
    max_val = params.get("max", 100)
    count = params.get("count", 1)

    logger.info(f"Generating {count} random numbers between {min_val} and {max_val}")

    if count > 1000:
        raise ValueError("Cannot generate more than 1000 random numbers")

    # Using random for demo purposes only, not for cryptographic use
    numbers = [random.randint(min_val, max_val) for _ in range(count)]  # nosec B311

    return {
        "numbers": numbers,
        "count": len(numbers),
        "min": min(numbers),
        "max": max(numbers),
        "average": sum(numbers) / len(numbers),
    }


def process_text(params: Dict[str, Any]) -> Dict[str, Any]:
    """Process text data"""
    text = params["text"]
    operation = params.get("operation", "analyze")

    logger.info(f"Processing text with operation: {operation}")

    result = {"original_text": text, "operation": operation}

    if operation == "analyze":
        result.update(
            {
                "length": len(text),
                "word_count": len(text.split()),
                "character_count": len(text),
                "uppercase_count": sum(1 for c in text if c.isupper()),
                "lowercase_count": sum(1 for c in text if c.islower()),
                "digit_count": sum(1 for c in text if c.isdigit()),
            }
        )
    elif operation == "uppercase":
        result["processed_text"] = text.upper()
    elif operation == "lowercase":
        result["processed_text"] = text.lower()
    elif operation == "reverse":
        result["processed_text"] = text[::-1]
    else:
        raise ValueError(f"Unknown operation: {operation}")

    return result


def main():
    """Main worker function"""
    scheduler_url = "http://localhost:8080"
    worker_group = "python_math_workers"

    print("=== Go-Server Python SDK Worker Example ===")
    print(f"Scheduler URL: {scheduler_url}")
    print(f"Worker Group: {worker_group}")
    print()

    # Create worker configuration
    config = Config(
        scheduler_url=scheduler_url,
        worker_group=worker_group,
        max_retry=3,
        ping_interval=30,
    )

    # Create worker instance
    worker = Worker(config)

    # Register methods with documentation
    print("Registering methods...")

    worker.register_method(
        "add",
        add_numbers,
        "Add two numbers",
        'Parameters: {"a": number, "b": number}',
        "Returns: number (sum of a and b)",
    )

    worker.register_method(
        "multiply",
        multiply_numbers,
        "Multiply two numbers",
        'Parameters: {"a": number, "b": number}',
        "Returns: number (product of a and b)",
    )

    worker.register_method(
        "subtract",
        subtract_numbers,
        "Subtract two numbers",
        'Parameters: {"a": number, "b": number}',
        "Returns: number (a minus b)",
    )

    worker.register_method(
        "divide",
        divide_numbers,
        "Divide two numbers",
        'Parameters: {"a": number, "b": number}',
        "Returns: number (a divided by b)",
        "Raises: ValueError if b is zero",
    )

    worker.register_method(
        "power",
        power_numbers,
        "Raise a number to a power",
        'Parameters: {"base": number, "exponent": number}',
        "Returns: number (base raised to exponent)",
    )

    worker.register_method(
        "factorial",
        factorial,
        "Calculate factorial of a number",
        'Parameters: {"n": integer (0 <= n <= 20)}',
        "Returns: integer (n!)",
        "Raises: ValueError for negative numbers or n > 20",
    )

    worker.register_method(
        "fibonacci",
        fibonacci,
        "Calculate nth Fibonacci number",
        'Parameters: {"n": integer (0 <= n <= 50)}',
        "Returns: integer (nth Fibonacci number)",
        "Raises: ValueError for negative numbers or n > 50",
    )

    worker.register_method(
        "long_task",
        long_running_task,
        "Execute a long-running task",
        'Parameters: {"duration": integer (seconds), "name": string (optional)}',
        "Returns: object with task completion details",
    )

    worker.register_method(
        "random",
        random_number,
        "Generate random numbers",
        'Parameters: {"min": integer, "max": integer, "count": integer (optional, max 1000)}',
        "Returns: object with generated numbers and statistics",
    )

    worker.register_method(
        "text",
        process_text,
        "Process text data",
        'Parameters: {"text": string, "operation": string (analyze|uppercase|lowercase|reverse)}',
        "Returns: object with processed text or analysis results",
    )

    print(f"Registered {len(worker.methods)} methods")
    print()

    # Setup signal handler for graceful shutdown
    def signal_handler(sig, frame):
        print("\nReceived shutdown signal...")
        print("Stopping worker gracefully...")
        worker.stop()
        print("Worker stopped.")
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Start the worker
    try:
        print("Starting worker...")
        worker.start()
        print("Worker started successfully!")
        print("Available methods:")
        for method_name in worker.methods.keys():
            print(f"  - {method_name}")
        print()
        print("Worker is ready to process tasks.")
        print("Press Ctrl+C to stop the worker.")
        print()

        # Keep the worker running
        while True:
            time.sleep(1)

    except KeyboardInterrupt:
        print("\nKeyboard interrupt received.")
    except Exception as e:
        print(f"Error starting worker: {e}")
        logger.exception("Worker startup failed")
    finally:
        print("Cleaning up...")
        worker.stop()
        print("Worker stopped.")


if __name__ == "__main__":
    main()
