//go:build gotenberg_integration

// Real-Gotenberg integration test (FR-6 acceptance gate). Excluded from the
// default suite by the gotenberg_integration build tag, so plain
// `go test ./...` never touches it. CI runs it as its own job — see
// deploy/pipeline/backend-render-gate.sh.
//
// It starts its own Gotenberg container (same testcontainers idiom as the
// Postgres-backed tests), so no setup is needed:
//
//	go test -tags gotenberg_integration -run TestCertificateRender_RealGotenberg ./internal/service/
//
// Set GOTENBERG_URL to point at an already-running instance instead — e.g.
// GOTENBERG_URL=http://localhost:3001 for the deploy/docker-compose.yml sidecar.
//
// Set CERT_SAMPLE_OUT=/path/to/sample.pdf to also dump the rendered PDF for the
// visual acceptance check (page count, orientation, fonts, background, field
// positions) per pdf-layout-needs-visual-verification.
package service

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// gotenbergTestImage is pinned to the same major the deploy compose file runs,
// so the gate exercises what production actually renders against.
const gotenbergTestImage = "gotenberg/gotenberg:8"

// startGotenberg brings up a throwaway Gotenberg and returns its base URL.
func startGotenberg(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        gotenbergTestImage,
			ExposedPorts: []string{"3000/tcp"},
			WaitingFor:   wait.ForHTTP("/health").WithPort("3000/tcp").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start gotenberg container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	endpoint, err := container.PortEndpoint(ctx, "3000/tcp", "http")
	if err != nil {
		t.Fatalf("gotenberg endpoint: %v", err)
	}
	return endpoint
}

func TestCertificateRender_RealGotenberg(t *testing.T) {
	url := os.Getenv("GOTENBERG_URL")
	if url == "" {
		url = startGotenberg(t)
	}

	const tmpl = "classic"
	layout := defaultLayout(tmpl)
	vals := certificateFieldValues(
		"Ujian Nasional Matematika 2026",
		"Saifullah Panca Dwiputra",
		time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC).Format("2 January 2006"),
		"ABK/2026/0001/000042",
	)
	bg := builtinCertificateBackground(tmpl)

	html, err := buildCertificateHTML(layout, vals, bg, nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML: %v", err)
	}

	renderer := newGotenbergRenderer(url, &http.Client{Timeout: 30 * time.Second})
	pdf, err := renderer.RenderHTML(context.Background(), html)
	if err != nil {
		t.Fatalf("RenderHTML against %s: %v", url, err)
	}

	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (no %%PDF- header); first bytes: %q", pdf[:min(16, len(pdf))])
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatalf("PDF missing %%%%EOF trailer — likely truncated (%d bytes)", len(pdf))
	}
	if len(pdf) < 1024 {
		t.Fatalf("PDF suspiciously small (%d bytes) — expected a real rendered certificate", len(pdf))
	}

	if out := os.Getenv("CERT_SAMPLE_OUT"); out != "" {
		if err := os.WriteFile(out, pdf, 0o644); err != nil {
			t.Fatalf("write sample PDF to %s: %v", out, err)
		}
		t.Logf("wrote %d-byte sample certificate to %s for visual inspection", len(pdf), out)
	}

	// Byte checks above cannot tell a correct certificate from a blank, rotated,
	// or upside-down one — exactly the bug class that shipped before (memory:
	// pdf-layout-needs-visual-verification). Assert on rendered pixels instead.
	// renderToPNG also fails if a second page appears. It skips when pdftoppm is
	// absent, which for this gate would mean silently checking nothing — so
	// require it here rather than degrading (CI installs poppler-utils).
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Fatal("pdftoppm not installed: the render gate cannot verify layout without it")
	}
	img := renderToPNG(t, pdf)

	bounds := img.Bounds()
	if bounds.Dx() <= bounds.Dy() {
		t.Errorf("expected A4 landscape, got %dx%d px", bounds.Dx(), bounds.Dy())
	}

	// The classic background's navy band runs along the top edge; a blank or
	// background-less render leaves this area white.
	r, g, b := avgColorAt(img, certificatePageWidthMm, certificatePageHeightMm, 148, 8)
	if r > 200 && g > 200 && b > 200 {
		t.Errorf("top band is near-white (%.0f,%.0f,%.0f) — background did not render", r, g, b)
	}

	// The student name must land in its own layout box. Scanning that band for
	// ink catches a field stamped somewhere else entirely (e.g. a Y-axis flip).
	var nameField LayoutField
	for _, f := range layout.Fields {
		if f.ID == "student_name" {
			nameField = f
		}
	}
	if nameField.ID == "" {
		t.Fatal("classic layout has no student_name field")
	}
	nameInk := regionMinBrightness(img, certificatePageWidthMm, certificatePageHeightMm,
		nameField.XMm, nameField.YMm, nameField.XMm+nameField.WMm, nameField.YMm+nominalLineHeightMm(nameField.SizePt))
	if nameInk > 600 {
		t.Errorf("no glyph ink in the student_name box (darkest pixel %.0f/765) — the field did not render where the layout puts it", nameInk)
	}
}

