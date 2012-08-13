package rpc4

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/rpc"
	//	"reflect"

	"code.google.com/p/goprotobuf/proto"
)

type ServerCodec struct {
	r *bufio.Reader
	w io.WriteCloser

	hasPayload bool
}

type ProtoReader interface {
	io.Reader
	io.ByteReader
}

func ReadProto(r ProtoReader, pb interface{}) error {
	var size uint32
	err := binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		return err
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	if pb != nil {
		return proto.Unmarshal(buf, pb.(proto.Message))
	}
	return nil
}

func WriteData(w io.Writer, data []byte) error {
	size := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, &size); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

func WriteProto(w io.Writer, pb interface{}) error {
	// Marshal the protobuf
	data, err := proto.Marshal(pb.(proto.Message))
	if err != nil {
		return err
	}
	return WriteData(w, data)
}

func NewServerCodec(conn net.Conn) *ServerCodec {
	return &ServerCodec{bufio.NewReader(conn), conn, false}
}

func (s *ServerCodec) ReadRequestHeader(req *rpc.Request) error {
	var header Header
	if err := ReadProto(s.r, &header); err != nil {
		return err
	}
	if header.GetMethod() == "" {
		return fmt.Errorf("header missing method: %s", header)
	}

	req.ServiceMethod = header.GetMethod()
	req.Seq = header.GetSequence()

	s.hasPayload = header.GetPayloadPresent()

	return nil
}

func (s *ServerCodec) ReadRequestBody(pb interface{}) error {
	if s.hasPayload {
		return ReadProto(s.r, pb)
	}
	return nil
}

func (s *ServerCodec) WriteResponse(resp *rpc.Response, pb interface{}) error {
	mt := Header_RESPONSE
	hasPayload := true

	if resp.Error != "" {
		mt = Header_ERROR
		// header.Error = &resp.Error
		// hasPayload = false
	}

	// Write the header
	header := Header{
		Method:         &resp.ServiceMethod,
		Sequence:       &resp.Seq,
		MessageType:    &mt,
		PayloadPresent: &hasPayload,
	}
	if err := WriteProto(s.w, &header); err != nil {
		return nil
	}

	if mt == Header_ERROR {
		return WriteData(s.w, []byte(resp.Error))
	}

	if hasPayload {
		// Write the proto
		return WriteProto(s.w, pb)
	}
	return nil
}

// Close closes the underlying conneciton.
func (s *ServerCodec) Close() error {
	return s.w.Close()
}

func ConnectRpc4(addr string, s *rpc.Server) {
	for {
		conn, err := net.Dial("tcp", addr)

		if err == nil {
			s.ServeCodec(NewServerCodec(conn))
		} else {
			fmt.Printf("%s\n", err)
		}
	}
}
