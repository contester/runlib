package storage

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	//log "github.com/Sirupsen/logrus"
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
)

var _ ProblemStore = &weedfilerStorage{}

type weedfilerStorage struct {
	URL string
	mu  sync.RWMutex
}

func NewWeed(url string) *weedfilerStorage {
	return &weedfilerStorage{
		URL: url,
	}
}

type uploadStatus struct {
	Size    int64
	Digests map[string]string
}

func filerUpload(localName, remoteName, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	ec := tools.ErrorContext("upload")
	if stat, err = tools.StatFile(localName, true); err != nil || stat == nil {
		return stat, err
	}
	if checksum != "" && stat.GetChecksum() != checksum {
		return nil, fmt.Errorf("Checksum mismatch, local %s != %s", stat.GetChecksum(), checksum)
	}
	checksum = stat.GetChecksum()

	local, err := os.Open(localName)
	if err != nil {
		return nil, ec.NewError(err, "local.Open")
	}
	defer local.Close()

	req, err := http.NewRequest("PUT", remoteName, local)
	if err != nil {
		return nil, err
	}
	if moduleType != "" {
		req.Header.Add("X-FS-Module-Type", moduleType)
	}
	if authToken != "" {
		req.Header.Add("Authorization", "bearer " + authToken)
	}
	req.Header.Add("X-FS-Content-Length", strconv.FormatUint(stat.GetSize(), 10))
	var base64sha1 string
	if checksum != "" && strings.HasPrefix(checksum, "sha1:") {
		if data, err := hex.DecodeString(strings.TrimPrefix(checksum, "sha1:")); err == nil {
			base64sha1 = base64.StdEncoding.EncodeToString(data)
			req.Header.Add("Digest", "SHA="+base64sha1)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var st uploadStatus
	err = json.NewDecoder(resp.Body).Decode(&st)
	if err != nil {
		return nil, err
	}
	if st.Size != int64(stat.GetSize()) || (base64sha1 != "" && base64sha1 != st.Digests["SHA"]) {
		return nil, fmt.Errorf("upload integrity verification failed")
	}
	return stat, nil
}

// remoteName must be full URL.
func filerDownload(localName, remoteName, authToken string) (stat *contester_proto.FileStat, err error) {
	local, err := os.Create(localName)
	if err != nil {
		return nil, err
	}
	defer local.Close()
	req, err := http.NewRequest("GET", remoteName, nil)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Add("Authorization", "bearer " + authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if _, err = io.Copy(local, resp.Body); err != nil {
		return nil, err
	}
	local.Close()
	return tools.StatFile(localName, true)
}

func filerCopy(localName, remoteName string, toRemote bool, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	if toRemote {
		return filerUpload(localName, remoteName, checksum, moduleType, authToken)
	}
	return filerDownload(localName, remoteName, authToken)
}

func isFilerRemote(src string) string {
	if strings.HasPrefix(src, "filer:") {
		return strings.TrimPrefix(src, "filer:")
	}
	return ""
}

func (s *weedfilerStorage) Copy(localName, remoteName string, toRemote bool, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	if fr := isFilerRemote(remoteName); fr != "" {
		return filerCopy(localName, fr, toRemote, checksum, moduleType, authToken)
	}
	remoteName = s.URL + "fs/" + remoteName
	return filerCopy(localName, remoteName, toRemote, checksum, moduleType, authToken)
}

func (s *weedfilerStorage) Cleanup(latest int) error {
	return nil
}

func (s *weedfilerStorage) Close() {
}

func (s *weedfilerStorage) GetAllManifests() ([]ProblemManifest, error) {
	resp, err := http.Get(s.URL + "problem/get/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result []ProblemManifest
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func (s *weedfilerStorage) GetNextRevision(id string) (int, error) {
	resp, err := http.Get(s.URL + "problem/get/?id=" + url.QueryEscape(id))
	if err != nil {
		return 1, err
	}
	defer resp.Body.Close()
	var result []ProblemManifest
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 1, err
	}
	if len(result) == 0 {
		return 1, nil
	}
	return result[0].Revision + 1, nil
}

func (s *weedfilerStorage) SetManifest(manifest *ProblemManifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	resp, err := http.Post(s.URL+"problem/set/", "application/octet-stream", bytes.NewReader(data))
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func (s *weedfilerStorage) String() string {
	return s.URL
}
