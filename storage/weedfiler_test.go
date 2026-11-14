package storage

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lukechampine.com/blake3"
)

func TestFilerCopyUploadV1(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "compiledModule")
	payload := []byte("ELF-binary-payload-12345")
	if err := os.WriteFile(localPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	wantSum := blake3.Sum256(payload)
	wantReprDigest := formatBlake3ReprDigest(wantSum)
	sha1Sum := sha1.Sum(payload)
	wantSha1 := "sha1:" + strings.ToLower(hex.EncodeToString(sha1Sum[:]))

	var gotAuth, gotMeta, gotDigest string
	var gotLen int64
	var gotBody []byte
	var hits int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/v1/buckets/art-fe1/objects/42/compiledModule")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		if r.Method != http.MethodPut || r.URL.Path != "/v1/buckets/art-fe1/objects/42/compiledModule" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotMeta = r.Header.Get("X-Meta-Module-Type")
		gotDigest = r.Header.Get("Repr-Digest")
		gotLen = r.ContentLength
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Repr-Digest", wantReprDigest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(v1ObjectInfo{
			Key:       "42/compiledModule",
			SizeBytes: int64(len(payload)),
			Digest:    wantReprDigest,
		})
	}))
	defer srv.Close()

	// Exercise redirect retry via GetBody as well as standard 201 Created handling.
	stat, err := FilerCopy(context.Background(), localPath, srv.URL+"/redirect", true, wantSha1, "exe", "tok-upload")
	if err != nil {
		t.Fatalf("FilerCopy upload: %v", err)
	}
	if hits != 2 {
		t.Fatalf("expected 2 requests across redirect, got %d", hits)
	}
	if gotAuth != "Bearer tok-upload" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer tok-upload")
	}
	if gotMeta != "exe" {
		t.Fatalf("X-Meta-Module-Type = %q, want exe", gotMeta)
	}
	if gotDigest != wantReprDigest {
		t.Fatalf("Repr-Digest = %q, want %q", gotDigest, wantReprDigest)
	}
	if gotLen != int64(len(payload)) || string(gotBody) != string(payload) {
		t.Fatalf("uploaded body mismatch: len=%d body=%q", gotLen, gotBody)
	}
	if stat.GetSize() != uint64(len(payload)) || stat.GetChecksum() != wantSha1 {
		t.Fatalf("returned FileStat mismatch: %+v", stat)
	}
}

func TestFilerCopyUploadEmptyAndErrors(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	emptySum := blake3.Sum256(nil)
	emptyDigest := formatBlake3ReprDigest(emptySum)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/buckets/art-fe1/objects/1/tests/1/output":
			if r.ContentLength != 0 {
				t.Fatalf("empty ContentLength = %d, want 0", r.ContentLength)
			}
			if got := r.Header.Get("Repr-Digest"); got != emptyDigest {
				t.Fatalf("empty Repr-Digest = %q, want %q", got, emptyDigest)
			}
			w.Header().Set("Repr-Digest", emptyDigest)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(v1ObjectInfo{
				Key:       "1/tests/1/output",
				SizeBytes: 0,
				Digest:    emptyDigest,
			})
		case "/tampered":
			badSum := blake3.Sum256([]byte("wrong"))
			w.Header().Set("Repr-Digest", formatBlake3ReprDigest(badSum))
			w.WriteHeader(http.StatusOK)
		case "/forbidden":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"title":"Forbidden"}`))
		}
	}))
	defer srv.Close()

	if _, err := FilerCopy(context.Background(), emptyPath, srv.URL+"/v1/buckets/art-fe1/objects/1/tests/1/output", true, "", "", ""); err != nil {
		t.Fatalf("empty file upload failed: %v", err)
	}
	if _, err := FilerCopy(context.Background(), emptyPath, srv.URL+"/tampered", true, "", "", ""); err == nil || !strings.Contains(err.Error(), "Repr-Digest mismatch") {
		t.Fatalf("expected Repr-Digest mismatch error, got %v", err)
	}
	if _, err := FilerCopy(context.Background(), emptyPath, srv.URL+"/forbidden", true, "", "", ""); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 error, got %v", err)
	}
}

func TestFilerCopyDownloadV1(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("test input data\n1 2 3\n")
	sum := blake3.Sum256(payload)
	goodDigest := formatBlake3ReprDigest(sum)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			if got := r.Header.Get("Authorization"); got != "Bearer tok-read" {
				t.Fatalf("Authorization = %q, want %q", got, "Bearer tok-read")
			}
			w.Header().Set("Repr-Digest", goodDigest)
			_, _ = w.Write(payload)
		case "/bad-digest":
			badSum := blake3.Sum256([]byte("corrupted"))
			w.Header().Set("Repr-Digest", formatBlake3ReprDigest(badSum))
			_, _ = w.Write(payload)
		case "/missing":
			http.NotFound(w, r)
		case "/denied":
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()

	okPath := filepath.Join(dir, "input.txt")
	st, err := FilerCopy(context.Background(), okPath, srv.URL+"/ok", false, "", "", "tok-read")
	if err != nil {
		t.Fatalf("download ok failed: %v", err)
	}
	if st.GetSize() != uint64(len(payload)) {
		t.Fatalf("downloaded size = %d, want %d", st.GetSize(), len(payload))
	}
	got, err := os.ReadFile(okPath)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("downloaded content mismatch: %q err=%v", got, err)
	}

	badPath := filepath.Join(dir, "bad.txt")
	if _, err := FilerCopy(context.Background(), badPath, srv.URL+"/bad-digest", false, "", "", "tok-read"); err == nil || !strings.Contains(err.Error(), "Repr-Digest mismatch") {
		t.Fatalf("expected Repr-Digest mismatch, got %v", err)
	}
	if _, statErr := os.Stat(badPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("corrupted download should not remain on disk, stat err=%v", statErr)
	}

	missingPath := filepath.Join(dir, "missing.txt")
	if _, err := FilerCopy(context.Background(), missingPath, srv.URL+"/missing", false, "", "", "tok-read"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist for 404, got %v", err)
	}
	if _, statErr := os.Stat(missingPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("404 should not create empty local file, stat err=%v", statErr)
	}

	deniedPath := filepath.Join(dir, "denied.txt")
	if _, err := FilerCopy(context.Background(), deniedPath, srv.URL+"/denied", false, "", "", "tok-read"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 error, got %v", err)
	}
	if _, statErr := os.Stat(deniedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("403 should not create empty local file, stat err=%v", statErr)
	}
}

func TestProblemBucketParity(t *testing.T) {
	// Short direct URL
	if got := ProblemBucket("direct://fe1/a-plus-b"); got != "prob-direct-fe1-a-plus-b" {
		t.Fatalf("direct bucket = %q", got)
	}

	// Long Polygon URL with mixed-case path segments: must hash raw BEFORE lowercasing.
	rawURL := "https://polygon.codeforces.com/p3e0Hkv/IAndrosov/squares-inside-a-square"
	raw := "prob-polygon-https-polygon.codeforces.com-p3e0Hkv-IAndrosov-squares-inside-a-square"
	h := blake3.Sum256([]byte(raw))
	want := strings.ToLower(raw)[:46] + "-" + hex.EncodeToString(h[:8])
	got := ProblemBucket(rawURL)
	if got != want || len(got) != 63 {
		t.Fatalf("long polygon bucket = %q (len=%d), want %q", got, len(got), want)
	}
}
