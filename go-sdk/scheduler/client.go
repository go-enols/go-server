// Package scheduler provides client functionality for interacting with the go-server scheduler.
package scheduler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
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

// encryptData encrypts data using AES-GCM with a deterministic IV derived from the key
func encryptData(data interface{}, key string) (string, error) {
	// 序列化数据
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal data failed: %w", err)
	}

	// 使用SHA-256哈希密钥
	keyHash := sha256.Sum256([]byte(key))

	// 创建AES cipher
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}

	// 创建GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM failed: %w", err)
	}

	// 使用密钥生成确定性IV（前12字节）
	ivHash := sha256.Sum256([]byte(key))
	iv := ivHash[:gcm.NonceSize()]

	// 加密数据
	ciphertext := gcm.Seal(nil, iv, dataBytes, nil)

	// 返回Base64编码的结果
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// saltKey encrypts the key using the salt as AES key
func saltKey(key string, salt int) (string, error) {
	// 使用盐值生成SHA-256哈希作为AES密钥
	saltStr := fmt.Sprintf("%d", salt)
	saltHash := sha256.Sum256([]byte(saltStr))

	// 创建AES cipher
	block, err := aes.NewCipher(saltHash[:])
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}

	// 创建GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM failed: %w", err)
	}

	// 使用盐值生成确定性IV（前12字节）
	ivHash := sha256.Sum256([]byte(saltStr))
	iv := ivHash[:gcm.NonceSize()]

	// 加密密钥
	keyBytes := []byte(key)
	ciphertext := gcm.Seal(nil, iv, keyBytes, nil)

	// 返回Base64编码的结果
	return base64.StdEncoding.EncodeToString(ciphertext), nil
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
	// 加密参数
	encryptedParams, err := encryptData(params, key)
	if err != nil {
		return nil, fmt.Errorf("encrypt params failed: %w", err)
	}

	// 对密钥进行加盐处理
	saltedKey, err := saltKey(key, salt)
	if err != nil {
		return nil, fmt.Errorf("salt key failed: %w", err)
	}

	// 构建加密请求
	requestBody, err := json.Marshal(ExecuteEncryptedRequest{
		Method: method,
		Params: encryptedParams,
		Key:    saltedKey,
		Crypto: fmt.Sprintf("%d", salt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/encrypted/execute", bytes.NewBuffer(requestBody))
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

// GetResult retrieves the result of a task by its ID.
func (c *Client) GetResultEncrypted(taskID string) (*ResultResponse, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/encrypted/result/"+taskID, http.NoBody)
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
