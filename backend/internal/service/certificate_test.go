package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	htmlutil "html"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// dataURIBytes extracts and decodes a data:image/png;base64,... URI from the
// given element (matched by its distinguishing style/class prefix), undoing
// html/template's attribute escaping (e.g. '+' -> "&#43;") the same way a real
// HTML parser (Chromium, via Gotenberg) would before the image bytes ever
// matter — a raw literal-base64 substring match is brittle against real binary
// asset bytes, which frequently contain '+' and so get entity-escaped.
func dataURIBytes(t *testing.T, htmlOut, elementPrefix string) []byte {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(elementPrefix) + `[^>]*src="data:image/png;base64,([^"]*)"`)
	m := re.FindStringSubmatch(htmlOut)
	if m == nil {
		t.Fatalf("no %q data URI found in output:\n%s", elementPrefix, htmlOut)
	}
	decoded, err := base64.StdEncoding.DecodeString(htmlutil.UnescapeString(m[1]))
	if err != nil {
		t.Fatalf("decode data URI base64: %v", err)
	}
	return decoded
}

// certDesignJSON builds a *json.RawMessage certificate_design blob naming only
// a template — the test-era equivalent of the pre-Task-8 CertificateTemplate
// column, for tests that don't care about background/layout.
func certDesignJSON(template string) *json.RawMessage {
	raw := json.RawMessage(`{"template":"` + template + `"}`)
	return &raw
}

// ---------- fakeSessionRepo: certificate extensions ----------

func (f *fakeSessionRepo) UpdateSessionCertificate(_ context.Context, sessionID uuid.UUID, key string, generatedAt time.Time) error {
	s, ok := f.sessions[sessionID]
	if !ok {
		return repository.ErrNotFound
	}
	s.CertificateKey = &key
	s.CertificateGeneratedAt = &generatedAt
	return nil
}

// AllocateCertificateNumber fakes the repository's idempotent allocation
// (FR-9/FR-10): mints once per session, keyed off the session id so it needs
// no extra counter state on fakeSessionRepo, and returns the same value on
// every later call for that session.
func (f *fakeSessionRepo) AllocateCertificateNumber(_ context.Context, sessionID uuid.UUID) (string, error) {
	s, ok := f.sessions[sessionID]
	if !ok {
		return "", repository.ErrNotFound
	}
	if s.CertificateNumber != nil {
		return *s.CertificateNumber, nil
	}
	number := "ABK/2026/" + sessionID.String()[:6]
	s.CertificateNumber = &number
	return number, nil
}

// ---------- shimSessionService: certificate shim ----------

func (s *shimSessionService) uploadCertificatePDF(_ context.Context, sessionID uuid.UUID, _ []byte) (string, error) {
	s.uploadCertCalls++
	if s.uploadCertErr != nil {
		return "", s.uploadCertErr
	}
	return "http://minio.example.com/certificates/" + sessionID.String() + ".pdf", nil
}

// downloadCertificateBackground fakes the private-bucket download for a custom
// background: returns a real embedded PNG (the classic built-in bytes stand in
// for "whatever was uploaded") so buildCertificateHTML can embed it for real.
func (s *shimSessionService) downloadCertificateBackground(_ context.Context, _ string) ([]byte, error) {
	return certBgClassicPNG, nil
}

// resolveCertificateBackground mirrors the real Service.resolveCertificateBackground:
// built-in templates use the embedded asset; "custom" downloads by key, or falls
// back to classic when the key is NULL (FR-19).
func (s *shimSessionService) resolveCertificateBackground(ctx context.Context, exam *model.Exam) ([]byte, error) {
	tmpl := certificateTemplate(exam)
	if tmpl == "custom" {
		if key := certificateBackgroundKey(exam); key != nil {
			return s.downloadCertificateBackground(ctx, *key)
		}
		return builtinCertificateBackground("classic"), nil
	}
	return builtinCertificateBackground(tmpl), nil
}

