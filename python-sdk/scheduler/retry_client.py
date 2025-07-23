"""Retry client for scheduler SDK"""

import time
from typing import Any

from .client import ResultResponse, SchedulerClient


class RetryClient:
    """Scheduler client with automatic retry functionality"""

    def __init__(
        self,
        base_url: str,
        max_retries: int = 3,
        retry_delay: float = 1.0,
        timeout: int = 30,
    ):
        """Initialize retry client

        Args:
            base_url: Base URL of the scheduler
            max_retries: Maximum number of retry attempts
            retry_delay: Delay between retries in seconds
            timeout: HTTP request timeout in seconds
        """
        self.client = SchedulerClient(base_url, timeout)
        self.max_retries = max_retries
        self.retry_delay = retry_delay

    def execute_with_retry(self, method: str, params: Any) -> ResultResponse:
        """Execute task with automatic retry on failure

        Args:
            method: Method name to execute
            params: Parameters for the method

        Returns:
            ResultResponse with task ID and initial status

        Raises:
            Exception: If all retry attempts fail
        """
        last_error = None

        for attempt in range(self.max_retries):
            try:
                return self.client.execute(method, params)
            except Exception as e:
                last_error = e
                if attempt < self.max_retries - 1:  # Don't sleep on last attempt
                    time.sleep(self.retry_delay)

        raise Exception(f"After {self.max_retries} retries: {last_error}")

    def execute_encrypted_with_retry(
        self, method: str, key: str, salt: int, params: Any
    ) -> ResultResponse:
        """Execute encrypted task with automatic retry on failure

        Args:
            method: Method name to execute
            key: Encryption key
            salt: Salt value for encryption
            params: Parameters for the method

        Returns:
            ResultResponse with task ID and initial status

        Raises:
            Exception: If all retry attempts fail
        """
        last_error = None

        for attempt in range(self.max_retries):
            try:
                return self.client.execute_encrypted(method, key, salt, params)
            except Exception as e:
                last_error = e
                if attempt < self.max_retries - 1:  # Don't sleep on last attempt
                    time.sleep(self.retry_delay)

        raise Exception(f"After {self.max_retries} retries: {last_error}")

    def execute_sync_with_retry(
        self, method: str, params: Any, timeout: float = 30.0
    ) -> ResultResponse:
        """Execute task synchronously with retry and polling

        Args:
            method: Method name to execute
            params: Parameters for the method
            timeout: Maximum time to wait for completion in seconds

        Returns:
            ResultResponse with final result

        Raises:
            Exception: If all retry attempts fail
        """
        last_error = None

        for attempt in range(self.max_retries):
            try:
                return self.client.execute_sync(method, params, timeout)
            except Exception as e:
                last_error = e
                if attempt < self.max_retries - 1:  # Don't sleep on last attempt
                    time.sleep(self.retry_delay)

        raise Exception(f"After {self.max_retries} retries: {last_error}")

    def execute_sync_encrypted_with_retry(
        self,
        method: str,
        key: str,
        salt: int,
        params: Any,
        timeout: float = 30.0,
    ) -> ResultResponse:
        """Execute encrypted task synchronously with retry logic

        Args:
            method: Method name to execute
            key: Encryption key
            salt: Salt value for encryption
            params: Parameters for the method
            timeout: Maximum time to wait for completion in seconds

        Returns:
            ResultResponse with final decrypted result

        Raises:
            Exception: If all retry attempts fail
        """
        last_exception = None

        for attempt in range(self.max_retries + 1):
            try:
                return self.client.execute_sync_encrypted(
                    method, key, salt, params, timeout
                )
            except Exception as e:
                last_exception = e
                if attempt < self.max_retries:
                    print(
                        f"Attempt {attempt + 1} failed: {e}. "
                        f"Retrying in {self.retry_delay} seconds..."
                    )
                    time.sleep(self.retry_delay)
                else:
                    print(f"All {self.max_retries + 1} attempts failed.")

        raise last_exception

    def get_result(self, task_id: str) -> ResultResponse:
        """Get task result (delegates to underlying client)

        Args:
            task_id: Task ID to get result for

        Returns:
            ResultResponse with final result
        """
        return self.client.get_result(task_id)

    def close(self):
        """Close the underlying client"""
        self.client.close()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
