package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
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
		"Akademi Bimbel Test",      // sender name, from app_name
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
	if got := strings.Count(html, "data:image/png;base64,"); got != 1 {
		t.Errorf("want only the waybill barcode, got %d", got)
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

type fakeShippingLabelStore struct {
	orders map[uuid.UUID]model.Order
	config map[string]string
	reads  []uuid.UUID
	err    error
}

func (f *fakeShippingLabelStore) GetOrderByID(_ context.Context, id uuid.UUID) (model.Order, error) {
	f.reads = append(f.reads, id)
	return f.orders[id], f.err
}

func (f *fakeShippingLabelStore) ListSystemConfig(context.Context) ([]repository.SystemConfigRow, error) {
	var rows []repository.SystemConfigRow
	config := f.config
	if config == nil {
		config = labelTestSenderConfig()
	}
	for k, v := range config {
		rows = append(rows, repository.SystemConfigRow{Key: k, Value: v})
	}
	return rows, f.err
}

func TestGetShippingLabels(t *testing.T) {
	for _, size := range []string{"100x150", "roll80"} {
		t.Run(size, func(t *testing.T) {
			store := &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{}}
			fake := &fakeLabelRenderer{pdf: []byte("PDF")}
			svc := &Service{labelStore: store, renderer: fake}
			var ids []string
			for i := 0; i < 3; i++ {
				o := labelTestOrderFull(fmt.Sprintf("WAYBILL%d", i))
				store.orders[o.ID] = o
				ids = append(ids, o.ID.String())
			}
			pdf, err := svc.GetShippingLabels(context.Background(), ids, size)
			if err != nil {
				t.Fatal(err)
			}
			if string(pdf) != "PDF" || fake.calls != 1 {
				t.Fatalf("pdf=%q calls=%d", pdf, fake.calls)
			}
			html := string(fake.lastHTML)
			if strings.Count(html, `class="slip"`) != 3 {
				t.Fatalf("want three slips")
			}
			last := -1
			for i, id := range ids {
				pos := strings.Index(html, `class="waybill-text">WAYBILL`+fmt.Sprint(i))
				if pos <= last || !strings.Contains(html, id) {
					t.Fatalf("missing or unordered slip %s", id)
				}
				last = pos
			}
			if size == "roll80" && (!strings.Contains(html, "width:78mm") || !strings.Contains(html, "getBoundingClientRect") || !strings.Contains(html, "size:80mm")) {
				t.Fatal("roll80 measurement/layout missing")
			}
		})
	}
}

func TestGetShippingLabelsFailClosed(t *testing.T) {
	for _, status := range []string{"courier_not_found", "courierNotFound", "rejected", "cancelled", "return_in_transit", "returnInTransit", "returned", "disposed"} {
		t.Run(status, func(t *testing.T) {
			valid := labelTestOrderFull("VALID123")
			empty := labelTestOrder("")
			failed := labelTestOrderFull("FAILED123")
			failed.ShipmentStatus = &status
			unknown := uuid.New()
			store := &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{valid.ID: valid, empty.ID: empty, failed.ID: failed}}
			fake := &fakeLabelRenderer{}
			svc := &Service{labelStore: store, renderer: fake}
			_, err := svc.GetShippingLabels(context.Background(), []string{valid.ID.String(), empty.ID.String(), failed.ID.String(), unknown.String()}, "100x150")
			var conflict *LabelsNotPrintableError
			if !errors.Is(err, ErrLabelsNotPrintable) || !errors.As(err, &conflict) {
				t.Fatalf("want conflict, got %v", err)
			}
			want := []LabelNotPrintable{{empty.ID.String(), "no_tracking_number"}, {failed.ID.String(), "shipment_failed"}, {unknown.String(), "not_found"}}
			if fmt.Sprint(conflict.Details) != fmt.Sprint(want) {
				t.Fatalf("details=%v want=%v", conflict.Details, want)
			}
			if fake.calls != 0 || len(store.reads) != 4 {
				t.Fatalf("renders=%d reads=%d", fake.calls, len(store.reads))
			}
		})
	}
}

func TestGetShippingLabelsValidation(t *testing.T) {
	var ids []string
	store := &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{}}
	for i := 0; i < 101; i++ {
		o := labelTestOrderFull(fmt.Sprintf("WB%d", i))
		store.orders[o.ID] = o
		ids = append(ids, o.ID.String())
	}
	tests := []struct {
		name string
		ids  []string
		size string
	}{{"empty", nil, "100x150"}, {"too many", ids, "100x150"}, {"duplicate", []string{ids[0], ids[0]}, "100x150"}, {"equivalent uuid", []string{ids[0], strings.ToUpper(ids[0])}, "100x150"}, {"malformed", []string{"oops"}, "100x150"}, {"missing size", ids[:1], ""}, {"unknown size", ids[:1], "a4"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLabelRenderer{}
			svc := &Service{labelStore: store, renderer: fake}
			store.reads = nil
			_, err := svc.GetShippingLabels(context.Background(), tt.ids, tt.size)
			if !errors.Is(err, ErrInvalidLabelRequest) {
				t.Fatalf("want invalid request got %v", err)
			}
			if fake.calls != 0 || len(store.reads) != 0 {
				t.Fatal("invalid input reached dependencies")
			}
		})
	}
	fake := &fakeLabelRenderer{}
	svc := &Service{labelStore: store, renderer: fake}
	if _, err := svc.GetShippingLabels(context.Background(), ids[:100], "100x150"); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 || strings.Count(string(fake.lastHTML), `class="slip"`) != 100 {
		t.Fatal("100 labels not accepted")
	}
}

