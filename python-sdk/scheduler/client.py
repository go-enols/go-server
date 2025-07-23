"""Scheduler client for Python SDK"""

import base64
import hashlib
import json
import time
from dataclasses import dataclass
from typing import Any

import requests
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


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

    def _encrypt_data(self, data: Any, key: str) -> str:
        """Encrypt data using AES-GCM with deterministic IV

        Args:
            data: Data to encrypt
            key: Encryption key

        Returns:
            Base64 encoded encrypted data
        """
        # Serialize data
        data_bytes = json.dumps(data).encode("utf-8")

        # Use SHA-256 hash of key
        key_hash = hashlib.sha256(key.encode()).digest()

        # Generate deterministic IV from key (first 12 bytes)
        iv_hash = hashlib.sha256(key.encode()).digest()
        iv = iv_hash[:12]

        # Encrypt using AES-GCM
        aesgcm = AESGCM(key_hash)
        ciphertext = aesgcm.encrypt(iv, data_bytes, None)

        # Return Base64 encoded result
        return base64.b64encode(ciphertext).decode("utf-8")

    def _salt_key(self, key: str, salt: int) -> str:
        """Encrypt key using salt as AES key

        Args:
            key: Original key
            salt: Salt value

        Returns:
            Base64 encoded encrypted key
        """
        # Use salt to generate SHA-256 hash as AES key
        salt_str = str(salt)
        salt_hash = hashlib.sha256(salt_str.encode()).digest()

        # Generate deterministic IV from salt (first 12 bytes)
        iv_hash = hashlib.sha256(salt_str.encode()).digest()
        iv = iv_hash[:12]

        # Encrypt key using AES-GCM
        key_bytes = key.encode("utf-8")
        aesgcm = AESGCM(salt_hash)
        ciphertext = aesgcm.encrypt(iv, key_bytes, None)

        # Return Base64 encoded result
        return base64.b64encode(ciphertext).decode("utf-8")

    def _decrypt_data(self, encrypted_data: str, key: str) -> Any:
        """Decrypt data using AES-GCM with deterministic IV

        Args:
            encrypted_data: Base64 encoded encrypted data
            key: Decryption key

        Returns:
            Decrypted data
        """
        # Base64 decode
        ciphertext = base64.b64decode(encrypted_data)

        # Use SHA-256 hash of key
        key_hash = hashlib.sha256(key.encode()).digest()

        # Generate deterministic IV from key (first 12 bytes)
        iv_hash = hashlib.sha256(key.encode()).digest()
        iv = iv_hash[:12]

        # Decrypt using AES-GCM
        aesgcm = AESGCM(key_hash)
        plaintext = aesgcm.decrypt(iv, ciphertext, None)

        # Parse JSON data
        return json.loads(plaintext.decode("utf-8"))

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

    def execute_encrypted(
        self, method: str, key: str, salt: int, params: Any
    ) -> ResultResponse:
        """Execute an encrypted task

        Args:
            method: Method name to execute
            key: Encryption key
            salt: Salt value for encryption
            params: Parameters for the method

        Returns:
            ResultResponse with task ID and initial status

        Raises:
            requests.RequestException: If HTTP request fails
            ValueError: If response format is invalid
        """
        # Encrypt parameters
        encrypted_params = self._encrypt_data(params, key)

        # Salt the key
        salted_key = self._salt_key(key, salt)

        request_data = {
            "method": method,
            "params": encrypted_params,
            "key": salted_key,
            "crypto": str(salt),
        }

        try:
            response = self.session.post(
                f"{self.base_url}/api/encrypted/execute",
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

    def get_result_encrypted(self, task_id: str, key: str, salt: int) -> ResultResponse:
        """Get encrypted task result with polling and decryption

        Args:
            task_id: Task ID to get result for
            key: Decryption key
            salt: Salt value used for encryption

        Returns:
            ResultResponse with final decrypted result

        Raises:
            requests.RequestException: If HTTP request fails
            ValueError: If response format is invalid
            RuntimeError: If task execution failed
        """
        try:
            response = self.session.get(
                f"{self.base_url}/api/encrypted/result/{task_id}"
            )
            response.raise_for_status()

            data = response.json()
            result_response = ResultResponse(
                task_id=data["taskId"], status=data["status"], result=data.get("result")
            )

            # Handle different status cases
            if result_response.status in ["pending", "processing"]:
                time.sleep(1)
                return self.get_result_encrypted(task_id, key, salt)
            elif result_response.status == "error":
                raise RuntimeError(str(result_response.result))
            elif result_response.status == "done" and result_response.result:
                # Decrypt result data using original key (not salted key)
                try:
                    decrypted_result = self._decrypt_data(result_response.result, key)
                    result_response.result = decrypted_result
                except Exception as e:
                    raise ValueError(f"Failed to decrypt result: {e}")

            return result_response

        except requests.RequestException as e:
            raise requests.RequestException(f"HTTP request failed: {e}")
        except (KeyError, json.JSONDecodeError) as e:
            raise ValueError(f"Invalid response format: {e}")

    def execute_sync_encrypted(
        self,
        method: str,
        key: str,
        salt: int,
        params: Any,
        timeout: float = 30.0,
    ) -> ResultResponse:
        """Execute encrypted task synchronously with polling and decryption

        Args:
            method: Method name to execute
            key: Encryption key
            salt: Salt value for encryption
            params: Parameters for the method
            timeout: Maximum time to wait for completion in seconds

        Returns:
            ResultResponse with final decrypted result

        Raises:
            TimeoutError: If task doesn't complete within timeout
            requests.RequestException: If HTTP request fails
            RuntimeError: If task execution failed
        """
        # Submit encrypted task
        exec_response = self.execute_encrypted(method, key, salt, params)

        # Poll for result with decryption
        start_time = time.time()
        while time.time() - start_time < timeout:
            try:
                result_response = self.get_result_encrypted(
                    exec_response.task_id, key, salt
                )

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
