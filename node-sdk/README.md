# Go-Server Node.js SDK

Node.js SDK for go-server distributed task scheduler and worker system.
[GitHub](https://github.com/go-enols/go-server)

## Installation

```bash
npm install go-server-sdk
```

## Dependencies

- `axios`: HTTP client for scheduler API calls
- `ws`: WebSocket client for worker connections

## Usage

### Scheduler Client

The scheduler client allows you to submit tasks and retrieve results.

```javascript
const { SchedulerClient } = require('go-server-sdk');

// Create client
const client = new SchedulerClient('http://localhost:8080');

// Execute task asynchronously
async function executeTask() {
  try {
    // Submit task
    const response = await client.execute('add', { a: 1, b: 2 });
    console.log('Task submitted:', response.taskId);

    // Get result
    const result = await client.getResult(response.taskId);
    console.log('Result:', result.result);
  } catch (error) {
    console.error('Error:', error.message);
  }
}

// Execute task synchronously (with polling)
async function executeSyncTask() {
  try {
    const result = await client.executeSync('add', { a: 1, b: 2 }, 30000);
    console.log('Sync result:', result.result);
  } catch (error) {
    console.error('Error:', error.message);
  }
}
```

### Worker

The worker connects to the scheduler via WebSocket and processes tasks.

```javascript
const { Worker } = require('go-server-sdk');

// Create worker
const worker = new Worker({
  schedulerURL: 'ws://localhost:8080/ws',
  workerGroup: 'math-workers',
  maxRetry: 3,
  pingInterval: 30,
});

// Register methods
worker.registerMethod(
  'add',
  async (params) => {
    const { a, b } = JSON.parse(params);
    return a + b;
  },
  'Add two numbers'
);

worker.registerMethod(
  'multiply',
  async (params) => {
    const { a, b } = JSON.parse(params);
    return a * b;
  },
  'Multiply two numbers'
);

// Start worker
async function startWorker() {
  try {
    await worker.start();
    console.log('Worker started successfully');
  } catch (error) {
    console.error('Failed to start worker:', error.message);
  }
}

// Stop worker gracefully
process.on('SIGINT', () => {
  console.log('Stopping worker...');
  worker.stop();
  process.exit(0);
});

startWorker();
```

### Simple RPC Call

For simple use cases, you can use the `call` function:

```javascript
const { call } = require('./index');

async function simpleCall() {
  try {
    const result = await call('http://localhost:8080', 'add', { a: 1, b: 2 });
    console.log('Result:', result);
  } catch (error) {
    console.error('Error:', error.message);
  }
}
```

## API Reference

### SchedulerClient

#### Constructor

- `new SchedulerClient(baseURL, timeout?)` - Create a new scheduler client
  - `baseURL`: Scheduler server URL
  - `timeout`: Request timeout in milliseconds (default: 30000)

#### Methods

- `execute(method, params)` - Submit a task for execution, returns `{taskId, status}`
- `getResult(taskId)` - Get task result (with automatic polling for pending tasks), returns `{taskId, status, result}`
- `executeSync(method, params, timeout?)` - Execute task synchronously with polling, returns `{taskId, status, result}`

### Worker

#### Constructor

- `new Worker(config)` - Create a new worker
  - `config.schedulerURL`: WebSocket URL of the scheduler
  - `config.workerGroup`: Worker group name
  - `config.maxRetry`: Maximum connection retry attempts (default: 3)
  - `config.pingInterval`: Ping interval in seconds (default: 30)

#### Methods

- `registerMethod(name, handler, ...docs)` - Register a method handler
  - `name`: Method name
  - `handler`: Async function that processes the task
  - `docs`: Documentation strings
- `start()` - Start the worker and connect to scheduler
- `stop()` - Stop the worker and close connections

### call(host, method, params)

Simple RPC call function that submits a task and waits for the result.

## Error Handling

All methods throw errors for:

- Network connection issues
- HTTP errors
- Task execution errors
- Timeout errors

Make sure to wrap calls in try-catch blocks.

## Features

- ✅ HTTP client for scheduler API
- ✅ WebSocket worker with automatic reconnection
- ✅ Method registration and documentation
- ✅ Automatic task result polling
- ✅ Heartbeat/ping-pong mechanism
- ✅ Graceful error handling
- ✅ Configurable timeouts and retry logic

## Examples

See the `examples/` directory for complete working examples.
