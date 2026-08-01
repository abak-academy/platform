package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
	adminExams.POST("/:id/certificate-preview", h.AdminGetExamCertificatePreview)
	adminExams.POST("/:id/certificate-assets/presign", h.AdminPresignExamCertificateAsset)
	adminExams.GET("/:id/certificate-design", h.AdminGetExamCertificateDesign)
	adminExams.PUT("/:id/certificate-design", h.AdminUpdateExamCertificateDesign)
	adminExams.PATCH("/:id/certificate-enabled", h.AdminSetExamCertificateEnabled)
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
// Fake Gotenberg (no live sidecar in the test env)
// ---------------------------------------------------------------------------

// fakePDFBytes is a minimal well-formed PDF good enough for the
// "%PDF"-prefix/non-empty assertions these tests make on the response body.
var fakePDFBytes = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF")

// fakeGotenbergRecorder counts hits per route and captures the last "url"
// form field posted to /forms/chromium/convert/url, so tests can prove a
// certificate render went through RenderURL (the print route) and never
// through RenderHTML (Task 13, FR-27/FR-29).
type fakeGotenbergRecorder struct {
	mu        sync.Mutex
	htmlCalls int
	urlCalls  int
	lastURL   string
}

func (r *fakeGotenbergRecorder) recordHTML() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.htmlCalls++
}

func (r *fakeGotenbergRecorder) recordURL(u string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urlCalls++
	r.lastURL = u
}

func (r *fakeGotenbergRecorder) snapshot() (htmlCalls, urlCalls int, lastURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.htmlCalls, r.urlCalls, r.lastURL
}

// newFakeGotenbergServer stands in for a real Gotenberg sidecar: it answers
// both the Chromium HTML-to-PDF route and the URL-to-PDF route (FR-26/FR-27)
// the renderer POSTs to with a fixed PDF, so certificate tests get a real
// 200+PDF round trip without a live Gotenberg in the test environment.
// Closed automatically via t.Cleanup.
func newFakeGotenbergServer(t *testing.T) *httptest.Server {
	rec, srv := newRecordingFakeGotenbergServer(t)
	_ = rec
	return srv
}

// newRecordingFakeGotenbergServer is newFakeGotenbergServer plus the request
// recorder, for tests that need to assert which Gotenberg route was hit.
func newRecordingFakeGotenbergServer(t *testing.T) (*fakeGotenbergRecorder, *httptest.Server) {
	t.Helper()
	rec := &fakeGotenbergRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/forms/chromium/convert/html":
			rec.recordHTML()
		case "/forms/chromium/convert/url":
			if err := r.ParseMultipartForm(1 << 20); err == nil {
				rec.recordURL(r.FormValue("url"))
			}
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		w.Write(fakePDFBytes)
	}))
	t.Cleanup(srv.Close)
	return rec, srv
}

// ---------------------------------------------------------------------------
// DB-backed test environment (testcontainers Postgres)
// ---------------------------------------------------------------------------

type testEnvWithStore struct {
	pool      *pgxpool.Pool
	mr        *miniredis.Miniredis
	e         *echo.Echo
	svc       *service.Service
	signer    *infra.JWTSigner
	gotenberg *fakeGotenbergRecorder
}

func newTestEnvWithStore(t *testing.T) *testEnvWithStore {
	t.Helper()
	return newTestEnvWithStoreCfg(t, nil, &config.Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
		WebInternalURL:  "http://web-internal.test:3000",
	})
}

