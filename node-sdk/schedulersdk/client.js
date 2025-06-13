const axios = require('axios');

class SchedulerClient {
  constructor(baseURL, timeout = 30000) {
    this.baseURL = baseURL;
    this.httpClient = axios.create({
      timeout: timeout,
      headers: {
        'Content-Type': 'application/json'
      }
    });
  }

  /**
   * 执行任务
   * @param {string} method - 方法名
   * @param {any} params - 参数
   * @returns {Promise<{taskId: string, status: string}>}
   */
  async execute(method, params) {
    try {
      const requestBody = {
        method: method,
        params: params
      };

      const response = await this.httpClient.post(
        `${this.baseURL}/api/execute`,
        requestBody
      );

      return response.data;
    } catch (error) {
      if (error.response) {
        const errorData = typeof error.response.data === 'string' 
          ? error.response.data 
          : JSON.stringify(error.response.data);
        throw new Error(`HTTP request failed with status ${error.response.status}: ${errorData}`);
      } else if (error.request) {
        throw new Error('HTTP request failed: No response received');
      } else {
        throw new Error(`HTTP request failed: ${error.message}`);
      }
    }
  }

  /**
   * 获取任务结果
   * @param {string} taskId - 任务ID
   * @returns {Promise<{taskId: string, status: string, result: any}>}
   */
  async getResult(taskId) {
    try {
      const response = await this.httpClient.get(
        `${this.baseURL}/api/result/${taskId}`
      );

      const result = response.data;

      // 如果任务还在处理中，递归等待
      if (result.status === 'pending' || result.status === 'processing') {
        await this.sleep(1000); // 等待1秒
        return await this.getResult(taskId);
      }

      // 如果任务出错，抛出异常
      if (result.status === 'error') {
        throw new Error(result.result);
      }

      return result;
    } catch (error) {
      if (error.response) {
        const errorData = typeof error.response.data === 'string' 
          ? error.response.data 
          : JSON.stringify(error.response.data);
        throw new Error(`HTTP request failed with status ${error.response.status}: ${errorData}`);
      } else if (error.request) {
        throw new Error('HTTP request failed: No response received');
      } else {
        throw new Error(`HTTP request failed: ${error.message}`);
      }
    }
  }

  /**
   * 同步执行任务（带轮询）
   * @param {string} method - 方法名
   * @param {any} params - 参数
   * @param {number} timeout - 超时时间（毫秒）
   * @returns {Promise<{taskId: string, status: string, result: any}>}
   */
  async executeSync(method, params, timeout = 30000) {
    // 提交任务
    const execResp = await this.execute(method, params);

    // 轮询结果
    const start = Date.now();
    while (Date.now() - start < timeout) {
      try {
        const resultResp = await this.getResult(execResp.taskId);
        
        if (resultResp.status === 'done') {
          return resultResp;
        } else if (resultResp.status === 'error') {
          throw new Error(resultResp.result);
        }
        // 'pending' 或 'processing' 状态继续等待
      } catch (error) {
        throw error;
      }

      await this.sleep(500); // 等待500毫秒
    }

    throw new Error('Timeout waiting for task completion');
  }

  /**
   * 睡眠函数
   * @param {number} ms - 毫秒数
   * @returns {Promise<void>}
   */
  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}

module.exports = SchedulerClient;