// resolveCertificateURL mirrors the real Service.resolveCertificateURL using the fake repo
// and fake I/O boundaries — follows the shimSessionService convention from
// exam_session_test.go / exam_result_test.go. resolveCertificateLayout and
// buildCertificateHTML are pure package functions, so this calls them for real
// rather than faking them.
func (s *shimSessionService) resolveCertificateURL(ctx context.Context, exam *model.Exam, sess *model.ExamSession, answers []model.ExamSessionAnswer, studentName string) (*string, error) {
	if sess.Status != "submitted" {
		return nil, nil
	}

	gradedAt := latestGradedAt(answers)
	designStale := exam.CertificateDesignUpdatedAt != nil && sess.CertificateGeneratedAt != nil &&
		exam.CertificateDesignUpdatedAt.After(*sess.CertificateGeneratedAt)

	if sess.CertificateKey == nil || sess.CertificateGeneratedAt == nil ||
		(gradedAt != nil && gradedAt.After(*sess.CertificateGeneratedAt)) || designStale {

		number, err := s.repo.AllocateCertificateNumber(ctx, sess.ID)
		if err != nil {
			return nil, err
		}
		layout, err := resolveCertificateLayout(exam)
		if err != nil {
			return nil, err
		}
		bg, err := s.resolveCertificateBackground(ctx, exam)
		if err != nil {
			return nil, err
		}

		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			return nil, err
		}
		dateStr := sess.SubmittedAt.In(loc).Format("2 January 2006")
		vals := certificateFieldValues(exam.Title, studentName, dateStr, number)

		// This shim exercises the plumbing (staleness/allocation/persist), not the
		// real HTML->PDF conversion, so buildCertificateHTML's output stands in
		// directly for the renderer's PDF bytes (mirrors the real Service passing
		// buildCertificateHTML's output through s.renderer.RenderHTML).
		pdf, err := buildCertificateHTML(layout, vals, bg, nil)
		if err != nil {
			return nil, err
		}
		key, err := s.uploadCertificatePDF(ctx, sess.ID, pdf)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		if err := s.repo.UpdateSessionCertificate(ctx, sess.ID, key, now); err != nil {
			return nil, err
		}
		sess.CertificateNumber = &number
		return &key, nil
	}

	return sess.CertificateKey, nil
}

// GetCertificatePreview mirrors the real Service.GetCertificatePreview: no
// allocation (FR-12), placeholder name/number, same background/layout
// resolution as real generation.
func (s *shimSessionService) GetCertificatePreview(ctx context.Context, examID uuid.UUID, templateOverride string) ([]byte, error) {
	exam, err := s.repo.GetExamForSession(ctx, examID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}

	storedTmpl := certificateTemplate(exam)
	tmpl := templateOverride
	if tmpl == "" {
		tmpl = storedTmpl
	}
	if err := validateCertificateTemplate(tmpl); err != nil {
		return nil, err
	}

	previewExam := *exam
	if templateOverride != "" && templateOverride != storedTmpl {
		raw, err := marshalCertificateDesign(certificateDesign{Template: tmpl})
		if err != nil {
			return nil, err
		}
		previewExam.CertificateDesign = raw
	}

	layout, err := resolveCertificateLayout(&previewExam)
	if err != nil {
		return nil, err
	}
	bg, err := s.resolveCertificateBackground(ctx, &previewExam)
	if err != nil {
		return nil, err
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return nil, err
	}
	vals := certificateFieldValues(exam.Title, previewStudentName, time.Now().In(loc).Format("2 January 2006"), previewCertificateNumber)

	return buildCertificateHTML(layout, vals, bg, nil)
}

// The preview placeholder must have the same shape as a real allocated number
// (repository.AllocateCertificateNumber: ABK/%04d/%04d/%06d), otherwise the
// admin lays the field out against a string narrower than the issued one.
func TestPreviewCertificateNumber_MatchesAllocatedShape(t *testing.T) {
	t.Parallel()
	if !regexp.MustCompile(`^ABK/\d{4}/\d{4}/\d{6}$`).MatchString(previewCertificateNumber) {
		t.Errorf("previewCertificateNumber = %q, want ABK/YYYY/NNNN/NNNNNN", previewCertificateNumber)
	}
}

