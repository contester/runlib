package main

import (
	//  "fmt"
	//  "runlib/sub32"
	"net/rpc"
	"runlib/rpc4"
	"runlib/service"
)

//func sptr(s string) *string {
//  return &s
//}

func main() {

	c := service.NewContester("server.ini")

	rpc.Register(c)
	rpc4.ConnectRpc4("localhost:9981", rpc.DefaultServer)

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
