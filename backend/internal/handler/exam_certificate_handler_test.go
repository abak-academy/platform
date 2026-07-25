package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// ---------------------------------------------------------------------------
// Route registration helpers for simple middleware-only tests (fast, no DB)
// ---------------------------------------------------------------------------

// registerAdminExamRoutes adds the three admin endpoints under /api/v1/admin,
// protected by JWTMiddleware + RBACMiddleware("products(exam):write").
func registerAdminExamRoutes(t *testing.T, env *testEnv, h *handler.Handler) {
	t.Helper()
	v1 := env.e.Group("/api/v1")
	admin := v1.Group("/admin")
	admin.Use(handler.JWTMiddleware(env.svc, env.signer))
	adminExams := admin.Group("/exams")
	adminExams.Use(handler.RBACMiddleware("products(exam):write"))
	adminExams.GET("/:id/leaderboard", h.AdminGetExamLeaderboard)
	adminExams.GET("/:id/analytics", h.AdminGetExamAnalytics)
	adminExams.GET("/:id/certificate-preview", h.AdminGetExamCertificatePreview)
	adminExams.PATCH("/:id", h.AdminUpdateExam)
}

// registerStudentLeaderboardRoute adds the student leaderboard endpoint under
// /api/v1/exam, protected by JWTMiddleware only (no RBAC).
func registerStudentLeaderboardRoute(t *testing.T, env *testEnv, h *handler.Handler) {
	t.Helper()
	v1 := env.e.Group("/api/v1")
	exam := v1.Group("/exam")
	exam.Use(handler.JWTMiddleware(env.svc, env.signer))
	exam.GET("/sessions/:id/leaderboard", h.StudentGetSessionLeaderboard)
}

// ---------------------------------------------------------------------------
// DB-backed test environment (testcontainers Postgres)
// ---------------------------------------------------------------------------

type testEnvWithStore struct {
	pool   *pgxpool.Pool
	mr     *miniredis.Miniredis
	e      *echo.Echo
	svc    *service.Service
	signer *infra.JWTSigner
}

func newTestEnvWithStore(t *testing.T) *testEnvWithStore {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_handler_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := infra.RunMigrations(ctx, dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	store := repository.New(pool)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cfg := &config.Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
	}
	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)

	svc := service.NewWithStore(
		store, store, rdb, signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{},
		nil, cfg,
	)

	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, h, svc, signer)

	return &testEnvWithStore{pool: pool, mr: mr, e: e, svc: svc, signer: signer}
}

