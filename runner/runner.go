package main

import (
	"fmt"
	"net/rpc"
	"os"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contester/rpc4/rpc4go"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/service"
)

func main() {
	f, err := os.OpenFile("server0.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}

	// don't forget to close it
	defer f.Close()

	log.SetOutput(f)
	log.SetLevel(log.DebugLevel)

	globalData, err := platform.CreateGlobalData()
	if err != nil {
		log.Fatal(err)
		return
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	c, err := service.NewContester("server.ini", globalData)
	if err != nil {
		log.Fatal(err)
		return
	}

	rpc.Register(c)
	for {
		if err = rpc4go.ConnectServer(c.ServerAddress, rpc.DefaultServer); err != nil {
			log.Error(err)
			time.Sleep(time.Second * 5)
		}
	}
}