// ---------- tests: latestGradedAt ----------

func TestLatestGradedAt_NilWhenEmpty(t *testing.T) {
	t.Parallel()
	got := latestGradedAt(nil)
	if got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestLatestGradedAt_NilWhenAllUngraded(t *testing.T) {
	t.Parallel()
	answers := []model.ExamSessionAnswer{
		{GradedAt: nil},
		{GradedAt: nil},
	}
	got := latestGradedAt(answers)
	if got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestLatestGradedAt_ReturnsMax(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	answers := []model.ExamSessionAnswer{
		{GradedAt: &t1},
		{GradedAt: nil},  // ungraded
		{GradedAt: &t2},  // latest
	}
	got := latestGradedAt(answers)
	if got == nil || !got.Equal(t2) {
		t.Errorf("want %v, got %v", t2, got)
	}
}

// ---------- tests: validateCertificateTemplate ----------

func TestValidateCertificateTemplate_ValidKeys(t *testing.T) {
	for _, k := range []string{"classic", "modern", "elegant"} {
		if err := validateCertificateTemplate(k); err != nil {
			t.Errorf("valid key %q: want nil, got %v", k, err)
		}
	}
}

func TestValidateCertificateTemplate_InvalidKey(t *testing.T) {
	err := validateCertificateTemplate("unknown")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

// ---------- tests: generateCertificatePDF (removed in Task 6 — PDF renderer gone) ----------
//
// generateCertificatePDF and the renderCertificate/renderCertificateWithImages
// PDF-rendering path it exercised were deleted in Task 6 (certificates now render via
// buildCertificateHTML + Gotenberg). Distinguishability across templates and
// long/non-ASCII names are re-asserted on the HTML pipeline below.
// TestValidateCertificateTemplate_InvalidKey above already covers the
// invalid-template validation error path directly.

// TestGenerateCertificatePDF_BuiltinsRenderOnePageWithBackground proves each
// builtin template renders a single A4-landscape page with its own embedded
// background — the HTML-pipeline equivalent of the deleted PDF-rendering
// rasterization assertion (one page, background filling the page).
func TestGenerateCertificatePDF_BuiltinsRenderOnePageWithBackground(t *testing.T) {
	for _, tmpl := range []string{"classic", "modern", "elegant"} {
		t.Run(tmpl, func(t *testing.T) {
			layout := defaultLayout(tmpl)
			bg := builtinCertificateBackground(tmpl)
			vals := certificateFieldValues("Ujian Matematika", "Budi Santoso", "21 Juli 2026", "ABK/2026/0001/000001")

			out, err := buildCertificateHTML(layout, vals, bg, nil)
			if err != nil {
				t.Fatalf("buildCertificateHTML(%s): unexpected error: %v", tmpl, err)
			}
			html := string(out)

			if !strings.Contains(html, "@page{size:297mm 210mm;margin:0;}") {
				t.Errorf("%s: expected a single A4-landscape @page rule, got:\n%s", tmpl, html)
			}
			if gotBg := dataURIBytes(t, html, `class="certificate-bg"`); !bytes.Equal(gotBg, bg) {
				t.Errorf("%s: embedded background bytes don't match its own built-in asset", tmpl)
			}
			if !strings.Contains(html, "Budi Santoso") {
				t.Errorf("%s: expected student name in output", tmpl)
			}
		})
	}
}

// TestGenerateCertificatePDF_DifferentTemplates_Distinguishable proves classic/
// modern/elegant produce different HTML (distinct layout/background/fonts) for
// the same field values — the regression guard for "all templates render
// identically" bugs.
func TestGenerateCertificatePDF_DifferentTemplates_Distinguishable(t *testing.T) {
	vals := certificateFieldValues("Ujian Matematika", "Budi Santoso", "21 Juli 2026", "ABK/2026/0001/000001")

	outputs := make(map[string]string, 3)
	for _, tmpl := range []string{"classic", "modern", "elegant"} {
		out, err := buildCertificateHTML(defaultLayout(tmpl), vals, builtinCertificateBackground(tmpl), nil)
		if err != nil {
			t.Fatalf("buildCertificateHTML(%s): unexpected error: %v", tmpl, err)
		}
		outputs[tmpl] = string(out)
	}

	if outputs["classic"] == outputs["modern"] || outputs["classic"] == outputs["elegant"] || outputs["modern"] == outputs["elegant"] {
		t.Error("distinct templates must render distinguishable HTML (different layout/background/fonts), got identical output")
	}
}

// TestGenerateCertificatePDF_LongAndNonASCIINames proves a very long name and a
// non-ASCII name both build without error and render verbatim — the fit script
// (FR-8) handles overflow client-side, so the Go builder itself must never
// reject or mangle these values.
func TestGenerateCertificatePDF_LongAndNonASCIINames(t *testing.T) {
	layout := defaultLayout("classic")
	bg := builtinCertificateBackground("classic")

	for _, name := range []string{
		"Muhammad Abdurrahman Al-Ghazali Bin Abdullah Wibisono Setiawan Nugroho",
		"Nguyễn Thị Phương Anh — 日本語テスト",
	} {
		vals := certificateFieldValues("Ujian Matematika", name, "21 Juli 2026", "ABK/2026/0001/000001")
		out, err := buildCertificateHTML(layout, vals, bg, nil)
		if err != nil {
			t.Fatalf("buildCertificateHTML: unexpected error for name %q: %v", name, err)
		}
		if !strings.Contains(string(out), name) {
			t.Errorf("expected long/non-ASCII name %q to render verbatim in output", name)
		}
	}
}

// ---------- tests: resolveCertificateURL ----------

func TestResolveCertificateURL_NotSubmitted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	sess := &model.ExamSession{
		ID: uuid.New(), Status: "in_progress",
		SubmittedAt: nil, CertificateKey: nil, CertificateGeneratedAt: nil,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "Test"}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != nil {
		t.Errorf("want nil for non-submitted session, got %q", *url)
	}
	// No side effects on an in_progress session.
	if svc.repo.sessions[sess.ID].CertificateKey != nil {
		t.Error("CertificateKey should remain nil")
	}
	if svc.uploadCertCalls != 0 {
		t.Errorf("non-submitted session must generate nothing, got %d upload calls", svc.uploadCertCalls)
	}
}

func TestResolveCertificateURL_FirstTimeGeneration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
		CertificateKey: nil, CertificateGeneratedAt: nil,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam"}
	wantURL := "http://minio.example.com/certificates/" + sess.ID.String() + ".pdf"

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == nil {
		t.Fatal("want non-nil URL for first-time generation")
	}
	if *url != wantURL {
		t.Errorf("URL: want %q, got %q", wantURL, *url)
	}
	// Session was updated.
	updated := svc.repo.sessions[sess.ID]
	if updated.CertificateKey == nil || *updated.CertificateKey != wantURL {
		t.Error("session CertificateKey should be set")
	}
	if updated.CertificateGeneratedAt == nil {
		t.Error("session CertificateGeneratedAt should be set")
	}
}

func TestResolveCertificateURL_NoRegenerationWhenNotStale(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	certGeneratedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	oldURL := "http://old.url/cert.pdf"
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
		CertificateKey: &oldURL, CertificateGeneratedAt: &certGeneratedAt,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam"}

	// Answers graded before the certificate was generated.
	gradedAt := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	answers := []model.ExamSessionAnswer{{GradedAt: &gradedAt}}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, answers, "Budi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == nil {
		t.Fatal("want non-nil URL (existing)")
	}
	if *url != oldURL {
		t.Errorf("want existing URL %q, got %q", oldURL, *url)
	}
	// No regeneration occurred — session fields unchanged.
	updated := svc.repo.sessions[sess.ID]
	if updated.CertificateKey == nil || *updated.CertificateKey != oldURL {
		t.Error("session CertificateKey should still be the old URL")
	}
}

