// Type definitions for go-server-node-sdk

export interface ExecuteRequest {
  method: string;
  params: any;
}

export interface ExecuteResponse {
  taskId: string;
  status: 'pending' | 'processing' | 'done' | 'error';
}

export interface ResultResponse {
  taskId: string;
  status: 'pending' | 'processing' | 'done' | 'error';
  result: any;
}

export interface WorkerConfig {
  schedulerURL: string;
  workerGroup: string;
  maxRetry?: number;
  pingInterval?: number;
}

export interface MethodInfo {
  name: string;
  docs: string[];
}

export interface TaskMessage {
  type: 'task';
  taskId: string;
  method: string;
  params: any;
}

export interface EncryptedTaskMessage {
  type: 'encrypted_task';
  taskId: string;
  method: string;
  params: string;
  key: string;
  crypto: string;
}

export interface PingMessage {
  type: 'ping';
}

export interface PongMessage {
  type: 'pong';
}

export interface ResultMessage {
  type: 'result';
  taskId: string;
  result?: any;
  error?: string;
}

export type WebSocketMessage =
  | TaskMessage
  | EncryptedTaskMessage
  | PingMessage
  | PongMessage
  | ResultMessage;

export type TaskHandler = (params: any) => Promise<any>;

export declare class SchedulerClient {
  constructor(baseURL: string, timeout?: number);

  execute(method: string, params: any): Promise<ExecuteResponse>;
  executeEncrypted(method: string, key: string, salt: number, params: any): Promise<ExecuteResponse>;
  getResult(taskId: string): Promise<ResultResponse>;
  executeSync(
    method: string,
    params: any,
    timeout?: number
  ): Promise<ResultResponse>;
}

export declare class Worker {
  constructor(config: WorkerConfig);

  registerMethod(name: string, handler: TaskHandler, ...docs: string[]): void;
  start(): Promise<void>;
  stop(): void;
  getMethodsWithDocs(): MethodInfo[];
}

export declare class RetryClient extends SchedulerClient {
  constructor(baseURL: string, maxRetries?: number, retryDelay?: number, timeout?: number);

  executeWithRetry(method: string, params: any): Promise<ExecuteResponse>;
  executeEncryptedWithRetry(method: string, key: string, salt: number, params: any): Promise<ExecuteResponse>;
}

export declare function call(
  host: string,
  method: string,
  params: any
): Promise<any>;

export declare function callEncrypted(
  host: string,
  method: string,
  key: string,
  salt: number,
  params: any
): Promise<any>;

export { SchedulerClient, RetryClient, Worker, call, callEncrypted };
