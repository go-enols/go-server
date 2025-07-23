const axios = require('axios');
const crypto = require('crypto');

class SchedulerClient {
  constructor(baseURL, timeout = 30000) {
    this.baseURL = baseURL;
    this.httpClient = axios.create({
      timeout: timeout,
      headers: {
        'Content-Type': 'application/json',
      },
    });
  }

  /**
   * 使用AES-GCM加密数据
   * @param {any} data - 要加密的数据
   * @param {string} key - 加密密钥
   * @returns {string} Base64编码的加密数据
   */
  encryptData(data, key) {
    // 序列化数据
    const dataBytes = Buffer.from(JSON.stringify(data), 'utf8');
    
    // 使用SHA-256哈希密钥
    const keyHash = crypto.createHash('sha256').update(key).digest();
    
    // 使用密钥生成确定性IV（前12字节）
    const ivHash = crypto.createHash('sha256').update(key).digest();
    const iv = ivHash.slice(0, 12);
    
    // 创建加密器
    const cipher = crypto.createCipherGCM('aes-256-gcm');
    cipher.setIV(iv);
    
    // 加密数据
    let encrypted = cipher.update(dataBytes);
    cipher.final();
    const authTag = cipher.getAuthTag();
    
    // 合并加密数据和认证标签
    const result = Buffer.concat([encrypted, authTag]);
    
    // 返回Base64编码的结果
    return result.toString('base64');
  }

  /**
   * 使用盐值加密密钥
   * @param {string} key - 原始密钥
   * @param {number} salt - 盐值
   * @returns {string} Base64编码的加密密钥
   */
  saltKey(key, salt) {
    // 使用盐值生成SHA-256哈希作为AES密钥
    const saltStr = salt.toString();
    const saltHash = crypto.createHash('sha256').update(saltStr).digest();
    
    // 使用盐值生成确定性IV（前12字节）
    const ivHash = crypto.createHash('sha256').update(saltStr).digest();
    const iv = ivHash.slice(0, 12);
    
    // 创建加密器
    const cipher = crypto.createCipherGCM('aes-256-gcm');
    cipher.setIV(iv);
    
    // 加密密钥
    const keyBytes = Buffer.from(key, 'utf8');
    let encrypted = cipher.update(keyBytes);
    cipher.final();
    const authTag = cipher.getAuthTag();
    
    // 合并加密数据和认证标签
    const result = Buffer.concat([encrypted, authTag]);
    
    // 返回Base64编码的结果
    return result.toString('base64');
  }

  /**
   * 使用AES-GCM解密数据
   * @param {string} encryptedData - Base64编码的加密数据
   * @param {string} key - 解密密钥
   * @returns {any} 解密后的数据
   */
  decryptData(encryptedData, key) {
    // Base64解码
    const ciphertext = Buffer.from(encryptedData, 'base64');
    
    // 分离加密数据和认证标签（最后16字节是认证标签）
    const encrypted = ciphertext.slice(0, -16);
    const authTag = ciphertext.slice(-16);
    
    // 使用SHA-256哈希密钥
    const keyHash = crypto.createHash('sha256').update(key).digest();
    
    // 使用密钥生成确定性IV（前12字节）
    const ivHash = crypto.createHash('sha256').update(key).digest();
    const iv = ivHash.slice(0, 12);
    
    // 创建解密器
    const decipher = crypto.createDecipherGCM('aes-256-gcm');
    decipher.setIV(iv);
    decipher.setAuthTag(authTag);
    
    // 解密数据
    let decrypted = decipher.update(encrypted);
    decipher.final();
    
    // 解析JSON数据
    return JSON.parse(decrypted.toString('utf8'));
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
        params: params,
      };

      const response = await this.httpClient.post(
        `${this.baseURL}/api/execute`,
        requestBody
      );

