// Package worker provides functionality for creating and managing distributed task workers.
package worker

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-enols/go-log"

	"github.com/gorilla/websocket"
)

// Config represents the configuration for a worker.
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
	docs      map[string][]string
	running   bool
	stopChan  chan struct{}
	connMutex sync.Mutex // 保护连接的并发访问
	reconnect bool       // 是否自动重连
}

// NewWorker creates a new Worker instance with the given configuration.
func NewWorker(config Config) *Worker {
	return &Worker{
		config:   config,
		methods:  make(map[string]reflect.Value),
		stopChan: make(chan struct{}),
		docs:     make(map[string][]string),
	}
}

// RegisterMethod registers a method handler with the worker.
func (w *Worker) RegisterMethod(name string, handler interface{}, docs ...string) error {
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
	w.docs[name] = docs
	return nil
}

// Start starts the worker with automatic reconnection support.
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
				"methods": w.getMethodsWithDocs(),
			}
			if e := w.conn.WriteJSON(registration); e != nil {
				log.Error(e)
				if closeErr := w.conn.Close(); closeErr != nil {
					log.Printf("Failed to close connection: %v", closeErr)
				}
			}
			return nil
		}

		retryCount++
		time.Sleep(time.Second * time.Duration(retryCount))
	}

	return err
}

// 增强processTasks方法
func (w *Worker) processTasks() {
	for w.running {
		if !w.ensureConnection() {
			return
		}

		msg, err := w.readMessage()
		if err != nil {
			if !w.running {
				return
			}
			w.handleReadError(err)
			continue
		}

		w.handleMessage(msg)
	}
}

func (w *Worker) ensureConnection() bool {
	w.connMutex.Lock()
	conn := w.conn
	w.connMutex.Unlock()

	if conn == nil {
		if !w.reconnect {
			return false
		}
		if err := w.connect(); err != nil {
			log.Printf("Reconnect failed: %v (retrying in 5s)", err)
			time.Sleep(5 * time.Second)
		}
	}
	return true
}

func (w *Worker) readMessage() (struct {
	Type   string          `json:"type"`
	TaskID string          `json:"taskId"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Key    string          `json:"key"`    // 加密任务的密钥
	Crypto string          `json:"crypto"` // 加密任务的加盐数据
}, error) {
	var msg struct {
		Type   string          `json:"type"`
		TaskID string          `json:"taskId"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Key    string          `json:"key"`    // 加密任务的密钥
		Crypto string          `json:"crypto"` // 加密任务的加盐数据
	}

	w.connMutex.Lock()
	conn := w.conn
	w.connMutex.Unlock()

	if conn == nil {
		return msg, fmt.Errorf("no connection")
	}

	err := conn.ReadJSON(&msg)
	return msg, err
}

func (w *Worker) handleReadError(err error) {
	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure) {
		log.Printf("Connection closed: %v", err)
	} else {
		log.Printf("Read error: %v", err)
	}

	w.closeConnection()
}

func (w *Worker) closeConnection() {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()
	if w.conn != nil {
		if closeErr := w.conn.Close(); closeErr != nil {
			log.Printf("Failed to close connection: %v", closeErr)
		}
		w.conn = nil
	}
}

