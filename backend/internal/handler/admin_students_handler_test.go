package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setAdminSchoolClaims sets admin_school claims with a schoolID on the echo context.
func setAdminSchoolClaims(c echo.Context, sub, schoolID string) {
	c.Set("claims", &infra.Claims{Sub: sub, Role: "admin_school", SchoolID: &schoolID})
}

// setAdminSchoolClaimsNil sets admin_school claims with nil schoolID.
func setAdminSchoolClaimsNil(c echo.Context, sub string) {
	c.Set("claims", &infra.Claims{Sub: sub, Role: "admin_school", SchoolID: nil})
}

func TestAdminListStudents_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/students", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaimsNil(c, "u1")

	err := env.h.AdminListStudents(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d", rec.Code)
	}
}

func TestAdminRegisterStudent_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"name": "Test", "nis": "12345"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaimsNil(c, "u1")

	err := env.h.AdminRegisterStudent(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d", rec.Code)
	}
}

func TestAdminRegisterStudent_MissingFields(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"nis": "12345"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminRegisterStudent(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing name, got %d", rec.Code)
	}
}

func TestAdminRegisterStudent_MissingNIS(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"name": "Test Student"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminRegisterStudent(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing nis, got %d", rec.Code)
	}
}

func TestAdminChangeStudentStatus_InvalidStatus(t *testing.T) {
	env := newAdminSystemEnv(t)

	body := map[string]string{"status": "deleted"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/admin/students/stu1", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("stu1")
	setAdminSchoolClaims(c, "u1", "s1")

	err := env.h.AdminChangeStudentStatus(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid status, got %d", rec.Code)
	}
}

func TestAdminGetStudentCredentials_NilSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/students/stu1/credentials", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("stu1")
	setAdminSchoolClaimsNil(c, "u1")

	err := env.h.AdminGetStudentCredentials(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for nil schoolID, got %d", rec.Code)
	}
}

