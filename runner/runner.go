package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"time"

	"github.com/contester/rpc4/rpc4go"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/service"

	log "github.com/sirupsen/logrus"
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

	globalData, err := platform.CreateGlobalData(platform.GlobalDataOptions{
		NeedDesktop:     true,
		NeedLoadLibrary: true,
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	c, err := service.NewContester("server.ini", globalData)
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
