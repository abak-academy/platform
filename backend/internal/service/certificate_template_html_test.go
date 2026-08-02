package service

// Finding 4 (external review of PR #69, 2026-08 review): the gate is
// decided on the validated Layout, but AdminUpdateExamCertificateDesign
// persisted req.TemplateHTML with zero validation of its own — a template
// could carry a {{score}} token (or an external resource reference) the
// layout never declared, bypassing the result gate entirely. These tests
// cover ValidateCertificateTemplateHTML (the save-time rejection) and
// certificateIsSensitive (the defense-in-depth re-check at gate time, in
// case a bad template ever lands in the DB some other way).

import (
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func classicLayoutNoScore() Layout {
	return normalizeCertificateLayout(defaultLayout("classic"))
}

func TestValidateCertificateTemplateHTML_EmptyIsNoOp(t *testing.T) {
	got, err := ValidateCertificateTemplateHTML("", classicLayoutNoScore())
	if err != nil || got != "" {
		t.Fatalf("got = %q, err = %v; want empty/no error", got, err)
	}
}

// TestValidateCertificateTemplateHTML_RejectsUndeclaredToken is the exact
// Finding 4 scenario: a template with a literal {{score}} spot on a layout
// that declares no score field must be rejected, not silently accepted.
func TestValidateCertificateTemplateHTML_RejectsUndeclaredToken(t *testing.T) {
	_, err := ValidateCertificateTemplateHTML(`<div>{{score}}</div>`, classicLayoutNoScore())
	if err == nil || !strings.Contains(err.Error(), "{{score}}") {
		t.Fatalf("err = %v, want a rejection naming the undeclared {{score}} token", err)
	}
}

func TestValidateCertificateTemplateHTML_AcceptsTokenDeclaredByLayout(t *testing.T) {
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{
		{ID: "score", Kind: "text", Content: "{{score}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
	}}
	got, err := ValidateCertificateTemplateHTML(`<div>{{score}}</div>`, layout)
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if !strings.Contains(got, "{{score}}") {
		t.Fatalf("sanitized output dropped the declared token: %q", got)
	}
}

// TestValidateCertificateTemplateHTML_AcceptsImplicitImageTokens proves the
// two worker-injected tokens (certificate_background_url, and
// certificate_asset_<fieldID> for an image field with an asset key) are
// always allowed even though no layout Content string ever names them
// literally — they come from LayoutField/Background, not Content.
func TestValidateCertificateTemplateHTML_AcceptsImplicitImageTokens(t *testing.T) {
	key := "certificates/exam/logo.png"
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{
		{ID: "logo", Kind: "image", XMm: 10, YMm: 10, WMm: 30, HMm: 20, Visible: true, AssetKey: &key},
	}}
	html := `<img src="{{certificate_background_url}}"/><img src="{{certificate_asset_logo}}"/>`
	got, err := ValidateCertificateTemplateHTML(html, layout)
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if !strings.Contains(got, "certificate_asset_logo") {
		t.Fatalf("sanitized output dropped the implicit asset token: %q", got)
	}
}

func TestValidateCertificateTemplateHTML_RejectsAbsoluteHTTPSrc(t *testing.T) {
	_, err := ValidateCertificateTemplateHTML(`<img src="https://evil.example/x.png"/>`, classicLayoutNoScore())
	if err == nil {
		t.Fatal("want rejection for an absolute external image src")
	}
}

// TestValidateCertificateTemplateHTML_RejectsProtocolRelativeImgSrc uses
// <img> (an allowlisted element, unlike <link>/<script>, which the sanitizer
// already drops outright) so the external-resource check itself — not the
// element allowlist — is what's under test.
func TestValidateCertificateTemplateHTML_RejectsProtocolRelativeImgSrc(t *testing.T) {
	_, err := ValidateCertificateTemplateHTML(`<img src="//evil.example/x.png"/>`, classicLayoutNoScore())
	if err == nil {
		t.Fatal("want rejection for a protocol-relative external image src")
	}
}

