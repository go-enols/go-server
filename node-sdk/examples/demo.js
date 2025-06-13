const { SchedulerClient, Worker, call } = require('go-server-sdk');

// 演示完整的工作流程
async function demo() {
  console.log('=== Go-Server Node.js SDK Demo ===\n');
  
  // 1. 创建并启动Worker
  console.log('1. Setting up worker...');
  const worker = new Worker({
    schedulerURL: 'ws://localhost:8080/api/worker/connect',
    workerGroup: 'demo-workers',
    maxRetry: 3,
    pingInterval: 30
  });
  
  // 注册一些示例方法
  worker.registerMethod('greet', async (params) => {
    const data = JSON.parse(params);
    return `Hello, ${data.name}! Welcome to go-server.`;
  }, 'Greet a person by name');
  
  worker.registerMethod('calculate', async (params) => {
    const data = JSON.parse(params);
    const { operation, a, b } = data;
    
    switch (operation) {
      case 'add': return a + b;
      case 'subtract': return a - b;
      case 'multiply': return a * b;
      case 'divide': 
        if (b === 0) throw new Error('Division by zero');
        return a / b;
      default:
        throw new Error(`Unknown operation: ${operation}`);
    }
  }, 'Perform basic mathematical operations');
  
  worker.registerMethod('processData', async (params) => {
    const data = JSON.parse(params);
    const { items, transform } = data;
    
    switch (transform) {
      case 'uppercase':
        return items.map(item => item.toUpperCase());
      case 'lowercase':
        return items.map(item => item.toLowerCase());
      case 'reverse':
        return items.map(item => item.split('').reverse().join(''));
      case 'length':
        return items.map(item => item.length);
      default:
        return items;
    }
  }, 'Transform array of strings');
  
  try {
    await worker.start();
    console.log('✓ Worker started successfully\n');
    
    // 等待一下让worker完全连接
    await new Promise(resolve => setTimeout(resolve, 2000));
    
    // 2. 使用SchedulerClient进行测试
    console.log('2. Testing with SchedulerClient...');
    const client = new SchedulerClient('http://localhost:8080');
    
    // 测试greet方法
    console.log('\n  Testing greet method:');
    const greetResult = await client.executeSync('greet', { name: 'Alice' }, 10000);
    console.log('  Result:', greetResult.result);
    
    // 测试calculate方法
    console.log('\n  Testing calculate method:');
    const calcResult = await client.executeSync('calculate', {
      operation: 'multiply',
      a: 15,
      b: 7
    }, 10000);
    console.log('  Result:', calcResult.result);
    
    // 测试processData方法
    console.log('\n  Testing processData method:');
    const processResult = await client.executeSync('processData', {
      items: ['hello', 'world', 'nodejs'],
      transform: 'uppercase'
    }, 10000);
    console.log('  Result:', processResult.result);
    
    // 3. 使用简单call函数进行测试
    console.log('\n3. Testing with simple call function...');
    
    const callResult1 = await call('http://localhost:8080', 'greet', { name: 'Bob' });
    console.log('  Call result 1:', callResult1);
    
    const callResult2 = await call('http://localhost:8080', 'calculate', {
      operation: 'add',
      a: 100,
      b: 200
    });
    console.log('  Call result 2:', callResult2);
    
    // 4. 批量任务测试
    console.log('\n4. Testing batch operations...');
    
    const batchTasks = [
      { method: 'calculate', params: { operation: 'add', a: 1, b: 2 } },
      { method: 'calculate', params: { operation: 'subtract', a: 10, b: 3 } },
      { method: 'calculate', params: { operation: 'multiply', a: 4, b: 5 } },
      { method: 'greet', params: { name: 'Charlie' } }
    ];
    
    console.log('  Submitting batch tasks...');
    const batchPromises = batchTasks.map(task => 
      client.executeSync(task.method, task.params, 10000)
    );
    
    const batchResults = await Promise.all(batchPromises);
    batchResults.forEach((result, index) => {
      console.log(`  Batch task ${index + 1} result:`, result.result);
    });
    
    // 5. 错误处理测试
    console.log('\n5. Testing error handling...');
    
    try {
      await call('http://localhost:8080', 'calculate', {
        operation: 'divide',
        a: 10,
        b: 0
      });
    } catch (error) {
      console.log('  Expected error caught:', error.message);
    }
    
    try {
      await call('http://localhost:8080', 'nonexistent_method', {});
    } catch (error) {
      console.log('  Expected error caught:', error.message);
    }
    
    console.log('\n✓ All tests completed successfully!');
    
  } catch (error) {
    console.error('Demo failed:', error.message);
  } finally {
    // 清理
    console.log('\nCleaning up...');
    worker.stop();
    console.log('✓ Demo completed');
  }
}

// 运行演示
if (require.main === module) {
  demo().catch(error => {
    console.error('Demo error:', error);
    process.exit(1);
  });
}

module.exports = { demo };