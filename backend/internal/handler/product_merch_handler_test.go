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

var (
	adminProductDBOnce sync.Once
	adminProductDBEnv  *adminProductDBTestEnv
)

type adminProductDBTestEnv struct {
	pool        *pgxpool.Pool
	pgContainer *tcpostgres.PostgresContainer
	rdb         *redis.Client
	e           *echo.Echo
	signer      *infra.JWTSigner
	mr          *miniredis.Miniredis
}

func newAdminProductDBEnv(t *testing.T) *adminProductDBTestEnv {
	t.Helper()
	adminProductDBOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_product_test"),
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

		v1 := e.Group("/api/v1")
		admin := v1.Group("/admin")
		admin.Use(handler.JWTMiddleware(svc, signer))
		adminProducts := admin.Group("/products")
		adminProducts.POST("", h.AdminCreateProduct)
		adminProducts.PATCH("/:id", h.AdminUpdateProduct)

		adminProductDBEnv = &adminProductDBTestEnv{
			pool:        pool,
			pgContainer: pgContainer,
			rdb:         rdb,
			e:           e,
			signer:      signer,
			mr:          mr,
		}
	})
	if adminProductDBEnv == nil {
		t.Fatal("admin product test env failed to initialize")
	}
	return adminProductDBEnv
}

func mintProductToken(t *testing.T, env *adminProductDBTestEnv, userID, role string) string {
	t.Helper()
	tokenString, jti, err := env.signer.SignAccess(userID, role, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := env.rdb.Set(context.Background(), "session:access:"+jti, userID, 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}
	return tokenString
}

// FR9: create merchandise carrying weight_grams + image_url; both round-trip.
func TestAdminCreateProduct_AdminStore_Merchandise_RoundTripsWeightAndImage(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "store1", service.RoleAdminStore)

	rec := postJSONWithToken(t, env.e, "/api/v1/admin/products", token, mustJSON(t, map[string]any{
		"type":         "merchandise",
		"name":         "Academy Tote",
		"price":        50000,
		"stock":        20,
		"weight_grams": 350,
		"image_url":    "avatars/store1/tote.png",
	}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["weight_grams"]; got != float64(350) {
		t.Errorf("weight_grams: want 350, got %v", got)
	}
	if got := resp["image_url"]; got != "avatars/store1/tote.png" {
		t.Errorf("image_url: want avatars/store1/tote.png, got %v", got)
	}
}

// FR8: admin_exam is forbidden from creating merchandise.
func TestAdminCreateProduct_AdminExam_Merchandise_Returns403(t *testing.T) {
	env := newTestEnv(t)

	rdb := redis.NewClient(&redis.Options{Addr: env.mr.Addr()})
	tokenString, jti, err := env.signer.SignAccess("admin_exam_user", service.RoleAdminExam, nil, []string{})
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}
	if err := rdb.Set(context.Background(), "session:access:"+jti, "admin_exam_user", 15*time.Minute).Err(); err != nil {
		t.Fatalf("redis set session: %v", err)
	}

	rec := postJSONWithToken(t, env.e, "/api/v1/admin/products", tokenString, mustJSON(t, map[string]any{
		"type": "merchandise",
		"name": "Academy Tote",
	}))

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateProduct_WeightGramsZero_PersistsAndPreservesOmittedImage(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "store-weight-zero", service.RoleAdminStore)

	created := postJSONWithToken(t, env.e, "/api/v1/admin/products", token, mustJSON(t, map[string]any{
		"type": "merchandise", "name": "Zero Weight Tote", "price": 50000, "stock": 20,
		"weight_grams": 350, "image_url": "avatars/store-weight-zero/tote.png",
	}))
	if created.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d body=%s", created.Code, created.Body.String())
	}
	var product map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &product); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := product["id"].(string)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/"+id, strings.NewReader(mustJSON(t, map[string]any{
		"name": "Zero Weight Tote", "price": 50000, "stock": 20, "weight_grams": 0,
	})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	updated := httptest.NewRecorder()
	env.e.ServeHTTP(updated, req)
	if updated.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d body=%s", updated.Code, updated.Body.String())
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &product); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if got := product["weight_grams"]; got != float64(0) {
		t.Errorf("weight_grams: want 0, got %v", got)
	}
	if got := product["image_url"]; got != "avatars/store-weight-zero/tote.png" {
		t.Errorf("image_url: want preserved value, got %v", got)
	}

	persisted, err := repository.New(env.pool).GetProductByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get persisted product: %v", err)
	}
	if persisted.WeightGrams != 0 {
		t.Errorf("persisted weight_grams: want 0, got %d", persisted.WeightGrams)
	}
}

func TestAdminUpdateProduct_EmptyImageURL_ClearsAndPreservesWeight(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "store-clear-image", service.RoleAdminStore)

	created := postJSONWithToken(t, env.e, "/api/v1/admin/products", token, mustJSON(t, map[string]any{
		"type": "merchandise", "name": "Image Clear Tote", "price": 50000, "stock": 20,
		"weight_grams": 350, "image_url": "avatars/store-clear-image/tote.png",
	}))
	if created.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d body=%s", created.Code, created.Body.String())
	}
	var product map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &product); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := product["id"].(string)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/"+id, strings.NewReader(mustJSON(t, map[string]any{
		"name": "Image Clear Tote", "price": 50000, "stock": 20, "image_url": "",
	})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	updated := httptest.NewRecorder()
	env.e.ServeHTTP(updated, req)
	if updated.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d body=%s", updated.Code, updated.Body.String())
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &product); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if got := product["image_url"]; got != "" {
		t.Errorf("image_url: want cleared value, got %v", got)
	}
	if got := product["weight_grams"]; got != float64(350) {
		t.Errorf("weight_grams: want preserved value, got %v", got)
	}

	persisted, err := repository.New(env.pool).GetProductByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get persisted product: %v", err)
	}
	if persisted.ImageURL != "" {
		t.Errorf("persisted image_url: want cleared value, got %q", persisted.ImageURL)
	}
	if persisted.WeightGrams != 350 {
		t.Errorf("persisted weight_grams: want 350, got %d", persisted.WeightGrams)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
