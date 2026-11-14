package storage

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"lukechampine.com/blake3"
)

type ProblemStore interface {
	Backend

	GetNextRevision(ctx context.Context, id string) (int, error)
	SetManifest(ctx context.Context, manifest *ProblemManifest) error
	GetAllManifests(ctx context.Context) ([]ProblemManifest, error)
}

type ProblemManifest struct {
	Key string `bson:"_id"`

	Id       string `json:"id"`
	Revision int    `json:"revision"`

	TestCount       int    `bson:"testCount" json:"testCount"`
	TimeLimitMicros int64  `bson:"timeLimitMicros" json:"timeLimitMicros"`
	MemoryLimit     int64  `bson:"memoryLimit" json:"memoryLimit"`
	Stdio           bool   `bson:"stdio" json:"stdio,omitempty"`
	TesterName      string `bson:"testerName" json:"testerName"`
	Answers         []int  `bson:"answers" json:"answers,omitempty"`
	InteractorName  string `bson:"interactorName,omitempty" json:"interactorName,omitempty"`
	CombinedHash    string `bson:"combinedHash,omitempty" json:"combinedHash,omitempty"`
}

// GetGridPrefix returns the canonical "<bucket>/<revision>/" prefix for this manifest.
func (s *ProblemManifest) GetGridPrefix() string {
	bucket := ProblemBucket(s.Id)
	if bucket == "" {
		return ""
	}
	return bucket + "/" + strconv.FormatInt(int64(s.Revision), 10) + "/"
}

// ObjectPrefixURL returns the full /v1/ object URL prefix ending in "/" for this manifest.
func (s *ProblemManifest) ObjectPrefixURL(baseURL string) string {
	gp := s.GetGridPrefix()
	if gp == "" {
		return ""
	}
	return ObjectPrefixURL(baseURL, gp)
}

// IdToGridPrefix returns the virtual bucket name for a problem URI.
func IdToGridPrefix(id string) string {
	return ProblemBucket(id)
}

// SanitizeBucket normalizes raw into a valid v1 bucket name matching ^[a-z0-9][a-z0-9._-]{0,62}$.
// Names exceeding 63 bytes are truncated and suffixed with a BLAKE3 hash of raw before lowercasing.
func SanitizeBucket(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			b.WriteByte(c)
		} else {
			b.WriteByte('-')
		}
	}
	s := b.String()
	if s == "" || !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= '0' && s[0] <= '9')) {
		s = "b" + s
	}
	if len(s) > 63 {
		h := blake3.Sum256([]byte(raw))
		s = s[:46] + "-" + hex.EncodeToString(h[:8])
	}
	return s
}

// ProblemBucket returns the virtual bucket name for a Polygon HTTP(S) or direct:// problem URL.
func ProblemBucket(problemID string) string {
	u, err := url.Parse(problemID)
	if err != nil || u.Host == "" {
		return ""
	}
	trimmed := strings.Trim(u.Path, "/")
	if trimmed == "" {
		return ""
	}
	switch u.Scheme {
	case "http", "https":
		raw := "prob-polygon-" + u.Scheme + "-" + u.Host + "-" + strings.ReplaceAll(trimmed, "/", "-")
		return SanitizeBucket(raw)
	case "direct":
		raw := "prob-direct-" + u.Host + "-" + strings.ReplaceAll(trimmed, "/", "-")
		return SanitizeBucket(raw)
	default:
		return ""
	}
}

// SplitRef splits a canonical "<bucket>/<key>" reference into its bucket and object key.
func SplitRef(ref string) (bucket, key string, err error) {
	trimmed := strings.TrimLeft(ref, "/")
	idx := strings.IndexByte(trimmed, '/')
	if idx <= 0 {
		return "", "", fmt.Errorf("invalid object ref %q: missing bucket/key separator", ref)
	}
	bucket = trimmed[:idx]
	key = strings.TrimLeft(trimmed[idx+1:], "/")
	if key == "" {
		return "", "", fmt.Errorf("invalid object ref %q: empty key", ref)
	}
	return bucket, key, nil
}

// SplitPrefixRef splits a "<bucket>/<prefix>" reference, allowing an empty prefix.
func SplitPrefixRef(ref string) (bucket, prefix string, err error) {
	trimmed := strings.TrimLeft(ref, "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("empty object prefix ref")
	}
	idx := strings.IndexByte(trimmed, '/')
	if idx < 0 {
		return trimmed, "", nil
	}
	if idx == 0 {
		return "", "", fmt.Errorf("invalid object prefix ref %q: empty bucket", ref)
	}
	return trimmed[:idx], strings.TrimLeft(trimmed[idx+1:], "/"), nil
}

func escapeObjectPath(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// ObjectURL constructs the v1 object URL "{baseURL}/v1/buckets/{bucket}/objects/{key}".
func ObjectURL(baseURL, ref string) string {
	if strings.Contains(ref, "://") {
		return ref
	}
	bucket, key, err := SplitRef(ref)
	if err != nil {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/v1/buckets/" + url.PathEscape(bucket) + "/objects/" + escapeObjectPath(key)
}

// ObjectPrefixURL constructs a v1 object URL prefix ending in "/" from a "<bucket>/<prefix>" ref.
func ObjectPrefixURL(baseURL, prefixRef string) string {
	bucket, prefix, err := SplitPrefixRef(prefixRef)
	if err != nil {
		return ""
	}
	base := strings.TrimRight(baseURL, "/") + "/v1/buckets/" + url.PathEscape(bucket) + "/objects/"
	if prefix == "" {
		return base
	}
	prefix = strings.TrimRight(prefix, "/") + "/"
	return base + escapeObjectPath(prefix)
}
