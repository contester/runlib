package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type testJWTGrant struct {
	Bucket   string   `json:"bucket"`
	Key      string   `json:"key,omitempty"`
	Prefix   string   `json:"prefix,omitempty"`
	Methods  []string `json:"methods"`
	MaxBytes int64    `json:"max_bytes,omitempty"`
}

func signHS256TestJWT(t *testing.T, secret []byte, grants []testJWTGrant) string {
	t.Helper()
	hdrJSON, err := json.Marshal(struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(struct {
		Sub    string         `json:"sub"`
		Exp    int64          `json:"exp"`
		Grants []testJWTGrant `json:"grants"`
	}{
		Sub:    "invoker-test",
		Exp:    time.Now().Add(5 * time.Minute).Unix(),
		Grants: grants,
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	hEnc := base64.RawURLEncoding.EncodeToString(hdrJSON)
	cEnc := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := hEnc + "." + cEnc
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	sig := mac.Sum(nil)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func startRunlibCrabfiler(t *testing.T, binPath string, extraEnv []string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	cmd := exec.Command(binPath, "serve")
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"CRABFILER_BACKEND=localfs",
		"CRABFILER_DATA_DIR="+t.TempDir(),
		"CRABFILER_LISTEN_HTTP="+addr,
		"CRABFILER_ALLOW_ANONYMOUS=false",
		"CRABFILER_LOG_LEVEL=warn",
	)
	env = append(env, extraEnv...)
	cmd.Env = env

	var logs syncLogBuffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start crabfiler: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
		}
		select {
		case <-waitDone:
		case <-time.After(3 * time.Second):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-waitDone
		}
	})

	probeClient := &http.Client{Timeout: 250 * time.Millisecond}
	probeURL := "http://" + addr + "/v1/buckets/probe/objects/probe"
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-waitDone:
			t.Fatalf("crabfiler exited early (%v); logs:\n%s", err, logs.String())
		default:
		}
		resp, err := probeClient.Get(probeURL)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized {
				return "http://" + addr
			}
			t.Fatalf("expected 401 on anonymous probe, got %d; logs:\n%s", resp.StatusCode, logs.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for crabfiler on %s; logs:\n%s", addr, logs.String())
	return ""
}

