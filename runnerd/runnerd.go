package main

import (
	"os/exec"
	"time"
)

func main() {
	for {
		cmd := exec.Command("runner.exe")
		cmd.Run()
		time.Sleep(time.Second * 5)
	}
}
