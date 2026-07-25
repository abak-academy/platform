package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
)

// newRegionTestPool spins up an ephemeral Postgres container, applies all
// migrations (including 0029 which creates the province/city/district tables),
// and returns a connected pool.
func newRegionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

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

	return pool
}

// seedRegionTables clears old data and inserts deterministic provinces,
// cities, and districts for testing.
func seedRegionTables(t *testing.T, pool *pgxpool.Pool) (sulselID, jatimID, makassarID, surabayaID string) {
	t.Helper()
	ctx := context.Background()

	// Delete in FK order.
	_, err := pool.Exec(ctx, `DELETE FROM district`)
	if err != nil {
		t.Fatalf("clear district: %v", err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM city`)
	if err != nil {
		t.Fatalf("clear city: %v", err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM province`)
	if err != nil {
		t.Fatalf("clear province: %v", err)
	}

	err = pool.QueryRow(ctx,
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
	for _, d := range []struct {
		id, cityID, name string
	}{
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

// Compile-time check: *Repository must implement all region methods.
var _ interface {
	ListProvinces(context.Context) ([]model.Province, error)
	ListCitiesByProvince(context.Context, string) ([]model.City, error)
	ListDistrictsByCity(context.Context, string) ([]model.District, error)
	GetProvinceByID(context.Context, string) (*model.Province, error)
	GetCityByID(context.Context, string) (*model.City, error)
	GetDistrictByID(context.Context, string) (*model.District, error)
	GetProvinceByName(context.Context, string) (*model.Province, error)
	GetCityByNameInProvince(context.Context, string, string) (*model.City, error)
	GetDistrictByNameInCity(context.Context, string, string) (*model.District, error)
} = (*Repository)(nil)

func TestListProvinces(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	sulselID, jatimID, _, _ := seedRegionTables(t, pool)

	// Insert a third province to verify alphabetical ordering.
	_, err := pool.Exec(ctx,
		`INSERT INTO province (id, name) VALUES ($1, $2)`,
		"31", "DKI JAKARTA",
	)
	if err != nil {
		t.Fatalf("insert province dki: %v", err)
	}

	provinces, err := repo.ListProvinces(ctx)
	if err != nil {
		t.Fatalf("ListProvinces: %v", err)
	}

	if len(provinces) != 3 {
		t.Fatalf("expected 3 provinces, got %d", len(provinces))
	}

	// Alphabetical order: DKI JAKARTA, JAWA TIMUR, SULAWESI SELATAN.
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

func TestListCitiesByProvince(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	sulselID, jatimID, makassarID, surabayaID := seedRegionTables(t, pool)

	// List cities for Sulsel -- should only get Makassar.
	cities, err := repo.ListCitiesByProvince(ctx, sulselID)
	if err != nil {
		t.Fatalf("ListCitiesByProvince: %v", err)
	}
	if len(cities) != 1 {
		t.Fatalf("expected 1 city for sulsel, got %d", len(cities))
	}
	if cities[0].ID != makassarID || cities[0].ProvinceID != sulselID || cities[0].Name != "KOTA MAKASSAR" {
		t.Errorf("unexpected city data: %+v", cities[0])
	}

	// List cities for Jawa Timur -- should only get Surabaya.
	cities, err = repo.ListCitiesByProvince(ctx, jatimID)
	if err != nil {
		t.Fatalf("ListCitiesByProvince: %v", err)
	}
	if len(cities) != 1 {
		t.Fatalf("expected 1 city for jatim, got %d", len(cities))
	}
	if cities[0].ID != surabayaID || cities[0].ProvinceID != jatimID {
		t.Errorf("unexpected city data: %+v", cities[0])
	}
}

func TestListCitiesByProvince_bogusID_returnsEmpty(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	seedRegionTables(t, pool)

	cities, err := repo.ListCitiesByProvince(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListCitiesByProvince with bogus id should not error: %v", err)
	}
	if len(cities) != 0 {
		t.Errorf("expected empty slice for bogus province id, got %d items", len(cities))
	}
}

func TestListDistrictsByCity(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	_, _, makassarID, surabayaID := seedRegionTables(t, pool)

	// List districts for Makassar -- should have 2.
	districts, err := repo.ListDistrictsByCity(ctx, makassarID)
	if err != nil {
		t.Fatalf("ListDistrictsByCity: %v", err)
	}
	if len(districts) != 2 {
		t.Fatalf("expected 2 districts for makassar, got %d", len(districts))
	}
	for _, d := range districts {
		if d.CityID != makassarID {
			t.Errorf("district %s has city_id %s, expected %s", d.ID, d.CityID, makassarID)
		}
	}

	// List districts for Surabaya -- should have 1.
	districts, err = repo.ListDistrictsByCity(ctx, surabayaID)
	if err != nil {
		t.Fatalf("ListDistrictsByCity: %v", err)
	}
	if len(districts) != 1 {
		t.Fatalf("expected 1 district for surabaya, got %d", len(districts))
	}
	if districts[0].CityID != surabayaID {
		t.Errorf("district %s has city_id %s, expected %s", districts[0].ID, districts[0].CityID, surabayaID)
	}
	if districts[0].Name != "GENTENG" {
		t.Errorf("expected district name 'GENTENG', got %q", districts[0].Name)
	}
}

func TestListDistrictsByCity_bogusID_returnsEmpty(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	seedRegionTables(t, pool)

	districts, err := repo.ListDistrictsByCity(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListDistrictsByCity with bogus city id should not error: %v", err)
	}
	if len(districts) != 0 {
		t.Errorf("expected empty slice for bogus city id, got %d items", len(districts))
	}
}

func TestGetProvinceByID(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	sulselID, _, _, _ := seedRegionTables(t, pool)

	// Found case.
	p, err := repo.GetProvinceByID(ctx, sulselID)
	if err != nil {
		t.Fatalf("GetProvinceByID: %v", err)
	}
	if p == nil {
		t.Fatal("expected province, got nil")
	}
	if p.ID != sulselID || p.Name != "SULAWESI SELATAN" {
		t.Errorf("unexpected province: %+v", p)
	}

	// Not-found case.
	p, err = repo.GetProvinceByID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetProvinceByID for non-existent should not error: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil for non-existent province, got %+v", p)
	}
}

func TestGetCityByID(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	sulselID, _, makassarID, _ := seedRegionTables(t, pool)

	// Found case.
	c, err := repo.GetCityByID(ctx, makassarID)
	if err != nil {
		t.Fatalf("GetCityByID: %v", err)
	}
	if c == nil {
		t.Fatal("expected city, got nil")
	}
	if c.ID != makassarID || c.ProvinceID != sulselID || c.Name != "KOTA MAKASSAR" {
		t.Errorf("unexpected city: %+v", c)
	}

	// Not-found case.
	c, err = repo.GetCityByID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetCityByID for non-existent should not error: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil for non-existent city, got %+v", c)
	}
}

func TestGetProvinceByName(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	sulselID, _, _, _ := seedRegionTables(t, pool)

	// Found by exact name.
	p, err := repo.GetProvinceByName(ctx, "SULAWESI SELATAN")
	if err != nil {
		t.Fatalf("GetProvinceByName: %v", err)
	}
	if p == nil {
		t.Fatal("expected province, got nil")
	}
	if p.ID != sulselID || p.Name != "SULAWESI SELATAN" {
		t.Errorf("unexpected province: %+v", p)
	}

	// Case-insensitive.
	p, err = repo.GetProvinceByName(ctx, "sulawesi selatan")
	if err != nil {
		t.Fatalf("GetProvinceByName case-insensitive: %v", err)
	}
	if p == nil {
		t.Fatal("expected province for lowercase name, got nil")
	}
	if p.ID != sulselID {
		t.Errorf("expected same province id, got %s", p.ID)
	}

	// Not-found case.
	p, err = repo.GetProvinceByName(ctx, "NONEXISTENT PROVINCE")
	if err != nil {
		t.Fatalf("GetProvinceByName for non-existent should not error: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil for non-existent province, got %+v", p)
	}
}

func TestGetCityByNameInProvince(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	sulselID, jatimID, makassarID, _ := seedRegionTables(t, pool)

	// Found by exact name in correct province.
	c, err := repo.GetCityByNameInProvince(ctx, "KOTA MAKASSAR", sulselID)
	if err != nil {
		t.Fatalf("GetCityByNameInProvince: %v", err)
	}
	if c == nil {
		t.Fatal("expected city, got nil")
	}
	if c.ID != makassarID || c.ProvinceID != sulselID {
		t.Errorf("unexpected city: %+v", c)
	}

	// Case-insensitive.
	c, err = repo.GetCityByNameInProvince(ctx, "kota makassar", sulselID)
	if err != nil {
		t.Fatalf("GetCityByNameInProvince case-insensitive: %v", err)
	}
	if c == nil {
		t.Fatal("expected city for lowercase name, got nil")
	}
	if c.ID != makassarID {
		t.Errorf("expected same city id, got %s", c.ID)
	}

	// Same city name in wrong province returns nil.
	c, err = repo.GetCityByNameInProvince(ctx, "KOTA MAKASSAR", jatimID)
	if err != nil {
		t.Fatalf("GetCityByNameInProvince wrong province: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil for city in wrong province, got %+v", c)
	}

	// Not-found case.
	c, err = repo.GetCityByNameInProvince(ctx, "NONEXISTENT", sulselID)
	if err != nil {
		t.Fatalf("GetCityByNameInProvince for non-existent should not error: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil for non-existent city, got %+v", c)
	}
}

func TestGetDistrictByNameInCity(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	_, _, makassarID, surabayaID := seedRegionTables(t, pool)

	// Found by exact name in correct city.
	d, err := repo.GetDistrictByNameInCity(ctx, "MARISO", makassarID)
	if err != nil {
		t.Fatalf("GetDistrictByNameInCity: %v", err)
	}
	if d == nil {
		t.Fatal("expected district, got nil")
	}
	if d.ID != "7371010" || d.CityID != makassarID {
		t.Errorf("unexpected district: %+v", d)
	}

	// Case-insensitive.
	d, err = repo.GetDistrictByNameInCity(ctx, "mariso", makassarID)
	if err != nil {
		t.Fatalf("GetDistrictByNameInCity case-insensitive: %v", err)
	}
	if d == nil {
		t.Fatal("expected district for lowercase name, got nil")
	}
	if d.ID != "7371010" {
		t.Errorf("expected same district id, got %s", d.ID)
	}

	// Same district name in wrong city returns nil.
	d, err = repo.GetDistrictByNameInCity(ctx, "MARISO", surabayaID)
	if err != nil {
		t.Fatalf("GetDistrictByNameInCity wrong city: %v", err)
	}
	if d != nil {
		t.Errorf("expected nil for district in wrong city, got %+v", d)
	}

	// Not-found case.
	d, err = repo.GetDistrictByNameInCity(ctx, "NONEXISTENT", makassarID)
	if err != nil {
		t.Fatalf("GetDistrictByNameInCity for non-existent should not error: %v", err)
	}
	if d != nil {
		t.Errorf("expected nil for non-existent district, got %+v", d)
	}
}

func TestGetDistrictByID(t *testing.T) {
	pool := newRegionTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	_, _, makassarID, _ := seedRegionTables(t, pool)

	// Found case: query for MARISO (id 7371010).
	d, err := repo.GetDistrictByID(ctx, "7371010")
	if err != nil {
		t.Fatalf("GetDistrictByID: %v", err)
	}
	if d == nil {
		t.Fatal("expected district, got nil")
	}
	if d.ID != "7371010" || d.CityID != makassarID || d.Name != "MARISO" {
		t.Errorf("unexpected district: %+v", d)
	}

	// Not-found case.
	d, err = repo.GetDistrictByID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetDistrictByID for non-existent should not error: %v", err)
	}
	if d != nil {
		t.Errorf("expected nil for non-existent district, got %+v", d)
	}
}
