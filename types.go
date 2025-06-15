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
}

type WorkerInfo struct {
	ID       string       `json:"id"`
	Methods  []MethodInfo `json:"methods"`
	LastPing time.Time    `json:"lastPing"`
	Count    int64        `json:"count"`
}

type MethodInfo struct {
	Name string   `json:"name"`
	Docs []string `json:"docs"`
}
