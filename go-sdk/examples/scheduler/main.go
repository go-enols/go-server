package main

import goserver "github.com/go-enols/go-server"

func main() {
	scheduler := goserver.NewScheduler()
	scheduler.Start(":8080", "123")
}