// TestValidateCertificateTemplateHTML_DisallowedElementIsStrippedNotRejected
// documents that <link> (never allowlisted) is silently dropped by the
// sanitizer rather than triggering the external-resource rejection — the
// tag never survives to be scanned. Still a closed door, just via the
// allowlist instead of the regex.
func TestValidateCertificateTemplateHTML_DisallowedElementIsStrippedNotRejected(t *testing.T) {
	got, err := ValidateCertificateTemplateHTML(`<link href="//evil.example/x.css"/><div>{{student_name}}</div>`, Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if strings.Contains(got, "evil.example") {
		t.Fatalf("sanitized output still carries the disallowed <link>: %q", got)
	}
}

func TestValidateCertificateTemplateHTML_RejectsExternalURLInStyle(t *testing.T) {
	html := `<html><head><style>body{background:url(https://evil.example/track.gif)}</style></head><body></body></html>`
	_, err := ValidateCertificateTemplateHTML(html, classicLayoutNoScore())
	if err == nil {
		t.Fatal("want rejection for an external url() inside a <style> block")
	}
}

// TestValidateCertificateTemplateHTML_PreservesDataURIFont proves the check
// doesn't false-positive on the FE's real output shape: an embedded
// base64 font in a @font-face src, which can coincidentally contain "//" in
// its payload and must not trip the external-resource rejection.
func TestValidateCertificateTemplateHTML_PreservesDataURIFont(t *testing.T) {
	html := `<html><head><style>@font-face{font-family:"x";src:url(data:font/ttf;base64,AAAA//BBBB==) format("truetype");}</style></head><body><div>{{student_name}}</div></body></html>`
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{
		{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
	}}
	got, err := ValidateCertificateTemplateHTML(html, layout)
	if err != nil {
		t.Fatalf("unexpected rejection of a self-contained data: URI font: %v", err)
	}
	if !strings.Contains(got, "base64,AAAA//BBBB==") {
		t.Fatalf("sanitizer mangled the embedded font: %q", got)
	}
}

// TestValidateCertificateTemplateHTML_StripsScriptPreservesStyle proves the
// document-shaped sanitizer's core trade-off against bluemonday's default
// behavior: <style> content (load-bearing — @page sizing, @font-face) must
// survive, while <script> (and its content, and on*-handlers) must not.
func TestValidateCertificateTemplateHTML_StripsScriptPreservesStyle(t *testing.T) {
	html := `<html><head><style>@page{size:297mm 210mm;margin:0}</style></head>` +
		`<body onload="evil()"><script>alert(document.cookie)</script><div>{{student_name}}</div></body></html>`
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{
		{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
	}}
	got, err := ValidateCertificateTemplateHTML(html, layout)
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if !strings.Contains(got, "@page{size:297mm 210mm;margin:0}") {
		t.Fatalf("sanitizer stripped load-bearing <style> content: %q", got)
	}
	if strings.Contains(got, "<script") || strings.Contains(got, "alert(document.cookie)") {
		t.Fatalf("sanitizer let <script> survive: %q", got)
	}
	if strings.Contains(got, "onload") {
		t.Fatalf("sanitizer let an on*-handler survive: %q", got)
	}
}

func TestValidateCertificateTemplateHTML_PrependsDoctype(t *testing.T) {
	got, err := ValidateCertificateTemplateHTML(`<html><body>{{student_name}}</body></html>`, Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if !strings.HasPrefix(got, "<!DOCTYPE html>") {
		t.Fatalf("got = %q, want a leading <!DOCTYPE html> (bluemonday drops any original doctype unconditionally)", got)
	}
}

// ---------- SSRF via CSS @import (2026-08 review, Finding B) ----------
//
// certificateTemplateHasExternalResource used to match only src=/href=/
// url(...) — @import's bare-string form ("@import \"http://...\";", no
// url() wrapper) slipped through it entirely, and <style> content is
// deliberately preserved (certificateTemplateDocumentPolicy), so this
// reached Gotenberg's Chromium, which runs inside the compose network and
// can reach internal hosts. The fix inverted the check (strip data: URIs,
// then reject anything shaped like an absolute/protocol-relative URL) so
// it isn't just this one syntax patched onto the old blacklist.

func TestValidateCertificateTemplateHTML_RejectsAtImportBareString(t *testing.T) {
	html := `<html><head><style>@import "http://169.254.169.254/latest/meta-data";</style></head><body>{{student_name}}</body></html>`
	_, err := ValidateCertificateTemplateHTML(html, Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
		},
	})
	if err == nil {
		t.Fatal("want rejection for @import with a bare http:// string (no url() wrapper)")
	}
}

func TestValidateCertificateTemplateHTML_RejectsAtImportURLFunction(t *testing.T) {
	html := `<html><head><style>@import url("http://internal-service/x.css");</style></head><body>{{student_name}}</body></html>`
	_, err := ValidateCertificateTemplateHTML(html, Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
		},
	})
	if err == nil {
		t.Fatal("want rejection for @import url(\"http://...\")")
	}
}

