"""Python SDK for go-server

This package provides Python client and worker libraries for interacting with the go-server
distributed task scheduling system.

Modules:
    schedulersdk: Client libraries for submitting tasks to the scheduler
    workersdk: Worker libraries for processing tasks

Example usage:
    # Client usage
    from python_sdk.schedulersdk import SchedulerClient
    
    client = SchedulerClient("http://localhost:8080")
    result = client.execute_sync("add", {"a": 1, "b": 2})
    
    # Worker usage
    from python_sdk.workersdk import Worker, Config
    
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
    from python_sdk.workersdk import call
    
    result = call("http://localhost:8080", "add", {"a": 1, "b": 2})
"""

__version__ = "1.2.0"
__author__ = "go-server team"
__description__ = "Python SDK for go-server distributed task scheduling system"

# Import main classes for convenience
from .schedulersdk import SchedulerClient, RetryClient
from .workersdk import Worker, Config, call

__all__ = [
    'SchedulerClient',
    'RetryClient', 
    'Worker',
    'Config',
    'call',
    '__version__',
    '__author__',
    '__description__'
]