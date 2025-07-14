// Package goserver provides a distributed task scheduling system with WebSocket-based communication.
package goserver

import (
	_ "embed"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-enols/go-log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	TaskStatusPending    = "pending"
	TaskStatusProcessing = "processing"
	TaskStatusDone       = "done"
	TaskStatusError      = "error"

	// Error messages
	WorkerNotFoundMsg      = "Worker not found"
	MethodNotFoundMsg      = "Method not found"
	TaskExecutionFailedMsg = "Task execution failed"
	WorkerTimeoutMsg       = "Worker timeout"
	WorkerDisconnectedMsg  = "Worker disconnected"
)

//go:embed index.html
var HTML string

type Scheduler struct {
	workers   sync.Map // map[string]*Worker
	tasks     sync.Map // map[string]*Task
	mu        sync.RWMutex
	upgrader  websocket.Upgrader
	taskQueue chan *Task
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		taskQueue: make(chan *Task, 100),
	}
}

func (s *Scheduler) handleWorkerConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Worker connection failed:", err)
		return
	}

	// 获取客户端IP地址
	clientIP := s.getClientIP(r)

	worker, err := s.registerWorker(conn, clientIP)
	if err != nil {
		return
	}

	// 启动心跳检测和消息处理
	go s.handleWorkerHeartbeat(worker, conn)
	go s.handleWorkerMessages(worker, conn)
}

func (s *Scheduler) registerWorker(conn *websocket.Conn, clientIP string) (*Worker, error) {
	// 接收Worker注册信息
	var reg struct {
		ID      string                   `json:"id"`
		Group   string                   `json:"group"`
		Methods []map[string]interface{} `json:"methods"`
	}
	if err := conn.ReadJSON(&reg); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Failed to close connection: %v", closeErr)
		}
		return nil, err
	}

	reg.ID = uuid.NewString()

	// 解析方法信息
	methods := s.parseMethods(reg.Methods)

	worker := &Worker{
		ID:       reg.ID,
		Conn:     conn,
		Methods:  methods,
		LastPing: time.Now().UnixNano(),
		Count:    0,
		IP:       clientIP,
		Group:    reg.Group,
	}

	s.workers.Store(worker.ID, worker)
	log.Printf("Worker registered: %s from %s, group: %s (methods: %v)", worker.ID, clientIP, reg.Group, worker.Methods)
	return worker, nil
}

func (s *Scheduler) parseMethods(methodsData []map[string]interface{}) []MethodInfo {
	methods := make([]MethodInfo, 0, len(methodsData))
	for _, methodData := range methodsData {
		if name, ok := methodData["name"].(string); ok {
			docs := []string{}
			if docsInterface, exists := methodData["docs"]; exists {
				if docsSlice, ok := docsInterface.([]interface{}); ok {
					for _, doc := range docsSlice {
						if docStr, ok := doc.(string); ok {
							docs = append(docs, docStr)
						}
					}
				}
			}
			methods = append(methods, MethodInfo{
				Name: name,
				Docs: docs,
			})
		}
	}
	return methods
}

func (s *Scheduler) handleWorkerHeartbeat(worker *Worker, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Failed to close connection: %v", closeErr)
		}
	}()

	for {
		<-ticker.C
		if s.checkWorkerTimeout(worker) {
			return
		}
		if s.sendPing(worker, conn) {
			return
		}
	}
}

func (s *Scheduler) checkWorkerTimeout(worker *Worker) bool {
	lastPingNano := atomic.LoadInt64(&worker.LastPing)
	lastPing := time.Unix(0, lastPingNano)

	if time.Since(lastPing) > 90*time.Second {
		s.cleanupWorkerTasks(worker, WorkerTimeoutMsg)
		log.Printf("Worker timeout: %s", worker.ID)
		return true
	}
	return false
}

func (s *Scheduler) sendPing(worker *Worker, conn *websocket.Conn) bool {
	worker.ConnMu.Lock()
	err := conn.WriteJSON(map[string]string{"type": "ping"})
	worker.ConnMu.Unlock()
	if err != nil {
		s.cleanupWorkerTasks(worker, WorkerDisconnectedMsg)
		log.Printf("Worker disconnected: %s", worker.ID)
		return true
	}
	return false
}

func (s *Scheduler) cleanupWorkerTasks(worker *Worker, errorMsg string) {
	s.mu.Lock()
	s.tasks.Range(func(key, value interface{}) bool {
		task := value.(*Task)
		if task.Worker != nil && task.Worker.ID == worker.ID && task.Status == TaskStatusProcessing {
			task.Status = TaskStatusError
			task.Result = errorMsg
			atomic.AddInt64(&task.Worker.Count, -1)
			task.Worker = nil
			log.Printf("Task %s failed due to worker %s issue", task.ID, worker.ID)
		}
		return true
	})
	s.workers.Delete(worker.ID)
	s.mu.Unlock()
}

