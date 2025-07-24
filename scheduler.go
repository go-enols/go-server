// Package goserver provides a distributed task scheduling system with WebSocket-based communication.
package goserver

import (
	_ "embed"
	"encoding/json"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	ServiceUnavailableMsg  = "服务不可用"
)

//go:embed index.html
var HTML string

type Scheduler struct {
	workers        sync.Map                  // map[string]*Worker
	tasks          sync.Map                  // map[string]*Task
	encryptedTasks map[string]*EncryptedTask // 加密任务存储
	mu             sync.RWMutex
	upgrader       websocket.Upgrader
	taskQueue      chan *Task
	cleanupTicker  *time.Ticker // 定期清理定时器
	stopCleanup    chan bool    // 停止清理信号
}

func NewScheduler() *Scheduler {
	s := &Scheduler{
		encryptedTasks: make(map[string]*EncryptedTask),
		upgrader:       websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		taskQueue:      make(chan *Task, 100),
		cleanupTicker:  time.NewTicker(1 * time.Minute), // 每分钟检查一次
		stopCleanup:    make(chan bool),
	}
	// 启动任务清理协程
	go s.startTaskCleanup()
	return s
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
			task.Completed = time.Now() // 记录任务完成时间
			atomic.AddInt64(&task.Worker.Count, -1)
			task.Worker = nil
			log.Printf("Task %s failed due to worker %s issue", task.ID, worker.ID)
		}
		return true
	})

	// 清理加密任务
	for taskID, encryptedTask := range s.encryptedTasks {
		if encryptedTask.Worker != nil && encryptedTask.Worker.ID == worker.ID && encryptedTask.Status == TaskStatusProcessing {
			encryptedTask.Status = TaskStatusError
			encryptedTask.Result = errorMsg
			encryptedTask.Completed = time.Now() // 记录任务完成时间
			atomic.AddInt64(&encryptedTask.Worker.Count, -1)
			encryptedTask.Worker = nil
			log.Printf("Encrypted task %s failed due to worker %s issue", taskID, worker.ID)
		}
	}

	s.workers.Delete(worker.ID)
	s.mu.Unlock()
}

// selectWorker 选择可用的Worker来执行指定方法
func (s *Scheduler) selectWorker(method string) *Worker {
	var selectedWorker *Worker
	minCount := math.MaxInt

	s.workers.Range(func(key, value interface{}) bool {
		worker := value.(*Worker)
		lastPingNano := atomic.LoadInt64(&worker.LastPing)
		lastPing := time.Unix(0, lastPingNano)
		if time.Since(lastPing) > 60*time.Second {
			return true
		}

		for _, workerMethod := range worker.Methods {
			if workerMethod.Name == method {
				workerCount := atomic.LoadInt64(&worker.Count)
				if workerCount < int64(minCount) {
					selectedWorker = worker
					minCount = int(workerCount)
				}
				break
			}
		}
		return true
	})

	return selectedWorker
}

func (s *Scheduler) handleWorkerMessages(worker *Worker, conn *websocket.Conn) {
	for {
		var msg TaskResultMessage
		if err := conn.ReadJSON(&msg); err != nil {
			s.cleanupWorkerTasks(worker, WorkerDisconnectedMsg)
			log.Printf("Worker %s disconnected: %v", worker.ID, err)
			return
		}

		switch msg.Type {
		case "pong":
			atomic.StoreInt64(&worker.LastPing, time.Now().UnixNano())
		case "result":
			s.processTaskResult(&msg)
		}
	}
}

