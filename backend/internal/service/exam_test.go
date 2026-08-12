package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func strPtr(s string) *string { return &s }

func TestFormatExamNumber_PadsToMinimumFourDigits(t *testing.T) {
	cases := map[int]string{
		1:     "0001",
		42:    "0042",
		999:   "0999",
		9999:  "9999",
		10000: "10000",
	}
	for n, want := range cases {
		if got := formatExamNumber(n); got != want {
			t.Errorf("formatExamNumber(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestValidateQuestion_mcq_accepts_exactly_one_correct(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "2+2", PointCorrect: 1}
	options := []model.QuestionOption{
		{Key: "a", Text: "4", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "5", SortOrder: 2},
	}
	if err := validateQuestion(q, options, nil, nil); err != nil {
		t.Errorf("mcq with 1 correct + 2 options should pass, got %v", err)
	}
}

func TestValidateQuestion_mcq_rejects_zero_correct(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "2+2"}
	options := []model.QuestionOption{
		{Key: "a", Text: "4", SortOrder: 1},
		{Key: "b", Text: "5", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("mcq with 0 correct should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "exactly 1 correct option") {
		t.Errorf("mcq 0-correct msg should mention 'exactly 1 correct option', got %q", err.Error())
	}
}

func TestValidateQuestion_mcq_rejects_two_correct(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "2+2"}
	options := []model.QuestionOption{
		{Key: "a", Text: "4", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "5", IsCorrect: true, SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("mcq with 2 correct should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "exactly 1 correct option") {
		t.Errorf("mcq 2-correct msg should mention 'exactly 1 correct option', got %q", err.Error())
	}
}

func TestValidateQuestion_mcq_rejects_fewer_than_2_options(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "2+2"}
	options := []model.QuestionOption{
		{Key: "a", Text: "4", IsCorrect: true, SortOrder: 1},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("mcq with 1 option should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least 2 options") {
		t.Errorf("mcq 1-option msg should mention 'at least 2 options', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_answer_accepts_one_or_more_correct(t *testing.T) {
	q := model.Question{Format: "multi_answer", Body: "primes", PointCorrect: 1}
	// one correct
	opts1 := []model.QuestionOption{
		{Key: "a", Text: "2", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "4", SortOrder: 2},
		{Key: "c", Text: "6", SortOrder: 3},
	}
	if err := validateQuestion(q, opts1, nil, nil); err != nil {
		t.Errorf("multi_answer with 1 correct + 3 options should pass, got %v", err)
	}
	// two correct
	opts2 := []model.QuestionOption{
		{Key: "a", Text: "2", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "4", IsCorrect: true, SortOrder: 2},
		{Key: "c", Text: "6", SortOrder: 3},
	}
	if err := validateQuestion(q, opts2, nil, nil); err != nil {
		t.Errorf("multi_answer with 2 correct + 3 options should pass, got %v", err)
	}
}

func TestValidateQuestion_multi_answer_rejects_zero_correct(t *testing.T) {
	q := model.Question{Format: "multi_answer", Body: "primes"}
	options := []model.QuestionOption{
		{Key: "a", Text: "2", SortOrder: 1},
		{Key: "b", Text: "4", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("multi_answer with 0 correct should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least 1 correct option") {
		t.Errorf("multi_answer 0-correct msg should mention 'at least 1 correct option', got %q", err.Error())
	}
}

func TestValidateQuestion_short_requires_correct_answer(t *testing.T) {
	q := model.Question{Format: "short", Body: "capital of France"}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("short with no accepted_answers should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least one accepted answer") {
		t.Errorf("short empty-answer msg should mention 'at least one accepted answer', got %q", err.Error())
	}
}

func TestValidateQuestion_short_rejects_options(t *testing.T) {
	q := model.Question{Format: "short", Body: "x", CorrectAnswer: strPtr("y")}
	options := []model.QuestionOption{
		{Key: "a", Text: "y", SortOrder: 1},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("short with options should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot have options") {
		t.Errorf("short options msg should mention 'cannot have options', got %q", err.Error())
	}
}

func TestValidateQuestion_fill_blank_requires_correct_answer(t *testing.T) {
	q := model.Question{Format: "fill_blank", Body: "the ___ is blue"}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("fill_blank with no accepted_answers should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least one accepted answer") {
		t.Errorf("fill_blank empty-answer msg should mention 'at least one accepted answer', got %q", err.Error())
	}
}

func TestValidateQuestion_essay_accepts_no_options_no_correct_answer(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain gravity", PointCorrect: 1}
	if err := validateQuestion(q, nil, nil, nil); err != nil {
		t.Errorf("essay with no options + no correct_answer should pass, got %v", err)
	}
}

func TestValidateQuestion_essay_rejects_options(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain"}
	options := []model.QuestionOption{
		{Key: "a", Text: "x", SortOrder: 1},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("essay with options should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot have options") {
		t.Errorf("essay options msg should mention 'cannot have options', got %q", err.Error())
	}
}

func TestValidateQuestion_essay_rejects_correct_answer(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain", CorrectAnswer: strPtr("something")}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("essay with correct_answer should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot have correct_answer") {
		t.Errorf("essay correct_answer msg should mention 'cannot have correct_answer', got %q", err.Error())
	}
}

func TestValidateQuestion_rejects_unknown_format(t *testing.T) {
	q := model.Question{Format: "matching", Body: "x"}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("unknown format should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "unknown question format") {
		t.Errorf("unknown format msg should mention 'unknown question format', got %q", err.Error())
	}
}

func TestValidateQuestion_rejects_duplicate_option_keys(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "x"}
	options := []model.QuestionOption{
		{Key: "a", Text: "1", IsCorrect: true, SortOrder: 1},
		{Key: "a", Text: "2", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("duplicate option key should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "duplicate option key") {
		t.Errorf("duplicate key msg should mention 'duplicate option key', got %q", err.Error())
	}
}

func TestValidateQuestion_rejects_empty_option_text(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "x"}
	options := []model.QuestionOption{
		{Key: "a", Text: "   ", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "y", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("empty (whitespace) option text should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "option text cannot be empty") {
		t.Errorf("empty option text msg should mention 'option text cannot be empty', got %q", err.Error())
	}
}

func TestValidateQuestion_mcq_rejects_correct_answer_set(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "x", CorrectAnswer: strPtr("a")}
	options := []model.QuestionOption{
		{Key: "a", Text: "1", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "2", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("mcq with correct_answer set should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "mcq cannot have correct_answer") {
		t.Errorf("mcq correct_answer msg should mention 'mcq cannot have correct_answer', got %q", err.Error())
	}
}

func TestValidateTest_rejects_empty_title(t *testing.T) {
	tst := model.Test{Title: "   ", Subject: "math", Topic: "algebra", DurationMinutes: 60}
	err := validateTest(tst)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("empty title should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "test title required") {
		t.Errorf("empty title msg should mention 'test title required', got %q", err.Error())
	}
}

func TestValidateTest_rejects_zero_duration(t *testing.T) {
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 0}
	err := validateTest(tst)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("zero duration should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "duration_minutes must be positive") {
		t.Errorf("zero duration msg should mention 'duration_minutes must be positive', got %q", err.Error())
	}
}

func TestValidateTest_rejects_empty_subject_topic(t *testing.T) {
	tst := model.Test{Title: "x", Subject: "", Topic: "", DurationMinutes: 60}
	err := validateTest(tst)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("empty subject should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "test subject/topic required") {
		t.Errorf("empty subject msg should mention 'test subject/topic required', got %q", err.Error())
	}
}

func TestValidateTest_rejects_negative_audio_play_limit(t *testing.T) {
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 60, AudioPlayLimit: intptr(-1)}
	err := validateTest(tst)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("negative audio_play_limit should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "audio_play_limit must be positive") {
		t.Errorf("negative audio_play_limit msg should mention 'audio_play_limit must be positive', got %q", err.Error())
	}
}

func TestValidateTest_accepts_valid(t *testing.T) {
	tst := model.Test{Title: "Algebra 1", Subject: "math", Topic: "algebra", DurationMinutes: 60}
	if err := validateTest(tst); err != nil {
		t.Errorf("valid test should pass, got %v", err)
	}
}

// sanity: validateQuestion for a short question with non-empty correct_answer passes
func TestValidateQuestion_short_accepts_valid(t *testing.T) {
	q := model.Question{Format: "short", Body: "capital of France", CorrectAnswer: strPtr("Paris"), PointCorrect: 1}
	if err := validateQuestion(q, nil, nil, nil); err != nil {
		t.Errorf("valid short should pass, got %v", err)
	}
}

func TestValidateQuestion_short_rejects_whitespace_only_correct_answer(t *testing.T) {
	q := model.Question{Format: "short", Body: "x", CorrectAnswer: strPtr("   ")}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("whitespace-only correct_answer should return ErrValidation, got %v", err)
	}
}

func TestValidateQuestion_empty_option_key(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "x"}
	options := []model.QuestionOption{
		{Key: "", Text: "1", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "2", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("empty option key should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "option key cannot be empty") {
		t.Errorf("empty key msg should mention 'option key cannot be empty', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_answer_rejects_fewer_than_2_options(t *testing.T) {
	q := model.Question{Format: "multi_answer", Body: "x"}
	options := []model.QuestionOption{
		{Key: "a", Text: "1", IsCorrect: true, SortOrder: 1},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("multi_answer with 1 option should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least 2 options") {
		t.Errorf("multi_answer 1-option msg should mention 'at least 2 options', got %q", err.Error())
	}
}

func TestValidateQuestion_fill_blank_rejects_options(t *testing.T) {
	q := model.Question{Format: "fill_blank", Body: "x", CorrectAnswer: strPtr("y")}
	options := []model.QuestionOption{
		{Key: "a", Text: "y", SortOrder: 1},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("fill_blank with options should return ErrValidation, got %v", err)
	}
}

func TestValidateQuestion_multi_answer_rejects_correct_answer_set(t *testing.T) {
	q := model.Question{Format: "multi_answer", Body: "x", CorrectAnswer: strPtr("a")}
	options := []model.QuestionOption{
		{Key: "a", Text: "1", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "2", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("multi_answer with correct_answer set should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "multi_answer cannot have correct_answer") {
		t.Errorf("multi_answer correct_answer msg should mention 'multi_answer cannot have correct_answer', got %q", err.Error())
	}
}

func TestValidateQuestion_rejects_point_correct_zero_or_below(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain gravity", PointCorrect: 0, PointWrong: 0}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("point_correct=0 should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "point_correct must be > 0") {
		t.Errorf("point_correct=0 msg should mention 'point_correct must be > 0', got %q", err.Error())
	}

	q.PointCorrect = -1
	err = validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("point_correct=-1 should return ErrValidation, got %v", err)
	}
}

// FR-16/FR-17: point_correct is fractional with a > 0 floor.
func TestValidateQuestion_acceptsFractionalPointCorrect(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain gravity", PointCorrect: 2.5, PointWrong: 0}
	if err := validateQuestion(q, nil, nil, nil); err != nil {
		t.Errorf("point_correct=2.5 should pass, got %v", err)
	}

	q.PointCorrect = 0.25
	if err := validateQuestion(q, nil, nil, nil); err != nil {
		t.Errorf("point_correct=0.25 should pass, got %v", err)
	}
}

func TestValidateQuestion_rejects_negative_point_wrong(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain gravity", PointCorrect: 1, PointWrong: -1}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("point_wrong=-1 should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "point_wrong must be >= 0") {
		t.Errorf("point_wrong=-1 msg should mention 'point_wrong must be >= 0', got %q", err.Error())
	}
}

func TestValidateQuestion_accepts_valid_points(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain gravity", PointCorrect: 2, PointWrong: 1}
	if err := validateQuestion(q, nil, nil, nil); err != nil {
		t.Errorf("point_correct=2, point_wrong=1 should pass, got %v", err)
	}
}

func TestValidateQuestion_rejects_body_empty_after_sanitization(t *testing.T) {
	// Simulates what every write path does: sanitize, then validate. <br> is
	// allowlisted (FB-24) and survives sanitization, but carries no text
	// content, so isQuestionBodyEmpty still treats it as blank.
	q := model.Question{Format: "essay", Body: sanitizeQuestionBody("<br>"), PointCorrect: 1}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("body that sanitizes to empty should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "body cannot be empty") {
		t.Errorf("empty-body msg should mention 'body cannot be empty', got %q", err.Error())
	}
}

// Multi-blank validation tests
func TestValidateQuestion_multi_blank_accepts_sequential_tokens_with_matching_blanks(t *testing.T) {
	q := model.Question{
		Format:       "multi_blank",
		Body:         "Ibu kota Indonesia adalah {{1}}, didirikan tahun {{2}}.",
		PointCorrect: 1,
	}
	blanks := []model.QuestionBlank{
		{Index: 1, CorrectAnswer: "jakarta"},
		{Index: 2, CorrectAnswer: "1945"},
	}
	if err := validateQuestion(q, nil, blanks, nil); err != nil {
		t.Errorf("valid multi_blank with sequential tokens should pass, got %v", err)
	}
}

func TestValidateQuestion_multi_blank_rejects_non_sequential_tokens(t *testing.T) {
	q := model.Question{
		Format:       "multi_blank",
		Body:         "{{1}} and {{3}}",
		PointCorrect: 1,
	}
	blanks := []model.QuestionBlank{
		{Index: 1, CorrectAnswer: "a"},
		{Index: 3, CorrectAnswer: "b"},
	}
	err := validateQuestion(q, nil, blanks, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("non-sequential tokens should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "gap") {
		t.Errorf("non-sequential tokens msg should mention 'gap', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_rejects_duplicate_tokens(t *testing.T) {
	q := model.Question{
		Format:       "multi_blank",
		Body:         "{{1}} and {{1}}",
		PointCorrect: 1,
	}
	blanks := []model.QuestionBlank{
		{Index: 1, CorrectAnswer: "a"},
	}
	err := validateQuestion(q, nil, blanks, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("duplicate tokens should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate tokens msg should mention 'duplicate', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_rejects_zero_tokens(t *testing.T) {
	q := model.Question{
		Format:       "multi_blank",
		Body:         "no tokens here",
		PointCorrect: 1,
	}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("zero tokens should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Errorf("zero tokens msg should mention 'at least one', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_rejects_non_empty_options(t *testing.T) {
	q := model.Question{
		Format:       "multi_blank",
		Body:         "{{1}}",
		PointCorrect: 1,
	}
	options := []model.QuestionOption{
		{Key: "a", Text: "opt", SortOrder: 1},
	}
	blanks := []model.QuestionBlank{
		{Index: 1, CorrectAnswer: "a"},
	}
	err := validateQuestion(q, options, blanks, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("multi_blank with options should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot have options") {
		t.Errorf("multi_blank with options msg should mention 'cannot have options', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_rejects_non_empty_correct_answer(t *testing.T) {
	q := model.Question{
		Format:        "multi_blank",
		Body:          "{{1}}",
		CorrectAnswer: strPtr("scalar"),
		PointCorrect:  1,
	}
	blanks := []model.QuestionBlank{
		{Index: 1, CorrectAnswer: "a"},
	}
	err := validateQuestion(q, nil, blanks, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("multi_blank with correct_answer should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot have correct_answer") {
		t.Errorf("multi_blank with correct_answer msg should mention 'cannot have correct_answer', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_rejects_blanks_count_mismatch(t *testing.T) {
	q := model.Question{
		Format:       "multi_blank",
		Body:         "{{1}} and {{2}}",
		PointCorrect: 1,
	}
	blanks := []model.QuestionBlank{
		{Index: 1, CorrectAnswer: "a"},
		// Missing blank for {{2}}
	}
	err := validateQuestion(q, nil, blanks, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("blanks count mismatch should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "blanks count") {
		t.Errorf("blanks count mismatch msg should mention 'blanks count', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_rejects_empty_blank_correct_answer(t *testing.T) {
	q := model.Question{
		Format:       "multi_blank",
		Body:         "{{1}}",
		PointCorrect: 1,
	}
	blanks := []model.QuestionBlank{
		{Index: 1, CorrectAnswer: ""},
	}
	err := validateQuestion(q, nil, blanks, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("empty blank correct_answer should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least one accepted answer") {
		t.Errorf("empty blank correct_answer msg should mention 'at least one accepted answer', got %q", err.Error())
	}
}

// ---- FB-10 accepted-answer set validation (FR-25/FR-26) ----

func TestValidateQuestion_short_rejects_empty_accepted_answers_set(t *testing.T) {
	q := model.Question{Format: "short", Body: "1+1", PointCorrect: 1, AcceptedAnswers: []string{}}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("empty accepted_answers set should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least one accepted answer") {
		t.Errorf("empty set msg should mention 'at least one accepted answer', got %q", err.Error())
	}
}

func TestValidateQuestion_short_rejects_whitespace_only_entry_in_accepted_answers(t *testing.T) {
	q := model.Question{Format: "short", Body: "1+1", PointCorrect: 1, AcceptedAnswers: []string{"2", "  "}}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("accepted_answers containing a blank entry should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("blank entry msg should mention 'cannot be empty', got %q", err.Error())
	}
}

func TestValidateQuestion_short_rejects_duplicate_after_normalisation(t *testing.T) {
	q := model.Question{Format: "short", Body: "1+1", PointCorrect: 1, AcceptedAnswers: []string{"Dua", "dua"}}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("duplicate accepted answers after normalisation should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "duplicate accepted answer") {
		t.Errorf("duplicate msg should mention 'duplicate accepted answer', got %q", err.Error())
	}
}

func TestValidateQuestion_short_accepts_multiple_accepted_answers(t *testing.T) {
	q := model.Question{Format: "short", Body: "1+1", PointCorrect: 1, AcceptedAnswers: []string{"2", "dua"}}
	if err := validateQuestion(q, nil, nil, nil); err != nil {
		t.Errorf("short with 2 distinct accepted answers should pass, got %v", err)
	}
}

func TestValidateQuestion_mcq_rejects_non_empty_accepted_answers(t *testing.T) {
	q := model.Question{Format: "mcq", Body: "x", AcceptedAnswers: []string{"a"}}
	options := []model.QuestionOption{
		{Key: "a", Text: "1", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "2", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("mcq with non-empty accepted_answers should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "mcq cannot have accepted_answers") {
		t.Errorf("mcq accepted_answers msg should mention 'mcq cannot have accepted_answers', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_answer_rejects_non_empty_accepted_answers(t *testing.T) {
	q := model.Question{Format: "multi_answer", Body: "x", AcceptedAnswers: []string{"a"}}
	options := []model.QuestionOption{
		{Key: "a", Text: "1", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "2", SortOrder: 2},
	}
	err := validateQuestion(q, options, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("multi_answer with non-empty accepted_answers should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "multi_answer cannot have accepted_answers") {
		t.Errorf("multi_answer accepted_answers msg should mention 'multi_answer cannot have accepted_answers', got %q", err.Error())
	}
}

func TestValidateQuestion_essay_rejects_non_empty_accepted_answers(t *testing.T) {
	q := model.Question{Format: "essay", Body: "explain", AcceptedAnswers: []string{"a"}}
	err := validateQuestion(q, nil, nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("essay with non-empty accepted_answers should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "essay cannot have accepted_answers") {
		t.Errorf("essay accepted_answers msg should mention 'essay cannot have accepted_answers', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_rejects_duplicate_accepted_answer_in_blank(t *testing.T) {
	q := model.Question{Format: "multi_blank", Body: "{{1}}", PointCorrect: 1}
	blanks := []model.QuestionBlank{
		{Index: 1, AcceptedAnswers: []string{"Empat", "empat"}},
	}
	err := validateQuestion(q, nil, blanks, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("multi_blank with a duplicate per-blank accepted answer should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "duplicate accepted answer") {
		t.Errorf("duplicate per-blank msg should mention 'duplicate accepted answer', got %q", err.Error())
	}
}

func TestValidateQuestion_multi_blank_accepts_per_blank_accepted_answers(t *testing.T) {
	q := model.Question{Format: "multi_blank", Body: "{{1}} and {{2}}", PointCorrect: 1}
	blanks := []model.QuestionBlank{
		{Index: 1, AcceptedAnswers: []string{"4", "empat"}},
		{Index: 2, AcceptedAnswers: []string{"jakarta"}},
	}
	if err := validateQuestion(q, nil, blanks, nil); err != nil {
		t.Errorf("multi_blank with valid per-blank accepted answers should pass, got %v", err)
	}
}

// ---- true_false validation (FR-29/FR-30) ----

func tfStatements(n int) []model.QuestionStatement {
	out := make([]model.QuestionStatement, n)
	for i := 0; i < n; i++ {
		out[i] = model.QuestionStatement{Index: i + 1, Body: fmt.Sprintf("statement %d", i+1), IsTrue: i%2 == 0}
	}
	return out
}

func TestValidateQuestion_true_false_acceptsFourContiguousStatements(t *testing.T) {
	q := model.Question{Format: "true_false", Body: "true_false stem", PointCorrect: 1}
	if err := validateQuestion(q, nil, nil, tfStatements(4)); err != nil {
		t.Errorf("true_false with 4 contiguous statements should pass, got %v", err)
	}
}

func TestValidateQuestion_true_false_rejectsSingleStatement(t *testing.T) {
	q := model.Question{Format: "true_false", Body: "true_false stem", PointCorrect: 1}
	err := validateQuestion(q, nil, nil, tfStatements(1))
	if !errors.Is(err, ErrValidation) {
		t.Errorf("true_false with 1 statement should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least 2 statements") {
		t.Errorf("msg should mention 'at least 2 statements', got %q", err.Error())
	}
}

func TestValidateQuestion_true_false_rejectsGapInIndices(t *testing.T) {
	q := model.Question{Format: "true_false", Body: "true_false stem", PointCorrect: 1}
	statements := []model.QuestionStatement{
		{Index: 1, Body: "s1", IsTrue: true},
		{Index: 3, Body: "s3", IsTrue: false},
	}
	err := validateQuestion(q, nil, nil, statements)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("true_false with a gap in indices should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "contiguous") {
		t.Errorf("msg should mention 'contiguous', got %q", err.Error())
	}
}

func TestValidateQuestion_true_false_rejectsEmptyStatementBody(t *testing.T) {
	q := model.Question{Format: "true_false", Body: "true_false stem", PointCorrect: 1}
	statements := []model.QuestionStatement{
		{Index: 1, Body: "s1", IsTrue: true},
		{Index: 2, Body: "   ", IsTrue: false},
	}
	err := validateQuestion(q, nil, nil, statements)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("true_false with an empty statement body should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "statement body cannot be empty") {
		t.Errorf("msg should mention 'statement body cannot be empty', got %q", err.Error())
	}
}

func TestValidateQuestion_true_false_rejectsOptions(t *testing.T) {
	q := model.Question{Format: "true_false", Body: "true_false stem", PointCorrect: 1}
	options := []model.QuestionOption{{Key: "a", Text: "opt"}}
	err := validateQuestion(q, options, nil, tfStatements(2))
	if !errors.Is(err, ErrValidation) {
		t.Errorf("true_false with options should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "true_false cannot have options") {
		t.Errorf("msg should mention 'true_false cannot have options', got %q", err.Error())
	}
}

func TestValidateQuestion_true_false_rejectsAcceptedAnswers(t *testing.T) {
	q := model.Question{Format: "true_false", Body: "true_false stem", PointCorrect: 1, AcceptedAnswers: []string{"x"}}
	err := validateQuestion(q, nil, nil, tfStatements(2))
	if !errors.Is(err, ErrValidation) {
		t.Errorf("true_false with accepted_answers should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "true_false cannot have accepted_answers") {
		t.Errorf("msg should mention 'true_false cannot have accepted_answers', got %q", err.Error())
	}
}

func TestValidateExam_rejects_empty_title(t *testing.T) {
	e := model.Exam{Title: "   "}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("empty title should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "exam title required") {
		t.Errorf("empty title msg should mention 'exam title required', got %q", err.Error())
	}
}

func TestValidateExam_rejects_invalid_timer_mode(t *testing.T) {
	e := model.Exam{Title: "Finals", TimerMode: "freeform"}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("invalid timer_mode should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "timer_mode must be overall or per_test") {
		t.Errorf("invalid timer_mode msg should mention 'timer_mode must be overall or per_test', got %q", err.Error())
	}
}

func TestValidateExam_requires_duration_when_overall(t *testing.T) {
	e := model.Exam{Title: "Finals", TimerMode: "overall"}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("overall with nil duration should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "duration_minutes required and positive when timer_mode=overall") {
		t.Errorf("overall nil-duration msg should mention duration requirement, got %q", err.Error())
	}

	zero := 0
	e2 := model.Exam{Title: "Finals", TimerMode: "overall", DurationMinutes: &zero}
	err = validateExam(e2)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("overall with zero duration should return ErrValidation, got %v", err)
	}
}

func TestValidateExam_accepts_valid_overall(t *testing.T) {
	e := model.Exam{Title: "Finals", TimerMode: "overall", DurationMinutes: intptr(120)}
	if err := validateExam(e); err != nil {
		t.Errorf("valid overall should pass, got %v", err)
	}
}

func TestValidateExam_accepts_valid_per_test(t *testing.T) {
	e := model.Exam{Title: "Finals", TimerMode: "per_test"}
	if err := validateExam(e); err != nil {
		t.Errorf("valid per_test should pass, got %v", err)
	}
}

func TestValidateExam_accepts_empty_timer_mode_legacy(t *testing.T) {
	e := model.Exam{Title: "Legacy", TimerMode: ""}
	if err := validateExam(e); err != nil {
		t.Errorf("empty timer_mode (legacy) should pass, got %v", err)
	}
}

func TestValidateExam_rejects_invalid_result_config(t *testing.T) {
	e := model.Exam{Title: "Finals", ResultConfig: "walkthrough"}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("invalid result_config should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "result_config must be hidden, score_only, or score_pembahasan") {
		t.Errorf("invalid result_config msg should mention allowed values, got %q", err.Error())
	}
}

func TestValidateExam_accepts_empty_result_config(t *testing.T) {
	e := model.Exam{Title: "Finals", ResultConfig: ""}
	if err := validateExam(e); err != nil {
		t.Errorf("empty result_config should pass validateExam (defaulting happens in CreateExam), got %v", err)
	}
}

func TestValidateExam_accepts_each_valid_result_config(t *testing.T) {
	for _, rc := range []string{"hidden", "score_only", "score_pembahasan"} {
		e := model.Exam{Title: "Finals", ResultConfig: rc}
		if err := validateExam(e); err != nil {
			t.Errorf("result_config=%q should pass, got %v", rc, err)
		}
	}
}

// --- FR-18: mode authoring validation ---

func TestValidateExam_rejects_invalid_mode(t *testing.T) {
	e := model.Exam{Title: "Finals", Mode: "foo"}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("invalid mode should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "mode must be") {
		t.Errorf("invalid mode msg should mention allowed modes, got %q", err.Error())
	}
}

func TestValidateExam_accepts_each_valid_mode(t *testing.T) {
	for _, m := range []string{"standard", "utbk", "ielts"} {
		e := model.Exam{Title: "Finals", Mode: m}
		if err := validateExam(e); err != nil {
			t.Errorf("mode=%q should pass, got %v", m, err)
		}
	}
}

func TestValidateExam_accepts_empty_mode(t *testing.T) {
	// empty on PATCH preserves; on CREATE, CreateExam defaults to standard before
	// validateExam runs. Either way validateExam must accept empty.
	e := model.Exam{Title: "Finals", Mode: ""}
	if err := validateExam(e); err != nil {
		t.Errorf("empty mode should pass validateExam (default/overlay happens in CreateExam/handler), got %v", err)
	}
}

// --- scheduled_end_at (availability window) validation ---

func TestValidateExam_acceptsNoScheduledEndAt(t *testing.T) {
	e := model.Exam{Title: "Finals", ScheduledAt: timePtr(fixedNow())}
	if err := validateExam(e); err != nil {
		t.Errorf("nil scheduled_end_at should pass, got %v", err)
	}
}

func TestValidateExam_rejectsScheduledEndAtWithoutScheduledAt(t *testing.T) {
	e := model.Exam{Title: "Finals", ScheduledEndAt: timePtr(fixedNow())}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("scheduled_end_at without scheduled_at should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "scheduled_end_at requires scheduled_at") {
		t.Errorf("msg should mention scheduled_at requirement, got %q", err.Error())
	}
}

func TestValidateExam_rejectsScheduledEndAtNotAfterScheduledAt(t *testing.T) {
	start := fixedNow()
	e := model.Exam{Title: "Finals", ScheduledAt: timePtr(start), ScheduledEndAt: timePtr(start)}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("scheduled_end_at == scheduled_at should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "scheduled_end_at must be after scheduled_at") {
		t.Errorf("msg should mention ordering, got %q", err.Error())
	}

	before := start.Add(-time.Hour)
	e2 := model.Exam{Title: "Finals", ScheduledAt: timePtr(start), ScheduledEndAt: timePtr(before)}
	if err := validateExam(e2); !errors.Is(err, ErrValidation) {
		t.Errorf("scheduled_end_at before scheduled_at should return ErrValidation, got %v", err)
	}
}

func TestValidateExam_acceptsValidScheduledWindow(t *testing.T) {
	start := fixedNow()
	e := model.Exam{Title: "Finals", ScheduledAt: timePtr(start), ScheduledEndAt: timePtr(start.Add(48 * time.Hour))}
	if err := validateExam(e); err != nil {
		t.Errorf("valid scheduled window should pass, got %v", err)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
}

// --- FR-18: section_type authoring validation ---

func TestValidateTest_rejects_invalid_section_type(t *testing.T) {
	invalid := "speaking"
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 60, SectionType: &invalid}
	err := validateTest(tst)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("invalid section_type should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "section_type must be") {
		t.Errorf("invalid section_type msg should mention allowed values, got %q", err.Error())
	}
}

func TestValidateTest_rejects_listening_without_audio_url(t *testing.T) {
	listening := "listening"
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 60, SectionType: &listening}
	err := validateTest(tst)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("listening without audio_url should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "audio_url required when section_type=listening") {
		t.Errorf("listening-no-audio msg should mention audio_url requirement, got %q", err.Error())
	}
}

func TestValidateTest_accepts_listening_with_audio_url(t *testing.T) {
	listening := "listening"
	audio := "https://cdn.example.com/track.mp3"
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 60, SectionType: &listening, AudioURL: &audio}
	if err := validateTest(tst); err != nil {
		t.Errorf("listening with audio_url should pass, got %v", err)
	}
}

func TestValidateTest_accepts_reading_section(t *testing.T) {
	reading := "reading"
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 60, SectionType: &reading}
	if err := validateTest(tst); err != nil {
		t.Errorf("reading section (no audio required) should pass, got %v", err)
	}
}

func TestValidateTest_accepts_null_section_type(t *testing.T) {
	// standard/utbk tests may be untyped; SectionType nil must pass.
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 60}
	if err := validateTest(tst); err != nil {
		t.Errorf("null section_type should pass, got %v", err)
	}
}

func TestValidateTest_accepts_writing_section(t *testing.T) {
	writing := "writing"
	tst := model.Test{Title: "x", Subject: "math", Topic: "algebra", DurationMinutes: 60, SectionType: &writing}
	if err := validateTest(tst); err != nil {
		t.Errorf("writing section should pass, got %v", err)
	}
}

// --- FR-19: publish-time completeness gate for sectioned modes ---

func entryTitled(title string, sectionType *string) model.ExamTestEntry {
	return model.ExamTestEntry{Test: struct {
		ID              uuid.UUID `json:"id"`
		Title           string    `json:"title"`
		Subject         string    `json:"subject"`
		Topic           *string   `json:"topic"`
		DurationMinutes *int      `json:"duration_minutes"`
		SectionType     *string   `json:"section_type,omitempty"`
		QuestionCount   int       `json:"question_count"`
	}{Title: title, SectionType: sectionType}}
}

func TestValidatePublishSections_rejects_sectioned_exam_with_zero_tests(t *testing.T) {
	for _, mode := range []string{"utbk", "ielts"} {
		exam := model.Exam{Mode: mode}
		err := validatePublishSections(exam, nil)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("mode=%s with 0 tests should return ErrValidation, got %v", mode, err)
		}
		if !strings.Contains(err.Error(), "at least one test") {
			t.Errorf("zero-tests msg should mention 'at least one test', got %q", err.Error())
		}
		err = validatePublishSections(exam, []model.ExamTestEntry{})
		if !errors.Is(err, ErrValidation) {
			t.Errorf("mode=%s with empty tests slice should return ErrValidation, got %v", mode, err)
		}
	}
}

func TestValidatePublishSections_rejects_ielts_with_untyped_section(t *testing.T) {
	exam := model.Exam{Mode: "ielts"}
	tests := []model.ExamTestEntry{
		entryTitled("Listening", strPtr("listening")),
		entryTitled("Untyped Section", nil),
	}
	err := validatePublishSections(exam, tests)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("ielts with an untyped attached section should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "Untyped Section") {
		t.Errorf("ielts untyped-section msg should name the offending section, got %q", err.Error())
	}
}

func TestValidatePublishSections_allows_fully_typed_ielts(t *testing.T) {
	exam := model.Exam{Mode: "ielts"}
	tests := []model.ExamTestEntry{
		entryTitled("Listening", strPtr("listening")),
		entryTitled("Reading", strPtr("reading")),
		entryTitled("Writing", strPtr("writing")),
	}
	if err := validatePublishSections(exam, tests); err != nil {
		t.Errorf("fully-typed ielts should pass, got %v", err)
	}
}

func TestValidatePublishSections_allows_utbk_with_untyped_tests(t *testing.T) {
	// utbk may have untyped tests per spec (FR-19); only duration_minutes>0 is
	// enforced, and that already lives in validateTest.
	exam := model.Exam{Mode: "utbk"}
	tests := []model.ExamTestEntry{
		entryTitled("Subtest 1", nil),
		entryTitled("Subtest 2", strPtr("reading")),
	}
	if err := validatePublishSections(exam, tests); err != nil {
		t.Errorf("utbk with a mix of untyped/typed tests should pass, got %v", err)
	}
}

func TestValidatePublishSections_allows_standard_with_any_tests(t *testing.T) {
	// standard publish is unchanged; the gate is skipped entirely.
	exam := model.Exam{Mode: "standard"}
	if err := validatePublishSections(exam, nil); err != nil {
		t.Errorf("standard with no tests should pass (gate skipped), got %v", err)
	}
	if err := validatePublishSections(exam, []model.ExamTestEntry{entryTitled("Any", nil)}); err != nil {
		t.Errorf("standard with untyped test should pass (gate skipped), got %v", err)
	}
}

func TestValidatePublishSections_allows_empty_mode(t *testing.T) {
	// empty mode (legacy rows / pre-default) must not trigger the gate.
	exam := model.Exam{Mode: ""}
	if err := validatePublishSections(exam, nil); err != nil {
		t.Errorf("empty mode should pass (gate skipped), got %v", err)
	}
}

func TestCheckTypeRBAC_admin_exam_allows_exam(t *testing.T) {
	if err := checkTypeRBAC(RoleAdminExam, "exam"); err != nil {
		t.Errorf("admin_exam on exam type should be allowed, got %v", err)
	}
}

func TestCheckTypeRBAC_admin_exam_blocks_book(t *testing.T) {
	err := checkTypeRBAC(RoleAdminExam, "book")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("admin_exam on book type should return ErrForbidden, got %v", err)
	}
}

func TestCheckTypeRBAC_admin_exam_allows_course(t *testing.T) {
	if err := checkTypeRBAC(RoleAdminExam, "course"); err != nil {
		t.Errorf("admin_exam on course type should be allowed, got %v", err)
	}
}

// --- FR-9..FR-15: bank question CRUD + delete-guard + list-bank ---

func seedTopicDirect(t *testing.T, ctx context.Context, repo *repository.Repository, name, subject string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_topic (name, subject) VALUES ($1, $2) RETURNING id`,
		name, subject,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedTestDirect(t *testing.T, ctx context.Context, repo *repository.Repository, title, subject, topic string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, $2, $3, $4) RETURNING id`,
		title, subject, topic, 60,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedBankQuestionDirect(t *testing.T, ctx context.Context, repo *repository.Repository, format, body string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ($1, $2, 1, 0) RETURNING id`,
		format, body,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// seedExamWithTestsDirect creates an exam and attaches the given tests to it via
// exam_test, in the order given.
func seedExamWithTestsDirect(t *testing.T, ctx context.Context, repo *repository.Repository, testIDs ...uuid.UUID) uuid.UUID {
	t.Helper()
	var examID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ($1, 'draft') RETURNING id`,
		"Exam "+uniqueSuffix(),
	).Scan(&examID)
	require.NoError(t, err)
	for i, tid := range testIDs {
		_, err := repo.Pool().Exec(ctx,
			`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`,
			examID, tid, i+1,
		)
		require.NoError(t, err)
	}
	return examID
}

// seedProductForExamDirect creates a product of the given status and links it
// to examID via product_exam (FR-7's published-product arm of "live").
func seedProductForExamDirect(t *testing.T, ctx context.Context, repo *repository.Repository, examID uuid.UUID, status string) {
	t.Helper()
	var productID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, status) VALUES ('exam', $1, 100000, $2) RETURNING id`,
		"Product "+uniqueSuffix(), status,
	).Scan(&productID)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`,
		productID, examID,
	)
	require.NoError(t, err)
}

// seedExamSessionForExamDirect creates a minimal exam_session row against examID
// (FR-7's second "live" arm, independent of any product).
func seedExamSessionForExamDirect(t *testing.T, ctx context.Context, repo *repository.Repository, examID uuid.UUID) {
	t.Helper()
	var studentID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (email, name, role, status) VALUES ($1, $2, 'student', 'active') RETURNING id`,
		"student-"+uniqueSuffix()+"@example.com", "Student",
	).Scan(&studentID)
	require.NoError(t, err)
	var regID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status) VALUES ($1, $2, $3, 'registered') RETURNING id`,
		studentID, examID, "TOKEN"+uniqueSuffix(),
	).Scan(&regID)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status) VALUES ($1, $2, $3, 1, now(), 'in_progress')`,
		regID, studentID, examID,
	)
	require.NoError(t, err)
}

func attachQuestionDirect(t *testing.T, ctx context.Context, repo *repository.Repository, testID, questionID uuid.UUID, sortOrder int) {
	t.Helper()
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`,
		testID, questionID, sortOrder,
	)
	require.NoError(t, err)
}

func answerQuestionDirect(t *testing.T, ctx context.Context, repo *repository.Repository, questionID uuid.UUID) {
	t.Helper()
	// exam_session_answer requires a session; create the minimal session row.
	var studentID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (email, name, role, status) VALUES ($1, $2, 'student', 'active') RETURNING id`,
		"student-"+uniqueSuffix()+"@example.com", "Student",
	).Scan(&studentID)
	require.NoError(t, err)
	var examID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ($1, 'draft') RETURNING id`,
		"Exam "+uniqueSuffix(),
	).Scan(&examID)
	require.NoError(t, err)
	var regID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status) VALUES ($1, $2, $3, 'registered') RETURNING id`,
		studentID, examID, "TOKEN"+uniqueSuffix(),
	).Scan(&regID)
	require.NoError(t, err)
	var sessionID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status) VALUES ($1, $2, $3, 1, now(), 'submitted') RETURNING id`,
		regID, studentID, examID,
	).Scan(&sessionID)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_session_answer (session_id, question_id, answer, saved_at) VALUES ($1, $2, $3, now())`,
		sessionID, questionID, "answer",
	)
	require.NoError(t, err)
}

func countQuestionAttachments(t *testing.T, ctx context.Context, repo *repository.Repository, id uuid.UUID) int {
	t.Helper()
	var count int
	err := repo.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM test_question WHERE question_id = $1`, id,
	).Scan(&count)
	require.NoError(t, err)
	return count
}

func listTestQuestions(t *testing.T, ctx context.Context, svc *Service, testID uuid.UUID) []model.QuestionWithOptions {
	t.Helper()
	detail, err := svc.GetTestDetail(ctx, testID)
	require.NoError(t, err)
	return detail.Questions
}

func TestCreateBankQuestion_creates_no_attachment(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	q := model.Question{Format: "essay", Body: "explain gravity", PointCorrect: 1, PointWrong: 0}
	out, err := svc.CreateBankQuestion(ctx, q, nil, nil, nil)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, out.Question.ID)
	assert.Equal(t, "essay", out.Question.Format)
	assert.Equal(t, 0, countQuestionAttachments(t, ctx, repo, out.Question.ID))
}

func TestCreateBankQuestion_rejects_body_that_sanitizes_to_empty(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	// <br> is allowlisted (FB-24) but carries no text content, so
	// isQuestionBodyEmpty still rejects it in validateQuestion — a blank
	// question must not be persisted.
	q := model.Question{Format: "essay", Body: "<br>", PointCorrect: 1, PointWrong: 0}
	_, err := svc.CreateBankQuestion(ctx, q, nil, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)

	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: "<br>", Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestListBankQuestions_populates_nested_question_and_options(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	// multi_answer (not mcq) so this doesn't collide with the unscoped
	// Format:"mcq" assertion in TestListBankQuestions_filters_and_counts_used_in.
	body := "bank list shape " + uniqueSuffix()
	q := model.Question{Format: "multi_answer", Body: body, PointCorrect: 1, PointWrong: 0}
	opts := []model.QuestionOption{
		{Key: "a", Text: "yes", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "no", IsCorrect: false, SortOrder: 2},
	}
	created, err := svc.CreateBankQuestion(ctx, q, opts, nil, nil)
	require.NoError(t, err)

	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Nested {question, options, attached_count} shape (not flattened/embedded) —
	// the admin bank page destructures item.question and reads item.options.
	assert.Equal(t, created.Question.ID, items[0].Question.ID)
	assert.Equal(t, body, items[0].Question.Body)
	require.Len(t, items[0].Options, 2)
	assert.Equal(t, "a", items[0].Options[0].Key)
	assert.Equal(t, "b", items[0].Options[1].Key)
}

// A fill_blank / short / essay question has no options. Its Options must
// serialize as [] not null — a nil slice becomes JSON null and crashes the
// admin question editor, which reads q.options.length when opening an edit.
func TestListBankQuestions_optionlessFormat_returnsNonNilOptions(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	body := "fill blank no options " + uniqueSuffix()
	seedBankQuestionDirect(t, ctx, repo, "fill_blank", body)

	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.NotNil(t, items[0].Options, "options must be a non-nil empty slice, not nil (serializes to null)")
	assert.Len(t, items[0].Options, 0)
}

// Task 6 relaxes the old "attached to any test" refusal: a question attached
// only to a draft test (no exam at all, let alone a live one) is now
// deletable, and the test_question join row goes with it via ON DELETE CASCADE.
func TestDeleteQuestion_succeeds_when_attached_to_draft_test_only(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Math "+uniqueSuffix(), "math", "algebra")
	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "explain")
	attachQuestionDirect(t, ctx, repo, testID, qID, 1)

	err := svc.DeleteQuestion(ctx, qID)
	require.NoError(t, err)

	assert.Equal(t, 0, countQuestionAttachments(t, ctx, repo, qID))
	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, qID).Scan(&exists))
	assert.False(t, exists)
}

// FR-7: a question attached (via test -> exam_test -> exam) to an exam sold
// through a published product must be refused, not just detached-and-warned.
func TestDeleteQuestion_rejects_when_examHasPublishedProduct(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Math "+uniqueSuffix(), "math", "algebra")
	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "explain live exam")
	attachQuestionDirect(t, ctx, repo, testID, qID, 1)
	examID := seedExamWithTestsDirect(t, ctx, repo, testID)
	seedProductForExamDirect(t, ctx, repo, examID, "published")

	err := svc.DeleteQuestion(ctx, qID)
	assert.ErrorIs(t, err, ErrQuestionInLiveExam)

	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, qID).Scan(&exists))
	assert.True(t, exists, "guard must be a no-op: question survives")
}

// The predicate's other arm: a draft-product exam that already has a session
// is still "live" and must refuse deletion (task's second fact about exam.status).
func TestDeleteQuestion_rejects_when_examHasSession(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Math "+uniqueSuffix(), "math", "algebra")
	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "explain session arm")
	attachQuestionDirect(t, ctx, repo, testID, qID, 1)
	examID := seedExamWithTestsDirect(t, ctx, repo, testID)
	seedExamSessionForExamDirect(t, ctx, repo, examID)

	err := svc.DeleteQuestion(ctx, qID)
	assert.ErrorIs(t, err, ErrQuestionInLiveExam)
}

// A draft exam with a draft product and no sessions is not live: delete succeeds.
func TestDeleteQuestion_succeeds_when_examDraftProductNoSessions(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Math "+uniqueSuffix(), "math", "algebra")
	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "explain draft exam")
	attachQuestionDirect(t, ctx, repo, testID, qID, 1)
	examID := seedExamWithTestsDirect(t, ctx, repo, testID)
	seedProductForExamDirect(t, ctx, repo, examID, "draft")

	err := svc.DeleteQuestion(ctx, qID)
	require.NoError(t, err)

	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, qID).Scan(&exists))
	assert.False(t, exists)
}

func TestDeleteQuestion_rejects_when_answered(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	qID := seedBankQuestionDirect(t, ctx, repo, "short", "capital of France")
	answerQuestionDirect(t, ctx, repo, qID)

	err := svc.DeleteQuestion(ctx, qID)
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "answered")

	// Guard must be a no-op: the question survives.
	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, qID).Scan(&exists))
	assert.True(t, exists)
}

func TestDeleteQuestion_succeeds_when_unattached_and_unanswered(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "explain relativity")

	err := svc.DeleteQuestion(ctx, qID)
	require.NoError(t, err)

	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, qID).Scan(&exists))
	assert.False(t, exists)
}

func TestListBankQuestions_filters_and_counts_used_in(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	topicA := seedTopicDirect(t, ctx, repo, "Algebra "+uniqueSuffix(), "math")
	topicB := seedTopicDirect(t, ctx, repo, "Geometry "+uniqueSuffix(), "math")

	// Three questions: mcq in topicA (attached to 2 tests), essay in topicB (unattached), short no topic.
	test1 := seedTestDirect(t, ctx, repo, "T1 "+uniqueSuffix(), "math", "algebra")
	test2 := seedTestDirect(t, ctx, repo, "T2 "+uniqueSuffix(), "math", "algebra")

	uniqueToken := "cursorbatch " + uniqueSuffix()

	mcqID := seedBankQuestionDirect(t, ctx, repo, "mcq", uniqueToken+" 2+2")
	_, err := repo.Pool().Exec(ctx, `UPDATE question SET topic_id = $1 WHERE id = $2`, topicA, mcqID)
	require.NoError(t, err)
	attachQuestionDirect(t, ctx, repo, test1, mcqID, 1)
	attachQuestionDirect(t, ctx, repo, test2, mcqID, 2)

	essayBody := uniqueToken + " explain photosynthesis " + uniqueSuffix()
	essayID := seedBankQuestionDirect(t, ctx, repo, "essay", essayBody)
	_, err = repo.Pool().Exec(ctx, `UPDATE question SET topic_id = $1 WHERE id = $2`, topicB, essayID)
	require.NoError(t, err)

	shortID := seedBankQuestionDirect(t, ctx, repo, "short", uniqueToken+" short")

	// Full list (filtered by unique token) returns exactly the three bank questions.
	all, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: uniqueToken, Limit: 50})
	require.NoError(t, err)
	ids := map[uuid.UUID]bool{}
	for _, it := range all {
		ids[it.Question.ID] = true
	}
	assert.True(t, ids[mcqID] && ids[essayID] && ids[shortID], "expected all three bank questions")

	// Filter by format (scoped by the unique token too — the DB is shared across the
	// whole test binary, and other tests seed mcq-format questions of their own).
	items, nextCursor, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Format: "mcq", Search: uniqueToken, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, mcqID, items[0].Question.ID)
	assert.Equal(t, 2, items[0].AttachedCount)
	assert.Empty(t, nextCursor)

	// Filter by topic_id.
	items, nextCursor, err = svc.ListBankQuestions(ctx, repository.QuestionFilter{TopicID: topicB.String(), Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, essayID, items[0].Question.ID)
	assert.Equal(t, 0, items[0].AttachedCount)
	assert.Empty(t, nextCursor)

	// Search by body substring (unique term so leftover rows don't match).
	items, nextCursor, err = svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: "photosynthesis", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, essayID, items[0].Question.ID)
	assert.Empty(t, nextCursor)

	// Cursor pagination: limit 2 on the unique-token batch should give first two rows and a cursor.
	items, nextCursor, err = svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: uniqueToken, Limit: 2})
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.NotEmpty(t, nextCursor)
	page1IDs := map[uuid.UUID]bool{items[0].Question.ID: true, items[1].Question.ID: true}

	// Follow cursor should return the remaining row.
	items, nextCursor, err = svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: uniqueToken, Limit: 2, Cursor: nextCursor})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Empty(t, nextCursor)
	assert.False(t, page1IDs[items[0].Question.ID], "cursor should advance to a new row")
}

// --- FR-21..FR-25: test ↔ question attach / detach / reorder ---

func TestAttachQuestions_appends_after_max_order_and_is_idempotent(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Attach "+uniqueSuffix(), "math", "algebra")
	q1 := seedBankQuestionDirect(t, ctx, repo, "short", "q1 "+uniqueSuffix())
	q2 := seedBankQuestionDirect(t, ctx, repo, "short", "q2 "+uniqueSuffix())
	q3 := seedBankQuestionDirect(t, ctx, repo, "short", "q3 "+uniqueSuffix())

	// First attach: q1 and q2 get orders 1 and 2.
	require.NoError(t, svc.AttachQuestions(ctx, testID, []uuid.UUID{q1, q2}))
	questions := listTestQuestions(t, ctx, svc, testID)
	require.Len(t, questions, 2)
	assert.Equal(t, q1, questions[0].Question.ID)
	assert.Equal(t, 1, questions[0].SortOrder)
	assert.Equal(t, q2, questions[1].Question.ID)
	assert.Equal(t, 2, questions[1].SortOrder)

	// Second attach includes an already-attached q2 plus a new q3: q3 appends as order 3.
	require.NoError(t, svc.AttachQuestions(ctx, testID, []uuid.UUID{q2, q3}))
	questions = listTestQuestions(t, ctx, svc, testID)
	require.Len(t, questions, 3)
	assert.Equal(t, q3, questions[2].Question.ID)
	assert.Equal(t, 3, questions[2].SortOrder)
}

func TestAttachQuestions_rejects_missing_test(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	missingTest := uuid.New()
	q := uuid.New()
	err := svc.AttachQuestions(ctx, missingTest, []uuid.UUID{q})
	assert.ErrorIs(t, err, ErrTestNotFound)
}

func TestAttachQuestions_rejects_missing_question(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Attach "+uniqueSuffix(), "math", "algebra")
	realQ := seedBankQuestionDirect(t, ctx, repo, "short", "real "+uniqueSuffix())
	missingQ := uuid.New()

	err := svc.AttachQuestions(ctx, testID, []uuid.UUID{realQ, missingQ})
	assert.ErrorIs(t, err, ErrQuestionNotFound)

	// No partial attachment must occur.
	assert.Equal(t, 0, countQuestionAttachments(t, ctx, repo, realQ))
}

func TestDetachQuestion_removes_only_join(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testA := seedTestDirect(t, ctx, repo, "A "+uniqueSuffix(), "math", "algebra")
	testB := seedTestDirect(t, ctx, repo, "B "+uniqueSuffix(), "math", "algebra")
	q := seedBankQuestionDirect(t, ctx, repo, "short", "shared "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, testA, q, 0)
	attachQuestionDirect(t, ctx, repo, testB, q, 0)

	require.NoError(t, svc.DetachQuestion(ctx, testA, q))

	assert.Equal(t, 1, countQuestionAttachments(t, ctx, repo, q))
	questionsA := listTestQuestions(t, ctx, svc, testA)
	assert.Len(t, questionsA, 0)
	questionsB := listTestQuestions(t, ctx, svc, testB)
	assert.Len(t, questionsB, 1)

	// Bank question survives.
	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, q).Scan(&exists))
	assert.True(t, exists)
}

func TestDetachQuestion_rejects_missing_test(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	err := svc.DetachQuestion(ctx, uuid.New(), uuid.New())
	assert.ErrorIs(t, err, ErrTestNotFound)
}

func TestReorderTestQuestions_rewrites_order_without_conflict(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Reorder "+uniqueSuffix(), "math", "algebra")
	q1 := seedBankQuestionDirect(t, ctx, repo, "short", "r1 "+uniqueSuffix())
	q2 := seedBankQuestionDirect(t, ctx, repo, "short", "r2 "+uniqueSuffix())
	q3 := seedBankQuestionDirect(t, ctx, repo, "short", "r3 "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, testID, q1, 0)
	attachQuestionDirect(t, ctx, repo, testID, q2, 1)
	attachQuestionDirect(t, ctx, repo, testID, q3, 2)

	// Reverse the order.
	require.NoError(t, svc.ReorderTestQuestions(ctx, testID, []uuid.UUID{q3, q2, q1}))

	questions := listTestQuestions(t, ctx, svc, testID)
	require.Len(t, questions, 3)
	assert.Equal(t, q3, questions[0].Question.ID)
	assert.Equal(t, 0, questions[0].SortOrder)
	assert.Equal(t, q2, questions[1].Question.ID)
	assert.Equal(t, 1, questions[1].SortOrder)
	assert.Equal(t, q1, questions[2].Question.ID)
	assert.Equal(t, 2, questions[2].SortOrder)
}

func TestReorderTestQuestions_rejects_mismatched_set(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Reorder "+uniqueSuffix(), "math", "algebra")
	q1 := seedBankQuestionDirect(t, ctx, repo, "short", "m1 "+uniqueSuffix())
	q2 := seedBankQuestionDirect(t, ctx, repo, "short", "m2 "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, testID, q1, 0)
	attachQuestionDirect(t, ctx, repo, testID, q2, 1)

	// Missing q2, extra q3 (not attached).
	q3 := seedBankQuestionDirect(t, ctx, repo, "short", "m3 "+uniqueSuffix())
	err := svc.ReorderTestQuestions(ctx, testID, []uuid.UUID{q1, q3})
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "must match the current attached set")
}