func TestCrabfilerLiveFilerCopy(t *testing.T) {
	binPath := os.Getenv("CRABFILER_BIN")
	if binPath == "" {
		binPath = filepath.Join("..", "..", "crabfiler", "target", "release", "crabfiler")
	}
	if info, err := os.Stat(binPath); err != nil || info.IsDir() {
		t.Skipf("crabfiler binary not found at %s (set CRABFILER_BIN to enable): %v", binPath, err)
	}

	hmacSecret := []byte("runlib-test-hmac-secret-32-bytes!!")
	baseURL := startRunlibCrabfiler(t, binPath, []string{
		"CRABFILER_AUTH_TOKENS=root-runlib-secret",
		"CRABFILER_JWT_HMAC_SECRET=" + string(hmacSecret),
	})

	ctx := context.Background()
	dir := t.TempDir()

	// 1. ProblemStore (NewWeedWithToken) flow: revisions + manifest + Copy input.
	store := NewWeedWithToken(baseURL, "root-runlib-secret")
	probID := "https://polygon.example.com/p/demo/sum"

	rev, err := store.GetNextRevision(ctx, probID)
	if err != nil {
		t.Fatalf("initial GetNextRevision: %v", err)
	}
	if rev != 1 {
		t.Fatalf("initial GetNextRevision = %d, want 1", rev)
	}

	if err := store.SetManifest(ctx, &ProblemManifest{
		Id:         probID,
		Revision:   1,
		TestCount:  1,
		TesterName: "check.exe",
	}); err != nil {
		t.Fatalf("SetManifest: %v", err)
	}

	localInput := filepath.Join(dir, "input.txt")
	inputPayload := []byte("10 32\n")
	if err := os.WriteFile(localInput, inputPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	inputRef := ProblemBucket(probID) + "/1/tests/1/input.txt"
	stat, err := store.Copy(ctx, localInput, inputRef, true, "", "", "")
	if err != nil {
		t.Fatalf("store.Copy input: %v", err)
	}
	if stat.GetSize() != uint64(len(inputPayload)) {
		t.Fatalf("store.Copy size = %d, want %d", stat.GetSize(), len(inputPayload))
	}

	rev, err = store.GetNextRevision(ctx, probID)
	if err != nil {
		t.Fatalf("post-upload GetNextRevision: %v", err)
	}
	if rev != 2 {
		t.Fatalf("post-upload GetNextRevision = %d, want 2", rev)
	}

	// 2. Scoped HS256 capability JWT with FilerCopy upload (non-empty + 0-byte).
	putToken := signHS256TestJWT(t, hmacSecret, []testJWTGrant{
		{
			Bucket:   "art-fe1",
			Prefix:   "77/tests/",
			Methods:  []string{"PUT"},
			MaxBytes: 512,
		},
	})

	localOut := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(localOut, []byte("42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outURL := "filer:" + baseURL + "/v1/buckets/art-fe1/objects/77/tests/1/output"
	outStat, err := FilerCopy(ctx, localOut, outURL, true, "", "text/plain", putToken)
	if err != nil {
		t.Fatalf("FilerCopy PUT non-empty output: %v", err)
	}
	if outStat.GetSize() != 3 {
		t.Fatalf("FilerCopy PUT output size = %d, want 3", outStat.GetSize())
	}

	localEmpty := filepath.Join(dir, "empty.stderr")
	if err := os.WriteFile(localEmpty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stderrURL := "filer:" + baseURL + "/v1/buckets/art-fe1/objects/77/tests/1/stderr"
	errStat, err := FilerCopy(ctx, localEmpty, stderrURL, true, "", "text/plain", putToken)
	if err != nil {
		t.Fatalf("FilerCopy PUT 0-byte stderr: %v", err)
	}
	if errStat.GetSize() != 0 {
		t.Fatalf("FilerCopy PUT empty size = %d, want 0", errStat.GetSize())
	}

	// Verify X-Meta-Module-Type was persisted on crabfiler.
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL+"/v1/buckets/art-fe1/objects/77/tests/1/stderr", nil)
	if err != nil {
		t.Fatal(err)
	}
	headReq.Header.Set("Authorization", "Bearer root-runlib-secret")
	headResp, err := http.DefaultClient.Do(headReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = headResp.Body.Close()
	if got := headResp.Header.Get("X-Meta-Module-Type"); got != "text/plain" {
		t.Fatalf("HEAD X-Meta-Module-Type = %q, want text/plain", got)
	}

	// Upload outside granted prefix must fail with 403.
	badKeyURL := "filer:" + baseURL + "/v1/buckets/art-fe1/objects/77/compiledModule"
	if _, err := FilerCopy(ctx, localOut, badKeyURL, true, "", "", putToken); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 uploading outside 77/tests/, got %v", err)
	}

	// Download using PUT-only token must fail with 403.
	dlDeniedPath := filepath.Join(dir, "dl_denied.txt")
	if _, err := FilerCopy(ctx, dlDeniedPath, outURL, false, "", "", putToken); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 downloading with PUT-only token, got %v", err)
	}

	// 3. Scoped HS256 capability JWT with FilerCopy download (verifies Repr-Digest BLAKE3).
	readToken := signHS256TestJWT(t, hmacSecret, []testJWTGrant{
		{
			Bucket:  ProblemBucket(probID),
			Prefix:  "1/",
			Methods: []string{"GET", "HEAD"},
		},
	})

	dlInputPath := filepath.Join(dir, "dl_input.txt")
	dlStat, err := FilerCopy(ctx, dlInputPath, "filer:"+ObjectURL(baseURL, inputRef), false, "", "", readToken)
	if err != nil {
		t.Fatalf("FilerCopy GET permitted input: %v", err)
	}
	if dlStat.GetSize() != uint64(len(inputPayload)) {
		t.Fatalf("FilerCopy GET size = %d, want %d", dlStat.GetSize(), len(inputPayload))
	}
	gotBytes, err := os.ReadFile(dlInputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, inputPayload) {
		t.Fatalf("downloaded bytes = %q, want %q", gotBytes, inputPayload)
	}

	// Download from ungranted art-fe1 bucket must fail with 403.
	if _, err := FilerCopy(ctx, filepath.Join(dir, "dl_forbidden.txt"), outURL, false, "", "", readToken); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 downloading from ungranted bucket, got %v", err)
	}
}