func TestResolveCertificateURL_RegenerationWhenStale(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	certGeneratedAt := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	oldURL := "http://old.url/cert.pdf"
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
		CertificateKey: &oldURL, CertificateGeneratedAt: &certGeneratedAt,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam"}

	// Answer graded AFTER certificate was generated → stale → regen.
	gradedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	answers := []model.ExamSessionAnswer{{GradedAt: &gradedAt}}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, answers, "Budi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == nil {
		t.Fatal("want non-nil URL (regenerated)")
	}
	if *url == oldURL {
		t.Error("regeneration should produce a different URL")
	}
	if *url != "http://minio.example.com/certificates/"+sess.ID.String()+".pdf" {
		t.Errorf("unexpected URL: %q", *url)
	}
	// Session was updated.
	updated := svc.repo.sessions[sess.ID]
	if updated.CertificateKey == nil || *updated.CertificateKey == oldURL {
		t.Error("session CertificateKey should have been updated")
	}
	if updated.CertificateGeneratedAt == nil {
		t.Error("session CertificateGeneratedAt should be set")
	}
}

func TestResolveCertificateURL_UploadFailure_ReturnsError(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)
	svc.uploadCertErr = errors.New("minio down")

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
		CertificateKey: nil, CertificateGeneratedAt: nil,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam"}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi")
	if err == nil {
		t.Fatal("want error when upload fails, got nil")
	}
	if url != nil {
		t.Errorf("want nil URL on upload failure, got %q", *url)
	}
	if svc.repo.sessions[sess.ID].CertificateKey != nil {
		t.Error("must not persist a certificate URL when upload failed")
	}
}

