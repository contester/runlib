package storage


import (
	"sync"
	"github.com/contester/runlib/contester_proto"
"github.com/contester/runlib/tools"
	"fmt"
	"os"
	"io"
	"compress/zlib"
	"net/http"
"github.com/Sirupsen/logrus"
)

type weedfilerStorage struct {
	URL string
	mu sync.RWMutex
}

func (s *weedfilerStorage) upload(localName, remoteName string) error {
	ec := tools.ErrorContext("weedfiler.upload")
	local, err := os.Open(localName)
	if err != nil {
		return ec.NewError(err, "local.Open")
	}
	defer local.Close()

	pr, pw := io.Pipe()
	go func() {
		dest := zlib.NewWriter(pw)
		io.Copy(dest, local)
	}()

	req, err := http.NewRequest(s.URL + "fs/" + remoteName, "PUT", pr)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	logrus.Info(resp, err)
	return err
}

func (s *weedfilerStorage) Copy(localName, remoteName string, toRemote bool, checksum, moduleType string) (stat *contester_proto.FileStat, err error) {
	ec := tools.ErrorContext("mongodb.Copy")

	if toRemote {
		stat, err = tools.StatFile(localName, true)
		if err != nil {
			err = ec.NewError(err, "local.CalculateChecksum")
		}
		// If file doesn't exist then stat == nil.
		if err != nil || stat == nil {
			return
		}

		if checksum != "" && *stat.Checksum != checksum {
			return nil, ec.NewError(fmt.Errorf("Checksum mismatch, local %s != %s", stat.Checksum, checksum))
		}

		checksum = *stat.Checksum
	}

	if toRemote {
		err = s.upload(localName, remoteName)
	}

	return stat, nil
}