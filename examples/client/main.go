package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-enols/go-server/schedulersdk" // 替换为你的实际模块路径
)

func main() {
	// 1. 初始化客户端
	client := schedulersdk.NewSchedulerClient("http://localhost:8080")

	// 示例1: 异步执行
	asyncExample(client)

	// 示例2: 同步执行
	syncExample(client)

	// 示例3：带重试
	retryExample()
}

func asyncExample(client *schedulersdk.Client) {
	fmt.Println("\n--- 异步执行示例 ---")

	// 提交任务
	resp, err := client.Execute("multiply", map[string]float64{
		"a": 5.2,
		"b": 3.8,
	})
	if err != nil {
		log.Fatalf("执行失败: %v", err)
	}

	fmt.Printf("任务已提交: ID=%s, 状态=%s\n", resp.TaskID, resp.Status)

	// 轮询结果
	for {
		result, err := client.GetResult(resp.TaskID)
		if err != nil {
			log.Fatalf("获取结果失败: %v", err)
		}

		fmt.Printf("轮询结果: 状态=%s", result.Status)
		if result.Status == "done" {
			var sum struct {
				Result float64 `json:"result"`
			}
			json.Unmarshal(result.Result, &sum)
			fmt.Printf(", 结果=%.2f\n", sum.Result)
			break
		} else if result.Status == "error" {
			fmt.Printf(", 错误=%s\n", result.Error)
			break
		}
		fmt.Println()

		time.Sleep(1 * time.Second)
	}
}

func syncExample(client *schedulersdk.Client) {
	fmt.Println("\n--- 同步执行示例 ---")

	// 同步执行（内置轮询）
	result, err := client.ExecuteSync("add", map[string]float64{
		"a": 4,
		"b": 7,
	}, 10*time.Second)
	if err != nil {
		log.Fatalf("同步执行失败: %v", err)
	}

	var product struct {
		Result float64 `json:"result"`
	}
	json.Unmarshal(result.Result, &product)
	fmt.Printf("同步执行完成: 结果=%.2f\n", product.Result)
}

func retryExample() {
	client := schedulersdk.NewRetryClient("http://localhost:8080", 3, 1*time.Second)

	resp, err := client.ExecuteWithRetry("analyze", map[string]interface{}{
		"dataset": []float64{1.1, 2.2, 3.3},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("任务ID:", resp.TaskID)
}
