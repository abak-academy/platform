package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