func TestReorderTestQuestions_rejects_duplicate_id_masquerading_as_full_set(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "Reorder "+uniqueSuffix(), "math", "algebra")
	q1 := seedBankQuestionDirect(t, ctx, repo, "short", "m1 "+uniqueSuffix())
	q2 := seedBankQuestionDirect(t, ctx, repo, "short", "m2 "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, testID, q1, 0)
	attachQuestionDirect(t, ctx, repo, testID, q2, 1)

	// Same length as the attached set, but q1 repeated and q2 missing entirely.
	err := svc.ReorderTestQuestions(ctx, testID, []uuid.UUID{q1, q1})
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "must match the current attached set")
}

func TestAttachQuestions_rejects_question_already_on_sibling_test_in_same_exam(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	test1 := seedTestDirect(t, ctx, repo, "T1 "+uniqueSuffix(), "math", "algebra")
	test2 := seedTestDirect(t, ctx, repo, "T2 "+uniqueSuffix(), "math", "algebra")
	seedExamWithTestsDirect(t, ctx, repo, test1, test2)

	qID := seedBankQuestionDirect(t, ctx, repo, "short", "shared "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, test1, qID, 1)

	err := svc.AttachQuestions(ctx, test2, []uuid.UUID{qID})
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "already attached to another test in the same exam")

	// Guard is a no-op: question remains attached only to test1.
	assert.Equal(t, 1, countQuestionAttachments(t, ctx, repo, qID))
}

