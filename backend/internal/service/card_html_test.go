package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

func baseCardRegistration() *model.RegistrationDetail {
	sched := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC) // 17:00 WIB
	detail := &model.RegistrationDetail{}
	detail.ExamRegistration = model.ExamRegistration{
		ID:     uuid.New(),
		Token:  "AB12CD34",
		Status: "registered",
	}
	detail.Exam.Title = "Ujian Simulasi UTBK Saintek"
	detail.Exam.ScheduledAt = &sched
	detail.Exam.RequiresCheckin = true
	mins := 30
	detail.Exam.CheckInWindowMinutes = &mins
	return detail
}

func TestCardScheduleText_FormatsAsiaJakarta(t *testing.T) {
	detail := baseCardRegistration()
	got := cardScheduleText(detail)
	want := "01 Aug 2026 17:00 WIB"
	if got != want {
		t.Errorf("cardScheduleText() = %q, want %q", got, want)
	}
}

func TestCardScheduleText_NilScheduledAt(t *testing.T) {
	detail := baseCardRegistration()
	detail.Exam.ScheduledAt = nil
	if got := cardScheduleText(detail); got != "-" {
		t.Errorf("cardScheduleText() with nil ScheduledAt = %q, want %q", got, "-")
	}
}

func TestNameInitials(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Saifullah Panca", "SP"},
		{"Budi", "B"},
		{"", ""},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := nameInitials(c.name); got != c.want {
			t.Errorf("nameInitials(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCardFooterNote_RequiresCheckinWithWindow(t *testing.T) {
	detail := baseCardRegistration()
	got := cardFooterNote(detail)
	want := "Harap check-in dalam waktu 30 menit sebelum ujian."
	if got != want {
		t.Errorf("cardFooterNote() = %q, want %q", got, want)
	}
}

func TestCardFooterNote_RequiresCheckinNoWindow(t *testing.T) {
	detail := baseCardRegistration()
	detail.Exam.CheckInWindowMinutes = nil
	got := cardFooterNote(detail)
	want := "Harap check-in sebelum ujian dimulai."
	if got != want {
		t.Errorf("cardFooterNote() = %q, want %q", got, want)
	}
}

func TestCardFooterNote_FreeAccess(t *testing.T) {
	detail := baseCardRegistration()
	detail.Exam.RequiresCheckin = false
	got := cardFooterNote(detail)
	want := "Akses bebas pada waktu yang ditentukan."
	if got != want {
		t.Errorf("cardFooterNote() = %q, want %q", got, want)
	}
}

func TestBuildCardHTML_EmbedsPageSizeTokenAndScheduleText(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "Saifullah Panca", "Akademi Bimbel", nil, nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	out := string(html)

	if !strings.Contains(out, "@page{size:148mm 105mm;margin:0;}") {
		t.Errorf("expected A6-landscape @page rule, got:\n%s", out)
	}
	if !strings.Contains(out, "AB12CD34") {
		t.Errorf("expected token to render verbatim, got:\n%s", out)
	}
	if !strings.Contains(out, "01 Aug 2026 17:00 WIB") {
		t.Errorf("expected formatted schedule text, got:\n%s", out)
	}
	if !strings.Contains(out, "Saifullah Panca") {
		t.Errorf("expected student name to render, got:\n%s", out)
	}
	if !strings.Contains(out, "Ujian Simulasi UTBK Saintek") {
		t.Errorf("expected exam title to render, got:\n%s", out)
	}
}

func TestBuildCardHTML_EmptyStudentNameFallsBackToDash(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "", "Akademi Bimbel", nil, nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	if !strings.Contains(string(html), `class="card-name">-<`) {
		t.Errorf("expected empty student name to fall back to \"-\", got:\n%s", html)
	}
}

func TestBuildCardHTML_NoPhotoFallsBackToInitials(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "Saifullah Panca", "Akademi Bimbel", nil, nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	out := string(html)
	if !strings.Contains(out, `card-photo-initials">SP<`) {
		t.Errorf("expected initials avatar fallback \"SP\", got:\n%s", out)
	}
	if strings.Contains(out, "card-photo\"") {
		t.Errorf("expected no <img class=\"card-photo\"> when photoImg is nil, got:\n%s", out)
	}
}

func TestBuildCardHTML_NoNameNoPhotoFallsBackToPlaceholder(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "", "Akademi Bimbel", nil, nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	if !strings.Contains(string(html), `class="card-photo-placeholder"`) {
		t.Errorf("expected neutral placeholder when no name and no photo, got:\n%s", html)
	}
}

func TestBuildCardHTML_ValidPhotoEmbedsDataURI(t *testing.T) {
	detail := baseCardRegistration()
	fakePhoto := fakePNGBytesForCard(t)
	html, err := buildCardHTML(detail, "Saifullah Panca", "Akademi Bimbel", nil, fakePhoto)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	out := string(html)
	if !strings.Contains(out, `<img class="card-photo" src="data:image/png;base64,`) {
		t.Errorf("expected a card-photo data URI, got:\n%s", out)
	}
	if strings.Contains(out, `class="card-photo-initials"`) {
		t.Errorf("expected no initials fallback when a valid photo is supplied, got:\n%s", out)
	}
}

func TestBuildCardHTML_CorruptPhotoFallsBackToInitials(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "Saifullah Panca", "Akademi Bimbel", nil, []byte("not an image"))
	if err != nil {
		t.Fatalf("buildCardHTML must not fail for corrupt photo bytes: %v", err)
	}
	if !strings.Contains(string(html), `class="card-photo-initials"`) {
		t.Errorf("expected initials fallback for corrupt photo bytes, got:\n%s", html)
	}
}

func TestBuildCardHTML_ValidLogoEmbedsDataURI(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "Saifullah Panca", "Akademi Bimbel", fakePNGBytesForCard(t), nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	if !strings.Contains(string(html), `<img class="card-logo" src="data:image/png;base64,`) {
		t.Errorf("expected a card-logo data URI, got:\n%s", html)
	}
}

func TestBuildCardHTML_NoLogoOmitsLogoFrame(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "Saifullah Panca", "Akademi Bimbel", nil, nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	if strings.Contains(string(html), `class="card-logo-frame"`) {
		t.Errorf("expected no logo frame when logoImg is nil, got:\n%s", html)
	}
}

func TestBuildCardHTML_ExamTitleIsEscaped(t *testing.T) {
	detail := baseCardRegistration()
	detail.Exam.Title = `<script>alert(1)</script>`
	html, err := buildCardHTML(detail, "Saifullah Panca", "Akademi Bimbel", nil, nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	out := string(html)
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Errorf("expected exam title to be HTML-escaped, got raw script tag in:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected escaped exam title entity, got:\n%s", out)
	}
}

func TestBuildCardHTML_NoTenantNameOmitsTenantLine(t *testing.T) {
	detail := baseCardRegistration()
	html, err := buildCardHTML(detail, "Saifullah Panca", "", nil, nil)
	if err != nil {
		t.Fatalf("buildCardHTML: %v", err)
	}
	if strings.Contains(string(html), `class="card-header-tenant"`) {
		t.Errorf("expected no tenant line when tenantName is empty, got:\n%s", html)
	}
}

// fakePNGBytesForCard is a minimal valid PNG, self-contained so this file has
// no compile-time dependency on pdf_test.go (deleted in Task 13).
func fakePNGBytesForCard(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 160, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
