package main

import (
	"fmt"

	"github.com/go-enols/go-server/go-sdk/worker"
)

func main() {
	fmt.Println(worker.Call("http://localhost:8080", "add", map[string]any{
		"a": 1,
		"b": 2,
	}, nil, "123456"))
}
