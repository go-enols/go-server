package goserver

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Task struct {
	ID      string
	Method  string
	Params  json.RawMessage
	Result  any
	Status  string // "pending", "processing", "done", "error"
	Worker  *Worker
	Created time.Time
}

type Worker struct {
	ID       string
	Conn     *websocket.Conn
	Methods  []string
	LastPing time.Time
	Count    int
}

type Scheduler struct {
	workers   map[string]*Worker
	tasks     map[string]*Task
	mu        sync.RWMutex
	upgrader  websocket.Upgrader
	taskQueue chan *Task
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		workers:   make(map[string]*Worker),
		tasks:     make(map[string]*Task),
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
		ID      string   `json:"id"`
		Methods []string `json:"methods"`
	}
	if err := conn.ReadJSON(&reg); err != nil {
		conn.Close()
		return
	}

	reg.ID = uuid.NewString()

	worker := &Worker{
		ID:       reg.ID,
		Conn:     conn,
		Methods:  reg.Methods,
		LastPing: time.Now(),
		Count:    0,
	}

	s.mu.Lock()
	s.workers[worker.ID] = worker
	s.mu.Unlock()

	log.Printf("Worker registered: %s (methods: %v)", worker.ID, worker.Methods)

	// 心跳检测
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer conn.Close()

		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				if time.Since(worker.LastPing) > 45*time.Second {
					// 处理该worker正在执行的任务
					for _, task := range s.tasks {
						if task.Worker != nil && task.Worker.ID == worker.ID && task.Status == "processing" {
							task.Status = "error"
							task.Result = "Worker timeout during task execution"
							log.Printf("Task %s failed due to worker %s timeout", task.ID, worker.ID)
						}
					}
					delete(s.workers, worker.ID)
					s.mu.Unlock()
					log.Printf("Worker timeout: %s", worker.ID)
					return
				}
				s.mu.Unlock()

				if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
					s.mu.Lock()
					// 处理该worker正在执行的任务
					for _, task := range s.tasks {
						if task.Worker != nil && task.Worker.ID == worker.ID && task.Status == "processing" {
							task.Status = "error"
							task.Result = "Worker disconnected during task execution"
							log.Printf("Task %s failed due to worker %s ping failure", task.ID, worker.ID)
						}
					}
					delete(s.workers, worker.ID)
					s.mu.Unlock()
					log.Printf("Worker disconnected: %s", worker.ID)
					return
				}
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
				for _, task := range s.tasks {
					if task.Worker != nil && task.Worker.ID == worker.ID && task.Status == "processing" {
						task.Status = "error"
						task.Result = "Worker disconnected during task execution"
						log.Printf("Task %s failed due to worker %s disconnection", task.ID, worker.ID)
					}
				}
				delete(s.workers, worker.ID)
				s.mu.Unlock()
				log.Printf("Worker %s disconnected: %v", worker.ID, err)
				return
			}

			switch msg.Type {
			case "pong":
				s.mu.Lock()
				worker.LastPing = time.Now()
				s.mu.Unlock()
			case "result":
				s.mu.Lock()
				if task, exists := s.tasks[msg.TaskID]; exists {
					if msg.Error != "" {
						task.Status = "error"
						task.Result = msg.Error
					} else {
						task.Status = "done"
						task.Result = msg.Result
					}
					// 任务完成后减少worker计数
					if task.Worker != nil {
						task.Worker.Count--
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
	s.tasks[task.ID] = task
	s.mu.Unlock()

	var selectedWorker *Worker
	minCount := math.MaxInt
	// 查找可用Worker - 修复负载均衡逻辑
	s.mu.RLock()
	for _, worker := range s.workers {
		for _, method := range worker.Methods {
			if method == req.Method {
				if worker.Count < minCount {
					selectedWorker = worker
					minCount = worker.Count
				}
				break // 找到支持的方法就跳出内层循环
			}
		}
	}
	s.mu.RUnlock()

	if selectedWorker == nil {
		s.mu.Lock()
		task.Status = "error"
		task.Result = "服务不可用"
		s.mu.Unlock()
	} else {
		// 先增加计数，防止并发分配到同一个worker
		s.mu.Lock()
		selectedWorker.Count++
		task.Worker = selectedWorker
		task.Status = "processing"
		s.mu.Unlock()

		if err := selectedWorker.Conn.WriteJSON(map[string]interface{}{
			"type":   "task",
			"taskId": task.ID,
			"method": task.Method,
			"params": task.Params,
		}); err != nil {
			// 发送失败时回滚状态
			s.mu.Lock()
			selectedWorker.Count--
			task.Status = "error"
			task.Result = err.Error()
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
		"status": status,
	})
}

func (s *Scheduler) handleResult(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Path[len("/api/result/"):]

	s.mu.RLock()
	task, exists := s.tasks[taskID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"taskId": task.ID,
		"status": task.Status,
		"result": task.Result,
	})
}

func (s *Scheduler) Start(addr string) {
	http.HandleFunc("/api/worker/connect", s.handleWorkerConnection)
	http.HandleFunc("/api/execute", s.handleExecute)
	http.HandleFunc("/api/result/", s.handleResult)

	log.Printf("Scheduler started on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
