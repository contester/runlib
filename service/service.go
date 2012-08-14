package service

import (
	"bytes"
	"code.google.com/p/goprotobuf/proto"
	"compress/zlib"
	"io"
	"os"
	"path/filepath"
	"runlib/contester_proto"
	"strings"
	"io/ioutil"
	"path"
)

type Contester struct {
	InvokerId     string
	Sandboxes     []SandboxPair
	Env           []*contester_proto.LocalEnvironment_Variable
	ServerAddress string
}

func getHostname() string {
	result, err := os.Hostname()
	if err != nil {
		return "undefined"
	}
	return result
}

func getLocalEnvironment() []*contester_proto.LocalEnvironment_Variable {
	list := os.Environ()
	result := make([]*contester_proto.LocalEnvironment_Variable, len(list))
	for i, v := range list {
		s := strings.SplitN(v, "=", 2)
		result[i] = &contester_proto.LocalEnvironment_Variable{
			Name:  proto.String(s[0]),
			Value: proto.String(s[1])}
	}
	return result
}

func NewContester(configFile string) *Contester {
	conf, err := readConfigFile(configFile)
	if err != nil {
		return nil
	}

	sandboxes, err := configureSandboxes(conf)
	if err != nil {
		return nil
	}

	result := &Contester{
		InvokerId:     getHostname(),
		Sandboxes:     sandboxes,
		Env:           getLocalEnvironment(),
		ServerAddress: conf.Server,
	}

	return result
}

func (s *Contester) Identify(request *contester_proto.IdentifyRequest, response *contester_proto.IdentifyResponse) error {
	response.InvokerId = &s.InvokerId
	response.Environment = &contester_proto.LocalEnvironment{
		Variable: s.Env[:]}
	response.Sandboxes = make([]*contester_proto.SandboxLocations, len(s.Sandboxes))
	for i, p := range s.Sandboxes {
		response.Sandboxes[i] = &contester_proto.SandboxLocations{
			Compile: proto.String(p.Compile.Path),
			Run:     proto.String(p.Run.Path)}
	}

	return nil
}

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

func (s *Contester) GetLocalEnvironment(request *contester_proto.EmptyMessage, response *contester_proto.LocalEnvironment) error {
	response.Variable = s.Env[:]
	return nil
}

func BlobReader(blob *contester_proto.Blob) (io.Reader, error) {
	if blob.Compression != nil && blob.Compression.GetMethod() == contester_proto.Blob_CompressionInfo_METHOD_ZLIB {
		buf := bytes.NewBuffer(blob.Data)
		r, err := zlib.NewReader(buf)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	return bytes.NewBuffer(blob.Data), nil
}

func Blob(data []byte) *contester_proto.Blob {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	io.Copy(w, bytes.NewBuffer(data))
	w.Close()
	
	z := contester_proto.Blob_CompressionInfo_METHOD_ZLIB
	result := &contester_proto.Blob{
		Compression: &contester_proto.Blob_CompressionInfo{
			Method: &z,
			OriginalSize: proto.Uint32(uint32(len(data))),
		},
		Data: b.Bytes(),
	}
	return result
}

func (s *Contester) Put(request *contester_proto.PutRequest, response *contester_proto.PutResponse) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetSandbox())
	if err != nil {
		return err
	}

	for _, module := range request.Module {
		destPath := filepath.Join(sandbox.Path, module.GetName())
		f, err := os.Create(destPath)
		if err != nil {
			return err
		}
		r, err := BlobReader(module.Data)
		if err != nil {
			return err
		}
		io.Copy(f, r)
		f.Close()
	}
	return nil
}

func (s *Contester) Clear(request *contester_proto.ClearRequest, response *contester_proto.EmptyMessage) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetSandbox())
	if err != nil {
		return err
	}

	path := sandbox.Path
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, info := range files {
		if info.Name() == "." || info.Name() == ".." {
			continue
		}
		fullpath := filepath.Join(path, info.Name())
		err = os.RemoveAll(fullpath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Contester) Get(request *contester_proto.GetRequest, response *contester_proto.GetResponse) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetPrefix())

	if err != nil {
		return err
	}

	response.Module = make([]*contester_proto.Module, 0, len(request.Filename))

	basepath := sandbox.Path
	for _, name := range request.Filename {
		fullpath := filepath.Join(basepath, name)
		data, err := ioutil.ReadFile(fullpath)
		if err != nil {
			continue
		}
		blob := Blob(data)
		module := &contester_proto.Module{
			Data: blob,
			Name: proto.String(name),
			Type: proto.String(path.Ext(name)[1:]),
		}
		response.Module = response.Module[:len(response.Module) + 1]
		response.Module[len(response.Module) - 1] = module
	}
	return nil
}