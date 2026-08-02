package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// regionTestDBEnv is a shared singleton DB-backed environment for region handler tests.
var (
	regionDBOnce sync.Once
	regionDBEnv  *regionTestDBEnv
)

type regionTestDBEnv struct {
	pool        *pgxpool.Pool
	pgContainer *tcpostgres.PostgresContainer
	rdb         *redis.Client
	e           *echo.Echo
	svc         *service.Service
	signer      *infra.JWTSigner
	mr          *miniredis.Miniredis
}

func newRegionTestDBEnv(t *testing.T) *regionTestDBEnv {
	t.Helper()
	regionDBOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("akademi_region_handler_test"),
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
			JWTSecret:       "region-test-secret",
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

		// Register only the public region routes.
		v1 := e.Group("/api/v1")
		v1.GET("/provinces", h.ListProvinces)
		v1.GET("/provinces/:id/cities", h.ListCitiesByProvince)
		v1.GET("/cities/:id/districts", h.ListDistrictsByCity)

		regionDBEnv = &regionTestDBEnv{
			pool:        pool,
			pgContainer: pgContainer,
			rdb:         rdb,
			e:           e,
			svc:         svc,
			signer:      signer,
			mr:          mr,
		}
	})
	if regionDBEnv == nil {
		t.Fatal("region test env failed to initialize")
	}
	return regionDBEnv
}

