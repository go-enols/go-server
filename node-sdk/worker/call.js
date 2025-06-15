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

module.exports = call;
