package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	// The full order with deliberately long addresses and a long product
	// name: overflow and wrapping are precisely what a unit test cannot see,
	// so the document under visual inspection has to be the awkward one, not
	// the tidy one.
	order := labelTestOrderFull("JNE1234567890")
	addr, _ := json.Marshal(map[string]string{
		"penerima": "Nur Aisyah Rahmawati Puspitaningrum",
		"telepon":  "081200000000",
		"alamat":   "Perumahan Griya Asri Permai Blok C-12 No. 45, RT 007 RW 003, Kelurahan Sukamaju Baru, Kecamatan Tapos, Depok, Jawa Barat",
		"kode_pos": "16455",
	})
	order.ShippingAddress = addr

	cfg := labelTestSenderConfig()
	cfg["app_address"] = "Komplek Perkantoran Sentra Niaga Blok B No. 8, Jl. Raya Serpong KM 7, Kelurahan Pakulonan, Kecamatan Serpong Utara, Tangerang Selatan, Banten"
	cfg["app_contact_email"] = "halo@abakacademy.id"

	pdf, err := renderShippingLabel(context.Background(), renderer, order, cfg)
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

// labelTestOrderFull is labelTestOrder plus everything the redesigned slip
// prints: courier, service, ongkir, and a mix of a physical and a digital
// item. Modelled on Biteship's own label, which is what this document was
// asked to look like.
func labelTestOrderFull(trackingNumber string) model.Order {
	order := labelTestOrder(trackingNumber)
	order.SelectedCourier = "TIKI"
	order.SelectedService = "Reguler"
	order.ShippingCost = 9000
	order.Items = []model.OrderItem{
		{Name: "Kumpulan Soal Fisika SMA", ProductType: "book", Qty: 2, WeightGrams: 500},
		{Name: "Try Out Nasional 2026", ProductType: "exam", Qty: 1},
	}
	return order
}

// TestRenderShippingLabel_CarriesOrderContentsAndOwnBranding covers the
// redesign: the slip must carry what a packer actually needs — items, weight,
// the order reference, courier and ongkir — and must be branded as ours, with
// no Biteship platform copy left anywhere on it.
func TestRenderShippingLabel_CarriesOrderContentsAndOwnBranding(t *testing.T) {
	fake := &fakeLabelRenderer{pdf: []byte("%PDF-fake-label")}
	// Waybill deliberately carries no courier name, so asserting on "TIKI"
	// proves the courier field renders rather than matching the resi.
	order := labelTestOrderFull("0099887766123")

	if _, err := renderShippingLabel(context.Background(), fake, order, labelTestSenderConfig()); err != nil {
		t.Fatalf("renderShippingLabel: %v", err)
	}
	html := string(fake.lastHTML)

	for _, want := range []string{
		"Kumpulan Soal Fisika SMA", // item name
		"2x",                       // quantity
		"1 kg",                     // 2 x 500 g
		"TIKI",                     // courier
		"Reguler",                  // service
		"Rp 9.000",                 // ongkir, Indonesian thousands separator
		order.ID.String(),          // order reference
		"Akademi Bimbel Test",      // our own brand, from app_name
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered html is missing %q", want)
		}
	}

	if strings.Contains(html, "Biteship") || strings.Contains(html, "biteship") {
		t.Error("no Biteship platform copy may remain on our own packing slip")
	}
	// Matched without the "+xml" part on purpose: html/template escapes the
	// plus to &#43; inside the attribute. The browser decodes it back before
	// loading, so the image is fine — but the literal bytes are not there.
	// That the mark actually renders is proven by the Gotenberg test below,
	// not here.
	if !strings.Contains(html, `class="head-logo" src="data:image/svg`) {
		t.Error("the Abak mark is not embedded as an inline SVG data URI")
	}
	if got := strings.Count(html, "data:image/png;base64,"); got != 2 {
		t.Errorf("want 2 barcodes (waybill + order reference), got %d", got)
	}
}

// TestRenderShippingLabel_ListsPhysicalItemsOnly keeps digital products off a
// document whose only job is to tell a packer what goes in the box.
func TestRenderShippingLabel_ListsPhysicalItemsOnly(t *testing.T) {
	fake := &fakeLabelRenderer{pdf: []byte("%PDF-fake-label")}
	// Waybill deliberately carries no courier name, so asserting on "TIKI"
	// proves the courier field renders rather than matching the resi.
	order := labelTestOrderFull("0099887766123")

	if _, err := renderShippingLabel(context.Background(), fake, order, labelTestSenderConfig()); err != nil {
		t.Fatalf("renderShippingLabel: %v", err)
	}

	if strings.Contains(string(fake.lastHTML), "Try Out Nasional 2026") {
		t.Error("a digital item must not appear on the packing slip")
	}
}

// TestRenderShippingLabel_CapsItemListToStayOnOnePage guards a defect found by
// rendering, not by reading: 12 items pushed the document onto a second page
// and left page one with no footer at all — on a 100x150mm sticker. The list
// is capped, and what is dropped is stated rather than silently truncated.
func TestRenderShippingLabel_CapsItemListToStayOnOnePage(t *testing.T) {
	fake := &fakeLabelRenderer{pdf: []byte("%PDF-fake-label")}
	order := labelTestOrderFull("0099887766123")
	order.Items = nil
	for i := 1; i <= 12; i++ {
		order.Items = append(order.Items, model.OrderItem{
			Name: fmt.Sprintf("Buku Latihan Nomor %d", i), ProductType: "book", Qty: 1, WeightGrams: 100,
		})
	}

	if _, err := renderShippingLabel(context.Background(), fake, order, labelTestSenderConfig()); err != nil {
		t.Fatalf("renderShippingLabel: %v", err)
	}
	html := string(fake.lastHTML)

	if got := strings.Count(html, "<li>"); got != labelMaxPrintedItems {
		t.Errorf("want %d printed item lines, got %d", labelMaxPrintedItems, got)
	}
	if !strings.Contains(html, "7 barang lainnya") {
		t.Error("the dropped items must be stated on the slip, not silently omitted")
	}
	if strings.Contains(html, "Buku Latihan Nomor 12") {
		t.Error("item 12 is past the cap and must not be printed")
	}
}

// TestRenderShippingLabel_DoesNotRepeatPostalAlreadyInAddress covers the
// duplicate seen on the live slip: app_address already ends with the postal
// code, and the template printed app_kode_pos underneath it as well.
func TestRenderShippingLabel_DoesNotRepeatPostalAlreadyInAddress(t *testing.T) {
	fake := &fakeLabelRenderer{pdf: []byte("%PDF-fake-label")}
	order := labelTestOrderFull("0099887766123")

	cfg := labelTestSenderConfig()
	cfg["app_address"] = "Jl. Kantor No. 2, Serpong, Tangerang Selatan, Banten 54321"
	cfg["app_kode_pos"] = "54321"

	if _, err := renderShippingLabel(context.Background(), fake, order, cfg); err != nil {
		t.Fatalf("renderShippingLabel: %v", err)
	}

	if got := strings.Count(string(fake.lastHTML), "54321"); got != 1 {
		t.Errorf("want the sender postal code printed exactly once, got %d", got)
	}
}
