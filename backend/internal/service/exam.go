package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/microcosm-cc/bluemonday"
	"github.com/minio/minio-go/v7"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

var validQuestionFormats = map[string]bool{
	"mcq":          true,
	"multi_answer": true,
	"short":        true,
	"fill_blank":   true,
	"multi_blank":  true,
	"essay":        true,
	"true_false":   true,
}

// validModes enumerates exam.mode values (FR-1). Empty is allowed here so that
// PATCH-overlay (absent preserves) and CreateExam defaulting both work — the
// service layer sets Mode='standard' on create before validateExam runs, and
// the PATCH handler only overlays Mode when the request supplies a non-empty one.
var validModes = map[string]bool{
	"standard": true,
	"utbk":     true,
	"ielts":    true,
}

// validSectionTypes enumerates test.section_type values (FR-2). NULL is allowed
// (standard/utbk tests may be untyped); only a supplied non-null value is checked.
var validSectionTypes = map[string]bool{
	"listening": true,
	"reading":   true,
	"writing":   true,
}

// sectionedModes are the exam modes that run attached Tests as sequential sections
// (FR-19 publish-gate trigger). 'standard' is excluded.
func isSectionedMode(mode string) bool { return mode == "utbk" || mode == "ielts" }

// questionBodyAllowedTags is the tag allowlist shared by sanitizeQuestionBody
// and (via web/lib/question-html.ts, kept in lockstep by a cross-language
// test) the three frontend render/sanitize sites. Attribute lists are NOT
// shared — they deliberately differ per site (see exam.go's style subset vs.
// RichTextEditor's no-style-on-paste rule).
var questionBodyAllowedTags = []string{"b", "i", "u", "ul", "ol", "li", "sup", "sub", "img", "br", "p", "table", "thead", "tbody", "tr", "td", "th"}

// questionBodyPolicy is the single allowlist used by sanitizeQuestionBody. It is
// built once at package init and used on every write path. See sanitizeQuestionBody
// for the rationale behind each element/attribute.
var questionBodyPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(questionBodyAllowedTags...)
	p.AllowAttrs("src", "alt").OnElements("img")
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	// Restricted style: only a safe subset is allowed, and "position" is
	// explicitly rejected by handler (covers "position:fixed" and any
	// other value). url() in style is rejected wholesale — images carry
	// their URL via the src attribute, not in style.
	p.AllowStyles("color", "background-color", "text-align", "font-weight", "font-style", "text-decoration").
		MatchingHandler(func(s string) bool { return !strings.Contains(s, "url(") }).
		OnElements("img")
	return p
}()

// sanitizeQuestionBody strips disallowed HTML from a question body at the
// service trust boundary. The same call is invoked at every write path
// (CreateBankQuestion, SaveQuestion, CreateQuestionForTest,
// ProcessQuestionImportRows) so the persisted value is the sanitized one.
//
// Allowlist: b, i, u, ul, ol, li, sup, sub, br, p, img (with src/alt and a
// restricted style attribute). On* handlers, <script>, <iframe>, javascript:
// URLs, and any non-allowlisted tag are stripped. "position" in style is
// rejected via the regex below; url() in style is rejected via the policy's
// value handler.
func sanitizeQuestionBody(body string) string {
	if body == "" {
		return body
	}
	// Reject "position" declarations regardless of value (fixed/absolute/...).
	// bluemonday processes the value handler first; this regexp is a
	// belt-and-suspenders guard for any value that doesn't trigger a
	// direct string check (e.g. "POSITION:fixed" via casing).
	positionDecl := regexp.MustCompile(`(?i)position\s*:`)
	if positionDecl.MatchString(body) {
		body = positionDecl.ReplaceAllString(body, "")
	}
	cleaned := questionBodyPolicy.Sanitize(body)
	// Collapse runs of ";" left behind by stripping a declaration.
	cleaned = regexp.MustCompile(`;\s*;`).ReplaceAllString(cleaned, ";")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// isQuestionBodyEmpty reports whether a question body carries no real
// content: stripping every tag leaves nothing but whitespace, and there is
// no <img>. Used instead of a raw TrimSpace check because br/p are
// allowlisted tags with no text content of their own (FR5).
func isQuestionBodyEmpty(body string) bool {
	if strings.Contains(body, "<img") {
		return false
	}
	return strings.TrimSpace(htmlTagPattern.ReplaceAllString(body, "")) == ""
}

// sanitizeQuestionOptions sanitizes the Text field of each option using the same
// policy as sanitizeQuestionBody. Called at every write path (CreateBankQuestion,
// SaveQuestion, CreateQuestionForTest, ProcessQuestionImportRows) so the persisted
// value is the sanitized one.
func sanitizeQuestionOptions(options []model.QuestionOption) []model.QuestionOption {
	sanitized := make([]model.QuestionOption, len(options))
	for i, opt := range options {
		sanitized[i] = opt
		sanitized[i].Text = sanitizeQuestionBody(opt.Text)
	}
	return sanitized
}

// sanitizeQuestionStatements sanitizes the Body field of each true_false statement
// using the same policy as sanitizeQuestionBody. Called at every write path so the
// persisted value is the sanitized one, exactly as option text already is.
func sanitizeQuestionStatements(statements []model.QuestionStatement) []model.QuestionStatement {
	sanitized := make([]model.QuestionStatement, len(statements))
	for i, st := range statements {
		sanitized[i] = st
		sanitized[i].Body = sanitizeQuestionBody(st.Body)
	}
	return sanitized
}

// extractTokensFromStem finds all {{N}} tokens in the stem and returns their
// numeric indices in order. Returns the slice and error if any token is malformed.
func extractTokensFromStem(body string) ([]int, error) {
	tokenPattern := regexp.MustCompile(`\{\{(\d+)\}\}`)
	matches := tokenPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return []int{}, nil
	}

	tokens := make([]int, len(matches))
	for i, match := range matches {
		// match[1] is the captured group (the number inside {{N}})
		num := 0
		_, err := fmt.Sscanf(match[1], "%d", &num)
		if err != nil {
			return nil, fmt.Errorf("%w: malformed token number", ErrValidation)
		}
		tokens[i] = num
	}
	return tokens, nil
}

// validateTokenSequence checks that the given token indices form an exact 1..N
// contiguous set with no duplicates or gaps.
func validateTokenSequence(tokens []int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("%w: multi_blank requires at least one {{N}} token", ErrValidation)
	}

	seen := make(map[int]bool)
	for _, t := range tokens {
		if t < 1 {
			return fmt.Errorf("%w: token index must be >= 1", ErrValidation)
		}
		if seen[t] {
			return fmt.Errorf("%w: duplicate token index: %d", ErrValidation, t)
		}
		seen[t] = true
	}

	for i := 1; i <= len(tokens); i++ {
		if !seen[i] {
			return fmt.Errorf("%w: token sequence has gap; expected {{%d}}", ErrValidation, i)
		}
	}

	return nil
}

