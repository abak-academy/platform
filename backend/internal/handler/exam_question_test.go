package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
