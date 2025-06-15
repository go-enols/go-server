package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-enols/go-server/go-sdk/scheduler"
)

func Call(host, method string, params interface{}, out interface{}) error {
	// 检查out参数是否为指针类型
	if out != nil {
		outType := reflect.TypeOf(out)
		if outType.Kind() != reflect.Ptr {
			return fmt.Errorf("参数out必须是指针类型或nil")
		}
	}
	client := scheduler.NewSchedulerClient(host)
	res, err := client.Execute(method, params)
	if err != nil {
		return err
	}
	if res.Status == "error" {
		return errors.New(string(res.Result))
	}
	// 轮询结果
	result, err := client.GetResult(res.TaskID)
	if err != nil {
		return err
	}

	if result.Status == "error" {
		return errors.New(string(result.Result))
	}
	if out != nil {
		return json.Unmarshal(result.Result, out)
	}
	return nil
}