func TestShippingLabelContentAndSharedRenderer(t *testing.T) {
	o := labelTestOrderFull("0099887766123")
	o.ID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	config := labelTestSenderConfig()
	config["app_contact_email"] = "sender@example.test"
	for _, size := range []string{"100x150", "roll80"} {
		t.Run(size, func(t *testing.T) {
			fake := &fakeLabelRenderer{}
			svc := &Service{labelStore: &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{o.ID: o}, config: config}, renderer: fake}
			if _, err := svc.GetShippingLabelSized(context.Background(), o.ID.String(), size); err != nil {
				t.Fatal(err)
			}
			single := string(fake.lastHTML)
			for _, want := range []string{o.TrackingNumber, o.ID.String(), "TIKI", "Reguler", "Budi Test", "081200000000", "Jl. Contoh No. 1", "12345", "Akademi Bimbel Test", "081299999999", "Jl. Kantor No. 2", "54321", "sender@example.test", "Kumpulan Soal Fisika SMA", "2x", "1 kg", "Rp 9.000", "bukan label resmi kurir.", "Liberation Sans"} {
				if !strings.Contains(single, want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Count(single, "data:image/png;base64,") != 1 {
				t.Fatal("only the waybill barcode must remain")
			}
			previous := -1
			for _, section := range []string{`class="row row-head"`, `class="waybill-text"`, `class="waybill-barcode"`, "Alamat Penerima:", "Alamat Pengirim:", "Nomor Pesanan", "Jumlah:", "Jenis Barang", "Ongkos Kirim:", "Packing slip"} {
				index := strings.Index(single, section)
				if index <= previous {
					t.Fatalf("reading order: %q", section)
				}
				previous = index
			}
			if _, err := svc.GetShippingLabels(context.Background(), []string{o.ID.String()}, size); err != nil {
				t.Fatal(err)
			}
			if single != string(fake.lastHTML) {
				t.Fatal("single and bulk must share the renderer")
			}
			if size == "100x150" {
				if _, err := svc.GetShippingLabelSized(context.Background(), o.ID.String(), ""); err != nil {
					t.Fatal(err)
				}
				if single != string(fake.lastHTML) {
					t.Fatal("absent size must default to 100x150")
				}
			}
		})
	}
}

func TestGetShippingLabelSizes(t *testing.T) {
	o := labelTestOrderFull("WAYBILL123")
	status := "returned"
	o.ShipmentStatus = &status
	store := &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{o.ID: o}}
	for _, size := range []string{"", "100x150", "roll80", "a4"} {
		t.Run(size, func(t *testing.T) {
			fake := &fakeLabelRenderer{}
			svc := &Service{labelStore: store, renderer: fake}
			_, err := svc.GetShippingLabelSized(context.Background(), o.ID.String(), size)
			if size == "a4" {
				if !errors.Is(err, ErrInvalidLabelRequest) || fake.calls != 0 {
					t.Fatalf("invalid size: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if fake.calls != 1 {
				t.Fatal("single failed shipment should still print")
			}
			if size == "roll80" && !strings.Contains(string(fake.lastHTML), "size:80mm") {
				t.Fatal("roll layout missing")
			}
		})
	}
}

type blockingLabelRenderer struct {
	entered chan struct{}
	release chan struct{}
}

func (f *blockingLabelRenderer) RenderHTML(context.Context, []byte) ([]byte, error) {
	f.entered <- struct{}{}
	<-f.release
	return []byte("PDF"), nil
}

func TestGetShippingLabelsSerializesProcesses(t *testing.T) {
	o := labelTestOrderFull("LOCKTEST123")
	renderer := &blockingLabelRenderer{entered: make(chan struct{}, 2), release: make(chan struct{})}
	store1 := &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{o.ID: o}}
	store2 := &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{o.ID: o}}
	first := &Service{labelStore: store1, renderer: renderer}
	second := &Service{labelStore: store2, renderer: renderer}
	done := make(chan error, 2)
	go func() {
		_, err := first.GetShippingLabels(context.Background(), []string{o.ID.String()}, "100x150")
		done <- err
	}()
	select {
	case <-renderer.entered:
	case <-time.After(time.Second):
		t.Fatal("first request did not render")
	}
	started := make(chan struct{})
	go func() {
		close(started)
		_, err := second.GetShippingLabels(context.Background(), []string{o.ID.String()}, "roll80")
		done <- err
	}()
	<-started
	select {
	case <-renderer.entered:
		t.Error("second bulk request rendered concurrently")
	case <-time.After(100 * time.Millisecond):
	}
	close(renderer.release)
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("request did not finish")
		}
	}
}