func TestAttachQuestions_allows_reattaching_to_its_own_test(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "T1 "+uniqueSuffix(), "math", "algebra")
	seedExamWithTestsDirect(t, ctx, repo, testID)
	qID := seedBankQuestionDirect(t, ctx, repo, "short", "self "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, testID, qID, 1)

	// Idempotent re-attach to the SAME test must not be blocked by the sibling guard.
	err := svc.AttachQuestions(ctx, testID, []uuid.UUID{qID})
	require.NoError(t, err)
}

func TestAttachQuestions_allows_question_shared_across_different_exams(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	test1 := seedTestDirect(t, ctx, repo, "T1 "+uniqueSuffix(), "math", "algebra")
	test2 := seedTestDirect(t, ctx, repo, "T2 "+uniqueSuffix(), "math", "algebra")
	seedExamWithTestsDirect(t, ctx, repo, test1)
	seedExamWithTestsDirect(t, ctx, repo, test2)

	qID := seedBankQuestionDirect(t, ctx, repo, "short", "crossexam "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, test1, qID, 1)

	// Same question reused across tests in DIFFERENT exams is fine — only
	// sibling tests inside the SAME exam collide.
	err := svc.AttachQuestions(ctx, test2, []uuid.UUID{qID})
	require.NoError(t, err)
}

