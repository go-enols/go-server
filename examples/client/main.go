package main

import "github.com/go-enols/go-server/workersdk"

func main() {
	workersdk.Call("http://localhost:8080", "add", map[string]any{
		"a": 1,
		"b": 2,
	}, nil)
}