// validateQuestion enforces the format-validation matrix from spec.md §4.
// All error returns wrap ErrValidation with a sub-message so callers can
// use errors.Is(err, ErrValidation) AND err.Error() carries the WHY.
func validateQuestion(q model.Question, options []model.QuestionOption, blanks []model.QuestionBlank, statements []model.QuestionStatement) error {
	if !validQuestionFormats[q.Format] {
		return fmt.Errorf("%w: unknown question format: %s", ErrValidation, q.Format)
	}

	// br/p are allowlisted (FB-24) so a body of just breaks and empty
	// paragraphs no longer sanitizes to "" — strip all tags before judging
	// emptiness, but still accept an <img>-only body as content.
	if isQuestionBodyEmpty(q.Body) {
		return fmt.Errorf("%w: body cannot be empty", ErrValidation)
	}

	seenKeys := map[string]bool{}
	for _, o := range options {
		if o.Key == "" {
			return fmt.Errorf("%w: option key cannot be empty", ErrValidation)
		}
		if strings.TrimSpace(o.Text) == "" {
			return fmt.Errorf("%w: option text cannot be empty", ErrValidation)
		}
		if seenKeys[o.Key] {
			return fmt.Errorf("%w: duplicate option key: %s", ErrValidation, o.Key)
		}
		seenKeys[o.Key] = true
	}

	hasOptions := len(options) > 0
	hasCorrectAnswer := q.CorrectAnswer != nil && strings.TrimSpace(*q.CorrectAnswer) != ""
	correctCount := 0
	for _, o := range options {
		if o.IsCorrect {
			correctCount++
		}
	}

	switch q.Format {
	case "mcq":
		if len(options) < 2 {
			return fmt.Errorf("%w: mcq requires at least 2 options", ErrValidation)
		}
		if correctCount != 1 {
			return fmt.Errorf("%w: mcq requires exactly 1 correct option", ErrValidation)
		}
		if hasCorrectAnswer {
			return fmt.Errorf("%w: mcq cannot have correct_answer", ErrValidation)
		}
		if len(q.AcceptedAnswers) > 0 {
			return fmt.Errorf("%w: mcq cannot have accepted_answers", ErrValidation)
		}
	case "multi_answer":
		if len(options) < 2 {
			return fmt.Errorf("%w: multi_answer requires at least 2 options", ErrValidation)
		}
		if correctCount < 1 {
			return fmt.Errorf("%w: multi_answer requires at least 1 correct option", ErrValidation)
		}
		if hasCorrectAnswer {
			return fmt.Errorf("%w: multi_answer cannot have correct_answer", ErrValidation)
		}
		if len(q.AcceptedAnswers) > 0 {
			return fmt.Errorf("%w: multi_answer cannot have accepted_answers", ErrValidation)
		}
	case "short":
		if hasOptions {
			return fmt.Errorf("%w: short cannot have options", ErrValidation)
		}
		if err := validateAcceptedAnswerSet("short", effectiveAcceptedAnswers(q.AcceptedAnswers, deref(q.CorrectAnswer))); err != nil {
			return err
		}
	case "fill_blank":
		if hasOptions {
			return fmt.Errorf("%w: fill_blank cannot have options", ErrValidation)
		}
		if err := validateAcceptedAnswerSet("fill_blank", effectiveAcceptedAnswers(q.AcceptedAnswers, deref(q.CorrectAnswer))); err != nil {
			return err
		}
	case "multi_blank":
		// multi_blank: stem contains {{N}} tokens, no options, no scalar correct_answer,
		// blanks array has one entry per token with its own accepted-answer set (FR-24/26)
		if hasOptions {
			return fmt.Errorf("%w: multi_blank cannot have options", ErrValidation)
		}
		if hasCorrectAnswer {
			return fmt.Errorf("%w: multi_blank cannot have correct_answer", ErrValidation)
		}
		if len(q.AcceptedAnswers) > 0 {
			return fmt.Errorf("%w: multi_blank cannot have accepted_answers", ErrValidation)
		}

		tokens, err := extractTokensFromStem(q.Body)
		if err != nil {
			return err
		}
		if err := validateTokenSequence(tokens); err != nil {
			return err
		}

		if len(blanks) != len(tokens) {
			return fmt.Errorf("%w: blanks count (%d) must match token count (%d)", ErrValidation, len(blanks), len(tokens))
		}

		for i, blank := range blanks {
			label := fmt.Sprintf("blank at index %d", i)
			if err := validateAcceptedAnswerSet(label, effectiveAcceptedAnswers(blank.AcceptedAnswers, blank.CorrectAnswer)); err != nil {
				return err
			}
		}
	case "essay":
		if hasOptions {
			return fmt.Errorf("%w: essay cannot have options", ErrValidation)
		}
		if hasCorrectAnswer {
			return fmt.Errorf("%w: essay cannot have correct_answer", ErrValidation)
		}
		if len(q.AcceptedAnswers) > 0 {
			return fmt.Errorf("%w: essay cannot have accepted_answers", ErrValidation)
		}
	case "true_false":
		// true_false: statements are a child table, no options/blanks/correct_answer/
		// accepted_answers (FR-29); at least 2 statements with contiguous 1..N indices
		// and non-empty bodies (FR-30).
		if hasOptions {
			return fmt.Errorf("%w: true_false cannot have options", ErrValidation)
		}
		if len(blanks) > 0 {
			return fmt.Errorf("%w: true_false cannot have blanks", ErrValidation)
		}
		if hasCorrectAnswer {
			return fmt.Errorf("%w: true_false cannot have correct_answer", ErrValidation)
		}
		if len(q.AcceptedAnswers) > 0 {
			return fmt.Errorf("%w: true_false cannot have accepted_answers", ErrValidation)
		}
		if len(statements) < 2 {
			return fmt.Errorf("%w: true_false requires at least 2 statements", ErrValidation)
		}

		seenIdx := map[int]bool{}
		for _, st := range statements {
			if st.Index < 1 {
				return fmt.Errorf("%w: statement index must be >= 1", ErrValidation)
			}
			if seenIdx[st.Index] {
				return fmt.Errorf("%w: duplicate statement index: %d", ErrValidation, st.Index)
			}
			seenIdx[st.Index] = true
			if isQuestionBodyEmpty(st.Body) {
				return fmt.Errorf("%w: statement body cannot be empty", ErrValidation)
			}
		}
		for i := 1; i <= len(statements); i++ {
			if !seenIdx[i] {
				return fmt.Errorf("%w: statement indices must be contiguous 1..%d; missing %d", ErrValidation, len(statements), i)
			}
		}
	}

	// FR-S5-02: point_correct/point_wrong are unsigned magnitudes; the scoring
	// engine (not the author) applies the sign for wrong answers.
	if q.PointCorrect <= 0 {
		return fmt.Errorf("%w: point_correct must be > 0", ErrValidation)
	}
	// Per-item points (0050) share point_correct's bound: nil means "inherit",
	// but an explicit value must be a real worth. The DB CHECK enforces the
	// same, this just fails before the transaction with a readable message.
	for _, o := range options {
		if o.Points != nil && *o.Points <= 0 {
			return fmt.Errorf("%w: option points must be > 0", ErrValidation)
		}
	}
	for _, b := range blanks {
		if b.Points != nil && *b.Points <= 0 {
			return fmt.Errorf("%w: blank points must be > 0", ErrValidation)
		}
	}
	for _, st := range statements {
		if st.Points != nil && *st.Points <= 0 {
			return fmt.Errorf("%w: statement points must be > 0", ErrValidation)
		}
	}
	if q.PointWrong < 0 {
		return fmt.Errorf("%w: point_wrong must be >= 0", ErrValidation)
	}

	return nil
}

