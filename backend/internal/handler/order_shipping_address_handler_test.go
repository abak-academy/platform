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

// This file proves the fix for the shipping_address wiring gap: PatchCart used
// to bind shipping_address as []byte, which encoding/json only accepts as a
// base64 string, so posting the JSON object the frontend sends returned 400
// and the address was never persisted. See order.go, store.go, order.go
// (repository), and order.go (model) for the json.RawMessage fix.

var (
	orderCartDBOnce sync.Once
	orderCartDBEnv  *orderCartDBTestEnv
)

type orderCartDBTestEnv struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	e      *echo.Echo
	signer *infra.JWTSigner
}

func newOrderCartDBEnv(t *testing.T) *orderCartDBTestEnv {
	t.Helper()
	orderCartDBOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_cart_test"),
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
		orders := v1.Group("/orders")
		orders.Use(handler.JWTMiddleware(svc, signer))
		orders.POST("", h.MintCart)
		orders.GET("/:id", h.GetOrder)
		orders.PATCH("/:id", h.PatchCart)

		orderCartDBEnv = &orderCartDBTestEnv{
			pool:   pool,
			rdb:    rdb,
			e:      e,
			signer: signer,
		}
	})
	if orderCartDBEnv == nil {
		t.Fatal("order cart test env failed to initialize")
	}
	return orderCartDBEnv
}

func mintCartToken(t *testing.T, env *orderCartDBTestEnv, userID, role string) string {
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

func insertCartStudent(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	email := "cart-student-" + time.Now().Format("150405.000000") + "@test.local"
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, $2, $3) RETURNING id`,
		email, service.RoleStudent, "Cart Student",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}
	return id
}

// TestPatchCart_ShippingAddressObject_BindsAndPersists proves the wiring
// gap is closed end to end: a JSON-object shipping_address (as the frontend
// sends it) binds without a 400, persists into the orders.shipping_address
// JSONB column intact, and reads back through GetOrder as the same object
// rather than a base64-encoded string.
func TestPatchCart_ShippingAddressObject_BindsAndPersists(t *testing.T) {
	env := newOrderCartDBEnv(t)
	studentID := insertCartStudent(t, env.pool)
	token := mintCartToken(t, env, studentID, service.RoleStudent)

	mintRec := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	mintRec.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, mintRec)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("mint cart: want 200/201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var order map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &order); err != nil {
		t.Fatalf("decode mint response: %v", err)
	}
	orderID := order["id"].(string)

	body := mustJSON(t, map[string]any{
		"shipping_address": map[string]any{
			"penerima":     "Budi Santoso",
			"telepon":      "081234567890",
			"alamat":       "Jl. Merdeka No. 1",
			"kode_pos":     "40123",
			"provinsi_id":  "32",
			"kota_id":      "3273",
			"kecamatan_id": "327301",
		},
	})
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/orders/"+orderID, strings.NewReader(body))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchRec := httptest.NewRecorder()
	env.e.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch cart: want 200, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	// Persisted JSONB column must contain the actual address fields, not a
	// base64-encoded blob.
	var raw []byte
	if err := env.pool.QueryRow(context.Background(),
		`SELECT shipping_address FROM orders WHERE id = $1`, orderID,
	).Scan(&raw); err != nil {
		t.Fatalf("query persisted shipping_address: %v", err)
	}
	var persisted map[string]string
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("persisted shipping_address is not a JSON object: %v (raw=%s)", err, raw)
	}
	if persisted["penerima"] != "Budi Santoso" {
		t.Errorf("persisted penerima: want %q, got %q", "Budi Santoso", persisted["penerima"])
	}
	if persisted["alamat"] != "Jl. Merdeka No. 1" {
		t.Errorf("persisted alamat: want %q, got %q", "Jl. Merdeka No. 1", persisted["alamat"])
	}

	// GetOrder must serialize shipping_address back as the same JSON object,
	// not a base64 string, so the order-detail page can render it directly.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	env.e.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get order: want 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getResp map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get order response: %v", err)
	}
	addr, ok := getResp["shipping_address"].(map[string]any)
	if !ok {
		t.Fatalf("shipping_address in GET response is not an object: %T (%v)", getResp["shipping_address"], getResp["shipping_address"])
	}
	if addr["penerima"] != "Budi Santoso" {
		t.Errorf("GET shipping_address.penerima: want %q, got %v", "Budi Santoso", addr["penerima"])
	}
}