func TestValidateCertificateTemplateHTML_RejectsFontFaceExternalSrc(t *testing.T) {
	html := `<html><head><style>@font-face{font-family:"x";src:url(http://evil.example/font.ttf) format("truetype");}</style></head><body>{{student_name}}</body></html>`
	_, err := ValidateCertificateTemplateHTML(html, Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
		},
	})
	if err == nil {
		t.Fatal("want rejection for @font-face src referencing an external http(s) URL")
	}
}

func TestValidateCertificateTemplateHTML_RejectsProtocolRelativeInStyle(t *testing.T) {
	html := `<html><head><style>@import "//evil.example/x.css";</style></head><body>{{student_name}}</body></html>`
	_, err := ValidateCertificateTemplateHTML(html, Layout{
		Page: Page{WidthMm: 297, HeightMm: 210},
		Fields: []LayoutField{
			{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
		},
	})
	if err == nil {
		t.Fatal("want rejection for a protocol-relative @import")
	}
}

// TestValidateCertificateTemplateHTML_LegitimateTemplatePasses is the
// companion positive case: a template using only data: URIs and
// {{certificate_*}} tokens — the two legitimate ways to reference an
// asset — must still be accepted by the inverted check.
func TestValidateCertificateTemplateHTML_LegitimateTemplatePasses(t *testing.T) {
	key := "certificates/exam/logo.png"
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{
		{ID: "student_name", Kind: "text", Content: "{{student_name}}", XMm: 10, YMm: 10, WMm: 50, Visible: true},
		{ID: "logo", Kind: "image", XMm: 10, YMm: 10, WMm: 30, HMm: 20, Visible: true, AssetKey: &key},
	}}
	html := `<html><head><style>` +
		`@font-face{font-family:"x";src:url(data:font/ttf;base64,AAAA//BBBB==) format("truetype");}` +
		`</style></head><body>` +
		`<div>{{student_name}}</div>` +
		`<img src="{{certificate_background_url}}"/>` +
		`<img src="{{certificate_asset_logo}}"/>` +
		`</body></html>`
	got, err := ValidateCertificateTemplateHTML(html, layout)
	if err != nil {
		t.Fatalf("unexpected rejection of a legitimate data:/{{}}-only template: %v", err)
	}
	if !strings.Contains(got, "base64,AAAA//BBBB==") || !strings.Contains(got, "{{certificate_asset_logo}}") {
		t.Fatalf("sanitizer mangled a legitimate template: %q", got)
	}
}

// ---------- defense in depth: certificateIsSensitive / certificateLayoutAllowed ----------

// TestCertificateIsSensitive_TemplateTokenNotInLayout_StillDetected proves
// the "if it somehow persisted anyway" half of Finding 4: even when the
// layout alone looks non-sensitive, a {{score}}-carrying template (e.g.
// written directly to the DB, bypassing ValidateCertificateTemplateHTML)
// must still be treated as sensitive.
func TestCertificateIsSensitive_TemplateTokenNotInLayout_StillDetected(t *testing.T) {
	layout := classicLayoutNoScore()
	html := `<div>{{score}}</div>`
	exam := model.Exam{CertificateTemplateHTML: &html}

	if certificateIsSensitive(exam, layout) == false {
		t.Fatal("want certificateIsSensitive to detect a sensitive token carried only by the template")
	}
}

func TestCertificateIsSensitive_NilTemplate_FallsBackToLayoutOnly(t *testing.T) {
	if certificateIsSensitive(model.Exam{}, classicLayoutNoScore()) {
		t.Fatal("a nil template and a non-score layout should not be sensitive")
	}
}

// TestCertificateLayoutAllowed_TemplateOnlyScoreToken_StillGated is Finding
// 4's exact "somehow persisted anyway" defence-in-depth case, driven through
// certificateLayoutAllowed itself: a layout that declares no score field but
// an exam whose stored template does must still be gated by result_config.
func TestCertificateLayoutAllowed_TemplateOnlyScoreToken_StillGated(t *testing.T) {
	sess := &model.ExamSession{Status: "submitted"}
	layout := classicLayoutNoScore()
	html := `<div>{{score}}</div>`
	exam := model.Exam{ResultConfig: "hidden", CertificateTemplateHTML: &html}

	if certificateLayoutAllowed(exam, sess, layout, nil, nil) {
		t.Fatal("a template-only score token must still be gated when the layout looks clean")
	}
}