func (w *Worker) handleMessage(msg struct {
	Type   string          `json:"type"`
	TaskID string          `json:"taskId"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Key    string          `json:"key"`
	Crypto string          `json:"crypto"`
}) {
	switch msg.Type {
	case "task":
		go w.handleTask(msg.TaskID, msg.Method, msg.Params)
	case "encrypted_task":
		go w.handleEncryptedTask(msg.TaskID, msg.Method, msg.Params, msg.Key, msg.Crypto)
	case "ping":
		w.handlePing()
	}
}

func (w *Worker) handlePing() {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()
	if w.conn != nil {
		pongMsg := map[string]string{"type": "pong"}
		if err := w.conn.WriteJSON(pongMsg); err != nil {
			log.Printf("Failed to send pong to scheduler: %v", err)
			if closeErr := w.conn.Close(); closeErr != nil {
				log.Printf("Failed to close connection: %v", closeErr)
			}
			w.conn = nil
		}
	}
}

// Stop stops the worker and closes all connections.
func (w *Worker) Stop() {
	if !w.running {
		return
	}

	w.running = false
	w.reconnect = false
	close(w.stopChan)

	w.connMutex.Lock()
	if w.conn != nil {
		if err := w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Printf("Failed to write close message: %v", err)
		}
		if closeErr := w.conn.Close(); closeErr != nil {
			log.Printf("Failed to close connection: %v", closeErr)
		}
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
			w.connMutex.Lock()
			if w.conn == nil {
				w.connMutex.Unlock()
				continue
			}

			if err := w.conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
				log.Printf("Ping failed: %v", err)
				// 不要直接调用Stop，而是关闭连接让processTasks处理重连
				if w.conn != nil {
					if closeErr := w.conn.Close(); closeErr != nil {
						log.Printf("Failed to close connection: %v", closeErr)
					}
					w.conn = nil
				}
			}
			w.connMutex.Unlock()
		case <-w.stopChan:
			return
		}
	}
}

// 处理加密任务
func (w *Worker) handleEncryptedTask(taskID, method string, encryptedParams json.RawMessage, key, crypto string) {
	handler, exists := w.methods[method]
	if !exists {
		w.sendResult(taskID, nil, errors.New("method not found"))
		return
	}

	var strEncryptedParams string
	if err := json.Unmarshal(encryptedParams, &strEncryptedParams); err != nil {
		w.sendResult(taskID, nil, errors.New("failed to unmarshal encrypted params"))
		return
	}

	// 解密参数
	decryptedParams, err := w.decryptData(strEncryptedParams, key, crypto)
	if err != nil {
		w.sendResult(taskID, nil, fmt.Errorf("failed to decrypt params: %v", err))
		return
	}

	// 调用注册的方法
	results := handler.Call([]reflect.Value{reflect.ValueOf(json.RawMessage(decryptedParams))})
	result := results[0].Interface()
	methodErr, _ := results[1].Interface().(error)

	if methodErr != nil {
		w.sendResult(taskID, nil, methodErr)
		return
	}

	// 加密结果
	resultBytes, err := json.Marshal(result)
	if err != nil {
		w.sendResult(taskID, nil, fmt.Errorf("failed to marshal result: %v", err))
		return
	}

	encryptedResult, err := w.encryptData(string(resultBytes), key, crypto)
	if err != nil {
		w.sendResult(taskID, nil, fmt.Errorf("failed to encrypt result: %v", err))
		return
	}

	w.sendResult(taskID, encryptedResult, nil)
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

	w.connMutex.Lock()
	defer w.connMutex.Unlock()

	if w.conn == nil {
		log.Printf("Cannot send result for task %s: connection is nil", taskID)
		return
	}

	if err := w.conn.WriteJSON(response); err != nil {
		log.Printf("Failed to send result for task %s: %v", taskID, err)
		// 连接出错时关闭连接
		if w.conn != nil {
			if closeErr := w.conn.Close(); closeErr != nil {
				log.Printf("Failed to close connection: %v", closeErr)
			}
			w.conn = nil
		}
	}
}

// 获取方法和文档信息
func (w *Worker) getMethodsWithDocs() []map[string]interface{} {
	methods := make([]map[string]interface{}, 0, len(w.methods))
	for name := range w.methods {
		methods = append(methods, map[string]interface{}{
			"name": name,
			"docs": w.docs[name],
		})
	}
	return methods
}

// AES-GCM加密数据
func (w *Worker) encryptData(data, saltedKey, crypto string) (string, error) {
	// 还原原始密钥
	originalKey, err := w.unsaltKey(saltedKey, crypto)
	if err != nil {
		return "", err
	}

	// 使用原始密钥生成AES密钥
	hash := sha256.Sum256([]byte(originalKey))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// 使用密钥生成确定性nonce（前12字节）
	nonce := hash[:gcm.NonceSize()]

	// 加密数据
	ciphertext := gcm.Seal(nil, nonce, []byte(data), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AES-GCM还原加盐密钥
func (w *Worker) unsaltKey(saltedKey, crypto string) (string, error) {
	// 解码base64
	ciphertext, err := base64.StdEncoding.DecodeString(saltedKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode salted key: %v", err)
	}

	// 使用crypto作为盐值生成AES密钥
	hash := sha256.Sum256([]byte(crypto))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// 使用盐值生成确定性nonce（前12字节）
	nonce := hash[:gcm.NonceSize()]

	// 解密密钥
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt key: %v", err)
	}

	return string(plaintext), nil
}

// AES-GCM解密数据
func (w *Worker) decryptData(encryptedData, saltedKey, crypto string) (string, error) {
	// 还原原始密钥
	originalKey, err := w.unsaltKey(saltedKey, crypto)
	if err != nil {
		return "", err
	}

	// 解码base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %v", err)
	}

	// 使用原始密钥生成AES密钥
	hash := sha256.Sum256([]byte(originalKey))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// 使用密钥生成确定性nonce（前12字节）
	nonce := hash[:gcm.NonceSize()]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %v", err)
	}

	return string(plaintext), nil
}
