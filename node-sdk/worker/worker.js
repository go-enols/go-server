const WebSocket = require('ws');
const EventEmitter = require('events');

class Worker extends EventEmitter {
  constructor(config) {
    super();
    this.config = {
      schedulerURL: config.schedulerURL,
      workerGroup: config.workerGroup,
      maxRetry: config.maxRetry || 3,
      pingInterval: config.pingInterval || 30, // 秒
    };

    this.ws = null;
    this.methods = new Map();
    this.docs = new Map();
    this.running = false;
    this.reconnect = false;
    this.pingTimer = null;
    this.reconnectTimer = null;
  }

  /**
   * 注册方法
   * @param {string} name - 方法名
   * @param {Function} handler - 处理函数，签名: async (params) => result
   * @param {string[]} docs - 文档说明
   */
  registerMethod(name, handler, ...docs) {
    if (typeof handler !== 'function') {
      throw new Error('Handler must be a function');
    }

    this.methods.set(name, handler);
    this.docs.set(name, docs);
  }

  /**
   * 启动Worker
   */
  async start() {
    this.running = true;
    this.reconnect = true;

    await this.connect();
    this.startKeepAlive();

    console.log(`Worker ${this.config.workerGroup} started`);
  }

  /**
   * 连接到调度器
   */
  async connect() {
    return new Promise((resolve, reject) => {
      let retryCount = 0;

      const attemptConnect = () => {
        if (retryCount >= this.config.maxRetry) {
          reject(
            new Error(
              `Failed to connect after ${this.config.maxRetry} attempts`
            )
          );
          return;
        }

        try {
          this.ws = new WebSocket(this.config.schedulerURL);

          this.ws.on('open', () => {
            console.log('Connected to scheduler');

            // 发送注册信息
            const registration = {
              group: this.config.workerGroup,
              methods: this.getMethodsWithDocs(),
            };

            this.ws.send(JSON.stringify(registration));
            resolve();
          });

          this.ws.on('message', (data) => {
            this.handleMessage(data);
          });

          this.ws.on('close', (code, reason) => {
            console.log(`Connection closed: ${code} ${reason}`);
            this.ws = null;

            if (this.running && this.reconnect) {
              console.log('Attempting to reconnect in 5 seconds...');
              this.reconnectTimer = setTimeout(() => {
                this.connect().catch((err) => {
                  console.error('Reconnect failed:', err);
                });
              }, 5000);
            }
          });

          this.ws.on('error', (error) => {
            console.error('WebSocket error:', error);
            if (retryCount === 0) {
              retryCount++;
              setTimeout(attemptConnect, 1000 * retryCount);
            }
          });
        } catch {
          retryCount++;
          setTimeout(attemptConnect, 1000 * retryCount);
        }
      };

      attemptConnect();
    });
  }

  /**
   * 处理接收到的消息
   */
  async handleMessage(data) {
    try {
      const message = JSON.parse(data.toString());

      switch (message.type) {
        case 'task':
          this.handleTask(message.taskId, message.method, message.params);
          break;
        case 'ping':
          this.sendPong();
          break;
        default:
          console.log('Unknown message type:', message.type);
      }
    } catch (error) {
      console.error('Error handling message:', error);
    }
  }

  /**
   * 处理任务
   */
  async handleTask(taskId, method, params) {
    try {
      const handler = this.methods.get(method);
      if (!handler) {
        this.sendResult(taskId, null, new Error('Method not found'));
        return;
      }

      // 调用注册的方法
      const result = await handler(params);
      this.sendResult(taskId, result, null);
    } catch (error) {
      this.sendResult(taskId, null, error);
    }
  }

  /**
   * 发送结果到调度器
   */
  sendResult(taskId, result, error) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.error(
        `Cannot send result for task ${taskId}: connection is not open`
      );
      return;
    }

    const response = {
      type: 'result',
      taskId: taskId,
    };

    if (error) {
      response.error = error.message;
    } else {
      response.result = result;
    }

    try {
      this.ws.send(JSON.stringify(response));
    } catch (err) {
      console.error(`Failed to send result for task ${taskId}:`, err);
    }
  }

  /**
   * 发送pong响应
   */
  sendPong() {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return;
    }

    try {
      this.ws.send(JSON.stringify({ type: 'pong' }));
    } catch (error) {
      console.error('Failed to send pong:', error);
    }
  }

  /**
   * 启动心跳保持
   */
  startKeepAlive() {
    this.pingTimer = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        try {
          this.ws.send(JSON.stringify({ type: 'ping' }));
        } catch (error) {
          console.error('Ping failed:', error);
        }
      }
    }, this.config.pingInterval * 1000);
  }

  /**
   * 停止Worker
   */
  stop() {
    if (!this.running) {
      return;
    }

    this.running = false;
    this.reconnect = false;

    // 清理定时器
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    // 关闭WebSocket连接
    if (this.ws) {
      this.ws.close(1000, 'Worker stopped');
      this.ws = null;
    }

    console.log('Worker stopped');
  }

  /**
   * 获取方法和文档信息
   */
  getMethodsWithDocs() {
    const methods = [];
    for (const [name] of this.methods) {
      methods.push({
        name: name,
        docs: this.docs.get(name) || [],
      });
    }
    return methods;
  }
}

module.exports = Worker;