// effectiveAcceptedAnswers returns accepted when non-empty, else falls back to
// a single-element set built from the legacy scalar correct-answer column
// (e.g. the CSV importer, which never populates accepted_answers directly).
// Mirrors the repository's read-side fallback (FR-27) so validation and reads
// agree on what "no accepted answers" means.
func effectiveAcceptedAnswers(accepted []string, scalar string) []string {
	if len(accepted) > 0 {
		return accepted
	}
	if strings.TrimSpace(scalar) != "" {
		return []string{scalar}
	}
	return nil
}

// validateAcceptedAnswerSet enforces FR-26: at least one entry, no entry empty
// after trimming, no duplicates after normalizeAnswer (FR-23). label identifies
// the set in the error message (e.g. "short", "blank at index 1").
func validateAcceptedAnswerSet(label string, answers []string) error {
	if len(answers) == 0 {
		return fmt.Errorf("%w: %s requires at least one accepted answer", ErrValidation, label)
	}
	seen := map[string]bool{}
	for _, a := range answers {
		if strings.TrimSpace(a) == "" {
			return fmt.Errorf("%w: %s accepted answer cannot be empty", ErrValidation, label)
		}
		norm := normalizeAnswer(a)
		if seen[norm] {
			return fmt.Errorf("%w: %s has duplicate accepted answer after normalisation: %q", ErrValidation, label, a)
		}
		seen[norm] = true
	}
	return nil
}

// validateTest enforces metadata-only invariants for Test CRUD. Format-specific
// validation lives in validateQuestion; this is just title/subject/topic/duration/audio.
func validateTest(t model.Test) error {
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("%w: test title required", ErrValidation)
	}
	if strings.TrimSpace(t.Subject) == "" || strings.TrimSpace(t.Topic) == "" {
		return fmt.Errorf("%w: test subject/topic required", ErrValidation)
	}
	if t.DurationMinutes <= 0 {
		return fmt.Errorf("%w: duration_minutes must be positive", ErrValidation)
	}
	if t.AudioPlayLimit != nil && *t.AudioPlayLimit <= 0 {
		return fmt.Errorf("%w: audio_play_limit must be positive when set", ErrValidation)
	}
	// FR-18: section_type ∈ {listening,reading,writing} when supplied; a listening
	// section requires audio_url (FR-25 audio player). NULL section_type is allowed
	// (standard/utbk tests may be untyped).
	if t.SectionType != nil {
		if !validSectionTypes[*t.SectionType] {
			return fmt.Errorf("%w: section_type must be listening, reading, or writing", ErrValidation)
		}
		if *t.SectionType == "listening" {
			if t.AudioURL == nil || strings.TrimSpace(*t.AudioURL) == "" {
				return fmt.Errorf("%w: audio_url required when section_type=listening", ErrValidation)
			}
		}
	}
	return nil
}

func (s *Service) CreateTest(ctx context.Context, t model.Test) (model.Test, error) {
	if err := validateTest(t); err != nil {
		return model.Test{}, err
	}
	if err := s.storeRepo.CreateTest(ctx, &t); err != nil {
		return model.Test{}, err
	}
	return t, nil
}

func (s *Service) UpdateTest(ctx context.Context, id uuid.UUID, t model.Test) (model.Test, error) {
	if err := validateTest(t); err != nil {
		return model.Test{}, err
	}
	if _, err := s.storeRepo.GetTestByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Test{}, ErrTestNotFound
		}
		return model.Test{}, err
	}
	if err := s.storeRepo.UpdateTest(ctx, id, &t); err != nil {
		if errors.Is(err, repository.ErrSortOrderConflict) {
			return model.Test{}, fmt.Errorf("%w: sort order conflict", ErrValidation)
		}
		return model.Test{}, err
	}
	t.ID = id
	return t, nil
}

func (s *Service) DeleteTest(ctx context.Context, id uuid.UUID) error {
	if _, err := s.storeRepo.GetTestByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTestNotFound
		}
		return err
	}
	return s.storeRepo.DeleteTest(ctx, id)
}

func (s *Service) ListTests(ctx context.Context, filter repository.TestFilter) ([]model.Test, string, error) {
	return s.storeRepo.ListTests(ctx, filter)
}

func (s *Service) GetTestDetail(ctx context.Context, id uuid.UUID) (model.TestDetail, error) {
	d, err := s.storeRepo.GetTestDetail(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.TestDetail{}, ErrTestNotFound
		}
		return model.TestDetail{}, err
	}
	return *d, nil
}

