package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestQuestionRequest_toQuestion_defaultsPoints(t *testing.T) {
	req := questionRequest{Format: "essay", Body: "explain gravity"}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("toQuestion returned error: %v", err)
	}
	if q.PointCorrect != 1 {
		t.Errorf("PointCorrect default = %v, want 1", q.PointCorrect)
	}
	if q.PointWrong != 0 {
		t.Errorf("PointWrong default = %v, want 0", q.PointWrong)
	}
}

func TestQuestionRequest_toQuestion_appliesExplicitPoints(t *testing.T) {
	pc, pw := 3.0, 2.0
	req := questionRequest{Format: "essay", Body: "explain gravity", PointCorrect: &pc, PointWrong: &pw}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("toQuestion returned error: %v", err)
	}
	if q.PointCorrect != 3 {
		t.Errorf("PointCorrect = %v, want 3", q.PointCorrect)
	}
	if q.PointWrong != 2 {
		t.Errorf("PointWrong = %v, want 2", q.PointWrong)
	}
}

// FR-16/FR-18: the handler must not coerce fractional points to int — that guard
// used to reproduce the bug even after the service/DB guards were fixed.
func TestQuestionRequest_toQuestion_acceptsFractionalPointCorrect(t *testing.T) {
	pc := 2.5
	req := questionRequest{Format: "essay", Body: "explain gravity", PointCorrect: &pc}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("fractional point_correct should be accepted, got error: %v", err)
	}
	if q.PointCorrect != 2.5 {
		t.Errorf("PointCorrect = %v, want 2.5", q.PointCorrect)
	}
}

func TestQuestionRequest_toQuestion_acceptsFractionalPointWrong(t *testing.T) {
	pw := 0.5
	req := questionRequest{Format: "essay", Body: "explain gravity", PointWrong: &pw}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("fractional point_wrong should be accepted, got error: %v", err)
	}
	if q.PointWrong != 0.5 {
		t.Errorf("PointWrong = %v, want 0.5", q.PointWrong)
	}
}

func TestQuestionRequest_toQuestion_parsesTopicID(t *testing.T) {
	topicID := "11111111-1111-1111-1111-111111111111"
	req := questionRequest{Format: "essay", Body: "explain", TopicID: &topicID}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("toQuestion returned error: %v", err)
	}
	if q.TopicID == nil {
		t.Fatal("expected TopicID to be set")
	}
	if q.TopicID.String() != topicID {
		t.Errorf("TopicID = %s, want %s", q.TopicID.String(), topicID)
	}
}

func TestQuestionRequest_toQuestion_ignoresEmptyTopicID(t *testing.T) {
	empty := ""
	req := questionRequest{Format: "essay", Body: "explain", TopicID: &empty}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("toQuestion returned error: %v", err)
	}
	if q.TopicID != nil {
		t.Errorf("expected nil TopicID for empty string, got %s", q.TopicID.String())
	}
}

func TestQuestionRequest_toQuestion_rejectsInvalidTopicID(t *testing.T) {
	bad := "not-a-uuid"
	req := questionRequest{Format: "essay", Body: "explain", TopicID: &bad}
	_, err := req.toQuestion()
	if err == nil {
		t.Fatal("invalid topic_id should return error")
	}
	if !errors.Is(err, service.ErrValidation) {
		t.Errorf("invalid topic_id should return ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "topic_id is not a valid UUID") {
		t.Errorf("msg should mention 'topic_id is not a valid UUID', got %q", err.Error())
	}
}

// FR-25/FR-26: accepted_answers travels from the request body through
// toQuestion()/toBlanks() unchanged.
func TestQuestionRequest_toQuestion_carriesAcceptedAnswers(t *testing.T) {
	req := questionRequest{Format: "short", Body: "1+1", AcceptedAnswers: []string{"2", "dua"}}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("toQuestion returned error: %v", err)
	}
	if len(q.AcceptedAnswers) != 2 || q.AcceptedAnswers[0] != "2" || q.AcceptedAnswers[1] != "dua" {
		t.Errorf("AcceptedAnswers = %v, want [2 dua]", q.AcceptedAnswers)
	}
}

func TestQuestionRequest_toQuestion_defaultsAcceptedAnswersToEmptySlice(t *testing.T) {
	req := questionRequest{Format: "essay", Body: "explain"}
	q, err := req.toQuestion()
	if err != nil {
		t.Fatalf("toQuestion returned error: %v", err)
	}
	if q.AcceptedAnswers == nil {
		t.Error("AcceptedAnswers should default to a non-nil empty slice")
	}
	if len(q.AcceptedAnswers) != 0 {
		t.Errorf("AcceptedAnswers = %v, want empty", q.AcceptedAnswers)
	}
}

