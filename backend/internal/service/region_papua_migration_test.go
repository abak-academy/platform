package service

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

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

// papuaProvinceIDs are the six current-code provinces the whole 91-96 block
// remaps to (task_15_finding.md, combined Appendix 1 + Appendix 2).
var papuaProvinceIDs = []string{"91", "92", "93", "94", "95", "96"}

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

// nonPapuaSnapshot is the row-count + checksum fingerprint of everything OUTSIDE
// the six Papua-block province ids. It is the guard for the up migration's DELETE
// scope: if the DELETE (or the reseed) ever touches so much as one row belonging
// to the other 32 provinces, this fingerprint changes.
type nonPapuaSnapshot struct {
	provinceCount   int
	provinceHash    string
	cityCount       int
	cityHash        string
	districtCount   int
	districtHash    string
}

func captureNonPapuaSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) nonPapuaSnapshot {
	t.Helper()
	var s nonPapuaSnapshot

	err := pool.QueryRow(ctx,
		`SELECT count(*), coalesce(md5(string_agg(id || name, ',' ORDER BY id)), '')
		 FROM province WHERE id NOT IN ('91','92','93','94','95','96')`,
	).Scan(&s.provinceCount, &s.provinceHash)
	if err != nil {
		t.Fatalf("snapshot province: %v", err)
	}

	err = pool.QueryRow(ctx,
		`SELECT count(*), coalesce(md5(string_agg(id || name || province_id, ',' ORDER BY id)), '')
		 FROM city WHERE province_id NOT IN ('91','92','93','94','95','96')`,
	).Scan(&s.cityCount, &s.cityHash)
	if err != nil {
		t.Fatalf("snapshot city: %v", err)
	}

	err = pool.QueryRow(ctx,
		`SELECT count(*), coalesce(md5(string_agg(id || name || city_id, ',' ORDER BY id)), '')
		 FROM district WHERE city_id NOT IN (
		     SELECT id FROM city WHERE province_id IN ('91','92','93','94','95','96')
		 )`,
	).Scan(&s.districtCount, &s.districtHash)
	if err != nil {
		t.Fatalf("snapshot district: %v", err)
	}

	return s
}

// TestPapuaMigration_ProvinceCountAndAllSixProvincesHaveCities covers FR36 (38
// provinces total, six Papua-block provinces carrying their official codes and
// uppercase names) and FR37 (every one of the six has a non-empty kabupaten/kota
// list via ListCitiesByProvince -- the same repository call the
// GET /provinces/:id/cities handler makes).
func TestPapuaMigration_ProvinceCountAndAllSixProvincesHaveCities(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)
	applyMigrationFile(t, ctx, pool, migration0046Up)

	repo := repository.New(pool)

	if got := countRows(t, ctx, pool, "province"); got != 38 {
		t.Errorf("province count: want 38, got %d", got)
	}

	wantNames := map[string]string{
		"91": "PAPUA",
		"92": "PAPUA BARAT",
		"93": "PAPUA SELATAN",
		"94": "PAPUA TENGAH",
		"95": "PAPUA PEGUNUNGAN",
		"96": "PAPUA BARAT DAYA",
	}

	for _, provinceID := range papuaProvinceIDs {
		prov, err := repo.GetProvinceByID(ctx, provinceID)
		if err != nil {
			t.Fatalf("GetProvinceByID(%s): %v", provinceID, err)
		}
		if prov == nil || prov.Name != wantNames[provinceID] {
			t.Errorf("province %s: want name %q, got %+v", provinceID, wantNames[provinceID], prov)
		}

		cities, err := repo.ListCitiesByProvince(ctx, provinceID)
		if err != nil {
			t.Fatalf("ListCitiesByProvince(%s): %v", provinceID, err)
		}
		if len(cities) == 0 {
			t.Errorf("province %s: expected non-empty cities (FR37), got none", provinceID)
		}
	}
}

// TestPapuaMigration_CombinedTotals covers the finding's combined-totals figures:
// 42 kabupaten/kota and 793 kecamatan across the full 91-96 block.
func TestPapuaMigration_CombinedTotals(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)
	applyMigrationFile(t, ctx, pool, migration0046Up)

	var cityCount int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM city WHERE province_id IN ('91','92','93','94','95','96')`,
	).Scan(&cityCount)
	if err != nil {
		t.Fatalf("count cities: %v", err)
	}
	if cityCount != 42 {
		t.Errorf("kabupaten/kota count across the 91-96 block: want 42, got %d", cityCount)
	}

	var districtCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM district WHERE city_id IN (
		     SELECT id FROM city WHERE province_id IN ('91','92','93','94','95','96')
		 )`,
	).Scan(&districtCount)
	if err != nil {
		t.Fatalf("count districts: %v", err)
	}
	if districtCount != 793 {
		t.Errorf("kecamatan count across the 91-96 block: want 793, got %d", districtCount)
	}
}

// TestPapuaMigration_NoLocSurrogatesRemain proves the retracted LOC-<code>
// surrogate scheme from the additive draft is fully gone: production is
// confirmed empty, so nothing needs to hide behind a non-official local id.
func TestPapuaMigration_NoLocSurrogatesRemain(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)
	applyMigrationFile(t, ctx, pool, migration0046Up)

	for _, table := range []string{"province", "city", "district"} {
		var n int
		err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE id LIKE 'LOC-%'").Scan(&n)
		if err != nil {
			t.Fatalf("count LOC- ids in %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("table %s: want 0 LOC- surrogate ids, got %d", table, n)
		}
	}
}

