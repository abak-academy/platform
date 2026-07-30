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
	svc *service.Service
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
		questionDBEnv = &questionHandlerTestEnv{svc: svc}
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
