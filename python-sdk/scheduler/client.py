"""Scheduler client for Python SDK"""

import json
import time
from dataclasses import dataclass
from typing import Any, Dict, Optional

import requests


@dataclass
class ExecuteRequest:
    """Task execution request"""

    method: str
    params: Any


@dataclass
class ResultResponse:
    """Task result response"""

    task_id: str
    status: str
    result: Any


class SchedulerClient:
    """Client for interacting with the scheduler"""

    def __init__(self, base_url: str, timeout: int = 30):
        """Initialize scheduler client

        Args:
            base_url: Base URL of the scheduler
            timeout: HTTP request timeout in seconds
        """
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()
        self.session.timeout = timeout

    def execute(self, method: str, params: Any) -> ResultResponse:
        """Execute a task

        Args:
            method: Method name to execute
            params: Parameters for the method

        Returns:
            ResultResponse with task ID and initial status

        Raises:
            requests.RequestException: If HTTP request fails
            ValueError: If response format is invalid
        """
        request_data = {"method": method, "params": params}

        try:
            response = self.session.post(
                f"{self.base_url}/api/execute",
                json=request_data,
                headers={"Content-Type": "application/json"},
            )
            response.raise_for_status()

            data = response.json()
            return ResultResponse(
                task_id=data["taskId"], status=data["status"], result=data.get("result")
            )

        except requests.RequestException as e:
            raise requests.RequestException(f"HTTP request failed: {e}")
        except (KeyError, json.JSONDecodeError) as e:
            raise ValueError(f"Invalid response format: {e}")

    def get_result(self, task_id: str) -> ResultResponse:
        """Get task result with polling for completion

        Args:
            task_id: Task ID to get result for

        Returns:
            ResultResponse with final result

        Raises:
            requests.RequestException: If HTTP request fails
            ValueError: If response format is invalid
            RuntimeError: If task execution failed
        """
        try:
            response = self.session.get(f"{self.base_url}/api/result/{task_id}")
            response.raise_for_status()

            data = response.json()
            result_response = ResultResponse(
                task_id=data["taskId"], status=data["status"], result=data.get("result")
            )

            # Handle different status cases
            if result_response.status in ["pending", "processing"]:
                time.sleep(1)
                return self.get_result(task_id)
            elif result_response.status == "error":
                raise RuntimeError(str(result_response.result))

            return result_response

        except requests.RequestException as e:
            raise requests.RequestException(f"HTTP request failed: {e}")
        except (KeyError, json.JSONDecodeError) as e:
            raise ValueError(f"Invalid response format: {e}")

    def execute_sync(
        self, method: str, params: Any, timeout: float = 30.0
    ) -> ResultResponse:
        """Execute task synchronously with polling

        Args:
            method: Method name to execute
            params: Parameters for the method
            timeout: Maximum time to wait for completion in seconds

        Returns:
            ResultResponse with final result

        Raises:
            TimeoutError: If task doesn't complete within timeout
            requests.RequestException: If HTTP request fails
            RuntimeError: If task execution failed
        """
        # Submit task
        exec_response = self.execute(method, params)

        # Poll for result
        start_time = time.time()
        while time.time() - start_time < timeout:
            try:
                result_response = self.get_result(exec_response.task_id)

                if result_response.status == "done":
                    return result_response
                elif result_response.status == "error":
                    raise RuntimeError(str(result_response.result))
                # Continue polling for "pending" or "processing" status

            except RuntimeError:
                # Re-raise task execution errors
                raise
            except Exception as e:
                # Continue polling on other errors
                print(f"Polling error (continuing): {e}")

            time.sleep(0.5)

        raise TimeoutError("Timeout waiting for task completion")

    def close(self):
        """Close the HTTP session"""
        self.session.close()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
