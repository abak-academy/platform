package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"image/png"
	"strconv"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// ErrNoTrackingNumber gates GetShippingLabel (FR-D-2): an order that hasn't
// been shipped yet has no waybill to put a Code128 barcode of, so the
// request is refused before any HTML is built or Gotenberg is called.
var ErrNoTrackingNumber = errors.New("order has no tracking number yet")

const (
	labelPageWidthMm  = 100.0
	labelPageHeightMm = 150.0

	labelBarcodeWidthPx  = 600
	labelBarcodeHeightPx = 150

	labelRefBarcodeWidthPx  = 520
	labelRefBarcodeHeightPx = 90

	// Found by rendering: past roughly this many lines the slip spills onto a
	// second page and page one loses its footer entirely, which on a
	// 100x150mm sticker is not a layout blemish but a broken document.
	labelMaxPrintedItems = 5
)

// Laid out after Biteship's own dashboard label — stacked full-width rows,
// two-column blocks for reference/measurements and for the two addresses —
// but carrying our mark and our copy, never theirs.
//
// It remains a packing slip, not the scannable carrier label (FR-D-4). The
// resemblance is exactly why the footer still says so: a document that looks
// like a carrier label invites being stuck on a parcel as one.
var shippingLabelHTMLTemplate = template.Must(template.New("shipping_label").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>{{.StyleBlock}}</style>
</head>
<body>
<div class="slip">

<div class="row row-head">
<div class="head-courier">{{.Courier}}</div>
<div class="head-brand">
<img class="head-logo" src="{{.LogoDataURI}}" alt="">
<div class="head-name">{{.SenderName}}</div>
</div>
</div>

<div class="row row-waybill">
<img class="waybill-barcode" src="data:image/png;base64,{{.BarcodeBase64}}" alt="">
<div class="waybill-text">Nomor Resi - {{.Waybill}}</div>
</div>

<div class="row row-service">
<div>Ongkos Kirim: {{.ShippingCostText}}</div>
<div>Kurir - {{.Courier}}. Jenis Layanan - {{.Service}}</div>
</div>

<div class="row row-split">
<div class="cell cell-ref">
<div class="cell-title">Nomor Pesanan</div>
<img class="ref-barcode" src="data:image/png;base64,{{.RefBarcodeBase64}}" alt="">
<div class="ref-text">{{.OrderReference}}</div>
</div>
<div class="cell cell-measure">
<div class="measure-line"><span>Jumlah:</span> {{.TotalQty}} Pcs</div>
<div class="measure-line"><span>Berat:</span> {{.TotalWeightText}}</div>
</div>
</div>

<div class="row row-split">
<div class="cell">
<div class="cell-title">Alamat Penerima:</div>
<div class="addr-name">{{.RecipientName}}</div>
<div class="addr-line">{{.RecipientPhone}}</div>
<div class="addr-line">{{.RecipientAddress}}</div>
{{if .RecipientPostal}}<div class="addr-line">{{.RecipientPostal}}</div>{{end}}
</div>
<div class="cell">
<div class="cell-title">Alamat Pengirim:</div>
<div class="addr-name">{{.SenderName}}</div>
<div class="addr-line">{{.SenderPhone}}</div>
<div class="addr-line">{{.SenderAddress}}</div>
{{if .SenderPostal}}<div class="addr-line">{{.SenderPostal}}</div>{{end}}
</div>
</div>

<div class="row row-items">
<div class="cell-title">Jenis Barang</div>
<ul class="item-list">
{{range .Items}}<li><span class="item-qty">{{.Qty}}x</span> {{.Name}}</li>
{{end}}</ul>
{{if .ItemsOmitted}}<div class="item-more">+{{.ItemsOmitted}} barang lainnya — lihat detail pesanan</div>{{end}}
</div>

<div class="row row-foot">
Packing slip {{.SenderName}} — bukan label resmi kurir.
{{if .SenderEmail}}<br>{{.SenderEmail}}{{end}}
</div>

</div>
</body>
</html>`))

// shippingLabelItem is one printable line of the "Jenis Barang" block.
type shippingLabelItem struct {
	Qty  int
	Name string
}

type shippingLabelHTMLData struct {
	StyleBlock template.CSS

	LogoDataURI template.URL

	Waybill          string
	BarcodeBase64    string
	OrderReference   string
	RefBarcodeBase64 string

	Courier          string
	Service          string
	ShippingCostText string
	TotalQty         int
	TotalWeightText  string
	Items            []shippingLabelItem
	ItemsOmitted     int

	SenderName    string
	SenderPhone   string
	SenderAddress string
	SenderPostal  string
	SenderEmail   string

	RecipientName    string
	RecipientPhone   string
	RecipientAddress string
	RecipientPostal  string
}

// buildShippingLabelHTML renders a self-contained packing-slip document
// (FR-D-1): a Code128 barcode of waybill embedded as a base64 PNG data URI
// (FR-D-3, the same embedding pattern card_logo.go's callers use for images),
// plain waybill text, and sender/recipient blocks. The barcode is generated
// in-process, so nothing here ever makes Gotenberg reach out to the network.
func buildShippingLabelHTML(data shippingLabelHTMLData) ([]byte, error) {
	barcodeBase64, err := shippingLabelBarcodeBase64(data.Waybill, labelBarcodeWidthPx, labelBarcodeHeightPx)
	if err != nil {
		return nil, fmt.Errorf("build shipping label barcode: %w", err)
	}
	refBarcodeBase64, err := shippingLabelBarcodeBase64(data.OrderReference, labelRefBarcodeWidthPx, labelRefBarcodeHeightPx)
	if err != nil {
		return nil, fmt.Errorf("build shipping label reference barcode: %w", err)
	}

	data.StyleBlock = shippingLabelStyleBlock()
	data.BarcodeBase64 = barcodeBase64
	data.RefBarcodeBase64 = refBarcodeBase64
	// The canonical Abak mark, generated from web/components/brand/AbakLogo.tsx
	// and shared with the exam card rather than copied — this repo has already
	// had the mark drift across hand-maintained duplicates.
	data.LogoDataURI = template.URL(svgDataURI(examCardLogoFallback))

	var buf bytes.Buffer
	if err := shippingLabelHTMLTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute shipping label html template: %w", err)
	}
	return buf.Bytes(), nil
}

// shippingLabelBarcodeBase64 encodes waybill as a Code128 barcode PNG and
// returns its base64 payload — the "data:image/png;base64," prefix is a
// static literal in shippingLabelHTMLTemplate (the card_html.go pattern:
// html/template's contextual auto-escaper only lets a data: URI through a
// src attribute when the scheme prefix is static template text, not part of
// the interpolated value).
func shippingLabelBarcodeBase64(value string, widthPx, heightPx int) (string, error) {
	bc, err := code128.Encode(value)
	if err != nil {
		return "", fmt.Errorf("code128 encode: %w", err)
	}
	scaled, err := barcode.Scale(bc, widthPx, heightPx)
	if err != nil {
		return "", fmt.Errorf("scale barcode: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", fmt.Errorf("encode barcode png: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func shippingLabelStyleBlock() template.CSS {
	var b bytes.Buffer
	fmt.Fprintf(&b, "@page{size:%.0fmm %.0fmm;margin:0;}", labelPageWidthMm, labelPageHeightMm)
	b.WriteString("*{margin:0;padding:0;box-sizing:border-box;}")
	fmt.Fprintf(&b, "html,body{width:%.0fmm;height:%.0fmm;font-family:sans-serif;color:#111;}", labelPageWidthMm, labelPageHeightMm)

	// A flex column whose item block absorbs the slack: the old layout stacked
	// four fixed blocks and left the bottom ~40% of the page blank.
	// overflow:hidden is the hard guarantee that this stays one sticker:
	// whatever else changes, nothing can push the footer onto a page two.
	b.WriteString(".slip{display:flex;flex-direction:column;width:100%;height:100%;overflow:hidden;border:0.8pt solid #111;}")
	b.WriteString(".row{border-bottom:0.8pt solid #111;padding:2mm 3mm;}")
	b.WriteString(".row:last-child{border-bottom:none;}")
	b.WriteString(".row-split{display:flex;padding:0;}")
	b.WriteString(".row-split>.cell{flex:1;padding:2mm 3mm;min-width:0;}")
	b.WriteString(".row-split>.cell:first-child{border-right:0.8pt solid #111;}")

	b.WriteString(".row-head{display:flex;align-items:center;gap:3mm;}")
	b.WriteString(".head-courier{flex:0 0 22mm;font-size:9pt;font-weight:bold;letter-spacing:0.3pt;}")
	b.WriteString(".head-brand{flex:1;display:flex;align-items:center;justify-content:center;gap:2mm;}")
	b.WriteString(".head-logo{width:9mm;height:9mm;}")
	b.WriteString(".head-name{font-size:14pt;font-weight:bold;letter-spacing:-0.2pt;}")

	b.WriteString(".row-waybill{text-align:center;}")
	b.WriteString(".waybill-barcode{width:78mm;height:16mm;}")
	b.WriteString(".waybill-text{margin-top:1mm;font-size:11pt;font-weight:bold;letter-spacing:0.5pt;}")

	b.WriteString(".row-service{text-align:center;font-size:8.5pt;line-height:1.5;}")

	b.WriteString(".cell-title{font-size:7.5pt;color:#444;}")
	b.WriteString(".cell-ref{text-align:left;}")
	b.WriteString(".ref-barcode{width:100%;height:9mm;margin-top:1mm;}")
	// The order UUID is long; letting it wrap is what keeps it inside its half.
	b.WriteString(".ref-text{font-size:6.5pt;word-break:break-all;line-height:1.3;}")
	b.WriteString(".cell-measure{display:flex;flex-direction:column;justify-content:center;gap:2mm;}")
	b.WriteString(".measure-line{font-size:9.5pt;}")
	b.WriteString(".measure-line span{color:#444;}")

	b.WriteString(".addr-name{font-size:9.5pt;font-weight:bold;margin-top:0.8mm;}")
	b.WriteString(".addr-line{font-size:8pt;line-height:1.35;}")

	b.WriteString(".row-items{flex:1;}")
	b.WriteString(".item-list{list-style:none;margin-top:1mm;}")
	// One line per item, ellipsised: a wrapping name made the row height
	// unpredictable, which is how five capped items still spilled onto a
	// second page. The ellipsis is visible, so nothing is cut silently.
	b.WriteString(".item-list li{font-size:8.5pt;line-height:1.4;margin-bottom:0.8mm;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}")
	b.WriteString(".item-qty{font-weight:bold;}")
	b.WriteString(".item-more{font-size:7.5pt;color:#444;margin-top:1mm;font-style:italic;}")

	b.WriteString(".row-foot{text-align:center;font-size:7pt;color:#444;line-height:1.4;}")
	return template.CSS(b.String())
}

// formatRupiahID renders an amount the way the rest of the product shows it —
// "Rp 9.000", dots as the thousands separator, no decimals.
func formatRupiahID(v float64) string {
	n := int64(v)
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	digits := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "Rp " + sign + b.String()
}

// postalUnlessAlreadyInAddress drops the standalone postal line when the
// address string already carries it. The live slip printed "…Banten 15310"
// and then "15310" again underneath, because app_address happens to end with
// the postal code while a checkout address does not — the template alone
// cannot tell the two apart.
func postalUnlessAlreadyInAddress(address, postal string) string {
	if postal != "" && strings.Contains(address, postal) {
		return ""
	}
	return postal
}

// formatWeightID renders grams as kilograms with a comma decimal, dropping a
// trailing ",0" so a round parcel reads "1 kg" rather than "1,0 kg".
func formatWeightID(grams int) string {
	s := strconv.FormatFloat(float64(grams)/1000, 'f', 1, 64)
	s = strings.TrimSuffix(s, ".0")
	return strings.ReplaceAll(s, ".", ",") + " kg"
}

// renderShippingLabel is GetShippingLabel's testable core: given an
// already-fetched order and system config, it refuses before any render if
// the order carries no tracking number (FR-D-2), otherwise builds the
// packing slip and renders it through the injected pdfGenerator.
//
// Split out because GetShippingLabel's own storeRepo dependency is a
// concrete *repository.Repository (not an interface), which forces
// DB-integration tests; this core needs neither DB nor Gotenberg (mirrors
// exam_test.go's shimExamCardService/fakeCardRenderer split).
func renderShippingLabel(ctx context.Context, renderer pdfGenerator, order model.Order, cfg map[string]string) ([]byte, error) {
	if order.TrackingNumber == "" {
		return nil, ErrNoTrackingNumber
	}

	dest, err := parseShipmentDestination(order.ShippingAddress)
	if err != nil {
		return nil, fmt.Errorf("shipping label: %w", err)
	}

	// shipmentItemsFromOrder, not a fresh filter: it is already the physical-
	// vs-digital rule the booking path uses, and this repo is on record with
	// four divergent copies of that definition in the frontend alone.
	var items []shippingLabelItem
	totalQty, totalWeightGrams := 0, 0
	for _, it := range shipmentItemsFromOrder(order.Items) {
		items = append(items, shippingLabelItem{Qty: it.Quantity, Name: it.Name})
		totalQty += it.Quantity
		totalWeightGrams += it.WeightGrams * it.Quantity
	}

	// Totals above are computed over every physical item, so the header's
	// Jumlah/Berat stay truthful even when the printed list is cut short.
	itemsOmitted := 0
	if len(items) > labelMaxPrintedItems {
		itemsOmitted = len(items) - labelMaxPrintedItems
		items = items[:labelMaxPrintedItems]
	}

	html, err := buildShippingLabelHTML(shippingLabelHTMLData{
		Waybill:        order.TrackingNumber,
		OrderReference: order.ID.String(),

		Courier:          order.SelectedCourier,
		Service:          order.SelectedService,
		ShippingCostText: formatRupiahID(order.ShippingCost),
		TotalQty:         totalQty,
		TotalWeightText:  formatWeightID(totalWeightGrams),
		Items:            items,
		ItemsOmitted:     itemsOmitted,

		SenderName:    cfg["app_name"],
		SenderPhone:   cfg["app_contact_phone"],
		SenderAddress: cfg["app_address"],
		SenderPostal:  postalUnlessAlreadyInAddress(cfg["app_address"], cfg["app_kode_pos"]),
		SenderEmail:   cfg["app_contact_email"],

		RecipientName:    dest.Penerima,
		RecipientPhone:   dest.Telepon,
		RecipientAddress: dest.Alamat,
		RecipientPostal:  postalUnlessAlreadyInAddress(dest.Alamat, dest.KodePos),
	})
	if err != nil {
		return nil, err
	}
	return renderer.RenderHTML(ctx, html)
}

// GetShippingLabel prints an order's packing slip (FR-D-1..D-4): sender
// details come from system_config, recipient details from the order's own
// shipping address, and the waybill is whatever tracking_number ship time
// stamped (Biteship or manual).
func (s *Service) GetShippingLabel(ctx context.Context, orderID string) ([]byte, error) {
	id, err := parseUUID(orderID)
	if err != nil {
		return nil, err
	}
	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.ID == uuid.Nil {
		return nil, ErrOrderNotFound
	}

	cfg, err := s.GetSystemConfig(ctx)
	if err != nil {
		return nil, err
	}

	return renderShippingLabel(ctx, s.renderer, order, cfg)
}
