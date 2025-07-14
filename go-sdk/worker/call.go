// Package worker provides functionality for calling methods on the scheduler.
package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-enols/go-server/go-sdk/scheduler"
)

const (
	TaskStatusError = "error"
)

// Call 执行远程方法调用，支持可选的加密参数
// 如果提供了 encryptionKey，则使用加密方式调用
func Call(host, method string, params, out interface{}, encryptionKey ...string) error {
	// 检查out参数是否为指针类型
	if out != nil {
		outType := reflect.TypeOf(out)
		if outType.Kind() != reflect.Ptr {
			return fmt.Errorf("参数out必须是指针类型或nil")
		}
	}
	client := scheduler.NewSchedulerClient(host)
	
	// 如果提供了加密密钥，使用加密方式调用
	if len(encryptionKey) > 0 && encryptionKey[0] != "" {
		// 使用默认盐值或可以扩展为可配置
		res, err := client.ExecuteEncrypted(method, encryptionKey[0], 12345, params)
		if err != nil {
			return err
		}
		if res.Status == TaskStatusError {
			return errors.New(string(res.Result))
		}
		// 轮询结果
		result, err := client.GetResult(res.TaskID)
		if err != nil {
			return err
		}
		if result.Status == TaskStatusError {
			return errors.New(string(result.Result))
		}
		if out != nil {
			return json.Unmarshal(result.Result, out)
		}
		return nil
	}
	
	// 普通调用
	res, err := client.Execute(method, params)
	if err != nil {
		return err
	}
	if res.Status == TaskStatusError {
		return errors.New(string(res.Result))
	}
	// 轮询结果
	result, err := client.GetResult(res.TaskID)
	if err != nil {
		return err
	}

	if result.Status == TaskStatusError {
		return errors.New(string(result.Result))
	}
	if out != nil {
		return json.Unmarshal(result.Result, out)
	}
	return nil
}