      return response.data;
    } catch (error) {
      if (error.response) {
        const errorData =
          typeof error.response.data === 'string'
            ? error.response.data
            : JSON.stringify(error.response.data);
        throw new Error(
          `HTTP request failed with status ${error.response.status}: ${errorData}`
        );
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
        const errorData =
          typeof error.response.data === 'string'
            ? error.response.data
            : JSON.stringify(error.response.data);
        throw new Error(
          `HTTP request failed with status ${error.response.status}: ${errorData}`
        );
      } else if (error.request) {
        throw new Error('HTTP request failed: No response received');
      } else {
        throw new Error(`HTTP request failed: ${error.message}`);
      }
    }
  }

  /**
   * 执行加密任务
   * @param {string} method - 方法名
   * @param {string} key - 加密密钥
   * @param {number} salt - 盐值
   * @param {any} params - 参数
   * @returns {Promise<{taskId: string, status: string}>}
   */
  async executeEncrypted(method, key, salt, params) {
    try {
      // 加密参数
      const encryptedParams = this.encryptData(params, key);
      
      // 对密钥进行加盐处理
      const saltedKey = this.saltKey(key, salt);
      
      const requestBody = {
        method: method,
        params: encryptedParams,
        key: saltedKey,
        crypto: salt.toString(),
      };

      const response = await this.httpClient.post(
        `${this.baseURL}/api/encrypted/execute`,
        requestBody
      );

      return response.data;
    } catch (error) {
      if (error.response) {
        const errorData =
          typeof error.response.data === 'string'
            ? error.response.data
            : JSON.stringify(error.response.data);
        throw new Error(
          `HTTP request failed with status ${error.response.status}: ${errorData}`
        );
      } else if (error.request) {
        throw new Error('HTTP request failed: No response received');
      } else {
        throw new Error(`HTTP request failed: ${error.message}`);
      }
    }
  }

  /**
   * 获取加密任务结果
   * @param {string} taskId - 任务ID
   * @param {string} key - 解密密钥
   * @param {number} salt - 盐值
   * @returns {Promise<{taskId: string, status: string, result: any}>}
   */
  async getResultEncrypted(taskId, key, salt) {
    try {
      const response = await this.httpClient.get(
        `${this.baseURL}/api/encrypted/result/${taskId}`
      );

      const result = response.data;

      // 如果任务还在处理中，递归等待
      if (result.status === 'pending' || result.status === 'processing') {
        await this.sleep(1000); // 等待1秒
        return await this.getResultEncrypted(taskId, key, salt);
      }

      // 如果任务出错，抛出异常
      if (result.status === 'error') {
        throw new Error(result.result);
      }

      // 如果任务完成，解密结果数据
      if (result.status === 'done' && result.result) {
        try {
          // 使用原始密钥解密数据（不使用加盐后的密钥）
          const decryptedResult = this.decryptData(result.result, key);
          result.result = decryptedResult;
        } catch (decryptError) {
          throw new Error(`Failed to decrypt result: ${decryptError.message}`);
        }
      }

      return result;
    } catch (error) {
      if (error.response) {
        const errorData =
          typeof error.response.data === 'string'
            ? error.response.data
            : JSON.stringify(error.response.data);
        throw new Error(
          `HTTP request failed with status ${error.response.status}: ${errorData}`
        );
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
      const resultResp = await this.getResult(execResp.taskId);

      if (resultResp.status === 'done') {
        return resultResp;
      } else if (resultResp.status === 'error') {
        throw new Error(resultResp.result);
      }
      // 'pending' 或 'processing' 状态继续等待

      await this.sleep(500); // 等待500毫秒
    }

    throw new Error('Timeout waiting for task completion');
  }

  /**
   * 同步执行加密任务（带轮询和解密）
   * @param {string} method - 方法名
   * @param {string} key - 加密密钥
   * @param {number} salt - 盐值
   * @param {any} params - 参数
   * @param {number} timeout - 超时时间（毫秒）
   * @returns {Promise<{taskId: string, status: string, result: any}>}
   */
  async executeSyncEncrypted(method, key, salt, params, timeout = 30000) {
    // 提交加密任务
    const execResp = await this.executeEncrypted(method, key, salt, params);

    // 轮询并解密结果
    const start = Date.now();
    while (Date.now() - start < timeout) {
      const resultResp = await this.getResultEncrypted(execResp.taskId, key, salt);

      if (resultResp.status === 'done') {
        return resultResp;
      } else if (resultResp.status === 'error') {
        throw new Error(resultResp.result);
      }
      // 'pending' 或 'processing' 状态继续等待

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
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}

module.exports = SchedulerClient;
