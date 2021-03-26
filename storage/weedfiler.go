package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"github.com/juju/errors"
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

func filerUpload(ctx context.Context, localName, remoteName, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	if stat, err = tools.StatFile(localName, true); err != nil || stat == nil {
		return stat, err
	}
	if checksum != "" && stat.GetChecksum() != checksum {
		return nil, errors.BadRequestf("Checksum mismatch, local %q != %q", stat.GetChecksum(), checksum)
	}
	checksum = stat.GetChecksum()

	local, err := os.Open(localName)
	if err != nil {
		return nil, errors.Annotatef(err, "os.Open(%q)", localName)
	}
	defer local.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, remoteName, local)
	if err != nil {
		return nil, errors.Annotatef(err, "http.NewRequest('PUT', %q, %q", remoteName, local)
	}
	if moduleType != "" {
		req.Header.Add("X-FS-Module-Type", moduleType)
	}
	if authToken != "" {
		req.Header.Add("Authorization", "bearer "+authToken)
	}
	req.Header.Add("X-FS-Content-Length", strconv.FormatUint(stat.GetSize_(), 10))
	var base64sha1 string
	if checksum != "" && strings.HasPrefix(checksum, "sha1:") {
		if data, err := hex.DecodeString(strings.TrimPrefix(checksum, "sha1:")); err == nil {
			base64sha1 = base64.StdEncoding.EncodeToString(data)
			req.Header.Add("Digest", "SHA="+base64sha1)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "http.Do")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.Errorf("invalid status: %d %q", resp.StatusCode, resp.Status)
	}
	var st uploadStatus
	err = json.NewDecoder(resp.Body).Decode(&st)
	if err != nil {
		return nil, errors.Annotate(err, "json.Decode")
	}
	if st.Size != int64(stat.GetSize_()) || (base64sha1 != "" && base64sha1 != st.Digests["SHA"]) {
		return nil, errors.NotValidf("upload integrity verification failed")
	}
	return stat, nil
}

// remoteName must be full URL.
func filerDownload(ctx context.Context, localName, remoteName, authToken string) (stat *contester_proto.FileStat, err error) {
	local, err := os.Create(localName)
	if err != nil {
		return nil, err
	}
	defer local.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteName, nil)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Add("Authorization", "bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return nil, errors.NotFoundf("not found: %q", remoteName)
		}
		return nil, errors.Errorf("invalid status: %d %q", resp.StatusCode, resp.Status)
	}
	if _, err = io.Copy(local, resp.Body); err != nil {
		return nil, err
	}
	local.Close()
	return tools.StatFile(localName, true)
}

func FilerReadRemote(ctx context.Context, name, authToken string) (*RemoteFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, name, nil)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Add("Authorization", "bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return nil, errors.NotFoundf("not found: %q", name)
		}
		return nil, errors.Errorf("invalid status: %d %q", resp.StatusCode, resp.Status)
	}
	result := RemoteFile{
		Body: resp.Body,
	}

	if sz := resp.Header.Get("Content-Length"); sz != "" {
		if isz, err := strconv.ParseUint(sz, 10, 64); err == nil {
			result.Stat.Size_ = isz
		}
	}

	return &result, nil
}

func FilerCopy(ctx context.Context, localName, remoteName string, toRemote bool, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	if toRemote {
		return filerUpload(ctx, localName, remoteName, checksum, moduleType, authToken)
	}
	return filerDownload(ctx, localName, remoteName, authToken)
}

func isFilerRemote(src string) string {
	if strings.HasPrefix(src, "filer:") {
		return strings.TrimPrefix(src, "filer:")
	}
	return ""
}

func (s *weedfilerStorage) Copy(ctx context.Context, localName, remoteName string, toRemote bool, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	if fr := isFilerRemote(remoteName); fr != "" {
		return FilerCopy(ctx, localName, fr, toRemote, checksum, moduleType, authToken)
	}
	remoteName = s.URL + "fs/" + remoteName
	return FilerCopy(ctx, localName, remoteName, toRemote, checksum, moduleType, authToken)
}

func (s *weedfilerStorage) ReadRemote(ctx context.Context, name, authToken string) (*RemoteFile, error) {
	return FilerReadRemote(ctx, name, authToken)
}

func (s *weedfilerStorage) GetAllManifests(ctx context.Context) ([]ProblemManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+"problem/get/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result []ProblemManifest
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func (s *weedfilerStorage) GetNextRevision(ctx context.Context, id string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+"problem/get/?id="+url.QueryEscape(id), nil)
	if err != nil {
		return 1, err
	}
	resp, err := http.DefaultClient.Do(req)
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

func (s *weedfilerStorage) SetManifest(ctx context.Context, manifest *ProblemManifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL+"problem/set/", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func (s *weedfilerStorage) String() string {
	return s.URL
}
