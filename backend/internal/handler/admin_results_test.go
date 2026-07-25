package handler_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// ---------------------------------------------------------------------------
// Lightweight handler-level tests (no DB, call handler directly)
// ---------------------------------------------------------------------------

func TestAdminResult_List_MissingExamID_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminListResults(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing exam_id, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminResult_List_BadExamID_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results?exam_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminListResults(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for bad exam_id, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminResult_List_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results?exam_id="+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaimsNil(c, "u1")

	err := env.h.AdminListResults(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("code: want forbidden, got %v", resp["code"])
	}
	if resp["message"] != "missing school scope" {
		t.Errorf("message: want 'missing school scope', got %v", resp["message"])
	}
}

// --- AdminGetResultDetail ---

func TestAdminResult_Detail_BadSessionID_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("session_id")
	c.SetParamValues("not-a-uuid")
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminGetResultDetail(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for bad session_id, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminResult_Detail_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("session_id")
	c.SetParamValues(uuid.NewString())
	setAdminSchoolClaimsNil(c, "u1")

	err := env.h.AdminGetResultDetail(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("code: want forbidden, got %v", resp["code"])
	}
}

// --- AdminExportResults ---

func TestAdminResult_Export_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results/export?exam_id="+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaimsNil(c, "u1")

	err := env.h.AdminExportResults(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("code: want forbidden, got %v", resp["code"])
	}
}

func TestAdminResult_Export_MissingExamID_400(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results/export", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminExportResults(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing exam_id, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminResult_List_AdminSchool_DifferentSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results?exam_id="+uuid.NewString()+"&school_id=s2", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminListResults(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for admin_school passing different school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("code: want forbidden, got %v", resp["code"])
	}
	if resp["message"] != "cannot widen school scope" {
		t.Errorf("message: want 'cannot widen school scope', got %v", resp["message"])
	}
}

func TestAdminResult_Detail_AdminSchool_DifferentSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results/"+uuid.NewString()+"?school_id=s2", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("session_id")
	c.SetParamValues(uuid.NewString())
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminGetResultDetail(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for admin_school passing different school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("code: want forbidden, got %v", resp["code"])
	}
}

func TestAdminResult_Export_AdminSchool_DifferentSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/results/export?exam_id="+uuid.NewString()+"&school_id=s2", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminExportResults(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for admin_school passing different school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("code: want forbidden, got %v", resp["code"])
	}
}

// ---------------------------------------------------------------------------
// Route-level test (no DB, exercises middleware)
// ---------------------------------------------------------------------------

func TestAdminResults_NoCapability_403(t *testing.T) {
	env := newTestEnv(t)
	env.repo.seed(&model.User{
		ID:     "admin-no-results",
		Email:  strptr("admin-no-results@test.com"),
		Role:   service.RoleStudent, // student lacks results:read
		Status: "active",
	})
	h := handler.New(env.svc)
	registerAdminResultsRoutes(t, env, h)

	token := mintToken(t, env, "admin-no-results", service.RoleStudent)

	for _, tc := range []struct {
		name string
		path string
	}{
		{"list", "/api/v1/admin/results?exam_id=" + uuid.NewString()},
		{"detail", "/api/v1/admin/results/" + uuid.NewString()},
		{"export", "/api/v1/admin/results/export?exam_id=" + uuid.NewString()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := getRequest(t, env.e, tc.path, token)
			if rec.Code != http.StatusForbidden {
				t.Errorf("want 403, got %d body=%s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			json.NewDecoder(rec.Body).Decode(&resp)
			if resp["code"] != "forbidden" {
				t.Errorf("code: want forbidden, got %v", resp["code"])
			}
		})
	}
}

func registerAdminResultsRoutes(t *testing.T, env *testEnv, h *handler.Handler) {
	t.Helper()
	v1 := env.e.Group("/api/v1")
	admin := v1.Group("/admin")
	admin.Use(handler.JWTMiddleware(env.svc, env.signer))
	adminResults := admin.Group("/results")
	adminResults.Use(handler.RBACMiddleware("results:read"))
	adminResults.GET("", h.AdminListResults)
	adminResults.GET("/export", h.AdminExportResults)
	adminResults.GET("/:session_id", h.AdminGetResultDetail)
}

// ---------------------------------------------------------------------------
// DB-backed integration tests (testcontainers Postgres, shared via sync.Once)
// ---------------------------------------------------------------------------

var (
	adminResultsDBOnce sync.Once
	adminResultsDBEnv  *adminResultsDBTestEnv
)

type adminResultsDBTestEnv struct {
	pool        *pgxpool.Pool
	pgContainer *tcpostgres.PostgresContainer
	rdb         *redis.Client
	e           *echo.Echo
	svc         *service.Service
	signer      *infra.JWTSigner
	mr          *miniredis.Miniredis
}

func newAdminResultsDBEnv(t *testing.T) *adminResultsDBTestEnv {
	t.Helper()
	adminResultsDBOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_admin_results_test"),
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
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatalf("new pool: %v", err)
		}
		store := repository.New(pool)

		mr, err := miniredis.Run()
		if err != nil {
			t.Fatalf("miniredis: %v", err)
		}
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
		e := echo.New()
		e.HideBanner = true
		h := handler.New(svc)

		// Register admin results routes only.
		v1 := e.Group("/api/v1")
		admin := v1.Group("/admin")
		admin.Use(handler.JWTMiddleware(svc, signer))
		adminResults := admin.Group("/results")
		adminResults.Use(handler.RBACMiddleware("results:read"))
		adminResults.GET("", h.AdminListResults)
		adminResults.GET("/export", h.AdminExportResults)
		adminResults.GET("/:session_id", h.AdminGetResultDetail)

		adminResultsDBEnv = &adminResultsDBTestEnv{
			pool:        pool,
			pgContainer: pgContainer,
			rdb:         rdb,
			e:           e,
			svc:         svc,
			signer:      signer,
			mr:          mr,
		}
	})
	if adminResultsDBEnv == nil {
		t.Fatal("admin results test env failed to initialize")
	}
	return adminResultsDBEnv
}

