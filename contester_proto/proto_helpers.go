package contester_proto

import (
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"compress/zlib"
	"crypto/sha1"
	"io"
)

func (blob *Blob) Reader() (io.Reader, error) {
	if blob.Compression != nil && blob.Compression.GetMethod() == Blob_CompressionInfo_METHOD_ZLIB {
		buf := bytes.NewBuffer(blob.Data)
		r, err := zlib.NewReader(buf)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	return bytes.NewBuffer(blob.Data), nil
}

func (blob *Blob) Bytes() ([]byte, error) {
	reader, err := blob.Reader()
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer
	_, err = io.Copy(&result, reader)
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func compress(data []byte) ([]byte, error) {
	var result bytes.Buffer
	writer := zlib.NewWriter(&result)
	if _, err := io.Copy(writer, bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	writer.Close()
	return result.Bytes(), nil
}

func calcSha1(data []byte) ([]byte, error) {
	result := sha1.New()
	if _, err := io.Copy(result, bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	return result.Sum(nil), nil
}

func NewBlob(data []byte) (*Blob, error) {
	sha1sum, err := calcSha1(data)
	if err != nil {
		return nil, err
	}

	compressed, err := compress(data)
	if err != nil {
		return nil, err
	}

	result := &Blob{
		Sha1: sha1sum,
	}

	if len(compressed) < len(data)-8 {
		method := Blob_CompressionInfo_METHOD_ZLIB
		result.Compression = &Blob_CompressionInfo{
			Method:       &method,
			OriginalSize: proto.Uint32(uint32(len(data))),
		}
		result.Data = compressed
	} else {
		result.Data = data
	}
	return result, nil
}
