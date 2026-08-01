package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

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
// for "whatever was uploaded") so the plumbing has real bytes to move around.
func (s *shimSessionService) downloadCertificateBackground(_ context.Context, _ string) ([]byte, error) {
	return certBgClassic, nil
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
// exam_session_test.go / exam_result_test.go. resolveCertificateLayout is a
// pure package function, so this calls it for real rather than faking it.
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
		if _, err := resolveCertificateLayout(exam); err != nil {
			return nil, err
		}
		bg, err := s.resolveCertificateBackground(ctx, exam)
		if err != nil {
			return nil, err
		}

		// This shim exercises the plumbing (staleness/allocation/persist), not the
		// real HTML->PDF conversion, so a fixed byte stand-in serves directly for
		// the renderer's PDF bytes (mirrors the real Service passing the built
		// HTML through s.renderer.RenderHTML) — the HTML pipeline itself is
		// covered by the print-route render gate, not this shim.
		pdf := append([]byte(nil), bg...)
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

	if _, err := resolveCertificateLayout(&previewExam); err != nil {
		return nil, err
	}
	bg, err := s.resolveCertificateBackground(ctx, &previewExam)
	if err != nil {
		return nil, err
	}

	// This shim exercises the plumbing (template resolution/no-allocation), not
	// the real HTML->PDF conversion — see resolveCertificateURL above.
	return append([]byte(nil), bg...), nil
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
		{GradedAt: nil}, // ungraded
		{GradedAt: &t2}, // latest
	}
	got := latestGradedAt(answers)
	if got == nil || !got.Equal(t2) {
		t.Errorf("want %v, got %v", t2, got)
	}
}

func TestCertificateSessionValues_DerivesDynamicTokens(t *testing.T) {
	started := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	submitted := started.Add(89*time.Minute + time.Second)
	score := 86.0
	sess := &model.ExamSession{StartedAt: started, SubmittedAt: &submitted, Score: &score}
	questions := []model.QuestionWithOptions{
		{Question: model.Question{PointCorrect: 40}},
		{Question: model.Question{PointCorrect: 60}},
	}

	got := certificateSessionValues(sess, questions, 2, 20)

	want := map[FieldID]string{
		"score": "86", "max_score": "100", "score_percent": "86%",
		"rank": "3", "percentile": "Top 15%", "duration": "90 minutes",
		"total_questions": "2 questions",
	}
	for field, value := range want {
		if got[field] != value {
			t.Errorf("%s = %q, want %q", field, got[field], value)
		}
	}
}

// FR-35: certificate max_score must reflect a true_false question's statement
// count x point_correct and a multi_blank question's blank count x point_correct,
// not a flat point_correct per question — proven through certificateSessionValues,
// the same shared questionMaxPoints helper topicBreakdown and the leaderboard use.
func TestCertificateSessionValues_TrueFalseAndMultiBlank_maxScoreUsesSharedHelper_FR35(t *testing.T) {
	sess := &model.ExamSession{StartedAt: time.Now(), Score: floatPtr(7)}
	tfID := uuid.New()
	mbID := uuid.New()
	questions := []model.QuestionWithOptions{
		{
			Question: model.Question{ID: tfID, Format: "true_false", PointCorrect: 1, Statements: []model.QuestionStatement{
				{QuestionID: tfID, Index: 1, Body: "s1", IsTrue: true},
				{QuestionID: tfID, Index: 2, Body: "s2", IsTrue: false},
				{QuestionID: tfID, Index: 3, Body: "s3", IsTrue: true},
				{QuestionID: tfID, Index: 4, Body: "s4", IsTrue: false},
			}},
		},
		{
			Question: model.Question{ID: mbID, Format: "multi_blank", PointCorrect: 1},
			Blanks: []model.QuestionBlank{
				{QuestionID: mbID, Index: 1, CorrectAnswer: "a"},
				{QuestionID: mbID, Index: 2, CorrectAnswer: "b"},
				{QuestionID: mbID, Index: 3, CorrectAnswer: "c"},
			},
		},
	}

	got := certificateSessionValues(sess, questions, -1, -1)

	if got["max_score"] != "7" {
		t.Errorf("max_score: want %q (4 true_false + 3 multi_blank), got %q", "7", got["max_score"])
	}
}