func seedRegionData(t *testing.T, pool *pgxpool.Pool) (sulselID, jatimID, makassarID, surabayaID string) {
	t.Helper()
	ctx := context.Background()

	pool.Exec(ctx, `DELETE FROM district`)
	pool.Exec(ctx, `DELETE FROM city`)
	pool.Exec(ctx, `DELETE FROM province`)

	err := pool.QueryRow(ctx,
		`INSERT INTO province (id, name) VALUES ($1, $2) RETURNING id`,
		"73", "SULAWESI SELATAN",
	).Scan(&sulselID)
	if err != nil {
		t.Fatalf("insert province sulsel: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO province (id, name) VALUES ($1, $2) RETURNING id`,
		"35", "JAWA TIMUR",
	).Scan(&jatimID)
	if err != nil {
		t.Fatalf("insert province jatim: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO city (id, province_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"7371", sulselID, "KOTA MAKASSAR",
	).Scan(&makassarID)
	if err != nil {
		t.Fatalf("insert city makassar: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO city (id, province_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"3578", jatimID, "KOTA SURABAYA",
	).Scan(&surabayaID)
	if err != nil {
		t.Fatalf("insert city surabaya: %v", err)
	}

	// Two districts under Makassar
	for _, d := range []struct{ id, cityID, name string }{
		{"7371010", makassarID, "MARISO"},
		{"7371020", makassarID, "MAMAJANG"},
	} {
		_, err = pool.Exec(ctx,
			`INSERT INTO district (id, city_id, name) VALUES ($1, $2, $3)`,
			d.id, d.cityID, d.name,
		)
		if err != nil {
			t.Fatalf("insert district %s: %v", d.name, err)
		}
	}

	// One district under Surabaya
	_, err = pool.Exec(ctx,
		`INSERT INTO district (id, city_id, name) VALUES ($1, $2, $3)`,
		"3578010", surabayaID, "GENTENG",
	)
	if err != nil {
		t.Fatalf("insert district genteng: %v", err)
	}

	return
}

// GET /api/v1/provinces returns 200 with seeded provinces, no JWT required.
func TestListProvinces_ReturnsSeeded(t *testing.T) {
	env := newRegionTestDBEnv(t)
	sulselID, jatimID, _, _ := seedRegionData(t, env.pool)

	// Also insert DKI JAKARTA to test ordering.
	_, err := env.pool.Exec(context.Background(),
		`INSERT INTO province (id, name) VALUES ($1, $2)`,
		"31", "DKI JAKARTA",
	)
	if err != nil {
		t.Fatalf("insert province dki: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var provinces []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &provinces); err != nil {
		t.Fatalf("unmarshal provinces: %v", err)
	}

	if len(provinces) != 3 {
		t.Fatalf("expected 3 provinces, got %d", len(provinces))
	}
	// Alphabetical: DKI JAKARTA, JAWA TIMUR, SULAWESI SELATAN
	if provinces[0].Name != "DKI JAKARTA" || provinces[0].ID != "31" {
		t.Errorf("first province should be DKI JAKARTA, got %+v", provinces[0])
	}
	if provinces[1].Name != "JAWA TIMUR" || provinces[1].ID != jatimID {
		t.Errorf("second province should be JAWA TIMUR, got %+v", provinces[1])
	}
	if provinces[2].Name != "SULAWESI SELATAN" || provinces[2].ID != sulselID {
		t.Errorf("third province should be SULAWESI SELATAN, got %+v", provinces[2])
	}
}

// GET /api/v1/provinces works without any JWT token.
func TestListProvinces_NoAuth_Returns200(t *testing.T) {
	env := newRegionTestDBEnv(t)
	seedRegionData(t, env.pool)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("public endpoint should return 200 without auth, got %d", rec.Code)
	}
}

// GET /api/v1/provinces/:id/cities returns only matching cities for known province.
func TestListCitiesByProvince_KnownProvince_ReturnsMatchingCities(t *testing.T) {
	env := newRegionTestDBEnv(t)
	sulselID, _, makassarID, _ := seedRegionData(t, env.pool)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/"+sulselID+"/cities", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var cities []struct {
		ID         string `json:"id"`
		ProvinceID string `json:"province_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cities); err != nil {
		t.Fatalf("unmarshal cities: %v", err)
	}

	if len(cities) != 1 {
		t.Fatalf("expected 1 city for sulsel, got %d", len(cities))
	}
	if cities[0].ID != makassarID || cities[0].ProvinceID != sulselID {
		t.Errorf("unexpected city: %+v", cities[0])
	}
}

// GET /api/v1/provinces/:id/cities returns 200 with [] when province_id is unknown.
func TestListCitiesByProvince_UnknownProvince_Returns200WithEmpty(t *testing.T) {
	env := newRegionTestDBEnv(t)
	seedRegionData(t, env.pool)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provinces/does-not-exist/cities", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var cities []any
	if err := json.Unmarshal(rec.Body.Bytes(), &cities); err != nil {
		t.Fatalf("unmarshal cities: %v", err)
	}
	if len(cities) != 0 {
		t.Errorf("expected empty slice for unknown province, got %d items", len(cities))
	}
}

// GET /api/v1/cities/:id/districts returns only matching districts for known city.
func TestListDistrictsByCity_KnownCity_ReturnsMatchingDistricts(t *testing.T) {
	env := newRegionTestDBEnv(t)
	_, _, makassarID, _ := seedRegionData(t, env.pool)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cities/"+makassarID+"/districts", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var districts []struct {
		ID     string `json:"id"`
		CityID string `json:"city_id"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &districts); err != nil {
		t.Fatalf("unmarshal districts: %v", err)
	}

	if len(districts) != 2 {
		t.Fatalf("expected 2 districts for makassar, got %d", len(districts))
	}
	for _, d := range districts {
		if d.CityID != makassarID {
			t.Errorf("district %s has unexpected city_id %s", d.ID, d.CityID)
		}
	}
}

// GET /api/v1/cities/:id/districts returns 200 with [] when city_id is unknown.
func TestListDistrictsByCity_UnknownCity_Returns200WithEmpty(t *testing.T) {
	env := newRegionTestDBEnv(t)
	seedRegionData(t, env.pool)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cities/does-not-exist/districts", nil)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var districts []any
	if err := json.Unmarshal(rec.Body.Bytes(), &districts); err != nil {
		t.Fatalf("unmarshal districts: %v", err)
	}
	if len(districts) != 0 {
		t.Errorf("expected empty slice for unknown city, got %d items", len(districts))
	}
}

// GET /api/v1/provinces/cities and GET /api/v1/cities/districts work without JWT.
func TestRegionEndpoints_NoAuth_Returns200(t *testing.T) {
	env := newRegionTestDBEnv(t)
	_, _, _, surabayaID := seedRegionData(t, env.pool)

	for _, tt := range []struct {
		name string
		path string
	}{
		{"provinces", "/api/v1/provinces"},
		{"provinces cities", "/api/v1/provinces/does-not-exist/cities"},
		{"cities districts", "/api/v1/cities/" + surabayaID + "/districts"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			env.e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("path %s: expected 200 without auth, got %d", tt.path, rec.Code)
			}
		})
	}
}
