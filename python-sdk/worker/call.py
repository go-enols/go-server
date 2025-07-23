"""Simple call function for worker SDK"""

import os
import sys
import time
from typing import Any, Optional, Type, TypeVar

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from scheduler import SchedulerClient

T = TypeVar("T")


def call(
    host: str,
    method: str,
    params: Any,
    out_type: Optional[Type[T]] = None,
    timeout: float = 30.0,
) -> Optional[T]:
    """Call a remote method on the scheduler

    Args:
        host: Scheduler host URL
        method: Method name to call
        params: Parameters to pass to the method
        out_type: Expected return type (for type hints)
        timeout: Timeout for the operation in seconds

    Returns:
        Result from the method call, optionally typed

    Raises:
        Exception: If the call fails or times out

    Example:
        # Simple call without type checking
        result = call("http://localhost:8080", "add", {"a": 1, "b": 2})

        # Call with type hint
        result: int = call(
            "http://localhost:8080", "add", {"a": 1, "b": 2}, int
        )

        # Call with complex return type
        from typing import Dict
        result: Dict[str, Any] = call(
            "http://localhost:8080", "get_info", {}, dict
        )
    """
    with SchedulerClient(host) as client:
        # Execute the task synchronously
        response = client.execute_sync(method, params, timeout)

        if response.status == "error":
            raise Exception(str(response.result))

        # Return the result
        return response.result


def call_async(host: str, method: str, params: Any) -> str:
    """Call a remote method asynchronously and return task ID

    Args:
        host: Scheduler host URL
        method: Method name to call
        params: Parameters to pass to the method

    Returns:
        Task ID for tracking the async operation

    Raises:
        Exception: If the call submission fails

    Example:
        task_id = call_async(
            "http://localhost:8080", "long_running_task", {"data": "..."}
        )
        # Later, get the result:
        client = SchedulerClient("http://localhost:8080")
        result = client.get_result(task_id)
    """
    with SchedulerClient(host) as client:
        response = client.execute(method, params)
        return response.task_id


def call_encrypted(
    host: str,
    method: str,
    key: str,
    salt: int,
    params: Any,
    out_type: Optional[Type[T]] = None,
    timeout: float = 30.0,
) -> Optional[T]:
    """Call a remote method with encryption

    Args:
        host: Scheduler host URL
        method: Method name to call
        key: Encryption key
        salt: Salt value for encryption
        params: Parameters to pass to the method
        out_type: Expected return type (for type hints)
        timeout: Timeout for the operation in seconds

    Returns:
        Result from the method call, optionally typed

    Raises:
        Exception: If the call fails or times out

    Example:
        result = call_encrypted(
            "http://localhost:8080",
            "secure_add",
            "my_secret_key",
            12345,
            {"a": 1, "b": 2},
        )
    """
    with SchedulerClient(host) as client:
        # Execute the encrypted task
        response = client.execute_encrypted(method, key, salt, params)
        task_id = response.task_id

        # Poll for result with timeout
        start_time = time.time()
        while time.time() - start_time < timeout:
            result = client.get_result(task_id)
            if result.status == "completed":
                return result.result
            elif result.status == "error":
                raise Exception(str(result.result))

            time.sleep(0.1)

        raise TimeoutError(
            f"Encrypted task {task_id} did not complete within " f"{timeout} seconds"
        )


def call_encrypted_async(
    host: str, method: str, key: str, salt: int, params: Any
) -> str:
    """Call a remote method asynchronously with encryption and return task ID

    Args:
        host: Scheduler host URL
        method: Method name to call
        key: Encryption key
        salt: Salt value for encryption
        params: Parameters to pass to the method

    Returns:
        Task ID for tracking the async operation

    Raises:
        Exception: If the call submission fails

    Example:
        task_id = call_encrypted_async(
            "http://localhost:8080",
            "secure_process",
            "my_secret_key",
            12345,
            {"data": "..."},
        )
    """
    with SchedulerClient(host) as client:
        response = client.execute_encrypted(method, key, salt, params)
        return response.task_id


def get_result(
    host: str, task_id: str, out_type: Optional[Type[T]] = None
) -> Optional[T]:
    """Get result for an async task

    Args:
        host: Scheduler host URL
        task_id: Task ID from call_async
        out_type: Expected return type (for type hints)

    Returns:
        Result from the method call, optionally typed

    Raises:
        Exception: If getting the result fails

    Example:
        task_id = call_async(
            "http://localhost:8080", "process_data", {"input": "..."}
        )
        result = get_result("http://localhost:8080", task_id, dict)
    """
    with SchedulerClient(host) as client:
        response = client.get_result(task_id)

        if response.status == "error":
            raise Exception(str(response.result))

        return response.result
