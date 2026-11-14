package tools

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"lukechampine.com/blake3"
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

// Blake3File computes the 32-byte BLAKE3 digest and byte size of filename.
func Blake3File(filename string) ([32]byte, uint64, error) {
	var zero [32]byte
	source, err := os.Open(filename)
	if err != nil {
		return zero, 0, fmt.Errorf("Open(%q): %w", filename, err)
	}
	defer source.Close()

	hasher := blake3.New(32, nil)
	n, err := io.Copy(hasher, source)
	if err != nil {
		return zero, 0, fmt.Errorf("io.Copy: %w", err)
	}
	var sum [32]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, uint64(n), nil
}
