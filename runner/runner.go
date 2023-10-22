package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/contester/rpc4/rpc4go"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/service"

	log "github.com/sirupsen/logrus"
)

var CLI struct {
	Logfile    string `short:"l" long:"logfile" description:"Log file" env:"RUNNER_LOGFILE" default:"server.log"`
	ConfigFile string `short:"c" long:"config" description:"Config file" env:"RUNNER_CONFIG" default:"server.toml" type:"existingfile"`
}

func main() {
	kong.Parse(&CLI)

	f, err := os.OpenFile(CLI.Logfile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	log.SetLevel(log.DebugLevel)

	globalData, err := platform.CreateGlobalData(true)
	if err != nil {
		log.Fatal(err)
		return
	}

	c, err := service.NewContester(CLI.ConfigFile, globalData)
	if err != nil {
		log.Fatal(err)
		return
	}

	rpc.Register(c)

	d := net.Dialer{
		Timeout:   time.Minute,
		DualStack: true,
		KeepAlive: time.Second * 10,
	}

	for {
		conn, err := d.Dial("tcp", c.ServerAddress)
		if err != nil {
			log.Error(err)
			time.Sleep(time.Second * 5)
			continue
		}
		rpc.DefaultServer.ServeCodec(rpc4go.NewServerCodec(conn))
		conn.Close()
	}
}
