#!/usr/bin/env python3
"""Example client usage for go-server Python SDK"""

import os
import sys

# Add the parent directory to the path so we can import the SDK
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import time

from scheduler import RetryClient, SchedulerClient
from worker import call, call_async, get_result


def main():
    scheduler_url = "http://localhost:8080"

    print("=== Go-Server Python SDK Client Examples ===")
    print(f"Connecting to scheduler: {scheduler_url}")
    print()

    # Example 1: Basic client usage
    print("1. Basic SchedulerClient Usage")
    print("-" * 40)

    try:
        with SchedulerClient(scheduler_url) as client:
            # Synchronous execution
            print("Executing 'add' method synchronously...")
            result = client.execute_sync(
                "add", {"a": 10, "b": 20}, timeout=30.0
            )
            print(f"Result: {result.result}")
            print(f"Status: {result.status}")
            print()

            # Asynchronous execution
            print("Executing 'multiply' method asynchronously...")
            response = client.execute("multiply", {"a": 5, "b": 6})
            print(f"Task ID: {response.task_id}")

            # Get result
            result = client.get_result(response.task_id)
            print(f"Async result: {result.result}")
            print()

    except Exception as e:
        print(f"Error with basic client: {e}")
        print()

    # Example 2: Retry client usage
    print("2. RetryClient Usage")
    print("-" * 40)

    try:
        with RetryClient(
            scheduler_url, max_retries=3, retry_delay=1.0
        ) as retry_client:
            print("Executing with retry client...")
            result = retry_client.execute_with_retry(
                "add", {"a": 100, "b": 200}
            )
            print(f"Task ID: {result.task_id}")

            # Get final result
            final_result = retry_client.get_result(result.task_id)
            print(f"Final result: {final_result.result}")
            print()

    except Exception as e:
        print(f"Error with retry client: {e}")
        print()

    # Example 3: Simple call function
    print("3. Simple Call Function")
    print("-" * 40)

    try:
        # Direct synchronous call
        print("Using simple call function...")
        result = call(scheduler_url, "add", {"a": 7, "b": 8})
        print(f"Simple call result: {result}")

        # Type-hinted call
        result_typed: int | None = call(
            scheduler_url, "multiply", {"a": 3, "b": 4}, int
        )
        print(f"Typed call result: {result_typed}")
        print()

    except Exception as e:
        print(f"Error with simple call: {e}")
        print()

    # Example 4: Async call functions
    print("4. Async Call Functions")
    print("-" * 40)

    try:
        # Submit async task
        print("Submitting async task...")
        task_id = call_async(scheduler_url, "add", {"a": 15, "b": 25})
        print(f"Submitted task: {task_id}")

        # Simulate some other work
        print("Doing other work...")
        time.sleep(1)

        # Get the result
        print("Getting async result...")
        result = get_result(scheduler_url, task_id)
        print(f"Async result: {result}")
        print()

    except Exception as e:
        print(f"Error with async calls: {e}")
        print()

    # Example 5: Error handling
    print("5. Error Handling")
    print("-" * 40)

    try:
        with SchedulerClient(scheduler_url) as client:
            # Try to call a non-existent method
            print("Calling non-existent method...")
            result = client.execute_sync(
                "nonexistent_method", {}, timeout=5.0
            )
            print(f"Unexpected success: {result.result}")

    except Exception as e:
        print(f"Expected error: {e}")
        print("Error handling working correctly!")
        print()

    # Example 6: Long-running task
    print("6. Long-running Task")
    print("-" * 40)

    try:
        print("Starting long-running task...")
        task_id = call_async(scheduler_url, "long_task", {"duration": 3})
        print(f"Long task submitted: {task_id}")

        # Poll for completion
        print("Waiting for completion...")
        start_time = time.time()
        result = get_result(scheduler_url, task_id)
        elapsed = time.time() - start_time

        print(f"Long task completed in {elapsed:.1f}s")
        print(f"Result: {result}")
        print()

    except Exception as e:
        print(f"Error with long-running task: {e}")
        print()

    print("=== All examples completed ===")


if __name__ == "__main__":
    main()