// mintTokenForEnv creates a signed JWT and stores the session in the env's
// miniredis so JWTMiddleware will accept it.
func mintTokenForEnv(t *testing.T, env *testEnvWithStore, userID, role string) string {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	tokenString, jti, err := env.signer.SignAccess(userID, role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(context.Background(), "session:access:"+jti, userID, 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}
	return tokenString
}

// getRequest issues a GET with optional Bearer token.
func getRequest(t *testing.T, e *echo.Echo, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// patchJSONRequest issues a PATCH with JSON body.
func patchJSONRequest(t *testing.T, e *echo.Echo, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// Seed helpers for DB-backed tests
// ---------------------------------------------------------------------------

func seedUser(t *testing.T, pool *pgxpool.Pool, role, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	email := fmt.Sprintf("%s-%s@test.local", role, uuid.NewString())
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		email, role, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func seedTest(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ($1, $2, $3, 60) RETURNING id`,
		"Handler Test", "math", "algebra",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert test: %v", err)
	}
	return id
}

func seedMCQuestion(t *testing.T, pool *pgxpool.Pool, testID uuid.UUID, body string, pointCorrect, sortOrder int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO question (format, body, point_correct, point_wrong)
		VALUES ('mcq', $1, $2, 0) RETURNING id`,
		body, pointCorrect,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert mcq: %v", err)
	}
	_, err = pool.Exec(context.Background(),
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`,
		testID, id, sortOrder,
	)
	if err != nil {
		t.Fatalf("insert test_question: %v", err)
	}
	// Insert options (2 options, first correct)
	for i, o := range []struct {
		key, text string
		correct   bool
	}{
		{"a", "Correct answer", true},
		{"b", "Wrong answer", false},
	} {
		_, err := pool.Exec(context.Background(),
			`INSERT INTO question_option (question_id, key, text, is_correct, sort_order) VALUES ($1, $2, $3, $4, $5)`,
			id, o.key, o.text, o.correct, i+1,
		)
		if err != nil {
			t.Fatalf("insert option: %v", err)
		}
	}
	return id
}

func seedExam(t *testing.T, pool *pgxpool.Pool, title string, allowLeaderboard bool, resultConfig string, certificateTemplate string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if resultConfig == "" {
		resultConfig = "hidden"
	}
	if certificateTemplate == "" {
		certificateTemplate = "classic"
	}
	err := pool.QueryRow(context.Background(),
		`INSERT INTO exam (title, allow_leaderboard, result_config, certificate_template, timer_mode, duration_minutes)
		VALUES ($1, $2, $3, $4, 'overall', 60) RETURNING id`,
		title, allowLeaderboard, resultConfig, certificateTemplate,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	return id
}

func seedExamTest(t *testing.T, pool *pgxpool.Pool, examID, testID uuid.UUID, sortOrder int) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`,
		examID, testID, sortOrder,
	)
	if err != nil {
		t.Fatalf("insert exam_test: %v", err)
	}
}

func seedRegistration(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert registration: %v", err)
	}
	return id
}

func seedSession(t *testing.T, pool *pgxpool.Pool, registrationID, studentID, examID uuid.UUID, status string, score float64, submittedAt *time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status, submitted_at, score)
		VALUES ($1, $2, $3, now(), $4, $5, $6) RETURNING id`,
		registrationID, studentID, examID, status, submittedAt, score,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return id
}

func seedAnswer(t *testing.T, pool *pgxpool.Pool, sessionID, questionID uuid.UUID, answer string, score float64) {
	t.Helper()
	now := time.Now()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO exam_session_answer (session_id, question_id, answer, is_correct, score, graded_at, saved_at)
		VALUES ($1, $2, $3, true, $4, $5, $5)`,
		sessionID, questionID, answer, score, now,
	)
	if err != nil {
		t.Fatalf("insert answer: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AdminGetExamLeaderboard tests
// ---------------------------------------------------------------------------

func TestAdminGetExamLeaderboard_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/leaderboard", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamLeaderboard_StudentToken_Returns403(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "student-leaderboard",
		Email:  strptr("student-lb@test.com"),
		Role:   service.RoleStudent,
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	token := mintToken(t, env, "student-leaderboard", service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/leaderboard", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("code: want forbidden, got %v", resp["code"])
	}
}

func TestAdminGetExamLeaderboard_AdminToken_Returns200(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Leaderboard")
	student := seedUser(t, env.pool, "student", "Student LB")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "Leaderboard Exam", true, "score_only", "classic")
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 90, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/leaderboard", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", resp["data"])
	}
	if len(data) != 1 {
		t.Fatalf("want 1 leaderboard entry, got %d", len(data))
	}
	entry := data[0].(map[string]any)
	if entry["rank"] != float64(1) {
		t.Errorf("rank: want 1, got %v", entry["rank"])
	}
	if entry["score"] != float64(90) {
		t.Errorf("score: want 90, got %v", entry["score"])
	}
}

func TestAdminGetExamLeaderboard_MalformedCursor_Returns422(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Bad Cursor")

	examID := seedExam(t, env.pool, "Bad Cursor Exam", true, "score_only", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/leaderboard?cursor=90,notauuid", token)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "validation_failed" {
		t.Errorf("code: want validation_failed, got %v", resp["code"])
	}
}

// ---------------------------------------------------------------------------
// AdminGetExamAnalytics tests
// ---------------------------------------------------------------------------

func TestAdminGetExamAnalytics_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/analytics", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamAnalytics_StudentToken_Returns403(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "student-analytics",
		Email:  strptr("student-analytics@test.com"),
		Role:   service.RoleStudent,
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	token := mintToken(t, env, "student-analytics", service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/analytics", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamAnalytics_AdminToken_Returns200(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Analytics")
	student := seedUser(t, env.pool, "student", "Student Analytics")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "3+3", 1, 1)

	examID := seedExam(t, env.pool, "Analytics Exam", true, "score_only", "classic")
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 80, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/analytics", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if _, ok := resp["average_score"]; !ok {
		t.Errorf("missing average_score in analytics response")
	}
	if _, ok := resp["completion_rate"]; !ok {
		t.Errorf("missing completion_rate in analytics response")
	}
	if _, ok := resp["distribution"]; !ok {
		t.Errorf("missing distribution in analytics response")
	}
}

