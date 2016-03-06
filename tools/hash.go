package tools

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"strings"
)

func HashFileString(name string) (string, error) {
	computedHash, err := HashFile(name)
	if err != nil {
		return "", err
	}
	return "sha1:" + strings.ToLower(hex.EncodeToString(computedHash)), nil
}

func HashFile(name string) ([]byte, error) {
	ec := ErrorContext("HashFile")

	source, err := os.Open(name)
	if err != nil {
		return nil, ec.NewError(err, "source.Open")
	}
	defer source.Close()

	destination := sha1.New()

	_, err = io.Copy(destination, source)
	if err != nil {
		return nil, ec.NewError(err, "io.Copy")
	}
	if err = source.Close(); err != nil {
		return nil, ec.NewError(err, "source.Close")
	}

	return destination.Sum(nil), nil
}
