package main

import (
	"strings"
	"strconv"
)

type TimeLimitFlag uint64

func (t *TimeLimitFlag) String() string {
	return strconv.Itoa(int(*t / 1000)) + "ms"
}

func (t *TimeLimitFlag) Set(v string) error {
	v = strings.ToLower(v)
	if strings.HasSuffix(v, "ms") {
		r, err := strconv.Atoi(v[:len(v) - 2])
		if err != nil {
			return err
		}
		*t = TimeLimitFlag(r * 1000)
		return nil
	}
	if strings.HasSuffix(v, "s") {
		v = v[:len(v) - 1]
	}
	r, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return err
	}
	*t = TimeLimitFlag(r * 1000000)
	return nil
}

type MemoryLimitFlag uint64

func (t *MemoryLimitFlag) String() string {
	return strconv.Itoa(int(*t))
}

func (t *MemoryLimitFlag) Set(v string) error {
	v = strings.ToUpper(v)
	m := 1
	switch v[len(v)-1] {
		case 'M': m = 1024*1024
		case 'K': m = 1024
		case 'G': m = 1024*1024*1024
	}
	if m != 1 {
		v = v[:len(v) - 1]
	}
	r, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	*t = MemoryLimitFlag(r * m)
	return nil
}

type EnvFlag []string

func (t *EnvFlag) String() string {
	return strings.Join(*t, "|")
}

func (t *EnvFlag) Set(v string) error {
	*t = append(*t, v)
	return nil
}
