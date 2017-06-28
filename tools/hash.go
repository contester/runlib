package tools

import (
	"crypto/sha1"
	"encoding/hex"
	"github.com/juju/errors"
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
	source, err := os.Open(name)
	if err != nil {
		return nil, errors.Annotatef(err, "os.Open(%q)", name)
	}

	destination := sha1.New()

	if _, err = io.Copy(destination, source); err != nil {
		return nil, errors.Annotate(err, "io.Copy")
	}

	return destination.Sum(nil), nil
}
