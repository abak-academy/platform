package service

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"akademi-bimbel/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// newOfflineStorageService builds a Service backed by a minio client that
// never dials out for presigning: PUT presigning is pure local signing as
// long as Region is set explicitly (matching production's newStorageClient in
// cmd/api/main.go) — without it, minio-go falls back to a network bucket-
// location lookup, which would hit whatever's actually listening on
// localhost:9000 in dev. GetObject calls (past OpenAvatar's prefix gate) do
// still dial out; these tests only assert they don't fail with ErrUploadNotFound.
func newOfflineStorageService(t *testing.T) *Service {
	t.Helper()
	client, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("ak", "sk", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	return &Service{storage: client, cfg: &config.Config{ObjectStorageBucketName: "bucket"}}
}

// The avatar read-proxy is unauthenticated, and avatars share one bucket with
// certificates and private student PII. OpenAvatar must therefore refuse any key
// outside the allowlisted prefixes (and reject path traversal) before it ever
// touches storage — otherwise the proxy would leak certs and PII to anyone.
func TestOpenAvatar_RejectsNonAvatarKeys(t *testing.T) {
	s := newOfflineStorageService(t)

	blocked := []string{
		"certificates/00000000-0000-0000-0000-000000000000.pdf",
		"cards/00000000-0000-0000-0000-000000000000.pdf",
		"student-bulk/school-1/import.csv",
		"avatars/../certificates/leak.pdf",
		"random-key",
		"",
	}
	for _, key := range blocked {
		if _, _, err := s.OpenAvatar(context.Background(), key); !errors.Is(err, ErrUploadNotFound) {
			t.Errorf("OpenAvatar(%q): expected ErrUploadNotFound, got %v", key, err)
		}
	}
}

// Product images and question assets moved off the avatars/ prefix (see
// GeneratePresignedUploadURL), so the read-proxy's allowlist must widen to
// match or every newly-uploaded product image, question image and listening
// audio 404s on read. Passing the gate means OpenAvatar proceeds to a real
// storage call — with no server at localhost:9000 that call fails, but it
// must fail with something other than ErrUploadNotFound.
func TestOpenAvatar_AcceptsWidenedPrefixes(t *testing.T) {
	s := newOfflineStorageService(t)

	accepted := []string{
		"product/x.png",
		"question/x.png",
	}
	for _, key := range accepted {
		if _, _, err := s.OpenAvatar(context.Background(), key); errors.Is(err, ErrUploadNotFound) {
			t.Errorf("OpenAvatar(%q): expected key to pass the prefix gate, got ErrUploadNotFound", key)
		}
	}
}

// GeneratePresignedUploadURL now takes a caller-supplied prefix (student
// endpoint routes avatars/product, admin endpoints route question) and must
// reject anything outside the three-prefix allowlist before signing.
func TestGeneratePresignedUploadURL_PrefixAllowlist(t *testing.T) {
	s := newOfflineStorageService(t)

	t.Run("allowlisted prefixes produce a namespaced key", func(t *testing.T) {
		for _, prefix := range []string{"avatars", "product", "question"} {
			resp, err := s.GeneratePresignedUploadURL(context.Background(), "user-1", prefix, "file.jpg", "image/jpeg")
			if err != nil {
				t.Fatalf("GeneratePresignedUploadURL(prefix=%q): unexpected error: %v", prefix, err)
			}
			want := regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + `/user-1/`)
			if !want.MatchString(resp.Key) {
				t.Errorf("prefix=%q: key %q does not match %s", prefix, resp.Key, want)
			}
		}
	})

	t.Run("non-allowlisted prefixes sign nothing", func(t *testing.T) {
		for _, prefix := range []string{"certificates", "../avatars", ""} {
			resp, err := s.GeneratePresignedUploadURL(context.Background(), "user-1", prefix, "file.jpg", "image/jpeg")
			if err == nil {
				t.Errorf("prefix=%q: expected error, got resp=%+v", prefix, resp)
			}
			if resp != nil {
				t.Errorf("prefix=%q: expected nil response on error, got %+v", prefix, resp)
			}
		}
	})
}
