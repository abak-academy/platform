package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatalf("encode png fixture: %v", err)
	}
	return buf.Bytes()
}

// The System Config contract stores app_logo_url as an ordinary https URL. The
// SSRF fix for student photos briefly routed it through the storage-key-only
// avatar loader, which silently dropped every configured logo from generated
// cards — this is that regression.
func TestFetchPublicImage_HTTPSLogoURL_IsFetched(t *testing.T) {
	want := testPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	// httptest binds to loopback, which the address guard rejects by design, so
	// this test drives the fetch through a dialer that treats it as public —
	// the guard itself is covered by the tests below.
	got, err := fetchImageWithDialGuard(context.Background(), srv.URL, func(net.IP) bool { return true })
	if err != nil {
		t.Fatalf("fetching a configured https logo must succeed, got: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("fetched logo bytes do not match what the server served")
	}
}

// An internal address must stay unreachable even though the value comes from a
// privileged admin: a typo or a copied metadata URL should not turn card
// rendering into an internal-network probe.
func TestFetchPublicImage_InternalAddress_IsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNGBytes(t))
	}))
	defer srv.Close()

	if _, err := fetchPublicImage(context.Background(), srv.URL); err == nil {
		t.Fatal("expected a loopback URL to be refused")
	}
}

func TestFetchPublicImage_RejectsNonImageAndBadScheme(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"file scheme", "file:///etc/passwd"},
		{"gopher scheme", "gopher://example.com/1"},
		{"no host", "https://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := fetchPublicImage(context.Background(), tc.url); err == nil {
				t.Errorf("expected %s to be refused", tc.name)
			}
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	defer srv.Close()
	if _, err := fetchImageWithDialGuard(context.Background(), srv.URL, func(net.IP) bool { return true }); err == nil {
		t.Error("expected a non-image response to be refused")
	}
}

func TestIsPublicIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"10.1.2.3",        // private
		"192.168.1.10",    // private
		"172.16.0.9",      // private
		"169.254.169.254", // cloud metadata (link-local)
		"0.0.0.0",         // unspecified
		"::1",             // IPv6 loopback
		"fd00::1",         // IPv6 unique-local
		"fe80::1",         // IPv6 link-local
	}
	for _, s := range blocked {
		if isPublicIP(net.ParseIP(s)) {
			t.Errorf("%s must not be treated as public", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"} {
		if !isPublicIP(net.ParseIP(s)) {
			t.Errorf("%s must be treated as public", s)
		}
	}
}

// loadCardLogoImage must still accept a logo stored as one of our own object
// keys, so a logo migrated into storage keeps working without an outbound call.
func TestLoadCardLogoImage_StorageKey_TakesTheStoragePath(t *testing.T) {
	svc := &Service{}
	// No storage client is configured, so the storage path returns nil rather
	// than falling through to an outbound fetch of a bare key.
	if got := svc.loadCardLogoImage(context.Background(), "avatars/tenant/logo.png"); got != nil {
		t.Errorf("expected nil from the storage path with no storage configured, got %d bytes", len(got))
	}
}

func TestLoadCardLogoImage_Empty_ReturnsNil(t *testing.T) {
	svc := &Service{}
	if got := svc.loadCardLogoImage(context.Background(), ""); got != nil {
		t.Errorf("expected nil for an unset logo, got %d bytes", len(got))
	}
}
