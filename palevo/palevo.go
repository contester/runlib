package main

import (
	"code.google.com/p/goprotobuf/proto"
	"log"
	"os"
	"runlib/contester_proto"
	"runtime"
	"runlib/subprocess"
)

func LogMem(s string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Println(s, &m)
}

func once() (*contester_proto.FileBlob, error) {
	name := "C:\\temp\\a\\0\\c\\fff"
	r, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	LogMem("blobstart")
	blob, err := contester_proto.BlobFromStream(r)
	LogMem("blobend")
	r.Close()
	if err != nil {
		return nil, err
	}
	return &contester_proto.FileBlob{
		Data: blob,
		Name: proto.String(name),
	}, nil
}

func main() {
	s := subprocess.SubprocessCreate()
	s.Cmd = &subprocess.CommandLine{
		ApplicationName: proto.String("C:\\mingw\\bin\\gcc.exe"),
		CommandLine: proto.String("C:\\mingw\\bin\\gcc.exe -fno-optimize-sibling-calls -fno-strict-aliasing -DONLINE_JUDGE -lm -s -Wl,--stack=268435456 -O2 -o Solution.exe Solution.c"),
	}
	s.CurrentDirectory = proto.String("C:\\Temp")
	s.StdErr = &subprocess.Redirect{
		Mode: subprocess.REDIRECT_MEMORY,
		Filename: proto.String("C:\\Temp\\footest"),
	}

	result, err := s.Execute()
	log.Println(result, err)
}