func TestGetShippingLabelsDependencyFailure(t *testing.T) {
	order := labelTestOrderFull("ERROR123")
	want := errors.New("database unavailable")
	fake := &fakeLabelRenderer{}
	store := &fakeShippingLabelStore{orders: map[uuid.UUID]model.Order{order.ID: order}, err: want}
	svc := &Service{labelStore: store, renderer: fake}
	if _, err := svc.GetShippingLabels(context.Background(), []string{order.ID.String()}, "100x150"); !errors.Is(err, want) || fake.calls != 0 {
		t.Fatalf("err=%v renders=%d", err, fake.calls)
	}
}

func TestShippingLabelBarcodeQuietZone(t *testing.T) {
	for _, value := range []string{"123", "SHORT12345", "LONG-WAYBILL-123456789012345678901234567890"} {
		t.Run(value, func(t *testing.T) {
			encoded, err := shippingLabelBarcodeBase64(value, labelBarcodeWidthPx, labelBarcodeHeightPx)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			first, last, narrow, run := -1, -1, img.Bounds().Dx(), 0
			for x := 0; x < img.Bounds().Dx(); x++ {
				r, _, _, _ := img.At(x, img.Bounds().Dy()/2).RGBA()
				if r == 0 {
					if first < 0 {
						first = x
					}
					last = x
					run++
				} else if run > 0 {
					narrow = min(narrow, run)
					run = 0
				}
			}
			if run > 0 {
				narrow = min(narrow, run)
			}
			if first < 10*narrow || img.Bounds().Dx()-last-1 < 10*narrow {
				t.Fatalf("quiet zones = %d/%d pixels, narrow bar = %d; want >=10 modules", first, img.Bounds().Dx()-last-1, narrow)
			}
		})
	}
}

func TestShippingLabelCourierHeader(t *testing.T) {
	for _, size := range []string{"100x150", "roll80"} {
		for _, code := range []string{"jne", "SICEPAT", "lion", "tiki", "anteraja", "unknown", "", "nil"} {
			t.Run(size+"/"+code, func(t *testing.T) {
				order := labelTestOrderFull("HEADER123")
				if code != "nil" {
					order.CourierCode = &code
				}
				data, err := shippingLabelData(order, labelTestSenderConfig())
				if err != nil {
					t.Fatal(err)
				}
				raw, err := buildShippingLabelsHTML([]shippingLabelHTMLData{data}, size)
				if err != nil {
					t.Fatal(err)
				}
				html := string(raw)
				for _, want := range []string{"hub.abakacademy.id", `class="head-name">abak academy</div>`, "#22315B", "#176A63", "font-size:12pt", "font-size:10pt;line-height:13pt", order.ID.String()} {
					if !strings.Contains(html, want) {
						t.Errorf("missing %q", want)
					}
				}
				hasLogo := strings.Contains(html, `class="courier-logo" src="data:image/`)
				wantLogo := code != "unknown" && code != "" && code != "nil"
				if wantLogo {
					file := strings.ToLower(code) + ".svg"
					mime := "image/svg+xml"
					if code == "tiki" || code == "anteraja" {
						file = code + ".png"
						mime = "image/png"
					}
					_, source, _, ok := runtime.Caller(0)
					if !ok {
						t.Fatal("resolve test source directory")
					}
					asset, err := os.ReadFile(filepath.Join(filepath.Dir(source), "assets", "couriers", file))
					if err != nil {
						t.Fatal(err)
					}
					wantURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(asset)
					if string(data.CourierLogoDataURI) != wantURI {
						t.Error("wrong courier asset or MIME type")
					}
				}
				if hasLogo != wantLogo {
					t.Errorf("courier logo = %v, want %v", hasLogo, wantLogo)
				}
				if !wantLogo && !strings.Contains(html, `class="courier-name">TIKI`) {
					t.Error("missing courier text fallback")
				}
				if strings.Contains(html, "ref-barcode") {
					t.Error("order UUID barcode must be removed")
				}
				if strings.Contains(html, "ZgotmplZ") {
					t.Error("data URI rejected by template")
				}
			})
		}
	}
}

func TestShippingLabelCourierAssetsMatchWeb(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source directory")
	}
	dir := filepath.Dir(source)
	for _, file := range []string{"jne.svg", "sicepat.svg", "lion.svg", "tiki.png", "anteraja.png"} {
		t.Run(file, func(t *testing.T) {
			backend, err := os.ReadFile(filepath.Join(dir, "assets", "couriers", file))
			if err != nil {
				t.Fatal(err)
			}
			web, err := os.ReadFile(filepath.Join(dir, "..", "..", "..", "web", "public", "couriers", file))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(backend, web) {
				t.Fatal("backend courier asset differs from web source")
			}
		})
	}
}