func TestLayoutUsesToken_FindsSensitiveTokenInMixedCopy(t *testing.T) {
	layout := Layout{Page: Page{WidthMm: 297, HeightMm: 210}, Fields: []LayoutField{{
		ID: "score", Kind: "text", Content: "Final score: {{score}}", XMm: 10, YMm: 10, WMm: 100, Visible: true,
	}}}
	if !layoutUsesToken(layout, "score", "rank") {
		t.Fatal("expected score token to be detected")
	}
	if layoutUsesToken(layout, "rank", "percentile") {
		t.Fatal("did not expect absent rank tokens to be detected")
	}
}

func TestCertificateLayoutAllowed_EnforcesResultGateOnlyForSensitiveTokens(t *testing.T) {
	release := time.Now().Add(time.Hour)
	sess := &model.ExamSession{Status: "submitted"}
	staticLayout := Layout{Fields: []LayoutField{{ID: "title", Kind: "text", Content: "Completed"}}}
	scoreLayout := Layout{Fields: []LayoutField{{ID: "score", Kind: "text", Content: "{{score}}"}}}

	if !certificateLayoutAllowed(model.Exam{ResultConfig: "hidden"}, sess, staticLayout, nil, nil) {
		t.Fatal("completion-only certificate should remain available")
	}
	for _, exam := range []model.Exam{
		{ResultConfig: "hidden"},
		{ResultConfig: "score", ResultReleaseAt: &release},
	} {
		if certificateLayoutAllowed(exam, sess, scoreLayout, nil, nil) {
			t.Fatalf("sensitive certificate should be gated for %+v", exam)
		}
	}
	if !certificateLayoutAllowed(model.Exam{ResultConfig: "score"}, sess, scoreLayout, nil, nil) {
		t.Fatal("sensitive certificate should be available after the result gate")
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

func TestServiceResolveCertificateURL_SubmittedWithoutTimestampReturnsNil(t *testing.T) {
	svc := newOfflineStorageService(t)
	sess := &model.ExamSession{ID: uuid.New(), Status: "submitted"}
	exam := &model.Exam{CertificateDesign: certDesignJSON("classic"), Title: "Test"}

	url, err := svc.resolveCertificateURL(context.Background(), exam, sess, nil, "Budi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != nil {
		t.Fatalf("expected no certificate without submitted_at, got %q", *url)
	}
}

// TestServiceResolveCertificateURL_FreshCacheStillResolvesLayoutForGate
// replaces a pre-#55-fix expectation that a fresh cache hit skipped layout
// resolution entirely. That was precisely the bug (NFR-S4: a cached PDF is
// never itself an authorization decision) — the gate now resolves the layout
// on every access, cached or not, so a malformed design surfaces its parse
// error instead of silently serving an unchecked URL.
func TestServiceResolveCertificateURL_FreshCacheStillResolvesLayoutForGate(t *testing.T) {
	svc := newOfflineStorageService(t)
	generatedAt := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	key := "certificates/session.pdf"
	sess := &model.ExamSession{
		ID:                     uuid.New(),
		Status:                 "submitted",
		CertificateKey:         &key,
		CertificateGeneratedAt: &generatedAt,
	}
	malformed := json.RawMessage(`{`)
	exam := &model.Exam{CertificateDesign: &malformed, Title: "Test", CertificateEnabled: true}

	url, err := svc.resolveCertificateURL(context.Background(), exam, sess, nil, "Budi")
	if err == nil {
		t.Fatalf("want a layout parse error on a fresh cache hit, got url=%v", url)
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
		{"classic", "#6B5B34"}, // dark gold: the 2026-07-30 background swap replaced
		// classic's navy footer band with a cream one, so the old pale #F0CB78 was
		// invisible there. The guard still is "must contrast with the footer".
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