func TestAdminListStudents_AdminSchool_DifferentSchoolID_403(t *testing.T) {
	env := newAdminSystemEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/students?school_id=s2", nil)
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1") // admin_school with JWT school_id=s1, but query says s2

	if err := env.h.AdminListStudents(c); err != nil {
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

func TestAdminListStudents_AdminSchool_SameSchoolID_Ignored(t *testing.T) {
	env := newAdminSystemEnv(t)

	// admin_school should ignore a school_id param matching their own scope.
	// This test uses RegisterStudent (enters the resolver path, then validates
	// request body — missing name means 400, not 403).
	body := map[string]string{"nis": "12345"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/students?school_id=s1", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := env.e.NewContext(req, rec)
	setAdminSchoolClaims(c, "u1", "s1")

	if err := env.h.AdminRegisterStudent(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code == http.StatusForbidden {
		t.Errorf("admin_school with matching school_id should NOT get 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	// Should be 400 because name is missing, not 403 from scope rejection.
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing name, got %d", rec.Code)
	}
}

// (Removed) TestAdminListStudents_RBAC_403 asserted that a super_admin without
// school_id is rejected — the rule this change deliberately reverses, since the
// roster must list registrants who have no school. Despite its name and comment
// it set super_admin claims, and the admin_school case it described is already
// covered by TestAdminListStudents_NilSchoolID_403 above.

// ---------------------------------------------------------------------------
// DB-backed tests for school scope (students)
// ---------------------------------------------------------------------------

var (
	adminStuDBOnce sync.Once
	adminStuDBEnv  *adminStuDBTestEnv
)

type adminStuDBTestEnv struct {
	pool        *pgxpool.Pool
	pgContainer *tcpostgres.PostgresContainer
	rdb         *redis.Client
	e           *echo.Echo
	svc         *service.Service
	signer      *infra.JWTSigner
	mr          *miniredis.Miniredis
}

func newAdminStuDBEnv(t *testing.T) *adminStuDBTestEnv {
	t.Helper()
	adminStuDBOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_admin_stu_test"),
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
			nil,
		)
		e := echo.New()
		e.HideBanner = true
		h := handler.New(svc)

		// Register admin students routes.
		v1 := e.Group("/api/v1")
		admin := v1.Group("/admin")
		admin.Use(handler.JWTMiddleware(svc, signer))
		adminStudents := admin.Group("/students")
		adminStudents.Use(handler.RBACMiddleware("students:*"))
		adminStudents.GET("", h.AdminListStudents)
		adminStudents.POST("", h.AdminRegisterStudent)
		adminStudents.PATCH("/:id/password", h.AdminSetStudentPassword, handler.RBACMiddleware("system:admin"))
		adminStudents.PATCH("/:id", h.AdminChangeStudentStatus)
		adminStudents.GET("/:id/credentials", h.AdminGetStudentCredentials)
		adminStudents.POST("/bulk/presign", h.AdminPresignStudentBulkUpload)
		adminStudents.POST("/bulk", h.AdminBulkImportStudents)
		adminStudents.POST("/bulk/credentials", h.AdminBulkReissueCredentials)

		adminStuDBEnv = &adminStuDBTestEnv{
			pool:        pool,
			pgContainer: pgContainer,
			rdb:         rdb,
			e:           e,
			svc:         svc,
			signer:      signer,
			mr:          mr,
		}
	})
	if adminStuDBEnv == nil {
		t.Fatal("admin students test env failed to initialize")
	}
	return adminStuDBEnv
}

func seedSchoolForStu(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO school (name, code) VALUES ($1, $2) RETURNING id`,
		"Stu School", "stu_"+time.Now().Format("150405"),
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert school: %v", err)
	}
	return id
}

func mintSuperAdminStuToken(t *testing.T, env *adminStuDBTestEnv, userID string) string {
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

func mintAdminStuToken(t *testing.T, env *adminStuDBTestEnv, userID, role string, schoolID *string) string {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	tokenString, jti, err := env.signer.SignAccess(userID, role, schoolID, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(context.Background(), "session:access:"+jti, userID, 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}
	return tokenString
}

func TestAdminListStudents_SuperAdmin_NonexistentSchool_404(t *testing.T) {
	env := newAdminStuDBEnv(t)

	superToken := mintSuperAdminStuToken(t, env, "super1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students?school_id="+"00000000-0000-0000-0000-000000000000", nil)
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for nonexistent school, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "not_found" {
		t.Errorf("code: want not_found, got %v", resp["code"])
	}
}

func TestAdminListStudents_SuperAdmin_MalformedSchoolID_400(t *testing.T) {
	env := newAdminStuDBEnv(t)

	superToken := mintSuperAdminStuToken(t, env, "super1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students?school_id=not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_request" {
		t.Errorf("code: want invalid_request, got %v", resp["code"])
	}
}

func TestAdminListStudents_SuperAdmin_ValidSchool_200(t *testing.T) {
	env := newAdminStuDBEnv(t)

	schoolID := seedSchoolForStu(t, env.pool)
	superToken := mintSuperAdminStuToken(t, env, "super2")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students?school_id="+schoolID, nil)
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["data"] == nil {
		t.Error("want non-nil data array")
	}
}

func TestAdminListStudents_AdminSchool_OwnScope_200(t *testing.T) {
	env := newAdminStuDBEnv(t)

	schoolID := seedSchoolForStu(t, env.pool)

	// Mint an admin_school token scoped to the school.
	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	tokenString, jti, err := env.signer.SignAccess("admin1", "admin_school", &schoolID, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(context.Background(), "session:access:"+jti, "admin1", 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for admin_school with own scope, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["data"] == nil {
		t.Error("want non-nil data array")
	}
}

func TestAdminListStudents_ExamEligibilityAndCursorValidation(t *testing.T) {
	env := newAdminStuDBEnv(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	var schoolID string
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO school (name, code, school_types) VALUES ($1, $2, $3) RETURNING id`,
		"Eligibility "+suffix, "as_"+suffix, []string{"sma"},
	).Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	seed := func(name, status string) string {
		var id string
		if err := env.pool.QueryRow(ctx,
			`INSERT INTO users (name, username, role, school_id, status, jenjang, grade, otp_enabled)
			 VALUES ($1, $2, 'student', $3, $4, 'sma', 10, false) RETURNING id`,
			name+" "+suffix, "as_"+uuid.NewString()[:12], schoolID, status,
		).Scan(&id); err != nil {
			t.Fatalf("seed student: %v", err)
		}
		return id
	}
	eligible := seed("Eligible", "active")
	registered := seed("Registered", "active")
	seed("Deactivated", "deactivated")
	var examID string
	if err := env.pool.QueryRow(ctx, `INSERT INTO exam (title) VALUES ($1) RETURNING id`, "Eligibility "+suffix).Scan(&examID); err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	if _, err := env.pool.Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
		registered, examID, "as_"+uuid.NewString(),
	); err != nil {
		t.Fatalf("seed registration: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	token, jti, err := env.signer.SignAccess("admin-eligibility", "admin_school", &schoolID, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(ctx, "session:access:"+jti, "admin-eligibility", 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}

	t.Run("exam context composes with school grade and jenjang", func(t *testing.T) {
		path := "/api/v1/admin/students?q=" + suffix + "&grade=10&jenjang=sma&exam_id=" + examID
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Data []struct {
				ID      string `json:"id"`
				Jenjang string `json:"jenjang"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].ID != eligible {
			t.Fatalf("want only eligible student %s, got %+v", eligible, resp.Data)
		}
		if resp.Data[0].Jenjang != "sma" {
			t.Fatalf("want jenjang sma, got %q", resp.Data[0].Jenjang)
		}
	})

	t.Run("malformed cursor returns invalid_cursor", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students?cursor=garbage", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		var apiErr struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if apiErr.Code != "invalid_cursor" {
			t.Errorf("code: want invalid_cursor, got %s", apiErr.Code)
		}
	})
}

// Registrants are not all school pupils — university students and members of
// the public sign up too — so super_admin's roster must show everyone,
// including those with no school on file, and must carry the school name so
// operations can see whose school still needs confirming.
func TestAdminListStudents_SuperAdmin_NoSchoolID_ListsEveryRegistrant(t *testing.T) {
	env := newAdminStuDBEnv(t)
	token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-0000000000b1")
	ctx := context.Background()

	schoolID := seedSchoolForStu(t, env.pool)
	suffix := time.Now().Format("150405.000000")

	// One student linked to a school, one with no school at all.
	var withSchoolID, noSchoolID string
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO users (username, name, role, status, school_id, password_hash)
		 VALUES ($1, 'Siswa Sekolah', 'student', 'active', $2, 'x') RETURNING id`,
		"stu_with_"+suffix, schoolID,
	).Scan(&withSchoolID); err != nil {
		t.Fatalf("seed student with school: %v", err)
	}
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO users (username, name, role, status, school_id, unlisted_school_name, password_hash)
		 VALUES ($1, 'Peserta Umum', 'student', 'active', NULL, 'Universitas Contoh', 'x') RETURNING id`,
		"stu_none_"+suffix,
	).Scan(&noSchoolID); err != nil {
		t.Fatalf("seed student without school: %v", err)
	}

	// username is nullable and really is NULL for some accounts; scanning it
	// into a plain string 500s the whole roster over one row.
	// users_check requires email OR username, so this mirrors a real account:
	// email present, username NULL.
	var noUsernameID string
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO users (username, email, name, role, status, school_id, password_hash)
		 VALUES (NULL, $1, 'Tanpa Username', 'student', 'active', NULL, 'x') RETURNING id`,
		"nouser_"+suffix+"@example.com",
	).Scan(&noUsernameID); err != nil {
		t.Fatalf("seed student without username: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students?limit=100", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for super_admin without school_id, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			ID                 string  `json:"id"`
			Username           *string `json:"username"`
			SchoolName         *string `json:"school_name"`
			UnlistedSchoolName *string `json:"unlisted_school_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	seen := map[string]int{}
	for i, s := range resp.Data {
		seen[s.ID] = i
	}
	iWith, okWith := seen[withSchoolID]
	iNone, okNone := seen[noSchoolID]
	if !okWith || !okNone {
		t.Fatalf("roster must include both the school-linked and school-less student; got %d rows", len(resp.Data))
	}
	if resp.Data[iWith].SchoolName == nil || *resp.Data[iWith].SchoolName != "Stu School" {
		t.Errorf("school-linked student should carry its school name, got %v", resp.Data[iWith].SchoolName)
	}
	if resp.Data[iNone].SchoolName != nil {
		t.Errorf("school-less student should have a null school_name, got %v", *resp.Data[iNone].SchoolName)
	}
	if resp.Data[iNone].UnlistedSchoolName == nil || *resp.Data[iNone].UnlistedSchoolName != "Universitas Contoh" {
		t.Errorf("self-entered school should be surfaced for follow-up, got %v", resp.Data[iNone].UnlistedSchoolName)
	}
	if _, ok := seen[noUsernameID]; !ok {
		t.Error("a student with a NULL username must not be dropped, nor 500 the roster")
	}
}

// A school can be confirmed after the fact, so registration must succeed
// without one and leave school_id NULL rather than failing or writing "".
func TestAdminRegisterStudent_SuperAdmin_WithoutSchool(t *testing.T) {
	env := newAdminStuDBEnv(t)
	token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-0000000000b2")

	body := `{"name":"Peserta IELTS","jenjang":"SMA"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/students", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("registering without a school should succeed, got %d body=%s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var schoolID *string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT school_id FROM users WHERE id = $1`, created.ID,
	).Scan(&schoolID); err != nil {
		t.Fatalf("read back student: %v", err)
	}
	if schoolID != nil {
		t.Errorf("school_id should be NULL when no school was given, got %q", *schoolID)
	}
}

func TestAdminRegisterStudent_ExplicitPassword(t *testing.T) {
	env := newAdminStuDBEnv(t)
	ctx := context.Background()
	schoolID := seedSchoolForStu(t, env.pool)
	password := "chosenPass123"

	t.Run("super_admin explicit password returns no temp_password and stores hash", func(t *testing.T) {
		token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-00000000e101")
		body := `{"name":"Explicit Handler","jenjang":"SMA","password":"` + password + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/students?school_id="+schoolID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), password) || strings.Contains(rec.Body.String(), "temp_password") {
			t.Fatalf("explicit-password response leaked secret or temp_password: %s", rec.Body.String())
		}
		var resp struct {
			ID           string  `json:"id"`
			Username     string  `json:"username"`
			TempPassword *string `json:"temp_password"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Username == "" || resp.TempPassword != nil {
			t.Fatalf("unexpected response: %+v", resp)
		}
		var hash string
		if err := env.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, resp.ID).Scan(&hash); err != nil {
			t.Fatalf("read hash: %v", err)
		}
		if hash == password || !strings.HasPrefix(hash, "$2") {
			t.Fatalf("password was not stored as bcrypt hash: %q", hash)
		}
	})

	t.Run("super_admin omitted password keeps generated temp_password", func(t *testing.T) {
		token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-00000000e102")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/students?school_id="+schoolID, strings.NewReader(`{"name":"Generated Handler","jenjang":"SMA"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			TempPassword string `json:"temp_password"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.TempPassword == "" {
			t.Fatal("omitted password should preserve generated temp_password")
		}
	})

	t.Run("admin_school explicit password is forbidden and inserts nothing", func(t *testing.T) {
		token := mintAdminStuToken(t, env, "00000000-0000-0000-0000-00000000e103", service.RoleAdminSchool, &schoolID)
		name := "Forbidden Handler " + uuid.NewString()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/students", strings.NewReader(`{"name":"`+name+`","jenjang":"SMA","password":"`+password+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), password) {
			t.Fatalf("forbidden response leaked password: %s", rec.Body.String())
		}
		var count int
		if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE name = $1`, name).Scan(&count); err != nil {
			t.Fatalf("count users: %v", err)
		}
		if count != 0 {
			t.Fatalf("forbidden create inserted %d rows", count)
		}
	})

	t.Run("admin_school omitted password keeps legacy create", func(t *testing.T) {
		token := mintAdminStuToken(t, env, "00000000-0000-0000-0000-00000000e104", service.RoleAdminSchool, &schoolID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/students", strings.NewReader(`{"name":"Scoped Generated Handler","jenjang":"SMA"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			TempPassword string `json:"temp_password"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.TempPassword == "" {
			t.Fatal("scoped omitted password should preserve generated temp_password")
		}
	})
}

func TestAdminSetStudentPassword(t *testing.T) {
	env := newAdminStuDBEnv(t)
	ctx := context.Background()
	schoolID := seedSchoolForStu(t, env.pool)
	reg, err := env.svc.RegisterStudent(ctx, schoolID, "Set Password Handler", "SMA", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}
	var originalHash string
	if err := env.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, reg.ID).Scan(&originalHash); err != nil {
		t.Fatalf("read original hash: %v", err)
	}

	t.Run("unauthenticated is 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/students/"+reg.ID+"/password", strings.NewReader(`{"new_password":"manualNew123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	for _, tc := range []struct {
		name string
		role string
	}{
		{"admin_school", service.RoleAdminSchool},
		{"admin_store", service.RoleAdminStore},
		{"student", service.RoleStudent},
	} {
		t.Run(tc.name+" is 403", func(t *testing.T) {
			token := mintAdminStuToken(t, env, "00000000-0000-0000-0000-"+uuid.NewString()[24:], tc.role, &schoolID)
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/students/"+reg.ID+"/password", strings.NewReader(`{"new_password":"manualNew123"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
			}
			var hash string
			if err := env.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, reg.ID).Scan(&hash); err != nil {
				t.Fatalf("read hash: %v", err)
			}
			if hash != originalHash {
				t.Fatal("denied request mutated hash")
			}
		})
	}

	for _, tc := range []struct {
		name string
		path string
		body string
		want int
		code string
	}{
		{"missing password", "/api/v1/admin/students/" + reg.ID + "/password", `{}`, http.StatusBadRequest, "invalid_request"},
		{"blank password", "/api/v1/admin/students/" + reg.ID + "/password", `{"new_password":""}`, http.StatusBadRequest, "invalid_request"},
		{"weak password", "/api/v1/admin/students/" + reg.ID + "/password", `{"new_password":"short"}`, http.StatusBadRequest, "invalid_request"},
		{"malformed id", "/api/v1/admin/students/not-a-uuid/password", `{"new_password":"manualNew123"}`, http.StatusBadRequest, "invalid_request"},
		{"missing target", "/api/v1/admin/students/00000000-0000-0000-0000-000000000000/password", `{"new_password":"manualNew123"}`, http.StatusNotFound, "student_not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-"+uuid.NewString()[24:])
			req := httptest.NewRequest(http.MethodPatch, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("want %d, got %d body=%s", tc.want, rec.Code, rec.Body.String())
			}
			var resp struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.Code != tc.code {
				t.Fatalf("want code %s, got %s", tc.code, resp.Code)
			}
		})
	}

	t.Run("super_admin succeeds with generic secret-free response", func(t *testing.T) {
		newPassword := "manualNew123"
		token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-00000000e201")
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/students/"+reg.ID+"/password", strings.NewReader(`{"new_password":"`+newPassword+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if strings.TrimSpace(rec.Body.String()) != `{"message":"password updated"}` {
			t.Fatalf("want exact generic response, got %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), newPassword) || strings.Contains(rec.Body.String(), reg.Username) {
			t.Fatalf("response leaked secret fields: %s", rec.Body.String())
		}
	})
}

func TestAdminGetStudentCredentials_SuperAdmin_NoSchoolID_200(t *testing.T) {
	env := newAdminStuDBEnv(t)
	ctx := context.Background()
	schoolID := seedSchoolForStu(t, env.pool)
	reg, err := env.svc.RegisterStudent(ctx, schoolID, "Cred Super", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}

	token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-0000000000c1")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/students/"+reg.ID+"/credentials", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 without school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "school_id is required") {
		t.Fatalf("super_admin credentials must not require school_id: %s", rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["temp_password"] == nil || resp["temp_password"] == "" {
		t.Errorf("want temp_password, got %v", resp)
	}
}

func TestAdminChangeStudentStatus_SuperAdmin_NoSchoolID_200(t *testing.T) {
	env := newAdminStuDBEnv(t)
	ctx := context.Background()
	schoolID := seedSchoolForStu(t, env.pool)
	reg, err := env.svc.RegisterStudent(ctx, schoolID, "Status Super", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("RegisterStudent: %v", err)
	}

	token := mintSuperAdminStuToken(t, env, "00000000-0000-0000-0000-0000000000c2")
	body := `{"status":"deactivated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/students/"+reg.ID, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 without school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "school_id is required") {
		t.Fatalf("super_admin status must not require school_id: %s", rec.Body.String())
	}
}
