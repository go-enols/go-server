const { SchedulerClient, call } = require('go-server-sdk');

// 示例1: 使用SchedulerClient
async function clientExample() {
  console.log('=== Scheduler Client Example ===');

  const client = new SchedulerClient('http://localhost:8080');

  try {
    // 异步执行任务
    console.log('\n1. Executing task asynchronously...');
    const response = await client.execute('add', { a: 10, b: 20 });
    console.log('Task submitted with ID:', response.taskId);

    // 获取结果
    const result = await client.getResult(response.taskId);
    console.log('Task result:', result.result);

    // 同步执行任务
    console.log('\n2. Executing task synchronously...');
    const syncResult = await client.executeSync(
      'multiply',
      { a: 5, b: 6 },
      30000
    );
    console.log('Sync task result:', syncResult.result);
  } catch (error) {
    console.error('Error:', error.message);
  }
}

// 示例2: 使用简单的call函数
async function callExample() {
  console.log('\n=== Simple Call Example ===');

  try {
    const result1 = await call('http://localhost:8080', 'add', {
      a: 100,
      b: 200,
    });
    console.log('Call result 1:', result1);

    const result2 = await call('http://localhost:8080', 'multiply', {
      a: 7,
      b: 8,
    });
    console.log('Call result 2:', result2);
  } catch (error) {
    console.error('Call error:', error.message);
  }
}

// 示例3: 批量任务处理
async function batchExample() {
  console.log('\n=== Batch Tasks Example ===');

  const client = new SchedulerClient('http://localhost:8080');

  try {
    // 提交多个任务
    const tasks = [
      { method: 'add', params: { a: 1, b: 2 } },
      { method: 'multiply', params: { a: 3, b: 4 } },
      { method: 'add', params: { a: 5, b: 6 } },
    ];

    console.log('Submitting batch tasks...');
    const taskPromises = tasks.map((task) =>
      client.execute(task.method, task.params)
    );

    const responses = await Promise.all(taskPromises);
    console.log(
      'All tasks submitted, task IDs:',
      responses.map((r) => r.taskId)
    );
    console.log(
      'Task statuses:',
      responses.map((r) => r.status)
    );

    // 获取所有结果
    console.log('Getting results...');
    const resultPromises = responses.map((response) =>
      client.getResult(response.taskId)
    );

    const results = await Promise.all(resultPromises);
    results.forEach((result, index) => {
      console.log(`Task ${index + 1} result:`, result.result);
    });
  } catch (error) {
    console.error('Batch error:', error.message);
  }
}

// 运行示例
async function runExamples() {
  await clientExample();
  await callExample();
  await batchExample();
}

if (require.main === module) {
  runExamples().catch(console.error);
}

module.exports = {
  clientExample,
  callExample,
  batchExample,
};