// ---------------------------------------------------------------------------
// AdminGetExamCertificatePreview tests
// ---------------------------------------------------------------------------

func TestAdminGetExamCertificatePreview_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-preview?template=classic", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamCertificatePreview_StudentToken_Returns403(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "student-cert-preview",
		Email:  strptr("student-cert@test.com"),
		Role:   service.RoleStudent,
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	token := mintToken(t, env, "student-cert-preview", service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-preview?template=classic", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamCertificatePreview_ValidToken_Returns200PDF(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert Preview")

	examID := seedExam(t, env.pool, "Certificate Test Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=classic", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/pdf" {
		t.Errorf("Content-Type: want application/pdf, got %q", contentType)
	}
	body := rec.Body.Bytes()
	if len(body) == 0 {
		t.Fatal("empty response body")
	}
	if !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Errorf("body should start with %%PDF, got %q", string(body[:min(len(body), 10)]))
	}
}

func TestAdminGetExamCertificatePreview_InvalidTemplate_Returns422(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert 422")

	examID := seedExam(t, env.pool, "Cert 422 Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=invalid-template-key", token)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "validation_failed" {
		t.Errorf("code: want validation_failed, got %v", resp["code"])
	}
}

func TestAdminGetExamCertificatePreview_UnknownExam_Returns404(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert 404")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-0000000000aa/certificate-preview", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "exam_not_found" {
		t.Errorf("code: want exam_not_found, got %v", resp["code"])
	}
}

// ---------------------------------------------------------------------------
// StudentGetSessionLeaderboard tests
// ---------------------------------------------------------------------------

func TestStudentGetSessionLeaderboard_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerStudentLeaderboardRoute(t, env, h)

	rec := getRequest(t, env.e, "/api/v1/exam/sessions/00000000-0000-0000-0000-000000000000/leaderboard", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStudentGetSessionLeaderboard_NotOwned_Returns404(t *testing.T) {
	env := newTestEnvWithStore(t)
	owner := seedUser(t, env.pool, "student", "Session Owner")
	other := seedUser(t, env.pool, "student", "Other Student")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "Leaderboard Exam", true, "score_only", "classic")
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, owner, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, owner, examID, "submitted", 90, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	// Other student tries to access owner's session
	token := mintTokenForEnv(t, env, other.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/leaderboard", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "session_not_found" {
		t.Errorf("code: want session_not_found, got %v", resp["code"])
	}
}

func TestStudentGetSessionLeaderboard_LeaderboardNotAvailable_Returns403(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Student LB Disabled")

	examID := seedExam(t, env.pool, "Disabled LB Exam", false, "score_only", "classic")

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 50, &submittedAt)

	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/leaderboard", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "leaderboard_not_available" {
		t.Errorf("code: want leaderboard_not_available, got %v", resp["code"])
	}
}

func TestStudentGetSessionLeaderboard_MalformedCursor_Returns422(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Student Bad Cursor")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "Student Bad Cursor Exam", true, "score_only", "classic")
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 85, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/leaderboard?cursor=nocomma", token)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "validation_failed" {
		t.Errorf("code: want validation_failed, got %v", resp["code"])
	}
}

func TestStudentGetSessionLeaderboard_Success_Returns200(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Student LB Success")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "LB Success Exam", true, "score_only", "classic")
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 85, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/leaderboard", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", resp["data"])
	}
	if len(data) != 1 {
		t.Fatalf("want 1 leaderboard entry, got %d", len(data))
	}
	entry := data[0].(map[string]any)
	if entry["student_id"] != student.String() {
		t.Errorf("student_id mismatch")
	}
	if entry["score"] != float64(85) {
		t.Errorf("score: want 85, got %v", entry["score"])
	}
	if _, ok := entry["rank"]; !ok {
		t.Errorf("missing rank in leaderboard entry")
	}
}

