package service

import (
	"code.google.com/p/goprotobuf/proto"
	"os"
	"path/filepath"
	"runlib/contester_proto"
)

func statFile(name string) *contester_proto.StatResponse_StatFile {
	f := &contester_proto.StatResponse_StatFile{}
	f.Name = &name
	info, err := os.Stat(name)
	if err == nil {
		f.Exists = proto.Bool(true)
		if info.IsDir() {
			f.IsDirectory = proto.Bool(true)
		}
	}
	return f
}

func (s *Contester) Stat(request *contester_proto.StatRequest, response *contester_proto.StatResponse) error {
	response.Stats = make([]*contester_proto.StatResponse_StatFile, len(request.Name))
	for index, name := range request.Name {
		response.Stats[index] = statFile(name)
	}
	return nil
}

func (s *Contester) Glob(request *contester_proto.GlobRequest, response *contester_proto.GlobResponse) (err error) {
	response.Results = make([]*contester_proto.GlobResponse_SingleGlob, len(request.Expression))
	for index, expr := range request.Expression {
		f := &contester_proto.GlobResponse_SingleGlob{Expression: proto.String(expr)}
		var e1 error
		f.Results, e1 = filepath.Glob(expr)
		if e1 != nil && err == nil {
			err = e1
		}
		response.Results[index] = f
	}

	return
}
