package main

import (
	l4g "code.google.com/p/log4go"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/service"
	"github.com/contester/runlib/tools"
	"github.com/contester/rpc4/rpc4go"
	"net/rpc"
	"runtime"
	"time"
)

func main() {
	tools.SetupLog("server.log")

	globalData, err := platform.CreateGlobalData()
	if err != nil {
		l4g.Error(err)
		return
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	go tools.LogMemLoop()

	c, err := service.NewContester("server.ini", globalData)
	if err != nil {
		l4g.Error(err)
		return
	}

	rpc.Register(c)
	for {
		if err = rpc4go.ConnectServer(c.ServerAddress, rpc.DefaultServer); err != nil {
			l4g.Error(err)
			time.Sleep(time.Second * 5)
		}
	}
}