// SaveQuestion routes create vs update by q.ID == uuid.Nil.
func (s *Service) SaveQuestion(ctx context.Context, q model.Question, options []model.QuestionOption, blanks []model.QuestionBlank, statements []model.QuestionStatement) (model.QuestionWithOptions, error) {
	q.Body = sanitizeQuestionBody(q.Body)
	options = sanitizeQuestionOptions(options)
	statements = sanitizeQuestionStatements(statements)

	// FR-14: a live-exam question's format is immutable. Checked before any
	// write, and before validateQuestion so a locked format never has to pass
	// the new format's validation rules just to be refused.
	if q.ID != uuid.Nil {
		stored, err := s.storeRepo.GetQuestionByID(ctx, q.ID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return model.QuestionWithOptions{}, ErrQuestionNotFound
			}
			return model.QuestionWithOptions{}, err
		}
		if stored.Format != q.Format {
			inLiveExam, err := s.storeRepo.IsQuestionInLiveExam(ctx, q.ID)
			if err != nil {
				return model.QuestionWithOptions{}, err
			}
			if inLiveExam {
				return model.QuestionWithOptions{}, ErrQuestionFormatLocked
			}
		}
	}

	if err := validateQuestion(q, options, blanks, statements); err != nil {
		return model.QuestionWithOptions{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.QuestionWithOptions{}, err
	}
	defer tx.Rollback(ctx)

	if q.ID == uuid.Nil {
		if err := s.storeRepo.CreateQuestionTx(ctx, tx, &q, options, blanks, statements); err != nil {
			return model.QuestionWithOptions{}, err
		}
	} else {
		if err := s.storeRepo.UpdateQuestionTx(ctx, tx, &q, options, blanks, statements); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return model.QuestionWithOptions{}, ErrQuestionNotFound
			}
			return model.QuestionWithOptions{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return model.QuestionWithOptions{}, err
	}

	q.Statements = statements
	return model.QuestionWithOptions{Question: q, Options: options, Blanks: blanks}, nil
}

