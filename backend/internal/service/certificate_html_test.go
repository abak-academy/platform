package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	htmlutil "html"
	"image"
	"image/jpeg"
	"regexp"
	"strings"
	"testing"
	"time"
)

func testCertificateLayout() Layout {
	return Layout{
		Page:       Page{WidthMm: certificatePageWidthMm, HeightMm: certificatePageHeightMm},
		Background: Background{Kind: "builtin", Ref: "classic"},
		Fields: []LayoutField{
			{ID: "title", XMm: 48.5, YMm: 66, WMm: 200, Align: "center", Font: "cinzel", Weight: "bold", SizePt: 25, Color: "#22315B", Visible: true},
			{ID: "student_name", XMm: 48.5, YMm: 108, WMm: 200, Align: "left", Font: "cormorant_garamond", Weight: "bold", SizePt: 40, Color: "#22315B", Visible: true},
			{ID: "date", XMm: 48.5, YMm: 166, WMm: 200, Align: "right", Font: "public_sans", Weight: "regular", SizePt: 11, Color: "#4A5568", Visible: true},
			// Malformed color + unknown font id to exercise FR-9 fallback.
			{ID: "certificate_number", XMm: 48.5, YMm: 193, WMm: 200, Align: "center", Font: "comic_sans_ms", Weight: "regular", SizePt: 9, Color: "not-a-color", Visible: true},
			// Not visible: must not appear in output at all.
			{ID: "subtitle", XMm: 48.5, YMm: 90, WMm: 200, Align: "center", Font: "public_sans", Weight: "regular", SizePt: 12, Color: "#4A5568", Visible: false},
			// Visible signature field but no image bytes supplied: must be skipped.
			{ID: "signature", XMm: 205, YMm: 150, WMm: 62, HMm: 22, Align: "center", Visible: true},
			// Visible logo field with image bytes supplied: must render.
			{ID: "logo", XMm: 10, YMm: 10, WMm: 30, HMm: 30, Align: "center", Visible: true},
		},
	}
}

func jakartaDateStr(t *testing.T) string {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load Asia/Jakarta: %v", err)
	}
	return time.Date(2026, 7, 21, 10, 0, 0, 0, loc).Format("2 January 2006")
}

func TestBuildCertificateHTML_PageSizeAndFieldGeometry(t *testing.T) {
	layout := testCertificateLayout()
	dateStr := jakartaDateStr(t)
	vals := certificateFieldValues("Ujian Matematika", "Budi Santoso", dateStr, "ABK/2026/0042/000005")
	bg := []byte("fake-png-bytes")
	logoBytes := []byte("fake-logo-bytes")

	out, err := buildCertificateHTML(layout, vals, bg, map[FieldID][]byte{"logo": logoBytes})
	if err != nil {
		t.Fatalf("buildCertificateHTML returned error: %v", err)
	}
	html := string(out)

	if !strings.Contains(html, "@page{size:297mm 210mm;margin:0;}") {
		t.Errorf("expected @page rule with 297mm 210mm size, got:\n%s", html)
	}

	// student_name: left align, correct mm geometry, color, size.
	if !strings.Contains(html, "left:48.5mm;top:108mm;width:200mm;text-align:left;color:#22315B;font-size:40pt;font-family:cormorant_garamond;font-weight:bold;") {
		t.Errorf("student_name field style not found as expected, got:\n%s", html)
	}
	if !strings.Contains(html, ">Budi Santoso<") {
		t.Errorf("expected student name value in output, got:\n%s", html)
	}

	// date: right align, correct geometry.
	if !strings.Contains(html, "left:48.5mm;top:166mm;width:200mm;text-align:right;color:#4A5568;font-size:11pt;font-family:public_sans;font-weight:normal;") {
		t.Errorf("date field style not found as expected, got:\n%s", html)
	}
	if !strings.Contains(html, ">"+dateStr+"<") {
		t.Errorf("expected Asia/Jakarta date string %q in output, got:\n%s", dateStr, html)
	}

	// certificate_number value present.
	if !strings.Contains(html, ">ABK/2026/0042/000005<") {
		t.Errorf("expected certificate number value in output, got:\n%s", html)
	}
}

func TestBuildCertificateHTML_UnknownFontAndMalformedColorFallback(t *testing.T) {
	layout := testCertificateLayout()
	dateStr := jakartaDateStr(t)
	vals := certificateFieldValues("Ujian Matematika", "Budi Santoso", dateStr, "ABK/2026/0042/000005")

	out, err := buildCertificateHTML(layout, vals, []byte("bg"), nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML returned error: %v", err)
	}
	html := string(out)

	// certificate_number has font "comic_sans_ms" (unknown) and color
	// "not-a-color" (malformed) -> must fall back to source_serif_4 / black.
	if !strings.Contains(html, "font-family:source_serif_4") {
		t.Errorf("expected unknown font to fall back to source_serif_4, got:\n%s", html)
	}
	if strings.Contains(html, "comic_sans_ms") {
		t.Errorf("unknown font family must not leak into output, got:\n%s", html)
	}
	if !strings.Contains(html, "color:#000000") {
		t.Errorf("expected malformed color to fall back to #000000, got:\n%s", html)
	}
	if strings.Contains(html, "color:not-a-color") {
		t.Errorf("malformed color must not leak into output, got:\n%s", html)
	}
}