func TestResolveCertificateURL_PersistFailure_ReturnsError(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	// Session NOT seeded in the repo → UpdateSessionCertificate returns ErrNotFound.
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
		CertificateKey: nil, CertificateGeneratedAt: nil,
	}
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam"}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("want ErrNotFound from persist step, got %v", err)
	}
	if url != nil {
		t.Errorf("want nil URL on persist failure, got %q", *url)
	}
}

// ---------- tests: resolveCertificateURL — design staleness (FR-13/FR-15) ----------

func TestResolveCertificateURL_RegenerationWhenDesignStale(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	certGeneratedAt := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	designUpdatedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC) // after cert generated
	oldURL := "http://old.url/cert.pdf"
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
		CertificateKey: &oldURL, CertificateGeneratedAt: &certGeneratedAt,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam", CertificateDesignUpdatedAt: &designUpdatedAt}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == nil {
		t.Fatal("want non-nil URL")
	}
	if svc.uploadCertCalls != 1 {
		t.Errorf("a design edit after generation should trigger exactly one regeneration, got %d upload calls", svc.uploadCertCalls)
	}
	updated := svc.repo.sessions[sess.ID]
	if updated.CertificateGeneratedAt == nil || !updated.CertificateGeneratedAt.After(certGeneratedAt) {
		t.Error("CertificateGeneratedAt should have been bumped by the regeneration")
	}
}

func TestResolveCertificateURL_NoRegenerationWhenDesignNotStaleOrChanged(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	certGeneratedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	designUpdatedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC) // before cert generated
	oldURL := "http://old.url/cert.pdf"
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
		CertificateKey: &oldURL, CertificateGeneratedAt: &certGeneratedAt,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam", CertificateDesignUpdatedAt: &designUpdatedAt}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == nil || *url != oldURL {
		t.Errorf("want existing URL %q, got %v", oldURL, url)
	}
	if svc.uploadCertCalls != 0 {
		t.Errorf("want zero regenerations, got %d upload calls", svc.uploadCertCalls)
	}

	// FR-15: a second read with nothing changed must trigger zero further regeneration.
	if _, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi"); err != nil {
		t.Fatalf("unexpected error on second read: %v", err)
	}
	if svc.uploadCertCalls != 0 {
		t.Errorf("second read with nothing changed should not regenerate, got %d upload calls", svc.uploadCertCalls)
	}
}

