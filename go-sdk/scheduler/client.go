// Package scheduler provides client functionality for interacting with the go-server scheduler.
package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	TaskStatusError = "error"
	TaskStatusDone  = "done"
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

// ExecuteEncryptedRequest represents an encrypted task execution request.
type ExecuteEncryptedRequest struct {
	Method string `json:"method"`
	Params string `json:"params"`
	Key    string `json:"key"`
	Crypto string `json:"crypto"`
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

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/execute", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
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

// ExecuteEncrypted executes an encrypted task with the given method, key, salt and parameters.
func (c *Client) ExecuteEncrypted(method, key string, salt int, params interface{}) (*ResultResponse, error) {
	// 序列化参数
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params failed: %w", err)
	}

	// 构建加密请求
	requestBody, err := json.Marshal(ExecuteEncryptedRequest{
		Method: method,
		Params: string(paramsBytes),
		Key:    key,
		Crypto: fmt.Sprintf("%d", salt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/execute-encrypted", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
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
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/result/"+taskID, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
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
	case TaskStatusError:
		return nil, errors.New(string(response.Result))
	}
	return &response, nil
}

// ExecuteSync executes a task synchronously with polling.
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
		case TaskStatusDone:
			return resultResp, nil
		case TaskStatusError:
			return nil, errors.New(string(resultResp.Result))
			// "pending" 或 "processing" 状态继续等待
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, errors.New("timeout waiting for task completion")
}