// CreateQuestionForTest creates a bank question and atomically attaches it to the
// given test (FR-25). This preserves the existing POST /admin/tests/:id/questions
// behavior after migration 0025 moved attachment to test_question.
func (s *Service) CreateQuestionForTest(ctx context.Context, testID uuid.UUID, q model.Question, options []model.QuestionOption, blanks []model.QuestionBlank, statements []model.QuestionStatement) (model.QuestionWithOptions, error) {
	q.Body = sanitizeQuestionBody(q.Body)
	options = sanitizeQuestionOptions(options)
	statements = sanitizeQuestionStatements(statements)
	if err := validateQuestion(q, options, blanks, statements); err != nil {
		return model.QuestionWithOptions{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.QuestionWithOptions{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.CreateQuestionTx(ctx, tx, &q, options, blanks, statements); err != nil {
		return model.QuestionWithOptions{}, err
	}
	if err := s.storeRepo.AttachQuestionToTestTx(ctx, tx, testID, q.ID); err != nil {
		return model.QuestionWithOptions{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.QuestionWithOptions{}, err
	}

	q.Statements = statements
	return model.QuestionWithOptions{Question: q, Options: options, Blanks: blanks}, nil
}

// AttachQuestions appends bank questions to a test. Already-attached questions are
// idempotent (no duplicate, no error). Every supplied question must exist; the
// test must exist (FR-21).
func (s *Service) AttachQuestions(ctx context.Context, testID uuid.UUID, questionIDs []uuid.UUID) error {
	if len(questionIDs) == 0 {
		return fmt.Errorf("%w: question_ids cannot be empty", ErrValidation)
	}
	if _, err := s.storeRepo.GetTestByID(ctx, testID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTestNotFound
		}
		return err
	}

	exists, err := s.storeRepo.CountQuestionsByIDs(ctx, questionIDs)
	if err != nil {
		return err
	}
	if exists != len(questionIDs) {
		return ErrQuestionNotFound
	}

	colliding, err := s.storeRepo.FindQuestionsAttachedToSiblingTests(ctx, testID, questionIDs)
	if err != nil {
		return err
	}
	if len(colliding) > 0 {
		return fmt.Errorf("%w: question(s) already attached to another test in the same exam: %v", ErrValidation, colliding)
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.AttachQuestionsToTestTx(ctx, tx, testID, questionIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DetachQuestion removes only the test_question join row; the bank question and any
// other attachments survive (FR-22).
func (s *Service) DetachQuestion(ctx context.Context, testID, questionID uuid.UUID) error {
	if _, err := s.storeRepo.GetTestByID(ctx, testID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTestNotFound
		}
		return err
	}
	return s.storeRepo.DetachQuestionFromTest(ctx, testID, questionID)
}

// ReorderTestQuestions validates that the supplied id set equals the currently
// attached set, then atomically rewrites sort_order to match the list position
// (FR-23).
func (s *Service) ReorderTestQuestions(ctx context.Context, testID uuid.UUID, orderedQuestionIDs []uuid.UUID) error {
	if len(orderedQuestionIDs) == 0 {
		return fmt.Errorf("%w: question_ids cannot be empty", ErrValidation)
	}
	if _, err := s.storeRepo.GetTestByID(ctx, testID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTestNotFound
		}
		return err
	}

	attached, err := s.storeRepo.ListAttachedQuestionIDs(ctx, testID)
	if err != nil {
		return err
	}
	if !sameUUIDSet(attached, orderedQuestionIDs) {
		return fmt.Errorf("%w: question_ids must match the current attached set", ErrValidation)
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.ReorderTestQuestionsTx(ctx, tx, testID, orderedQuestionIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func sameUUIDSet(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[uuid.UUID]bool, len(a))
	for _, v := range a {
		m[v] = true
	}
	seen := make(map[uuid.UUID]bool, len(b))
	for _, v := range b {
		if !m[v] || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

// CreateBankQuestion creates a question in the bank with no test attachment (FR-9).
func (s *Service) CreateBankQuestion(ctx context.Context, q model.Question, options []model.QuestionOption, blanks []model.QuestionBlank, statements []model.QuestionStatement) (model.QuestionWithOptions, error) {
	q.Body = sanitizeQuestionBody(q.Body)
	options = sanitizeQuestionOptions(options)
	statements = sanitizeQuestionStatements(statements)
	if err := validateQuestion(q, options, blanks, statements); err != nil {
		return model.QuestionWithOptions{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.QuestionWithOptions{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.CreateQuestionTx(ctx, tx, &q, options, blanks, statements); err != nil {
		return model.QuestionWithOptions{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.QuestionWithOptions{}, err
	}

	q.Statements = statements
	return model.QuestionWithOptions{Question: q, Options: options, Blanks: blanks}, nil
}

func (s *Service) DeleteQuestion(ctx context.Context, id uuid.UUID) error {
	inLiveExam, err := s.storeRepo.IsQuestionInLiveExam(ctx, id)
	if err != nil {
		return err
	}
	if inLiveExam {
		return ErrQuestionInLiveExam
	}

	answered, err := s.storeRepo.CountAnswerReferences(ctx, id)
	if err != nil {
		return err
	}
	if answered > 0 {
		return fmt.Errorf("%w: question has been answered in %d session(s) and cannot be deleted", ErrValidation, answered)
	}

	if err := s.storeRepo.DeleteQuestion(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrQuestionNotFound
		}
		return err
	}
	return nil
}

// ListBankQuestions returns cursor-paginated bank questions with topic name and
// attached-test count (FR-14).
func (s *Service) ListBankQuestions(ctx context.Context, filter repository.QuestionFilter) ([]model.BankQuestionListItem, string, error) {
	return s.storeRepo.ListBankQuestions(ctx, filter)
}

func (s *Service) CountBankQuestions(ctx context.Context, filter repository.QuestionFilter) (int, error) {
	return s.storeRepo.CountBankQuestions(ctx, filter)
}

var validTimerModes = map[string]bool{
	"overall":  true,
	"per_test": true,
}

var validResultConfigs = map[string]bool{
	"hidden":           true,
	"score_only":       true,
	"score_pembahasan": true,
}

// validateExam enforces exam-level invariants: title required, timer_mode
// ∈ {overall, per_test} (empty allowed for legacy rows), duration
// required when timer_mode=overall, and result_config ∈ {hidden, score_only,
// score_pembahasan} when set (empty allowed here — CreateExam defaults it
// before this runs; the DB CHECK constraint added by migration 0015 rejects
// empty string, so this must never reach validateExam still empty on create).
func validateExam(e model.Exam) error {
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("%w: exam title required", ErrValidation)
	}
	// FR-18: mode ∈ {standard,utbk,ielts} when supplied. Empty is accepted here
	// because CreateExam defaults it to 'standard' before validateExam runs and
	// the PATCH handler only overlays Mode when the request supplies a non-empty
	// value (absent preserves the stored value).
	if e.Mode != "" && !validModes[e.Mode] {
		return fmt.Errorf("%w: mode must be standard, utbk, or ielts", ErrValidation)
	}
	if e.TimerMode != "" && !validTimerModes[e.TimerMode] {
		return fmt.Errorf("%w: timer_mode must be overall or per_test", ErrValidation)
	}
	if e.TimerMode == "overall" {
		if e.DurationMinutes == nil || *e.DurationMinutes <= 0 {
			return fmt.Errorf("%w: duration_minutes required and positive when timer_mode=overall", ErrValidation)
		}
	}
	if e.ResultConfig != "" && !validResultConfigs[e.ResultConfig] {
		return fmt.Errorf("%w: result_config must be hidden, score_only, or score_pembahasan", ErrValidation)
	}
	if err := validateCertificateDesign(e.CertificateDesign); err != nil {
		return err
	}
	if e.CheckInWindowMinutes != nil && *e.CheckInWindowMinutes < 0 {
		return fmt.Errorf("%w: check_in_window_minutes cannot be negative", ErrValidation)
	}
	if e.GraceWindowMinutes != nil && *e.GraceWindowMinutes < 0 {
		return fmt.Errorf("%w: grace_window_minutes cannot be negative", ErrValidation)
	}
	if e.MaxAttempts != nil && *e.MaxAttempts < 0 {
		return fmt.Errorf("%w: max_attempts cannot be negative", ErrValidation)
	}
	if e.ScheduledEndAt != nil {
		if e.ScheduledAt == nil {
			return fmt.Errorf("%w: scheduled_end_at requires scheduled_at", ErrValidation)
		}
		if !e.ScheduledEndAt.After(*e.ScheduledAt) {
			return fmt.Errorf("%w: scheduled_end_at must be after scheduled_at", ErrValidation)
		}
	}
	return nil
}

func validateCertificateDesign(raw *json.RawMessage) error {
	if raw == nil {
		return nil
	}
	design, err := parseCertificateDesign(raw)
	if err != nil {
		return fmt.Errorf("%w: invalid certificate design json", ErrValidation)
	}
	if design.Template != "" {
		if err := validateCertificateTemplate(design.Template); err != nil {
			return err
		}
	}
	if len(design.Fields) > 0 {
		return ValidateLayout(design.Layout)
	}
	return nil
}

// formatExamNumber renders an exam's human-friendly serial zero-padded to a
// minimum of 4 digits (FR-23). "%04d" pads but never truncates, so numbers
// past 9999 simply grow wider instead of being capped.
func formatExamNumber(n int) string {
	return fmt.Sprintf("%04d", n)
}

// formatParticipantNo composes the FR-24 display string
// "YYMMDD-<exam_number(pad4)>-<participant_number(pad6)>" from a date prefix
// (already converted to the desired timezone by the caller), exam number, and
// participant number. Shared by GetExamRegistration and AdminGetExamRoster —
// keep byte-identical to preserve existing display output.
func formatParticipantNo(prefix time.Time, examNumber, participantNumber int) string {
	return fmt.Sprintf("%s-%s-%06d", prefix.Format("060102"), formatExamNumber(examNumber), participantNumber)
}

// CreateExam creates a standalone exam — no product is created here. Selling the
// exam is a separate step: attach it to a Product via the generic Product flow
// (POST/PATCH /admin/products with exam_ids), mirroring how course-type products
// attach existing Courses.
func (s *Service) CreateExam(ctx context.Context, m model.Exam) (model.Exam, error) {
	if m.ResultConfig == "" {
		m.ResultConfig = "hidden"
	}
	if m.Status == "" {
		m.Status = "draft"
	}
	if m.Mode == "" {
		m.Mode = "standard"
	}
	if err := validateExam(m); err != nil {
		return model.Exam{}, err
	}
	if err := s.storeRepo.CreateExam(ctx, &m); err != nil {
		return model.Exam{}, err
	}
	return m, nil
}

func (s *Service) UpdateExam(ctx context.Context, id uuid.UUID, m model.Exam) (model.Exam, error) {
	commonFields := m
	commonFields.CertificateDesign = nil
	if err := validateExam(commonFields); err != nil {
		return model.Exam{}, err
	}
	existing, err := s.storeRepo.GetExamByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Exam{}, ErrExamNotFound
		}
		return model.Exam{}, err
	}
	if !rawMessagePtrEqual(existing.CertificateDesign, m.CertificateDesign) {
		if err := validateCertificateDesign(m.CertificateDesign); err != nil {
			return model.Exam{}, err
		}
	}
	m.ID = id
	m.CreatedAt = existing.CreatedAt
	// C3/FR-14: template, background key, and layout now share one JSON blob
	// (Task 8/FR-26), so a single raw-bytes compare on that blob is the whole
	// staleness check — bump only when it actually changed, so an unrelated
	// field edit doesn't falsely mark the design stale.
	if !rawMessagePtrEqual(existing.CertificateDesign, m.CertificateDesign) {
		now := time.Now()
		m.CertificateDesignUpdatedAt = &now
	} else {
		m.CertificateDesignUpdatedAt = existing.CertificateDesignUpdatedAt
	}
	if err := s.storeRepo.UpdateExam(ctx, id, &m); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Exam{}, ErrExamNotFound
		}
		return model.Exam{}, err
	}
	return m, nil
}

// SetExamCertificateEnabled flips an exam's certificate_enabled flag through
// its own dedicated action (FR-11/FR-12) — never through UpdateExam's general
// PATCH — mirroring ChangeSchoolStatus. The underlying UPDATE touches only the
// certificate_enabled column, so certificate_design and
// certificate_design_updated_at are byte-for-byte unchanged across a
// disable/re-enable round trip.
func (s *Service) SetExamCertificateEnabled(ctx context.Context, id uuid.UUID, enabled bool) (model.Exam, error) {
	exam, err := s.storeRepo.GetExamByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Exam{}, ErrExamNotFound
		}
		return model.Exam{}, err
	}
	if err := s.storeRepo.SetExamCertificateEnabled(ctx, id, enabled); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Exam{}, ErrExamNotFound
		}
		return model.Exam{}, err
	}
	exam.CertificateEnabled = enabled
	return *exam, nil
}

func rawMessagePtrEqual(a, b *json.RawMessage) bool {
	if a == nil || b == nil {
		return a == b
	}
	return string(*a) == string(*b)
}

// CertificateDesignResponse is the admin editor's read model for GET
// certificate-design: the current template, a freshly presigned background URL
// for display (FR-18 — the URL is signed per read, never persisted), and the
// resolved layout — the built-in default when the admin has not saved one yet
// (FR-29). BackgroundKey is also returned because the PUT replaces the design
// wholesale: without it an editor that never touches the background has nothing
// to send back and would erase the persisted upload.
type CertificateDesignResponse struct {
	Template      string                      `json:"template"`
	BackgroundKey *string                     `json:"background_key"`
	BackgroundURL *string                     `json:"background_url"`
	SignatureURL  *string                     `json:"signature_url"`
	Layout        Layout                      `json:"layout"`
	Presets       []CertificatePresetResponse `json:"presets"`
	AssetURLs     map[string]string           `json:"asset_urls"`
}

type CertificatePresetResponse struct {
	Template      string `json:"template"`
	BackgroundURL string `json:"background_url"`
	Layout        Layout `json:"layout"`
}

// GetCertificateDesign returns the certificate design the admin editor renders:
// template, a presigned URL for any custom background, and the resolved layout.
func (s *Service) GetCertificateDesign(ctx context.Context, examID uuid.UUID) (*CertificateDesignResponse, error) {
	exam, err := s.storeRepo.GetExamByID(ctx, examID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}
	if !exam.CertificateEnabled {
		return nil, ErrCertificateDisabled
	}

	layout, err := resolveCertificateLayout(exam)
	if err != nil {
		return nil, err
	}
	normalizedLayout := normalizeCertificateLayout(layout)

	bgKey := certificateBackgroundKey(exam)
	var bgURL *string
	if bgKey != nil {
		signed, err := s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *bgKey, presignedDocumentURLTTL)
		if err != nil {
			return nil, fmt.Errorf("presign certificate background: %w", err)
		}
		bgURL = &signed
	}

	assetURLs, err := s.certificateImageAssetURLs(ctx, normalizedLayout, s.presignReadURL)
	if err != nil {
		return nil, err
	}
	var sigURL *string
	if signed, ok := assetURLs["signature"]; ok {
		sigURL = &signed
	}

	presets := make([]CertificatePresetResponse, 0, 3)
	for _, template := range []string{"classic", "modern", "elegant"} {
		presets = append(presets, CertificatePresetResponse{
			Template:      template,
			BackgroundURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(builtinCertificateBackground(template)),
			Layout:        normalizeCertificateLayout(defaultLayout(template)),
		})
	}

	return &CertificateDesignResponse{
		Template:      certificateTemplate(exam),
		BackgroundKey: bgKey,
		BackgroundURL: bgURL,
		SignatureURL:  sigURL,
		Layout:        normalizedLayout,
		Presets:       presets,
		AssetURLs:     assetURLs,
	}, nil
}

func (s *Service) ReplaceExamTests(ctx context.Context, examID uuid.UUID, testIDs []uuid.UUID) error {
	if _, err := s.storeRepo.GetExamByID(ctx, examID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrExamNotFound
		}
		return err
	}

	for _, testID := range testIDs {
		if _, err := s.storeRepo.GetTestByID(ctx, testID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrTestNotFound
			}
			return err
		}
	}

	tests := make([]model.ExamTest, len(testIDs))
	for i, testID := range testIDs {
		tests[i] = model.ExamTest{ExamID: examID, TestID: testID, SortOrder: i}
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.ReplaceExamTestsTx(ctx, tx, examID, tests); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// validatePublishSections enforces FR-19: a sectioned exam (mode ∈ {utbk,ielts})
// must have at least one attached Test, and an ielts exam must have a non-null
// section_type on every attached Test (the offending section is named in the
// error message). Standard/empty modes skip the gate entirely. The function is
// pure over (exam, tests) so it can be unit-tested without a DB; PublishProduct
// runs this for every exam attached to an exam-type product before publishing.
func validatePublishSections(exam model.Exam, tests []model.ExamTestEntry) error {
	if !isSectionedMode(exam.Mode) {
		return nil
	}
	if len(tests) == 0 {
		return fmt.Errorf("%w: sectioned exam (mode=%s) requires at least one test", ErrValidation, exam.Mode)
	}
	if exam.Mode == "ielts" {
		for _, te := range tests {
			if te.Test.SectionType == nil {
				title := te.Test.Title
				if title == "" {
					title = te.TestID.String()
				}
				return fmt.Errorf("%w: ielts exam has an untyped section (%q); section_type required", ErrValidation, title)
			}
		}
	}
	return nil
}

func (s *Service) GetExam(ctx context.Context, id uuid.UUID) (model.ExamDetail, error) {
	d, err := s.storeRepo.GetExamDetail(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ExamDetail{}, ErrExamNotFound
		}
		return model.ExamDetail{}, err
	}
	return *d, nil
}

