package tools

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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
		return nil, fmt.Errorf("Open(%q): %w", name, err)
	}
	defer source.Close()

	destination := sha1.New()

	if _, err = io.Copy(destination, source); err != nil {
		return nil, fmt.Errorf("io.Copy: %w", err)
	}

	return destination.Sum(nil), nil
}
