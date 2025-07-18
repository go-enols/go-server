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
	Status  TaskStatus // "pending", "processing", "done", "error"
	Worker  *Worker
	Created time.Time
}

type Worker struct {
	ID       string
	Conn     *websocket.Conn
	Methods  []MethodInfo
	LastPing int64      // 使用原子操作，存储Unix纳秒时间戳
	Count    int64      // 使用原子操作
	ConnMu   sync.Mutex // 保护websocket连接的并发写入
	IP       string     // Worker的IP地址
	Group    string     // Worker的分组
}

type WorkerInfo struct {
	ID       string       `json:"id"`
	Methods  []MethodInfo `json:"methods"`
	LastPing time.Time    `json:"lastPing"`
	Count    int64        `json:"count"`
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
	ID      string
	Key     string // 用户加盐后的密钥(原封不动传递)
	Method  string // 方法名(明文，用于路由)
	Params  string // 加密后的参数(原封不动传递)
	Crypto  string // 加盐数据(原封不动传递)
	Result  any
	Status  TaskStatus
	Worker  *Worker
	Created time.Time
}