// ---------- tests: resolveCertificateURL — certificate number immutability (FR-10) ----------

func TestResolveCertificateURL_RegenerationReusesCertificateNumber(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "My Exam"}

	if _, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi"); err != nil {
		t.Fatalf("first generation: %v", err)
	}
	firstNumber := svc.repo.sessions[sess.ID].CertificateNumber
	if firstNumber == nil {
		t.Fatal("want a certificate number allocated on first generation")
	}

	// Force a regeneration via re-grading staleness.
	gradedAt := time.Now().Add(time.Hour)
	answers := []model.ExamSessionAnswer{{GradedAt: &gradedAt}}
	if _, err := svc.resolveCertificateURL(ctx, exam, sess, answers, "Budi"); err != nil {
		t.Fatalf("regeneration: %v", err)
	}
	secondNumber := svc.repo.sessions[sess.ID].CertificateNumber
	if secondNumber == nil || *secondNumber != *firstNumber {
		t.Errorf("regeneration should reuse the original number: first=%v second=%v", firstNumber, secondNumber)
	}
	if svc.uploadCertCalls != 2 {
		t.Errorf("want 2 uploads (first generation + regeneration), got %d", svc.uploadCertCalls)
	}
}

// ---------- tests: custom template with NULL background key (FR-19) ----------

func TestResolveCertificateURL_CustomTemplateNilBackgroundKey_Renders(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	submittedAt := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	sess := &model.ExamSession{
		ID: uuid.New(), Status: "submitted", SubmittedAt: &submittedAt,
	}
	svc.repo.sessions[sess.ID] = sess
	exam := &model.Exam{CertificateDesign: certDesignJSON("custom"), Title: "Custom Exam"}

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil, "Budi")
	if err != nil {
		t.Fatalf("custom template with a NULL background key should still render, got error: %v", err)
	}
	if url == nil {
		t.Fatal("want non-nil URL")
	}
}

// ---------- tests: GetCertificatePreview (FR-12, FR-19) ----------

func TestGetCertificatePreview_DoesNotAllocate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "Preview Exam"}
	svc.repo.seedExam(exam)

	pdf, err := svc.GetCertificatePreview(ctx, exam.ID, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("want a non-empty PDF")
	}
	// No session exists for this exam. If GetCertificatePreview ever called
	// AllocateCertificateNumber, there would be nothing to allocate against —
	// the fake repo would return ErrNotFound and this call would fail.
	if len(svc.repo.sessions) != 0 {
		t.Fatal("preview must not create or touch any session")
	}
}

func TestGetCertificatePreview_CustomTemplateNilBackgroundKey_Renders(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	exam := &model.Exam{CertificateDesign: certDesignJSON("custom"), Title: "Custom Preview Exam"}
	svc.repo.seedExam(exam)

	pdf, err := svc.GetCertificatePreview(ctx, exam.ID, "")
	if err != nil {
		t.Fatalf("custom template with a NULL background key should still render, got error: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("want a non-empty PDF")
	}
}

func TestGetCertificatePreview_UnknownExam_ReturnsErrExamNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newShimSessionService(t)

	_, err := svc.GetCertificatePreview(ctx, uuid.New(), "")
	if !errors.Is(err, ErrExamNotFound) {
		t.Errorf("want ErrExamNotFound, got %v", err)
	}
}

// ---------- tests: resolveCertificateLayout (FR-29) ----------

func TestResolveCertificateLayout_NilLayout_SeedsBuiltinDefault(t *testing.T) {
	exam := &model.Exam{CertificateDesign: certDesignJSON("modern")}
	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := defaultLayout("modern")
	if !reflect.DeepEqual(layout, want) {
		t.Error("an exam with no saved layout should seed the built-in template's default layout, not an empty canvas")
	}
}

