package main

import (
	l4g "code.google.com/p/log4go"
	"net/rpc"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/rpc4"
	"github.com/contester/runlib/service"
	"github.com/contester/runlib/tools"
	"runtime"
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
	rpc4.ConnectRpc4(c.ServerAddress, rpc.DefaultServer)
}
