package workersdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-enols/go-server/schedulersdk"
)

func Call(host, method string, params interface{}, out interface{}) error {
	// 检查out参数是否为指针类型
	if out != nil {
		outType := reflect.TypeOf(out)
		if outType.Kind() != reflect.Ptr {
			return fmt.Errorf("参数out必须是指针类型或nil")
		}
	}
	client := schedulersdk.NewSchedulerClient(host)
	res, err := client.Execute(method, params)
	if err != nil {
		return err
	}
	if res.Status == "error" {
		return errors.New(string(res.Result))
	}
	if out != nil {
		return json.Unmarshal(res.Result, out)
	}
	return nil
}