func TestCreateQuestionForTest_creates_bank_question_and_join(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	testID := seedTestDirect(t, ctx, repo, "CreateInTest "+uniqueSuffix(), "math", "algebra")
	// Pre-attach one question so the new one appends after it.
	existingQ := seedBankQuestionDirect(t, ctx, repo, "short", "existing "+uniqueSuffix())
	attachQuestionDirect(t, ctx, repo, testID, existingQ, 0)

	q := model.Question{Format: "essay", Body: "explain relativity", PointCorrect: 1, PointWrong: 0}
	out, err := svc.CreateQuestionForTest(ctx, testID, q, nil, nil, nil)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, out.Question.ID)

	// It lives in the bank.
	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM question WHERE id = $1)`, out.Question.ID).Scan(&exists))
	assert.True(t, exists)

	// It is attached to the test as the last item.
	questions := listTestQuestions(t, ctx, svc, testID)
	require.Len(t, questions, 2)
	assert.Equal(t, out.Question.ID, questions[1].Question.ID)
	assert.Equal(t, 1, questions[1].SortOrder)
}

// suppress unused: uuid is imported to avoid unused-import lint if tests get trimmed later
var _ = uuid.Nil

// --- FR-18/19: CreateExam default mode + PublishProduct's sectioned gate (integration) ---
// These exercise the service against the real Postgres fixture (testcontainers),
// matching the existing school_test.go pattern. They verify the CreateExam
// defaulting and that PublishProduct, for an exam-type product, loads every
// attached exam's Tests and delegates to validatePublishSections.

func TestCreateExam_Integration_DefaultsModeToStandard(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	title := "Default Mode Exam " + uniqueSuffix()
	exam, err := svc.CreateExam(ctx, model.Exam{Title: title, Mode: ""})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if exam.Mode != "standard" {
		t.Errorf("CreateExam with empty Mode should default to standard, got %q", exam.Mode)
	}

	// explicit mode must round-trip unchanged.
	exam2, err := svc.CreateExam(ctx, model.Exam{Title: "UTBK Exam " + uniqueSuffix(), Mode: "utbk"})
	if err != nil {
		t.Fatalf("CreateExam utbk: %v", err)
	}
	if exam2.Mode != "utbk" {
		t.Errorf("CreateExam with mode=utbk should persist utbk, got %q", exam2.Mode)
	}
}

// TestScheduledEndAt_Integration_RoundTripsThroughCreateAndUpdate proves
// migration 0036 + the widened repository queries actually persist and
// return scheduled_end_at through the real DB, not just in-memory shims.
func TestScheduledEndAt_Integration_RoundTripsThroughCreateAndUpdate(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	start := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	end := start.Add(48 * time.Hour)

	exam, err := svc.CreateExam(ctx, model.Exam{
		Title:          "Open Window Exam " + uniqueSuffix(),
		ScheduledAt:    &start,
		ScheduledEndAt: &end,
	})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if exam.ScheduledEndAt == nil || !exam.ScheduledEndAt.Equal(end) {
		t.Fatalf("CreateExam did not persist scheduled_end_at, got %v", exam.ScheduledEndAt)
	}

	fetched, err := svc.GetExam(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetExam: %v", err)
	}
	if fetched.ScheduledEndAt == nil || !fetched.ScheduledEndAt.Equal(end) {
		t.Fatalf("GetExam did not return scheduled_end_at, got %v", fetched.ScheduledEndAt)
	}

	newEnd := end.Add(24 * time.Hour)
	updateInput := fetched.Exam
	updateInput.ScheduledEndAt = &newEnd
	updated, err := svc.UpdateExam(ctx, exam.ID, updateInput)
	if err != nil {
		t.Fatalf("UpdateExam: %v", err)
	}
	if updated.ScheduledEndAt == nil || !updated.ScheduledEndAt.Equal(newEnd) {
		t.Fatalf("UpdateExam did not persist the new scheduled_end_at, got %v", updated.ScheduledEndAt)
	}
}

func TestScheduledEndAt_Integration_RejectsEndBeforeStart(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	start := time.Now().Add(24 * time.Hour)
	before := start.Add(-time.Hour)

	_, err := svc.CreateExam(ctx, model.Exam{
		Title:          "Invalid Window Exam " + uniqueSuffix(),
		ScheduledAt:    &start,
		ScheduledEndAt: &before,
	})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("CreateExam with scheduled_end_at before scheduled_at should return ErrValidation, got %v", err)
	}
}

func TestPublishProduct_Integration_RejectsSectionedExamWithNoTests(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "UTBK No-Tests " + uniqueSuffix(), Mode: "utbk"})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	product, err := svc.CreateProductWithExams(ctx, model.Product{Type: "exam", Name: exam.Title, Price: 0, Status: "draft"}, []string{exam.ID.String()}, RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateProductWithExams: %v", err)
	}
	err = svc.PublishProduct(ctx, product.ID, RoleAdminStore)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("PublishProduct on a product attaching a utbk exam with 0 tests should return ErrValidation, got %v", err)
	}
}

func TestPublishProduct_Integration_StandardExamSkipsSectionGate(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	// Standard exam with no tests must NOT be rejected by the section gate — it
	// proceeds to the underlying product publish (which may then fail for other
	// product reasons, but not with the sectioned-mode ErrValidation). We assert
	// only that the error is not the sectioned-zero-tests validation.
	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Standard No-Tests " + uniqueSuffix(), Mode: "standard"})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	product, err := svc.CreateProductWithExams(ctx, model.Product{Type: "exam", Name: exam.Title, Price: 0, Status: "draft"}, []string{exam.ID.String()}, RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateProductWithExams: %v", err)
	}
	err = svc.PublishProduct(ctx, product.ID, RoleAdminStore)
	if err != nil && strings.Contains(err.Error(), "sectioned exam") {
		t.Errorf("standard exam must not hit the sectioned gate, got %v", err)
	}
}

// --- Slice 3: registration reads + exam card ---

// fakeRegRepo is a minimal stub for the repository methods needed by
// GetExamRegistration / GetExamCard. storeRepo is a concrete *repository.Repository
// in the production Service, so we replicate the relevant logic via a shim that
// matches the existing student_test.go / store_test.go patterns.
type fakeRegRepo struct {
	regsByIDStudent map[[2]uuid.UUID]*model.RegistrationDetail
}

func newFakeRegRepo() *fakeRegRepo {
	return &fakeRegRepo{
		regsByIDStudent: map[[2]uuid.UUID]*model.RegistrationDetail{},
	}
}

func (f *fakeRegRepo) seed(reg model.RegistrationDetail) {
	f.regsByIDStudent[[2]uuid.UUID{reg.ExamRegistration.ID, reg.ExamRegistration.StudentID}] = &reg
}

func (f *fakeRegRepo) GetExamRegistrationByID(_ context.Context, regID, studentID uuid.UUID) (*model.RegistrationDetail, error) {
	key := [2]uuid.UUID{regID, studentID}
	if d, ok := f.regsByIDStudent[key]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, repository.ErrNotFound
}

// shimRegistrationService mirrors Service.GetExamRegistration against a fakeRegRepo.
type shimRegistrationService struct {
	fake *fakeRegRepo
}

func (s *shimRegistrationService) GetExamRegistration(ctx context.Context, regID, studentID string) (*model.RegistrationDetail, error) {
	rid, err := uuid.Parse(regID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid registration id", ErrValidation)
	}
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	detail, err := s.fake.GetExamRegistrationByID(ctx, rid, sid)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrRegistrationNotFound
	}
	if err == nil && detail != nil && detail.ParticipantNumber != nil {
		prefix := detail.CreatedAt
		if detail.Exam.ScheduledAt != nil {
			prefix = *detail.Exam.ScheduledAt
		}
		if wib, e := time.LoadLocation("Asia/Jakarta"); e == nil {
			prefix = prefix.In(wib)
		}
		examNo := 0
		if detail.Exam.ExamNumber != nil {
			examNo = *detail.Exam.ExamNumber
		}
		detail.ParticipantNo = fmt.Sprintf("%s-%s-%06d", prefix.Format("060102"), formatExamNumber(examNo), *detail.ParticipantNumber)
	}
	return detail, err
}

func TestGetExamRegistration_NotOwned_ReturnsErrRegistrationNotFound(t *testing.T) {
	ctx := context.Background()
	fake := newFakeRegRepo()

	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherStudent := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	regID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	detail := model.RegistrationDetail{}
	detail.ExamRegistration = model.ExamRegistration{
		ID:        regID,
		StudentID: owner,
		Token:     "ABCD1234",
		Status:    "registered",
	}
	fake.seed(detail)

	svc := &shimRegistrationService{fake: fake}

	_, err := svc.GetExamRegistration(ctx, regID.String(), otherStudent.String())
	if !errors.Is(err, ErrRegistrationNotFound) {
		t.Errorf("non-owner should get ErrRegistrationNotFound, got %v", err)
	}

	_, err = svc.GetExamRegistration(ctx, regID.String(), owner.String())
	if err != nil {
		t.Errorf("owner lookup failed, got %v", err)
	}

	absent := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	_, err = svc.GetExamRegistration(ctx, absent.String(), owner.String())
	if !errors.Is(err, ErrRegistrationNotFound) {
		t.Errorf("absent id should return ErrRegistrationNotFound, got %v", err)
	}
}

func TestGetExamRegistration_ParticipantNoFormat_IncludesExamNumber(t *testing.T) {
	ctx := context.Background()
	fake := newFakeRegRepo()

	studentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	regID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	scheduledAt := time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)
	examNumber := 42
	participantNumber := 5

	detail := model.RegistrationDetail{}
	detail.ExamRegistration = model.ExamRegistration{
		ID:                regID,
		StudentID:         studentID,
		Status:            "registered",
		ParticipantNumber: &participantNumber,
	}
	detail.Exam.ScheduledAt = &scheduledAt
	detail.Exam.ExamNumber = &examNumber
	fake.seed(detail)

	svc := &shimRegistrationService{fake: fake}

	got, err := svc.GetExamRegistration(ctx, regID.String(), studentID.String())
	if err != nil {
		t.Fatalf("GetExamRegistration: %v", err)
	}

	want := "250620-0042-000005"
	if got.ParticipantNo != want {
		t.Errorf("ParticipantNo = %q, want %q", got.ParticipantNo, want)
	}
}

// UpdateRegistrationCard fakes the repository persist call (FR-30): finds the
// seeded registration by ID (regardless of which studentID key it was seeded
// under) and stamps its CardKey.
func (f *fakeRegRepo) UpdateRegistrationCard(_ context.Context, regID uuid.UUID, key string) error {
	for _, v := range f.regsByIDStudent {
		if v.ExamRegistration.ID == regID {
			v.CardKey = &key
			return nil
		}
	}
	return repository.ErrNotFound
}

// fakeCardRenderer stands in for the Gotenberg-backed pdfGenerator so
// GetExamCard's lazy generate-once/reuse logic can be tested without a live
// Gotenberg — tracks call count so a cache-hit can assert it never re-renders.
type fakeCardRenderer struct {
	calls int
	err   error
	pdf   []byte
}

func (f *fakeCardRenderer) RenderHTML(_ context.Context, _ []byte) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.pdf, nil
}

var fakeCardPDFBytes = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF")

// shimExamCardService mirrors Service.GetExamCard against a fakeRegRepo; injected
// studentName and tenantName stand in for the system_config + Me lookups, and
// stored/uploadCalls/downloadCalls stand in for the private GCS bucket so the
// lazy generate-once/reuse cache path (FR-30) can be exercised without real
// object storage.
type shimExamCardService struct {
	fake          *fakeRegRepo
	studentName   string
	tenantName    string
	renderer      *fakeCardRenderer
	stored        map[string][]byte
	uploadCalls   int
	downloadCalls int
}

func (s *shimExamCardService) uploadCardPDF(_ context.Context, regID uuid.UUID, pdf []byte) (string, error) {
	s.uploadCalls++
	if s.stored == nil {
		s.stored = map[string][]byte{}
	}
	key := "cards/" + regID.String() + ".pdf"
	s.stored[key] = pdf
	return key, nil
}

func (s *shimExamCardService) downloadCardPDF(_ context.Context, key string) ([]byte, error) {
	s.downloadCalls++
	return s.stored[key], nil
}

func (s *shimExamCardService) GetExamCard(ctx context.Context, regID, studentID string) ([]byte, string, error) {
	detail, err := s.fake.GetExamRegistrationByID(ctx, mustParse(regID), mustParse(studentID))
	if errors.Is(err, repository.ErrNotFound) {
		return nil, "", ErrRegistrationNotFound
	}
	if err != nil {
		return nil, "", err
	}
	filename := "kartu-peserta-" + detail.Token + ".pdf"

	if detail.CardKey != nil && *detail.CardKey != "" {
		pdf, err := s.downloadCardPDF(ctx, *detail.CardKey)
		if err != nil {
			return nil, "", err
		}
		return pdf, filename, nil
	}

	// This shim exercises the plumbing (generate-once/reuse cache), not the real
	// HTML->PDF conversion, so a fixed byte stand-in serves directly as the
	// "HTML" passed to the (fake) renderer.
	pdf, err := s.renderer.RenderHTML(ctx, []byte("<html></html>"))
	if err != nil {
		return nil, "", err
	}
	rid := mustParse(regID)
	key, err := s.uploadCardPDF(ctx, rid, pdf)
	if err != nil {
		return nil, "", err
	}
	if err := s.fake.UpdateRegistrationCard(ctx, rid, key); err != nil {
		return nil, "", err
	}
	return pdf, filename, nil
}

func mustParse(s string) uuid.UUID {
	v, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

func newShimExamCardService(fake *fakeRegRepo) *shimExamCardService {
	return &shimExamCardService{
		fake:        fake,
		studentName: "Saifullah",
		tenantName:  "Akademi Bimbel",
		renderer:    &fakeCardRenderer{pdf: fakeCardPDFBytes},
	}
}

func TestGetExamCard_ReturnsPdfBytes(t *testing.T) {
	ctx := context.Background()
	fake := newFakeRegRepo()

	studentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	examID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	regID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	detail := model.RegistrationDetail{}
	detail.ExamRegistration = model.ExamRegistration{
		ID:        regID,
		StudentID: studentID,
		ExamID:    examID,
		Token:     "AB12CD34",
		Status:    "registered",
	}
	detail.Exam.ID = examID
	detail.Exam.Title = "Finals"
	detail.Exam.RequiresCheckin = false
	fake.seed(detail)

	svc := newShimExamCardService(fake)

	pdf, filename, err := svc.GetExamCard(ctx, regID.String(), studentID.String())
	if err != nil {
		t.Fatalf("GetExamCard: %v", err)
	}

	if len(pdf) < 5 {
		t.Fatalf("PDF bytes too short: %d", len(pdf))
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("PDF bytes should start with %q, got %q", "%PDF-", string(pdf[:5]))
	}

	wantPattern := regexp.MustCompile(`^kartu-peserta-[A-Z0-9]{8}\.pdf$`)
	if !wantPattern.MatchString(filename) {
		t.Errorf("filename %q does not match kartu-peserta-<8-char-token>.pdf", filename)
	}
	if svc.renderer.calls != 1 {
		t.Errorf("expected exactly 1 render call, got %d", svc.renderer.calls)
	}
	if svc.uploadCalls != 1 {
		t.Errorf("expected exactly 1 upload call, got %d", svc.uploadCalls)
	}
}

// TestGetExamCard_SecondCallReusesCachedKey covers the lazy generate-once/
// reuse contract (FR-30): once card_key is persisted, a repeat download must
// not re-render via Gotenberg, only re-fetch the cached PDF.
func TestGetExamCard_SecondCallReusesCachedKey(t *testing.T) {
	ctx := context.Background()
	fake := newFakeRegRepo()

	studentID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	regID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	detail := model.RegistrationDetail{}
	detail.ExamRegistration = model.ExamRegistration{
		ID:        regID,
		StudentID: studentID,
		Token:     "XY98ZZ11",
		Status:    "registered",
	}
	detail.Exam.Title = "Finals"
	fake.seed(detail)

	svc := newShimExamCardService(fake)

	first, _, err := svc.GetExamCard(ctx, regID.String(), studentID.String())
	if err != nil {
		t.Fatalf("first GetExamCard: %v", err)
	}
	second, _, err := svc.GetExamCard(ctx, regID.String(), studentID.String())
	if err != nil {
		t.Fatalf("second GetExamCard: %v", err)
	}

	if svc.renderer.calls != 1 {
		t.Errorf("expected the renderer to be called exactly once across both downloads, got %d", svc.renderer.calls)
	}
	if svc.uploadCalls != 1 {
		t.Errorf("expected exactly 1 upload call across both downloads, got %d", svc.uploadCalls)
	}
	if svc.downloadCalls != 1 {
		t.Errorf("expected exactly 1 cache-hit download call, got %d", svc.downloadCalls)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("expected the cached download to return the same bytes as the original render")
	}
}

// avatarKeyFromStored (exam.go) extracts the object key from a stored avatar
// reference for card generation. It must ignore the URL host entirely (so a
// student-supplied photo_url cannot cause an outbound/SSRF request) and reject
// anything that isn't a real avatars/ object.
func TestAvatarKeyFromStored(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"proxy url", "http://localhost:8080/api/v1/files/avatars/u1/pic.png", "avatars/u1/pic.png"},
		{"https proxy url", "https://stg.abakacademy.id/api/v1/files/avatars/u1/pic.png", "avatars/u1/pic.png"},
		{"bare key", "avatars/u1/pic.png", "avatars/u1/pic.png"},
		{"leading slash key", "/files/avatars/u1/pic.png", "avatars/u1/pic.png"},
		// SSRF probes: the host is never used, and non-avatar targets yield "".
		{"metadata ssrf", "http://169.254.169.254/latest/meta-data/iam/security-credentials/role", ""},
		{"loopback ssrf", "http://127.0.0.1:9000/internal/secret", ""},
		{"non-avatar files key", "http://evil.example/files/certificates/secret.pdf", ""},
		{"traversal", "http://evil.example/files/avatars/../certificates/secret.pdf", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := avatarKeyFromStored(tc.in); got != tc.want {
				t.Errorf("avatarKeyFromStored(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// --- Rich-text question body sanitization (FR-1..FR-7) ---

func TestSanitizeQuestionBody_stripsScriptTag(t *testing.T) {
	got := sanitizeQuestionBody(`<script>alert(1)</script>Hello`)
	if strings.Contains(got, "<script>") {
		t.Errorf("sanitized body must not contain <script>, got %q", got)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("sanitized body should preserve plain text, got %q", got)
	}
}

func TestSanitizeQuestionBody_stripsOnErrorAttr(t *testing.T) {
	got := sanitizeQuestionBody(`<img src=x onerror="alert(1)">`)
	if strings.Contains(strings.ToLower(got), "onerror") {
		t.Errorf("sanitized body must not contain onerror attribute, got %q", got)
	}
	if !strings.Contains(got, "<img") {
		t.Errorf("sanitized body should keep a safe <img> tag, got %q", got)
	}
	if !strings.Contains(got, "src=\"x\"") {
		t.Errorf("sanitized body should keep src=\"x\", got %q", got)
	}
}

func TestSanitizeQuestionBody_stripsPositionFromStyle(t *testing.T) {
	got := sanitizeQuestionBody(`<img src="a" style="position:fixed;top:0">`)
	lower := strings.ToLower(got)
	if strings.Contains(lower, "position") {
		t.Errorf("sanitized style must not contain 'position', got %q", got)
	}
}

func TestSanitizeQuestionBody_preservesAllowlistedTags(t *testing.T) {
	in := `<b>bold</b> <i>italic</i> <u>under</u> <sup>2</sup> <sub>i</sub>`
	got := sanitizeQuestionBody(in)
	if got != in {
		t.Errorf("allowlisted tags must round-trip unchanged\n in: %q\nout: %q", in, got)
	}
}

func TestSanitizeQuestionBody_plainTextRoundTrip(t *testing.T) {
	in := "what is 2 + 2?"
	got := sanitizeQuestionBody(in)
	if got != in {
		t.Errorf("plain text body must round-trip byte-for-byte\n in: %q\nout: %q", in, got)
	}
}

func TestSanitizeQuestionBody_preservesListTags(t *testing.T) {
	in := `<ul><li>one</li><li>two</li></ul>`
	got := sanitizeQuestionBody(in)
	if got != in {
		t.Errorf("list tags must round-trip unchanged\n in: %q\nout: %q", in, got)
	}
}

// --- Question option text sanitization (FR-14) ---

func TestCreateBankQuestion_sanitizes_option_text(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	// Test that malicious option text is sanitized at persist time
	body := "option sanitize mcq " + uniqueSuffix()
	q := model.Question{
		Format:       "mcq",
		Body:         body,
		PointCorrect: 1,
		PointWrong:   0,
	}
	opts := []model.QuestionOption{
		{Key: "a", Text: "<script>alert(1)</script>ok", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "<b>bold</b> text", IsCorrect: false, SortOrder: 2},
	}

	_, err := svc.CreateBankQuestion(ctx, q, opts, nil, nil)
	require.NoError(t, err)

	// Fetch back the created question with options via ListBankQuestions
	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	fetched := items[0]

	// Verify first option: malicious script removed, text preserved
	require.Len(t, fetched.Options, 2)
	if strings.Contains(fetched.Options[0].Text, "<script>") {
		t.Errorf("option text must not contain <script>, got %q", fetched.Options[0].Text)
	}
	if !strings.Contains(fetched.Options[0].Text, "ok") {
		t.Errorf("option text must preserve plain text, got %q", fetched.Options[0].Text)
	}

	// Verify second option: rich text preserved
	if fetched.Options[1].Text != "<b>bold</b> text" {
		t.Errorf("option text must preserve allowed tags\n in: %q\nout: %q", "<b>bold</b> text", fetched.Options[1].Text)
	}
}

// FR-16: the NUMERIC column and the pgx scan must agree on a fractional value —
// a build against the widened column alone (Task 1) does not prove this.
func TestCreateBankQuestion_roundTripsFractionalPoints(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	body := "fractional points round trip " + uniqueSuffix()
	q := model.Question{
		Format:       "essay",
		Body:         body,
		PointCorrect: 2.5,
		PointWrong:   0.25,
	}

	_, err := svc.CreateBankQuestion(ctx, q, nil, nil, nil)
	require.NoError(t, err)

	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)

	if items[0].Question.PointCorrect != 2.5 {
		t.Errorf("PointCorrect round trip: want 2.5, got %v", items[0].Question.PointCorrect)
	}
	if items[0].Question.PointWrong != 0.25 {
		t.Errorf("PointWrong round trip: want 0.25, got %v", items[0].Question.PointWrong)
	}
}

func TestSaveQuestion_sanitizes_option_text(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	// Create a question first
	body := "save question sanitize " + uniqueSuffix()
	q := model.Question{
		Format:       "mcq",
		Body:         body,
		PointCorrect: 1,
		PointWrong:   0,
	}
	opts := []model.QuestionOption{
		{Key: "a", Text: "<script>alert(1)</script>safe", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "no", IsCorrect: false, SortOrder: 2},
	}

	out, err := svc.CreateBankQuestion(ctx, q, opts, nil, nil)
	require.NoError(t, err)
	qid := out.Question.ID

	// Update the question with new malicious option text
	updatedBody := "save question updated " + uniqueSuffix()
	updatedOpts := []model.QuestionOption{
		{Key: "a", Text: "<img src=x onerror=\"alert(1)\">updated", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "no update", IsCorrect: false, SortOrder: 2},
	}
	q.ID = qid
	q.Body = updatedBody

	_, err = svc.SaveQuestion(ctx, q, updatedOpts, nil, nil)
	require.NoError(t, err)

	// Verify sanitization happened at persist time via ListBankQuestions
	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: updatedBody, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	fetched := items[0]

	if strings.Contains(strings.ToLower(fetched.Options[0].Text), "onerror") {
		t.Errorf("option text must not contain onerror attribute, got %q", fetched.Options[0].Text)
	}
	if !strings.Contains(fetched.Options[0].Text, "updated") {
		t.Errorf("option text must preserve plain text, got %q", fetched.Options[0].Text)
	}
}

// FR-14: changing format on a question attached to a live exam is refused
// before any write, regardless of other field changes in the same request.
func TestSaveQuestion_rejects_formatChange_whenInLiveExam(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	qID := seedBankQuestionDirect(t, ctx, repo, "mcq", "original mcq body "+uniqueSuffix())
	// mcq needs options to be a valid question at read time, but the format-lock
	// check must fire before validateQuestion ever runs, so this is fine as-is.
	testID := seedTestDirect(t, ctx, repo, "Math "+uniqueSuffix(), "math", "algebra")
	attachQuestionDirect(t, ctx, repo, testID, qID, 1)
	examID := seedExamWithTestsDirect(t, ctx, repo, testID)
	seedProductForExamDirect(t, ctx, repo, examID, "published")

	q := model.Question{ID: qID, Format: "short", Body: "changed body", PointCorrect: 1, PointWrong: 0, AcceptedAnswers: []string{"x"}}
	_, err := svc.SaveQuestion(ctx, q, nil, nil, nil)
	assert.ErrorIs(t, err, ErrQuestionFormatLocked)

	var storedFormat, storedBody string
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT format, body FROM question WHERE id = $1`, qID).Scan(&storedFormat, &storedBody))
	assert.Equal(t, "mcq", storedFormat)
	assert.NotEqual(t, "changed body", storedBody, "refused format change must write nothing")
}

