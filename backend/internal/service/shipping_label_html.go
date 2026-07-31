package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"image/png"

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
)

// This is a packing slip, not the scannable carrier label (FR-D-4) — that one
// comes from Biteship's dashboard. Every user-facing string here says so.
var shippingLabelHTMLTemplate = template.Must(template.New("shipping_label").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>{{.StyleBlock}}</style>
</head>
<body>
<div class="label-title">PACKING SLIP</div>
<div class="label-disclaimer">Bukan label resmi kurir. Label pengiriman resmi (barcode pelacakan kurir) tersedia di dashboard Biteship.</div>
<div class="label-section">
<div class="label-section-title">DARI</div>
<div class="label-name">{{.SenderName}}</div>
<div class="label-line">{{.SenderPhone}}</div>
<div class="label-line">{{.SenderAddress}}</div>
<div class="label-line">{{.SenderPostal}}</div>
</div>
<div class="label-section">
<div class="label-section-title">KEPADA</div>
<div class="label-name">{{.RecipientName}}</div>
<div class="label-line">{{.RecipientPhone}}</div>
<div class="label-line">{{.RecipientAddress}}</div>
<div class="label-line">{{.RecipientPostal}}</div>
</div>
<div class="label-waybill">
<img class="label-barcode" src="data:image/png;base64,{{.BarcodeBase64}}" alt="">
<div class="label-waybill-text">{{.Waybill}}</div>
</div>
</body>
</html>`))

type shippingLabelHTMLData struct {
	StyleBlock       template.CSS
	Waybill          string
	BarcodeBase64    string
	SenderName       string
	SenderPhone      string
	SenderAddress    string
	SenderPostal     string
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
func buildShippingLabelHTML(waybill, senderName, senderPhone, senderAddress, senderPostal, recipientName, recipientPhone, recipientAddress, recipientPostal string) ([]byte, error) {
	barcodeBase64, err := shippingLabelBarcodeBase64(waybill)
	if err != nil {
		return nil, fmt.Errorf("build shipping label barcode: %w", err)
	}

	data := shippingLabelHTMLData{
		StyleBlock:       shippingLabelStyleBlock(),
		Waybill:          waybill,
		BarcodeBase64:    barcodeBase64,
		SenderName:       senderName,
		SenderPhone:      senderPhone,
		SenderAddress:    senderAddress,
		SenderPostal:     senderPostal,
		RecipientName:    recipientName,
		RecipientPhone:   recipientPhone,
		RecipientAddress: recipientAddress,
		RecipientPostal:  recipientPostal,
	}

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
func shippingLabelBarcodeBase64(waybill string) (string, error) {
	bc, err := code128.Encode(waybill)
	if err != nil {
		return "", fmt.Errorf("code128 encode: %w", err)
	}
	scaled, err := barcode.Scale(bc, labelBarcodeWidthPx, labelBarcodeHeightPx)
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
	fmt.Fprintf(&b, "html,body{width:%.0fmm;height:%.0fmm;font-family:sans-serif;color:#1a1a1a;}", labelPageWidthMm, labelPageHeightMm)
	b.WriteString(".label-title{padding:4mm 4mm 0 4mm;font-size:14pt;font-weight:bold;}")
	b.WriteString(".label-disclaimer{padding:0 4mm 3mm 4mm;font-size:6.5pt;color:#555;border-bottom:0.5pt solid #999;}")
	b.WriteString(".label-section{padding:3mm 4mm;border-bottom:0.5pt solid #999;}")
	b.WriteString(".label-section-title{font-size:7pt;font-weight:bold;color:#555;letter-spacing:0.5pt;}")
	b.WriteString(".label-name{font-size:10pt;font-weight:bold;margin-top:1mm;}")
	b.WriteString(".label-line{font-size:8.5pt;margin-top:0.5mm;}")
	b.WriteString(".label-waybill{padding:4mm;text-align:center;}")
	b.WriteString(".label-barcode{width:80mm;height:20mm;}")
	b.WriteString(".label-waybill-text{margin-top:2mm;font-size:11pt;font-weight:bold;letter-spacing:1pt;}")
	return template.CSS(b.String())
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

	html, err := buildShippingLabelHTML(
		order.TrackingNumber,
		cfg["app_name"], cfg["app_contact_phone"], cfg["app_address"], cfg["app_kode_pos"],
		dest.Penerima, dest.Telepon, dest.Alamat, dest.KodePos,
	)
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
