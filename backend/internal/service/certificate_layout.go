package service

// Coordinate contract (FR-1, FR-2): every coordinate in this file is expressed in
// millimetres, origin at the page's top-left corner, Y increasing downward — no
// flip anywhere. x_mm,y_mm is the box's top-left corner and align applies inside
// the box. The renderer consumes these values directly in "mm" mode; the
// editor's only conversion is the uniform scale
// mm = px * (page_width_mm / preview_width_px). Never compute pageHeight - y.

import (
	"encoding/json"
	"fmt"

	"akademi-bimbel/internal/model"
)

// FieldID identifies a certificate layout field. It aliases string rather than
// defining a distinct type so it drops in wherever LayoutField.ID and
// validLayoutFieldIDs already use plain strings, with no conversion at call sites.
type FieldID = string

// certificatePageWidthMm and certificatePageHeightMm are the A4 landscape page
// dimensions used by every certificate layout.
const (
	certificatePageWidthMm  = 297
	certificatePageHeightMm = 210
)

// validLayoutFieldIDs is the closed set from FR-3. Any other id is rejected.
var validLayoutFieldIDs = map[string]bool{
	"title":              true,
	"subtitle":           true,
	"student_name":       true,
	"exam_title":         true,
	"completion_text":    true,
	"date":               true,
	"certificate_number": true,
	"logo":               true,
	"signature":          true,
}

// imageFieldIDs are the fields drawn from an uploaded image rather than text;
// they carry an explicit HMm box height instead of a font-derived line height.
var imageFieldIDs = map[string]bool{"logo": true, "signature": true}

// Page is the layout's page size in millimetres.
type Page struct {
	WidthMm  float64 `json:"width_mm"`
	HeightMm float64 `json:"height_mm"`
}

// Background describes the certificate's background image. Kind is "builtin"
// (Ref names one of classic/modern/elegant) or "custom" (Ref is an uploaded
// object key).
type Background struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// LayoutField positions one stamped field. XMm/YMm is the box's top-left corner
// (FR-2); Align governs text placement inside the box. Font is validated against
// the closed FR-7a family set only at render time, not here: an unknown family
// must still render by falling back to a brand default (Invariant 8), so parse-time
// rejection would defeat that fallback. The logo field carries HMm instead of the
// text properties (Font/Weight/SizePt) — those are left zero-valued for it.
type LayoutField struct {
	ID      string  `json:"id"`
	XMm     float64 `json:"x_mm"`
	YMm     float64 `json:"y_mm"`
	WMm     float64 `json:"w_mm"`
	Align   string  `json:"align"`
	Font    string  `json:"font,omitempty"`
	Weight  string  `json:"weight,omitempty"`
	SizePt  float64 `json:"size_pt,omitempty"`
	Color   string  `json:"color,omitempty"`
	Visible bool    `json:"visible"`
	HMm     float64 `json:"h_mm,omitempty"`
}

// Layout is the JSON contract shared by the certificate renderer and the admin
// editor, embedded (flattened) inside the exam.certificate_design blob.
type Layout struct {
	Page       Page          `json:"page"`
	Background Background    `json:"background"`
	Fields     []LayoutField `json:"fields"`
	// SignatureKey is the private-bucket object key of an uploaded signature
	// image; nil until an admin uploads one. The image is stamped at the
	// "signature" field's box when that field is visible.
	SignatureKey *string `json:"signature_key,omitempty"`
}

// certificateDesign is the full shape persisted in exam.certificate_design
// (FR-26/FR-27): Layout's fields (page/background/fields/signature_key) plus
// Template and BackgroundKey, which lived in separate certificate_template and
// certificate_background_key columns before migration 0042. Embedding Layout
// keeps those keys at the top level of the JSON object rather than nesting
// them under a "layout" key.
type certificateDesign struct {
	Layout
	Template string `json:"template,omitempty"`
	// BackgroundKey is the private-bucket object key of an uploaded custom
	// background (never a raw or presigned URL). Nil when no custom background
	// is set. Distinct from Layout.Background, which only carries display
	// metadata (kind/ref) for the editor.
	BackgroundKey *string `json:"background_key,omitempty"`
}

// parseCertificateDesign unmarshals exam.CertificateDesign. A nil blob (an
// exam that has never had a design saved) parses to a zero-value design —
// empty template, no background key, no layout fields — so callers apply
// their own defaults rather than erroring.
func parseCertificateDesign(raw *json.RawMessage) (certificateDesign, error) {
	var d certificateDesign
	if raw == nil {
		return d, nil
	}
	if err := json.Unmarshal(*raw, &d); err != nil {
		return certificateDesign{}, fmt.Errorf("unmarshal certificate design: %w", err)
	}
	return d, nil
}