// TestCertificateRender_DraggedFieldLandsWhereDragged is the pixel-level
// successor to the old renderer-era drag test (a permanently-skipped placeholder
// until now). The default layout cannot carry this check: its student_name sits
// near the middle of the page, so a mirrored Y axis would land it close enough
// to its own box to pass. A field dragged to the lower-left discriminates —
// under mirroring its ink appears near the top instead.
func TestCertificateRender_DraggedFieldLandsWhereDragged(t *testing.T) {
	url := os.Getenv("GOTENBERG_URL")
	if url == "" {
		url = startGotenberg(t)
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Fatal("pdftoppm not installed: the render gate cannot verify layout without it")
	}

	const (
		draggedXMm = 20
		draggedYMm = 175
		draggedWMm = 90
		sizePt     = 20
	)
	layout := Layout{
		Page:       Page{WidthMm: certificatePageWidthMm, HeightMm: certificatePageHeightMm},
		Background: Background{Kind: "builtin", Ref: "classic"},
		Fields: []LayoutField{{
			ID: "student_name", XMm: draggedXMm, YMm: draggedYMm, WMm: draggedWMm,
			Align: "left", Font: "public_sans", Weight: "bold", SizePt: sizePt,
			Color: "#000000", Visible: true,
		}},
	}
	vals := certificateFieldValues("Ujian", "Budi Santoso", "1 Januari 2026", previewCertificateNumber)

	html, err := buildCertificateHTML(layout, vals, builtinCertificateBackground("classic"), nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML: %v", err)
	}
	pdf, err := newGotenbergRenderer(url, &http.Client{Timeout: 30 * time.Second}).
		RenderHTML(context.Background(), html)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	img := renderToPNG(t, pdf)

	boxH := nominalLineHeightMm(sizePt)
	inkAtDrop := regionMinBrightness(img, certificatePageWidthMm, certificatePageHeightMm,
		draggedXMm, draggedYMm, draggedXMm+draggedWMm, draggedYMm+boxH)
	if inkAtDrop > 600 {
		t.Errorf("no ink where the field was dropped (%.0fmm,%.0fmm), darkest pixel %.0f/765", float64(draggedXMm), float64(draggedYMm), inkAtDrop)
	}

	mirroredY := certificatePageHeightMm - draggedYMm - boxH
	inkAtMirror := regionMinBrightness(img, certificatePageWidthMm, certificatePageHeightMm,
		draggedXMm, mirroredY, draggedXMm+draggedWMm, mirroredY+boxH)
	if inkAtMirror < inkAtDrop {
		t.Errorf("more ink at the mirrored position (y=%.0fmm, %.0f) than where the field was dropped (y=%.0fmm, %.0f) — the Y axis is inverted",
			mirroredY, inkAtMirror, float64(draggedYMm), inkAtDrop)
	}
}
