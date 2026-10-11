package service

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
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

	// Found by rendering: past roughly this many lines the slip spills onto a
	// second page and page one loses its footer entirely, which on a
	// 100x150mm sticker is not a layout blemish but a broken document.
	labelMaxPrintedItems = 5
)

const (
	labelBrandWordmark = "abak academy"
	labelBrandHub      = "hub.abakacademy.id"
)

//go:embed assets/couriers/*
var labelCourierAssets embed.FS

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
{{range .Slips}}<div class="slip">

<div class="row row-head">
<div class="head-courier">
{{if .CourierLogoDataURI}}<img class="courier-logo" src="{{.CourierLogoDataURI}}" alt="{{.Courier}}">{{else}}<div class="courier-name">{{.Courier}}</div>{{end}}
<div class="courier-service">{{.Service}}</div>
</div>
<div class="head-brand">
<img class="head-logo" src="{{.LogoDataURI}}" alt="">
<div class="head-wordmark">
<div class="head-name">{{.BrandWordmark}}</div>
<div class="head-hub">{{.BrandHub}}</div>
</div>
</div>
</div>

<div class="row row-waybill">
<div class="cell-title">Nomor Resi</div>
<div class="waybill-text">{{.Waybill}}</div>
<img class="waybill-barcode" src="data:image/png;base64,{{.BarcodeBase64}}" alt="">
</div>

<div class="row row-recipient">
<div class="cell-title">Alamat Penerima:</div>
<div class="addr-name recipient-name">{{.RecipientName}}</div>
<div class="recipient-phone">{{.RecipientPhone}}</div>
<div class="addr-line">{{.RecipientAddress}}</div>
{{if .RecipientPostal}}<div class="addr-line">{{.RecipientPostal}}</div>{{end}}
</div>

<div class="row row-sender">
<div class="cell-title">Alamat Pengirim:</div>
<div class="addr-name">{{.SenderName}} / {{.SenderPhone}}</div>
<div class="addr-line">{{.SenderAddress}}</div>
{{if .SenderPostal}}<div class="addr-line">{{.SenderPostal}}</div>{{end}}
</div>

<div class="row row-reference">
<div class="cell-title">Nomor Pesanan</div>
<div class="ref-text">{{.OrderReference}}</div>
<div class="measure-line">Jumlah: {{.TotalQty}} Pcs / Berat: {{.TotalWeightText}}</div>
</div>

<div class="row row-items">
<div class="cell-title">Jenis Barang</div>
<ul class="item-list">
{{range .Items}}<li><span class="item-qty">{{.Qty}}x</span> {{.Name}}</li>
{{end}}</ul>
{{if .ItemsOmitted}}<div class="item-more">+{{.ItemsOmitted}} barang lainnya — lihat detail pesanan</div>{{end}}
</div>

<div class="row row-cost">Ongkos Kirim: {{.ShippingCostText}}</div>

<div class="row row-foot">
Packing slip {{.SenderName}} — bukan label resmi kurir.
{{if .SenderEmail}}<br>{{.SenderEmail}}{{end}}
</div>

