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
	"regexp"
	"strings"

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
	"score":              true,
	"max_score":          true,
	"score_percent":      true,
	"rank":               true,
	"percentile":         true,
	"duration":           true,
	"total_questions":    true,
	"logo":               true,
	"signature":          true,
}

// imageFieldIDs are the fields drawn from an uploaded image rather than text;
// they carry an explicit HMm box height instead of a font-derived line height.
var imageFieldIDs = map[string]bool{"logo": true, "signature": true}

var generatedLayerID = regexp.MustCompile(`^(text|image)_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var certificateTokenPattern = regexp.MustCompile(`\{\{([^{}]+)\}\}`)
var validCertificateTokens = map[string]bool{
	"student_name": true, "exam_title": true, "completion_date": true,
	"certificate_number": true, "score": true, "max_score": true,
	"score_percent": true, "rank": true, "percentile": true,
	"duration": true, "total_questions": true,
}

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
	ID       string  `json:"id"`
	Kind     string  `json:"kind,omitempty"`
	Name     string  `json:"name,omitempty"`
	Content  string  `json:"content,omitempty"`
	XMm      float64 `json:"x_mm"`
	YMm      float64 `json:"y_mm"`
	WMm      float64 `json:"w_mm"`
	Align    string  `json:"align"`
	Font     string  `json:"font,omitempty"`
	Weight   string  `json:"weight,omitempty"`
	Italic   bool    `json:"italic,omitempty"`
	SizePt   float64 `json:"size_pt,omitempty"`
	Color    string  `json:"color,omitempty"`
	Visible  bool    `json:"visible"`
	HMm      float64 `json:"h_mm,omitempty"`
	AssetKey *string `json:"asset_key,omitempty"`
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

func defaultCertificateContent(fieldID string) string {
	switch fieldID {
	case "title":
		return "CERTIFICATE OF COMPLETION"
	case "subtitle":
		return "This certificate is proudly awarded to"
	case "student_name":
		return "{{student_name}}"
	case "completion_text":
		return "for successfully completing"
	case "exam_title":
		return "{{exam_title}}"
	case "date":
		return "{{completion_date}}"
	case "certificate_number":
		return "{{certificate_number}}"
	default:
		return ""
	}
}

func normalizeCertificateLayout(layout Layout) Layout {
	out := layout
	out.Fields = append([]LayoutField(nil), layout.Fields...)
	for i := range out.Fields {
		f := &out.Fields[i]
		if f.Kind == "" {
			if imageFieldIDs[f.ID] || strings.HasPrefix(f.ID, "image_") {
				f.Kind = "image"
			} else {
				f.Kind = "text"
			}
		}
		if f.Name == "" {
			f.Name = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(f.ID, "text_"), "image_"), "_", " ")
		}
		if f.Kind == "text" {
			if f.Content == "" {
				f.Content = defaultCertificateContent(f.ID)
			}
			if f.Font == "" {
				f.Font = "public_sans"
			}
			if f.SizePt <= 0 {
				f.SizePt = 12
			}
			if f.Color == "" {
				f.Color = "#17213B"
			}
		}
		if f.Align == "" {
			f.Align = "center"
		}
		if f.ID == "signature" && f.AssetKey == nil && layout.SignatureKey != nil {
			f.AssetKey = layout.SignatureKey
		}
	}
	return out
}

func certificateTokens(content string) []string {
	matches := certificateTokenPattern.FindAllStringSubmatch(content, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

func ValidateCertificateDesignAssetKeys(examID string, backgroundKey *string, layout Layout, existingDesign *json.RawMessage) error {
	prefix := "certificates/" + examID + "/"
	legacyKeys := make(map[string]bool)
	if existing, err := parseCertificateDesign(existingDesign); err == nil {
		if existing.BackgroundKey != nil {
			legacyKeys[*existing.BackgroundKey] = true
		}
		for _, field := range normalizeCertificateLayout(existing.Layout).Fields {
			if field.AssetKey != nil {
				legacyKeys[*field.AssetKey] = true
			}
		}
	}
	validate := func(key *string) error {
		if key == nil || *key == "" || strings.HasPrefix(*key, prefix) || legacyKeys[*key] {
			return nil
		}
		return fmt.Errorf("%w: certificate asset key is outside this exam", ErrValidation)
	}
	if err := validate(backgroundKey); err != nil {
		return err
	}
	for _, field := range normalizeCertificateLayout(layout).Fields {
		if field.Kind == "image" {
			if err := validate(field.AssetKey); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateLayout rejects a degenerate page size, an unknown field id, a
// duplicate field id, and any field box that falls outside the page. It does
// not validate Font (see LayoutField).
func ValidateLayout(l Layout) error {
	l = normalizeCertificateLayout(l)
	if l.Page.WidthMm <= 0 || l.Page.HeightMm <= 0 {
		return fmt.Errorf("%w: page dimensions must be positive", ErrValidation)
	}
	if l.Page.WidthMm != 297 || l.Page.HeightMm != 210 {
		return fmt.Errorf("%w: certificate page must be 297x210mm", ErrValidation)
	}

	seen := make(map[string]bool, len(l.Fields))
	for _, f := range l.Fields {
		if !validLayoutFieldIDs[f.ID] && !generatedLayerID.MatchString(f.ID) {
			return fmt.Errorf("%w: unknown field id: %s", ErrValidation, f.ID)
		}
		if seen[f.ID] {
			return fmt.Errorf("%w: duplicate field id: %s", ErrValidation, f.ID)
		}
		seen[f.ID] = true

		if f.Kind != "text" && f.Kind != "image" {
			return fmt.Errorf("%w: unknown layer kind: %s", ErrValidation, f.Kind)
		}
		if f.WMm <= 0 {
			return fmt.Errorf("%w: field %s width must be positive", ErrValidation, f.ID)
		}
		boxHeightMm := f.HMm
		if f.Kind == "text" {
			for _, token := range certificateTokens(f.Content) {
				if !validCertificateTokens[token] {
					return fmt.Errorf("%w: Unknown certificate token: {{%s}}", ErrValidation, token)
				}
			}
			boxHeightMm = nominalLineHeightMm(f.SizePt)
		} else if f.HMm <= 0 {
			return fmt.Errorf("%w: field %s height must be positive", ErrValidation, f.ID)
		} else if strings.HasPrefix(f.ID, "image_") && (f.AssetKey == nil || *f.AssetKey == "") {
			return fmt.Errorf("%w: field %s asset key is required", ErrValidation, f.ID)
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
		// Score-bearing layout (redesign 2026-07-30). Symmetric around the page
		// centre; the background's navy waves occupy the top-left and bottom-right
		// corners and a ribbon sits top-right, so nothing is placed at x<14,
		// x>246 above y=80, or x>200 below y=150.
		//
		// This is the ONLY built-in layout that stamps score/percentile, which
		// makes it "sensitive" to certificateLayoutAllowed: an exam whose
		// result_config is 'hidden' (the column default) produces NO certificate
		// at all on this template, silently. See ADR note in the epic doc.
		//
		// Space at x 16..34, y 166..180 is deliberately left empty for the QR
		// code that "scan to verify" needs; that field type does not exist yet.
		return Layout{
			Page:       page,
			Background: Background{Kind: "builtin", Ref: "modern"},
			Fields: []LayoutField{
				// Off by default: this background's medallion is far smaller than the
				// mockup's and already carries stars + laurel, so text inside it clips.
				// Kept as a layer so an admin can enable and reposition it in the studio.
				{ID: "text_3f2a91c4-0e7b-4a15-9c83-6d1e8b4f27a0", Kind: "text", Name: "well done badge", Content: "WELL DONE!", XMm: 249, YMm: 27, WMm: 29, Align: "center", Font: "public_sans", Weight: "bold", SizePt: 8, Color: "#F0CB78", Visible: false},

				{ID: "title", Content: "CERTIFICATE", XMm: 48, YMm: 35, WMm: 201, Align: "center", Font: "playfair_display", Weight: "bold", SizePt: 38, Color: "#1B2A4E", Visible: true},
				{ID: "subtitle", Content: "OF COMPLETION", XMm: 48, YMm: 57, WMm: 201, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 12.5, Color: "#B8873B", Visible: true},
				{ID: "text_8c14d7e2-5b39-4f60-a8d1-2e9f7a03c5b8", Kind: "text", Name: "presented to", Content: "This is proudly presented to", XMm: 48, YMm: 73, WMm: 201, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 12, Color: "#3D4A5C", Visible: true},
				{ID: "student_name", XMm: 48, YMm: 83, WMm: 201, Align: "center", Font: "great_vibes", Weight: "regular", SizePt: 42, Color: "#1B2A4E", Visible: true},
				{ID: "completion_text", XMm: 62, YMm: 111, WMm: 173, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 11, Color: "#3D4A5C", Visible: true},
				{ID: "exam_title", XMm: 62, YMm: 117, WMm: 173, Align: "center", Font: "source_serif_4", Weight: "bold", SizePt: 12, Color: "#1B2A4E", Visible: true},

				// Left metadata column. Values are real fields; the labels above
				// them are static text layers because the background carries no
				// icons (the mockup's calendar/clock/document glyphs are absent).
				{ID: "text_a7e05b31-9d64-4c28-b0f7-3a5c1e8d4b26", Kind: "text", Name: "exam date label", Content: "Exam Date", XMm: 16, YMm: 107, WMm: 54, Align: "left", Font: "public_sans", Weight: "bold", SizePt: 9, Color: "#B8873B", Visible: true},
				{ID: "date", XMm: 16, YMm: 112, WMm: 54, Align: "left", Font: "public_sans", Weight: "regular", SizePt: 10.5, Color: "#3D4A5C", Visible: true},
				{ID: "text_b2f18c40-6a73-4e91-8d5b-7c0e2f9a4d16", Kind: "text", Name: "duration label", Content: "Duration", XMm: 16, YMm: 123, WMm: 54, Align: "left", Font: "public_sans", Weight: "bold", SizePt: 9, Color: "#B8873B", Visible: true},
				{ID: "duration", Content: "{{duration}}", XMm: 16, YMm: 128, WMm: 54, Align: "left", Font: "public_sans", Weight: "regular", SizePt: 10.5, Color: "#3D4A5C", Visible: true},
				{ID: "text_c93d6e57-1f82-4b04-a6c9-8e5b3d7f0a29", Kind: "text", Name: "total questions label", Content: "Total Questions", XMm: 16, YMm: 139, WMm: 54, Align: "left", Font: "public_sans", Weight: "bold", SizePt: 9, Color: "#B8873B", Visible: true},
				{ID: "total_questions", Content: "{{total_questions}}", XMm: 16, YMm: 144, WMm: 54, Align: "left", Font: "public_sans", Weight: "regular", SizePt: 10.5, Color: "#3D4A5C", Visible: true},

				// Score row. Two composed text layers rather than four bare fields
				// so "86 / 100" and "TOP 15%" read as single units. Content tokens
				// are what layoutUsesToken scans, so the gate still sees these.
				{ID: "text_d40a2b68-7c15-493f-b2e8-1a6d9c4f3e07", Kind: "text", Name: "your score label", Content: "YOUR SCORE", XMm: 80, YMm: 133, WMm: 58, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 9.5, Color: "#3D4A5C", Visible: true},
				{ID: "text_e51b3c79-8d26-4a40-c3f9-2b7e0d5a4f18", Kind: "text", Name: "score value", Content: "{{score}} / {{max_score}}", XMm: 80, YMm: 140, WMm: 58, Align: "center", Font: "playfair_display", Weight: "bold", SizePt: 21, Color: "#1B2A4E", Visible: true},
				{ID: "text_f62c4d80-9e37-4b51-d40a-3c8f1e6b5a29", Kind: "text", Name: "percentile label", Content: "PERCENTILE", XMm: 140, YMm: 133, WMm: 58, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 9.5, Color: "#3D4A5C", Visible: true},
				{ID: "text_073d5e91-af48-4c62-e51b-4d902f7c6b3a", Kind: "text", Name: "percentile value", Content: "{{percentile}}", XMm: 140, YMm: 140, WMm: 58, Align: "center", Font: "playfair_display", Weight: "bold", SizePt: 21, Color: "#B8873B", Visible: true},

				{ID: "signature", XMm: 118, YMm: 166, WMm: 62, HMm: 18, Align: "center", Visible: false},
				{ID: "text_184e6fa2-b059-4d73-f62c-5e01308d7c4b", Kind: "text", Name: "signatory title", Content: "Academic Director", XMm: 118, YMm: 187, WMm: 62, Align: "center", Font: "public_sans", Weight: "bold", SizePt: 9.5, Color: "#3D4A5C", Visible: true},

				{ID: "text_29507ab3-c16a-4e84-073d-6f12419e8d5c", Kind: "text", Name: "certificate id label", Content: "Certificate ID", XMm: 16, YMm: 182, WMm: 62, Align: "left", Font: "public_sans", Weight: "bold", SizePt: 8.5, Color: "#B8873B", Visible: true},
				{ID: "certificate_number", XMm: 16, YMm: 187, WMm: 62, Align: "left", Font: "public_sans", Weight: "regular", SizePt: 9, Color: "#3D4A5C", Visible: true},
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
				{ID: "date", XMm: 48.5, YMm: 157, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 11, Color: "#4A5568", Visible: true},
				{ID: "certificate_number", XMm: 48.5, YMm: 193, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 9, Color: "#6B5B34", Visible: true},
				{ID: "signature", XMm: 205, YMm: 150, WMm: 62, HMm: 22, Align: "center", Visible: false},
			},
		}
	}
}