// FR-13: fields other than format may change freely on a live-exam question.
func TestSaveQuestion_allows_nonFormatChange_whenInLiveExam(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "original essay body "+uniqueSuffix())
	testID := seedTestDirect(t, ctx, repo, "Math "+uniqueSuffix(), "math", "algebra")
	attachQuestionDirect(t, ctx, repo, testID, qID, 1)
	examID := seedExamWithTestsDirect(t, ctx, repo, testID)
	seedProductForExamDirect(t, ctx, repo, examID, "published")

	updatedBody := "updated essay body " + uniqueSuffix()
	q := model.Question{ID: qID, Format: "essay", Body: updatedBody, PointCorrect: 1, PointWrong: 0}
	_, err := svc.SaveQuestion(ctx, q, nil, nil, nil)
	require.NoError(t, err)

	var storedBody string
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT body FROM question WHERE id = $1`, qID).Scan(&storedBody))
	assert.Equal(t, updatedBody, storedBody)
}

// FR-15: a question that is not in a live exam is free to change format, and
// the new format's own validation rules apply.
func TestSaveQuestion_allows_formatChange_whenNotInLiveExam(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "not attached to anything "+uniqueSuffix())

	q := model.Question{ID: qID, Format: "short", Body: "now a short question", PointCorrect: 1, PointWrong: 0, AcceptedAnswers: []string{"42"}}
	_, err := svc.SaveQuestion(ctx, q, nil, nil, nil)
	require.NoError(t, err)

	var storedFormat string
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT format FROM question WHERE id = $1`, qID).Scan(&storedFormat))
	assert.Equal(t, "short", storedFormat)
}