</div>{{end}}{{if .Roll80}}
<script>
const k = 25.4 / 96;
let css = "";
document.querySelectorAll(".slip").forEach((el, i) => {
  const h = Math.ceil(el.getBoundingClientRect().height * k) + 1;
  css += "@page s" + i + "{size:80mm " + h + "mm;margin:0}";
  el.style.page = "s" + i;
  el.style.height = h + "mm";
});
const style = document.createElement("style");
style.textContent = css;
document.head.append(style);
</script>{{end}}
</body>
</html>`))

// shippingLabelItem is one printable line of the "Jenis Barang" block.
type shippingLabelItem struct {
	Qty  int
	Name string
}

type shippingLabelHTMLData struct {
	LogoDataURI template.URL

	CourierLogoDataURI template.URL
	BrandWordmark      string
	BrandHub           string

	Waybill        string
	BarcodeBase64  string
	OrderReference string

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
	return buildShippingLabelsHTML([]shippingLabelHTMLData{data}, "100x150")
}

func buildShippingLabelsHTML(slips []shippingLabelHTMLData, size string) ([]byte, error) {
	for i, data := range slips {
		barcodeBase64, err := shippingLabelBarcodeBase64(data.Waybill, labelBarcodeWidthPx, labelBarcodeHeightPx)
		if err != nil {
			return nil, fmt.Errorf("build shipping label barcode: %w", err)
		}
		data.BarcodeBase64 = barcodeBase64
		data.BrandWordmark = labelBrandWordmark
		data.BrandHub = labelBrandHub
		// The canonical Abak mark, generated from web/components/brand/AbakLogo.tsx
		// and shared with the exam card rather than copied — this repo has already
		// had the mark drift across hand-maintained duplicates.
		data.LogoDataURI = template.URL(svgDataURI(examCardLogoFallback))

		slips[i] = data
	}
	paged := len(slips) > 1 || size == "roll80"
	style := shippingLabelStyleBlock(paged)
	if paged {
		style += ".slip{height:150mm;break-after:page}.slip:last-of-type{break-after:auto}"
	}
	if size == "roll80" {
		style += "html,body{width:80mm}.slip{width:78mm;margin-left:1mm;padding:2mm 2.5mm;height:auto;overflow:visible;display:block}.waybill-barcode{width:70mm}"
	}
	data := struct {
		StyleBlock template.CSS
		Slips      []shippingLabelHTMLData
		Roll80     bool
	}{style, slips, size == "roll80"}
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
	widthPx = widthPx / bc.Bounds().Dx() * bc.Bounds().Dx()
	scaled, err := barcode.Scale(bc, widthPx, heightPx)
	if err != nil {
		return "", fmt.Errorf("scale barcode: %w", err)
	}
	quietPx := 10 * widthPx / bc.Bounds().Dx()
	withQuietZone := image.NewRGBA(image.Rect(0, 0, widthPx+2*quietPx, heightPx))
	draw.Draw(withQuietZone, withQuietZone.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(withQuietZone, scaled.Bounds().Add(image.Pt(quietPx, 0)), scaled, scaled.Bounds().Min, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, withQuietZone); err != nil {
		return "", fmt.Errorf("encode barcode png: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func shippingLabelStyleBlock(paged bool) template.CSS {
	var b bytes.Buffer
	fmt.Fprintf(&b, "@page{size:%.0fmm %.0fmm;margin:0;}", labelPageWidthMm, labelPageHeightMm)
	b.WriteString("*{margin:0;padding:0;box-sizing:border-box;}")
	bodyHeight := fmt.Sprintf("height:%.0fmm;", labelPageHeightMm)
	if paged {
		bodyHeight = ""
	}
	fmt.Fprintf(&b, `html,body{width:%.0fmm;%sfont-family:"Liberation Sans",Arial,Helvetica,sans-serif;color:#111;font-size:8.4pt;line-height:1.2;}`, labelPageWidthMm, bodyHeight)
	// overflow:hidden is the hard guarantee that this stays one sticker:
	// whatever else changes, nothing can push the footer onto a page two.
	b.WriteString(".slip{display:flex;flex-direction:column;justify-content:space-between;width:100%;height:100%;padding:2mm 3mm;overflow:hidden;}")
	b.WriteString(".row{flex-shrink:0;border-bottom:0.4pt solid #888;padding:0.8mm 0;overflow-wrap:anywhere;}")
	b.WriteString(".row:last-child{border-bottom:none;}")
	b.WriteString(".row-head{display:flex;align-items:center;gap:2mm;}")
	b.WriteString(".head-courier{flex:1;min-width:0;max-width:29mm;}.courier-logo{display:block;width:29mm;max-width:100%;height:7mm;object-fit:contain;object-position:left center;filter:brightness(0.65);}.courier-name{font-size:9pt;font-weight:700;}.courier-service{font-size:7.2pt;margin-top:0.6mm;}")
	b.WriteString(".head-brand{margin-left:auto;display:flex;align-items:flex-start;gap:1mm;flex-shrink:0;color:#22315B;}.head-wordmark{text-align:center;}")
	b.WriteString(".head-logo{width:5.8mm;height:5.8mm;flex-shrink:0;filter:brightness(0.65);}")
	b.WriteString(".head-name{font-size:10pt;font-weight:700;white-space:nowrap;}.head-hub{font-size:5.6pt;font-weight:700;color:#176A63;margin-top:0.6mm;white-space:nowrap;}")
	b.WriteString(".cell-title{font-size:7.2pt;font-weight:700;margin-bottom:0.6mm;}")
	b.WriteString(".waybill-text{font-size:11pt;font-weight:700;}")
	b.WriteString(".waybill-barcode{display:block;width:78mm;max-width:100%;height:13mm;margin:1mm auto 0.5mm;}")
	b.WriteString(".addr-name{font-weight:700;font-size:8.2pt;}")
	b.WriteString(".recipient-name{font-size:12pt;}.recipient-phone{font-size:9.5pt;}.row-recipient .addr-line{font-size:10pt;line-height:13pt;}")
	b.WriteString(".addr-line{font-size:7.8pt;line-height:10pt;}")
	b.WriteString(".ref-text{font-size:7.2pt;overflow-wrap:anywhere;}")
	b.WriteString(".measure-line{font-size:8.4pt;}")
	b.WriteString(".item-list{list-style:none;margin-top:0.6mm;}")
	// One line per item, ellipsised: a wrapping name made the row height
	// unpredictable, which is how five capped items still spilled onto a
	// second page. The ellipsis is visible, so nothing is cut silently.
	b.WriteString(".item-list li{font-size:8.4pt;line-height:1.2;margin-bottom:0.6mm;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}")
	b.WriteString(".item-qty{font-weight:700;}")
	b.WriteString(".item-more{font-size:7.2pt;margin-top:0.6mm;}")
	b.WriteString(".row-foot{font-size:6.5pt;line-height:1.2;}")
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

// renderShippingLabel renders one 100x150 slip through the same shippingLabelData and buildShippingLabelsHTML as GetShippingLabelSized.
func renderShippingLabel(ctx context.Context, renderer pdfGenerator, order model.Order, cfg map[string]string) ([]byte, error) {
	if order.TrackingNumber == "" {
		return nil, ErrNoTrackingNumber
	}
	data, err := shippingLabelData(order, cfg)
	if err != nil {
		return nil, err
	}
	html, err := buildShippingLabelHTML(data)
	if err != nil {
		return nil, err
	}
	return renderer.RenderHTML(ctx, html)
}

func shippingLabelCourierLogo(code *string) template.URL {
	if code == nil {
		return ""
	}
	var file string
	switch strings.ToLower(*code) {
	case "jne", "sicepat", "lion":
		file = strings.ToLower(*code) + ".svg"
	case "tiki", "anteraja":
		file = strings.ToLower(*code) + ".png"
	default:
		return ""
	}
	raw, err := labelCourierAssets.ReadFile("assets/couriers/" + file)
	if err != nil || len(raw) == 0 {
		return ""
	}
	mime := "image/png"
	if strings.HasSuffix(file, ".svg") {
		mime = "image/svg+xml"
	}
	return template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw))
}

