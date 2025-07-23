"""Worker implementation for Python SDK"""

import base64
import inspect
import json
import logging
import threading
import time
import websocket
from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Optional

from cryptography.hazmat.primitives.ciphers.aead import AESGCM


@dataclass
class Config:
    """Worker configuration"""

    scheduler_url: str  # Scheduler WebSocket URL
    worker_group: str  # Worker group name
    max_retry: int = 3  # Connection retry attempts
    ping_interval: int = 30  # Heartbeat interval in seconds


class Worker:
    """Worker client for connecting to scheduler"""

    def __init__(self, config: Config):
        """Initialize worker

        Args:
            config: Worker configuration
        """
        self.config = config
        self.ws: Optional[websocket.WebSocket] = None
        self.methods: Dict[str, Callable] = {}
        self.docs: Dict[str, List[str]] = {}
        self.running = False
        self.reconnect = False
        self.stop_event = threading.Event()
        self.conn_lock = threading.Lock()

        # Setup logging
        self.logger = logging.getLogger(__name__)

    def register_method(self, name: str, handler: Callable, *docs: str) -> None:
        """Register a method handler

        Args:
            name: Method name
            handler: Function to handle the method call
            *docs: Documentation strings for the method

        Raises:
            ValueError: If handler is not callable
        """
        if not callable(handler):
            raise ValueError("Handler must be callable")

        # Validate function signature
        sig = inspect.signature(handler)
        if len(sig.parameters) != 1:
            raise ValueError(
                "Handler signature must be: func(params: dict) -> (result, error)"
            )

        self.methods[name] = handler
        self.docs[name] = list(docs)
        self.logger.info(f"Registered method: {name}")

    def start(self) -> None:
        """Start the worker

        Raises:
            Exception: If connection fails
        """
        self.running = True
        self.reconnect = True

        if not self._connect():
            raise Exception("Failed to connect to scheduler")

        # Start background threads
        threading.Thread(target=self._keep_alive, daemon=True).start()
        threading.Thread(target=self._process_tasks, daemon=True).start()

        self.logger.info(f"Worker {self.config.worker_group} started")

    def stop(self) -> None:
        """Stop the worker"""
        if not self.running:
            return

        self.running = False
        self.reconnect = False
        self.stop_event.set()

        with self.conn_lock:
            if self.ws:
                try:
                    self.ws.close()
                except Exception as e:
                    self.logger.debug(f"Error closing WebSocket: {e}")
                self.ws = None

        self.logger.info("Worker stopped")

    def _connect(self) -> bool:
        """Connect to scheduler with retry logic

        Returns:
            True if connection successful, False otherwise
        """
        with self.conn_lock:
            retry_count = 0

            while retry_count < self.config.max_retry:
                try:
                    # Convert HTTP URL to WebSocket URL
                    ws_url = self.config.scheduler_url.replace(
                        "http://", "ws://"
                    ).replace("https://", "wss://")
                    if not ws_url.endswith("/api/worker/connect"):
                        ws_url = ws_url.rstrip("/") + "/api/worker/connect"

                    self.ws = websocket.create_connection(ws_url)

                    # Send registration
                    registration = {
                        "group": self.config.worker_group,
                        "methods": self._get_methods_with_docs(),
                    }
                    self.ws.send(json.dumps(registration))

                    self.logger.info(f"Connected to scheduler: {ws_url}")
                    return True

                except Exception as e:
                    self.logger.error(
                        f"Connection attempt {retry_count + 1} failed: {e}"
                    )
                    retry_count += 1
                    if retry_count < self.config.max_retry:
                        time.sleep(retry_count)  # Exponential backoff

            return False

    def _get_methods_with_docs(self) -> List[Dict[str, Any]]:
        """Get methods with their documentation

        Returns:
            List of method info dictionaries
        """
        methods_info = []
        for name in self.methods:
            methods_info.append({"name": name, "docs": self.docs.get(name, [])})
        return methods_info

    def _keep_alive(self) -> None:
        """Keep connection alive with periodic pings"""
        while self.running:
            try:
                if self.stop_event.wait(self.config.ping_interval):
                    break

                with self.conn_lock:
                    if self.ws:
                        ping_msg = {"type": "ping"}
                        self.ws.send(json.dumps(ping_msg))

            except Exception as e:
                self.logger.error(f"Ping failed: {e}")
                with self.conn_lock:
                    if self.ws:
                        self.ws.close()
                        self.ws = None

    def _process_tasks(self) -> None:
        """Process incoming tasks"""
        while self.running:
            with self.conn_lock:
                ws = self.ws

            if not ws:
                if not self.reconnect:
                    return

                if self._connect():
                    continue
                else:
                    self.logger.error("Reconnect failed, retrying in 5s")
                    time.sleep(5)
                    continue

            try:
                # Receive message
                message = ws.recv()
                if not message:
                    continue

                msg = json.loads(message)
                msg_type = msg.get("type")

                if msg_type == "task":
                    threading.Thread(
                        target=self._handle_task,
                        args=(msg.get("taskId"), msg.get("method"), msg.get("params")),
                        daemon=True,
                    ).start()
                elif msg_type == "encrypted_task":
                    threading.Thread(
                        target=self._handle_encrypted_task,
                        args=(
                            msg.get("taskId"),
                            msg.get("method"),
                            msg.get("params"),
                            msg.get("key"),
                            msg.get("crypto"),
                        ),
                        daemon=True,
                    ).start()
                elif msg_type == "ping":
                    # Respond to ping
                    with self.conn_lock:
                        if self.ws:
                            pong_msg = {"type": "pong"}
                            self.ws.send(json.dumps(pong_msg))

            except websocket.WebSocketConnectionClosedException:
                self.logger.warning("WebSocket connection closed")
                with self.conn_lock:
                    if self.ws:
                        self.ws.close()
                        self.ws = None
            except Exception as e:
                if self.running:
                    self.logger.error(f"Error processing tasks: {e}")
                    with self.conn_lock:
                        if self.ws:
                            self.ws.close()
                            self.ws = None

    def _handle_task(self, task_id: str, method: str, params: Any) -> None:
        """Handle a single task

        Args:
            task_id: Task identifier
            method: Method name to execute
            params: Method parameters
        """
        try:
            if method not in self.methods:
                raise ValueError(f"Unknown method: {method}")

            handler = self.methods[method]

            # Execute the method
            try:
                result = handler(params)

                # Send success response
                response = {
                    "type": "result",
                    "taskId": task_id,
                    "status": "success",
                    "result": result,
                }

            except Exception as e:
                # Send error response
                response = {
                    "type": "result",
                    "taskId": task_id,
                    "status": "error",
                    "error": str(e),
                }

            # Send response
            with self.conn_lock:
                if self.ws:
                    self.ws.send(json.dumps(response))

        except Exception as e:
            self.logger.error(f"Error handling task {task_id}: {e}")

            # Send error response
            try:
                response = {
                    "type": "result",
                    "taskId": task_id,
                    "status": "error",
                    "error": str(e),
                }

                with self.conn_lock:
                    if self.ws:
                        self.ws.send(json.dumps(response))
            except Exception as e:
                self.logger.debug(f"Error sending error response: {e}")

    def _handle_encrypted_task(
        self,
        task_id: str,
        method: str,
        encrypted_params: str,
        key: str,
        crypto_info: Dict[str, Any],
    ) -> None:
        """Handle an encrypted task

        Args:
            task_id: Task identifier
            method: Method name to execute
            encrypted_params: Encrypted method parameters
            key: Encryption key
            crypto_info: Cryptographic information
        """
        try:
            # Decrypt parameters
            try:
                decoded_key = base64.b64decode(key)
                decoded_params = base64.b64decode(encrypted_params)

                if crypto_info.get("algorithm") == "AES-GCM":
                    aesgcm = AESGCM(decoded_key)
                    nonce = base64.b64decode(crypto_info.get("nonce", ""))
                    decrypted_data = aesgcm.decrypt(nonce, decoded_params, None)
                    params = json.loads(decrypted_data.decode("utf-8"))
                else:
                    raise ValueError(
                        f"Unsupported encryption algorithm: "
                        f"{crypto_info.get('algorithm')}"
                    )
            except Exception as e:
                raise ValueError(f"Failed to decrypt parameters: {e}")

            if method not in self.methods:
                raise ValueError(f"Unknown method: {method}")

            handler = self.methods[method]

            # Execute the method
            try:
                result = handler(params)

                # Encrypt result
                try:
                    result_json = json.dumps(result)
                    if crypto_info.get("algorithm") == "AES-GCM":
                        aesgcm = AESGCM(decoded_key)
                        nonce = base64.b64decode(crypto_info.get("nonce", ""))
                        encrypted_result = aesgcm.encrypt(
                            nonce, result_json.encode("utf-8"), None
                        )
                        encoded_result = base64.b64encode(encrypted_result).decode(
                            "utf-8"
                        )
                    else:
                        raise ValueError(
                            f"Unsupported encryption algorithm: "
                            f"{crypto_info.get('algorithm')}"
                        )
                except Exception as e:
                    raise ValueError(f"Failed to encrypt result: {e}")

                # Send success response
                response = {
                    "type": "encrypted_result",
                    "taskId": task_id,
                    "status": "success",
                    "result": encoded_result,
                    "crypto": crypto_info,
                }

            except Exception as e:
                # Send error response (unencrypted for debugging)
                response = {
                    "type": "result",
                    "taskId": task_id,
                    "status": "error",
                    "error": str(e),
                }

            # Send response
            with self.conn_lock:
                if self.ws:
                    self.ws.send(json.dumps(response))

        except Exception as e:
            self.logger.error(f"Error handling encrypted task {task_id}: {e}")

            # Send error response
            try:
                response = {
                    "type": "result",
                    "taskId": task_id,
                    "status": "error",
                    "error": str(e),
                }

                with self.conn_lock:
                    if self.ws:
                        self.ws.send(json.dumps(response))
            except Exception as e:
                self.logger.debug(f"Error sending error response: {e}")
