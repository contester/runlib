package rpc4

import (
	"bufio"
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

	decodeBuf, headerBuf, dataBuf *proto.Buffer
}

type ProtoReader interface {
	io.Reader
	io.ByteReader
}

func ReadProto(r ProtoReader, pb interface{}, dbuf *proto.Buffer) error {
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
		dbuf.SetBuf(buf)
		err := dbuf.Unmarshal(pb.(proto.Message))
		dbuf.Reset()
		return err
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

func WriteProto(w io.Writer, pb interface{}, ebuf *proto.Buffer) error {
	// Marshal the protobuf
	ebuf.Reset()
	err := ebuf.Marshal(pb.(proto.Message))
	if err != nil {
		return err
	}
	err = WriteData(w, ebuf.Bytes())
	ebuf.Reset()
	return err
}

func NewServerCodec(conn net.Conn) *ServerCodec {
	return &ServerCodec{r: bufio.NewReader(conn), w: conn, hasPayload: false, headerBuf: proto.NewBuffer(nil), dataBuf: proto.NewBuffer(nil), decodeBuf: proto.NewBuffer(nil)}
}

func (s *ServerCodec) ReadRequestHeader(req *rpc.Request) error {
	var header Header
	if err := ReadProto(s.r, &header, s.decodeBuf); err != nil {
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
		return ReadProto(s.r, pb, s.decodeBuf)
	}
	return nil
}

func (s *ServerCodec) WriteResponse(resp *rpc.Response, pb interface{}) error {
	mt := Header_RESPONSE
	hasPayload := false
	var data []byte
	var err error

	if resp.Error != "" {
		mt = Header_ERROR
		data = []byte(resp.Error)
	} else {
		s.dataBuf.Reset()
		err = s.dataBuf.Marshal(pb.(proto.Message))
		pb.(proto.Message).Reset()
		if err != nil {
			mt = Header_ERROR
			data = []byte(err.Error())
		} else {
			data = s.dataBuf.Bytes()
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
	if err = WriteProto(s.w, &header, s.headerBuf); err != nil {
		return err
	}

	if hasPayload {
		if err = WriteData(s.w, data); err != nil {
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
