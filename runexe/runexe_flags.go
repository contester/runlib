package main

import (
	"strings"
	"strconv"
)

type TimeLimit uint64

func (t *TimeLimit) String() string {
	return t.String()
}

func (t *TimeLimit) Set(v string) error {
	if strings.HasSuffix(v, "ms") {
		r, err := strconv.Atoi(v[:len(v) - 2])
		if err != nil {
			return err
		}
		*t = TimeLimit(r * 1000)
		return nil
	}

	r, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return err
	}
	*t = TimeLimit(r * 1000000)
	return nil
}

type MemoryLimit uint64

func (t *MemoryLimit) String() string {
	return t.String()
}

func (t *MemoryLimit) Set(v string) error {
	if strings.HasSuffix(v, "M") {
		r, err := strconv.Atoi(v[:len(v) - 1])
		if err != nil {
			return err
		}
		*t = MemoryLimit(r * 1024 * 1024)
		return nil
	}
	r, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	*t = MemoryLimit(r)
	return nil
}