// FR-15's other half: the new format's validation still applies — a "short"
// question requires an accepted answer, so an empty one must still be rejected
// even though the question isn't in a live exam.
func TestSaveQuestion_formatChange_stillValidatesNewFormat(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	qID := seedBankQuestionDirect(t, ctx, repo, "essay", "not attached "+uniqueSuffix())

	q := model.Question{ID: qID, Format: "short", Body: "now a short question", PointCorrect: 1, PointWrong: 0}
	_, err := svc.SaveQuestion(ctx, q, nil, nil, nil)
	assert.ErrorIs(t, err, ErrValidation)
}

// Task 6: the bank list exposes in_live_exam so the UI can disable controls
// without a second round trip.
func TestListBankQuestions_exposes_inLiveExamFlag(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	liveBody := "in live exam list flag " + uniqueSuffix()
	liveID := seedBankQuestionDirect(t, ctx, repo, "essay", liveBody)
	testID := seedTestDirect(t, ctx, repo, "Math "+uniqueSuffix(), "math", "algebra")
	attachQuestionDirect(t, ctx, repo, testID, liveID, 1)
	examID := seedExamWithTestsDirect(t, ctx, repo, testID)
	seedProductForExamDirect(t, ctx, repo, examID, "published")

	notLiveBody := "not in live exam list flag " + uniqueSuffix()
	notLiveID := seedBankQuestionDirect(t, ctx, repo, "essay", notLiveBody)

	liveItems, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: liveBody, Limit: 10})
	require.NoError(t, err)
	require.Len(t, liveItems, 1)
	assert.True(t, liveItems[0].InLiveExam)
	assert.Equal(t, liveID, liveItems[0].Question.ID)

	notLiveItems, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: notLiveBody, Limit: 10})
	require.NoError(t, err)
	require.Len(t, notLiveItems, 1)
	assert.False(t, notLiveItems[0].InLiveExam)
	assert.Equal(t, notLiveID, notLiveItems[0].Question.ID)
}

