package contester_proto

//go:generate protoc --go_out=. --go_opt=paths=source_relative Blobs.proto Contester.proto Execution.proto Local.proto

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
)

func (blob *Blob) Reader() (io.Reader, error) {
	if blob.Compression != nil && blob.GetCompression().GetMethod() == Blob_CompressionInfo_METHOD_ZLIB {
		buf := bytes.NewReader(blob.Data)
		r, err := zlib.NewReader(buf)
		if err != nil {
			return nil, fmt.Errorf("zlib.NewReader(): %w", err)
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
		return nil, fmt.Errorf("io.Copy: %w", err)
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
	if data == nil {
		return nil, nil
	}
	sha1sum, err := calcSha1(data)
	if err != nil {
		return nil, err
	}

	compressed, err := compress(data)
	if err != nil {
		return nil, err
	}

	result := Blob{
		Sha1: sha1sum,
	}
	if len(compressed) < len(data)-8 {
		result.Compression = &Blob_CompressionInfo{
			Method:       Blob_CompressionInfo_METHOD_ZLIB,
			OriginalSize: uint32(len(data)),
		}
		result.Data = compressed
	} else {
		result.Data = data
	}
	return &result, nil
}

func BlobFromStream(r io.Reader) (*Blob, error) {
	var compressed bytes.Buffer
	compressor := zlib.NewWriter(&compressed)
	shaCalculator := sha1.New()
	writer := io.MultiWriter(compressor, shaCalculator)

	size, err := io.Copy(writer, r)
	if err != nil {
		return nil, fmt.Errorf("io.Copy: %w", err)
	}
	compressor.Close()
	return &Blob{
		Sha1: shaCalculator.Sum(nil),
		Data: compressed.Bytes(),
		Compression: &Blob_CompressionInfo{
			Method:       Blob_CompressionInfo_METHOD_ZLIB,
			OriginalSize: uint32(size),
		},
	}, nil
}