// ---------------------------------------------------------------------------
// Seed helpers for DB-backed tests
// ---------------------------------------------------------------------------

func seedSchool(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO school (name, code) VALUES ($1, $2) RETURNING id`,
		"Admin Results School", "ars_"+uuid.NewString()[:8],
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert school: %v", err)
	}
	return id
}

func seedUserWithSchool(t *testing.T, pool *pgxpool.Pool, role, name, schoolID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	email := fmt.Sprintf("%s-%s@test.local", role, uuid.NewString())
	username := "u-" + uuid.NewString()[:8]
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, school_id, username)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		email, role, name, schoolID, username,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func seedExamWithMCQ(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var examID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam (title, allow_leaderboard, result_config, certificate_design, timer_mode, duration_minutes)
		VALUES ($1, true, 'score_only', '{"template":"classic"}', 'overall', 60) RETURNING id`,
		"AR Exam "+uuid.NewString()[:8],
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert exam: %v", err)
	}

	var testID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO test (title, subject, topic, duration_minutes)
		VALUES ($1, 'math', 'algebra', 60) RETURNING id`,
		"AR Test",
	).Scan(&testID)
	if err != nil {
		t.Fatalf("insert test: %v", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
		examID, testID,
	)
	if err != nil {
		t.Fatalf("insert exam_test: %v", err)
	}

	var qID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		VALUES ('mcq', 'Sample question', 1, 0) RETURNING id`,
	).Scan(&qID)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, 1)`,
		testID, qID,
	)
	if err != nil {
		t.Fatalf("insert test_question: %v", err)
	}

	for i, o := range []struct{ key, text string; correct bool }{
		{"a", "Correct answer", true},
		{"b", "Wrong answer", false},
	} {
		_, err = pool.Exec(ctx,
			`INSERT INTO question_option (question_id, key, text, is_correct, sort_order) VALUES ($1, $2, $3, $4, $5)`,
			qID, o.key, o.text, o.correct, i+1,
		)
		if err != nil {
			t.Fatalf("insert option: %v", err)
		}
	}

	return examID
}

func seedSubmittedSession(t *testing.T, pool *pgxpool.Pool, studentID, examID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var regID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, examID, uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("insert registration: %v", err)
	}

	now := time.Now()
	var sessID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, status, submitted_at, score)
		VALUES ($1, $2, $3, now(), 'submitted', $4, 80) RETURNING id`,
		regID, studentID, examID, now,
	).Scan(&sessID)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	var qID uuid.UUID
	err = pool.QueryRow(ctx,
		`SELECT q.id FROM question q
		JOIN test_question tq ON tq.question_id = q.id
		JOIN exam_test et ON et.test_id = tq.test_id
		WHERE et.exam_id = $1 LIMIT 1`, examID,
	).Scan(&qID)
	if err != nil {
		t.Fatalf("get question: %v", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO exam_session_answer (session_id, question_id, answer, is_correct, score, graded_at, saved_at)
		VALUES ($1, $2, 'a', true, 1, now(), now())`,
		sessID, qID,
	)
	if err != nil {
		t.Fatalf("insert answer: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Token helper for DB-backed env (school-scoped admin_school)
// ---------------------------------------------------------------------------

func mintAdminToken(t *testing.T, env *adminResultsDBTestEnv, userID, schoolID string) string {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	schoolIDCopy := schoolID
	tokenString, jti, err := env.signer.SignAccess(userID, "admin_school", &schoolIDCopy, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(context.Background(), "session:access:"+jti, userID, 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}
	return tokenString
}

func mintSuperAdminToken(t *testing.T, env *adminResultsDBTestEnv, userID string) string {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	tokenString, jti, err := env.signer.SignAccess(userID, "super_admin", nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(context.Background(), "session:access:"+jti, userID, 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}
	return tokenString
}

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

func TestAdminResult_List_ExamNotFound_404(t *testing.T) {
	env := newAdminResultsDBEnv(t)

	schoolID := seedSchool(t, env.pool)
	adminID := seedUserWithSchool(t, env.pool, "admin_school", "NF Admin", schoolID)
	token := mintAdminToken(t, env, adminID.String(), schoolID)

	rec := getRequest(t, env.e, "/api/v1/admin/results?exam_id="+uuid.NewString(), token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "exam_not_found" {
		t.Errorf("code: want exam_not_found, got %v", resp["code"])
	}
}

func TestAdminResult_Export_CSVContent(t *testing.T) {
	env := newAdminResultsDBEnv(t)

	schoolID := seedSchool(t, env.pool)
	adminID := seedUserWithSchool(t, env.pool, "admin_school", "CSV Admin", schoolID)
	examID := seedExamWithMCQ(t, env.pool)

	student1 := seedUserWithSchool(t, env.pool, "student", "Student One", schoolID)
	student2 := seedUserWithSchool(t, env.pool, "student", "Student Two", schoolID)
	seedSubmittedSession(t, env.pool, student1, examID)
	seedSubmittedSession(t, env.pool, student2, examID)

	token := mintAdminToken(t, env, adminID.String(), schoolID)
	rec := getRequest(t, env.e, "/api/v1/admin/results/export?exam_id="+examID.String(), token)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/csv" {
		t.Errorf("Content-Type: want text/csv, got %q", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if cd != `attachment; filename="results.csv"` {
		t.Errorf("Content-Disposition: want attachment; filename=\"results.csv\", got %q", cd)
	}

	r := csv.NewReader(bytes.NewReader(rec.Body.Bytes()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 records (header + 2 data rows), got %d", len(records))
	}

	wantHeader := []string{"name", "username", "score", "submitted_at"}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Errorf("header[%d]: want %s, got %s", i, h, records[0][i])
		}
	}

	names := map[string]bool{"Student One": false, "Student Two": false}
	for _, row := range records[1:] {
		names[row[0]] = true
		if row[1] == "" {
			t.Errorf("row %s: expected non-empty username", row[0])
		}
		if row[2] != "80" {
			t.Errorf("row %s: want score 80, got %s", row[0], row[2])
		}
		if row[3] == "" {
			t.Errorf("row %s: expected non-empty submitted_at", row[0])
		}
	}
	for name, found := range names {
		if !found {
			t.Errorf("CSV missing student: %s", name)
		}
	}
}

func TestAdminResult_List_SuperAdmin_NonexistentSchool_404(t *testing.T) {
	env := newAdminResultsDBEnv(t)

	examID := seedExamWithMCQ(t, env.pool)
	superToken := mintSuperAdminToken(t, env, "super1")

	rec := getRequest(t, env.e, "/api/v1/admin/results?exam_id="+examID.String()+"&school_id="+uuid.NewString(), superToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for nonexistent school, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "not_found" {
		t.Errorf("code: want not_found, got %v", resp["code"])
	}
}

func TestAdminResult_List_SuperAdmin_ValidSchool_200(t *testing.T) {
	env := newAdminResultsDBEnv(t)

	schoolID := seedSchool(t, env.pool)
	examID := seedExamWithMCQ(t, env.pool)
	superToken := mintSuperAdminToken(t, env, "super2")

	rec := getRequest(t, env.e, "/api/v1/admin/results?exam_id="+examID.String()+"&school_id="+schoolID, superToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["data"] == nil {
		t.Error("want non-nil data array")
	}
}

func TestAdminResult_List_SuperAdmin_NoSchoolID_400(t *testing.T) {
	env := newAdminResultsDBEnv(t)

	examID := seedExamWithMCQ(t, env.pool)
	superToken := mintSuperAdminToken(t, env, "super3")

	rec := getRequest(t, env.e, "/api/v1/admin/results?exam_id="+examID.String(), superToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for super_admin without school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_request" {
		t.Errorf("code: want invalid_request, got %v", resp["code"])
	}
}

func TestAdminResults_RoutePrecedence(t *testing.T) {
	// FR-SCHOOL-08-18: /export must resolve to the export handler, not the detail handler.
	env := newAdminResultsDBEnv(t)

	schoolID := seedSchool(t, env.pool)
	adminID := seedUserWithSchool(t, env.pool, "admin_school", "Prec Admin", schoolID)
	examID := seedExamWithMCQ(t, env.pool)

	token := mintAdminToken(t, env, adminID.String(), schoolID)

	// If route precedence is wrong, Echo interprets "export" as :session_id
	// and returns 400 "invalid session_id" instead of CSV.
	rec := getRequest(t, env.e, "/api/v1/admin/results/export?exam_id="+examID.String(), token)
	if rec.Code == http.StatusBadRequest {
		body := rec.Body.String()
		t.Fatalf("route precedence broken: /export matched detail handler (400: %s)", body)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/csv" {
		t.Errorf("Content-Type: want text/csv, got %q", ct)
	}
}
