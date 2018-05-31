package rpc4go

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/rpc"

	"github.com/golang/protobuf/proto"
	rpc4 "github.com/contester/rpc4/proto"
)

type codec struct {
	r *bufio.Reader
	w io.WriteCloser

	responsePayload, requestPayload bool
	Shutdown bool
}

type ProtoReader interface {
	io.Reader
	io.ByteReader
}

func readProto(r ProtoReader, pb proto.Message) (err error) {
	var size uint32
	if err = binary.Read(r, binary.BigEndian, &size); err != nil {
		return
	}
	buf := make([]byte, size)
	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}
	if pb != nil {
		err = proto.Unmarshal(buf, pb)
	}
	return
}

// Unbuffered
func writeData(w io.Writer, data []byte) (err error) {
	if err = binary.Write(w, binary.BigEndian, proto.Uint32(uint32(len(data)))); err != nil {
		return
	}
	_, err = w.Write(data)
	return
}

func writeProto(w io.Writer, pb proto.Message) error {
	data, err := proto.Marshal(pb)
	if err != nil {
		return err
	}
	return writeData(w, data)
}

func NewServerCodec(conn net.Conn) *codec {
	return &codec{r: bufio.NewReader(conn), w: conn}
}

func (s *codec) ReadRequestHeader(req *rpc.Request) error {
	if s.Shutdown {
		return rpc.ErrShutdown
	}

	var header rpc4.Header
	if err := readProto(s.r, &header); err != nil {
		return err
	}
	if header.GetMethod() == "" {
		return fmt.Errorf("header missing method: %s", header)
	}

	req.ServiceMethod = header.GetMethod()
	req.Seq = header.GetSequence()

	s.requestPayload = header.GetPayloadPresent()

	return nil
}

func (s *codec) ReadRequestBody(pb interface{}) error {
	if s.requestPayload {
		return readProto(s.r, pb.(proto.Message))
	}
	return nil
}

func (s *codec) writeHeaderData(header *rpc4.Header, data []byte) (err error) {
	if len(data) > 0 {
		header.PayloadPresent = proto.Bool(true)
	}

	if err = writeProto(s.w, header); err == nil && header.GetPayloadPresent() {
		err = writeData(s.w, data)
	}
	return
}

func (s *codec) WriteResponse(resp *rpc.Response, pb interface{}) error {
	var header rpc4.Header
	header.Method, header.Sequence, header.MessageType = &resp.ServiceMethod, &resp.Seq, rpc4.Header_RESPONSE.Enum()

	var data []byte

	if resp.Error == "" {
		var err error
		if data, err = proto.Marshal(pb.(proto.Message)); err != nil {
			resp.Error = err.Error()
		}
	}

	if resp.Error != "" {
		header.MessageType = rpc4.Header_ERROR.Enum()
		data = []byte(resp.Error)
	}

	return s.writeHeaderData(&header, data)
}

func (s *codec) WriteRequest(req *rpc.Request, pb interface{}) error {
	var header rpc4.Header
	header.Method, header.Sequence, header.MessageType = &req.ServiceMethod, &req.Seq, rpc4.Header_REQUEST.Enum()
	var data []byte
	if pb != nil {
		var err error
		if data, err = proto.Marshal(pb.(proto.Message)); err != nil {
			return err
		}
	}
	return s.writeHeaderData(&header, data)
}

func (s *codec) ReadResponseHeader(resp *rpc.Response) error {
	if s.Shutdown {
		return rpc.ErrShutdown
	}

	var header rpc4.Header
	if err := readProto(s.r, &header); err != nil {
		return err
	}
	if header.GetMethod() == "" {
		return fmt.Errorf("header missing method: %s", header)
	}

	resp.ServiceMethod = header.GetMethod()
	resp.Seq = header.GetSequence()

	s.responsePayload = header.GetPayloadPresent()
	return nil
}

func (s *codec) ReadResponseBody(pb interface{}) error {
	if s.responsePayload {
		return readProto(s.r, pb.(proto.Message))
	}
	return nil
}

// Close closes the underlying conneciton.
func (s *codec) Close() error {
	return s.w.Close()
}
