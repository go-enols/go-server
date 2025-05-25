package schedulersdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// 任务执行请求
type ExecuteRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// 任务执行响应
type ExecuteResponse struct {
	TaskID string `json:"taskId"`
	Status string `json:"status"`
}

// 任务结果响应
type ResultResponse struct {
	TaskID string          `json:"taskId"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

// 执行任务
func (c *Client) Execute(method string, params interface{}) (*ExecuteResponse, error) {
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var response ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	return &response, nil
}

// 获取任务结果
func (c *Client) GetResult(taskID string) (*ResultResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/result/" + taskID)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

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
			return nil, fmt.Errorf("get result failed: %w", err)
		}

		switch resultResp.Status {
		case "done":
			return resultResp, nil
		case "error":
			return nil, errors.New(resultResp.Error)
			// "pending" 或 "processing" 状态继续等待
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, errors.New("timeout waiting for task completion")
}