func (s *Service) ListExams(ctx context.Context, filter repository.ExamFilter) ([]model.ExamListItem, string, error) {
	return s.storeRepo.ListExams(ctx, filter)
}

func (s *Service) GetExamRegistrations(ctx context.Context, studentID string) ([]model.RegistrationListItem, error) {
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	return s.storeRepo.GetExamRegistrationsByStudent(ctx, sid)
}

func (s *Service) GetExamRegistration(ctx context.Context, regID, studentID string) (*model.RegistrationDetail, error) {
	rid, err := uuid.Parse(regID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid registration id", ErrValidation)
	}
	sid, err := uuid.Parse(studentID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid student id", ErrValidation)
	}
	detail, err := s.storeRepo.GetExamRegistrationByID(ctx, rid, sid)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrRegistrationNotFound
	}
	if err == nil && detail != nil {
		if detail.ParticipantNumber != nil {
			// Prefix by the exam's scheduled date (WIB), falling back to the
			// registration date when the exam is not yet scheduled.
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
			detail.ParticipantNo = formatParticipantNo(prefix, examNo, *detail.ParticipantNumber)
		}
		// Platform/Ruang is a single system-config value (one platform for all exams).
		detail.Platform = examPlatformDefault
		if cfg, cErr := s.GetSystemConfig(ctx); cErr == nil {
			if v := cfg["exam_platform"]; v != "" {
				detail.Platform = v
			}
		}
	}
	return detail, err
}