// FR-27: accepted_answers round-trips through create -> read -> update -> read,
// and question.correct_answer (the legacy scalar column) always holds the first
// entry of the CURRENT accepted-answer set.
func TestAcceptedAnswers_roundTripsAndStampsScalarCorrectAnswer_FR27(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	body := "FB-10 accepted answers round trip " + uniqueSuffix()
	q := model.Question{
		Format:          "short",
		Body:            body,
		PointCorrect:    1,
		AcceptedAnswers: []string{"2", "dua"},
	}

	created, err := svc.CreateBankQuestion(ctx, q, nil, nil, nil)
	require.NoError(t, err)
	qid := created.Question.ID

	fetch := func() model.Question {
		t.Helper()
		items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
		require.NoError(t, err)
		require.Len(t, items, 1)
		return items[0].Question
	}

	afterCreate := fetch()
	assert.Equal(t, []string{"2", "dua"}, afterCreate.AcceptedAnswers)
	require.NotNil(t, afterCreate.CorrectAnswer)
	assert.Equal(t, "2", *afterCreate.CorrectAnswer, "correct_answer must hold the first accepted answer")

	// Update to three entries with a different first entry.
	q.ID = qid
	q.AcceptedAnswers = []string{"dua", "2", "empat"}
	_, err = svc.SaveQuestion(ctx, q, nil, nil, nil)
	require.NoError(t, err)

	afterUpdate := fetch()
	assert.Equal(t, []string{"dua", "2", "empat"}, afterUpdate.AcceptedAnswers)
	require.NotNil(t, afterUpdate.CorrectAnswer)
	assert.Equal(t, "dua", *afterUpdate.CorrectAnswer, "correct_answer must track the first entry of the updated set")
}

// TestStatements_roundTripsCreateReadUpdateDeleteRow proves the question_statement
// child table follows the same delete-then-insert transaction shape already used for
// options/blanks (FB-6): 4 statements persist on create, an update to 3 leaves the
// removed row gone from the real question_statement table, not just absent from the
// read shape.
func TestStatements_roundTripsCreateReadUpdateDeleteRow(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	body := "FB-6 statements round trip " + uniqueSuffix()
	q := model.Question{Format: "true_false", Body: body, PointCorrect: 1}
	statements := []model.QuestionStatement{
		{Index: 1, Body: "statement one", IsTrue: true},
		{Index: 2, Body: "statement two", IsTrue: false},
		{Index: 3, Body: "statement three", IsTrue: true},
		{Index: 4, Body: "statement four", IsTrue: false},
	}

	created, err := svc.CreateBankQuestion(ctx, q, nil, nil, statements)
	require.NoError(t, err)
	qid := created.Question.ID

	fetch := func() model.QuestionWithOptions {
		t.Helper()
		items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
		require.NoError(t, err)
		require.Len(t, items, 1)
		return model.QuestionWithOptions{Question: items[0].Question}
	}

	afterCreate := fetch()
	require.Len(t, afterCreate.Question.Statements, 4)
	for i, st := range afterCreate.Question.Statements {
		assert.Equal(t, i+1, st.Index)
		assert.Equal(t, statements[i].Body, st.Body)
		assert.Equal(t, statements[i].IsTrue, st.IsTrue)
	}

	var rowCount int
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM question_statement WHERE question_id = $1`, qid).Scan(&rowCount))
	assert.Equal(t, 4, rowCount)

	// Update to 3 statements.
	q.ID = qid
	updated := []model.QuestionStatement{
		{Index: 1, Body: "updated one", IsTrue: false},
		{Index: 2, Body: "updated two", IsTrue: true},
		{Index: 3, Body: "updated three", IsTrue: false},
	}
	_, err = svc.SaveQuestion(ctx, q, nil, nil, updated)
	require.NoError(t, err)

	afterUpdate := fetch()
	require.Len(t, afterUpdate.Question.Statements, 3)
	for i, st := range afterUpdate.Question.Statements {
		assert.Equal(t, i+1, st.Index)
		assert.Equal(t, updated[i].Body, st.Body)
		assert.Equal(t, updated[i].IsTrue, st.IsTrue)
	}

	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM question_statement WHERE question_id = $1`, qid).Scan(&rowCount))
	assert.Equal(t, 3, rowCount, "the removed 4th statement row must be gone, not just absent from the read shape")
}

func TestCreateQuestionForTest_sanitizes_option_text(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	// Create a test first
	testID := seedTestDirect(t, ctx, repo, "test for question "+uniqueSuffix(), "math", "algebra")

	body := "question for test sanitize " + uniqueSuffix()
	q := model.Question{
		Format:       "mcq",
		Body:         body,
		PointCorrect: 1,
		PointWrong:   0,
	}
	opts := []model.QuestionOption{
		{Key: "a", Text: "<script>alert(1)</script>answer", IsCorrect: true, SortOrder: 1},
		{Key: "b", Text: "plain answer", IsCorrect: false, SortOrder: 2},
	}

	_, err := svc.CreateQuestionForTest(ctx, testID, q, opts, nil, nil)
	require.NoError(t, err)

	// Verify option text was sanitized via ListBankQuestions
	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	fetched := items[0]

	if strings.Contains(fetched.Options[0].Text, "<script>") {
		t.Errorf("option text must not contain <script>, got %q", fetched.Options[0].Text)
	}
	if !strings.Contains(fetched.Options[0].Text, "answer") {
		t.Errorf("option text must preserve plain text, got %q", fetched.Options[0].Text)
	}
}

func TestProcessQuestionImportRows_sanitizes_option_text(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	// Create a topic first for import
	subject := "Math"
	topicName := "Algebra " + uniqueSuffix()
	seedTopicDirect(t, ctx, repo, topicName, subject)

	// Create an import row with malicious option text
	rows := []QuestionImportRow{
		{
			Subject:      subject,
			Topic:        topicName,
			Format:       "mcq",
			Body:         "What is 2+2? " + uniqueSuffix(),
			PointCorrect: 1,
			PointWrong:   0,
			Options: []model.QuestionOption{
				{Key: "a", Text: "<script>alert(1)</script>4", IsCorrect: true, SortOrder: 1},
				{Key: "b", Text: "5", IsCorrect: false, SortOrder: 2},
			},
		},
	}

	result, err := svc.ProcessQuestionImportRows(ctx, rows)
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)
	require.Equal(t, "inserted", result.Rows[0].Status)
	require.NotNil(t, result.Rows[0].QuestionID)

	// Verify option text was sanitized via ListBankQuestions
	body := rows[0].Body
	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{Search: body, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	fetched := items[0]

	if strings.Contains(fetched.Options[0].Text, "<script>") {
		t.Errorf("option text must not contain <script>, got %q", fetched.Options[0].Text)
	}
	if !strings.Contains(fetched.Options[0].Text, "4") {
		t.Errorf("option text must preserve plain text, got %q", fetched.Options[0].Text)
	}
}

// ---------- tests: certificate design staleness — single-blob compare (FR-14/C3/FR-27) ----------