func (s *Scheduler) processTaskResult(msg *TaskResultMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 处理普通任务结果
	if value, exists := s.tasks.Load(msg.TaskID); exists {
		task := value.(*Task)
		if msg.Error != "" {
			task.Status = TaskStatusError
			task.Result = msg.Error
		} else {
			task.Status = TaskStatusDone
			task.Result = msg.Result
		}
		task.Completed = time.Now() // 记录任务完成时间
		if task.Worker != nil {
			atomic.AddInt64(&task.Worker.Count, -1)
		}
		return
	}

	// 处理加密任务结果
	if encryptedTask, exists := s.encryptedTasks[msg.TaskID]; exists {
		if msg.Error != "" {
			encryptedTask.Status = TaskStatusError
			encryptedTask.Result = msg.Error
		} else {
			encryptedTask.Status = TaskStatusDone
			encryptedTask.Result = msg.Result
		}
		encryptedTask.Completed = time.Now() // 记录任务完成时间
		if encryptedTask.Worker != nil {
			atomic.AddInt64(&encryptedTask.Worker.Count, -1)
		}
	}
}

func (s *Scheduler) handleEncryptedExecute(w http.ResponseWriter, r *http.Request) {
	var req EncryptedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	encryptedTask := &EncryptedTask{
		ID:      uuid.New().String(),
		Key:     req.Key,
		Method:  req.Method,
		Params:  req.Params,
		Crypto:  req.Crypto,
		Status:  TaskStatusPending,
		Created: time.Now(),
	}

	s.mu.Lock()
	s.encryptedTasks[encryptedTask.ID] = encryptedTask

	// 选择可用的Worker
	selectedWorker := s.selectWorker(req.Method)

	if selectedWorker == nil {
		encryptedTask.Status = TaskStatusError
		encryptedTask.Result = ServiceUnavailableMsg
		s.mu.Unlock()
	} else {
		atomic.AddInt64(&selectedWorker.Count, 1)
		encryptedTask.Worker = selectedWorker
		encryptedTask.Status = TaskStatusProcessing
		s.mu.Unlock()

		// 将加密数据原封不动地发送给worker
		selectedWorker.ConnMu.Lock()
		err := selectedWorker.Conn.WriteJSON(map[string]interface{}{
			"type":   "encrypted_task",
			"taskId": encryptedTask.ID,
			"key":    encryptedTask.Key,
			"method": encryptedTask.Method,
			"params": encryptedTask.Params,
			"crypto": encryptedTask.Crypto,
		})
		selectedWorker.ConnMu.Unlock()
		if err != nil {
			atomic.AddInt64(&selectedWorker.Count, -1)
			s.mu.Lock()
			encryptedTask.Status = TaskStatusError
			encryptedTask.Result = "Worker连接异常: " + err.Error()
			encryptedTask.Worker = nil
			s.mu.Unlock()
			log.Printf("Failed to send encrypted task %s to worker %s: %v", encryptedTask.ID, selectedWorker.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	s.mu.RLock()
	status := encryptedTask.Status
	s.mu.RUnlock()
	if err := json.NewEncoder(w).Encode(map[string]string{
		"taskId": encryptedTask.ID,
		"status": string(status),
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
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
	selectedWorker := s.selectWorker(req.Method)

	if selectedWorker == nil {
		task.Status = TaskStatusError
		task.Result = ServiceUnavailableMsg
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

func (s *Scheduler) handleEncryptedResult(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Path[len("/api/encrypted/result/"):]

	s.mu.Lock()
	encryptedTask, exists := s.encryptedTasks[taskID]
	if !exists {
		s.mu.Unlock()
		http.Error(w, "encrypted task not found", http.StatusNotFound)
		return
	}

	// 复制任务信息用于响应
	response := map[string]interface{}{
		"taskId": encryptedTask.ID,
		"status": encryptedTask.Status,
		"result": encryptedTask.Result,
	}

	// 如果任务已完成，获取结果后立即删除
	if encryptedTask.Status == TaskStatusDone || encryptedTask.Status == TaskStatusError {
		delete(s.encryptedTasks, taskID)
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
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

	// 复制任务信息用于响应
	response := map[string]interface{}{
		"taskId": task.ID,
		"status": task.Status,
		"result": task.Result,
	}

	// 如果任务已完成，获取结果后立即删除
	if task.Status == TaskStatusDone || task.Status == TaskStatusError {
		s.tasks.Delete(taskID)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
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

	// 统计任务信息
	totalTasks := 0
	activeTasks := 0
	completedTasks := 0
	s.tasks.Range(func(key, value interface{}) bool {
		task := value.(*Task)
		totalTasks++
		if task.Status == TaskStatusPending || task.Status == TaskStatusProcessing {
			activeTasks++
		} else {
			completedTasks++
		}
		return true
	})

	// 统计加密任务信息
	s.mu.RLock()
	totalEncryptedTasks := len(s.encryptedTasks)
	activeEncryptedTasks := 0
	completedEncryptedTasks := 0
	for _, encryptedTask := range s.encryptedTasks {
		if encryptedTask.Status == TaskStatusPending || encryptedTask.Status == TaskStatusProcessing {
			activeEncryptedTasks++
		} else {
			completedEncryptedTasks++
		}
	}
	s.mu.RUnlock()

	response := map[string]interface{}{
		"workers":                 workers,
		"totalMethods":            totalMethods,
		"totalTasks":              totalTasks,
		"activeTasks":             activeTasks,
		"completedTasks":          completedTasks,
		"totalEncryptedTasks":     totalEncryptedTasks,
		"activeEncryptedTasks":    activeEncryptedTasks,
		"completedEncryptedTasks": completedEncryptedTasks,
		"memoryInfo": map[string]interface{}{
			"description":      "Tasks are automatically cleaned up after completion: immediately when result is retrieved, or after 5 minutes",
			"cleanupInterval":  "1 minute",
			"autoCleanupDelay": "5 minutes after completion",
		},
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

// startTaskCleanup 启动任务清理协程
func (s *Scheduler) startTaskCleanup() {
	for {
		select {
		case <-s.cleanupTicker.C:
			s.cleanupExpiredTasks()
		case <-s.stopCleanup:
			s.cleanupTicker.Stop()
			return
		}
	}
}

// cleanupExpiredTasks 清理过期任务（完成后5分钟）
func (s *Scheduler) cleanupExpiredTasks() {
	now := time.Now()
	expireTime := 5 * time.Minute
	cleanedCount := 0
	encryptedCleanedCount := 0

	// 清理普通任务
	s.tasks.Range(func(key, value interface{}) bool {
		task := value.(*Task)
		// 检查任务是否已完成且超过5分钟
		if (task.Status == TaskStatusDone || task.Status == TaskStatusError) &&
			!task.Completed.IsZero() &&
			now.Sub(task.Completed) > expireTime {
			s.tasks.Delete(key)
			cleanedCount++
			log.Printf("Task %s cleaned up after 5 minutes", task.ID)
		}
		return true
	})

	// 清理加密任务
	s.mu.Lock()
	for taskID, encryptedTask := range s.encryptedTasks {
		// 检查任务是否已完成且超过5分钟
		if (encryptedTask.Status == TaskStatusDone || encryptedTask.Status == TaskStatusError) &&
			!encryptedTask.Completed.IsZero() &&
			now.Sub(encryptedTask.Completed) > expireTime {
			delete(s.encryptedTasks, taskID)
			encryptedCleanedCount++
			log.Printf("Encrypted task %s cleaned up after 5 minutes", taskID)
		}
	}
	s.mu.Unlock()

	if cleanedCount > 0 || encryptedCleanedCount > 0 {
		log.Printf("Cleanup completed: %d tasks, %d encrypted tasks removed", cleanedCount, encryptedCleanedCount)
	}
}

// Stop 停止调度器并清理资源
func (s *Scheduler) Stop() {
	close(s.stopCleanup)
}

func (s *Scheduler) Start(addr, key string) {
	http.HandleFunc("/", s.handleUI)
	http.HandleFunc("/api/worker/connect/"+key, s.handleWorkerConnection)
	http.HandleFunc("/api/execute", s.handleExecute)
	http.HandleFunc("/api/encrypted/execute", s.handleEncryptedExecute)
	http.HandleFunc("/api/result/", s.handleResult)
	http.HandleFunc("/api/encrypted/result/", s.handleEncryptedResult)
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
