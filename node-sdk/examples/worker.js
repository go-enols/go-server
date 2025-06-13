const { Worker } = require('go-server-sdk');

// 创建worker实例
const worker = new Worker({
  schedulerURL: 'ws://localhost:8080/api/worker/connect',
  workerGroup: 'math-workers',
  maxRetry: 3,
  pingInterval: 30
});

// 注册数学运算方法
worker.registerMethod('add', async ({ a, b }) => {
  console.log(`Executing add: ${a} + ${b}`);
  return a + b;
}, 'Add two numbers together');

worker.registerMethod('subtract', async ({ a, b }) => {

  console.log(`Executing subtract: ${a} - ${b}`);
  return a - b;
}, 'Subtract second number from first number');

worker.registerMethod('multiply', async ({ a, b }) => {

  console.log(`Executing multiply: ${a} * ${b}`);
  return a * b;
}, 'Multiply two numbers');

worker.registerMethod('divide', async ({ a, b }) => {
  if (b === 0) {
    throw new Error('Division by zero is not allowed');
  }
  
  console.log(`Executing divide: ${a} / ${b}`);
  return a / b;
}, 'Divide first number by second number');

// 注册复杂计算方法
worker.registerMethod('factorial', async ({ n }) => {
  if (n < 0) {
    throw new Error('Factorial is not defined for negative numbers');
  }
  
  console.log(`Executing factorial: ${n}!`);
  
  let result = 1;
  for (let i = 2; i <= n; i++) {
    result *= i;
  }
  
  return result;
}, 'Calculate factorial of a number');

worker.registerMethod('fibonacci', async ({ n }) => {
  
  if (n < 0) {
    throw new Error('Fibonacci is not defined for negative numbers');
  }
  
  console.log(`Executing fibonacci: fib(${n})`);
  
  if (n <= 1) return n;
  
  let a = 0, b = 1;
  for (let i = 2; i <= n; i++) {
    [a, b] = [b, a + b];
  }
  
  return b;
}, 'Calculate nth Fibonacci number');

// 注册异步任务示例
worker.registerMethod('delay', async ({ seconds, message }) => {
  
  console.log(`Executing delay: waiting ${seconds} seconds...`);
  
  await new Promise(resolve => setTimeout(resolve, seconds * 1000));
  
  return `Delayed message after ${seconds} seconds: ${message}`;
}, 'Delay execution for specified seconds and return message');

// 注册数据处理方法
worker.registerMethod('processArray', async ({ numbers, operation }) => {
  
  console.log(`Processing array with operation: ${operation}`);
  
  switch (operation) {
    case 'sum':
      return numbers.reduce((sum, num) => sum + num, 0);
    case 'average':
      return numbers.reduce((sum, num) => sum + num, 0) / numbers.length;
    case 'max':
      return Math.max(...numbers);
    case 'min':
      return Math.min(...numbers);
    case 'sort':
      return [...numbers].sort((a, b) => a - b);
    default:
      throw new Error(`Unknown operation: ${operation}`);
  }
}, 'Process array of numbers with specified operation (sum, average, max, min, sort)');

// 启动worker
async function startWorker() {
  try {
    console.log('Starting math worker...');
    await worker.start();
    console.log('Math worker started successfully!');
    console.log('Registered methods:');
    
    // 显示注册的方法
    const methods = worker.getMethodsWithDocs();
    methods.forEach(method => {
      console.log(`  - ${method.name}: ${method.docs.join(', ')}`);
    });
    
  } catch (error) {
    console.error('Failed to start worker:', error.message);
    process.exit(1);
  }
}

// 优雅关闭
process.on('SIGINT', () => {
  console.log('\nReceived SIGINT, stopping worker...');
  worker.stop();
  process.exit(0);
});

process.on('SIGTERM', () => {
  console.log('\nReceived SIGTERM, stopping worker...');
  worker.stop();
  process.exit(0);
});

// 错误处理
process.on('unhandledRejection', (reason, promise) => {
  console.error('Unhandled Rejection at:', promise, 'reason:', reason);
});

process.on('uncaughtException', (error) => {
  console.error('Uncaught Exception:', error);
  worker.stop();
  process.exit(1);
});

if (require.main === module) {
  startWorker();
}

module.exports = worker;