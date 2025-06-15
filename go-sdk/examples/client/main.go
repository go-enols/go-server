package main

import "github.com/go-enols/go-server/go-sdk/worker"

func main() {
	worker.Call("http://localhost:8080", "add", map[string]any{
		"a": 1,
		"b": 2,
	}, nil)
}
