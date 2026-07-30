package service

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

// --- FB-24: br/p on the question-body allowlist ---

func TestSanitizeQuestionBody_preservesBrAndP(t *testing.T) {
	in := `<p>A</p><p>B</p><br>`
	got := sanitizeQuestionBody(in)
	if got != in {
		t.Errorf("br/p must round-trip unchanged\n in: %q\nout: %q", in, got)
	}
}

func TestSanitizeQuestionBody_retainsTwoParagraphs(t *testing.T) {
	got := sanitizeQuestionBody("<p>A</p><p>B</p>")
	want := "<p>A</p><p>B</p>"
	if got != want {
		t.Errorf("both <p> elements must survive sanitization\n got: %q\nwant: %q", got, want)
	}
}

func TestValidateQuestion_rejects_break_only_body(t *testing.T) {
	q := model.Question{Format: "essay", Body: "<br><br>   ", PointCorrect: 1}
	err := validateQuestion(q, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("break-only body should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "body cannot be empty") {
		t.Errorf("break-only body msg should mention 'body cannot be empty', got %q", err.Error())
	}
}

func TestValidateQuestion_accepts_paragraph_body_with_text(t *testing.T) {
	q := model.Question{Format: "essay", Body: "<p>A</p>", PointCorrect: 1}
	if err := validateQuestion(q, nil, nil); err != nil {
		t.Errorf("paragraph body with real text should be accepted, got %v", err)
	}
}


// --- Cross-language tag-list equality (C4, FR4) ---

func TestQuestionBodyAllowedTags_matchesFrontendAllowlist(t *testing.T) {
	data, err := os.ReadFile("../../../web/lib/question-html.ts")
	if err != nil {
		t.Fatalf("failed to read web/lib/question-html.ts: %v", err)
	}

	arrayPattern := regexp.MustCompile(`(?s)QUESTION_BODY_ALLOWED_TAGS\s*=\s*\[(.*?)\]`)
	match := arrayPattern.FindSubmatch(data)
	if match == nil {
		t.Fatalf("could not find QUESTION_BODY_ALLOWED_TAGS array in web/lib/question-html.ts")
	}

	tagPattern := regexp.MustCompile(`"([^"]+)"`)
	rawTags := tagPattern.FindAllSubmatch(match[1], -1)
	frontendTags := make([]string, len(rawTags))
	for i, m := range rawTags {
		frontendTags[i] = string(m[1])
	}

	if len(frontendTags) != len(questionBodyAllowedTags) {
		t.Fatalf("tag count mismatch: web=%v (%d) go=%v (%d)", frontendTags, len(frontendTags), questionBodyAllowedTags, len(questionBodyAllowedTags))
	}
	for i, tag := range questionBodyAllowedTags {
		if frontendTags[i] != tag {
			t.Errorf("tag mismatch at index %d: go=%q web=%q", i, tag, frontendTags[i])
		}
	}
}