func TestQuestionRequest_toBlanks_carriesAcceptedAnswers(t *testing.T) {
	req := questionRequest{
		Format: "multi_blank",
		Body:   "{{1}} and {{2}}",
		Blanks: []blankRequest{
			{Index: 1, CorrectAnswer: "4", AcceptedAnswers: []string{"4", "empat"}},
			{Index: 2, CorrectAnswer: "jakarta"},
		},
	}
	blanks := req.toBlanks()
	if len(blanks) != 2 {
		t.Fatalf("toBlanks returned %d blanks, want 2", len(blanks))
	}
	if len(blanks[0].AcceptedAnswers) != 2 || blanks[0].AcceptedAnswers[0] != "4" || blanks[0].AcceptedAnswers[1] != "empat" {
		t.Errorf("blanks[0].AcceptedAnswers = %v, want [4 empat]", blanks[0].AcceptedAnswers)
	}
	if blanks[1].AcceptedAnswers == nil || len(blanks[1].AcceptedAnswers) != 0 {
		t.Errorf("blanks[1].AcceptedAnswers = %v, want non-nil empty slice", blanks[1].AcceptedAnswers)
	}
}

// ---------------------------------------------------------------------------
// DB-backed: FR-16/FR-18 must hold at the handler layer, not only in
// toQuestion() unit tests above. This is the exact guard that used to
// reproduce the bug even with the service and DB guards fixed.
// ---------------------------------------------------------------------------

var (
	questionDBOnce sync.Once
	questionDBEnv  *questionHandlerTestEnv
)

type questionHandlerTestEnv struct {
	svc  *service.Service
	repo *repository.Repository
}

func newQuestionHandlerEnv(t *testing.T) *questionHandlerTestEnv {
	t.Helper()
	questionDBOnce.Do(func() {
		ctx := context.Background()
		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_question_handler_test"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatalf("connection string: %v", err)
		}
		if err := infra.RunMigrations(ctx, dsn); err != nil {
			t.Fatalf("run migrations: %v", err)
		}
		pool, err := infra.NewPool(ctx, dsn)
		if err != nil {
			t.Fatalf("new pool: %v", err)
		}
		repo := repository.New(pool)
		svc := service.NewWithStore(repo, repo, nil, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{}, nil, nil, nil, nil)
		questionDBEnv = &questionHandlerTestEnv{svc: svc, repo: repo}
	})
	if questionDBEnv == nil {
		t.Fatal("question handler test env failed to initialize")
	}
	return questionDBEnv
}

func TestAdminCreateBankQuestion_acceptsFractionalPointCorrect(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)

	body := []byte(`{"format":"essay","body":"explain gravity","point_correct":2.5}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)

	if err := h.AdminCreateBankQuestion(c); err != nil {
		t.Fatalf("AdminCreateBankQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Question struct {
			PointCorrect float64 `json:"point_correct"`
		} `json:"question"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Question.PointCorrect != 2.5 {
		t.Errorf("response point_correct = %v, want 2.5", resp.Question.PointCorrect)
	}
}

// FR-2: a freshly created question's JSON carries a non-zero question_number
// from the column DEFAULT.
func TestAdminCreateBankQuestion_returnsNonZeroQuestionNumber(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)

	body := []byte(`{"format":"essay","body":"explain the tides"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)

	if err := h.AdminCreateBankQuestion(c); err != nil {
		t.Fatalf("AdminCreateBankQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Question struct {
			QuestionNumber int `json:"question_number"`
		} `json:"question"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Question.QuestionNumber == 0 {
		t.Errorf("response question_number = 0, want a non-zero value assigned by the sequence DEFAULT")
	}
}

