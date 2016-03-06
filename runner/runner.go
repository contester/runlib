package main

import (
	"net/rpc"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contester/rpc4/rpc4go"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/service"
)

func main() {
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
