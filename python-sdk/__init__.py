"""Python SDK for go-server

This package provides Python client and worker libraries for interacting with the go-server
distributed task scheduling system.

Modules:
    scheduler: Client libraries for submitting tasks to the scheduler
    worker: Worker libraries for processing tasks

Example usage:
    # Client usage
    from python_sdk.scheduler import SchedulerClient

    client = SchedulerClient("http://localhost:8080")
    result = client.execute_sync("add", {"a": 1, "b": 2})

    # Worker usage
    from python_sdk.worker import Worker, Config

    def add_numbers(params):
        return params["a"] + params["b"]

    config = Config(
        scheduler_url="http://localhost:8080",
        worker_group="math_workers"
    )
    worker = Worker(config)
    worker.register_method("add", add_numbers, "Add two numbers")
    worker.start()

    # Simple call function
    from python_sdk.worker import call

    result = call("http://localhost:8080", "add", {"a": 1, "b": 2})
"""

__version__ = "1.4.5"
__author__ = "go-server team"
__description__ = "Python SDK for go-server distributed task scheduling system"

# Import main classes for convenience
from .scheduler import RetryClient, SchedulerClient
from .worker import Config, Worker, call

__all__ = [
    "SchedulerClient",
    "RetryClient",
    "Worker",
    "Config",
    "call",
    "__version__",
    "__author__",
    "__description__",
]
