const SchedulerClient = require('./client');

class RetryClient extends SchedulerClient {
  constructor(baseURL, maxRetries = 3, retryDelay = 1000, timeout = 30000) {
    super(baseURL, timeout);
    this.maxRetries = maxRetries;
    this.retryDelay = retryDelay;
  }

  /**
   * 执行任务（带重试）
   * @param {string} method - 方法名
   * @param {any} params - 参数
   * @returns {Promise<{taskId: string, status: string}>}
   */
  async executeWithRetry(method, params) {
    let lastError;
    
    for (let i = 0; i < this.maxRetries; i++) {
      try {
        return await this.execute(method, params);
      } catch (error) {
        lastError = error;
        if (i < this.maxRetries - 1) {
          await this.sleep(this.retryDelay);
        }
      }
    }
    
    throw new Error(`After ${this.maxRetries} retries: ${lastError.message}`);
  }

  /**
   * 执行加密任务（带重试）
   * @param {string} method - 方法名
   * @param {string} key - 加密密钥
   * @param {number} salt - 盐值
   * @param {any} params - 参数
   * @returns {Promise<{taskId: string, status: string}>}
   */
  async executeEncryptedWithRetry(method, key, salt, params) {
    let lastError;
    
    for (let i = 0; i < this.maxRetries; i++) {
      try {
        return await this.executeEncrypted(method, key, salt, params);
      } catch (error) {
        lastError = error;
        if (i < this.maxRetries - 1) {
          await this.sleep(this.retryDelay);
        }
      }
    }
    
    throw new Error(`After ${this.maxRetries} retries: ${lastError.message}`);
  }
}

module.exports = RetryClient;