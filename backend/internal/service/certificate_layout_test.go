package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeCertificateLayout_LegacyStudentNameGetsTokenContent(t *testing.T) {
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{{
		ID: "student_name", XMm: 48.5, YMm: 100, WMm: 200,
		Align: "center", Font: "source_serif_4", SizePt: 26, Visible: true,
	}}}
	got := normalizeCertificateLayout(layout)
	if got.Fields[0].Kind != "text" || got.Fields[0].Content != "{{student_name}}" {
		t.Fatalf("normalized field = %+v", got.Fields[0])
	}
}

func TestNormalizeCertificateLayout_LegacyTextUsesEditorDefaultColor(t *testing.T) {
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{{
		ID: "student_name", XMm: 48.5, YMm: 100, WMm: 200, Visible: true,
	}}}

	got := normalizeCertificateLayout(layout)
	if got.Fields[0].Color != "#17213B" {
		t.Fatalf("legacy text color = %q, want editor default #17213B", got.Fields[0].Color)
	}
}

func TestValidateLayout_RejectsUnknownToken(t *testing.T) {
	layout := normalizeCertificateLayout(defaultLayout("classic"))
	layout.Fields[0].Content = "{{secret_score}}"
	err := ValidateLayout(layout)
	if err == nil || !strings.Contains(err.Error(), "Unknown certificate token: {{secret_score}}") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateLayout_RejectsUnknownTokenWithUppercaseOrDigits(t *testing.T) {
	for _, token := range []string{"{{Secret}}", "{{secret2}}"} {
		layout := normalizeCertificateLayout(defaultLayout("classic"))
		layout.Fields[0].Content = token
		if err := ValidateLayout(layout); err == nil || !strings.Contains(err.Error(), "Unknown certificate token") {
			t.Fatalf("token %q err = %v", token, err)
		}
	}
}

func TestValidateCertificateDesignAssetKeys_ScopesNewKeysToExam(t *testing.T) {
	examID := "550e8400-e29b-41d4-a716-446655440000"
	valid := "certificates/" + examID + "/logo.png"
	invalid := "student-bulk/another-school/students.csv"
	layout := normalizeCertificateLayout(defaultLayout("classic"))
	layout.Fields = append(layout.Fields, LayoutField{
		ID: "image_550e8400-e29b-41d4-a716-446655440001", Kind: "image", AssetKey: &valid,
		XMm: 10, YMm: 10, WMm: 20, HMm: 20, Visible: true,
	})
	if err := ValidateCertificateDesignAssetKeys(examID, &valid, layout, nil); err != nil {
		t.Fatal(err)
	}
	layout.Fields[len(layout.Fields)-1].AssetKey = &invalid
	if err := ValidateCertificateDesignAssetKeys(examID, &valid, layout, nil); err == nil {
		t.Fatal("expected cross-namespace key to be rejected")
	}
}

func TestValidateCertificateDesignAssetKeys_AllowsExistingLegacyKey(t *testing.T) {
	examID := "550e8400-e29b-41d4-a716-446655440000"
	legacy := "avatars/admin/legacy-signature.png"
	raw := json.RawMessage(`{"page":{"width_mm":297,"height_mm":210},"background":{"kind":"builtin","ref":"classic"},"fields":[{"id":"signature","kind":"image","asset_key":"avatars/admin/legacy-signature.png","x_mm":10,"y_mm":10,"w_mm":20,"h_mm":20,"align":"center","visible":true}]}`)
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{{
		ID: "signature", Kind: "image", AssetKey: &legacy, XMm: 10, YMm: 10, WMm: 20, HMm: 20, Visible: true,
	}}}
	if err := ValidateCertificateDesignAssetKeys(examID, nil, layout, &raw); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLayout_AllowsGeneratedImageID(t *testing.T) {
	key := "certificates/assets/logo.png"
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{{
		ID: "image_550e8400-e29b-41d4-a716-446655440000", Kind: "image",
		AssetKey: &key, XMm: 10, YMm: 10, WMm: 30, HMm: 30, Visible: true,
	}}}
	if err := ValidateLayout(layout); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLayout_UnknownFieldIDRejected(t *testing.T) {
	l := Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "not_a_real_field", XMm: 10, YMm: 10, WMm: 50, Align: "left"},
		},
	}
	err := ValidateLayout(l)
	if err == nil {
		t.Fatal("expected error for unknown field id, got nil")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestValidateLayout_DuplicateFieldIDRejected(t *testing.T) {
	l := Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "title", XMm: 10, YMm: 10, WMm: 50, Align: "left"},
			{ID: "title", XMm: 20, YMm: 20, WMm: 50, Align: "left"},
		},
	}
	err := ValidateLayout(l)
	if err == nil {
		t.Fatal("expected error for duplicate field id, got nil")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestValidateLayout_OutOfPageBoxRejected(t *testing.T) {
	cases := []struct {
		name  string
		field LayoutField
	}{
		{"negative x", LayoutField{ID: "title", XMm: -5, YMm: 10, WMm: 50, Align: "left"}},
		{"negative y", LayoutField{ID: "title", XMm: 10, YMm: -5, WMm: 50, Align: "left"}},
		{"right edge overflow", LayoutField{ID: "title", XMm: 280, YMm: 10, WMm: 50, Align: "left"}},
		{"bottom edge overflow", LayoutField{ID: "title", XMm: 10, YMm: 250, WMm: 50, Align: "left"}},
		{"logo height overflow", LayoutField{ID: "logo", XMm: 10, YMm: 200, WMm: 20, Align: "left", HMm: 30}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := Layout{
				Page:   Page{WidthMm: 297, HeightMm: 210},
				Fields: []LayoutField{tc.field},
			}
			err := ValidateLayout(l)
			if err == nil {
				t.Fatalf("expected error for out-of-page box (%s), got nil", tc.name)
			}
			if !errors.Is(err, ErrValidation) {
				t.Errorf("expected ErrValidation, got %v", err)
			}
		})
	}
}

