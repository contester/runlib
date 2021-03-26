package main

import "os"

func main() {
	b := make([]byte, 4096)
	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			return
		}
	}
}