// TestCertificateDesignBlobChanged proves UpdateExam's staleness bump (rawMessagePtrEqual
// on exam.CertificateDesign) fires for ANY change inside the consolidated blob — template,
// background_key, or layout fields, since Task 8 folded all three into one JSON column —
// and stays quiet for an unrelated field change (title), since the blob itself is untouched.
func TestCertificateDesignBlobChanged(t *testing.T) {
	classicKeyA := json.RawMessage(`{"template":"classic","background_key":"certificates/bg/a.png","fields":[]}`)
	classicKeyASame := json.RawMessage(`{"template":"classic","background_key":"certificates/bg/a.png","fields":[]}`)
	modern := json.RawMessage(`{"template":"modern","background_key":"certificates/bg/a.png","fields":[]}`)
	keyB := json.RawMessage(`{"template":"classic","background_key":"certificates/bg/b.png","fields":[]}`)
	keyCleared := json.RawMessage(`{"template":"classic","fields":[]}`)
	layoutB := json.RawMessage(`{"template":"classic","background_key":"certificates/bg/a.png","fields":[{"id":"title"}]}`)

	cases := []struct {
		name string
		old  model.Exam
		new  model.Exam
		want bool
	}{
		{
			name: "identical blob",
			old:  model.Exam{CertificateDesign: &classicKeyA},
			new:  model.Exam{CertificateDesign: &classicKeyASame},
			want: false,
		},
		{
			name: "template changed",
			old:  model.Exam{CertificateDesign: &classicKeyA},
			new:  model.Exam{CertificateDesign: &modern},
			want: true,
		},
		{
			name: "background key changed",
			old:  model.Exam{CertificateDesign: &classicKeyA},
			new:  model.Exam{CertificateDesign: &keyB},
			want: true,
		},
		{
			name: "background key cleared",
			old:  model.Exam{CertificateDesign: &classicKeyA},
			new:  model.Exam{CertificateDesign: &keyCleared},
			want: true,
		},
		{
			name: "layout fields changed",
			old:  model.Exam{CertificateDesign: &classicKeyA},
			new:  model.Exam{CertificateDesign: &layoutB},
			want: true,
		},
		{
			name: "unrelated field only (title), blob untouched",
			old:  model.Exam{CertificateDesign: &classicKeyA, Title: "Old Title"},
			new:  model.Exam{CertificateDesign: &classicKeyASame, Title: "New Title"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := !rawMessagePtrEqual(tc.old.CertificateDesign, tc.new.CertificateDesign)
			if got != tc.want {
				t.Errorf("blob changed(%+v, %+v) = %v, want %v", tc.old, tc.new, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Task 8: certificate design endpoints (validateExam layout gate, GetCertificateDesign,
// GetCertificatePreviewWithLayout)
// ---------------------------------------------------------------------------

func TestValidateExam_rejects_unknown_certificate_layout_field_id(t *testing.T) {
	raw := json.RawMessage(`{"page":{"width_mm":297,"height_mm":210},"background":{"kind":"builtin","ref":"classic"},"fields":[{"id":"not_a_real_field","x_mm":10,"y_mm":10,"w_mm":50,"align":"center","visible":true}]}`)
	e := model.Exam{Title: "Layout Exam", CertificateDesign: &raw}
	err := validateExam(e)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("unknown field id should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "unknown field id") {
		t.Errorf("error should mention 'unknown field id', got %q", err.Error())
	}
}

func TestValidateExam_accepts_valid_certificate_layout(t *testing.T) {
	raw := json.RawMessage(`{"page":{"width_mm":297,"height_mm":210},"background":{"kind":"builtin","ref":"classic"},"fields":[{"id":"title","x_mm":10,"y_mm":10,"w_mm":50,"align":"center","visible":true}]}`)
	e := model.Exam{Title: "Layout Exam", CertificateDesign: &raw}
	if err := validateExam(e); err != nil {
		t.Errorf("valid layout should pass, got %v", err)
	}
}

func TestUpdateExam_Integration_AllowsUnrelatedEditWithLegacyCertificateLayout(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Legacy Layout " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	legacyDesign := `{"template":"classic","page":{"width_mm":280,"height_mm":200},"background":{"kind":"builtin","ref":"classic"},"fields":[{"id":"title","x_mm":10,"y_mm":10,"w_mm":50,"align":"center","visible":true}]}`
	if _, err := repo.Pool().Exec(ctx, `UPDATE exam SET certificate_design = $1::jsonb WHERE id = $2`, legacyDesign, exam.ID); err != nil {
		t.Fatalf("seed legacy certificate design: %v", err)
	}

	fetched, err := svc.GetExam(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetExam: %v", err)
	}
	update := fetched.Exam
	update.Title = "Updated without touching certificate"
	if _, err := svc.UpdateExam(ctx, exam.ID, update); err != nil {
		t.Fatalf("unrelated update should not revalidate stored certificate design: %v", err)
	}
}

func TestGetCertificateDesign_Integration_UntouchedExam_ReturnsBuiltinDefaultLayout(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Design Default Exam " + uniqueSuffix(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	// certificate_enabled is only ever mutated through the dedicated action
	// (never CreateExam/UpdateExam), so tests exercising GetCertificateDesign
	// must flip it explicitly.
	if _, err := svc.SetExamCertificateEnabled(ctx, exam.ID, true); err != nil {
		t.Fatalf("SetExamCertificateEnabled: %v", err)
	}

	design, err := svc.GetCertificateDesign(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetCertificateDesign: %v", err)
	}
	if design.Template != "classic" {
		t.Errorf("Template: want classic, got %q", design.Template)
	}
	if design.BackgroundURL != nil {
		t.Errorf("BackgroundURL: want nil for an untouched exam, got %v", *design.BackgroundURL)
	}
	if len(design.Layout.Fields) == 0 {
		t.Fatal("expected the built-in default layout, got zero fields")
	}
}

func TestGetCertificateDesign_Integration_UnknownExam_ReturnsErrExamNotFound(t *testing.T) {
	svc, _ := newRealDBService(t)
	_, err := svc.GetCertificateDesign(context.Background(), uuid.New())
	if !errors.Is(err, ErrExamNotFound) {
		t.Errorf("want ErrExamNotFound, got %v", err)
	}
}

// TestGetCertificateDesign_Integration_CustomBackground_ReturnsPresignedURLNotRawKey
// proves FR-18: the DB stores only the object key, and GetCertificateDesign
// always signs a fresh time-limited GET rather than ever returning the key itself.
func TestGetCertificateDesign_Integration_CustomBackground_ReturnsPresignedURLNotRawKey(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()

	// Presigning is a pure local computation once Region is set explicitly (see
	// presignStorage's comment on why): it never needs a reachable endpoint.
	client, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("test-access", "test-secret", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	svc := NewWithStore(
		repo, repo, nil, nil,
		&NoopOTPProvider{}, &NoopEmailProvider{}, nil, nil,
		client, &config.Config{ObjectStorageBucketName: "test-bucket", ObjectStorageRegion: "us-east-1"},
		nil,
	)

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Design Presign Exam " + uniqueSuffix(), CertificateDesign: certDesignJSON("custom")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if _, err := svc.SetExamCertificateEnabled(ctx, exam.ID, true); err != nil {
		t.Fatalf("SetExamCertificateEnabled: %v", err)
	}
	key := "avatars/admin/" + uuid.NewString() + "-bg.png"
	designWithKey := json.RawMessage(`{"template":"custom","background_key":"` + key + `"}`)
	exam.CertificateDesign = &designWithKey
	if _, err := svc.UpdateExam(ctx, exam.ID, exam); err != nil {
		t.Fatalf("UpdateExam: %v", err)
	}

	design, err := svc.GetCertificateDesign(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetCertificateDesign: %v", err)
	}
	if design.BackgroundURL == nil {
		t.Fatal("expected a non-nil BackgroundURL for a custom background")
	}
	if *design.BackgroundURL == key {
		t.Errorf("BackgroundURL must be presigned, not the raw key: got %q", *design.BackgroundURL)
	}
	if !strings.Contains(*design.BackgroundURL, "X-Amz-Signature") {
		t.Errorf("expected a presigned URL (X-Amz-Signature query param), got %q", *design.BackgroundURL)
	}
	// The editor replaces the design wholesale on save, so the read model must
	// also hand back the raw key — otherwise a save that never touched the
	// background sends background_key:null and erases the upload.
	if design.BackgroundKey == nil {
		t.Fatal("expected BackgroundKey to be returned so the editor can round-trip it")
	}
	if *design.BackgroundKey != key {
		t.Errorf("BackgroundKey = %q, want %q", *design.BackgroundKey, key)
	}
}

// TestGetCertificateDesign_Integration_BackgroundURL_TTLAtMost15Min proves FR-7:
// the presigned background URL must expire within 15 minutes, not the old
// hour-long default — once results are hidden, this TTL is the entire residual
// window an already-signed URL keeps working.
func TestGetCertificateDesign_Integration_BackgroundURL_TTLAtMost15Min(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()

	client, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("test-access", "test-secret", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	svc := NewWithStore(
		repo, repo, nil, nil,
		&NoopOTPProvider{}, &NoopEmailProvider{}, nil, nil,
		client, &config.Config{ObjectStorageBucketName: "test-bucket", ObjectStorageRegion: "us-east-1"},
		nil,
	)

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "TTL Exam " + uniqueSuffix(), CertificateDesign: certDesignJSON("custom")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if _, err := svc.SetExamCertificateEnabled(ctx, exam.ID, true); err != nil {
		t.Fatalf("SetExamCertificateEnabled: %v", err)
	}
	key := "avatars/admin/" + uuid.NewString() + "-bg.png"
	designWithKey := json.RawMessage(`{"template":"custom","background_key":"` + key + `"}`)
	exam.CertificateDesign = &designWithKey
	if _, err := svc.UpdateExam(ctx, exam.ID, exam); err != nil {
		t.Fatalf("UpdateExam: %v", err)
	}

	design, err := svc.GetCertificateDesign(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetCertificateDesign: %v", err)
	}
	if design.BackgroundURL == nil {
		t.Fatal("expected a non-nil BackgroundURL for a custom background")
	}
	parsed, err := url.Parse(*design.BackgroundURL)
	if err != nil {
		t.Fatalf("parse BackgroundURL: %v", err)
	}
	expires, err := strconv.Atoi(parsed.Query().Get("X-Amz-Expires"))
	if err != nil {
		t.Fatalf("parse X-Amz-Expires: %v", err)
	}
	if expires > 15*60 {
		t.Errorf("X-Amz-Expires = %ds, want <= 900s (15 minutes)", expires)
	}
}

// TestGetCertificateDesign_Integration_NoCustomBackground_ReturnsNilKey pins the
// other half of the round-trip: an exam without an upload must not invent a key.
func TestGetCertificateDesign_Integration_NoCustomBackground_ReturnsNilKey(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "No Background Exam " + uniqueSuffix(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	if _, err := svc.SetExamCertificateEnabled(ctx, exam.ID, true); err != nil {
		t.Fatalf("SetExamCertificateEnabled: %v", err)
	}

	design, err := svc.GetCertificateDesign(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetCertificateDesign: %v", err)
	}
	if design.BackgroundKey != nil {
		t.Errorf("BackgroundKey = %q, want nil", *design.BackgroundKey)
	}
	if design.BackgroundURL != nil {
		t.Errorf("BackgroundURL = %q, want nil", *design.BackgroundURL)
	}
}

// TestGetCertificatePreviewPDF_Integration_EmptyHTML_ReturnsValidationError
// covers the async redesign's replacement preview contract (2026-08-02): the
// FE now sends fully-serialized HTML (web/app/api/admin/certificate-template
// + the editor's own preview values), so there is no server-side layout to
// validate here — only that html was actually sent.
func TestGetCertificatePreviewPDF_Integration_EmptyHTML_ReturnsValidationError(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Preview Override Exam " + uniqueSuffix(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}

	_, err = svc.GetCertificatePreviewPDF(ctx, exam.ID, "")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation for empty html, got %v", err)
	}
}

// TestGetCertificatePreviewPDF_Integration_UnknownExam_ReturnsErrExamNotFound
// proves the exam lookup 404s before any Gotenberg round trip.
func TestGetCertificatePreviewPDF_Integration_UnknownExam_ReturnsErrExamNotFound(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	_, err := svc.GetCertificatePreviewPDF(ctx, uuid.New(), "<html></html>")
	if !errors.Is(err, ErrExamNotFound) {
		t.Errorf("want ErrExamNotFound, got %v", err)
	}
}

// --- FR-32: admin participant roster ---

func TestFormatParticipantNo_ComposesYYMMDDExamNumberParticipantNumber(t *testing.T) {
	prefix := time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)
	got := formatParticipantNo(prefix, 42, 5)
	want := "250620-0042-000005"
	if got != want {
		t.Errorf("formatParticipantNo = %q, want %q", got, want)
	}
}

func seedStudentDirect(t *testing.T, ctx context.Context, repo *repository.Repository, name, username string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, password_hash) VALUES ($1, $2, 'student', 'active', '') RETURNING id`,
		name, username,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestAdminGetExamRoster_OrdersByParticipantNumber_NilSafeForMissingNumbers(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	scheduledAt := time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)
	var examID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, scheduled_at) VALUES ($1, 'draft', $2) RETURNING id`,
		"Roster Exam "+uniqueSuffix(), scheduledAt,
	).Scan(&examID)
	require.NoError(t, err)

	var examNumber int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT exam_number FROM exam WHERE id = $1`, examID,
	).Scan(&examNumber))

	studentWithNo := seedStudentDirect(t, ctx, repo, "Andi Saputra", "andi-"+uniqueSuffix())
	studentNoNo := seedStudentDirect(t, ctx, repo, "Budi Santoso", "budi-"+uniqueSuffix())

	// studentWithNo has a stored participant_number (FR-24); studentNoNo
	// predates the backfill and has none — the roster row's display number
	// must degrade to "" rather than crash or show a bogus number.
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number)
		VALUES ($1, $2, $3, 'registered', 5)`,
		studentWithNo, examID, "TOKEN"+uniqueSuffix(),
	)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status)
		VALUES ($1, $2, $3, 'registered')`,
		studentNoNo, examID, "TOKEN"+uniqueSuffix(),
	)
	require.NoError(t, err)

	rows, err := svc.AdminGetExamRoster(ctx, examID, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// ORDER BY participant_number NULLS LAST: the numbered row comes first.
	assert.Equal(t, studentWithNo, rows[0].StudentID)
	assert.Equal(t, "Andi Saputra", rows[0].StudentName)
	require.NotNil(t, rows[0].StudentUsername)
	require.NotNil(t, rows[0].ParticipantNumber)
	assert.Equal(t, 5, *rows[0].ParticipantNumber)
	want := fmt.Sprintf("250620-%s-000005", formatExamNumber(examNumber))
	assert.Equal(t, want, rows[0].ParticipantNo)

	assert.Equal(t, studentNoNo, rows[1].StudentID)
	assert.Nil(t, rows[1].ParticipantNumber)
	assert.Empty(t, rows[1].ParticipantNo, "nil-safe: missing participant_number must not render a bogus number")
}

func TestValidateExam_rejects_too_many_card_notes(t *testing.T) {
	notes := make([]string, maxCardNotes+1)
	for i := range notes {
		notes[i] = "ok"
	}
	err := validateExam(model.Exam{Title: "Finals", CardNotes: notes})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("over-cap card_notes should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "card_notes cannot exceed") {
		t.Errorf("msg should mention the entry cap, got %q", err.Error())
	}
}

func TestValidateExam_accepts_card_notes_at_the_cap(t *testing.T) {
	notes := make([]string, maxCardNotes)
	for i := range notes {
		notes[i] = strings.Repeat("a", maxCardNoteLen)
	}
	if err := validateExam(model.Exam{Title: "Finals", CardNotes: notes}); err != nil {
		t.Errorf("exactly %d notes of %d chars must be accepted, got %v", maxCardNotes, maxCardNoteLen, err)
	}
}

// Length is counted in runes: the card's column budget is characters, and a
// byte count would reject accented copy that renders on one line.
func TestValidateExam_rejects_overlong_card_note_by_runes(t *testing.T) {
	err := validateExam(model.Exam{
		Title:     "Finals",
		CardNotes: []string{strings.Repeat("é", maxCardNoteLen+1)},
	})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("overlong note should return ErrValidation, got %v", err)
	}

	if err := validateExam(model.Exam{
		Title:     "Finals",
		CardNotes: []string{strings.Repeat("é", maxCardNoteLen)},
	}); err != nil {
		t.Errorf("%d multi-byte runes must fit the rune cap, got %v", maxCardNoteLen, err)
	}
}

func TestNormalizeCardNotes_drops_blanks_and_trims(t *testing.T) {
	got := normalizeCardNotes([]string{"  Bawa kartu.  ", "", "   ", "Datang awal."})
	want := []string{"Bawa kartu.", "Datang awal."}
	if len(got) != len(want) {
		t.Fatalf("got %d notes %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("note %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveCardNotes_falls_back_to_defaults(t *testing.T) {
	if got := resolveCardNotes(nil); len(got) != len(defaultCardNotes) {
		t.Errorf("nil notes should fall back to the defaults, got %q", got)
	}
	if got := resolveCardNotes([]string{"custom"}); len(got) != 1 || got[0] != "custom" {
		t.Errorf("authored notes must win over the defaults, got %q", got)
	}
}

// cardNotesHTML output is spliced past substituteTemplateTokens' escaping, so
// it must escape the note text itself.
func TestCardNotesHTML_escapes_note_text(t *testing.T) {
	got := cardNotesHTML([]string{`<script>x</script>`}, `<b>note</b>`)
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>") {
		t.Errorf("note text must be escaped, got %q", got)
	}
	if strings.Count(got, "<li>") != 2 {
		t.Errorf("want one <li> per note plus the footer note, got %q", got)
	}
}
