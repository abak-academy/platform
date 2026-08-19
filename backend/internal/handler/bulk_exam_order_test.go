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

// bulkExamRBACEnv holds the full stack for testing RBAC on bulk-order routes.
type bulkExamRBACEnv struct {
	e      *echo.Echo
	h      *handler.Handler
	mr     *miniredis.Miniredis
	pool   *pgxpool.Pool
	signer *infra.JWTSigner
}

var (
	bulkRBACOnce sync.Once
	bulkRBACEnv  *bulkExamRBACEnv
)

func newBulkExamRBACEnv(t *testing.T) *bulkExamRBACEnv {
	t.Helper()
	bulkRBACOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_bulk_rbac_test"),
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

		// Register bulk-exam-order routes with middleware
		v1 := e.Group("/api/v1")
		admin := v1.Group("/admin")
		admin.Use(handler.JWTMiddleware(svc, signer))
		bulkOrders := admin.Group("/bulk-exam-orders")
		bulkOrders.Use(handler.RBACMiddleware("bulk-exam-orders:write"))
		bulkOrders.GET("/exams", h.AdminListOrderableExams)
		bulkOrders.POST("/preview", h.AdminPreviewBulkOrder)
		bulkOrders.POST("", h.AdminCreateBulkOrder)
		bulkOrders.POST("/:id/checkout", h.AdminCheckoutBulkOrder)

		bulkRBACEnv = &bulkExamRBACEnv{
			e: e, h: h, mr: mr, pool: pool, signer: signer,
		}
	})
	return bulkRBACEnv
}

func mintBulkToken(t *testing.T, env *bulkExamRBACEnv, userID, role string) string {
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

func TestBulkExamOrderRBAC_NoCapability_403(t *testing.T) {
	env := newBulkExamRBACEnv(t)

	// admin_exam does NOT hold bulk-exam-orders:write
	token := mintBulkToken(t, env, "admin-exam-u1", service.RoleAdminExam)

	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/admin/bulk-exam-orders/exams", ""},
		{http.MethodPost, "/api/v1/admin/bulk-exam-orders/preview", `{"exam_id":"00000000-0000-0000-0000-000000000000"}`},
		{http.MethodPost, "/api/v1/admin/bulk-exam-orders", `{"exam_id":"00000000-0000-0000-0000-000000000000"}`},
		{http.MethodPost, "/api/v1/admin/bulk-exam-orders/00000000-0000-0000-0000-000000000000/checkout", `{}`},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			var req *http.Request
			if rt.body != "" {
				req = httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(rt.method, rt.path, nil)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("want 403 for role %q on %s %s, got %d body=%s",
					service.RoleAdminExam, rt.method, rt.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestBulkExamOrderRBAC_AdminSchool_CanAccessExamsEndpoint(t *testing.T) {
	env := newBulkExamRBACEnv(t)

	token := mintBulkToken(t, env, "admin-school-u1", service.RoleAdminSchool)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/bulk-exam-orders/exams", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	// admin_school holds bulk-exam-orders:write, so the middleware allows it
	if rec.Code != http.StatusOK {
		t.Errorf("want 200 for admin_school on GET /exams, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminPreviewBulkOrder_SuperAdmin_StudentIDs_NoSchoolID(t *testing.T) {
	env := newBulkExamRBACEnv(t)
	token := mintBulkToken(t, env, "super-preview-1", service.RoleSuperAdmin)

	body := `{"exam_id":"00000000-0000-0000-0000-000000000001","student_ids":["00000000-0000-0000-0000-000000000002"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/bulk-exam-orders/preview", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "school_id is required") {
		t.Fatalf("super_admin preview with student_ids must not require school_id: %s", rec.Body.String())
	}
	if rec.Code == http.StatusBadRequest {
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["message"] == "school_id is required" {
			t.Fatalf("want no school_id required, got %s", rec.Body.String())
		}
	}
}

func TestAdminPreviewBulkOrder_SuperAdmin_All_NoSchoolID_400(t *testing.T) {
	env := newBulkExamRBACEnv(t)
	token := mintBulkToken(t, env, "super-preview-2", service.RoleSuperAdmin)

	body := `{"exam_id":"00000000-0000-0000-0000-000000000001","all":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/bulk-exam-orders/preview", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 when all=true without school_id, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "school_id is required") {
		t.Errorf("want school_id is required for all=true, got %s", rec.Body.String())
	}
}