// FR-4: an unparseable cursor must be rejected, never silently ignored.
func TestAdminListBankQuestions_invalidCursorReturns400(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/questions?cursor=not-a-number", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)

	if err := h.AdminListBankQuestions(c); err != nil {
		t.Fatalf("AdminListBankQuestions returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "invalid_request" {
		t.Errorf("response code = %q, want %q", resp.Code, "invalid_request")
	}
}

// FR-4: a valid integer cursor is honoured -- it must exclude every question at
// or above that question_number from the page.
func TestAdminListBankQuestions_honoursIntegerCursor(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)
	e := echo.New()

	create := func(body string) int {
		reqBody := []byte(`{"format":"essay","body":"` + body + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/questions", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := h.AdminCreateBankQuestion(c); err != nil {
			t.Fatalf("AdminCreateBankQuestion returned error: %v", err)
		}
		var resp struct {
			Question struct {
				QuestionNumber int `json:"question_number"`
			} `json:"question"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp.Question.QuestionNumber
	}

	first := create("cursor question one")
	second := create("cursor question two")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/questions?cursor=%d", second), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.AdminListBankQuestions(c); err != nil {
		t.Fatalf("AdminListBankQuestions returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			Question struct {
				QuestionNumber int `json:"question_number"`
			} `json:"question"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, item := range resp.Data {
		if item.Question.QuestionNumber >= second {
			t.Errorf("page after cursor=%d must not include question_number %d", second, item.Question.QuestionNumber)
		}
	}
	found := false
	for _, item := range resp.Data {
		if item.Question.QuestionNumber == first {
			found = true
		}
	}
	if !found {
		t.Errorf("page after cursor=%d must include question_number %d (first)", second, first)
	}
}

// FR-10/FR-11: the download endpoint must return the CSV template as a named
// attachment.
func TestAdminGetQuestionImportTemplate_returnsCSVAttachment(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/questions/import-template", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)

	if err := h.AdminGetQuestionImportTemplate(c); err != nil {
		t.Fatalf("AdminGetQuestionImportTemplate returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type: want text/csv, got %q", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if cd != `attachment; filename="question_import_template.csv"` {
		t.Errorf("Content-Disposition: want attachment; filename=\"question_import_template.csv\", got %q", cd)
	}
}

// ---------------------------------------------------------------------------
// Task 6 (FB-3 + FB-9): delete guard and format lock, exercised end to end
// through the handler so the 409 codes the frontend reads are actually
// produced by the server, not just the underlying sentinels.
// ---------------------------------------------------------------------------

func seedTestRowDirect(t *testing.T, ctx context.Context, repo *repository.Repository, title, subject, topic string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, $2, $3, $4) RETURNING id`,
		title, subject, topic, 60,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed test: %v", err)
	}
	return id
}

func seedBankQuestionRowDirect(t *testing.T, ctx context.Context, repo *repository.Repository, format, body string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ($1, $2, 1, 0) RETURNING id`,
		format, body,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed question: %v", err)
	}
	return id
}

func attachQuestionRowDirect(t *testing.T, ctx context.Context, repo *repository.Repository, testID, questionID uuid.UUID) {
	t.Helper()
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`,
		testID, questionID,
	)
	if err != nil {
		t.Fatalf("attach question: %v", err)
	}
}

// seedExamRowDirect creates an exam (status is always 'draft' — see the task's
// note that "live" is never read off exam.status) and attaches testID to it.
func seedExamRowDirect(t *testing.T, ctx context.Context, repo *repository.Repository, testID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	var examID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ($1, 'draft') RETURNING id`,
		title,
	).Scan(&examID)
	if err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		examID, testID,
	)
	if err != nil {
		t.Fatalf("attach exam_test: %v", err)
	}
	return examID
}

func seedProductForExamRowDirect(t *testing.T, ctx context.Context, repo *repository.Repository, examID uuid.UUID, status string) {
	t.Helper()
	var productID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, status) VALUES ('exam', $1, 100000, $2) RETURNING id`,
		"Product "+uuid.NewString(), status,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`,
		productID, examID,
	)
	if err != nil {
		t.Fatalf("link product_exam: %v", err)
	}
}

func seedExamSessionRowDirect(t *testing.T, ctx context.Context, repo *repository.Repository, examID uuid.UUID) {
	t.Helper()
	var studentID uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (email, name, role, status) VALUES ($1, 'Student', 'student', 'active') RETURNING id`,
		"student-"+uuid.NewString()+"@example.test",
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("seed student: %v", err)
	}
	var regID uuid.UUID
	err = repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status) VALUES ($1, $2, $3, 'registered') RETURNING id`,
		studentID, examID, "TOKEN-"+uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("seed registration: %v", err)
	}
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status) VALUES ($1, $2, $3, 1, now(), 'in_progress')`,
		regID, studentID, examID,
	)
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}
}

func questionExistsInTable(t *testing.T, ctx context.Context, repo *repository.Repository, table string, questionID uuid.UUID) bool {
	t.Helper()
	col := "question_id"
	if table == "question" {
		col = "id"
	}
	var exists bool
	if err := repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE `+col+` = $1)`, questionID).Scan(&exists); err != nil {
		t.Fatalf("check %s: %v", table, err)
	}
	return exists
}

