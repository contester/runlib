package main

import (
	//  "fmt"
	//  "runlib/sub32"
	"log"
	"net/rpc"
	"runlib/contester_proto"
	"runlib/rpc4"
	"runlib/service"
	"runtime"
	"time"
)

//func sptr(s string) *string {
//  return &s
//}

func LogMem() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("Alloc: %d, Sys: %d, HeapAlloc: %d, HeapIdle: %d, NextGC: %d, LastGC: %s, NumGC: %d, Blobs: %d\n",
		m.Alloc, m.Sys, m.HeapAlloc, m.HeapIdle, m.NextGC, time.Now().Sub(time.Unix(0, int64(m.LastGC))), m.NumGC, contester_proto.BlobCount())
}

func LogMemLoop() {
	for {
		LogMem()
		runtime.GC()
		runtime.Gosched()
		runtime.GC()
		time.Sleep(time.Second * 15)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	go LogMemLoop()

	c := service.NewContester("server.ini")

	rpc.Register(c)
	rpc4.ConnectRpc4(c.ServerAddress, rpc.DefaultServer)

	/*
	  sub := sub32.SubprocessCreate()
	  cmd := "C:\\WINDOWS\\System32\\cmd.exe"
	  sub.ApplicationName = sptr(cmd)
	  sub.CommandLine = sptr(cmd + " /c echo test")
	  sub.Username = sptr("test")
	  sub.Password = sptr("test321")
	  sub.StdOut = &sub32.SubprocessOutputRedirect{}
	  sub.StdOut.ToMemory = true

	  sig, err := sub.Start()



	  // env := sub32.GetEnvMap()
	  // env["ZZZTEST"] = "VAVA"

	  // r, e := sub32.CreateProcessWithLogonW("test", nil, "test321", 0, &cmd, &cmd, 0, sub32.EnvironmentMap(env), nil, nil)
	  // r, e := sub32.CreateProcessW(&cmd, &cmd, sub32.EnvironmentMap(env), nil, nil)

	  fmt.Printf("%s %s\n", sig, err)

	  fmt.Printf("%s\n", <-sig)
	*/
}
