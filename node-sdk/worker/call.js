const SchedulerClient = require('../scheduler/client');

/**
 * 简单的RPC调用函数
 * @param {string} host - 调度器地址
 * @param {string} method - 方法名
 * @param {any} params - 参数
 * @returns {Promise<any>} 返回结果
 */
async function call(host, method, params) {
  const client = new SchedulerClient(host);

  const res = await client.execute(method, params);

  // 直接获取结果，execute 只返回 taskId 和 status
  const result = await client.getResult(res.taskId);

  if (result.status === 'error') {
    throw new Error(result.result);
  }

  return result.result;
}

/**
 * 加密的RPC调用函数
 * @param {string} host - 调度器地址
 * @param {string} method - 方法名
 * @param {string} key - 加密密钥
 * @param {number} salt - 加密盐值
 * @param {any} params - 参数
 * @returns {Promise<any>} 返回结果
 */
async function callEncrypted(host, method, key, salt, params) {
  const client = new SchedulerClient(host);
  
  try {
    // 执行加密任务
    const response = await client.executeEncrypted(method, key, salt, params);
    const taskId = response.taskId;
    
    // 轮询结果，设置超时
    const timeout = 30000; // 30秒
    const startTime = Date.now();
    
    while (Date.now() - startTime < timeout) {
      const result = await client.getResult(taskId);
      
      if (result.status === 'completed') {
        return result.result;
      } else if (result.status === 'error') {
        throw new Error(result.result);
      }
      
      // 等待后再次轮询
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    
    throw new Error(`加密任务 ${taskId} 在 ${timeout}ms 内未完成`);
  } finally {
    // 客户端清理自动处理
  }
}

module.exports = { call, callEncrypted };
// 向后兼容
module.exports.default = call;