func TestBuildCertificateHTML_SkipsInvisibleAndImagelessFields(t *testing.T) {
	layout := testCertificateLayout()
	dateStr := jakartaDateStr(t)
	vals := certificateFieldValues("Ujian Matematika", "Budi Santoso", dateStr, "ABK/2026/0042/000005")

	out, err := buildCertificateHTML(layout, vals, []byte("bg"), map[FieldID][]byte{"logo": []byte("logo-bytes")})
	if err != nil {
		t.Fatalf("buildCertificateHTML returned error: %v", err)
	}
	html := string(out)

	// subtitle is Visible:false -> its fixed copy must not appear.
	if strings.Contains(html, "This certificate is proudly awarded to") {
		t.Errorf("invisible subtitle field leaked into output:\n%s", html)
	}

	// signature is visible but no image bytes were supplied -> box geometry
	// (205mm/150mm) must not appear as a rendered field.
	if strings.Contains(html, "left:205mm;top:150mm") {
		t.Errorf("signature field with no image bytes must be skipped, got:\n%s", html)
	}

	// logo is visible and image bytes were supplied -> must render as data: img.
	wantLogoB64 := base64.StdEncoding.EncodeToString([]byte("logo-bytes"))
	if !strings.Contains(html, fmt.Sprintf(`src="data:image/png;base64,%s"`, wantLogoB64)) {
		t.Errorf("expected logo image data URI in output, got:\n%s", html)
	}
	if !strings.Contains(html, "left:10mm;top:10mm;width:30mm;height:30mm;object-fit:contain;") {
		t.Errorf("expected logo field geometry in output, got:\n%s", html)
	}
}

func TestBuildCertificateHTML_BackgroundIsDataURI(t *testing.T) {
	layout := testCertificateLayout()
	bg := []byte("distinctive-background-bytes")
	vals := certificateFieldValues("Ujian", "Nama", "1 Januari 2026", "ABK/2026/0001/000001")

	out, err := buildCertificateHTML(layout, vals, bg, nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML returned error: %v", err)
	}
	html := string(out)

	wantB64 := base64.StdEncoding.EncodeToString(bg)
	wantSrc := fmt.Sprintf(`src="data:image/png;base64,%s"`, wantB64)
	if !strings.Contains(html, wantSrc) {
		t.Errorf("expected background data URI %q in output, got:\n%s", wantSrc, html)
	}
	if !strings.Contains(html, `class="certificate-bg"`) {
		t.Errorf("expected certificate-bg element, got:\n%s", html)
	}
}

// The background picker accepts any image type, so a JPEG upload must be
// embedded as image/jpeg — it used to be hardcoded to image/png regardless.
func TestBuildCertificateHTML_JPEGBackgroundKeepsItsOwnMime(t *testing.T) {
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatalf("encode jpeg fixture: %v", err)
	}

	layout := testCertificateLayout()
	vals := certificateFieldValues("Ujian", "Nama", "1 Januari 2026", "ABK/2026/0001/000001")

	out, err := buildCertificateHTML(layout, vals, jpg.Bytes(), nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML returned error: %v", err)
	}
	html := string(out)

	if !strings.Contains(html, `class="certificate-bg" src="data:image/jpeg;base64,`) {
		t.Errorf("expected a jpeg-typed background data URI, got:\n%s", html)
	}
	if strings.Contains(html, `class="certificate-bg" src="data:image/png`) {
		t.Error("jpeg background was mislabeled as image/png")
	}

	// The payload must still be the exact upload once attribute escaping is
	// undone, the way a real HTML parser does before decoding (see dataURIBytes).
	re := regexp.MustCompile(`class="certificate-bg" src="data:image/jpeg;base64,([^"]*)"`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		t.Fatal("could not extract the background data URI payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(htmlutil.UnescapeString(m[1]))
	if err != nil {
		t.Fatalf("decode background base64: %v", err)
	}
	if !bytes.Equal(decoded, jpg.Bytes()) {
		t.Error("embedded background bytes do not match the uploaded jpeg")
	}
}

func TestBuildCertificateHTML_ExactlySixFontFaceFamilies(t *testing.T) {
	layout := testCertificateLayout()
	vals := certificateFieldValues("Ujian", "Nama", "1 Januari 2026", "ABK/2026/0001/000001")

	out, err := buildCertificateHTML(layout, vals, []byte("bg"), nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML returned error: %v", err)
	}
	html := string(out)

	re := regexp.MustCompile(`@font-face\{font-family:([a-z0-9_]+);`)
	matches := re.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		t.Fatalf("expected at least one @font-face rule, got:\n%s", html)
	}

	families := map[string]bool{}
	for _, m := range matches {
		families[m[1]] = true
	}
	if len(families) != 6 {
		t.Errorf("expected exactly 6 distinct @font-face families, got %d: %v", len(families), families)
	}

	wantFamilies := []string{
		FontSourceSerif4, FontPublicSans, FontCinzel,
		FontPlayfairDisplay, FontCormorantGaramond, FontGreatVibes,
	}
	for _, f := range wantFamilies {
		if !families[f] {
			t.Errorf("expected font family %q among @font-face rules, got: %v", f, families)
		}
	}

	// data: base64 TTF src present for each face.
	if !strings.Contains(html, "src:url(data:font/ttf;base64,") {
		t.Errorf("expected base64 data: font src, got:\n%s", html)
	}
}

func TestBuildCertificateHTML_EscapesFieldValues(t *testing.T) {
	layout := Layout{
		Page: Page{WidthMm: certificatePageWidthMm, HeightMm: certificatePageHeightMm},
		Fields: []LayoutField{
			{ID: "student_name", XMm: 10, YMm: 10, WMm: 100, Align: "center", Font: "public_sans", SizePt: 20, Color: "#000000", Visible: true},
		},
	}
	vals := map[FieldID]string{"student_name": `<script>alert("x")</script>`}

	out, err := buildCertificateHTML(layout, vals, []byte("bg"), nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML returned error: %v", err)
	}
	html := string(out)

	if strings.Contains(html, `<script>alert("x")</script>`) {
		t.Errorf("expected field value to be HTML-escaped, got raw script tag:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag in output, got:\n%s", html)
	}
}