// examPlatformDefault is used when the exam_platform system-config key is unset.
const examPlatformDefault = "exam.abakacademy.id"

// AdminGetExamRoster returns the read-only admin participant roster for an
// exam (FR-32): every registration joined with student name/username, with
// each row's FR-24 display participant number composed here (nil-safe — rows
// without a stored participant_number keep ParticipantNo empty rather than
// crashing or showing a bogus number). schoolFilter, when non-nil, restricts
// the roster to that school's students (admin_school tenant isolation).
func (s *Service) AdminGetExamRoster(ctx context.Context, examID uuid.UUID, schoolFilter *string) ([]model.ExamRosterEntry, error) {
	rows, err := s.storeRepo.GetExamRoster(ctx, examID, schoolFilter)
	if err != nil {
		return nil, err
	}
	wib, _ := time.LoadLocation("Asia/Jakarta")
	for i := range rows {
		if rows[i].ParticipantNumber == nil {
			continue
		}
		prefix := rows[i].RegisteredAt
		if rows[i].ExamScheduledAt != nil {
			prefix = *rows[i].ExamScheduledAt
		}
		if wib != nil {
			prefix = prefix.In(wib)
		}
		examNo := 0
		if rows[i].ExamNumber != nil {
			examNo = *rows[i].ExamNumber
		}
		rows[i].ParticipantNo = formatParticipantNo(prefix, examNo, *rows[i].ParticipantNumber)
	}
	return rows, nil
}

// GetExamCard returns a freshly presigned URL for the exam card PDF, generating
// it once via Gotenberg through the print route and caching the object key
// thereafter (FR-28, FR-30): mint a card print token → RenderURL(card print
// route) → object-store put → card_key persisted → fresh presigned GET. A
// registration with a CardKey already set is presigned straight away, so a
// repeated download never re-renders — and the API is never the data-transfer
// path for the PDF bytes themselves.
func (s *Service) GetExamCard(ctx context.Context, regID, studentID string) (string, string, error) {
	detail, err := s.GetExamRegistration(ctx, regID, studentID)
	if err != nil {
		return "", "", err
	}
	filename := "kartu-peserta-" + detail.Token + ".pdf"

	if detail.CardKey != nil && *detail.CardKey != "" {
		signed, err := s.presignCardURL(ctx, *detail.CardKey, filename)
		if err != nil {
			return "", "", err
		}
		return signed, filename, nil
	}

	regUUID, err := uuid.Parse(regID)
	if err != nil {
		return "", "", fmt.Errorf("%w: invalid registration id", ErrValidation)
	}

	data, err := s.GetCardPrintData(ctx, regID)
	if err != nil {
		return "", "", fmt.Errorf("resolve card print data: %w", err)
	}
	pdf, err := generateExamCardPDF(ctx, s.renderer, data)
	if err != nil {
		return "", "", fmt.Errorf("generate card pdf: %w", err)
	}
	key, err := s.uploadCardPDF(ctx, regUUID, pdf)
	if err != nil {
		return "", "", fmt.Errorf("upload card pdf: %w", err)
	}
	if err := s.storeRepo.UpdateRegistrationCard(ctx, regUUID, key); err != nil {
		return "", "", fmt.Errorf("persist card key: %w", err)
	}

	signed, err := s.presignCardURL(ctx, key, filename)
	if err != nil {
		return "", "", err
	}
	return signed, filename, nil
}

// CardPrintData is the full set of server-authored values the exam card
// print route displays (FR-20): the participant number, student name,
// school, exam title/schedule, and check-in code, resolved from the server's
// own records — never from anything the caller supplies beyond the
// registration id bound to a redeemed print token. TenantName/TenantLogoURL/
// PhotoURL/FooterNote restore the parity buildCardHTML (card_html.go) used
// to carry before Task 12 switched GetExamCard to this print route (Task 25).
type CardPrintData struct {
	ParticipantNo string `json:"participant_no"`
	StudentName   string `json:"student_name"`
	School        string `json:"school"`
	ExamTitle     string `json:"exam_title"`
	ExamSchedule  string `json:"exam_schedule"`
	CheckInCode   string `json:"check_in_code"`
	TenantName    string `json:"tenant_name"`
	TenantLogoURL string `json:"tenant_logo_url,omitempty"`
	PhotoURL      string `json:"photo_url,omitempty"`
	FooterNote    string `json:"footer_note"`
}