// TestValidateLayout_ZeroPageDimensionsRejected covers Warning 4/Invariant 8:
// a PUT that omits `layout` marshals the zero Layout (page 0x0mm, nil fields),
// which the old bounds loop accepted as a no-op since it never checked page
// size — every later render would then produce a zero-size page.
func TestValidateLayout_ZeroPageDimensionsRejected(t *testing.T) {
	cases := []struct {
		name string
		page Page
	}{
		{"zero page", Page{WidthMm: 0, HeightMm: 0}},
		{"zero width only", Page{WidthMm: 0, HeightMm: 210}},
		{"zero height only", Page{WidthMm: 297, HeightMm: 0}},
		{"negative width", Page{WidthMm: -297, HeightMm: 210}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := Layout{Page: tc.page}
			err := ValidateLayout(l)
			if err == nil {
				t.Fatal("expected error for degenerate page dimensions, got nil")
			}
			if !errors.Is(err, ErrValidation) {
				t.Errorf("expected ErrValidation, got %v", err)
			}
		})
	}
}

// TestValidateLayout_TextFieldNearBottomEdgeRejected covers Warning 5/FR-28: a
// text field's box has no h_mm, so a y_mm that leaves no room for even one
// line of its own font size runs the text off the page — the box's top-left
// corner sitting exactly on the bottom edge is not "inside the page".
func TestValidateLayout_TextFieldNearBottomEdgeRejected(t *testing.T) {
	l := Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "certificate_number", XMm: 48.5, YMm: 210, WMm: 200, Align: "center", SizePt: 9},
		},
	}
	err := ValidateLayout(l)
	if err == nil {
		t.Fatal("expected error for a text field with no room for its line height, got nil")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestValidateLayout_InPageBoxAccepted(t *testing.T) {
	l := Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "title", XMm: 10, YMm: 10, WMm: 50, Align: "left"},
			{ID: "logo", XMm: 10, YMm: 10, WMm: 20, Align: "left", HMm: 20},
		},
	}
	if err := ValidateLayout(l); err != nil {
		t.Errorf("expected in-page boxes to be accepted, got %v", err)
	}
}

func TestDefaultLayout_RoundTripsThroughJSON(t *testing.T) {
	for _, tmpl := range []string{"classic", "modern", "elegant"} {
		t.Run(tmpl, func(t *testing.T) {
			original := defaultLayout(tmpl)

			if err := ValidateLayout(original); err != nil {
				t.Fatalf("default layout for %s failed validation: %v", tmpl, err)
			}

			b, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var roundTripped Layout
			if err := json.Unmarshal(b, &roundTripped); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			b2, err := json.Marshal(roundTripped)
			if err != nil {
				t.Fatalf("second Marshal failed: %v", err)
			}

			if string(b) != string(b2) {
				t.Errorf("layout for %s did not round-trip unchanged\nfirst:  %s\nsecond: %s", tmpl, b, b2)
			}
		})
	}
}

// TestDefaultLayout_CoversAllClosedFieldIDs checks the default layouts cover
// every closed field id (FR-3) except "logo": renderCertificate never stamps
// the logo field (Warning 3), so shipping it in every default would draw a
// draggable box in the editor for a field that can never appear on the
// certificate — the defaults deliberately omit it.
func TestDefaultLayout_CoversAllClosedFieldIDs(t *testing.T) {
	want := []string{
		"title", "subtitle", "student_name", "exam_title",
		"completion_text", "date", "certificate_number",
	}
	for _, tmpl := range []string{"classic", "modern", "elegant"} {
		l := defaultLayout(tmpl)
		seen := make(map[string]bool, len(l.Fields))
		for _, f := range l.Fields {
			seen[f.ID] = true
		}
		for _, id := range want {
			if !seen[id] {
				t.Errorf("defaultLayout(%s) missing field id %q", tmpl, id)
			}
		}
		if seen["logo"] {
			t.Errorf("defaultLayout(%s) still ships a logo field, but renderCertificate never stamps it", tmpl)
		}
	}
}
