package schedulersdk

import (
	"fmt"
	"time"
)

type RetryClient struct {
	*Client
	maxRetries int
	retryDelay time.Duration
}

func NewRetryClient(baseURL string, maxRetries int, retryDelay time.Duration) *RetryClient {
	return &RetryClient{
		Client:     NewSchedulerClient(baseURL),
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

func (c *RetryClient) ExecuteWithRetry(method string, params interface{}) (*ExecuteResponse, error) {
	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		resp, err := c.Execute(method, params)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		time.Sleep(c.retryDelay)
	}
	return nil, fmt.Errorf("after %d retries: %w", c.maxRetries, lastErr)
}
