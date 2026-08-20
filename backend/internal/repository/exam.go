package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"akademi-bimbel/internal/model"
)

// ErrSortOrderConflict — uq_question_order SQLSTATE 23505 — surfaced for service-layer mapping.
var ErrSortOrderConflict = errors.New("sort order conflict")

// ErrInvalidCursor — malformed pagination cursor — surfaced for service-layer mapping to 4xx.
var ErrInvalidCursor = errors.New("invalid pagination cursor")

// ErrNoAttemptsLeft — CreateExamSessionTx's atomic guard matched no row (ceiling
// exhausted, or an in_progress session already exists). Surfaced for service-layer
// mapping: resume the live session when one exists, otherwise ErrAlreadyAttempted.
var ErrNoAttemptsLeft = errors.New("no attempts left")

// ErrNoActiveSection — AdvanceSessionSectionTx's atomic status='active' guard matched no
// row (wrong test_id, or the section is already submitted / still pending). Surfaced for
// service-layer mapping: idempotent-200 when already submitted, ErrSectionNotActive when
// pending (Task 3 owns that decision).
var ErrNoActiveSection = errors.New("no active section matched the guard")

func scanTest(row interface{ Scan(dest ...any) error }, t *model.Test) error {
	var audioURL *string
	var audioPlayLimit *int
	err := row.Scan(
		&t.ID, &t.Title, &t.Subject, &t.Topic, &t.DurationMinutes,
		&audioURL, &audioPlayLimit, &t.SectionType, &t.CreatedAt,
	)
	if err != nil {
		return err
	}
	if audioURL != nil {
		t.AudioURL = audioURL
	}
	if audioPlayLimit != nil {
		t.AudioPlayLimit = audioPlayLimit
	}
	return nil
}

// scanTestWithCount is used by ListTests where the SELECT also LEFT JOINs a
// grouped question count; keeps GetByID/CreateTest untouched.
func scanTestWithCount(row interface{ Scan(dest ...any) error }, t *model.Test) error {
	var audioURL *string
	var audioPlayLimit *int
	err := row.Scan(
		&t.ID, &t.Title, &t.Subject, &t.Topic, &t.DurationMinutes,
		&audioURL, &audioPlayLimit, &t.SectionType, &t.QuestionCount, &t.CreatedAt,
	)
	if err != nil {
		return err
	}
	if audioURL != nil {
		t.AudioURL = audioURL
	}
	if audioPlayLimit != nil {
		t.AudioPlayLimit = audioPlayLimit
	}
	return nil
}

func scanQuestion(row interface{ Scan(dest ...any) error }, q *model.Question) error {
	var correctAnswer, explanation, difficulty, imageURL *string
	err := row.Scan(
		&q.ID, &q.QuestionNumber, &q.Format, &q.Body,
		&correctAnswer, &explanation, &difficulty, &imageURL,
		&q.TopicID, &q.PointCorrect, &q.PointWrong,
	)
	if err != nil {
		return err
	}
	if correctAnswer != nil {
		q.CorrectAnswer = correctAnswer
	}
	if explanation != nil {
		q.Explanation = explanation
	}
	if difficulty != nil {
		q.Difficulty = difficulty
	}
	if imageURL != nil {
		q.ImageURL = imageURL
	}
	return nil
}

func scanQuestionOption(row interface{ Scan(dest ...any) error }, o *model.QuestionOption) error {
	var imageURL *string
	var isCorrect bool
	err := row.Scan(
		&o.QuestionID, &o.Key, &o.Text, &imageURL, &isCorrect, &o.SortOrder, &o.Points,
	)
	if err != nil {
		return err
	}
	if imageURL != nil {
		o.ImageURL = imageURL
	}
	o.IsCorrect = isCorrect
	return nil
}

// TestFilter mirrors ProductFilter shape.
type TestFilter struct {
	Subject string
	Topic   string
	Q       string
	Cursor  string
	Limit   int
}

// QuestionFilter is the filter for the bank question list endpoint (FR-14).
type QuestionFilter struct {
	Format  string
	TopicID string
	Search  string
	Cursor  string
	Offset  int
	Limit   int
}

// bankQuestionFilterSQL renders the shared WHERE tail for the bank list and its
// count so the two cannot drift. Cursor/Offset are pagination, not filtering,
// and are deliberately excluded here.
func bankQuestionFilterSQL(filter QuestionFilter, argIdx int) (string, []interface{}, int) {
	query := ""
	args := []interface{}{}
	if filter.Format != "" {
		query += fmt.Sprintf(` AND q.format = $%d`, argIdx)
		args = append(args, filter.Format)
		argIdx++
	}
	if filter.TopicID != "" {
		query += fmt.Sprintf(` AND q.topic_id = $%d::uuid`, argIdx)
		args = append(args, filter.TopicID)
		argIdx++
	}
	if filter.Search != "" {
		query += fmt.Sprintf(` AND (LOWER(q.body) LIKE LOWER($%d) OR q.id::text LIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}
	return query, args, argIdx
}

// CountBankQuestions returns the total row count for the same filters the bank
// list applies, so the UI can render numbered pages.
func (r *Repository) CountBankQuestions(ctx context.Context, filter QuestionFilter) (int, error) {
	where, args, _ := bankQuestionFilterSQL(filter, 1)
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM question q WHERE 1=1`+where, args...).Scan(&total)
	return total, err
}

func (r *Repository) CreateTest(ctx context.Context, t *model.Test) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes, audio_url, audio_play_limit, section_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		t.Title, t.Subject, t.Topic, t.DurationMinutes, t.AudioURL, t.AudioPlayLimit, t.SectionType,
	).Scan(&t.ID, &t.CreatedAt)
	return err
}

func (r *Repository) GetTestByID(ctx context.Context, id uuid.UUID) (*model.Test, error) {
	out := &model.Test{}
	err := scanTest(r.pool.QueryRow(ctx,
		`SELECT id, title, subject, topic, duration_minutes, audio_url, audio_play_limit, section_type, created_at
		FROM test
		WHERE id = $1`,
		id,
	), out)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return out, nil
}

func (r *Repository) GetTestDetail(ctx context.Context, id uuid.UUID) (*model.TestDetail, error) {
	test, err := r.GetTestByID(ctx, id)
	if err != nil {
		return nil, err
	}

	questions, err := r.ListQuestions(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.TestDetail{
		Test:      *test,
		Questions: questions,
	}, nil
}

func (r *Repository) ListTests(ctx context.Context, filter TestFilter) ([]model.Test, string, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	// LEFT JOIN with a grouped count keeps tests without questions counted as 0.
	// Count through test_question since post-0025 attachment lives on the join.
	query := `SELECT t.id, t.title, t.subject, t.topic, t.duration_minutes,
		t.audio_url, t.audio_play_limit, t.section_type, COALESCE(q.cnt, 0), t.created_at
	FROM test t
	LEFT JOIN (
		SELECT test_id, COUNT(*) AS cnt FROM test_question GROUP BY test_id
	) q ON q.test_id = t.id
	WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Subject != "" {
		query += fmt.Sprintf(` AND t.subject = $%d`, argIdx)
		args = append(args, filter.Subject)
		argIdx++
	}
	if filter.Topic != "" {
		query += fmt.Sprintf(` AND t.topic = $%d`, argIdx)
		args = append(args, filter.Topic)
		argIdx++
	}
	if filter.Q != "" {
		query += fmt.Sprintf(` AND t.title ILIKE '%%' || $%d || '%%'`, argIdx)
		args = append(args, filter.Q)
		argIdx++
	}
	if filter.Cursor != "" {
		query += fmt.Sprintf(` AND t.id > $%d`, argIdx)
		args = append(args, filter.Cursor)
		argIdx++
	}

	query += ` ORDER BY t.id LIMIT $` + fmt.Sprintf("%d", argIdx)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	tests := []model.Test{}
	for rows.Next() {
		t := model.Test{}
		if err := scanTestWithCount(rows, &t); err != nil {
			return nil, "", err
		}
		tests = append(tests, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(tests) > filter.Limit {
		nextCursor = tests[filter.Limit].ID.String()
		tests = tests[:filter.Limit]
	}

	return tests, nextCursor, nil
}

func (r *Repository) UpdateTest(ctx context.Context, id uuid.UUID, t *model.Test) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE test
		SET title = $1, subject = $2, topic = $3, duration_minutes = $4, audio_url = $5, audio_play_limit = $6, section_type = $7
		WHERE id = $8`,
		t.Title, t.Subject, t.Topic, t.DurationMinutes, t.AudioURL, t.AudioPlayLimit, t.SectionType, id,
	)
	return err
}

