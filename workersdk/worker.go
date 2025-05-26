package workersdk

import (
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Worker 配置
type Config struct {
	SchedulerURL string // 调度器地址
	WorkerGroup  string // Worker分组
	MaxRetry     int    // 连接重试次数
	PingInterval int    // 心跳间隔(秒)
}

// Method 方法定义
type Method struct {
	Name    string
	Handler interface{}
}

type Worker struct {
	config    Config
	conn      *websocket.Conn
	methods   map[string]reflect.Value
	running   bool
	stopChan  chan struct{}
	connMutex sync.Mutex // 保护连接的并发访问
	reconnect bool       // 是否自动重连
}

// 创建新Worker实例
func NewWorker(config Config) *Worker {
	return &Worker{
		config:   config,
		methods:  make(map[string]reflect.Value),
		stopChan: make(chan struct{}),
	}
}

// 注册方法
func (w *Worker) RegisterMethod(name string, handler interface{}) error {
	// 验证handler必须是函数
	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		return errors.New("handler must be a function")
	}

	// 验证函数签名: 必须有1个输入参数(JSON数据)和2个返回值(结果,错误)
	if handlerValue.Type().NumIn() != 1 || handlerValue.Type().NumOut() != 2 {
		return errors.New("handler signature must be: func(input json.RawMessage) (result interface{}, err error)")
	}

	w.methods[name] = handlerValue
	return nil
}

// 修改Start方法支持自动重连
func (w *Worker) Start() error {
	w.running = true
	w.reconnect = true

	if err := w.connect(); err != nil {
		return err
	}

	go w.keepAlive()
	go w.processTasks()

	log.Printf("Worker %s started", w.config.WorkerGroup)
	return nil
}

// 新增安全的连接方法
func (w *Worker) connect() error {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()

	var err error
	var retryCount int

	for retryCount < w.config.MaxRetry {
		w.conn, _, err = websocket.DefaultDialer.Dial(w.config.SchedulerURL, nil)
		if err == nil {
			// 发送注册信息
			registration := map[string]interface{}{
				"group":   w.config.WorkerGroup,
				"methods": w.getMethodNames(),
			}
			if err := w.conn.WriteJSON(registration); err == nil {
				return nil
			}
			w.conn.Close()
		}

		retryCount++
		time.Sleep(time.Second * time.Duration(retryCount))
	}

	return err
}

// 增强processTasks方法
func (w *Worker) processTasks() {
	for w.running {
		w.connMutex.Lock()
		conn := w.conn
		w.connMutex.Unlock()

		if conn == nil {
			if !w.reconnect {
				return
			}
			if err := w.connect(); err != nil {
				log.Printf("Reconnect failed: %v (retrying in 5s)", err)
				time.Sleep(5 * time.Second)
				continue
			}
			continue
		}

		var msg struct {
			Type   string          `json:"type"`
			TaskID string          `json:"taskId"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}

		if err := conn.ReadJSON(&msg); err != nil {
			if !w.running {
				return
			}

			if websocket.IsCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {

				log.Printf("Connection closed: %v", err)
			} else {
				log.Printf("Read error: %v", err)
			}

			// 关闭当前连接
			w.connMutex.Lock()
			if w.conn != nil {
				w.conn.Close()
				w.conn = nil
			}
			w.connMutex.Unlock()

			continue
		}

		if msg.Type == "task" {
			go w.handleTask(msg.TaskID, msg.Method, msg.Params)
		}
	}
}

// 修改Stop方法
func (w *Worker) Stop() {
	if !w.running {
		return
	}

	w.running = false
	w.reconnect = false
	close(w.stopChan)

	w.connMutex.Lock()
	if w.conn != nil {
		w.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		w.conn.Close()
		w.conn = nil
	}
	w.connMutex.Unlock()
}

// 心跳保持
func (w *Worker) keepAlive() {
	ticker := time.NewTicker(time.Duration(w.config.PingInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
				log.Printf("Ping failed: %v", err)
				w.Stop()
				return
			}
		case <-w.stopChan:
			return
		}
	}
}

// 处理单个任务
func (w *Worker) handleTask(taskID, method string, params json.RawMessage) {
	handler, exists := w.methods[method]
	if !exists {
		w.sendResult(taskID, nil, errors.New("method not found"))
		return
	}

	// 调用注册的方法
	results := handler.Call([]reflect.Value{reflect.ValueOf(params)})
	result := results[0].Interface()
	err, _ := results[1].Interface().(error)

	w.sendResult(taskID, result, err)
}

// 发送结果回调度器
func (w *Worker) sendResult(taskID string, result interface{}, err error) {
	response := map[string]interface{}{
		"type":   "result",
		"taskId": taskID,
	}

	if err != nil {
		response["error"] = err.Error()
	} else {
		response["result"] = result
	}

	if err := w.conn.WriteJSON(response); err != nil {
		log.Printf("Failed to send result for task %s: %v", taskID, err)
	}
}

// 获取所有注册的方法名
func (w *Worker) getMethodNames() []string {
	names := make([]string, 0, len(w.methods))
	for name := range w.methods {
		names = append(names, name)
	}
	return names
}
