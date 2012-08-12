package main

import (
  "fmt"
  "runlib/sub32"
)

func main() {

  sub := &sub32.Subprocess{}
  sub.ApplicationName = "C:\\WINDOWS\\System32\\cmd.exe"
  sub.CommandLine = "C:\\WINDOWS\\System32\\cmd.exe"

  // env := sub32.GetEnvMap()
  // env["ZZZTEST"] = "VAVA"

  // r, e := sub32.CreateProcessWithLogonW("test", nil, "test321", 0, &cmd, &cmd, 0, sub32.EnvironmentMap(env), nil, nil)
  // r, e := sub32.CreateProcessW(&cmd, &cmd, sub32.EnvironmentMap(env), nil, nil)

  // fmt.Printf("%s %d", r, e)
}