func TestResolveCertificateLayout_CustomTemplateNilLayout_FallsBackToClassic(t *testing.T) {
	exam := &model.Exam{CertificateDesign: certDesignJSON("custom")}
	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := defaultLayout("classic")
	if !reflect.DeepEqual(layout, want) {
		t.Error("a custom template with no saved layout should fall back to classic's default layout")
	}
}

func TestResolveCertificateLayout_SavedLayout_UsesPersistedFields(t *testing.T) {
	raw := json.RawMessage(`{"template":"classic","page":{"width_mm":297,"height_mm":210},"background":{"kind":"builtin","ref":"classic"},"fields":[{"id":"title","x_mm":10,"y_mm":10,"w_mm":50,"align":"left","visible":true}]}`)
	exam := &model.Exam{CertificateDesign: &raw}
	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Fields) != 1 || layout.Fields[0].ID != "title" || layout.Fields[0].XMm != 10 {
		t.Errorf("want the persisted layout fields, got %+v", layout.Fields)
	}
}

// ---------- tests: certificate output geometry (FR-30, FR-31) ----------
//
// These assert on the generated HTML's geometry, which is what the renderer
// consumes — not on rendered pixels. Pixel-level verification of the real
// Gotenberg output (orientation, background, glyph ink at the layout position)
// lives in the gotenberg_integration gate, certificate_integration_test.go: a
// substring check here is blind to an upside-down or blank page, and both
// shipped past a green suite in v1 (memory: pdf-layout-needs-visual-verification).

// TestGenerateCertificatePDF_BuiltinsRenderOnePageWithBackground and
// TestGenerateCertificatePDF_LongAndNonASCIINames above, and
// TestGenerateCertificatePDF_FieldsRenderAtLayoutPositions below, are the
// HTML-pipeline equivalents of the deleted PDF-rendering rasterization assertions
// (one page, A4 landscape, background present, field ink at the correct mm
// position — the direct guard for R1's off-centre/upside-down bug).

// TestGenerateCertificatePDF_FieldsRenderAtLayoutPositions proves the full path
// from an exam's saved certificate_design blob (resolveCertificateLayout) to
// rendered HTML preserves field geometry exactly, keeping the date field as
// the Y-inversion discriminator (R1): its top sits near the page bottom
// (166mm of a 210mm page), which a pageHeight-y inversion bug would instead
// place near the top. certificate_html_test.go's
// TestBuildCertificateHTML_PageSizeAndFieldGeometry covers buildCertificateHTML
// directly on a hand-built Layout; this covers the exam->layout->HTML path.
func TestGenerateCertificatePDF_FieldsRenderAtLayoutPositions(t *testing.T) {
	raw := json.RawMessage(`{"template":"classic","page":{"width_mm":297,"height_mm":210},"background":{"kind":"builtin","ref":"classic"},"fields":[{"id":"student_name","x_mm":48.5,"y_mm":108,"w_mm":200,"align":"left","font":"cormorant_garamond","weight":"bold","size_pt":40,"color":"#22315B","visible":true},{"id":"date","x_mm":48.5,"y_mm":166,"w_mm":200,"align":"right","font":"public_sans","weight":"regular","size_pt":11,"color":"#4A5568","visible":true}]}`)
	exam := &model.Exam{CertificateDesign: &raw, Title: "Ujian Matematika"}

	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		t.Fatalf("resolveCertificateLayout: unexpected error: %v", err)
	}
	vals := certificateFieldValues(exam.Title, "Budi Santoso", "21 Juli 2026", "ABK/2026/0001/000001")
	out, err := buildCertificateHTML(layout, vals, builtinCertificateBackground("classic"), nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML: unexpected error: %v", err)
	}
	html := string(out)

	if !strings.Contains(html, "left:48.5mm;top:108mm;width:200mm;text-align:left;") {
		t.Errorf("student_name field not at its saved geometry, got:\n%s", html)
	}
	if !strings.Contains(html, "left:48.5mm;top:166mm;width:200mm;text-align:right;") {
		t.Errorf("date field not at its saved geometry (Y-inversion guard), got:\n%s", html)
	}
	if strings.Contains(html, "top:44mm") {
		t.Errorf("date field appears inverted toward the page top, got:\n%s", html)
	}
}