// newTestEnvWithStoreAndStorage is like newTestEnvWithStore but wires a real
// MinIO client so certificate-design tests can assert on presigned URLs
// (FR-18). Region is set explicitly so presigning never needs a reachable
// endpoint — it's a pure local computation once region is known (see
// presignStorage's own comment on why).
func newTestEnvWithStoreAndStorage(t *testing.T) *testEnvWithStore {
	t.Helper()
	client, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("test-access", "test-secret", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	return newTestEnvWithStoreCfg(t, client, &config.Config{
		JWTSecret:               "test-secret",
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         168 * time.Hour,
		OTPTTL:                  5 * time.Minute,
		ObjectStorageBucketName: "test-bucket",
		ObjectStorageRegion:     "us-east-1",
		WebInternalURL:          "http://web-internal.test:3000",
	})
}

func newTestEnvWithStoreCfg(t *testing.T, storage *minio.Client, cfg *config.Config) *testEnvWithStore {
	t.Helper()
	ctx := context.Background()

	// No live Gotenberg sidecar runs in the test environment; point the
	// renderer at a fake one unless a caller already configured a URL.
	var gotenberg *fakeGotenbergRecorder
	if cfg.GotenbergURL == "" {
		var srv *httptest.Server
		gotenberg, srv = newRecordingFakeGotenbergServer(t)
		cfg.GotenbergURL = srv.URL
	}

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

	signer := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)

	svc := service.NewWithStore(
		store, store, rdb, signer,
		&service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{},
		storage, cfg,
		nil,
	)

	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	server.RegisterRoutesForTest(e, h, svc, signer)

	return &testEnvWithStore{pool: pool, mr: mr, e: e, svc: svc, signer: signer, gotenberg: gotenberg}
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

// postRequest issues a bodyless POST — used by certificate-preview when no
// unsaved-layout override is being tested.
func postRequest(t *testing.T, e *echo.Echo, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
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

// putJSONRequest issues a PUT with JSON body.
func putJSONRequest(t *testing.T, e *echo.Echo, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// postCertificatePreviewRequest issues a POST carrying a JSON body — the
// transport a real browser can use, unlike a GET with a body — used by the
// certificate-preview endpoint's optional unsaved-layout override.
func postCertificatePreviewRequest(t *testing.T, e *echo.Echo, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
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

// seedExam inserts an exam with certificate_enabled = true — most callers
// exercise certificate-design/preview/leaderboard flows that assume the
// feature is on; tests for the certificate_enabled gate itself (Task 4)
// explicitly flip it off with seedExamCertificateDisabled or a direct UPDATE.
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
		`INSERT INTO exam (title, allow_leaderboard, result_config, certificate_design, certificate_enabled, timer_mode, duration_minutes)
		VALUES ($1, $2, $3, $4, true, 'overall', 60) RETURNING id`,
		title, allowLeaderboard, resultConfig, fmt.Sprintf(`{"template":%q}`, certificateTemplate),
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	return id
}

// setExamCertificateEnabled directly flips certificate_enabled in the DB —
// used by tests that need a specific starting state without going through
// the admin action under test.
func setExamCertificateEnabled(t *testing.T, pool *pgxpool.Pool, examID uuid.UUID, enabled bool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE exam SET certificate_enabled = $1 WHERE id = $2`, enabled, examID,
	); err != nil {
		t.Fatalf("set certificate_enabled: %v", err)
	}
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

	rec := postRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-preview?template=classic", "")
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
	rec := postRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-preview?template=classic", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamCertificatePreview_ValidToken_Returns200PDF(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert Preview")

	examID := seedExam(t, env.pool, "Certificate Test Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := postRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=classic", token)
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
	rec := postRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=invalid-template-key", token)
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
	rec := postRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-0000000000aa/certificate-preview", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "exam_not_found" {
		t.Errorf("code: want exam_not_found, got %v", resp["code"])
	}
}

// TestAdminGetExamCertificatePreview_WithUnsavedLayout_RendersOverride proves
// the editor can preview a change before saving: an optional body layout still
// renders through the same engine as real generation.
func TestAdminGetExamCertificatePreview_WithUnsavedLayout_RendersOverride(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preview Override")

	examID := seedExam(t, env.pool, "Preview Override Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := postCertificatePreviewRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=classic", token,
		map[string]any{"layout": validCertLayoutBody()},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("expected a PDF body, got %q", string(rec.Body.Bytes()[:min(len(rec.Body.Bytes()), 10)]))
	}
}

func TestAdminGetExamCertificatePreview_WithInvalidUnsavedLayout_Returns422(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preview Bad Override")

	examID := seedExam(t, env.pool, "Preview Bad Override Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := postCertificatePreviewRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=classic", token,
		map[string]any{"layout": invalidCertLayoutBody()},
	)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAdminGetExamCertificatePreview_RendersThroughPrintRoute_NotRenderHTML is
// Task 13's core assertion for FR-27/FR-29: a preview must mint a print token
// and ask Gotenberg to fetch the certificate print route on the configured
// internal web origin — never build HTML in Go and post it to Gotenberg's
// HTML route. The unsaved layout override must travel in the minted token's
// payload, not the URL (NFR-S5): the posted "url" carries only token+id.
func TestAdminGetExamCertificatePreview_RendersThroughPrintRoute_NotRenderHTML(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preview Print Route")

	examID := seedExam(t, env.pool, "Preview Print Route Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := postCertificatePreviewRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=classic", token,
		map[string]any{"layout": validCertLayoutBody()},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	htmlCalls, urlCalls, lastURL := env.gotenberg.snapshot()
	if htmlCalls != 0 {
		t.Errorf("Gotenberg HTML route calls = %d, want 0 — no certificate HTML may be built in Go (FR-27)", htmlCalls)
	}
	if urlCalls != 1 {
		t.Fatalf("Gotenberg URL route calls = %d, want 1", urlCalls)
	}
	wantPrefix := "http://web-internal.test:3000/documents/certificate?token="
	if !strings.HasPrefix(lastURL, wantPrefix) {
		t.Errorf("rendered url = %q, want prefix %q (the configured internal web origin)", lastURL, wantPrefix)
	}
	if !strings.Contains(lastURL, "&id="+examID.String()) {
		t.Errorf("rendered url = %q, want it to carry id=%s", lastURL, examID.String())
	}
	// NFR-S5: the unsaved layout override must never appear in the URL itself.
	if strings.Contains(lastURL, "x_mm") || strings.Contains(lastURL, "title") {
		t.Errorf("rendered url = %q, must not leak the layout override — it belongs in the token payload only", lastURL)
	}
}

// TestAdminGetExamCertificatePreview_NoObjectStoreWrite proves FR-29: a
// preview never serves or writes a stored PDF. env has no MinIO client
// (newTestEnvWithStore passes storage=nil) — uploadCertificatePDF guards on a
// nil s.storage and returns ErrStorageNotConfigured immediately, so a 200 PDF
// response here is direct proof the preview path never attempted an upload.
func TestAdminGetExamCertificatePreview_NoObjectStoreWrite(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Preview No Store Write")

	examID := seedExam(t, env.pool, "Preview No Store Write Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := postRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-preview?template=classic", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 (which is only reachable if no object-store write was attempted against a nil storage client), got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("expected a PDF body, got %q", string(rec.Body.Bytes()[:min(len(rec.Body.Bytes()), 10)]))
	}
}

// ---------------------------------------------------------------------------
// AdminGetExamCertificateDesign / AdminUpdateExamCertificateDesign tests (Task 8)
// ---------------------------------------------------------------------------

// validCertLayoutBody is a minimal layout JSON body that passes ValidateLayout:
// a real A4-landscape page and one known field id inside its bounds.
func validCertLayoutBody() map[string]any {
	return map[string]any{
		"page":       map[string]any{"width_mm": 297, "height_mm": 210},
		"background": map[string]any{"kind": "builtin", "ref": "classic"},
		"fields": []map[string]any{
			{"id": "title", "x_mm": 48.5, "y_mm": 42, "w_mm": 200, "align": "center", "visible": true},
		},
	}
}

// invalidCertLayoutBody carries an unknown field id, which ValidateLayout (Task 3)
// must reject.
func invalidCertLayoutBody() map[string]any {
	return map[string]any{
		"page":       map[string]any{"width_mm": 297, "height_mm": 210},
		"background": map[string]any{"kind": "builtin", "ref": "classic"},
		"fields": []map[string]any{
			{"id": "not_a_real_field", "x_mm": 10, "y_mm": 10, "w_mm": 50, "align": "center", "visible": true},
		},
	}
}

func TestAdminGetExamCertificateDesign_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-design", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamCertificateDesign_StudentToken_Returns403(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "student-cert-design",
		Email:  strptr("student-cert-design@test.com"),
		Role:   service.RoleStudent,
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	token := mintToken(t, env, "student-cert-design", service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-design", token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateExamCertificateDesign_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	rec := putJSONRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-design", "",
		map[string]any{"template": "classic", "layout": validCertLayoutBody()},
	)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateExamCertificateDesign_StudentToken_Returns403(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "student-cert-design-put",
		Email:  strptr("student-cert-design-put@test.com"),
		Role:   service.RoleStudent,
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	token := mintToken(t, env, "student-cert-design-put", service.RoleStudent)
	rec := putJSONRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-design", token,
		map[string]any{"template": "classic", "layout": validCertLayoutBody()},
	)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetExamCertificateDesign_UntouchedExam_ReturnsBuiltinDefaultLayout(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Default")

	examID := seedExam(t, env.pool, "Design Default Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Template      string  `json:"template"`
		BackgroundURL *string `json:"background_url"`
		Layout        struct {
			Fields []struct {
				ID string `json:"id"`
			} `json:"fields"`
		} `json:"layout"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Template != "classic" {
		t.Errorf("template: want classic, got %q", resp.Template)
	}
	if resp.BackgroundURL != nil {
		t.Errorf("background_url: want nil for an untouched exam, got %v", *resp.BackgroundURL)
	}
	if len(resp.Layout.Fields) == 0 {
		t.Fatal("expected the built-in default layout, got zero fields")
	}
}

func TestAdminGetExamCertificateDesign_UnknownExam_Returns404(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design 404")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-0000000000aa/certificate-design", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminPresignExamCertificateAsset_ReturnsExamScopedKey(t *testing.T) {
	env := newTestEnvWithStoreAndStorage(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Asset")
	examID := seedExam(t, env.pool, "Design Asset Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := postRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-assets/presign?filename=logo.png&content_type=image/png", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Key string `json:"key"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(resp.Key, "certificates/"+examID.String()+"/") {
		t.Fatalf("key %q is not scoped to exam %s", resp.Key, examID)
	}
	if !strings.Contains(resp.URL, "X-Amz-Signature") {
		t.Fatalf("expected a presigned URL, got %q", resp.URL)
	}
	signedURL, err := url.Parse(resp.URL)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	if got := signedURL.Query().Get("X-Amz-SignedHeaders"); !strings.Contains(got, "content-type") {
		t.Fatalf("signed headers = %q, want content-type to be enforced", got)
	}
}

func TestAdminPresignExamCertificateAsset_RejectsSVG(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "admin-cert-svg",
		Email:  strptr("admin-cert-svg@test.com"),
		Role:   service.RoleAdminExam,
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	token := mintToken(t, env, "admin-cert-svg", service.RoleAdminExam)
	rec := postRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-assets/presign?filename=logo.svg&content_type=image/svg%2Bxml", token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAdminGetExamCertificateDesign_CustomBackground_ReturnsPresignedURLNotRawKey
// proves FR-18: the DB stores only the object key, and reads always sign a fresh
// time-limited GET rather than ever returning the key or a raw URL.
func TestAdminGetExamCertificateDesign_CustomBackground_ReturnsPresignedURLNotRawKey(t *testing.T) {
	env := newTestEnvWithStoreAndStorage(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Presign")

	examID := seedExam(t, env.pool, "Design Presign Exam", false, "hidden", "custom")
	key := "avatars/admin/" + uuid.NewString() + "-bg.png"
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET certificate_design = jsonb_set(COALESCE(certificate_design, '{}'::jsonb), '{background_key}', to_jsonb($1::text)) WHERE id = $2`,
		key, examID,
	); err != nil {
		t.Fatalf("seed certificate_design background_key: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		BackgroundURL *string `json:"background_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.BackgroundURL == nil {
		t.Fatal("expected a non-nil background_url")
	}
	if *resp.BackgroundURL == key {
		t.Errorf("background_url must be presigned, not the raw key: got %q", *resp.BackgroundURL)
	}
	if !strings.Contains(*resp.BackgroundURL, "X-Amz-Signature") {
		t.Errorf("expected a presigned URL (X-Amz-Signature query param), got %q", *resp.BackgroundURL)
	}
}

// TestAdminUpdateExamCertificateDesign_ValidPUT_PersistsAndBumpsTimestamp proves
// a valid PUT persists template/background_key/layout and bumps
// certificate_design_updated_at (FR-14/C3), reusing UpdateExam's own wiring.
func TestAdminUpdateExamCertificateDesign_ValidPUT_PersistsAndBumpsTimestamp(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design PUT")

	examID := seedExam(t, env.pool, "Design PUT Exam", false, "hidden", "classic")

	var before *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design_updated_at FROM exam WHERE id = $1`, examID,
	).Scan(&before); err != nil {
		t.Fatalf("query certificate_design_updated_at (before): %v", err)
	}
	if before != nil {
		t.Fatalf("want certificate_design_updated_at initially NULL, got %v", *before)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	key := "certificates/" + examID.String() + "/" + uuid.NewString() + "-bg.png"
	rec := putJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token,
		map[string]any{"template": "custom", "background_key": key, "layout": validCertLayoutBody()},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persistedDesign []byte
	var after *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design, certificate_design_updated_at FROM exam WHERE id = $1`, examID,
	).Scan(&persistedDesign, &after); err != nil {
		t.Fatalf("query persisted design: %v", err)
	}
	var decoded struct {
		Template      string `json:"template"`
		BackgroundKey string `json:"background_key"`
		Fields        []any  `json:"fields"`
	}
	if err := json.Unmarshal(persistedDesign, &decoded); err != nil {
		t.Fatalf("unmarshal certificate_design: %v", err)
	}
	if decoded.Template != "custom" {
		t.Errorf("certificate_design template: want custom, got %q", decoded.Template)
	}
	if decoded.BackgroundKey != key {
		t.Errorf("certificate_design background_key: want %q, got %q", key, decoded.BackgroundKey)
	}
	if len(decoded.Fields) == 0 {
		t.Error("expected certificate_design fields to be persisted")
	}
	if after == nil {
		t.Fatal("expected certificate_design_updated_at to be bumped, got NULL")
	}
}

func TestAdminUpdateExamCertificateDesign_CrossExamAssetRejected(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Cross Asset")
	examID := seedExam(t, env.pool, "Design Cross Asset Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := putJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token,
		map[string]any{
			"template":       "custom",
			"background_key": "certificates/00000000-0000-0000-0000-0000000000aa/background.png",
			"layout":         validCertLayoutBody(),
		},
	)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAdminUpdateExamCertificateDesign_UnknownFieldID_Rejected proves the
// editor is not the security boundary: an unknown layout field id is rejected
// server-side (Task 3's ValidateLayout), even though the request otherwise
// looks well-formed.
func TestAdminUpdateExamCertificateDesign_UnknownFieldID_Rejected(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Bad Field")

	examID := seedExam(t, env.pool, "Design Bad Field Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := putJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token,
		map[string]any{"template": "classic", "layout": invalidCertLayoutBody()},
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

// TestAdminUpdateExamCertificateDesign_OmittedLayout_Rejected covers Warning
// 4/Invariant 8: a PUT that omits `layout` entirely marshals the zero Layout
// (page 0x0mm, nil fields) into the exam row. Before ValidateLayout checked
// page dimensions this degenerate layout was accepted and persisted, so every
// later certificate render for this exam would produce a zero-size page.
func TestAdminUpdateExamCertificateDesign_OmittedLayout_Rejected(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Omitted Layout")

	examID := seedExam(t, env.pool, "Design Omitted Layout Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := putJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token,
		map[string]any{"template": "classic"},
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

func TestAdminUpdateExamCertificateDesign_UnknownExam_Returns404(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design PUT 404")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := putJSONRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-0000000000aa/certificate-design", token,
		map[string]any{"template": "classic", "layout": validCertLayoutBody()},
	)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// AdminSetExamCertificateEnabled / certificate_enabled gate tests (Task 4)
// ---------------------------------------------------------------------------

// TestAdminGetExamCertificateDesign_Disabled_Returns404 proves FR-9: the
// design editor is unreachable while certificate_enabled is false, even
// though the exam itself (and any saved design) exists.
func TestAdminGetExamCertificateDesign_Disabled_Returns404(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert Disabled")

	examID := seedExam(t, env.pool, "Disabled Cert Exam", false, "hidden", "classic")
	setExamCertificateEnabled(t, env.pool, examID, false)

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "certificate_disabled" {
		t.Errorf("code: want certificate_disabled, got %v", resp["code"])
	}
	if _, hasLayout := resp["layout"]; hasLayout {
		t.Errorf("expected no design payload on a disabled exam, got layout=%v", resp["layout"])
	}
}

// TestAdminGetExamCertificateDesign_EnabledAfterAction_Returns200 proves
// FR-9/FR-11: the same GET that 404s while disabled returns 200 once an
// admin enables the exam's certificate through the dedicated action.
func TestAdminGetExamCertificateDesign_EnabledAfterAction_Returns200(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert Enable")

	examID := seedExam(t, env.pool, "Enable Cert Exam", false, "hidden", "classic")
	setExamCertificateEnabled(t, env.pool, examID, false)

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("before enabling: want 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	enableRec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-enabled", token,
		map[string]any{"enabled": true},
	)
	if enableRec.Code != http.StatusOK {
		t.Fatalf("enable: want 200, got %d body=%s", enableRec.Code, enableRec.Body.String())
	}
	// The JSON key must be visible in a real handler response body, not only
	// asserted against the Go struct.
	var enableResp map[string]any
	if err := json.NewDecoder(enableRec.Body).Decode(&enableResp); err != nil {
		t.Fatalf("decode enable response: %v", err)
	}
	enabledVal, ok := enableResp["certificate_enabled"]
	if !ok {
		t.Fatal("enable response missing certificate_enabled key")
	}
	if enabledVal != true {
		t.Errorf("certificate_enabled: want true, got %v", enabledVal)
	}

	rec = getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("after enabling: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAdminSetExamCertificateEnabled_DisableThenReenable_PreservesDesign
// proves FR-12: toggling certificate_enabled off and back on never touches
// certificate_design or certificate_design_updated_at — the dedicated action
// is a single-column write, not a general update.
func TestAdminSetExamCertificateEnabled_DisableThenReenable_PreservesDesign(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert Preserve")

	examID := seedExam(t, env.pool, "Preserve Cert Exam", false, "hidden", "classic")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	putRec := putJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-design", token,
		map[string]any{"template": "classic", "layout": validCertLayoutBody()},
	)
	if putRec.Code != http.StatusOK {
		t.Fatalf("seed design PUT: want 200, got %d body=%s", putRec.Code, putRec.Body.String())
	}

	var beforeDesign []byte
	var beforeUpdatedAt time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design, certificate_design_updated_at FROM exam WHERE id = $1`, examID,
	).Scan(&beforeDesign, &beforeUpdatedAt); err != nil {
		t.Fatalf("query design before: %v", err)
	}

	disableRec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-enabled", token,
		map[string]any{"enabled": false},
	)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable: want 200, got %d body=%s", disableRec.Code, disableRec.Body.String())
	}
	enableRec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/certificate-enabled", token,
		map[string]any{"enabled": true},
	)
	if enableRec.Code != http.StatusOK {
		t.Fatalf("re-enable: want 200, got %d body=%s", enableRec.Code, enableRec.Body.String())
	}

	var afterDesign []byte
	var afterUpdatedAt time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design, certificate_design_updated_at FROM exam WHERE id = $1`, examID,
	).Scan(&afterDesign, &afterUpdatedAt); err != nil {
		t.Fatalf("query design after: %v", err)
	}

	if !bytes.Equal(beforeDesign, afterDesign) {
		t.Errorf("certificate_design changed across disable/re-enable:\nbefore=%s\nafter=%s", beforeDesign, afterDesign)
	}
	if !beforeUpdatedAt.Equal(afterUpdatedAt) {
		t.Errorf("certificate_design_updated_at changed across disable/re-enable: before=%v after=%v", beforeUpdatedAt, afterUpdatedAt)
	}
}

func TestAdminSetExamCertificateEnabled_NoToken_Returns401(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-enabled", "",
		map[string]any{"enabled": true},
	)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminSetExamCertificateEnabled_StudentToken_Returns403(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "student-cert-enabled",
		Email:  strptr("student-cert-enabled@test.com"),
		Role:   service.RoleStudent,
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminExamRoutes(t, env, h)

	token := mintToken(t, env, "student-cert-enabled", service.RoleStudent)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-000000000000/certificate-enabled", token,
		map[string]any{"enabled": true},
	)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminSetExamCertificateEnabled_UnknownExam_Returns404(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Cert Enable 404")

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/00000000-0000-0000-0000-0000000000aa/certificate-enabled", token,
		map[string]any{"enabled": true},
	)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestStudentGetSessionResult_DisabledCertificate_ReturnsNullCertificateURL
// proves FR-10 end-to-end through the real result endpoint: a submitted
// session of an exam with certificate_enabled=false must carry a null
// certificate_url, not merely at the service unit level.
func TestStudentGetSessionResult_DisabledCertificate_ReturnsNullCertificateURL(t *testing.T) {
	env := newTestEnvWithStore(t)
	student := seedUser(t, env.pool, "student", "Student Disabled Cert")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "Disabled Cert Result Exam", false, "score_only", "classic")
	setExamCertificateEnabled(t, env.pool, examID, false)
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 1, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/result", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.Bytes()
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Task 13 dropped `,omitempty` from SessionResult.CertificateURL (NFR-R3,
	// FR-5): the key must be present and null, not merely absent — so this
	// checks both, where the pre-Task-13 version of this test could only
	// check the value (a missing key and a null value decode identically).
	if !strings.Contains(string(body), `"certificate_url"`) {
		t.Fatalf(`response body missing the "certificate_url" key entirely, want it present with a null value: %s`, body)
	}
	if certURL := resp["certificate_url"]; certURL != nil {
		t.Errorf("certificate_url: want nil for a disabled exam, got %v", certURL)
	}
}

// TestStudentGetSessionResult_CertificateRenderURLFailure_DegradesToNullCertificateURL
// proves NFR-R1 end to end through the real result endpoint after Task 13's
// switch to RenderURL: a stopped/erroring web container must degrade to a
// logged error and a null certificate_url, never a 5xx. The exam here has
// certificate_enabled=true and no cached certificate, so GetSessionResult must
// attempt a fresh render (unlike the disabled-certificate test above, which
// never reaches the renderer at all).
func TestStudentGetSessionResult_CertificateRenderURLFailure_DegradesToNullCertificateURL(t *testing.T) {
	failing := newFailingURLGotenbergServer(t)
	env := newTestEnvWithStoreCfg(t, nil, &config.Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
		OTPTTL:          5 * time.Minute,
		GotenbergURL:    failing.URL,
		WebInternalURL:  "http://web-internal.test:3000",
	})
	student := seedUser(t, env.pool, "student", "Student RenderURL Failure")

	testID := seedTest(t, env.pool)
	qID := seedMCQuestion(t, env.pool, testID, "2+2", 1, 1)

	examID := seedExam(t, env.pool, "RenderURL Failure Exam", false, "score_only", "classic")
	seedExamTest(t, env.pool, examID, testID, 1)

	regID := seedRegistration(t, env.pool, student, examID)
	submittedAt := time.Now()
	sessionID := seedSession(t, env.pool, regID, student, examID, "submitted", 1, &submittedAt)
	seedAnswer(t, env.pool, sessionID, qID, "a", 1)

	token := mintTokenForEnv(t, env, student.String(), service.RoleStudent)
	rec := getRequest(t, env.e, "/api/v1/exam/sessions/"+sessionID.String()+"/result", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 even though the certificate render failed (NFR-R1), got %d body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.Bytes()
	if !strings.Contains(string(body), `"certificate_url"`) {
		t.Fatalf(`response body missing the "certificate_url" key entirely: %s`, body)
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if certURL := resp["certificate_url"]; certURL != nil {
		t.Errorf("certificate_url: want null when the certificate render fails, got %v", certURL)
	}
}

// newFailingURLGotenbergServer answers the HTML route like a normal
// Gotenberg, but always 500s the URL route — standing in for NFR-R1's "a
// stopped, restarting or erroring web container" scenario, since a failed
// RenderURL is exactly what a real Gotenberg would report when the web
// container it's asked to fetch from is down.
func newFailingURLGotenbergServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/forms/chromium/convert/url":
			http.Error(w, "simulated web container failure", http.StatusInternalServerError)
		case "/forms/chromium/convert/html":
			w.Header().Set("Content-Type", "application/pdf")
			w.WriteHeader(http.StatusOK)
			w.Write(fakePDFBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
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
	var persisted []byte
	err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design FROM exam WHERE id = $1`, examID,
	).Scan(&persisted)
	if err != nil {
		t.Fatalf("query certificate_design: %v", err)
	}
	var decoded struct {
		Template string `json:"template"`
	}
	if err := json.Unmarshal(persisted, &decoded); err != nil {
		t.Fatalf("unmarshal certificate_design: %v", err)
	}
	if decoded.Template != "modern" {
		t.Errorf("certificate_design template: want modern, got %q", decoded.Template)
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

// TestAdminUpdateExam_CertificateTemplateChange_BumpsDesignUpdatedAt proves FR-14:
// a write that changes certificate_template bumps certificate_design_updated_at,
// which is what makes resolveCertificateURL's staleness check (FR-13) fire.
func TestAdminUpdateExam_CertificateTemplateChange_BumpsDesignUpdatedAt(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Bump")

	examID := seedExam(t, env.pool, "Design Bump Exam", false, "hidden", "classic")

	var before *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design_updated_at FROM exam WHERE id = $1`, examID,
	).Scan(&before); err != nil {
		t.Fatalf("query certificate_design_updated_at (before): %v", err)
	}
	if before != nil {
		t.Fatalf("want certificate_design_updated_at initially NULL, got %v", *before)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"certificate_template": "modern"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var after *time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design_updated_at FROM exam WHERE id = $1`, examID,
	).Scan(&after); err != nil {
		t.Fatalf("query certificate_design_updated_at (after): %v", err)
	}
	if after == nil {
		t.Fatal("certificate_design_updated_at should be set after a template change")
	}
}

// TestAdminUpdateExam_UnrelatedFieldChange_PreservesDesignUpdatedAt proves the
// inverse of FR-14: a PATCH that does not touch template/background/layout
// must not bump certificate_design_updated_at.
func TestAdminUpdateExam_UnrelatedFieldChange_PreservesDesignUpdatedAt(t *testing.T) {
	env := newTestEnvWithStore(t)
	admin := seedUser(t, env.pool, "admin_exam", "Admin Design Preserve")

	examID := seedExam(t, env.pool, "Design Preserve Exam", false, "hidden", "classic")
	seededAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE exam SET certificate_design_updated_at = $1 WHERE id = $2`, seededAt, examID,
	); err != nil {
		t.Fatalf("seed certificate_design_updated_at: %v", err)
	}

	token := mintTokenForEnv(t, env, admin.String(), service.RoleAdminExam)
	rec := patchJSONRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), token,
		map[string]string{"title": "Design Preserve Exam Renamed"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var after time.Time
	if err := env.pool.QueryRow(context.Background(),
		`SELECT certificate_design_updated_at FROM exam WHERE id = $1`, examID,
	).Scan(&after); err != nil {
		t.Fatalf("query certificate_design_updated_at: %v", err)
	}
	if !after.Equal(seededAt) {
		t.Errorf("certificate_design_updated_at: want preserved %v, got %v", seededAt, after)
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