// cardScheduleText preserves the pre-existing schedule formatting: Asia/Jakarta,
// "02 Jan 2006 15:04 WIB" (FR-23).
func cardScheduleText(reg *model.RegistrationDetail) string {
	if reg.Exam.ScheduledAt == nil {
		return "-"
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	return reg.Exam.ScheduledAt.In(loc).Format("02 Jan 2006 15:04 WIB")
}

// cardFooterNote preserves the pre-existing check-in vs free-access copy,
// keyed on reg.Exam.RequiresCheckin / CheckInWindowMinutes.
func cardFooterNote(reg *model.RegistrationDetail) string {
	if reg.Exam.RequiresCheckin {
		if reg.Exam.CheckInWindowMinutes != nil {
			return fmt.Sprintf("Harap check-in dalam waktu %d menit sebelum ujian.", *reg.Exam.CheckInWindowMinutes)
		}
		return "Harap check-in sebelum ujian dimulai."
	}
	return "Akses bebas pada waktu yang ditentukan."
}

// GetCardPrintData resolves every server-authored value the exam card
// displays, keyed by regID alone — GetExamCard (JWT-authenticated) calls
// this directly to build the card's substitution values; it is no longer
// reached through a print token (async redesign 2026-08-02 removed the
// print-data HTTP surface entirely).
func (s *Service) GetCardPrintData(ctx context.Context, regID string) (*CardPrintData, error) {
	rid, err := uuid.Parse(regID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid registration id", ErrValidation)
	}
	detail, err := s.storeRepo.GetRegistrationForPrint(ctx, rid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRegistrationNotFound
		}
		return nil, err
	}

	studentName, school, photoURL := "", "", ""
	if user, mErr := s.Me(ctx, detail.StudentID.String()); mErr == nil && user != nil {
		studentName = user.Name
		if user.SchoolID != nil && *user.SchoolID != "" {
			if sch, sErr := s.storeRepo.GetSchoolByID(ctx, *user.SchoolID); sErr == nil && sch != nil {
				school = sch.Name
			}
		}
		if school == "" && user.UnlistedSchoolName != nil {
			school = *user.UnlistedSchoolName
		}
		if user.PhotoURL != nil {
			photoURL = s.presignCardAvatarURL(ctx, *user.PhotoURL)
		}
	}

	tenantName, tenantLogoURL := "", ""
	if cfg, cErr := s.GetSystemConfig(ctx); cErr == nil {
		tenantName = cfg["app_name"]
		tenantLogoURL = s.resolveCardLogoURL(ctx, cfg["app_logo_url"])
	}

	participantNo := ""
	if detail.ParticipantNumber != nil {
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
		participantNo = formatParticipantNo(prefix, examNo, *detail.ParticipantNumber)
	}

	return &CardPrintData{
		ParticipantNo: participantNo,
		StudentName:   studentName,
		School:        school,
		ExamTitle:     detail.Exam.Title,
		ExamSchedule:  cardScheduleText(detail),
		CheckInCode:   detail.Token,
		TenantName:    tenantName,
		TenantLogoURL: tenantLogoURL,
		PhotoURL:      photoURL,
		FooterNote:    cardFooterNote(detail),
	}, nil
}

// presignCardAvatarURL signs a URL for a student's stored avatar for the card
// print route, reusing avatarKeyFromStored's trust rule (see
// loadCardAvatarImage): only a key under our own bucket is ever signed,
// never an outbound fetch keyed off a stored proxy URL naming some other
// host. It's signed against the internal storage endpoint, not the
// browser-facing one (see presignInternalReadURL) — the only consumer is
// Gotenberg's headless Chromium fetching this print route from inside the
// docker network. Any failure — no storage key, presign error — collapses
// to "" so a missing/foreign photo just omits the image (mirrors
// loadCardAvatarImage's FR-21 best-effort behavior) instead of failing the
// print route.
func (s *Service) presignCardAvatarURL(ctx context.Context, stored string) string {
	key := avatarKeyFromStored(stored)
	if key == "" {
		return ""
	}
	signed, err := s.presignInternalReadURL(ctx, s.cfg.ObjectStorageBucketName, key, presignedDocumentURLTTL)
	if err != nil {
		return ""
	}
	return signed
}

// resolveCardLogoURL returns a URL for the configured tenant logo, loadable
// by Gotenberg's headless Chromium (see presignCardAvatarURL): a presigned
// GET against the internal storage endpoint when app_logo_url names one of
// our own avatar keys, or the configured value unchanged when it is an
// ordinary external https:// URL — the System Config contract (see
// card_logo.go) — which has no object key to presign.
func (s *Service) resolveCardLogoURL(ctx context.Context, stored string) string {
	if stored == "" {
		return ""
	}
	if key := avatarKeyFromStored(stored); key != "" {
		signed, err := s.presignInternalReadURL(ctx, s.cfg.ObjectStorageBucketName, key, presignedDocumentURLTTL)
		if err != nil {
			return ""
		}
		return signed
	}
	return stored
}

// presignedDocumentURLTTL bounds every presigned certificate/card/asset URL.
// Once an admin hides results, this TTL is the entire residual exposure
// window during which an already-signed URL keeps working — the gate at #55
// only stops new signs, not URLs already handed to a browser.
const presignedDocumentURLTTL = 15 * time.Minute

// presignCardURL signs a time-limited GET for a stored card PDF. The bucket is
// private, so every read signs afresh rather than persisting a URL. The
// response-content-disposition parameter makes the object download under the
// card's own filename even though the client is fetching an opaque object key.
func (s *Service) presignCardURL(ctx context.Context, key, filename string) (string, error) {
	if s.storage == nil {
		return "", ErrStorageNotConfigured
	}
	params := url.Values{}
	params.Set("response-content-disposition", `attachment; filename="`+filename+`"`)
	u, err := s.presignStorage().PresignedGetObject(ctx, s.cfg.ObjectStorageBucketName, key, presignedDocumentURLTTL, params)
	if err != nil {
		return "", fmt.Errorf("presign card url: %w", err)
	}
	return u.String(), nil
}

// uploadCardPDF uploads the rendered card PDF at cards/<regID>.pdf and returns
// its object key. The bucket is private (mirrors uploadCertificatePDF).
func (s *Service) uploadCardPDF(ctx context.Context, regID uuid.UUID, pdf []byte) (string, error) {
	if s.storage == nil {
		return "", ErrStorageNotConfigured
	}
	bucket := s.cfg.ObjectStorageBucketName
	key := fmt.Sprintf("cards/%s.pdf", regID.String())
	if _, err := s.storage.PutObject(ctx, bucket, key, bytes.NewReader(pdf), int64(len(pdf)), minio.PutObjectOptions{
		ContentType: "application/pdf",
	}); err != nil {
		return "", err
	}
	return key, nil
}

// avatarKeyFromStored extracts the object key from a stored avatar reference —
// either an "<api-base>/files/<key>" proxy URL or a bare key — and returns ""
// for anything that isn't an avatars/ object. Only the key is used; the host
// in a stored URL is ignored, so a student-supplied photo_url cannot cause an
// outbound (SSRF) request during card generation.
func avatarKeyFromStored(stored string) string {
	if stored == "" {
		return ""
	}
	key := stored
	if i := strings.Index(stored, "/files/"); i >= 0 {
		key = stored[i+len("/files/"):]
	}
	key = strings.TrimPrefix(key, "/")
	if !strings.HasPrefix(key, "avatars/") || strings.Contains(key, "..") {
		return ""
	}
	return key
}

// loadCardAvatarImage best-effort reads an avatar image from object storage by
// key. Any failure returns nil rather than an error — a missing/unreadable
// asset must never fail card generation (FR-21).
func (s *Service) loadCardAvatarImage(ctx context.Context, stored string) []byte {
	key := avatarKeyFromStored(stored)
	if key == "" {
		return nil
	}
	obj, _, err := s.OpenAvatar(ctx, key)
	if err != nil {
		return nil
	}
	defer obj.Close()
	data, err := io.ReadAll(io.LimitReader(obj, 5<<20))
	if err != nil {
		return nil
	}
	return data
}
