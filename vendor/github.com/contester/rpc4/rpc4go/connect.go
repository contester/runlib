package rpc4go

import (
	"net/rpc"
	"net"
)

func ConnectServer(addr string, s *rpc.Server) error {
	if conn, err := net.Dial("tcp", addr); err == nil {
		s.ServeCodec(NewServerCodec(conn))
	} else {
		return err
	}
	return nil
}
