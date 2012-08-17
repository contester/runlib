package rpc4

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/rpc"
//	"os"
//"log"

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

// Unbuffered
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
	return &ServerCodec{r: bufio.NewReader(conn), w: conn, hasPayload:false}
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
	var buf bytes.Buffer
	err := WriteResponseUnbuffered(&buf, resp, pb)
	if err != nil {
		return err
	}
	_, err = s.w.Write(buf.Bytes())
	return err
}

func WriteResponseUnbuffered(w io.Writer, resp *rpc.Response, pb interface{}) error {
	mt := Header_RESPONSE
	hasPayload := false
	var data []byte
	var err error

	if resp.Error != "" {
		mt = Header_ERROR
		data = []byte(resp.Error)
	} else {
		data, err = proto.Marshal(pb.(proto.Message))
		if err != nil {
			mt = Header_ERROR
			data = []byte(err.Error())
		}
	}

	if data != nil && len(data) > 0 {
		hasPayload = true
	}

	// Write the header
	header := Header{
		Method:         &resp.ServiceMethod,
		Sequence:       &resp.Seq,
		MessageType:    &mt,
		PayloadPresent: &hasPayload,
	}
	if err = WriteProto(w, &header); err != nil {
		return err
	}

	if hasPayload {
		if err = WriteData(w, data); err != nil {
			return err
		}
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
