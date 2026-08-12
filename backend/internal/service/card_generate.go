package service

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html"
	"strings"
)

// examCardTemplateHTML is the exam card's self-contained {{token}} HTML
// template (async redesign 2026-08-02, orchestrator decision on the "no
// design studio" question): the exam card has exactly one template, so
// instead of a per-exam admin-authored design (like the certificate) there
// is exactly one static build-time artifact, generated from
// web/components/exam/ExamCardPrintable.tsx — the SAME component the
// on-screen student card renders — by web/scripts/build-card-template.mjs
// (`npm run build:card-template`). Re-run that script and commit the
// regenerated file whenever ExamCardPrintable.tsx or its CSS module changes;
// nothing here re-derives it automatically.
//
//go:embed assets/exam_card_template.html
var examCardTemplateHTML string

// examCardLogoFallback/examCardPhotoFallback are the exact SVG markup
// ExamCardPrintable.tsx itself renders when tenantLogoUrl/photoUrl is empty
// (AbakMarkFull, PhotoPlaceholder) — extracted by the same build script so a
// missing logo/photo in a generated PDF looks identical to the on-screen
// card's own missing-value state, not a hand-rewritten substitute.
//
//go:embed assets/exam_card_logo_fallback.svg
var examCardLogoFallback string

//go:embed assets/exam_card_photo_fallback.svg
var examCardPhotoFallback string

func svgDataURI(svg string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// buildExamCardHTML substitutes GetCardPrintData's server-authored values
// into the static template. photo_url/tenant_logo_url always resolve to a
// loadable image src — the on-screen component's placeholder branch
// (AbakMarkFull/PhotoPlaceholder) is baked into the template as an always-
// present <img>, so an empty value here falls back to the matching
// pre-rendered SVG rather than leaving a broken src.
func buildExamCardHTML(data *CardPrintData) string {
	tenantName := data.TenantName
	if tenantName == "" {
		tenantName = "Abak Academy"
	}
	tenantLogoURL := data.TenantLogoURL
	if tenantLogoURL == "" {
		tenantLogoURL = svgDataURI(examCardLogoFallback)
	}
	photoURL := data.PhotoURL
	if photoURL == "" {
		photoURL = svgDataURI(examCardPhotoFallback)
	}

	helpText := data.HelpURL
	if helpText == "" {
		helpText = data.ContactEmail
	}

	values := map[FieldID]string{
		"participant_no":  data.ParticipantNo,
		"student_name":    data.StudentName,
		"school":          data.School,
		"exam_title":      data.ExamTitle,
		"exam_schedule":   data.ExamSchedule,
		"check_in_code":   data.CheckInCode,
		"tenant_name":     tenantName,
		"tenant_logo_url": tenantLogoURL,
		"photo_url":       photoURL,
		"contact_phone":   dashIfEmpty(data.ContactPhone),
		"help_url":        dashIfEmpty(helpText),
		"social_handle":   data.SocialHandle,
		// notes_html is markup, but substituteTemplateTokens escapes every value
		// it substitutes (and blanks any token missing from this map). Stand a
		// sentinel in its place and splice the <li> list in afterwards; the note
		// text is escaped by cardNotesHTML, only its tags are not.
		"notes_html": cardNotesSentinel,
	}
	out := substituteTemplateTokens(examCardTemplateHTML, values)
	return strings.ReplaceAll(out, cardNotesSentinel, cardNotesHTML(data.Notes, data.FooterNote))
}

// cardNotesSentinel survives HTML escaping unchanged (no <, >, &, or quotes).
const cardNotesSentinel = "CARD_NOTES_SLOT_7f3a2b"

func dashIfEmpty(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

// cardNotesHTML fills the template's single {{notes_html}} slot with the whole
// Perhatian list — the admin's notes followed by the generated check-in
// bullet. Escaped, since notes are admin-authored free text.
func cardNotesHTML(notes []string, footerNote string) string {
	var b strings.Builder
	for _, n := range notes {
		b.WriteString("<li>" + html.EscapeString(n) + "</li>")
	}
	if footerNote != "" {
		b.WriteString("<li>" + html.EscapeString(footerNote) + "</li>")
	}
	return b.String()
}

// generateExamCardPDF computes the exam card's print-data (GetCardPrintData,
// unchanged — still the single server-authored source of every displayed
// value) and renders it through Gotenberg's RenderHTML against the static
// template — no certificate/card HTML is fetched by Gotenberg, only posted
// directly (FR-30 lineage). Kept as a small free function, not a Service
// method, so it's testable without a real renderer injected into a full
// Service (mirrors renderShippingLabel's split in shipping_label_html.go).
func generateExamCardPDF(ctx context.Context, renderer pdfGenerator, data *CardPrintData) ([]byte, error) {
	html := buildExamCardHTML(data)
	pdf, err := renderer.RenderHTML(ctx, []byte(html))
	if err != nil {
		return nil, fmt.Errorf("render exam card pdf: %w", err)
	}
	return pdf, nil
}