// marshalCertificateDesign serializes a certificateDesign back into the raw
// JSON shape stored in exam.CertificateDesign.
func marshalCertificateDesign(d certificateDesign) (*json.RawMessage, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(b)
	return &raw, nil
}

// certificateTemplate returns the template name persisted in an exam's
// certificate_design blob, defaulting to "classic" for an exam that has no
// design yet (mirrors defaultLayout's own "unknown template" fallback, and
// the pre-Task-8 certificate_template column's NOT NULL DEFAULT 'classic').
// Helper for call sites that only need the template, not the full parsed design.
func certificateTemplate(e *model.Exam) string {
	d, _ := parseCertificateDesign(e.CertificateDesign)
	if d.Template == "" {
		return "classic"
	}
	return d.Template
}

// certificateBackgroundKey returns the custom background object key
// persisted in an exam's certificate_design blob, or nil if unset.
func certificateBackgroundKey(e *model.Exam) *string {
	d, _ := parseCertificateDesign(e.CertificateDesign)
	return d.BackgroundKey
}

// SetCertificateTemplate overlays a new template name onto an exam's existing
// certificate_design blob, preserving its background key/layout/signature key.
// Used by AdminUpdateExam's plain certificate_template PATCH field, which
// (unlike the dedicated certificate-design PUT) doesn't also send a layout.
func SetCertificateTemplate(design *json.RawMessage, template string) (*json.RawMessage, error) {
	d, err := parseCertificateDesign(design)
	if err != nil {
		return nil, err
	}
	d.Template = template
	return marshalCertificateDesign(d)
}

// MarshalCertificateDesign builds the exam.certificate_design JSON blob from
// the admin editor's PUT body (template, background key, layout) — the
// combined shape GetCertificateDesign/resolveCertificateLayout/
// resolveCertificateBackground read back (FR-26/FR-27). Exported so the
// handler layer can assemble the blob without reaching into this package's
// unexported certificateDesign type.
func MarshalCertificateDesign(template string, backgroundKey *string, layout Layout) (*json.RawMessage, error) {
	return marshalCertificateDesign(certificateDesign{
		Layout:        layout,
		Template:      template,
		BackgroundKey: backgroundKey,
	})
}

// nominalLineHeightMm derives an approximate single-line text box height in
// millimetres from a font size in points, mirroring the line-height factor
// renderCertificate uses when stamping text (1pt = 0.3528mm, with a 1.15
// leading multiplier — see renderCertificate's lineHeightMm). A text field has
// no h_mm of its own, so this is what both the editor's clamp and this
// validation use as the field's effective box height (FR-28).
func nominalLineHeightMm(sizePt float64) float64 {
	return sizePt * 0.3528 * 1.15
}

// ValidateLayout rejects a degenerate page size, an unknown field id, a
// duplicate field id, and any field box that falls outside the page. It does
// not validate Font (see LayoutField).
func ValidateLayout(l Layout) error {
	if l.Page.WidthMm <= 0 || l.Page.HeightMm <= 0 {
		return fmt.Errorf("%w: page dimensions must be positive", ErrValidation)
	}

	seen := make(map[string]bool, len(l.Fields))
	for _, f := range l.Fields {
		if !validLayoutFieldIDs[f.ID] {
			return fmt.Errorf("%w: unknown field id: %s", ErrValidation, f.ID)
		}
		if seen[f.ID] {
			return fmt.Errorf("%w: duplicate field id: %s", ErrValidation, f.ID)
		}
		seen[f.ID] = true

		boxHeightMm := f.HMm
		if !imageFieldIDs[f.ID] {
			boxHeightMm = nominalLineHeightMm(f.SizePt)
		}
		if f.XMm < 0 || f.YMm < 0 || f.XMm+f.WMm > l.Page.WidthMm || f.YMm+boxHeightMm > l.Page.HeightMm {
			return fmt.Errorf("%w: field %s box is outside the page", ErrValidation, f.ID)
		}
	}
	return nil
}

