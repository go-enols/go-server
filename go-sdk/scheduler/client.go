// Package scheduler provides client functionality for communicating with the distributed task scheduler.
package scheduler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewSchedulerClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExecuteRequest represents a task execution request.
type ExecuteRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// ResultResponse represents a task result response.
type ResultResponse struct {
	TaskID string          `json:"taskId"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
}

// Execute executes a task with the given method and parameters.
func (c *Client) Execute(method string, params interface{}) (*ResultResponse, error) {
	requestBody, err := json.Marshal(ExecuteRequest{
		Method: method,
		Params: params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/execute",
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var response ResultResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	return &response, nil
}

// GetResult retrieves the result of a task by its ID.
func (c *Client) GetResult(taskID string) (*ResultResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/result/" + taskID)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var response ResultResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	switch response.Status {
	case "pending", "processing":
		time.Sleep(1 * time.Second)
		return c.GetResult(taskID)
	case "error":
		return nil, errors.New(string(response.Result))
	}
	return &response, nil
}

// 同步执行任务（带轮询）
func (c *Client) ExecuteSync(method string, params interface{}, timeout time.Duration) (*ResultResponse, error) {
	// 提交任务
	execResp, err := c.Execute(method, params)
	if err != nil {
		return nil, fmt.Errorf("execute failed: %w", err)
	}

	// 轮询结果
	start := time.Now()
	for time.Since(start) < timeout {
		resultResp, err := c.GetResult(execResp.TaskID)
		if err != nil {
			return nil, err
		}

		switch resultResp.Status {
		case "done":
			return resultResp, nil
		case "error":
			return nil, errors.New(string(resultResp.Result))
			// "pending" 或 "processing" 状态继续等待
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, errors.New("timeout waiting for task completion")
}