// TestPapuaMigration_NonPapuaProvincesUnchanged is the guard for the up
// migration's DELETE scope -- the single biggest risk called out for this task.
// It captures a row-count + checksum fingerprint of the other 32 provinces (and
// their cities/districts) before 0046 runs and asserts it is bit-identical
// after. This is the most important test in this file.
func TestPapuaMigration_NonPapuaProvincesUnchanged(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)

	before := captureNonPapuaSnapshot(t, ctx, pool)
	if before.provinceCount != 32 {
		t.Fatalf("sanity check failed: expected 32 non-Papua provinces at baseline, got %d", before.provinceCount)
	}

	applyMigrationFile(t, ctx, pool, migration0046Up)

	after := captureNonPapuaSnapshot(t, ctx, pool)

	if after.provinceCount != before.provinceCount || after.provinceHash != before.provinceHash {
		t.Errorf("non-Papua provinces changed: before count=%d hash=%s, after count=%d hash=%s",
			before.provinceCount, before.provinceHash, after.provinceCount, after.provinceHash)
	}
	if after.cityCount != before.cityCount || after.cityHash != before.cityHash {
		t.Errorf("non-Papua cities changed: before count=%d hash=%s, after count=%d hash=%s",
			before.cityCount, before.cityHash, after.cityCount, after.cityHash)
	}
	if after.districtCount != before.districtCount || after.districtHash != before.districtHash {
		t.Errorf("non-Papua districts changed: before count=%d hash=%s, after count=%d hash=%s",
			before.districtCount, before.districtHash, after.districtCount, after.districtHash)
	}
}

// TestPapuaMigration_ValidateAddressHierarchy_AcceptsNewProvinceTriples covers
// FR38: validateAddressHierarchy (store.go:604-628) must accept a province ->
// city -> district triple drawn from each of the four new provinces (93, 94,
// 95, 96), all using their true official Kemendagri code -- no LOC- surrogate
// exists anymore.
func TestPapuaMigration_ValidateAddressHierarchy_AcceptsNewProvinceTriples(t *testing.T) {
	pool := newPapuaMigrationTestPool(t)
	ctx := context.Background()
	applyMigrationsExcept0046(t, ctx, pool)
	applyMigrationFile(t, ctx, pool, migration0046Up)

	repo := repository.New(pool)
	svc := NewWithStore(repo, repo, nil, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, nil, nil, nil, nil, nil)

	cases := []struct {
		name       string
		provinceID string
		cityID     string
		districtID string
	}{
		{"Papua Selatan", "93", "9301", "930101"},
		{"Papua Tengah", "94", "9401", "940101"},
		{"Papua Pegunungan", "95", "9501", "950101"},
		{"Papua Barat Daya", "96", "9601", "960101"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.validateAddressHierarchy(ctx, tc.provinceID, tc.cityID, tc.districtID); err != nil {
				t.Errorf("validateAddressHierarchy(%s, %s, %s): %v", tc.provinceID, tc.cityID, tc.districtID, err)
			}
		})
	}
}

// TestPapuaMigration_DownReversesToBaseline exercises 0046's down migration on a
// database already at 0045: apply up, then down, and assert province/city/
// district counts return to exactly the pre-migration baseline, that the tables
// still exist (0046's down must not copy 0029's DROP TABLE behavior), and that
// the original meanings of 91/94 are restored -- not just the counts.
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
	assertCounts(t, 38, 514, 7224)

	applyMigrationFile(t, ctx, pool, migration0046Down)
	assertCounts(t, 34, 514, 7215)

	repo := repository.New(pool)
	prov91, err := repo.GetProvinceByID(ctx, "91")
	if err != nil {
		t.Fatalf("GetProvinceByID(91): %v", err)
	}
	if prov91 == nil || prov91.Name != "PAPUA BARAT" {
		t.Errorf("province 91 after down: want original seed name PAPUA BARAT, got %+v", prov91)
	}

	prov94, err := repo.GetProvinceByID(ctx, "94")
	if err != nil {
		t.Fatalf("GetProvinceByID(94): %v", err)
	}
	if prov94 == nil || prov94.Name != "PAPUA" {
		t.Errorf("province 94 after down: want original seed name PAPUA, got %+v", prov94)
	}

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

// TestPapuaMigration_NoDropOrTruncate is a static guard on the migration files
// themselves: drop-and-reseed uses DELETE FROM, never DROP TABLE or TRUNCATE.
func TestPapuaMigration_NoDropOrTruncate(t *testing.T) {
	dropOrTruncate := regexp.MustCompile(`(?i)DROP TABLE|TRUNCATE`)

	for _, filename := range []string{migration0046Up, migration0046Down} {
		path := filepath.Join("..", "..", "db", "migrations", filename)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", filename, err)
		}
		if dropOrTruncate.Match(content) {
			t.Errorf("%s: must not contain DROP TABLE or TRUNCATE", filename)
		}
	}
}
