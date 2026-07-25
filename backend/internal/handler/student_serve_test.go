package handler

import "testing"

func TestSafeServeContentType(t *testing.T) {
	cases := []struct {
		stored       string
		wantServed   string
		wantDownload bool
	}{
		{"image/png", "image/png", false},
		{"image/jpeg", "image/jpeg", false},
		{"image/webp", "image/webp", false},
		{"image/gif", "image/gif", false},
		// The XSS-capable types must never be served inline.
		{"text/html", "application/octet-stream", true},
		{"image/svg+xml", "application/octet-stream", true},
		{"application/pdf", "application/octet-stream", true},
		{"", "application/octet-stream", true},
	}
	for _, c := range cases {
		served, download := safeServeContentType(c.stored)
		if served != c.wantServed || download != c.wantDownload {
			t.Errorf("safeServeContentType(%q) = (%q, %v), want (%q, %v)",
				c.stored, served, download, c.wantServed, c.wantDownload)
		}
	}
}
