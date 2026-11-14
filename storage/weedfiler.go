package storage

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"lukechampine.com/blake3"
)

var _ ProblemStore = &weedfilerStorage{}

type weedfilerStorage struct {
	URL       string
	AuthToken string
}

func NewWeed(url string) *weedfilerStorage {
	return &weedfilerStorage{
		URL: url,
	}
}

// NewWeedWithToken constructs an object storage backend with a bearer token.
func NewWeedWithToken(url, authToken string) *weedfilerStorage {
	return &weedfilerStorage{
		URL:       url,
		AuthToken: authToken,
	}
}

func (s *weedfilerStorage) baseURL() string {
	return strings.TrimRight(s.URL, "/")
}

type v1ObjectInfo struct {
	Key       string `json:"key"`
	SizeBytes int64  `json:"size_bytes"`
	Digest    string `json:"digest"`
}

type v1ListResponse struct {
	Objects        []v1ObjectInfo `json:"objects"`
	CommonPrefixes []string       `json:"common_prefixes"`
	NextCursor     string         `json:"next_cursor,omitempty"`
}

func formatBlake3ReprDigest(sum [32]byte) string {
	return "blake3=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// parseBlake3ReprDigest scans RFC 9530 Repr-Digest header values and extracts a 32-byte blake3 digest if present.
func parseBlake3ReprDigest(values []string) ([]byte, bool, error) {
	for _, hv := range values {
		for _, part := range strings.Split(hv, ",") {
			member := strings.TrimSpace(part)
			eq := strings.IndexByte(member, '=')
			if eq <= 0 {
				continue
			}
			alg := strings.TrimSpace(member[:eq])
			if alg != "blake3" {
				continue
			}
			rest := strings.TrimSpace(member[eq+1:])
			if !strings.HasPrefix(rest, ":") {
				return nil, false, fmt.Errorf("malformed blake3 Repr-Digest value")
			}
			rest = rest[1:]
			closeIdx := strings.IndexByte(rest, ':')
			if closeIdx < 0 {
				return nil, false, fmt.Errorf("unterminated blake3 Repr-Digest byte sequence")
			}
			raw, err := base64.StdEncoding.DecodeString(rest[:closeIdx])
			if err != nil {
				return nil, false, fmt.Errorf("decode blake3 Repr-Digest base64: %w", err)
			}
			if len(raw) != 32 {
				return nil, false, fmt.Errorf("blake3 Repr-Digest length %d != 32", len(raw))
			}
			return raw, true, nil
		}
	}
	return nil, false, nil
}

func readErrorDetail(r io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(r, 512))
	return strings.TrimSpace(string(raw))
}

