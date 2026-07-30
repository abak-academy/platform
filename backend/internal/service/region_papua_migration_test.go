package service

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	dbmigrations "akademi-bimbel/db"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
)

// newPapuaMigrationTestPool spins up a dedicated ephemeral Postgres container for
// this file's tests (NOT the shared newRealDBService() singleton used elsewhere in
// this package), because these tests need precise control over which migrations are
// applied and when, to prove 0046's up/down behavior against a known baseline.
func newPapuaMigrationTestPool(t *testing.T) *pgxpool.Pool {
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

	pool, err := infra.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

const (
	migration0046Up   = "0046_papua_2022_regions.up.sql"
	migration0046Down = "0046_papua_2022_regions.down.sql"
)

// applyMigrationsExcept0046 applies every *.up.sql migration in order, skipping
// 0046, leaving the database at the exact baseline that existed right before
// Task 16's migration (34 provinces, 514 cities, 7215 districts).
func applyMigrationsExcept0046(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	entries, err := fs.Glob(dbmigrations.FS, "migrations/*.up.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(entries)

	for _, path := range entries {
		filename := path[len("migrations/"):]
		if filename == migration0046Up {
			continue
		}
		sql, err := fs.ReadFile(dbmigrations.FS, path)
		if err != nil {
			t.Fatalf("read migration %s: %v", filename, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply migration %s: %v", filename, err)
		}
	}
}

// applyMigrationFile execs the given migration file (up or down) verbatim.
func applyMigrationFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, filename string) {
	t.Helper()
	sql, err := fs.ReadFile(dbmigrations.FS, "migrations/"+filename)
	if err != nil {
		t.Fatalf("read migration %s: %v", filename, err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply migration %s: %v", filename, err)
	}
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// TestPapuaMigration_ProvinceCountAndNewProvincesHaveCities covers FR36 (38
// provinces total) and FR37 (every new province has a non-empty kabupaten/kota
// list via ListCitiesByProvince).
func TestPapuaMigration_ProvinceCountAndNewProvincesHaveCities(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)
	applyMigrationFile(t, ctx, pool, migration0046Up)

	repo := repository.New(pool)

	if got := countRows(t, ctx, pool, "province"); got != 38 {
		t.Errorf("province count: want 38, got %d", got)
	}

	for _, provinceID := range []string{"93", "LOC-94", "95", "96"} {
		cities, err := repo.ListCitiesByProvince(ctx, provinceID)
		if err != nil {
			t.Fatalf("ListCitiesByProvince(%s): %v", provinceID, err)
		}
		if len(cities) == 0 {
			t.Errorf("province %s: expected non-empty cities (FR37), got none", provinceID)
		}
	}
}

// TestPapuaMigration_CollidingCodesKeepOriginalSeedNames proves Invariant 5: the
// migration must not relabel any of the eleven official 2022+ codes that collide
// with a pre-existing seed id -- province 94 and the ten kabupaten/kota ids below
// (per task_15_finding.md's collision table). If any of these resolved to their
// new official name instead of the seed's original name, a historical order FK'd
// to that id would have been silently relabelled.
func TestPapuaMigration_CollidingCodesKeepOriginalSeedNames(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)
	applyMigrationFile(t, ctx, pool, migration0046Up)

	repo := repository.New(pool)

	prov, err := repo.GetProvinceByID(ctx, "94")
	if err != nil {
		t.Fatalf("GetProvinceByID(94): %v", err)
	}
	if prov == nil || prov.Name != "PAPUA" {
		t.Errorf("province 94: want original seed name PAPUA, got %+v", prov)
	}

	wantCityNames := map[string]string{
		"9103": "KABUPATEN TELUK WONDAMA",
		"9105": "KABUPATEN MANOKWARI",
		"9106": "KABUPATEN SORONG SELATAN",
		"9110": "KABUPATEN MAYBRAT",
		"9111": "KABUPATEN MANOKWARI SELATAN",
		"9171": "KOTA SORONG",
		"9401": "KABUPATEN MERAUKE",
		"9402": "KABUPATEN JAYAWIJAYA",
		"9403": "KABUPATEN JAYAPURA",
		"9404": "KABUPATEN NABIRE",
		"9408": "KABUPATEN KEPULAUAN YAPEN",
	}
	for id, wantName := range wantCityNames {
		city, err := repo.GetCityByID(ctx, id)
		if err != nil {
			t.Fatalf("GetCityByID(%s): %v", id, err)
		}
		if city == nil || city.Name != wantName {
			t.Errorf("city %s: want original seed name %q, got %+v", id, wantName, city)
		}
	}
}

// TestPapuaMigration_ValidateAddressHierarchy_AcceptsNewProvinceTriples covers
// FR38: validateAddressHierarchy (store.go:604-628) must accept a province -> city
// -> district triple from a new province, including one built entirely from LOC-
// surrogate ids (FR36a).
func TestPapuaMigration_ValidateAddressHierarchy_AcceptsNewProvinceTriples(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)
	applyMigrationFile(t, ctx, pool, migration0046Up)

	repo := repository.New(pool)
	svc := NewWithStore(repo, repo, nil, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, nil, nil, nil, nil)

	cases := []struct {
		name       string
		provinceID string
		cityID     string
		districtID string
	}{
		{"true-code triple (Papua Selatan)", "93", "9301", "930101"},
		{"LOC- surrogate triple (Papua Tengah / Kabupaten Paniai)", "LOC-94", "LOC-9403", "940301"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.validateAddressHierarchy(ctx, tc.provinceID, tc.cityID, tc.districtID); err != nil {
				t.Errorf("validateAddressHierarchy(%s, %s, %s): %v", tc.provinceID, tc.cityID, tc.districtID, err)
			}
		})
	}
}