func (r *Repository) DeleteTest(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM test WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) ListQuestions(ctx context.Context, testID uuid.UUID) ([]model.QuestionWithOptions, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT q.id, q.question_number, q.format, q.body, q.correct_answer, q.explanation, q.difficulty, q.image_url, q.audio_url, q.topic_id, et.name AS topic, q.point_correct, q.point_wrong, tq.sort_order
		FROM question q
		JOIN test_question tq ON tq.question_id = q.id
		LEFT JOIN exam_topic et ON et.id = q.topic_id
		WHERE tq.test_id = $1
		ORDER BY tq.sort_order`,
		testID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	questions := make([]model.QuestionWithOptions, 0)
	for rows.Next() {
		q := model.Question{}
		var sortOrder int
		var correctAnswer, explanation, difficulty, imageURL, audioURL, topic *string
		var topicID *uuid.UUID
		if err := rows.Scan(
			&q.ID, &q.QuestionNumber, &q.Format, &q.Body,
			&correctAnswer, &explanation, &difficulty, &imageURL, &audioURL,
			&topicID, &topic, &q.PointCorrect, &q.PointWrong, &sortOrder,
		); err != nil {
			return nil, err
		}
		if correctAnswer != nil {
			q.CorrectAnswer = correctAnswer
		}
		if explanation != nil {
			q.Explanation = explanation
		}
		if difficulty != nil {
			q.Difficulty = difficulty
		}
		if imageURL != nil {
			q.ImageURL = imageURL
		}
		if audioURL != nil {
			q.AudioURL = audioURL
		}
		if topicID != nil {
			q.TopicID = topicID
		}
		if topic != nil {
			q.Topic = topic
		}
		questions = append(questions, model.QuestionWithOptions{
			Question:  q,
			SortOrder: sortOrder,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	questionIDs := make([]uuid.UUID, len(questions))
	for i, q := range questions {
		questionIDs[i] = q.Question.ID
	}

	opts, err := r.queryOptionsForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, err
	}

	blanks, err := r.queryBlanksForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, err
	}

	acceptedAnswers, err := r.queryAcceptedAnswersForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, err
	}

	statements, err := r.queryStatementsForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, err
	}

	for i := range questions {
		qid := questions[i].Question.ID
		questions[i].Options = opts[qid]
		questions[i].Blanks = blanks[qid]
		questions[i].Question.Statements = statements[qid]
		questions[i].Question.AcceptedAnswers = acceptedAnswersOrFallback(acceptedAnswers[qid][0], derefString(questions[i].Question.CorrectAnswer))
		for bi := range questions[i].Blanks {
			b := &questions[i].Blanks[bi]
			b.AcceptedAnswers = acceptedAnswersOrFallback(acceptedAnswers[qid][b.Index], b.CorrectAnswer)
		}
	}
	return questions, nil
}

func (r *Repository) queryOptionsForQuestions(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID][]model.QuestionOption, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT question_id, key, text, image_url, is_correct, sort_order, points
		FROM question_option
		WHERE question_id = ANY($1)
		ORDER BY question_id, sort_order`,
		questionIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Seed every requested id with a non-nil empty slice so optionless formats
	// (short / fill_blank / essay) surface as [] not nil. A nil slice serializes
	// to JSON null and crashes the admin editor's q.options.length read on edit.
	out := make(map[uuid.UUID][]model.QuestionOption, len(questionIDs))
	for _, id := range questionIDs {
		out[id] = []model.QuestionOption{}
	}
	for rows.Next() {
		o := model.QuestionOption{}
		if err := scanQuestionOption(rows, &o); err != nil {
			return nil, err
		}
		out[o.QuestionID] = append(out[o.QuestionID], o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) queryBlanksForQuestions(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID][]model.QuestionBlank, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT question_id, blank_index, correct_answer, points
		FROM question_blank
		WHERE question_id = ANY($1)
		ORDER BY question_id, blank_index`,
		questionIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Seed every requested id with a non-nil empty slice so non-multi_blank formats
	// surface as [] not nil, consistent with options.
	out := make(map[uuid.UUID][]model.QuestionBlank, len(questionIDs))
	for _, id := range questionIDs {
		out[id] = []model.QuestionBlank{}
	}
	for rows.Next() {
		b := model.QuestionBlank{}
		if err := rows.Scan(&b.QuestionID, &b.Index, &b.CorrectAnswer, &b.Points); err != nil {
			return nil, err
		}
		out[b.QuestionID] = append(out[b.QuestionID], b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) queryStatementsForQuestions(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID][]model.QuestionStatement, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT question_id, statement_index, body, is_true, points
		FROM question_statement
		WHERE question_id = ANY($1)
		ORDER BY question_id, statement_index`,
		questionIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Seed every requested id with a non-nil empty slice so non-true_false formats
	// surface as [] not nil, consistent with options/blanks.
	out := make(map[uuid.UUID][]model.QuestionStatement, len(questionIDs))
	for _, id := range questionIDs {
		out[id] = []model.QuestionStatement{}
	}
	for rows.Next() {
		st := model.QuestionStatement{}
		if err := rows.Scan(&st.QuestionID, &st.Index, &st.Body, &st.IsTrue, &st.Points); err != nil {
			return nil, err
		}
		out[st.QuestionID] = append(out[st.QuestionID], st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// queryAcceptedAnswersForQuestions reads question_accepted_answer for a question set,
// keyed by question_id then blank_index (0 = question-level, mirroring
// queryOptionsForQuestions / queryBlanksForQuestions). Every requested id is seeded with
// a non-nil (possibly empty) inner map so callers can look up any blank_index safely.
func (r *Repository) queryAcceptedAnswersForQuestions(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID]map[int][]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT question_id, blank_index, answer
		FROM question_accepted_answer
		WHERE question_id = ANY($1)
		ORDER BY question_id, blank_index, answer_index`,
		questionIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]map[int][]string, len(questionIDs))
	for _, id := range questionIDs {
		out[id] = map[int][]string{}
	}
	for rows.Next() {
		var qid uuid.UUID
		var blankIndex int
		var answer string
		if err := rows.Scan(&qid, &blankIndex, &answer); err != nil {
			return nil, err
		}
		if out[qid] == nil {
			out[qid] = map[int][]string{}
		}
		out[qid][blankIndex] = append(out[qid][blankIndex], answer)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// acceptedAnswersOrFallback returns accepted when non-empty, else a single-element
// set built from the legacy scalar correct_answer column (FR-27) — [] when both are
// empty/unset, never nil.
func acceptedAnswersOrFallback(accepted []string, scalar string) []string {
	if len(accepted) > 0 {
		return accepted
	}
	if scalar != "" {
		return []string{scalar}
	}
	return []string{}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// stampScalarCorrectAnswers writes the first accepted answer into the legacy scalar
// correct_answer column — on the question and on each blank — so existing display
// and import paths keep working unchanged (FR-27). A question/blank with no accepted
// answers is left untouched (whatever correct_answer it already carries, if any).
func stampScalarCorrectAnswers(q *model.Question, blanks []model.QuestionBlank) {
	if len(q.AcceptedAnswers) > 0 {
		first := q.AcceptedAnswers[0]
		q.CorrectAnswer = &first
	}
	for i := range blanks {
		if len(blanks[i].AcceptedAnswers) > 0 {
			blanks[i].CorrectAnswer = blanks[i].AcceptedAnswers[0]
		}
	}
}

// replaceAcceptedAnswersTx deletes and reinserts question_accepted_answer rows for a
// question — delete-then-insert, the same shape used for options and blanks.
// blank_index 0 holds the question-level set; each blank's set is stored under its
// own blank_index.
func replaceAcceptedAnswersTx(ctx context.Context, tx pgx.Tx, questionID uuid.UUID, questionAccepted []string, blanks []model.QuestionBlank) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM question_accepted_answer WHERE question_id = $1`,
		questionID,
	); err != nil {
		return err
	}

	for i, a := range questionAccepted {
		if _, err := tx.Exec(ctx,
			`INSERT INTO question_accepted_answer (question_id, blank_index, answer_index, answer)
			VALUES ($1, 0, $2, $3)`,
			questionID, i+1, a,
		); err != nil {
			return err
		}
	}

	for _, b := range blanks {
		for i, a := range b.AcceptedAnswers {
			if _, err := tx.Exec(ctx,
				`INSERT INTO question_accepted_answer (question_id, blank_index, answer_index, answer)
				VALUES ($1, $2, $3, $4)`,
				questionID, b.Index, i+1, a,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Repository) CreateQuestionTx(ctx context.Context, tx pgx.Tx, q *model.Question, options []model.QuestionOption, blanks []model.QuestionBlank, statements []model.QuestionStatement) error {
	stampScalarCorrectAnswers(q, blanks)

	err := tx.QueryRow(ctx,
		`INSERT INTO question (format, body, correct_answer, explanation, difficulty, image_url, audio_url, topic_id, point_correct, point_wrong)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, question_number`,
		q.Format, q.Body, q.CorrectAnswer, q.Explanation, q.Difficulty, q.ImageURL, q.AudioURL, q.TopicID, q.PointCorrect, q.PointWrong,
	).Scan(&q.ID, &q.QuestionNumber)
	if err != nil {
		return err
	}

	if err := insertQuestionOptions(ctx, tx, q.ID, options); err != nil {
		return err
	}

	if err := insertQuestionBlanks(ctx, tx, q.ID, blanks); err != nil {
		return err
	}

	if err := insertQuestionStatements(ctx, tx, q.ID, statements); err != nil {
		return err
	}

	if err := replaceAcceptedAnswersTx(ctx, tx, q.ID, q.AcceptedAnswers, blanks); err != nil {
		return err
	}

	return nil
}

func (r *Repository) UpdateQuestionTx(ctx context.Context, tx pgx.Tx, q *model.Question, options []model.QuestionOption, blanks []model.QuestionBlank, statements []model.QuestionStatement) error {
	stampScalarCorrectAnswers(q, blanks)

	var updatedID uuid.UUID
	err := tx.QueryRow(ctx,
		`UPDATE question
		SET format = $1, body = $2, correct_answer = $3, explanation = $4, difficulty = $5, image_url = $6, audio_url = $7, topic_id = $8, point_correct = $9, point_wrong = $10
		WHERE id = $11 RETURNING id`,
		q.Format, q.Body, q.CorrectAnswer, q.Explanation, q.Difficulty, q.ImageURL, q.AudioURL, q.TopicID, q.PointCorrect, q.PointWrong, q.ID,
	).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgx.ErrNoRows
		}
		return err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM question_option WHERE question_id = $1`,
		q.ID,
	); err != nil {
		return err
	}

	if err := insertQuestionOptions(ctx, tx, q.ID, options); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM question_blank WHERE question_id = $1`,
		q.ID,
	); err != nil {
		return err
	}

	if err := insertQuestionBlanks(ctx, tx, q.ID, blanks); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM question_statement WHERE question_id = $1`,
		q.ID,
	); err != nil {
		return err
	}

	if err := insertQuestionStatements(ctx, tx, q.ID, statements); err != nil {
		return err
	}

	if err := replaceAcceptedAnswersTx(ctx, tx, q.ID, q.AcceptedAnswers, blanks); err != nil {
		return err
	}

	return nil
}

func (r *Repository) DeleteQuestion(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM question WHERE id = $1`,
		id,
	)
	return err
}

// GetQuestionByID returns a single question by id (no options/blanks/statements),
// used by SaveQuestion to compare a submitted format against the stored one
// before writing (FR-14).
func (r *Repository) GetQuestionByID(ctx context.Context, id uuid.UUID) (*model.Question, error) {
	q := &model.Question{}
	err := scanQuestion(r.pool.QueryRow(ctx,
		`SELECT id, question_number, format, body, correct_answer, explanation, difficulty, image_url, topic_id, point_correct, point_wrong
		FROM question
		WHERE id = $1`,
		id,
	), q)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

// questionInLiveExamSQL returns the boolean predicate for whether the question
// identified by questionIDExpr is attached (via test_question -> exam_test ->
// exam) to a live exam: sold through a published product, or already has at
// least one exam_session row. Shared by IsQuestionInLiveExam and
// ListBankQuestions so the delete guard and the format lock and the bank-list
// flag can never drift apart (FR-6/FR-7/FR-13/FR-14).
func questionInLiveExamSQL(questionIDExpr string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM test_question tq
		JOIN exam_test et ON et.test_id = tq.test_id
		JOIN exam e ON e.id = et.exam_id
		WHERE tq.question_id = %s
		  AND (
			EXISTS (
				SELECT 1 FROM product_exam pe
				JOIN product p ON p.id = pe.product_id
				WHERE pe.exam_id = e.id AND p.status = 'published'
			)
			OR EXISTS (
				SELECT 1 FROM exam_session es WHERE es.exam_id = e.id
			)
		  )
	)`, questionIDExpr)
}

// IsQuestionInLiveExam reports whether the question is attached to a live exam
// (FR-7/FR-14 delete-guard and format-lock predicate).
func (r *Repository) IsQuestionInLiveExam(ctx context.Context, questionID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT `+questionInLiveExamSQL("$1"),
		questionID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CountQuestionsByIDs returns how many of the supplied IDs exist in the question table.
func (r *Repository) CountQuestionsByIDs(ctx context.Context, ids []uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM question WHERE id = ANY($1)`,
		ids,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ListAttachedQuestionIDs returns the question_ids attached to a test, ordered by
// sort_order.
func (r *Repository) ListAttachedQuestionIDs(ctx context.Context, testID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT question_id FROM test_question WHERE test_id = $1 ORDER BY sort_order`,
		testID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// CountAnswerReferences returns the number of exam_session_answer rows for a question.
func (r *Repository) CountAnswerReferences(ctx context.Context, id uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM exam_session_answer WHERE question_id = $1`,
		id,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ListBankQuestions returns cursor-paginated bank questions with their topic name
// and the count of tests they are attached to (FR-14).
func (r *Repository) ListBankQuestions(ctx context.Context, filter QuestionFilter) ([]model.BankQuestionListItem, string, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	query := `SELECT q.id, q.question_number, q.format, q.body, q.correct_answer, q.explanation, q.difficulty, q.image_url, q.audio_url, q.topic_id, et.name AS topic, q.point_correct, q.point_wrong, COALESCE(tq.cnt, 0), ` +
		questionInLiveExamSQL("q.id") + ` AS in_live_exam
FROM question q
LEFT JOIN exam_topic et ON et.id = q.topic_id
LEFT JOIN (
    SELECT question_id, COUNT(*) AS cnt FROM test_question GROUP BY question_id
) tq ON tq.question_id = q.id
WHERE 1=1`
	where, args, argIdx := bankQuestionFilterSQL(filter, 1)
	query += where

	if filter.Cursor != "" {
		query += fmt.Sprintf(` AND q.question_number < $%d`, argIdx)
		args = append(args, filter.Cursor)
		argIdx++
	}

	query += ` ORDER BY q.question_number DESC LIMIT $` + fmt.Sprintf("%d", argIdx)
	args = append(args, filter.Limit+1)
	argIdx++
	if filter.Offset > 0 {
		query += fmt.Sprintf(` OFFSET $%d`, argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := make([]model.BankQuestionListItem, 0)
	for rows.Next() {
		q := model.Question{}
		var attachedCount int
		var inLiveExam bool
		var correctAnswer, explanation, difficulty, imageURL, audioURL, topic *string
		var topicID *uuid.UUID
		if err := rows.Scan(
			&q.ID, &q.QuestionNumber, &q.Format, &q.Body,
			&correctAnswer, &explanation, &difficulty, &imageURL, &audioURL,
			&topicID, &topic, &q.PointCorrect, &q.PointWrong, &attachedCount, &inLiveExam,
		); err != nil {
			return nil, "", err
		}
		if correctAnswer != nil {
			q.CorrectAnswer = correctAnswer
		}
		if explanation != nil {
			q.Explanation = explanation
		}
		if difficulty != nil {
			q.Difficulty = difficulty
		}
		if imageURL != nil {
			q.ImageURL = imageURL
		}
		if audioURL != nil {
			q.AudioURL = audioURL
		}
		if topicID != nil {
			q.TopicID = topicID
		}
		if topic != nil {
			q.Topic = topic
		}
		items = append(items, model.BankQuestionListItem{
			Question:      q,
			AttachedCount: attachedCount,
			InLiveExam:    inLiveExam,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(items) > filter.Limit {
		// Cursor is the question_number of the last *returned* row; the next page
		// continues strictly below it.
		nextCursor = strconv.Itoa(items[filter.Limit-1].Question.QuestionNumber)
		items = items[:filter.Limit]
	}

	questionIDs := make([]uuid.UUID, len(items))
	for i, item := range items {
		questionIDs[i] = item.Question.ID
	}
	opts, err := r.queryOptionsForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, "", err
	}

	blanks, err := r.queryBlanksForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, "", err
	}

	acceptedAnswers, err := r.queryAcceptedAnswersForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, "", err
	}

	statements, err := r.queryStatementsForQuestions(ctx, questionIDs)
	if err != nil {
		return nil, "", err
	}

	for i := range items {
		qid := items[i].Question.ID
		items[i].Options = opts[qid]
		items[i].Blanks = blanks[qid]
		items[i].Question.Statements = statements[qid]
		items[i].Question.AcceptedAnswers = acceptedAnswersOrFallback(acceptedAnswers[qid][0], derefString(items[i].Question.CorrectAnswer))
		for bi := range items[i].Blanks {
			b := &items[i].Blanks[bi]
			b.AcceptedAnswers = acceptedAnswersOrFallback(acceptedAnswers[qid][b.Index], b.CorrectAnswer)
		}
	}

	return items, nextCursor, nil
}

// AttachQuestionToTestTx appends an existing bank question to a test, assigning the
// next sort_order. Attaching an already-attached question is idempotent (no duplicate,
// no error — FR-21).
func (r *Repository) AttachQuestionToTestTx(ctx context.Context, tx pgx.Tx, testID, questionID uuid.UUID) error {
	nextOrder, err := r.GetMaxSortOrderForTestTx(ctx, tx, testID)
	if err != nil {
		return err
	}
	nextOrder++
	_, err = tx.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)
		ON CONFLICT (test_id, question_id) DO NOTHING`,
		testID, questionID, nextOrder,
	)
	return err
}

// GetMaxSortOrderForTestTx returns the current maximum sort_order for a test, or 0
// when the test has no attached questions.
func (r *Repository) GetMaxSortOrderForTestTx(ctx context.Context, tx pgx.Tx, testID uuid.UUID) (int, error) {
	var maxOrder int
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), 0) FROM test_question WHERE test_id = $1`,
		testID,
	).Scan(&maxOrder)
	return maxOrder, err
}

// FindQuestionsAttachedToSiblingTests returns the subset of questionIDs that are
// already attached to some OTHER test sharing an exam with testID. A reusable
// question attached to two tests inside the same exam would render twice in a
// session, but exam_session_answer keys answers only by question_id (its
// PRIMARY KEY is (session_id, question_id)) — a second occurrence overwrites the
// first. This is used to block that cross-test-in-same-exam collision at attach
// time (testID's own current attachments don't count as siblings).
func (r *Repository) FindQuestionsAttachedToSiblingTests(ctx context.Context, testID uuid.UUID, questionIDs []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT tq.question_id
		FROM test_question tq
		JOIN exam_test sibling ON sibling.test_id = tq.test_id
		JOIN exam_test target ON target.exam_id = sibling.exam_id
		WHERE target.test_id = $1
		  AND tq.test_id != $1
		  AND tq.question_id = ANY($2)`,
		testID, questionIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var colliding []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		colliding = append(colliding, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return colliding, nil
}

// AttachQuestionsToTestTx attaches a batch of bank questions to a test, appending
// after the current max sort_order and skipping already-attached questions (FR-21).
func (r *Repository) AttachQuestionsToTestTx(ctx context.Context, tx pgx.Tx, testID uuid.UUID, questionIDs []uuid.UUID) error {
	maxOrder, err := r.GetMaxSortOrderForTestTx(ctx, tx, testID)
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx,
		`SELECT question_id FROM test_question WHERE test_id = $1`,
		testID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[uuid.UUID]bool)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		existing[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	nextOrder := maxOrder + 1
	for _, qid := range questionIDs {
		if existing[qid] {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`,
			testID, qid, nextOrder,
		); err != nil {
			return err
		}
		nextOrder++
	}
	return nil
}

// DetachQuestionFromTest removes the test_question join row for (testID, questionID).
// It is idempotent: deleting a non-existent attachment returns no error (FR-22).
func (r *Repository) DetachQuestionFromTest(ctx context.Context, testID, questionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM test_question WHERE test_id = $1 AND question_id = $2`,
		testID, questionID,
	)
	return err
}

// ReorderTestQuestionsTx atomically rewrites sort_order for all attached questions
// to match the provided order. The offset rewrite avoids UNIQUE(test_id, sort_order)
// conflicts during the update (FR-23).
func (r *Repository) ReorderTestQuestionsTx(ctx context.Context, tx pgx.Tx, testID uuid.UUID, orderedQuestionIDs []uuid.UUID) error {
	// Shift existing orders far out of range so the subsequent per-row updates cannot
	// collide with each other or with stale values under the unique index.
	offset := len(orderedQuestionIDs) + 1000000
	if _, err := tx.Exec(ctx,
		`UPDATE test_question SET sort_order = sort_order + $1 WHERE test_id = $2`,
		offset, testID,
	); err != nil {
		return err
	}

	for i, qid := range orderedQuestionIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE test_question SET sort_order = $1 WHERE test_id = $2 AND question_id = $3`,
			i, testID, qid,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertQuestionOptions(ctx context.Context, tx pgx.Tx, questionID uuid.UUID, options []model.QuestionOption) error {
	for _, o := range options {
		_, err := tx.Exec(ctx,
			`INSERT INTO question_option (question_id, key, text, image_url, is_correct, sort_order, points)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			questionID, o.Key, o.Text, o.ImageURL, o.IsCorrect, o.SortOrder, o.Points,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertQuestionBlanks(ctx context.Context, tx pgx.Tx, questionID uuid.UUID, blanks []model.QuestionBlank) error {
	for _, b := range blanks {
		_, err := tx.Exec(ctx,
			`INSERT INTO question_blank (question_id, blank_index, correct_answer, points)
			VALUES ($1, $2, $3, $4)`,
			questionID, b.Index, b.CorrectAnswer, b.Points,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertQuestionStatements(ctx context.Context, tx pgx.Tx, questionID uuid.UUID, statements []model.QuestionStatement) error {
	for _, st := range statements {
		_, err := tx.Exec(ctx,
			`INSERT INTO question_statement (question_id, statement_index, body, is_true, points)
			VALUES ($1, $2, $3, $4, $5)`,
			questionID, st.Index, st.Body, st.IsTrue, st.Points,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExamFilter mirrors TestFilter/ProductFilter for cursor-paginated ListExams.
type ExamFilter struct {
	Cursor string
	Limit  int
	Q      string
	Status string
	// SchoolFilter scopes the registration_count subquery to one school's
	// students; nil counts every registration (mirrors GetExamRoster).
	SchoolFilter *string
}

// decodeCardNotes turns the card_notes jsonb column into []string. pgx would
// otherwise treat a []string destination as a Postgres text[], not jsonb.
func decodeCardNotes(raw []byte, dst *[]string) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// encodeCardNotes renders []string for the card_notes jsonb column, never NULL.
func encodeCardNotes(notes []string) []byte {
	if notes == nil {
		notes = []string{}
	}
	raw, err := json.Marshal(notes)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

func scanExam(row interface{ Scan(dest ...any) error }, e *model.Exam) error {
	var cardNotes []byte
	err := row.Scan(
		&e.ID, &e.Title, &e.IsFree, &e.ScheduledAt, &e.ScheduledEndAt,
		&e.RequiresCheckin, &e.AllowLeaderboard, &e.CDNBundle,
		&e.BundleURL, &e.BundleGeneratedAt,
		&e.CheckInWindowMinutes, &e.GraceWindowMinutes, &e.MaxAttempts,
		&e.TimerMode, &e.DurationMinutes, &e.Randomize,
		&e.ResultConfig, &e.ResultReleaseAt, &e.Status, &e.CreatedAt,
		&e.Mode, &e.CertificateDesign, &e.CertificateDesignUpdatedAt,
		&e.ExamNumber, &e.CertificateEnabled, &e.CertificateTemplateHTML,
		&e.EndScreenImageURL, &e.EndScreenPromoText,
		&e.CardEnabled, &cardNotes,
	)
	if err != nil {
		return err
	}
	return decodeCardNotes(cardNotes, &e.CardNotes)
}

// scanExamListItem scans an Exam plus the trailing has_published_product column
// added by ListExams's query.
func scanExamListItem(row interface{ Scan(dest ...any) error }, item *model.ExamListItem) error {
	var cardNotes []byte
	err := row.Scan(
		&item.ID, &item.Title, &item.IsFree, &item.ScheduledAt, &item.ScheduledEndAt,
		&item.RequiresCheckin, &item.AllowLeaderboard, &item.CDNBundle,
		&item.BundleURL, &item.BundleGeneratedAt,
		&item.CheckInWindowMinutes, &item.GraceWindowMinutes, &item.MaxAttempts,
		&item.TimerMode, &item.DurationMinutes, &item.Randomize,
		&item.ResultConfig, &item.ResultReleaseAt, &item.Status, &item.CreatedAt,
		&item.Mode, &item.CertificateDesign, &item.CertificateDesignUpdatedAt,
		&item.ExamNumber, &item.CertificateEnabled, &item.CertificateTemplateHTML,
		&item.EndScreenImageURL, &item.EndScreenPromoText,
		&item.CardEnabled, &cardNotes,
		&item.HasPublishedProduct, &item.RegistrationCount,
	)
	if err != nil {
		return err
	}
	return decodeCardNotes(cardNotes, &item.CardNotes)
}

// CreateExam inserts a standalone exam row — no product is created or linked here.
// Selling an exam is a separate step: attach it to a Product via product_exam (see
// CreateProductWithExams/ReplaceProductExams), mirroring how course-type products attach
// existing Courses.
func (r *Repository) CreateExam(ctx context.Context, e *model.Exam) error {
	// mode is NOT NULL DEFAULT 'standard'; COALESCE empty caller value to the default
	// so existing callers that don't set Mode keep working. RETURNING mode stamps the
	// resolved value back into the struct.
	return r.pool.QueryRow(ctx,
		`INSERT INTO exam (title, is_free, scheduled_at, scheduled_end_at, requires_checkin, allow_leaderboard,
			cdn_bundle, bundle_url, bundle_generated_at, check_in_window_minutes, grace_window_minutes,
			max_attempts, timer_mode, duration_minutes, randomize, result_config, result_release_at,
			status, mode,
			certificate_design, certificate_design_updated_at, card_notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			COALESCE(NULLIF($19, ''), 'standard'), $20, $21, $22)
		RETURNING id, created_at, mode, exam_number`,
		e.Title, e.IsFree, e.ScheduledAt, e.ScheduledEndAt, e.RequiresCheckin, e.AllowLeaderboard,
		e.CDNBundle, e.BundleURL, e.BundleGeneratedAt, e.CheckInWindowMinutes, e.GraceWindowMinutes,
		e.MaxAttempts, e.TimerMode, e.DurationMinutes, e.Randomize, e.ResultConfig, e.ResultReleaseAt,
		e.Status, e.Mode,
		e.CertificateDesign, e.CertificateDesignUpdatedAt, encodeCardNotes(e.CardNotes),
	).Scan(&e.ID, &e.CreatedAt, &e.Mode, &e.ExamNumber)
}

func (r *Repository) GetExamByID(ctx context.Context, id uuid.UUID) (*model.Exam, error) {
	out := &model.Exam{}
	err := scanExam(r.pool.QueryRow(ctx,
		`SELECT id, title, is_free, scheduled_at, scheduled_end_at, requires_checkin, allow_leaderboard,
			cdn_bundle, bundle_url, bundle_generated_at, check_in_window_minutes, grace_window_minutes,
			max_attempts, timer_mode, duration_minutes, randomize, result_config, result_release_at,
			status, created_at, mode,
			certificate_design, certificate_design_updated_at, exam_number, certificate_enabled, certificate_template_html,
			end_screen_image_url, end_screen_promo_text, card_enabled, card_notes
		FROM exam
		WHERE id = $1`,
		id,
	), out)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return out, nil
}

// GetExamsByProductID returns all exams linked to a product via product_exam,
// mirroring GetCoursesByProductID.
func (r *Repository) GetExamsByProductID(ctx context.Context, productID uuid.UUID) ([]model.Exam, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, e.title, e.is_free, e.scheduled_at, e.scheduled_end_at, e.requires_checkin, e.allow_leaderboard,
			e.cdn_bundle, e.bundle_url, e.bundle_generated_at, e.check_in_window_minutes, e.grace_window_minutes,
			e.max_attempts, e.timer_mode, e.duration_minutes, e.randomize, e.result_config, e.result_release_at,
			e.status, e.created_at, e.mode,
			e.certificate_design, e.certificate_design_updated_at, e.exam_number, e.certificate_enabled, e.certificate_template_html,
		e.end_screen_image_url, e.end_screen_promo_text, e.card_enabled, e.card_notes
		FROM exam e
		JOIN product_exam pe ON pe.exam_id = e.id
		WHERE pe.product_id = $1
		ORDER BY e.title`,
		productID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exams := []model.Exam{}
	for rows.Next() {
		var e model.Exam
		if err := scanExam(rows, &e); err != nil {
			return nil, err
		}
		exams = append(exams, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return exams, nil
}

func (r *Repository) ListExams(ctx context.Context, filter ExamFilter) ([]model.ExamListItem, string, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	query := `SELECT e.id, e.title, e.is_free, e.scheduled_at, e.scheduled_end_at, e.requires_checkin, e.allow_leaderboard,
		e.cdn_bundle, e.bundle_url, e.bundle_generated_at, e.check_in_window_minutes, e.grace_window_minutes,
		e.max_attempts, e.timer_mode, e.duration_minutes, e.randomize, e.result_config, e.result_release_at,
		e.status, e.created_at, e.mode,
		e.certificate_design, e.certificate_design_updated_at, e.exam_number, e.certificate_enabled, e.certificate_template_html,
		e.end_screen_image_url, e.end_screen_promo_text, e.card_enabled, e.card_notes,
		EXISTS (
			SELECT 1 FROM product_exam pe
			JOIN product p ON p.id = pe.product_id
			WHERE pe.exam_id = e.id AND p.status = 'published'
			  AND (p.available_from IS NULL OR p.available_from <= now())
			  AND (p.available_until IS NULL OR p.available_until >= now())
		) AS has_published_product,
		(
			SELECT COUNT(*) FROM exam_registration r
			JOIN users u ON u.id = r.student_id
			WHERE r.exam_id = e.id AND ($1::uuid IS NULL OR u.school_id = $1)
		) AS registration_count
	FROM exam e
	WHERE 1=1`
	// registration_count's placeholder is rendered in the SELECT list, before
	// WHERE, so it must bind first regardless of which other predicates apply.
	args := []interface{}{filter.SchoolFilter}
	argIdx := 2

	if filter.Q != "" {
		query += fmt.Sprintf(` AND e.title ILIKE '%%' || $%d || '%%'`, argIdx)
		args = append(args, filter.Q)
		argIdx++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(` AND e.status = $%d`, argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Cursor != "" {
		query += fmt.Sprintf(` AND e.id > $%d`, argIdx)
		args = append(args, filter.Cursor)
		argIdx++
	}

	query += ` ORDER BY e.id LIMIT $` + fmt.Sprintf("%d", argIdx)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := []model.ExamListItem{}
	for rows.Next() {
		var item model.ExamListItem
		if err := scanExamListItem(rows, &item); err != nil {
			return nil, "", err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(items) > filter.Limit {
		nextCursor = items[filter.Limit].ID.String()
		items = items[:filter.Limit]
	}

	return items, nextCursor, nil
}

func (r *Repository) GetExamDetail(ctx context.Context, id uuid.UUID) (*model.ExamDetail, error) {
	detail := &model.ExamDetail{}
	err := scanExam(r.pool.QueryRow(ctx,
		`SELECT e.id, e.title, e.is_free, e.scheduled_at, e.scheduled_end_at, e.requires_checkin, e.allow_leaderboard,
			e.cdn_bundle, e.bundle_url, e.bundle_generated_at, e.check_in_window_minutes, e.grace_window_minutes,
			e.max_attempts, e.timer_mode, e.duration_minutes, e.randomize, e.result_config, e.result_release_at,
			e.status, e.created_at, e.mode,
			e.certificate_design, e.certificate_design_updated_at, e.exam_number, e.certificate_enabled, e.certificate_template_html,
		e.end_screen_image_url, e.end_screen_promo_text, e.card_enabled, e.card_notes
		FROM exam e
		WHERE e.id = $1`,
		id,
	), &detail.Exam)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT et.id, et.exam_id, et.test_id, et.sort_order,
			t.id, t.title, t.subject, t.topic, t.duration_minutes, t.section_type,
			COALESCE(q.cnt, 0)
		FROM exam_test et
		JOIN test t ON t.id = et.test_id
		LEFT JOIN (
			SELECT test_id, COUNT(*) AS cnt FROM test_question GROUP BY test_id
		) q ON q.test_id = t.id
		WHERE et.exam_id = $1
		ORDER BY et.sort_order ASC`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []model.ExamTestEntry
	for rows.Next() {
		var entry model.ExamTestEntry
		var topic *string
		var sectionType *string
		if err := rows.Scan(
			&entry.ID, &entry.ExamID, &entry.TestID, &entry.SortOrder,
			&entry.Test.ID, &entry.Test.Title, &entry.Test.Subject, &topic, &entry.Test.DurationMinutes,
			&sectionType, &entry.Test.QuestionCount,
		); err != nil {
			return nil, err
		}
		entry.Test.Topic = topic
		entry.Test.SectionType = sectionType
		tests = append(tests, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tests == nil {
		tests = []model.ExamTestEntry{}
	}
	detail.Tests = tests

	return detail, nil
}

// execer is satisfied by both *pgxpool.Pool and pgx.Tx — it lets UpdateExam
// and SetExamCertificateEnabled share one SQL statement each with their
// _Tx twin instead of hand-duplicating a 20+ column UPDATE, which is exactly
// how certificate_template_html went missing from the SET clause in the
// first place (present in every SELECT, absent from this one hand-copied
// statement).
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *Repository) UpdateExam(ctx context.Context, id uuid.UUID, e *model.Exam) error {
	return updateExam(ctx, r.pool, id, e)
}

// UpdateExamTx is UpdateExam run against a caller-supplied transaction, so an
// exam update and its outbox fan-out (async redesign 2026-08-02 — certificate
// design/template edits re-enqueue CertificateNeeded for already-submitted
// sessions) commit atomically.
func (r *Repository) UpdateExamTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, e *model.Exam) error {
	return updateExam(ctx, tx, id, e)
}

func updateExam(ctx context.Context, q execer, id uuid.UUID, e *model.Exam) error {
	tag, err := q.Exec(ctx,
		`UPDATE exam
		SET title = $1, is_free = $2, scheduled_at = $3, scheduled_end_at = $4, requires_checkin = $5, allow_leaderboard = $6,
			cdn_bundle = $7, bundle_url = $8, bundle_generated_at = $9,
			check_in_window_minutes = $10, grace_window_minutes = $11, max_attempts = $12,
			timer_mode = $13, duration_minutes = $14, randomize = $15,
			result_config = $16, result_release_at = $17, status = $18,
			mode = COALESCE(NULLIF($19, ''), mode),
			certificate_design = $20, certificate_design_updated_at = $21,
			end_screen_image_url = $22, end_screen_promo_text = $23,
			certificate_template_html = $24, card_notes = $25
		WHERE id = $26`,
		e.Title, e.IsFree, e.ScheduledAt, e.ScheduledEndAt, e.RequiresCheckin, e.AllowLeaderboard,
		e.CDNBundle, e.BundleURL, e.BundleGeneratedAt,
		e.CheckInWindowMinutes, e.GraceWindowMinutes, e.MaxAttempts,
		e.TimerMode, e.DurationMinutes, e.Randomize,
		e.ResultConfig, e.ResultReleaseAt, e.Status, e.Mode,
		e.CertificateDesign, e.CertificateDesignUpdatedAt,
		e.EndScreenImageURL, e.EndScreenPromoText,
		e.CertificateTemplateHTML, encodeCardNotes(e.CardNotes), id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearRegistrationCardsByExamTx drops the cached card PDF key for one exam's
// registrations, so the next download re-renders. The object key is
// deterministic (cards/<regID>.pdf), so a regenerated card overwrites rather
// than orphaning the old one.
func (r *Repository) ClearRegistrationCardsByExamTx(ctx context.Context, tx pgx.Tx, examID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE exam_registration SET card_key = NULL WHERE exam_id = $1 AND card_key IS NOT NULL`, examID)
	return err
}

// ClearAllRegistrationCards is the same invalidation for a system_config change,
// which affects every card rather than one exam's.
func (r *Repository) ClearAllRegistrationCards(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `UPDATE exam_registration SET card_key = NULL WHERE card_key IS NOT NULL`)
	return err
}

// SetExamCardEnabled flips card_enabled in isolation, never card_notes, so
// toggling the card off and back on preserves the admin's notes.
func (r *Repository) SetExamCardEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE exam SET card_enabled = $1 WHERE id = $2`, enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetExamCertificateEnabled flips certificate_enabled in isolation — never
// certificate_design or certificate_design_updated_at (FR-11/FR-12) — a
// single-column UPDATE mirroring UpdateSchoolStatus.
func (r *Repository) SetExamCertificateEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	return setExamCertificateEnabled(ctx, r.pool, id, enabled)
}

// SetExamCertificateEnabledTx is SetExamCertificateEnabled run against a
// caller-supplied transaction, so flipping the flag on and fanning out
// CertificateNeeded for already-submitted sessions (Finding 3, 2026-08 review)
// commit atomically.
func (r *Repository) SetExamCertificateEnabledTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, enabled bool) error {
	return setExamCertificateEnabled(ctx, tx, id, enabled)
}

func setExamCertificateEnabled(ctx context.Context, q execer, id uuid.UUID, enabled bool) error {
	tag, err := q.Exec(ctx, `UPDATE exam SET certificate_enabled = $1 WHERE id = $2`, enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSubmittedSessionIDsTx returns every submitted exam_session id for an
// exam, regardless of grading state (unlike dedupedSubmittedSessions, which
// dedupes to one row per registration and is scoped to leaderboard/analytics
// display — regeneration must reach every submitted session, not just the
// latest graded attempt per student). Used to fan out CertificateNeeded when
// a design/template edit or a certificate_enabled flip makes previously
// ineligible sessions eligible (Findings 2/3, 2026-08 review).
func (r *Repository) ListSubmittedSessionIDsTx(ctx context.Context, tx pgx.Tx, examID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `SELECT id FROM exam_session WHERE exam_id = $1 AND status = 'submitted'`, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ReplaceExamTestsTx atomically replaces all exam_test links for an exam. Caller
// supplies the tx; verification of test existence is the caller's responsibility
// (mirrors ReplaceProductCourses).
func (r *Repository) ReplaceExamTestsTx(ctx context.Context, tx pgx.Tx, examID uuid.UUID, tests []model.ExamTest) error {
	if _, err := tx.Exec(ctx, `DELETE FROM exam_test WHERE exam_id = $1`, examID); err != nil {
		return err
	}
	for _, t := range tests {
		if _, err := tx.Exec(ctx,
			`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`,
			examID, t.TestID, t.SortOrder,
		); err != nil {
			return err
		}
	}
	return nil
}

// CreateExamRegistration inserts a row using ON CONFLICT DO NOTHING — outbox
// re-delivery (same OrderPaid event processed twice) collapses to a no-op when
// (student_id, exam_id) already exists. RowsAffected == 0 is success, not error.
func (r *Repository) CreateExamRegistration(ctx context.Context, tx pgx.Tx, reg model.ExamRegistration) error {
	// Serialize participant-number assignment per exam so concurrent
	// registrations (outbox fan-out, admin grant) can't compute the same
	// MAX(participant_number)+1 and collide on uq_examregistration_participant.
	// The advisory lock is held to end-of-tx and is re-entrant within one tx
	// (fan-out loops register several students under the same lock).
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext('exam_participant_number'), hashtext($1::text))`,
		reg.ExamID,
	); err != nil {
		return err
	}
	// ON CONFLICT DO NOTHING keeps re-delivery idempotent; when it skips, the
	// MAX+1 subquery result is simply discarded so no number is consumed.
	_, err := tx.Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number)
		VALUES ($1, $2, $3, $4,
			(SELECT COALESCE(MAX(participant_number), 0) + 1
			 FROM exam_registration WHERE exam_id = $2))
		ON CONFLICT (student_id, exam_id) DO NOTHING`,
		reg.StudentID, reg.ExamID, reg.Token, reg.Status,
	)
	return err
}

func (r *Repository) StampOrderItemFulfilledAt(ctx context.Context, tx pgx.Tx, orderID, productID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE order_item SET fulfilled_at = now() WHERE order_id = $1 AND product_id = $2`,
		orderID, productID,
	)
	return err
}

func (r *Repository) GetExamRegistrationsByStudent(ctx context.Context, studentID uuid.UUID) ([]model.RegistrationListItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT reg.id, reg.student_id, reg.exam_id, reg.token, reg.card_key,
			reg.checked_in_at, reg.attempts_used, reg.status, reg.created_at,
			e.title, e.scheduled_at, e.scheduled_end_at, e.is_free, e.requires_checkin,
			e.check_in_window_minutes, e.duration_minutes, e.max_attempts, s.id
		FROM exam_registration reg
		JOIN exam e ON e.id = reg.exam_id
		LEFT JOIN LATERAL (
			SELECT id FROM exam_session
			WHERE registration_id = reg.id
			ORDER BY attempt_number DESC
			LIMIT 1
		) s ON true
		WHERE reg.student_id = $1
		ORDER BY reg.created_at DESC`,
		studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.RegistrationListItem
	for rows.Next() {
		var item model.RegistrationListItem
		var cardKey *string
		var checkedInAt *time.Time
		if err := rows.Scan(
			&item.ID, &item.StudentID, &item.ExamID, &item.Token, &cardKey,
			&checkedInAt, &item.AttemptsUsed, &item.Status, &item.CreatedAt,
			&item.ExamTitle, &item.ScheduledAt, &item.ScheduledEndAt, &item.IsFree, &item.RequiresCheckin,
			&item.CheckInWindowMinutes, &item.DurationMinutes, &item.MaxAttempts, &item.SessionID,
		); err != nil {
			return nil, err
		}
		if cardKey != nil {
			item.CardKey = cardKey
		}
		if checkedInAt != nil {
			item.CheckedInAt = checkedInAt
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.RegistrationListItem{}
	}
	return items, nil
}

func (r *Repository) GetExamRegistrationByID(ctx context.Context, regID, studentID uuid.UUID) (*model.RegistrationDetail, error) {
	var detail model.RegistrationDetail
	var cardKey *string
	var checkedInAt *time.Time
	var cardNotesRaw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT reg.id, reg.student_id, reg.exam_id, reg.token, reg.card_key,
			reg.checked_in_at, reg.attempts_used, reg.status, reg.created_at, reg.participant_number,
			e.id, e.title, e.scheduled_at, e.scheduled_end_at, e.requires_checkin, e.check_in_window_minutes,
			e.timer_mode, e.duration_minutes, e.result_config, e.exam_number,
			e.card_enabled, e.card_notes,
			COALESCE((
				SELECT string_agg(DISTINCT t.subject, ', ')
				FROM exam_test et JOIN test t ON t.id = et.test_id
				WHERE et.exam_id = e.id
			), '') AS subject
		FROM exam_registration reg
		JOIN exam e ON e.id = reg.exam_id
		WHERE reg.id = $1 AND reg.student_id = $2`,
		regID, studentID,
	).Scan(
		&detail.ID, &detail.StudentID, &detail.ExamID, &detail.Token, &cardKey,
		&checkedInAt, &detail.AttemptsUsed, &detail.Status, &detail.CreatedAt, &detail.ParticipantNumber,
		&detail.Exam.ID, &detail.Exam.Title, &detail.Exam.ScheduledAt, &detail.Exam.ScheduledEndAt, &detail.Exam.RequiresCheckin,
		&detail.Exam.CheckInWindowMinutes, &detail.Exam.TimerMode, &detail.Exam.DurationMinutes,
		&detail.Exam.ResultConfig, &detail.Exam.ExamNumber,
		&detail.Exam.CardEnabled, &cardNotesRaw, &detail.Subject,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if cardKey != nil {
		detail.CardKey = cardKey
	}
	if checkedInAt != nil {
		detail.CheckedInAt = checkedInAt
	}
	if err := decodeCardNotes(cardNotesRaw, &detail.Exam.CardNotes); err != nil {
		return nil, err
	}
	return &detail, nil
}

// GetRegistrationForPrint returns a registration by id alone, with no student
// scoping (mirrors GetExamRegistrationByID minus the student_id predicate):
// the card print-data endpoint (Task 7) authorizes via a print token bound to
// the registration id, so there is no student id available to scope by at
// that call site.
func (r *Repository) GetRegistrationForPrint(ctx context.Context, regID uuid.UUID) (*model.RegistrationDetail, error) {
	var detail model.RegistrationDetail
	var cardKey *string
	var checkedInAt *time.Time
	var cardNotesRaw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT reg.id, reg.student_id, reg.exam_id, reg.token, reg.card_key,
			reg.checked_in_at, reg.attempts_used, reg.status, reg.created_at, reg.participant_number,
			e.id, e.title, e.scheduled_at, e.scheduled_end_at, e.requires_checkin, e.check_in_window_minutes,
			e.timer_mode, e.duration_minutes, e.result_config, e.exam_number,
			e.card_enabled, e.card_notes,
			COALESCE((
				SELECT string_agg(DISTINCT t.subject, ', ')
				FROM exam_test et JOIN test t ON t.id = et.test_id
				WHERE et.exam_id = e.id
			), '') AS subject
		FROM exam_registration reg
		JOIN exam e ON e.id = reg.exam_id
		WHERE reg.id = $1`,
		regID,
	).Scan(
		&detail.ID, &detail.StudentID, &detail.ExamID, &detail.Token, &cardKey,
		&checkedInAt, &detail.AttemptsUsed, &detail.Status, &detail.CreatedAt, &detail.ParticipantNumber,
		&detail.Exam.ID, &detail.Exam.Title, &detail.Exam.ScheduledAt, &detail.Exam.ScheduledEndAt, &detail.Exam.RequiresCheckin,
		&detail.Exam.CheckInWindowMinutes, &detail.Exam.TimerMode, &detail.Exam.DurationMinutes,
		&detail.Exam.ResultConfig, &detail.Exam.ExamNumber,
		&detail.Exam.CardEnabled, &cardNotesRaw, &detail.Subject,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if cardKey != nil {
		detail.CardKey = cardKey
	}
	if checkedInAt != nil {
		detail.CheckedInAt = checkedInAt
	}
	if err := decodeCardNotes(cardNotesRaw, &detail.Exam.CardNotes); err != nil {
		return nil, err
	}
	return &detail, nil
}

// GetExamRoster returns every registration for an exam joined with the
// student's name/username, the exam's scheduled_at/exam_number (the
// ingredients the service needs to compose each row's FR-24 display
// participant number), and the registration's check-in token (FR-47), ordered
// by participant_number (NULLs — rows predating the FR-24 backfill — sort
// last, then by registration time).
// GetExamRoster's schoolFilter, when non-nil, constrains rows to students of
// that school (tenant isolation for admin_school — a nil filter is the
// all-schools view used by super_admin/admin_exam).
func (r *Repository) GetExamRoster(ctx context.Context, examID uuid.UUID, schoolFilter *string) ([]model.ExamRosterEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT reg.id, reg.student_id, u.name, u.username, reg.participant_number,
			reg.status, reg.checked_in_at, reg.created_at, e.scheduled_at, e.exam_number,
			reg.token
		FROM exam_registration reg
		JOIN exam e ON e.id = reg.exam_id
		JOIN users u ON u.id = reg.student_id
		WHERE reg.exam_id = $1 AND ($2::uuid IS NULL OR u.school_id = $2)
		ORDER BY reg.participant_number NULLS LAST, reg.created_at`,
		examID, schoolFilter,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ExamRosterEntry
	for rows.Next() {
		var item model.ExamRosterEntry
		if err := rows.Scan(
			&item.RegistrationID, &item.StudentID, &item.StudentName, &item.StudentUsername,
			&item.ParticipantNumber, &item.Status, &item.CheckedInAt, &item.RegisteredAt,
			&item.ExamScheduledAt, &item.ExamNumber, &item.Token,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.ExamRosterEntry{}
	}
	return items, nil
}

// ---------- Session scan helpers ----------

func scanExamSession(row interface{ Scan(dest ...any) error }, s *model.ExamSession) error {
	return row.Scan(
		&s.ID, &s.RegistrationID, &s.StudentID, &s.ExamID,
		&s.AttemptNumber, &s.StartedAt, &s.SubmittedAt,
		&s.ExtendedUntil, &s.AdminSubmitted, &s.Score,
		&s.CertificateKey, &s.CertificateGeneratedAt, &s.CertificateNumber, &s.LastSavedAt,
		&s.CurrentPosition, &s.Status, &s.CreatedAt,
	)
}

func scanExamSessionAnswer(row interface{ Scan(dest ...any) error }, a *model.ExamSessionAnswer) error {
	return row.Scan(
		&a.SessionID, &a.QuestionID, &a.Answer, &a.IsCorrect, &a.Score,
		&a.GradedBy, &a.GradedAt, &a.GraderComment, &a.FlaggedForReview, &a.SavedAt,
	)
}

// ---------- Session repository methods ----------

// GetExamRegistrationByToken retrieves a registration by student ID and token.
// Returns ErrNotFound when no match exists.
func (r *Repository) GetExamRegistrationByToken(ctx context.Context, studentID uuid.UUID, token string) (*model.ExamRegistration, error) {
	var reg model.ExamRegistration
	var cardKey *string
	var checkedInAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT id, student_id, exam_id, token, card_key, checked_in_at, attempts_used, status, created_at
		FROM exam_registration
		WHERE student_id = $1 AND token = $2`,
		studentID, token,
	).Scan(
		&reg.ID, &reg.StudentID, &reg.ExamID, &reg.Token,
		&cardKey, &checkedInAt, &reg.AttemptsUsed, &reg.Status, &reg.CreatedAt,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if cardKey != nil {
		reg.CardKey = cardKey
	}
	if checkedInAt != nil {
		reg.CheckedInAt = checkedInAt
	}
	return &reg, nil
}

// CheckInExamTx stamps checked_in_at (if NULL) and sets status='checked_in'.
func (r *Repository) CheckInExamTx(ctx context.Context, tx pgx.Tx, regID uuid.UUID) error {
	tag, err := tx.Exec(ctx,
		`UPDATE exam_registration
		SET checked_in_at = COALESCE(checked_in_at, now()), status = 'checked_in'
		WHERE id = $1`,
		regID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateExamSessionTx increments attempts_used, sets status='in_progress',
// optionally stamps checked_in_at when NULL, and inserts an exam_session row.
// The attempts_used < ceiling predicate is the atomic attempt guard: the service's
// read-then-act check alone would let two concurrent starts both pass. maxAttempts
// is the exam's raw max_attempts column: nil means unlimited, 0 or 1 means a
// single sitting (COALESCE(NULLIF($2, 0), 1)), >= 2 is that ceiling. A
// FOR UPDATE on the registration row plus a live-session count is the
// one-live-session lock — a second create while a sitting is still open
// returns ErrNoAttemptsLeft even when the ceiling is unlimited.
func (r *Repository) CreateExamSessionTx(ctx context.Context, tx pgx.Tx, reg model.ExamRegistration, maxAttempts *int) (model.ExamSession, error) {
	var lockedID uuid.UUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM exam_registration WHERE id = $1 FOR UPDATE`,
		reg.ID,
	).Scan(&lockedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ExamSession{}, ErrNotFound
		}
		return model.ExamSession{}, err
	}

	var live int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM exam_session WHERE registration_id = $1 AND status = 'in_progress'`,
		reg.ID,
	).Scan(&live)
	if err != nil {
		return model.ExamSession{}, err
	}
	if live > 0 {
		return model.ExamSession{}, ErrNoAttemptsLeft
	}

	var attemptsUsed int
	err = tx.QueryRow(ctx,
		`UPDATE exam_registration
		SET attempts_used = attempts_used + 1,
		    status = 'in_progress',
		    checked_in_at = COALESCE(checked_in_at, now())
		WHERE id = $1
		  AND ($2::int IS NULL OR attempts_used < COALESCE(NULLIF($2::int, 0), 1))
		RETURNING attempts_used`,
		reg.ID, maxAttempts,
	).Scan(&attemptsUsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ExamSession{}, ErrNoAttemptsLeft
		}
		return model.ExamSession{}, err
	}

	var s model.ExamSession
	err = tx.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status)
		VALUES ($1, $2, $3, $4, now(), 'in_progress')
		RETURNING id, registration_id, student_id, exam_id, attempt_number, started_at,
			submitted_at, extended_until, admin_submitted, score, certificate_key,
			certificate_generated_at, certificate_number, last_saved_at, status, created_at`,
		reg.ID, reg.StudentID, reg.ExamID, attemptsUsed,
	).Scan(
		&s.ID, &s.RegistrationID, &s.StudentID, &s.ExamID,
		&s.AttemptNumber, &s.StartedAt, &s.SubmittedAt,
		&s.ExtendedUntil, &s.AdminSubmitted, &s.Score,
		&s.CertificateKey, &s.CertificateGeneratedAt, &s.CertificateNumber, &s.LastSavedAt, &s.Status, &s.CreatedAt,
	)
	if err != nil {
		return model.ExamSession{}, err
	}
	return s, nil
}

// GetInProgressSessionForRegistration returns the student's live session for a
// registration, if any. When more than one in_progress row exists (legacy data),
// the highest attempt_number wins. ErrNotFound when none is live.
func (r *Repository) GetInProgressSessionForRegistration(ctx context.Context, registrationID, studentID uuid.UUID) (*model.ExamSession, error) {
	var s model.ExamSession
	err := scanExamSession(r.pool.QueryRow(ctx,
		`SELECT id, registration_id, student_id, exam_id, attempt_number, started_at,
			submitted_at, extended_until, admin_submitted, score, certificate_key,
			certificate_generated_at, certificate_number, last_saved_at, current_position, status, created_at
		FROM exam_session
		WHERE registration_id = $1 AND student_id = $2 AND status = 'in_progress'
		ORDER BY attempt_number DESC
		LIMIT 1`,
		registrationID, studentID,
	), &s)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// GetExamSessionForStudent returns a session scoped to the owning student.
func (r *Repository) GetExamSessionForStudent(ctx context.Context, sessionID, studentID uuid.UUID) (*model.ExamSession, error) {
	var s model.ExamSession
	err := scanExamSession(r.pool.QueryRow(ctx,
		`SELECT id, registration_id, student_id, exam_id, attempt_number, started_at,
			submitted_at, extended_until, admin_submitted, score, certificate_key,
			certificate_generated_at, certificate_number, last_saved_at, current_position, status, created_at
		FROM exam_session
		WHERE id = $1 AND student_id = $2`,
		sessionID, studentID,
	), &s)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// GetExamSessionByID returns a session by ID without ownership filter (admin use).
func (r *Repository) GetExamSessionByID(ctx context.Context, sessionID uuid.UUID) (*model.ExamSession, error) {
	var s model.ExamSession
	err := scanExamSession(r.pool.QueryRow(ctx,
		`SELECT id, registration_id, student_id, exam_id, attempt_number, started_at,
			submitted_at, extended_until, admin_submitted, score, certificate_key,
			certificate_generated_at, certificate_number, last_saved_at, current_position, status, created_at
		FROM exam_session
		WHERE id = $1`,
		sessionID,
	), &s)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// GetSessionWithQuestions returns the ordered test->question->option tree for an exam.
// Reuses GetTestDetail for each attached test.
func (r *Repository) GetSessionWithQuestions(ctx context.Context, examID uuid.UUID) ([]model.TestDetail, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT et.test_id
		FROM exam_test et
		WHERE et.exam_id = $1
		ORDER BY et.sort_order ASC`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var testIDs []uuid.UUID
	for rows.Next() {
		var testID uuid.UUID
		if err := rows.Scan(&testID); err != nil {
			return nil, err
		}
		testIDs = append(testIDs, testID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(testIDs) == 0 {
		return nil, nil
	}

	result := make([]model.TestDetail, len(testIDs))
	for i, tid := range testIDs {
		detail, err := r.GetTestDetail(ctx, tid)
		if err != nil {
			return nil, err
		}
		result[i] = *detail
	}
	return result, nil
}

// GetSessionAnswers returns all answers for a session ordered by question_id.
func (r *Repository) GetSessionAnswers(ctx context.Context, sessionID uuid.UUID) ([]model.ExamSessionAnswer, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT session_id, question_id, answer, is_correct, score, graded_by,
			graded_at, grader_comment, flagged_for_review, saved_at
		FROM exam_session_answer
		WHERE session_id = $1
		ORDER BY question_id`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []model.ExamSessionAnswer
	for rows.Next() {
		var a model.ExamSessionAnswer
		if err := scanExamSessionAnswer(rows, &a); err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if answers == nil {
		answers = []model.ExamSessionAnswer{}
	}
	return answers, nil
}

// SaveAnswersTx upserts answers and stamps last_saved_at on the session, in one
// transaction. The FOR UPDATE lock serializes saves against SubmitSessionTx's CAS:
// a late autosave that already passed the service's status pre-check waits on the
// submit's row lock, re-reads 'submitted', and becomes a no-op instead of
// overwriting graded rows. position is optional (FR-35): when non-nil it is
// persisted alongside the answers in the same UPDATE.
func (r *Repository) SaveAnswersTx(ctx context.Context, sessionID uuid.UUID, answers []model.ExamSessionAnswer, position *int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status string
	err = tx.QueryRow(ctx,
		`SELECT status FROM exam_session WHERE id = $1 FOR UPDATE`,
		sessionID,
	).Scan(&status)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if status != "in_progress" {
		return nil
	}

	for _, a := range answers {
		_, err := tx.Exec(ctx,
			`INSERT INTO exam_session_answer (session_id, question_id, answer, is_correct, score, graded_by, graded_at, grader_comment, flagged_for_review, saved_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
			ON CONFLICT (session_id, question_id) DO UPDATE SET
				answer = EXCLUDED.answer,
				is_correct = EXCLUDED.is_correct,
				score = EXCLUDED.score,
				graded_by = EXCLUDED.graded_by,
				graded_at = EXCLUDED.graded_at,
				grader_comment = EXCLUDED.grader_comment,
				flagged_for_review = EXCLUDED.flagged_for_review,
				saved_at = now()`,
			sessionID, a.QuestionID, a.Answer, a.IsCorrect, a.Score,
			a.GradedBy, a.GradedAt, a.GraderComment, a.FlaggedForReview,
		)
		if err != nil {
			return err
		}
	}

	if position != nil {
		_, err = tx.Exec(ctx,
			`UPDATE exam_session SET last_saved_at = now(), current_position = $2 WHERE id = $1`,
			sessionID, *position,
		)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE exam_session SET last_saved_at = now() WHERE id = $1`,
			sessionID,
		)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SubmitSessionTx performs a CAS submit of a session, writes graded answers,
// and sets the overall score. Returns the number of rows affected by the CAS update.
func (r *Repository) SubmitSessionTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, graded []model.ExamSessionAnswer, score float64, adminSubmitted bool) (int64, error) {
	query := `UPDATE exam_session SET status = 'submitted', submitted_at = now()`
	if adminSubmitted {
		query += `, admin_submitted = true`
	}
	query += ` WHERE id = $1 AND status = 'in_progress'`

	tag, err := tx.Exec(ctx, query, sessionID)
	if err != nil {
		return 0, err
	}

	if tag.RowsAffected() == 1 {
		if _, err := tx.Exec(ctx,
			`UPDATE exam_registration SET status = 'submitted'
			WHERE id = (SELECT registration_id FROM exam_session WHERE id = $1)`,
			sessionID,
		); err != nil {
			return 0, err
		}

		for _, a := range graded {
			_, err := tx.Exec(ctx,
				`INSERT INTO exam_session_answer (session_id, question_id, answer, is_correct, score, graded_by, graded_at, grader_comment, flagged_for_review, saved_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
				ON CONFLICT (session_id, question_id) DO UPDATE SET
					answer = EXCLUDED.answer,
					is_correct = EXCLUDED.is_correct,
					score = EXCLUDED.score,
					graded_by = EXCLUDED.graded_by,
					graded_at = EXCLUDED.graded_at,
					grader_comment = EXCLUDED.grader_comment,
					flagged_for_review = EXCLUDED.flagged_for_review,
					saved_at = now()`,
				sessionID, a.QuestionID, a.Answer, a.IsCorrect, a.Score,
				a.GradedBy, a.GradedAt, a.GraderComment, a.FlaggedForReview,
			)
			if err != nil {
				return 0, err
			}
		}

		_, err = tx.Exec(ctx,
			`UPDATE exam_session SET score = $1 WHERE id = $2`,
			score, sessionID,
		)
		if err != nil {
			return 0, err
		}
	}

	return tag.RowsAffected(), nil
}

// fullyGradedFilter is the shared "no ungraded essay" predicate for a submitted session,
// reused inside dedupedSubmittedSessions to keep the rank/total derivation consistent
// (FR-S5-15/18).
const fullyGradedFilter = `NOT EXISTS (
	SELECT 1 FROM exam_session_answer a
	JOIN question q ON q.id = a.question_id
	WHERE a.session_id = s.id AND q.format = 'essay' AND a.graded_at IS NULL
)`

// dedupedSubmittedSessions collapses exam_session to one row per registration_id,
// keeping the row with the greatest attempt_number — "latest attempt is authoritative,
// everywhere" (FB-26/FR22, Task 7's rule). Without this, a registration with two
// submitted attempts would double-count in CountHigherScores/CountFullyGradedSessions/
// GetFullyGradedScores and appear twice in ListExamLeaderboard. References $1 =
// exam_id; callers append further placeholders starting at $2.
const dedupedSubmittedSessions = `(
	SELECT DISTINCT ON (s.registration_id) s.id, s.registration_id, s.student_id, s.score
	FROM exam_session s
	WHERE s.exam_id = $1 AND s.status = 'submitted' AND ` + fullyGradedFilter + `
	ORDER BY s.registration_id, s.attempt_number DESC
)`

// ListSessionsNeedingGrading returns submitted sessions for an exam that still have at
// least one ungraded essay answer, joined to the student's name, with the ungraded-essay
// count per session (FR-S5-16). Single query/GROUP BY — no N+1.
func (r *Repository) ListSessionsNeedingGrading(ctx context.Context, examID uuid.UUID) ([]model.GradingSessionItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.student_id, u.name, s.submitted_at, COUNT(*) AS ungraded_count
		FROM exam_session s
		JOIN users u ON u.id = s.student_id
		JOIN exam_session_answer a ON a.session_id = s.id
		JOIN question q ON q.id = a.question_id
		WHERE s.exam_id = $1 AND s.status = 'submitted'
			AND q.format = 'essay' AND a.graded_at IS NULL
		GROUP BY s.id, s.student_id, u.name, s.submitted_at
		ORDER BY s.submitted_at ASC`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.GradingSessionItem
	for rows.Next() {
		var item model.GradingSessionItem
		if err := rows.Scan(&item.SessionID, &item.StudentID, &item.StudentName, &item.SubmittedAt, &item.UngradedEssayCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.GradingSessionItem{}
	}
	return items, nil
}

// GetSessionEssayAnswers returns each essay answer for a session joined to its question
// (body, point_correct), for the admin per-session grading read (FR-S5-17).
// Ordering uses the essay question's test_question.sort_order within the session's
// exam, falling back to question.id so the list is stable even when a question is
// no longer attached to any test in that exam.
func (r *Repository) GetSessionEssayAnswers(ctx context.Context, sessionID uuid.UUID) ([]model.GradingEssayItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.question_id, q.body, a.answer, q.point_correct, a.score, a.grader_comment, a.graded_at,
			COALESCE(tq.sort_order, 0) AS q_order
		FROM exam_session_answer a
		JOIN question q ON q.id = a.question_id
		JOIN exam_session s ON s.id = a.session_id
		LEFT JOIN LATERAL (
			SELECT tq.sort_order
			FROM exam_test et
			JOIN test_question tq ON tq.test_id = et.test_id
			WHERE et.exam_id = s.exam_id AND tq.question_id = a.question_id
			ORDER BY tq.sort_order
			LIMIT 1
		) tq ON true
		WHERE a.session_id = $1 AND q.format = 'essay'
		ORDER BY q_order, q.id`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.GradingEssayItem
	for rows.Next() {
		var item model.GradingEssayItem
		var qOrder int
		if err := rows.Scan(
			&item.QuestionID, &item.Body, &item.Answer, &item.PointCorrect, &item.Score, &item.GraderComment, &item.GradedAt, &qOrder); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.GradingEssayItem{}
	}
	return items, nil
}

// CountHigherScores counts fully-graded submitted sessions for an exam with a strictly
// higher score than the given score — the rank aggregate (FR-S5-18), one query, no N+1.
func (r *Repository) CountHigherScores(ctx context.Context, examID uuid.UUID, score float64) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM `+dedupedSubmittedSessions+` d WHERE d.score > $2`,
		examID, score,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountFullyGradedSessions counts submitted sessions for an exam with no ungraded essay
// answers — used for total_participants.
func (r *Repository) CountFullyGradedSessions(ctx context.Context, examID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM `+dedupedSubmittedSessions+` d`,
		examID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GradeEssayAnswerTx persists an admin's grade for one essay answer inside an existing
// transaction; caller recomputes and persists the session total in the same tx (FR-S5-12).
func (r *Repository) GradeEssayAnswerTx(ctx context.Context, tx pgx.Tx, sessionID, questionID uuid.UUID, score float64, comment *string, gradedBy uuid.UUID) error {
	tag, err := tx.Exec(ctx,
		`UPDATE exam_session_answer
		SET score = $1, grader_comment = $2, graded_by = $3, graded_at = now()
		WHERE session_id = $4 AND question_id = $5`,
		score, comment, gradedBy, sessionID, questionID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateSessionScoreTx persists a session's recomputed total inside an existing transaction;
// used by the essay-grading write path after GradeEssayAnswerTx (FR-S5-12/14).
func (r *Repository) UpdateSessionScoreTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, score float64) error {
	_, err := tx.Exec(ctx,
		`UPDATE exam_session SET score = $1 WHERE id = $2`,
		score, sessionID,
	)
	return err
}

// LogViolation records an integrity event for a session.
func (r *Repository) LogViolation(ctx context.Context, v model.SessionViolationLog) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO session_violation_log (session_id, student_id, violation_type, occurred_at)
		VALUES ($1, $2, $3, $4)`,
		v.SessionID, v.StudentID, v.ViolationType, v.OccurredAt,
	)
	return err
}

// ReopenSession extends a session by the given minutes. Only applies to
// in_progress or submitted sessions. Returns ErrNotFound if no session matched.
func (r *Repository) ReopenSession(ctx context.Context, sessionID uuid.UUID, minutes int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE exam_session
		SET extended_until = now() + make_interval(mins => $2)
		WHERE id = $1 AND status IN ('in_progress', 'submitted')`,
		sessionID, minutes,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetExamForSession retrieves an exam by ID. Delegates to GetExamByID.
func (r *Repository) GetExamForSession(ctx context.Context, examID uuid.UUID) (*model.Exam, error) {
	return r.GetExamByID(ctx, examID)
}

// UpdateSessionCertificate persists a certificate object key and generation timestamp for a session.
func (r *Repository) UpdateSessionCertificate(ctx context.Context, sessionID uuid.UUID, key string, generatedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exam_session SET certificate_key = $1, certificate_generated_at = $2 WHERE id = $3`,
		key, generatedAt, sessionID,
	)
	return err
}

// UpdateRegistrationCard persists a card PDF object key for a registration (FR-30).
func (r *Repository) UpdateRegistrationCard(ctx context.Context, regID uuid.UUID, key string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exam_registration SET card_key = $1 WHERE id = $2`,
		key, regID,
	)
	return err
}

// AllocateCertificateNumber assigns ABK/YYYY/<exam_number(pad4)>/<participant_number(pad6)>
// to a session's certificate_number exactly once (FR-25). The number is composed in Go —
// no global sequence is consumed — from the session's exam (exam_number, scheduled_at) and
// its registration (participant_number). A second call for an already-numbered session
// returns the existing value unchanged. A concurrent second call loses the guarding
// UPDATE (WHERE certificate_number IS NULL matches no row) and falls through to a read
// of the value the first call committed.
func (r *Repository) AllocateCertificateNumber(ctx context.Context, sessionID uuid.UUID) (string, error) {
	var existing *string
	var examNumber, participantNumber int
	var scheduledAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT s.certificate_number, e.exam_number, reg.participant_number, e.scheduled_at
		FROM exam_session s
		JOIN exam_registration reg ON reg.id = s.registration_id
		JOIN exam e ON e.id = s.exam_id
		WHERE s.id = $1`,
		sessionID,
	).Scan(&existing, &examNumber, &participantNumber, &scheduledAt)
	if err != nil {
		if isNotFound(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	if existing != nil {
		return *existing, nil
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return "", err
	}
	year := time.Now().In(loc).Year()
	if scheduledAt != nil {
		year = scheduledAt.In(loc).Year()
	}
	number := fmt.Sprintf("ABK/%04d/%04d/%06d", year, examNumber, participantNumber)

	var allocated string
	err = r.pool.QueryRow(ctx,
		`UPDATE exam_session SET certificate_number = $1
		WHERE id = $2 AND certificate_number IS NULL
		RETURNING certificate_number`,
		number, sessionID,
	).Scan(&allocated)
	if err == nil {
		return allocated, nil
	}
	if !isNotFound(err) {
		return "", err
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT certificate_number FROM exam_session WHERE id = $1`, sessionID,
	).Scan(&allocated); err != nil {
		return "", err
	}
	return allocated, nil
}

// ListExamLeaderboard returns a cursor-paginated ranked list of fully-graded submitted
// sessions for an exam, ordered by score descending with ties sharing a rank.
func (r *Repository) ListExamLeaderboard(ctx context.Context, examID uuid.UUID, cursor string, limit int) ([]model.ExamLeaderboardEntry, string, error) {
	if limit == 0 {
		limit = 20
	}

	query := `SELECT id, student_id, student_name, score, rank FROM (
		SELECT d.id, d.student_id, u.name AS student_name, d.score,
		       RANK() OVER (ORDER BY d.score DESC) AS rank
		FROM ` + dedupedSubmittedSessions + ` d
		JOIN users u ON u.id = d.student_id
	) ranked`
	args := []interface{}{examID}
	argIdx := 2

	if cursor != "" {
		scoreStr, idStr, found := strings.Cut(cursor, ",")
		if !found {
			return nil, "", fmt.Errorf("%w: %q", ErrInvalidCursor, cursor)
		}
		cursorScore, err := strconv.ParseFloat(scoreStr, 64)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrInvalidCursor, err)
		}
		cursorID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrInvalidCursor, err)
		}
		// Strictly after the last returned row under ORDER BY score DESC, id ASC —
		// a single tuple compare cannot express the mixed sort directions.
		query += fmt.Sprintf(` WHERE (ranked.score < $%d::numeric OR (ranked.score = $%d::numeric AND ranked.id > $%d::uuid))`, argIdx, argIdx, argIdx+1)
		args = append(args, cursorScore, cursorID)
		argIdx += 2
	}

	query += ` ORDER BY ranked.score DESC, ranked.id ASC LIMIT $` + fmt.Sprintf("%d", argIdx)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	entries := []model.ExamLeaderboardEntry{}
	for rows.Next() {
		var e model.ExamLeaderboardEntry
		if err := rows.Scan(&e.SessionID, &e.StudentID, &e.StudentName, &e.Score, &e.Rank); err != nil {
			return nil, "", err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(entries) > limit {
		entries = entries[:limit]
		last := entries[limit-1]
		nextCursor = strconv.FormatFloat(last.Score, 'f', -1, 64) + "," + last.SessionID.String()
	}

	return entries, nextCursor, nil
}

// GetExamCompletionStats returns total and submitted session counts for an exam.
func (r *Repository) GetExamCompletionStats(ctx context.Context, examID uuid.UUID) (total int, submitted int, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'submitted') FROM exam_session WHERE exam_id = $1`,
		examID,
	).Scan(&total, &submitted)
	if err != nil {
		return 0, 0, err
	}
	return total, submitted, nil
}

// GetFullyGradedScores returns scores for all fully-graded submitted sessions for an
// exam, one per registration (dedupedSubmittedSessions) — otherwise a student who
// retakes contributes both attempts' scores to GetExamAnalytics' average, silently
// over-weighting repeat sitters.
func (r *Repository) GetFullyGradedScores(ctx context.Context, examID uuid.UUID) ([]float64, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT d.score FROM `+dedupedSubmittedSessions+` d`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []float64
	for rows.Next() {
		var score float64
		if err := rows.Scan(&score); err != nil {
			return nil, err
		}
		scores = append(scores, score)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if scores == nil {
		scores = []float64{}
	}
	return scores, nil
}

func scanSessionMonitorRow(row interface{ Scan(dest ...any) error }, r *model.SessionMonitorRow) error {
	var schoolName, sessionStatus *string
	var sessionID *uuid.UUID
	var startedAt, extendedUntil, checkedInAt, lastSavedAt *time.Time
	var adminSubmitted bool
	var answersSaved, violationCount int
	var activeSectionTestID *uuid.UUID
	var activeSectionTitle *string
	var activeSectionStartedAt, activeSectionExtendedUntil *time.Time
	var activeSectionDurationMinutes *int
	err := row.Scan(
		&r.RegistrationID, &r.StudentID, &r.StudentName,
		&schoolName, &sessionID, &sessionStatus,
		&startedAt, &extendedUntil, &adminSubmitted,
		&checkedInAt, &lastSavedAt,
		&answersSaved, &violationCount,
		&activeSectionTestID, &activeSectionTitle, &activeSectionStartedAt,
		&activeSectionDurationMinutes, &activeSectionExtendedUntil,
	)
	if err != nil {
		return err
	}
	if schoolName != nil {
		r.SchoolName = schoolName
	}
	if sessionID != nil {
		r.SessionID = sessionID
	}
	if sessionStatus != nil {
		r.SessionStatus = sessionStatus
	}
	if startedAt != nil {
		r.StartedAt = startedAt
	}
	if extendedUntil != nil {
		r.ExtendedUntil = extendedUntil
	}
	if checkedInAt != nil {
		r.CheckedInAt = checkedInAt
	}
	if lastSavedAt != nil {
		r.LastSavedAt = lastSavedAt
	}
	r.AdminSubmitted = adminSubmitted
	r.AnswersSaved = answersSaved
	r.ViolationCount = violationCount
	r.ActiveSectionTestID = activeSectionTestID
	r.ActiveSectionTitle = activeSectionTitle
	r.ActiveSectionStartedAt = activeSectionStartedAt
	r.ActiveSectionDurationMinutes = activeSectionDurationMinutes
	r.ActiveSectionExtendedUntil = activeSectionExtendedUntil
	return nil
}

// GetSessionMonitorRows returns one registrant row per exam_registration for the given
// exam, LEFT JOINed with exam_session (max one per registration), plus the student's
// name, school, and answer/violation counts via correlated subqueries. For sectioned
// exams the active section (status='active') is LEFT JOINed so the proctor UI can show
// "which section, how long left" (FR-20/21); all Active* fields are nil for standard
// sessions or sessions with no active section.
func (r *Repository) GetSessionMonitorRows(ctx context.Context, examID uuid.UUID) ([]model.SessionMonitorRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT r.id, r.student_id, u.name, sc.name,
			s.id, s.status, s.started_at, s.extended_until,
			COALESCE(s.admin_submitted, false),
			r.checked_in_at, s.last_saved_at,
			COALESCE((SELECT COUNT(*) FROM exam_session_answer esa WHERE esa.session_id = s.id), 0),
			COALESCE((SELECT COUNT(*) FROM session_violation_log svl WHERE svl.session_id = s.id), 0),
			ss.test_id, t.title, ss.started_at, ss.duration_minutes, ss.extended_until
		FROM exam_registration r
		JOIN users u ON u.id = r.student_id
		LEFT JOIN school sc ON sc.id = u.school_id
		LEFT JOIN exam_session s ON s.registration_id = r.id
		LEFT JOIN exam_session_section ss ON ss.session_id = s.id AND ss.status = 'active'
		LEFT JOIN test t ON t.id = ss.test_id
		WHERE r.exam_id = $1
		ORDER BY r.created_at DESC`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.SessionMonitorRow
	for rows.Next() {
		var item model.SessionMonitorRow
		if err := scanSessionMonitorRow(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.SessionMonitorRow{}
	}
	return items, nil
}

// GetExamQuestionTotal returns the total number of questions across all tests attached
// to an exam.
func (r *Repository) GetExamQuestionTotal(ctx context.Context, examID uuid.UUID) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*)
		FROM exam_test et
		JOIN test_question tq ON tq.test_id = et.test_id
		WHERE et.exam_id = $1`,
		examID,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// GetRecentViolations returns per-session violation aggregates for an exam, newest-first,
// capped at the given limit. Each entry includes the session's total count and the most
// recent violation type and timestamp.
func (r *Repository) GetRecentViolations(ctx context.Context, examID uuid.UUID, limit int) ([]model.ViolationRecent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT session_id, student_name, count, latest_type, latest_occurred_at
		FROM (
			SELECT s.id AS session_id, u.name AS student_name,
				COUNT(*) OVER (PARTITION BY s.id) AS count,
				svl.violation_type AS latest_type,
				svl.occurred_at AS latest_occurred_at,
				ROW_NUMBER() OVER (PARTITION BY s.id ORDER BY svl.occurred_at DESC) AS rn
			FROM session_violation_log svl
			JOIN exam_session s ON s.id = svl.session_id
			JOIN users u ON u.id = s.student_id
			WHERE s.exam_id = $1
		) ranked
		WHERE rn = 1
		ORDER BY latest_occurred_at DESC
		LIMIT $2`,
		examID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ViolationRecent
	for rows.Next() {
		var item model.ViolationRecent
		if err := rows.Scan(
			&item.SessionID, &item.StudentName,
			&item.Count, &item.LatestType, &item.LatestOccurredAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.ViolationRecent{}
	}
	return items, nil
}

// ListSessionViolations returns all violation log rows for a session, newest-first.
func (r *Repository) ListSessionViolations(ctx context.Context, sessionID uuid.UUID) ([]model.SessionViolationLog, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, session_id, student_id, violation_type, occurred_at
		FROM session_violation_log
		WHERE session_id = $1
		ORDER BY occurred_at DESC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.SessionViolationLog
	for rows.Next() {
		var item model.SessionViolationLog
		if err := rows.Scan(
			&item.ID, &item.SessionID, &item.StudentID,
			&item.ViolationType, &item.OccurredAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.SessionViolationLog{}
	}
	return items, nil
}

// ---------- Sectioned-exam section rows (FR-3 / FR-5 / FR-10 / FR-22) ----------

func scanExamSessionSection(row interface{ Scan(dest ...any) error }, s *model.ExamSessionSection) error {
	return row.Scan(
		&s.SessionID, &s.TestID, &s.SortOrder, &s.DurationMinutes,
		&s.Status, &s.StartedAt, &s.SubmittedAt, &s.ExtendedUntil,
	)
}

// CreateSessionSectionsTx inserts the per-section timing rows for a sectioned exam
// session inside the caller's transaction (FR-5). The caller (service) decides each
// row's status/started_at — typically the lowest sort_order is 'active' with
// started_at=now() and the rest are 'pending'. No business rules here.
func (r *Repository) CreateSessionSectionsTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, sections []model.ExamSessionSection) error {
	for _, s := range sections {
		s.SessionID = sessionID
		if _, err := tx.Exec(ctx,
			`INSERT INTO exam_session_section
				(session_id, test_id, sort_order, duration_minutes, status, started_at, submitted_at, extended_until)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			s.SessionID, s.TestID, s.SortOrder, s.DurationMinutes,
			s.Status, s.StartedAt, s.SubmittedAt, s.ExtendedUntil,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetSessionSections returns all section rows for a session ordered by sort_order (FR-16).
func (r *Repository) GetSessionSections(ctx context.Context, sessionID uuid.UUID) ([]model.ExamSessionSection, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT session_id, test_id, sort_order, duration_minutes, status, started_at, submitted_at, extended_until
		FROM exam_session_section
		WHERE session_id = $1
		ORDER BY sort_order ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ExamSessionSection
	for rows.Next() {
		var s model.ExamSessionSection
		if err := scanExamSessionSection(rows, &s); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.ExamSessionSection{}
	}
	return items, nil
}

// AdvanceSessionSectionTx performs the atomic guarded advance of a sectioned exam
// (FR-10, FR-11, NFR-5). The WHERE status='active' guard is the point: a double-fire
// or wrong-section call affects 0 rows and surfaces as ErrNoActiveSection so the
// service (Task 3) can decide idempotent-200 vs ErrSectionNotActive. On success it
// flips the active section to 'submitted' (stamping submitted_at), promotes the next
// 'pending' row by lowest sort_order to 'active' (stamping started_at=now()), and
// returns the activated next test_id (nil when advancing the last section — FR-12).
func (r *Repository) AdvanceSessionSectionTx(ctx context.Context, tx pgx.Tx, sessionID, testID uuid.UUID) (*uuid.UUID, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE exam_session_section
		SET status = 'submitted', submitted_at = now()
		WHERE session_id = $1 AND test_id = $2 AND status = 'active'`,
		sessionID, testID,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNoActiveSection
	}

	var nextTestID *uuid.UUID
	err = tx.QueryRow(ctx,
		`WITH next AS (
			SELECT test_id FROM exam_session_section
			WHERE session_id = $1 AND status = 'pending'
			ORDER BY sort_order ASC
			LIMIT 1
		)
		UPDATE exam_session_section s
		SET status = 'active', started_at = now()
		FROM next
		WHERE s.session_id = $1 AND s.test_id = next.test_id
		RETURNING s.test_id`,
		sessionID,
	).Scan(&nextTestID)
	if err != nil {
		if isNotFound(err) {
			// No pending section left — advancing the last section (FR-12).
			return nil, nil
		}
		return nil, err
	}
	return nextTestID, nil
}

// ExtendActiveSectionTx pushes the active section's extended_until forward by the
// given minutes (FR-22 reopen). Returns ErrNoActiveSection when no row is active.
func (r *Repository) ExtendActiveSectionTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, extendMinutes int) error {
	tag, err := tx.Exec(ctx,
		`UPDATE exam_session_section
		SET extended_until = now() + make_interval(mins => $2)
		WHERE session_id = $1 AND status = 'active'`,
		sessionID, extendMinutes,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoActiveSection
	}
	return nil
}