func shippingLabelData(order model.Order, cfg map[string]string) (shippingLabelHTMLData, error) {

	dest, err := parseShipmentDestination(order.ShippingAddress)
	if err != nil {
		return shippingLabelHTMLData{}, fmt.Errorf("shipping label: %w", err)
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

	return shippingLabelHTMLData{
		Waybill:        order.TrackingNumber,
		OrderReference: order.ID.String(),

		CourierLogoDataURI: shippingLabelCourierLogo(order.CourierCode),

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
	}, nil
}

// GetShippingLabel prints an order's packing slip (FR-D-1..D-4): sender
// details come from system_config, recipient details from the order's own
// shipping address, and the waybill is whatever tracking_number ship time
// stamped (Biteship or manual).
func (s *Service) GetShippingLabel(ctx context.Context, orderID string) ([]byte, error) {
	return s.GetShippingLabelSized(ctx, orderID, "")
}

func (s *Service) shippingLabelStore() shippingLabelStore {
	if s.labelStore != nil {
		return s.labelStore
	}
	return s.storeRepo
}

type shippingLabelStore interface {
	GetOrderByID(context.Context, uuid.UUID) (model.Order, error)
	ListSystemConfig(context.Context) ([]repository.SystemConfigRow, error)
}

type LabelNotPrintable struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

type LabelsNotPrintableError struct{ Details []LabelNotPrintable }

func (e *LabelsNotPrintableError) Error() string { return ErrLabelsNotPrintable.Error() }
func (e *LabelsNotPrintableError) Unwrap() error { return ErrLabelsNotPrintable }

var bulkLabelRender = make(chan struct{}, 1)

func (s *Service) GetShippingLabels(ctx context.Context, orderIDs []string, size string) ([]byte, error) {
	if size != "100x150" && size != "roll80" {
		return nil, ErrInvalidLabelRequest
	}
	if len(orderIDs) < 1 || len(orderIDs) > 100 {
		return nil, ErrInvalidLabelRequest
	}
	ids := make([]uuid.UUID, len(orderIDs))
	seen := make(map[uuid.UUID]bool, len(orderIDs))
	for i, raw := range orderIDs {
		id, err := parseUUID(raw)
		if err != nil || seen[id] {
			return nil, ErrInvalidLabelRequest
		}
		seen[id] = true
		ids[i] = id
	}
	select {
	case bulkLabelRender <- struct{}{}:
		defer func() { <-bulkLabelRender }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	store := s.shippingLabelStore()
	orders := make([]model.Order, 0, len(ids))
	var details []LabelNotPrintable
	for i, id := range ids {
		order, err := store.GetOrderByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get shipping label order: %w", err)
		}
		reason := ""
		switch {
		case order.ID == uuid.Nil:
			reason = "not_found"
		case order.TrackingNumber == "":
			reason = "no_tracking_number"
		case isShipmentFailureStatus(order.ShipmentStatus):
			reason = "shipment_failed"
		}
		if reason != "" {
			details = append(details, LabelNotPrintable{orderIDs[i], reason})
		}
		orders = append(orders, order)
	}
	if len(details) > 0 {
		return nil, &LabelsNotPrintableError{Details: details}
	}
	return s.renderShippingLabels(ctx, orders, size)
}

func (s *Service) GetShippingLabelSized(ctx context.Context, orderID, size string) ([]byte, error) {
	if size == "" {
		size = "100x150"
	}
	if size != "100x150" && size != "roll80" {
		return nil, ErrInvalidLabelRequest
	}
	id, err := parseUUID(orderID)
	if err != nil {
		return nil, err
	}
	order, err := s.shippingLabelStore().GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.ID == uuid.Nil {
		return nil, ErrOrderNotFound
	}
	if order.TrackingNumber == "" {
		return nil, ErrNoTrackingNumber
	}
	return s.renderShippingLabels(ctx, []model.Order{order}, size)
}

func (s *Service) renderShippingLabels(ctx context.Context, orders []model.Order, size string) ([]byte, error) {
	rows, err := s.shippingLabelStore().ListSystemConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get shipping label config: %w", err)
	}
	cfg := buildConfigMap(rows)
	slips := make([]shippingLabelHTMLData, 0, len(orders))
	for _, order := range orders {
		data, err := shippingLabelData(order, cfg)
		if err != nil {
			return nil, err
		}
		slips = append(slips, data)
	}
	html, err := buildShippingLabelsHTML(slips, size)
	if err != nil {
		return nil, err
	}
	return s.renderer.RenderHTML(ctx, html)
}