// ---------------------------------------------------------------------------
// AdminUpdateExam with certificate_template tests
// ---------------------------------------------------------------------------

func TestAdminUpdateExam_ValidCertificateTemplate_Returns200(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Update Cert")

	examID := seedExam(t, env.pool, "Update Cert Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"certificate_template": "modern"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Verify the value was persisted by reading it back via a separate query.
	var persisted string
	err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_template FROM exam WHERE id = $1`, examID,
	).Scan(&persisted)
	if err != nil {
		t.Fatalf("query certificate_template: %v", err)
	}
	if persisted != "modern" {
		t.Errorf("certificate_template: want modern, got %q", persisted)
	}
}

func TestAdminUpdateExam_InvalidCertificateTemplate_Returns422(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Update Cert 422")

	examID := seedExam(t, env.pool, "Update Cert 422 Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"certificate_template": "invalid-template-key"},
	)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "validation_failed" {
		t.Errorf("code: want validation_failed, got %v", resp["code"])
	}
}

func TestAdminUpdateExam_ExplicitNullClearsCheckInWindow(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Clear CheckIn")

	examID := seedExam(t, env.pool, "Clear CheckIn Exam", false, "hidden", "classic")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET check_in_window_minutes = 30 WHERE id = $1`, examID,
	); err != nil {
		t.Fatalf("seed check_in_window_minutes: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	// Explicit null must CLEAR the field, not be treated as "absent."
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]any{"check_in_window_minutes": nil},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persisted *int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT check_in_window_minutes FROM exam WHERE id = $1`, examID,
	).Scan(&persisted); err != nil {
		t.Fatalf("query check_in_window_minutes: %v", err)
	}
	if persisted != nil {
		t.Errorf("check_in_window_minutes: want cleared (nil), got %v", *persisted)
	}
}

func TestAdminUpdateExam_OmittedFieldPreservesCheckInWindow(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preserve CheckIn")

	examID := seedExam(t, env.pool, "Preserve CheckIn Exam", false, "hidden", "classic")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET check_in_window_minutes = 30 WHERE id = $1`, examID,
	); err != nil {
		t.Fatalf("seed check_in_window_minutes: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	// An unrelated-field PATCH that omits check_in_window_minutes must PRESERVE it.
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"certificate_template": "modern"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persisted *int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT check_in_window_minutes FROM exam WHERE id = $1`, examID,
	).Scan(&persisted); err != nil {
		t.Fatalf("query check_in_window_minutes: %v", err)
	}
	if persisted == nil || *persisted != 30 {
		t.Errorf("check_in_window_minutes: want preserved 30, got %v", persisted)
	}
}

func seedTestRow(t *testing.T, pool *pgxpool.Pool, title string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO test (title, subject, topic, duration_minutes, audio_url, audio_play_limit)
		VALUES ($1, 'english', 'listening', 30, 'https://example.com/audio.mp3', 2) RETURNING id`,
		title,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert test: %v", err)
	}
	return id
}

func TestAdminUpdateTest_ExplicitNullClearsAudioURL(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Clear Audio")
	testID := seedTestRow(t, env.pool, "Clear Audio Test")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	// Explicit null must CLEAR audio_url, not be treated as "absent."
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/tests/"+testID.String(), token,
		map[string]any{"audio_url": nil},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persisted *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT audio_url FROM test WHERE id = $1`, testID,
	).Scan(&persisted); err != nil {
		t.Fatalf("query audio_url: %v", err)
	}
	if persisted != nil {
		t.Errorf("audio_url: want cleared (nil), got %v", *persisted)
	}
}

func TestAdminUpdateTest_OmittedFieldPreservesAudioURL(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preserve Audio")
	testID := seedTestRow(t, env.pool, "Preserve Audio Test")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	// An unrelated-field PATCH that omits audio_url must PRESERVE it.
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/tests/"+testID.String(), token,
		map[string]string{"title": "Renamed Test"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persisted *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT audio_url FROM test WHERE id = $1`, testID,
	).Scan(&persisted); err != nil {
		t.Fatalf("query audio_url: %v", err)
	}
	if persisted == nil || *persisted != "https://example.com/audio.mp3" {
		t.Errorf("audio_url: want preserved, got %v", persisted)
	}
}