// defaultLayout returns the built-in default field placement for template
// ("classic", "modern", or "elegant"). All three share the A4 landscape page and
// the closed field id set; they differ in font, color, and layout rhythm.
func defaultLayout(template string) Layout {
	page := Page{WidthMm: certificatePageWidthMm, HeightMm: certificatePageHeightMm}

	switch template {
	case "modern":
		// Asymmetric: a navy songket band occupies the left ~46mm, so text is
		// centred in the remaining field (x 55..283, centre ~169), not the page.
		return Layout{
			Page:       page,
			Background: Background{Kind: "builtin", Ref: "modern"},
			Fields: []LayoutField{
				{ID: "title", XMm: 55, YMm: 44, WMm: 228, Align: "center", Font: "playfair_display", Weight: "bold", SizePt: 29, Color: "#22315B", Visible: true},
				{ID: "subtitle", XMm: 55, YMm: 76, WMm: 228, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 12, Color: "#157A6E", Visible: true},
				{ID: "student_name", XMm: 55, YMm: 100, WMm: 228, Align: "center", Font: "cormorant_garamond", Weight: "bold", SizePt: 34, Color: "#22315B", Visible: true},
				{ID: "completion_text", XMm: 55, YMm: 122, WMm: 228, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 11, Color: "#4A5568", Visible: true},
				{ID: "exam_title", XMm: 55, YMm: 137, WMm: 228, Align: "center", Font: "source_serif_4", Weight: "regular", SizePt: 15, Color: "#22315B", Visible: true},
				{ID: "date", XMm: 55, YMm: 160, WMm: 228, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 11, Color: "#4A5568", Visible: true},
				{ID: "certificate_number", XMm: 55, YMm: 196, WMm: 228, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 9, Color: "#157A6E", Visible: true},
				{ID: "signature", XMm: 205, YMm: 150, WMm: 62, HMm: 22, Align: "center", Visible: false},
			},
		}
	case "elegant":
		return Layout{
			Page:       page,
			Background: Background{Kind: "builtin", Ref: "elegant"},
			Fields: []LayoutField{
				{ID: "title", XMm: 48.5, YMm: 56, WMm: 200, Align: "center", Font: "cinzel", Weight: "bold", SizePt: 26, Color: "#22315B", Visible: true},
				{ID: "subtitle", XMm: 48.5, YMm: 82, WMm: 200, Align: "center", Font: "cormorant_garamond", Weight: "regular", SizePt: 14, Color: "#6B5B34", Visible: true},
				{ID: "student_name", XMm: 48.5, YMm: 104, WMm: 200, Align: "center", Font: "great_vibes", Weight: "regular", SizePt: 40, Color: "#22315B", Visible: true},
				{ID: "completion_text", XMm: 48.5, YMm: 124, WMm: 200, Align: "center", Font: "cormorant_garamond", Weight: "regular", SizePt: 12, Color: "#6B5B34", Visible: true},
				{ID: "exam_title", XMm: 48.5, YMm: 139, WMm: 200, Align: "center", Font: "source_serif_4", Weight: "regular", SizePt: 15, Color: "#22315B", Visible: true},
				{ID: "date", XMm: 48.5, YMm: 162, WMm: 200, Align: "center", Font: "cormorant_garamond", Weight: "regular", SizePt: 12.5, Color: "#6B5B34", Visible: true},
				{ID: "certificate_number", XMm: 48.5, YMm: 182, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 9, Color: "#8A6A16", Visible: true},
				{ID: "signature", XMm: 205, YMm: 150, WMm: 62, HMm: 22, Align: "center", Visible: false},
			},
		}
	default: // "classic"
		return Layout{
			Page:       page,
			Background: Background{Kind: "builtin", Ref: "classic"},
			Fields: []LayoutField{
				{ID: "title", XMm: 48.5, YMm: 66, WMm: 200, Align: "center", Font: "cinzel", Weight: "bold", SizePt: 25, Color: "#22315B", Visible: true},
				{ID: "subtitle", XMm: 48.5, YMm: 90, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 12, Color: "#4A5568", Visible: true},
				{ID: "student_name", XMm: 48.5, YMm: 108, WMm: 200, Align: "center", Font: "cormorant_garamond", Weight: "bold", SizePt: 40, Color: "#22315B", Visible: true},
				{ID: "completion_text", XMm: 48.5, YMm: 130, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 12, Color: "#4A5568", Visible: true},
				{ID: "exam_title", XMm: 48.5, YMm: 145, WMm: 200, Align: "center", Font: "source_serif_4", Weight: "regular", SizePt: 15, Color: "#22315B", Visible: true},
				{ID: "date", XMm: 48.5, YMm: 166, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 11, Color: "#4A5568", Visible: true},
				{ID: "certificate_number", XMm: 48.5, YMm: 193, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 9, Color: "#F0CB78", Visible: true},
				{ID: "signature", XMm: 205, YMm: 150, WMm: 62, HMm: 22, Align: "center", Visible: false},
			},
		}
	}
}