// TestDefaultLayout_CertificateNumberColorContrastsWithBackground is the
// regression guard for the "certificate_number recolored for contrast" fix:
// classic's number sits on the navy footer band (needs a light color) and
// elegant's sits on the cream page fill (needs a dark color) — a pixel-average
// ink-presence check can't distinguish "adequately contrasting" from "the old
// low-contrast gray" here, because gray is still measurably different from
// either background by raw color distance even though it reads as washed-out
// against navy (both were verified as false-negative against a manual
// mutation back to the original gray, which this equality check does catch).
// This pins the specific color the fix chose for each template so a revert to
// a same-hue value is caught deterministically, not by ambiguous pixel math.
func TestDefaultLayout_CertificateNumberColorContrastsWithBackground(t *testing.T) {
	cases := []struct {
		tmpl      string
		wantColor string
	}{
		{"classic", "#F0CB78"}, // gold on the navy footer band
		{"elegant", "#8A6A16"}, // dark gold on the cream page fill
	}
	for _, tc := range cases {
		t.Run(tc.tmpl, func(t *testing.T) {
			layout := defaultLayout(tc.tmpl)
			var got string
			found := false
			for _, f := range layout.Fields {
				if f.ID == "certificate_number" {
					got = f.Color
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s default layout has no certificate_number field", tc.tmpl)
			}
			if got != tc.wantColor {
				t.Errorf("%s certificate_number color = %q, want %q (contrast fix)", tc.tmpl, got, tc.wantColor)
			}
		})
	}
}

// solidDarkPNG returns a small opaque dark-blue PNG usable as a stand-in
// signature/stamp image for render assertions.
func solidDarkPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 30, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestRenderCertificate_SignatureImageStampsAtFieldBoxOnlyWhenPresent proves
// buildCertificateHTML's image field handling (the HTML-pipeline equivalent of
// the deleted renderCertificateWithImages behavior): a visible "signature"
// field stamps the uploaded image at its own box (object-fit:contain, no
// stretch/crop) only when image bytes are supplied, and renders nothing at all
// when they are not.
func TestRenderCertificate_SignatureImageStampsAtFieldBoxOnlyWhenPresent(t *testing.T) {
	layout := testCertificateLayout()
	dateStr := jakartaDateStr(t)
	vals := certificateFieldValues("Ujian Matematika", "Budi Santoso", dateStr, "ABK/2026/0042/000005")
	sigBytes := solidDarkPNG(t, 40, 20)

	withSig, err := buildCertificateHTML(layout, vals, []byte("bg"), map[FieldID][]byte{"signature": sigBytes})
	if err != nil {
		t.Fatalf("buildCertificateHTML: unexpected error: %v", err)
	}
	html := string(withSig)
	if gotSig := dataURIBytes(t, html, `style="position:absolute;left:205mm;top:150mm;width:62mm;height:22mm;object-fit:contain;"`); !bytes.Equal(gotSig, sigBytes) {
		t.Error("expected the signature image bytes to be embedded at its field box")
	}
	if !strings.Contains(html, "left:205mm;top:150mm;width:62mm;height:22mm;object-fit:contain;") {
		t.Errorf("expected signature field geometry (object-fit:contain, no stretch/crop), got:\n%s", html)
	}

	withoutSig, err := buildCertificateHTML(layout, vals, []byte("bg"), nil)
	if err != nil {
		t.Fatalf("buildCertificateHTML: unexpected error: %v", err)
	}
	if strings.Contains(string(withoutSig), "left:205mm;top:150mm") {
		t.Errorf("signature field with no image bytes must not render, got:\n%s", string(withoutSig))
	}
}