func filerUpload(ctx context.Context, localName, remoteName, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	if stat, err = tools.StatFile(localName, true); err != nil || stat == nil {
		return stat, err
	}
	b3sum, b3size, err := tools.Blake3File(localName)
	if err != nil {
		return nil, err
	}
	if b3size != stat.GetSize() {
		return nil, fmt.Errorf("local file size changed during hashing (%d != %d)", b3size, stat.GetSize())
	}
	reprDigest := formatBlake3ReprDigest(b3sum)
	b3hex := hex.EncodeToString(b3sum[:])

	if checksum != "" {
		switch {
		case strings.HasPrefix(checksum, "sha1:"):
			if stat.GetChecksum() != checksum {
				return nil, fmt.Errorf("checksum mismatch, local %q != %q", stat.GetChecksum(), checksum)
			}
		case strings.HasPrefix(checksum, "blake3=:"):
			if reprDigest != checksum {
				return nil, fmt.Errorf("checksum mismatch, local %q != %q", reprDigest, checksum)
			}
		case strings.HasPrefix(checksum, "blake3:"):
			if "blake3:"+b3hex != checksum {
				return nil, fmt.Errorf("checksum mismatch, local %q != %q", "blake3:"+b3hex, checksum)
			}
		default:
			if stat.GetChecksum() != checksum {
				return nil, fmt.Errorf("checksum mismatch, local %q != %q", stat.GetChecksum(), checksum)
			}
		}
	}

	var req *http.Request
	if stat.GetSize() == 0 {
		req, err = http.NewRequestWithContext(ctx, http.MethodPut, remoteName, http.NoBody)
		if err == nil {
			req.GetBody = func() (io.ReadCloser, error) {
				return http.NoBody, nil
			}
		}
	} else {
		local, oerr := os.Open(localName)
		if oerr != nil {
			return nil, fmt.Errorf("os.Open(%q): %w", localName, oerr)
		}
		defer local.Close()
		req, err = http.NewRequestWithContext(ctx, http.MethodPut, remoteName, local)
		if err == nil {
			req.GetBody = func() (io.ReadCloser, error) {
				return os.Open(localName)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest('PUT', %q, %q): %w", remoteName, localName, err)
	}
	req.ContentLength = int64(stat.GetSize())
	req.Header.Set("Repr-Digest", reprDigest)
	if moduleType != "" {
		req.Header.Set("X-Meta-module-type", moduleType)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.Do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		detail := readErrorDetail(resp.Body)
		if detail != "" {
			return nil, fmt.Errorf("invalid status: %d %q (%s)", resp.StatusCode, resp.Status, detail)
		}
		return nil, fmt.Errorf("invalid status: %d %q", resp.StatusCode, resp.Status)
	}

	if gotDigest, hasDigest, derr := parseBlake3ReprDigest(resp.Header.Values("Repr-Digest")); derr != nil {
		return nil, fmt.Errorf("upload integrity verification failed: %w", derr)
	} else if hasDigest && subtle.ConstantTimeCompare(gotDigest, b3sum[:]) != 1 {
		return nil, fmt.Errorf("upload integrity verification failed: Repr-Digest mismatch")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read upload response: %w", err)
	}
	if len(bytes.TrimSpace(body)) > 0 {
		var info v1ObjectInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, fmt.Errorf("json.Decode: %w", err)
		}
		if info.SizeBytes != int64(stat.GetSize()) {
			return nil, fmt.Errorf("upload integrity verification failed: size %d != %d", info.SizeBytes, stat.GetSize())
		}
		if info.Digest != "" && info.Digest != reprDigest {
			return nil, fmt.Errorf("upload integrity verification failed: digest %q != %q", info.Digest, reprDigest)
		}
	}
	return stat, nil
}

// remoteName must be full URL.
func filerDownload(ctx context.Context, localName, remoteName, authToken string) (stat *contester_proto.FileStat, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteName, nil)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			_, _ = io.Copy(io.Discard, resp.Body)
			return nil, fmt.Errorf("%w: %q", fs.ErrNotExist, remoteName)
		}
		detail := readErrorDetail(resp.Body)
		if detail != "" {
			return nil, fmt.Errorf("invalid status: %d %q (%s)", resp.StatusCode, resp.Status, detail)
		}
		return nil, fmt.Errorf("invalid status: %d %q", resp.StatusCode, resp.Status)
	}

	expectedDigest, hasExpectedDigest, derr := parseBlake3ReprDigest(resp.Header.Values("Repr-Digest"))
	if derr != nil {
		return nil, fmt.Errorf("invalid server Repr-Digest: %w", derr)
	}

	local, err := os.Create(localName)
	if err != nil {
		return nil, err
	}
	cleanupOnErr := true
	defer func() {
		_ = local.Close()
		if cleanupOnErr {
			_ = os.Remove(localName)
		}
	}()

	hasher := blake3.New(32, nil)
	n, err := io.Copy(io.MultiWriter(local, hasher), resp.Body)
	if err != nil {
		return nil, err
	}
	if err := local.Close(); err != nil {
		return nil, err
	}

	if resp.ContentLength != -1 && n != resp.ContentLength {
		return nil, fmt.Errorf("incomplete read %d want %d", n, resp.ContentLength)
	}
	if hasExpectedDigest {
		gotDigest := hasher.Sum(nil)
		if subtle.ConstantTimeCompare(gotDigest, expectedDigest) != 1 {
			return nil, fmt.Errorf("download integrity verification failed: Repr-Digest mismatch")
		}
	}

	cleanupOnErr = false
	return tools.StatFile(localName, true)
}

