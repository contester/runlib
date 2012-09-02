package main

import (
	"net/rpc"
	"runlib/rpc4"
	"runlib/service"
	"runtime"
	l4g "code.google.com/p/log4go"
	"runlib/platform"
	"runlib/tools"
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
