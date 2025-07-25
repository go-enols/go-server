package main

import (
	"encoding/json"
	"log"

	"github.com/go-enols/go-server/go-sdk/worker" // 假设SDK被导入为这个包名
)

func main() {
	// 1. 创建Worker配置
	config := worker.Config{
		SchedulerURL: "ws://localhost:8080/api/worker/connect/123456",
		WorkerGroup:  "math",
		MaxRetry:     5,
		PingInterval: 5,
	}

	// 2. 创建Worker实例
	worker := worker.NewWorker(config)

	// 3. 注册业务方法
	if err := worker.RegisterMethod("add", addNumbers,
		"计算两个数字的和",
		"参数: {\"a\": number, \"b\": number}",
		"返回: number"); err != nil {
		log.Fatal("Failed to register add method:", err)
	}

	if err := worker.RegisterMethod("multiply", multiplyNumbers,
		"计算两个数字的乘积",
		"参数: {\"a\": number, \"b\": number}",
		"返回: number"); err != nil {
		log.Fatal("Failed to register multiply method:", err)
	}

	// 4. 启动Worker
	if err := worker.Start(); err != nil {
		log.Fatal("Worker failed to start:", err)
	}

	// 保持运行
	select {}
}

// 加法方法
func addNumbers(params json.RawMessage) (interface{}, error) {
	var input struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	return input.A + input.B, nil
}

// 乘法方法
func multiplyNumbers(params json.RawMessage) (interface{}, error) {
	var input struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	return input.A * input.B, nil
}
