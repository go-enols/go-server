package goserver

import (
	_ "embed"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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

	// 接收Worker注册信息
	var reg struct {
		ID      string                   `json:"id"`
		Group   string                   `json:"group"`
		Methods []map[string]interface{} `json:"methods"`
	}
	if err := conn.ReadJSON(&reg); err != nil {
		conn.Close()
		return
	}

	reg.ID = uuid.NewString()

	// 解析方法信息
	methods := make([]MethodInfo, 0, len(reg.Methods))
	for _, methodData := range reg.Methods {
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

	worker := &Worker{
		ID:       reg.ID,
		Conn:     conn,
		Methods:  methods,
		LastPing: time.Now().UnixNano(),
		Count:    0,
	}

	s.workers.Store(worker.ID, worker)

	log.Printf("Worker registered: %s (methods: %v)", worker.ID, worker.Methods)

	// 心跳检测
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer conn.Close()

		for {
				<-ticker.C
				// 使用原子操作读取LastPing
				lastPingNano := atomic.LoadInt64(&worker.LastPing)
				lastPing := time.Unix(0, lastPingNano)
				
				// 增加超时时间到90秒，避免网络波动导致的误判
				if time.Since(lastPing) > 90*time.Second {
					s.mu.Lock()
					// 处理该worker正在执行的任务
					s.tasks.Range(func(key, value interface{}) bool {
						task := value.(*Task)
						if task.Worker != nil && task.Worker.ID == worker.ID && task.Status == "processing" {
							task.Status = "error"
							task.Result = "Worker timeout during task execution"
							// 使用原子操作减少计数
							atomic.AddInt64(&task.Worker.Count, -1)
							task.Worker = nil
							log.Printf("Task %s failed due to worker %s timeout", task.ID, worker.ID)
						}
						return true
					})
					s.workers.Delete(worker.ID)
					s.mu.Unlock()
					log.Printf("Worker timeout: %s", worker.ID)
					return
				}

			// 发送ping前检查连接状态，使用互斥锁保护websocket写入
			worker.ConnMu.Lock()
			err := conn.WriteJSON(map[string]string{"type": "ping"})
			worker.ConnMu.Unlock()
			if err != nil {
				s.mu.Lock()
				// 处理该worker正在执行的任务
				s.tasks.Range(func(key, value interface{}) bool {
					task := value.(*Task)
					if task.Worker != nil && task.Worker.ID == worker.ID && task.Status == "processing" {
						task.Status = "error"
						task.Result = "Worker disconnected during task execution"
						// 使用原子操作减少计数
						atomic.AddInt64(&task.Worker.Count, -1)
						task.Worker = nil
						log.Printf("Task %s failed due to worker %s ping failure", task.ID, worker.ID)
					}
					return true
				})
				s.workers.Delete(worker.ID)
				s.mu.Unlock()
				log.Printf("Worker disconnected: %s", worker.ID)
				return
			}

		}
	}()

	// 处理Worker消息
	go func() {
		for {
			var msg struct {
				Type   string          `json:"type"`
				TaskID string          `json:"taskId"`
				Result json.RawMessage `json:"result"`
				Error  string          `json:"error"`
			}
			if err := conn.ReadJSON(&msg); err != nil {
				s.mu.Lock()
				// 处理该worker正在执行的任务
				s.tasks.Range(func(key, value interface{}) bool {
					task := value.(*Task)
					if task.Worker != nil && task.Worker.ID == worker.ID && task.Status == "processing" {
						task.Status = "error"
						task.Result = "Worker disconnected during task execution"
						// 使用原子操作减少计数
						atomic.AddInt64(&task.Worker.Count, -1)
						task.Worker = nil
						log.Printf("Task %s failed due to worker %s disconnection", task.ID, worker.ID)
					}
					return true
				})
				s.workers.Delete(worker.ID)
				s.mu.Unlock()
				log.Printf("Worker %s disconnected: %v", worker.ID, err)
				return
			}

			switch msg.Type {
			case "pong":
				// 使用原子操作更新LastPing
				atomic.StoreInt64(&worker.LastPing, time.Now().UnixNano())
			case "result":
				s.mu.Lock()
				if value, exists := s.tasks.Load(msg.TaskID); exists {
					task := value.(*Task)
					if msg.Error != "" {
						task.Status = "error"
						task.Result = msg.Error
					} else {
						task.Status = "done"
						task.Result = msg.Result
					}
					// 使用原子操作减少worker计数
					if task.Worker != nil {
						atomic.AddInt64(&task.Worker.Count, -1)
					}
				}
				s.mu.Unlock()
			}
		}
	}()
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
		Status:  "pending",
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
		task.Status = "error"
		task.Result = "服务不可用"
		s.mu.Unlock()
	} else {
		// 使用原子操作增加计数
		atomic.AddInt64(&selectedWorker.Count, 1)
		task.Worker = selectedWorker
		task.Status = "processing"
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
			task.Status = "error"
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
	json.NewEncoder(w).Encode(map[string]string{
		"taskId": task.ID,
		"status": string(status),
	})
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"taskId": task.ID,
		"status": task.Status,
		"result": task.Result,
	})
}

func (s *Scheduler) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(HTML))
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
	json.NewEncoder(w).Encode(response)
}

func (s *Scheduler) Start(addr string) {
	http.HandleFunc("/", s.handleUI)
	http.HandleFunc("/api/worker/connect", s.handleWorkerConnection)
	http.HandleFunc("/api/execute", s.handleExecute)
	http.HandleFunc("/api/result/", s.handleResult)
	http.HandleFunc("/api/status", s.handleStatus)

	log.Printf("Scheduler started on %s", addr)
	log.Printf("Web UI available at: http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
