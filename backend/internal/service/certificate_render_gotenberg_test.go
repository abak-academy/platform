package service

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestCertificateRendersThroughGotenberg is E1's verification-debt item: the
// sidecar has never rendered a document end to end, and cached certificates
// masked its total absence for weeks while the api logs stayed silent.
//
// It is skipped unless GOTENBERG_URL is set, so it never runs in CI without a
// sidecar. Point it at the local stack (host port 3001 per deploy/compose/local.yml):
//
//	GOTENBERG_URL=http://localhost:3001 \
//	  CERT_PDF_OUT=/tmp/cert-proof.pdf \
//	  go test ./internal/service/ -run TestCertificateRendersThroughGotenberg -v
//
// Byte assertions are not the point — the PDF must be opened and looked at.
// This project already shipped a certificate that rendered fully upside-down
// while byte-level tests stayed green, so write it out and inspect it.
func TestCertificateRendersThroughGotenberg(t *testing.T) {
	url := os.Getenv("GOTENBERG_URL")
	if url == "" {
		t.Skip("GOTENBERG_URL not set; this test needs a real Gotenberg sidecar")
	}

	for _, template := range []string{"classic", "modern", "elegant"} {
		t.Run(template, func(t *testing.T) {
			layout := defaultLayout(template)
			vals := certificateFieldValues(
				"Try Out Nasional Matematika 2026",
				"Saifullah Panca",
				"30 Juli 2026",
				"ABAK/2026/VII/000123",
			)
			// modern is the only score-bearing layout; these come from
			// certificateSessionValues in production, which needs a session.
			vals["score"] = "86"
			vals["max_score"] = "100"
			vals["score_percent"] = "86%"
			vals["rank"] = "12"
			vals["percentile"] = "Top 15%" // production format: certificateSessionValues prefixes "Top "
			vals["duration"] = "90 minutes"
			vals["total_questions"] = "50 questions"

			bg := builtinCertificateBackground(template)
			if len(bg) == 0 {
				t.Fatalf("no embedded background for template %q", template)
			}

			html, err := buildCertificateHTML(layout, vals, bg, nil)
			if err != nil {
				t.Fatalf("buildCertificateHTML: %v", err)
			}

			pdf, err := newGotenbergPDFGenerator(url, http.DefaultClient).
				RenderHTML(context.Background(), html)
			if err != nil {
				t.Fatalf("Gotenberg render failed: %v", err)
			}

			// A PDF, not an error page rendered as one.
			if len(pdf) < 1024 {
				t.Fatalf("suspiciously small PDF: %d bytes", len(pdf))
			}
			if got := string(pdf[:4]); got != "%PDF" {
				t.Fatalf("response is not a PDF, starts with %q", got)
			}

			if out := os.Getenv("CERT_PDF_OUT"); out != "" {
				path := filepath.Join(
					filepath.Dir(out),
					template+"-"+filepath.Base(out),
				)
				if err := os.WriteFile(path, pdf, 0o644); err != nil {
					t.Fatalf("write %s: %v", path, err)
				}
				t.Logf("wrote %s (%d bytes) — OPEN IT AND LOOK", path, len(pdf))
			}
		})
	}
}
