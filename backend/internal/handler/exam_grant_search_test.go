package handler_test

import (
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// ---------------------------------------------------------------------------
// RBAC tests for AdminSearchGrantStudents (FR-SEARCH-02)
// ---------------------------------------------------------------------------

func TestAdminSearchGrantStudents_RBAC(t *testing.T) {
	env := newTestEnv(t)
	h := handler.New(env.svc)

	admin := env.e.Group("/api/v1/admin")
	admin.Use(handler.JWTMiddleware(env.svc, env.signer))
	examGrants := admin.Group("/exam-grants")
	examGrants.Use(handler.RBACMiddleware("exam-grants:write"))
	examGrants.GET("/students/search", h.AdminSearchGrantStudents)

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search", nil)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-super_admin role gets 403", func(t *testing.T) {
		token := mintAccessToken(t, env, "admin-store-1", service.RoleAdminStore, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp["code"] != "forbidden" {
			t.Errorf("code: want forbidden, got %s", resp["code"])
		}
	})

	t.Run("admin_school role gets 403", func(t *testing.T) {
		schoolID := "00000000-0000-0000-0000-000000000001"
		token := mintAccessToken(t, env, "admin-school-1", service.RoleAdminSchool, &schoolID)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

// ---------------------------------------------------------------------------
// DB-backed functional tests for AdminSearchGrantStudents
// ---------------------------------------------------------------------------

var (
	searchGrantDBOnce sync.Once
	searchGrantDBEnv  *searchGrantDBTestEnv
)

type searchGrantDBTestEnv struct {
	pool        *pgxpool.Pool
	pgContainer *tcpostgres.PostgresContainer
	rdb         *redis.Client
	e           *echo.Echo
	svc         *service.Service
	signer      *infra.JWTSigner
	mr          *miniredis.Miniredis
}

func newSearchGrantDBEnv(t *testing.T) *searchGrantDBTestEnv {
	t.Helper()
	searchGrantDBOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_search_grant_test"),
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

		// Register the exam-grant search route.
		v1 := e.Group("/api/v1")
		admin := v1.Group("/admin")
		admin.Use(handler.JWTMiddleware(svc, signer))
		adminExamGrants := admin.Group("/exam-grants")
		adminExamGrants.Use(handler.RBACMiddleware("exam-grants:write"))
		adminExamGrants.GET("/students/search", h.AdminSearchGrantStudents)

		searchGrantDBEnv = &searchGrantDBTestEnv{
			pool:        pool,
			pgContainer: pgContainer,
			rdb:         rdb,
			e:           e,
			svc:         svc,
			signer:      signer,
			mr:          mr,
		}
	})
	if searchGrantDBEnv == nil {
		t.Fatal("search grant test env failed to initialize")
	}
	return searchGrantDBEnv
}

func mintSearchSuperAdminToken(t *testing.T, env *searchGrantDBTestEnv, userID string) string {
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

func seedSchoolForSearch(t *testing.T, pool *pgxpool.Pool, name, code string, schoolTypes []string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO school (name, code, school_types) VALUES ($1, $2, $3) RETURNING id`,
		name, code, schoolTypes,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert school %s: %v", name, err)
	}
	return id
}

func seedStudentForSearch(t *testing.T, pool *pgxpool.Pool, name, username, schoolID, jenjang string, grade int) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (name, username, role, school_id, status, jenjang, grade, otp_enabled)
		 VALUES ($1, $2, 'student', $3, 'active', $4, $5, false)`,
		name, username, schoolID, jenjang, grade,
	)
	if err != nil {
		t.Fatalf("insert student %s: %v", name, err)
	}
}

func TestAdminSearchGrantStudents_SuperAdmin_ReturnsCrossSchoolResults(t *testing.T) {
	env := newSearchGrantDBEnv(t)

	// Seed two schools.
	schoolA := seedSchoolForSearch(t, env.pool, "SMA A", "sma_a", []string{"sma", "sma_ipas"})
	schoolB := seedSchoolForSearch(t, env.pool, "SMA B", "sma_b", []string{"sma", "sma_ips"})

	// Seed students across both schools.
	seedStudentForSearch(t, env.pool, "Alice", "alic0012", schoolA, "sma", 10)
	seedStudentForSearch(t, env.pool, "Bob", "bob_7890", schoolA, "sma", 11)
	seedStudentForSearch(t, env.pool, "Charlie", "char3456", schoolB, "sma", 10)
	seedStudentForSearch(t, env.pool, "Diana", "diana7890", schoolB, "sma", 12)

	superToken := mintSearchSuperAdminToken(t, env, "super1")

	t.Run("no filters returns students from all schools", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search", nil)
		req.Header.Set("Authorization", "Bearer "+superToken)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok {
			t.Fatal("data is not an array")
		}
		if len(data) != 4 {
			t.Fatalf("want 4 students, got %d", len(data))
		}
		// Verify school_name is populated.
		for _, item := range data {
			row := item.(map[string]any)
			if row["school_name"] == "" {
				t.Errorf("student %v has empty school_name", row["name"])
			}
			if row["school_id"] == "" {
				t.Errorf("student %v has empty school_id", row["name"])
			}
		}
		// Verify both school names appear.
		names := make(map[string]bool)
		for _, item := range data {
			row := item.(map[string]any)
			names[row["school_name"].(string)] = true
		}
		if !names["SMA A"] {
			t.Error("SMA A not in results")
		}
		if !names["SMA B"] {
			t.Error("SMA B not in results")
		}
	})

	t.Run("school_id filter narrows to one school", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search?school_id="+schoolA, nil)
		req.Header.Set("Authorization", "Bearer "+superToken)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok {
			t.Fatal("data is not an array")
		}
		if len(data) != 2 {
			t.Fatalf("want 2 students from school A, got %d", len(data))
		}
		for _, item := range data {
			row := item.(map[string]any)
			if row["school_id"] != schoolA {
				t.Errorf("student %v has wrong school_id %v", row["name"], row["school_id"])
			}
		}
	})

	t.Run("q filter searches name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search?q=Ali", nil)
		req.Header.Set("Authorization", "Bearer "+superToken)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok {
			t.Fatal("data is not an array")
		}
		if len(data) != 1 {
			t.Fatalf("want 1 student matching 'Ali', got %d", len(data))
		}
		name := data[0].(map[string]any)["name"].(string)
		if !strings.Contains(name, "Ali") {
			t.Errorf("expected name containing 'Ali', got %s", name)
		}
	})

	t.Run("grade filter narrows results", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search?grade=10", nil)
		req.Header.Set("Authorization", "Bearer "+superToken)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok {
			t.Fatal("data is not an array")
		}
		if len(data) != 2 {
			t.Fatalf("want 2 students with grade 10, got %d", len(data))
		}
	})

	t.Run("super_admin with malformed school_id gets 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search?school_id=not-a-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+superToken)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bounded pagination with limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/exam-grants/students/search?limit=2", nil)
		req.Header.Set("Authorization", "Bearer "+superToken)
		rec := httptest.NewRecorder()
		env.e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok {
			t.Fatal("data is not an array")
		}
		if len(data) != 2 {
			t.Fatalf("want 2 students (limit=2), got %d", len(data))
		}
		nextCursor, ok := resp["next_cursor"].(string)
		if !ok || nextCursor == "" {
			t.Error("expected non-empty next_cursor when results exceed limit")
		}
	})
}