// TestPapuaMigration_NF5_OrdersRowUnchangedAcrossMigration is the NF5 proof: an
// orders row FK'd into the pre-migration Papua block (province 94 / city 9403 /
// district 9403080, all pre-existing seed rows) must be byte-for-byte unchanged
// after 0046 runs.
func TestPapuaMigration_NF5_OrdersRowUnchangedAcrossMigration(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)

	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', 'NF5 Test Student') RETURNING id`,
		fmt.Sprintf("nf5-%s@test.local", uuid.NewString()),
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var orderID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO orders (student_id, province_id, city_id, district_id) VALUES ($1, '94', '9403', '9403080') RETURNING id`,
		userID,
	).Scan(&orderID)
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}

	applyMigrationFile(t, ctx, pool, migration0046Up)

	var provinceID, cityID, districtID string
	err = pool.QueryRow(ctx,
		`SELECT province_id, city_id, district_id FROM orders WHERE id = $1`,
		orderID,
	).Scan(&provinceID, &cityID, &districtID)
	if err != nil {
		t.Fatalf("re-read order: %v", err)
	}
	if provinceID != "94" || cityID != "9403" || districtID != "9403080" {
		t.Errorf("order region FKs changed by migration: got province=%s city=%s district=%s (want 94/9403/9403080)",
			provinceID, cityID, districtID)
	}
}

// TestPapuaMigration_DownReversesToBaseline exercises 0046's down migration on a
// database already at 0045: apply up, then down, and assert province/city/district
// counts return to exactly the pre-migration baseline and the tables still exist
// (0046's down must not copy 0029's DROP TABLE behavior).
func TestPapuaMigration_DownReversesToBaseline(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)

	assertCounts := func(t *testing.T, wantProvince, wantCity, wantDistrict int) {
		t.Helper()
		if got := countRows(t, ctx, pool, "province"); got != wantProvince {
			t.Errorf("province count: want %d, got %d", wantProvince, got)
		}
		if got := countRows(t, ctx, pool, "city"); got != wantCity {
			t.Errorf("city count: want %d, got %d", wantCity, got)
		}
		if got := countRows(t, ctx, pool, "district"); got != wantDistrict {
			t.Errorf("district count: want %d, got %d", wantDistrict, got)
		}
	}

	assertCounts(t, 34, 514, 7215)

	applyMigrationFile(t, ctx, pool, migration0046Up)
	assertCounts(t, 38, 540, 7812)

	applyMigrationFile(t, ctx, pool, migration0046Down)
	assertCounts(t, 34, 514, 7215)

	for _, table := range []string{"province", "city", "district"} {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s exists: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s should still exist after down migration", table)
		}
	}
}