// FR-7: the full chain question -> test -> exam -> published product must
// refuse the delete with 409 question_in_published_exam, and the row survives.
func TestAdminDeleteQuestion_refused_examHasPublishedProduct(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)
	ctx := context.Background()

	testID := seedTestRowDirect(t, ctx, env.repo, "Math "+uuid.NewString(), "math", "algebra")
	qID := seedBankQuestionRowDirect(t, ctx, env.repo, "essay", "fb3 published product chain")
	attachQuestionRowDirect(t, ctx, env.repo, testID, qID)
	examID := seedExamRowDirect(t, ctx, env.repo, testID, "Exam FB3 published "+uuid.NewString())
	seedProductForExamRowDirect(t, ctx, env.repo, examID, "published")

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/questions/"+qID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(qID.String())

	if err := h.AdminDeleteQuestion(c); err != nil {
		t.Fatalf("AdminDeleteQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "question_in_published_exam" {
		t.Errorf("code = %q, want question_in_published_exam", resp.Code)
	}

	if !questionExistsInTable(t, ctx, env.repo, "question", qID) {
		t.Error("question must survive a refused delete")
	}
}

// FR-6: the same chain with the product left draft and no sessions must
// succeed with 204, and every dependent row (options, blanks, accepted
// answers, statements, test_question) must be gone via ON DELETE CASCADE.
func TestAdminDeleteQuestion_succeeds_examDraftNoSessions_cascadesFully(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)
	ctx := context.Background()

	testID := seedTestRowDirect(t, ctx, env.repo, "Math "+uuid.NewString(), "math", "algebra")
	qID := seedBankQuestionRowDirect(t, ctx, env.repo, "essay", "fb3 draft product cascade")
	attachQuestionRowDirect(t, ctx, env.repo, testID, qID)
	examID := seedExamRowDirect(t, ctx, env.repo, testID, "Exam FB3 draft "+uuid.NewString())
	seedProductForExamRowDirect(t, ctx, env.repo, examID, "draft")

	// Seed one row in every child table to prove the cascade reaches all of
	// them, regardless of whether this shape is realistic for an "essay" format.
	if _, err := env.repo.Pool().Exec(ctx, `INSERT INTO question_option (question_id, key, text, sort_order) VALUES ($1, 'a', 'opt', 1)`, qID); err != nil {
		t.Fatalf("seed option: %v", err)
	}
	if _, err := env.repo.Pool().Exec(ctx, `INSERT INTO question_blank (question_id, blank_index, correct_answer) VALUES ($1, 1, 'ans')`, qID); err != nil {
		t.Fatalf("seed blank: %v", err)
	}
	if _, err := env.repo.Pool().Exec(ctx, `INSERT INTO question_accepted_answer (question_id, blank_index, answer_index, answer) VALUES ($1, 0, 1, 'ans')`, qID); err != nil {
		t.Fatalf("seed accepted answer: %v", err)
	}
	if _, err := env.repo.Pool().Exec(ctx, `INSERT INTO question_statement (question_id, statement_index, body, is_true) VALUES ($1, 1, 'stmt', true)`, qID); err != nil {
		t.Fatalf("seed statement: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/questions/"+qID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(qID.String())

	if err := h.AdminDeleteQuestion(c); err != nil {
		t.Fatalf("AdminDeleteQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body.String())
	}

	for _, table := range []string{"question", "question_option", "question_blank", "question_accepted_answer", "question_statement", "test_question"} {
		if questionExistsInTable(t, ctx, env.repo, table, qID) {
			t.Errorf("%s row should be gone after delete", table)
		}
	}
}

// The predicate's second arm: an otherwise-draft exam (no product at all) that
// already has an exam_session row must still refuse the delete.
func TestAdminDeleteQuestion_refused_examHasSession(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)
	ctx := context.Background()

	testID := seedTestRowDirect(t, ctx, env.repo, "Math "+uuid.NewString(), "math", "algebra")
	qID := seedBankQuestionRowDirect(t, ctx, env.repo, "essay", "fb3 session arm")
	attachQuestionRowDirect(t, ctx, env.repo, testID, qID)
	examID := seedExamRowDirect(t, ctx, env.repo, testID, "Exam FB3 session "+uuid.NewString())
	seedExamSessionRowDirect(t, ctx, env.repo, examID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/questions/"+qID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(qID.String())

	if err := h.AdminDeleteQuestion(c); err != nil {
		t.Fatalf("AdminDeleteQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "question_in_published_exam" {
		t.Errorf("code = %q, want question_in_published_exam", resp.Code)
	}
}

// FR-13/FR-14: a format change on a live-exam question is refused with 409
// question_format_locked and writes nothing; the same request changing only
// body succeeds with 200.
func TestAdminUpdateQuestion_formatChangeOnLiveExamQuestion_refused(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)
	ctx := context.Background()

	testID := seedTestRowDirect(t, ctx, env.repo, "Math "+uuid.NewString(), "math", "algebra")
	qID := seedBankQuestionRowDirect(t, ctx, env.repo, "mcq", "original mcq body")
	attachQuestionRowDirect(t, ctx, env.repo, testID, qID)
	examID := seedExamRowDirect(t, ctx, env.repo, testID, "Exam format lock "+uuid.NewString())
	seedProductForExamRowDirect(t, ctx, env.repo, examID, "published")

	e := echo.New()
	body := []byte(`{"format":"short","body":"changed body","accepted_answers":["x"]}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/questions/"+qID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(qID.String())

	if err := h.AdminUpdateQuestion(c); err != nil {
		t.Fatalf("AdminUpdateQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "question_format_locked" {
		t.Errorf("code = %q, want question_format_locked", resp.Code)
	}

	var storedFormat, storedBody string
	if err := env.repo.Pool().QueryRow(ctx, `SELECT format, body FROM question WHERE id=$1`, qID).Scan(&storedFormat, &storedBody); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if storedFormat != "mcq" || storedBody != "original mcq body" {
		t.Errorf("refused format change must write nothing, got format=%q body=%q", storedFormat, storedBody)
	}
}

func TestAdminUpdateQuestion_bodyOnlyOnLiveExamQuestion_succeeds(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)
	ctx := context.Background()

	testID := seedTestRowDirect(t, ctx, env.repo, "Math "+uuid.NewString(), "math", "algebra")
	qID := seedBankQuestionRowDirect(t, ctx, env.repo, "essay", "original essay body")
	attachQuestionRowDirect(t, ctx, env.repo, testID, qID)
	examID := seedExamRowDirect(t, ctx, env.repo, testID, "Exam body edit "+uuid.NewString())
	seedProductForExamRowDirect(t, ctx, env.repo, examID, "published")

	e := echo.New()
	body := []byte(`{"format":"essay","body":"updated essay body"}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/questions/"+qID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(qID.String())

	if err := h.AdminUpdateQuestion(c); err != nil {
		t.Fatalf("AdminUpdateQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var storedBody string
	if err := env.repo.Pool().QueryRow(ctx, `SELECT body FROM question WHERE id=$1`, qID).Scan(&storedBody); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if storedBody != "updated essay body" {
		t.Errorf("body = %q, want %q", storedBody, "updated essay body")
	}
}

// FR-15: a question not in a live exam is free to change format, validated
// against the new format's own rules.
func TestAdminUpdateQuestion_formatChangeOnNonLiveQuestion_succeeds(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)
	ctx := context.Background()

	qID := seedBankQuestionRowDirect(t, ctx, env.repo, "essay", "not attached to anything "+uuid.NewString())

	e := echo.New()
	body := []byte(`{"format":"short","body":"changed to short format","accepted_answers":["42"]}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/questions/"+qID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(qID.String())

	if err := h.AdminUpdateQuestion(c); err != nil {
		t.Fatalf("AdminUpdateQuestion returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var storedFormat string
	if err := env.repo.Pool().QueryRow(ctx, `SELECT format FROM question WHERE id=$1`, qID).Scan(&storedFormat); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if storedFormat != "short" {
		t.Errorf("format = %q, want short", storedFormat)
	}
}

// page selects offset pagination; a bad value must be rejected like a bad cursor.
func TestAdminListBankQuestions_invalidPageReturns400(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)

	for _, bad := range []string{"0", "-1", "x"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/questions?page="+bad, nil)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		if err := h.AdminListBankQuestions(c); err != nil {
			t.Fatalf("page=%s: returned error: %v", bad, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("page=%s: want 400, got %d: %s", bad, rec.Code, rec.Body.String())
		}
	}
}

// The list response carries total at the top level — the numbered-pagination UI
// computes its page count from it, so the key going missing or moving breaks
// pages silently (fixtures-invent-the-payload failure shape).
func TestAdminListBankQuestions_pageModeCarriesTotal(t *testing.T) {
	env := newQuestionHandlerEnv(t)
	h := New(env.svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/questions?page=1&limit=1", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if err := h.AdminListBankQuestions(c); err != nil {
		t.Fatalf("AdminListBankQuestions returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data  []json.RawMessage `json:"data"`
		Total *int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total == nil {
		t.Fatal("response has no top-level total")
	}
	if len(resp.Data) > 1 {
		t.Fatalf("limit=1 returned %d rows", len(resp.Data))
	}
	if *resp.Total < len(resp.Data) {
		t.Fatalf("total %d < returned rows %d", *resp.Total, len(resp.Data))
	}
}