func (s *Scheduler) handleWorkerMessages(worker *Worker, conn *websocket.Conn) {
	for {
		var msg struct {
			Type   string          `json:"type"`
			TaskID string          `json:"taskId"`
			Result json.RawMessage `json:"result"`
			Error  string          `json:"error"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			s.cleanupWorkerTasks(worker, WorkerDisconnectedMsg)
			log.Printf("Worker %s disconnected: %v", worker.ID, err)
			return
		}

		switch msg.Type {
		case "pong":
			atomic.StoreInt64(&worker.LastPing, time.Now().UnixNano())
		case "result":
			s.processTaskResult(msg)
		}
	}
}

func (s *Scheduler) processTaskResult(msg struct {
	Type   string          `json:"type"`
	TaskID string          `json:"taskId"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value, exists := s.tasks.Load(msg.TaskID); exists {
		task := value.(*Task)
		if msg.Error != "" {
			task.Status = TaskStatusError
			task.Result = msg.Error
		} else {
			task.Status = TaskStatusDone
			task.Result = msg.Result
		}
		if task.Worker != nil {
			atomic.AddInt64(&task.Worker.Count, -1)
		}
	}
}

func (s *Scheduler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task := &Task{
		ID:      uuid.New().String(),
		Method:  req.Method,
		Params:  req.Params,
		Status:  TaskStatusPending,
		Created: time.Now(),
	}

	s.mu.Lock()
	s.tasks.Store(task.ID, task)

	// 在同一个锁内完成worker选择和状态更新，避免竞态条件
	var selectedWorker *Worker
	minCount := math.MaxInt

	// 查找可用Worker并验证其连接状态
	s.workers.Range(func(key, value interface{}) bool {
		worker := value.(*Worker)
		// 使用原子操作检查worker连接是否仍然有效
		lastPingNano := atomic.LoadInt64(&worker.LastPing)
		lastPing := time.Unix(0, lastPingNano)
		if time.Since(lastPing) > 60*time.Second {
			return true // 跳过可能已断线的worker
		}

		for _, method := range worker.Methods {
			if method.Name == req.Method {
				// 使用原子操作读取计数
				workerCount := atomic.LoadInt64(&worker.Count)
				if workerCount < int64(minCount) {
					selectedWorker = worker
					minCount = int(workerCount)
				}
				break // 找到匹配方法就跳出内层循环
			}
		}
		return true
	})

	if selectedWorker == nil {
		task.Status = TaskStatusError
		task.Result = "服务不可用"
		s.mu.Unlock()
	} else {
		// 使用原子操作增加计数
		atomic.AddInt64(&selectedWorker.Count, 1)
		task.Worker = selectedWorker
		task.Status = TaskStatusProcessing
		s.mu.Unlock()

		// 发送任务到worker，使用互斥锁保护websocket写入
		selectedWorker.ConnMu.Lock()
		err := selectedWorker.Conn.WriteJSON(map[string]interface{}{
			"type":   "task",
			"taskId": task.ID,
			"method": task.Method,
			"params": task.Params,
		})
		selectedWorker.ConnMu.Unlock()
		if err != nil {
			// 发送失败时使用原子操作回滚状态
			atomic.AddInt64(&selectedWorker.Count, -1)
			s.mu.Lock()
			task.Status = TaskStatusError
			task.Result = "Worker连接异常: " + err.Error()
			task.Worker = nil
			s.mu.Unlock()
			log.Printf("Failed to send task %s to worker %s: %v", task.ID, selectedWorker.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	s.mu.RLock()
	status := task.Status
	s.mu.RUnlock()
	if err := json.NewEncoder(w).Encode(map[string]string{
		"taskId": task.ID,
		"status": string(status),
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (s *Scheduler) handleResult(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Path[len("/api/result/"):]

	value, exists := s.tasks.Load(taskID)
	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	task := value.(*Task)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"taskId": task.ID,
		"status": task.Status,
		"result": task.Result,
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (s *Scheduler) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(HTML)); err != nil {
		log.Printf("Failed to write HTML response: %v", err)
	}
}

func (s *Scheduler) handleStatus(w http.ResponseWriter, r *http.Request) {
	var workers []WorkerInfo
	totalMethods := 0

	s.workers.Range(func(key, value interface{}) bool {
		worker := value.(*Worker)
		totalMethods += len(worker.Methods)
		// 使用原子操作读取worker状态
		lastPingNano := atomic.LoadInt64(&worker.LastPing)
		count := atomic.LoadInt64(&worker.Count)
		workers = append(workers, WorkerInfo{
			ID:       worker.ID,
			Methods:  worker.Methods,
			LastPing: time.Unix(0, lastPingNano),
			Count:    count,
			IP:       worker.IP,
			Group:    worker.Group,
		})
		return true
	})

	totalTasks := 0
	s.tasks.Range(func(key, value interface{}) bool {
		totalTasks++
		return true
	})

	response := map[string]interface{}{
		"workers":      workers,
		"totalMethods": totalMethods,
		"totalTasks":   totalTasks,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode status response: %v", err)
	}
}

// getClientIP 获取客户端真实IP地址
func (s *Scheduler) getClientIP(r *http.Request) string {
	// 检查 X-Forwarded-For 头
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For 可能包含多个IP，取第一个
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// 检查 X-Real-IP 头
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// 使用 RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Scheduler) Start(addr, key string) {
	http.HandleFunc("/", s.handleUI)
	http.HandleFunc("/api/worker/connect/"+key, s.handleWorkerConnection)
	http.HandleFunc("/api/execute", s.handleExecute)
	http.HandleFunc("/api/result/", s.handleResult)
	http.HandleFunc("/api/status", s.handleStatus)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Scheduler started on %s", addr)
	log.Printf("Web UI available at: http://localhost%s", addr)
	log.Fatal(server.ListenAndServe())
}
