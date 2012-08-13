package rpc4

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/rpc"

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
	return proto.Unmarshal(buf, pb.(proto.Message))
}

func WriteProto(w io.Writer, pb interface{}) error {
	// Allocate enough space for the biggest uvarint
	var size uint32

	// Marshal the protobuf
	data, err := proto.Marshal(pb.(proto.Message))
	if err != nil {
		return err
	}

	// Write the size and data
	size = uint32(len(data))
	if err = binary.Write(w, binary.BigEndian, &size); err != nil {
		return err
	}
	if _, err = w.Write(data); err != nil {
		return err
	}
	return nil
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
	if resp.Error != "" {
		mt = Header_ERROR
		// header.Error = &resp.Error
	}

	// Write the header
	header := Header{
		Method:      &resp.ServiceMethod,
		Sequence:    &resp.Seq,
		MessageType: &mt,
	}
	if err := WriteProto(s.w, &header); err != nil {
		return nil
	}

	// Write the proto
	return WriteProto(s.w, pb)
}

// Close closes the underlying conneciton.
func (s *ServerCodec) Close() error {
	return s.w.Close()
}
