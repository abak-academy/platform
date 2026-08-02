package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// fakeLabelRenderer stands in for the Gotenberg-backed pdfGenerator (mirrors
// exam_test.go's fakeCardRenderer) and additionally captures the HTML it was
// asked to render, so tests can assert on the document's content without a
// live Gotenberg.
type fakeLabelRenderer struct {
	calls    int
	lastHTML []byte
	pdf      []byte
	err      error
}

func (f *fakeLabelRenderer) RenderHTML(_ context.Context, html []byte) ([]byte, error) {
	f.calls++
	f.lastHTML = html
	if f.err != nil {
		return nil, f.err
	}
	return f.pdf, nil
}

func labelTestOrder(trackingNumber string) model.Order {
	addr, _ := json.Marshal(map[string]string{
		"penerima": "Budi Test",
		"telepon":  "081200000000",
		"alamat":   "Jl. Contoh No. 1",
		"kode_pos": "12345",
	})
	return model.Order{
		ID:              uuid.New(),
		TrackingNumber:  trackingNumber,
		ShippingAddress: addr,
	}
}

func labelTestSenderConfig() map[string]string {
	return map[string]string{
		"app_name":          "Akademi Bimbel Test",
		"app_contact_phone": "081299999999",
		"app_address":       "Jl. Kantor No. 2",
		"app_kode_pos":      "54321",
	}
}

// TestRenderShippingLabel_RefusesWhenNoTrackingNumber proves FR-D-2: an
// order with no tracking_number is refused before any HTML is built or the
// renderer is ever called.
func TestRenderShippingLabel_RefusesWhenNoTrackingNumber(t *testing.T) {
	fake := &fakeLabelRenderer{}
	order := labelTestOrder("")

	_, err := renderShippingLabel(context.Background(), fake, order, labelTestSenderConfig())
	if !errors.Is(err, ErrNoTrackingNumber) {
		t.Fatalf("want ErrNoTrackingNumber, got %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("renderer must not be called when tracking number is empty, got %d calls", fake.calls)
	}
}

// TestRenderShippingLabel_CarriesWaybillBarcodeNoExternalRefs proves FR-D-1
// and FR-D-3: the rendered document carries the waybill as plain text, the
// Code128 barcode as a base64 data URI, and — since Gotenberg renders this
// HTML in a sandbox where any http(s) reference silently fails to load —
// contains no reference to an external host anywhere in the output.
func TestRenderShippingLabel_CarriesWaybillBarcodeNoExternalRefs(t *testing.T) {
	fake := &fakeLabelRenderer{pdf: []byte("%PDF-fake-label")}
	order := labelTestOrder("JNE1234567890")

	pdf, err := renderShippingLabel(context.Background(), fake, order, labelTestSenderConfig())
	if err != nil {
		t.Fatalf("renderShippingLabel: %v", err)
	}
	if string(pdf) != string(fake.pdf) {
		t.Errorf("returned bytes don't match the renderer's output")
	}
	if fake.calls != 1 {
		t.Fatalf("want exactly 1 render call, got %d", fake.calls)
	}

	html := string(fake.lastHTML)
	if !strings.Contains(html, "JNE1234567890") {
		t.Errorf("waybill text not found in rendered html")
	}
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Errorf("barcode data URI not found in rendered html")
	}
	if !strings.Contains(html, "Budi Test") || !strings.Contains(html, "Jl. Contoh No. 1") {
		t.Errorf("recipient details not found in rendered html")
	}
	if !strings.Contains(html, "Akademi Bimbel Test") || !strings.Contains(html, "Jl. Kantor No. 2") {
		t.Errorf("sender details not found in rendered html")
	}
	if !strings.Contains(html, "packing slip") && !strings.Contains(strings.ToLower(html), "packing slip") {
		t.Errorf("expected the document to identify itself as a packing slip (FR-D-4)")
	}
	if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Errorf("rendered html contains an external host reference, which Gotenberg's sandbox cannot fetch")
	}
}

// TestShippingLabelRendersThroughGotenberg is the visual-verification gate
// for this task (memory: pdf-layout-needs-visual-verification — unit tests
// cannot see layout). Skipped unless GOTENBERG_URL is set:
//
//	GOTENBERG_URL=http://localhost:3001 \
//	  LABEL_PDF_OUT=/tmp/label-proof.pdf \
//	  go test ./internal/service/ -run TestShippingLabelRendersThroughGotenberg -v
//
// Byte assertions are not the point — open LABEL_PDF_OUT and look at it.
func TestShippingLabelRendersThroughGotenberg(t *testing.T) {
	url := os.Getenv("GOTENBERG_URL")
	if url == "" {
		t.Skip("GOTENBERG_URL not set; this test needs a real Gotenberg sidecar")
	}

	renderer := newGotenbergPDFGenerator(url, http.DefaultClient)
	order := labelTestOrder("JNE1234567890")

	pdf, err := renderShippingLabel(context.Background(), renderer, order, labelTestSenderConfig())
	if err != nil {
		t.Fatalf("renderShippingLabel: %v", err)
	}
	if len(pdf) < 512 {
		t.Fatalf("suspiciously small PDF: %d bytes", len(pdf))
	}
	if got := string(pdf[:4]); got != "%PDF" {
		t.Fatalf("response is not a PDF, starts with %q", got)
	}

	if out := os.Getenv("LABEL_PDF_OUT"); out != "" {
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", out, err)
		}
		if err := os.WriteFile(out, pdf, 0o644); err != nil {
			t.Fatalf("write %s: %v", out, err)
		}
		t.Logf("wrote %s (%d bytes) — OPEN IT AND LOOK", out, len(pdf))
	}
}