func FilerReadRemote(ctx context.Context, name, authToken string) (*RemoteFile, error) {
	name = remoteNameCleanup(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, name, nil)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("%w: %q", fs.ErrNotExist, name)
		}
		detail := readErrorDetail(resp.Body)
		_ = resp.Body.Close()
		if detail != "" {
			return nil, fmt.Errorf("invalid status: %d %q (%s)", resp.StatusCode, resp.Status, detail)
		}
		return nil, fmt.Errorf("invalid status: %d %q", resp.StatusCode, resp.Status)
	}
	result := RemoteFile{
		Body: resp.Body,
	}

	if sz := resp.Header.Get("Content-Length"); sz != "" {
		if isz, err := strconv.ParseUint(sz, 10, 64); err == nil {
			result.Stat.Size = isz
		}
	}

	return &result, nil
}

func remoteNameCleanup(s string) string {
	return strings.TrimPrefix(s, "filer:")
}

func FilerCopy(ctx context.Context, localName, remoteName string, toRemote bool, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	remoteName = remoteNameCleanup(remoteName)
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
	if !strings.Contains(remoteName, "://") {
		resolved := ObjectURL(s.baseURL(), remoteName)
		if resolved == "" {
			return nil, fmt.Errorf("invalid storage object ref %q", remoteName)
		}
		remoteName = resolved
	}
	if authToken == "" {
		authToken = s.AuthToken
	}
	return FilerCopy(ctx, localName, remoteName, toRemote, checksum, moduleType, authToken)
}

func (s *weedfilerStorage) ReadRemote(ctx context.Context, name, authToken string) (*RemoteFile, error) {
	if authToken == "" {
		authToken = s.AuthToken
	}
	return FilerReadRemote(ctx, name, authToken)
}

func (s *weedfilerStorage) GetAllManifests(ctx context.Context) ([]ProblemManifest, error) {
	return nil, fmt.Errorf("GetAllManifests is not supported by object storage v1 protocol")
}

func (s *weedfilerStorage) GetNextRevision(ctx context.Context, id string) (int, error) {
	bucket := ProblemBucket(id)
	if bucket == "" {
		return 1, fmt.Errorf("invalid problem id %q", id)
	}
	maxRev := 0
	cursor := ""
	for {
		u := s.baseURL() + "/v1/buckets/" + url.PathEscape(bucket) + "/objects?delimiter=%2F&limit=1000"
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return 1, err
		}
		if s.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+s.AuthToken)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 1, err
		}
		if resp.StatusCode == http.StatusNotFound {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return 1, nil
		}
		if resp.StatusCode != http.StatusOK {
			detail := readErrorDetail(resp.Body)
			_ = resp.Body.Close()
			return 1, fmt.Errorf("list revisions for %s: HTTP %d (%s)", id, resp.StatusCode, detail)
		}
		var lr v1ListResponse
		err = json.NewDecoder(resp.Body).Decode(&lr)
		_ = resp.Body.Close()
		if err != nil {
			return 1, err
		}
		for _, pref := range lr.CommonPrefixes {
			revStr := strings.TrimSuffix(pref, "/")
			if rev, perr := strconv.Atoi(revStr); perr == nil && rev > maxRev {
				maxRev = rev
			}
		}
		if lr.NextCursor == "" {
			break
		}
		cursor = lr.NextCursor
	}
	return maxRev + 1, nil
}

func (s *weedfilerStorage) SetManifest(ctx context.Context, manifest *ProblemManifest) error {
	bucket := ProblemBucket(manifest.Id)
	if bucket == "" {
		return fmt.Errorf("invalid problem id %q", manifest.Id)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	sum := blake3.Sum256(data)
	ref := bucket + "/" + strconv.Itoa(manifest.Revision) + "/manifest.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, ObjectURL(s.baseURL(), ref), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Repr-Digest", formatBlake3ReprDigest(sum))
	if s.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.AuthToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("set manifest HTTP %d %q", resp.StatusCode, resp.Status)
	}
	return nil
}

func (s *weedfilerStorage) String() string {
	return s.URL
}
