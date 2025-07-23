// Package goserver defines the core types and structures for the distributed task scheduling system.
package goserver

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TaskStatus 任务状态
type TaskStatus string

type Task struct {
	ID      string
	Method  string
	Params  json.RawMessage
	Result  any
	Worker  *Worker
	Created time.Time
	Status  TaskStatus // "pending", "processing", "done", "error"
}

type Worker struct {
	Conn     *websocket.Conn
	Methods  []MethodInfo
	ConnMu   sync.Mutex // 保护websocket连接的并发写入
	LastPing int64      // 使用原子操作，存储Unix纳秒时间戳
	Count    int64      // 使用原子操作
	ID       string
	IP       string     // Worker的IP地址
	Group    string     // Worker的分组
}

type WorkerInfo struct {
	Methods  []MethodInfo `json:"methods"`
	LastPing time.Time    `json:"lastPing"`
	Count    int64        `json:"count"`
	ID       string       `json:"id"`
	IP       string       `json:"ip"`
	Group    string       `json:"group"`
}

type MethodInfo struct {
	Name string   `json:"name"`
	Docs []string `json:"docs"`
}

// EncryptedRequest 端到端加密请求结构
type EncryptedRequest struct {
	Key    string `json:"key"`    // 用户加盐后的密钥
	Method string `json:"method"` // 需要调用的方法名
	Params string `json:"params"` // 用户加密后的调用参数
	Crypto string `json:"crypto"` // 用户加盐的实际数据(如位运算)
}

// EncryptedTask 端到端加密任务结构(调度器只做中转)
type EncryptedTask struct {
	Result  interface{} `json:"result"`
	Worker  *Worker     `json:"-"`
	Created time.Time   `json:"created"`
	ID      string      `json:"id"`
	Key     string      `json:"key"`
	Method  string      `json:"method"`
	Params  string      `json:"params"`
	Crypto  string      `json:"crypto"`
	Status  TaskStatus  `json:"status"`
}

// TaskResultMessage represents the message structure for task results
type TaskResultMessage struct {
	Type   string          `json:"type"`
	TaskID string          `json:"taskId"`
	Error  string          `json:"error"`
	Result json.RawMessage `json:"result"`
}
