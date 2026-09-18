package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"akademi-bimbel/internal/infra"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestDatabaseImport(t *testing.T) {
	ctx := t.Context()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine", tcpostgres.WithDatabase("import_test"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, infra.RunMigrations(ctx, dsn))
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	_, err = conn.Exec(ctx, `CREATE UNIQUE INDEX uq_school_npsn_normalized ON school (upper(btrim(npsn))) WHERE npsn IS NOT NULL`)
	require.NoError(t, err)
	var schoolID, userID string
	err = conn.QueryRow(ctx, `INSERT INTO school (name,code,npsn,school_types,status,alamat) VALUES ('Legacy','legacy',' 00123456 ',ARRAY['SD','SMA'],'deactivated','Legacy address') RETURNING id::text`).Scan(&schoolID)
	require.NoError(t, err)
	err = conn.QueryRow(ctx, `INSERT INTO users (email,role,name,school_id,jenjang) VALUES ('import@test.local','student','Student',$1,'SD') RETURNING id::text`, schoolID).Scan(&userID)
	require.NoError(t, err)
	s := sourceCSV(t, "00123456,Source,SMA,,,,KAB. BOGOR,,,New address,,\nK1234567,Kursus,KURSUS,,,,KOTA BOGOR,,,New address,,\n")
	out := func() string { return filepath.Join(t.TempDir(), "report") }
	count := func() int {
		var n int
		require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM school`).Scan(&n))
		return n
	}
	before := count()
	dry, err := execute(ctx, conn, options{Out: out()}, s, nil)
	require.NoError(t, err)
	require.Equal(t, "dry_run", dry.Status)
	require.Equal(t, 1, dry.Counts["insert"])
	require.Equal(t, 1, dry.Counts["update"])
	require.Equal(t, before, count())
	again, err := execute(ctx, conn, options{Out: out()}, s, nil)
	require.NoError(t, err)
	require.Equal(t, dry.PlanSHA256, again.PlanSHA256)
	_, err = execute(ctx, conn, options{Out: out(), Apply: true, ExpectedPlan: strings.Repeat("0", 64)}, s, nil)
	require.ErrorContains(t, err, "plan changed")
	require.Equal(t, before, count())
	_, err = conn.Exec(ctx, `UPDATE school SET name='Changed concurrently' WHERE id=$1`, schoolID)
	require.NoError(t, err)
	_, err = execute(ctx, conn, options{Out: out(), Apply: true, ExpectedPlan: dry.PlanSHA256}, s, nil)
	require.ErrorContains(t, err, "plan changed")
	require.Equal(t, before, count())
	_, err = conn.Exec(ctx, `UPDATE school SET name='Legacy' WHERE id=$1`, schoolID)
	require.NoError(t, err)
	reportDir := out()
	applied, err := execute(ctx, conn, options{Out: reportDir, Apply: true, ExpectedPlan: dry.PlanSHA256}, s, nil)
	require.NoError(t, err)
	require.Equal(t, "applied", applied.Status)
	require.Equal(t, before+1, count())
	var linkedID, name, code, status, address string
	var types []string
	require.NoError(t, conn.QueryRow(ctx, `SELECT u.school_id::text,s.name,s.code,s.status,s.alamat,s.school_types FROM users u JOIN school s ON s.id=u.school_id WHERE u.id=$1`, userID).Scan(&linkedID, &name, &code, &status, &address, &types))
	require.Equal(t, schoolID, linkedID)
	require.Equal(t, "Legacy", name)
	require.Equal(t, "legacy", code)
	require.Equal(t, "deactivated", status)
	require.Equal(t, "Legacy address", address)
	require.Equal(t, []string{"SD", "SMA"}, types)
	var locationOK bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT c.province_id=s.provinsi_id FROM school s JOIN city c ON c.id=s.kota_id WHERE s.npsn='K1234567'`).Scan(&locationOK))
	require.True(t, locationOK)
	data, err := os.ReadFile(filepath.Join(reportDir, "result.json"))
	require.NoError(t, err)
	var recorded result
	require.NoError(t, json.Unmarshal(data, &recorded))
	require.Equal(t, "applied", recorded.Status)
	repeat, err := execute(ctx, conn, options{Out: out()}, s, nil)
	require.NoError(t, err)
	require.Equal(t, 2, repeat.Counts["unchanged"])
	_, err = execute(ctx, conn, options{Out: out(), Apply: true, ExpectedPlan: repeat.PlanSHA256}, s, nil)
	require.NoError(t, err)
	require.Equal(t, before+1, count())
	_, err = execute(ctx, conn, options{Out: reportDir}, s, nil)
	require.ErrorContains(t, err, "create new report directory")

	t.Run("all or nothing on source conflicts", func(t *testing.T) {
		bad := sourceCSV(t, "00123456,Source,SMA,,,,KAB. BOGOR,,,New address,,\n00123456,Conflicting,SMA,,,,KAB. BOGOR,,,Elsewhere,,\nK7654321,Valid,KURSUS,,,,KOTA BOGOR,,,Address,,\n")
		r, err := execute(ctx, conn, options{Out: out(), Apply: true, ExpectedPlan: strings.Repeat("0", 64)}, bad, nil)
		require.ErrorContains(t, err, "unresolved")
		require.Equal(t, 1, r.Counts["conflict"])
		require.Equal(t, before+1, count())
	})
	t.Run("rollback update if insert fails", func(t *testing.T) {
		_, err := conn.Exec(ctx, `ALTER TABLE school ADD CONSTRAINT import_test_failure CHECK (npsn <> 'K7654321')`)
		require.NoError(t, err)
		defer func() {
			_, err := conn.Exec(ctx, `ALTER TABLE school DROP CONSTRAINT import_test_failure`)
			require.NoError(t, err)
		}()
		_, err = conn.Exec(ctx, `UPDATE school SET provinsi_id=NULL,kota_id=NULL WHERE id=$1`, schoolID)
		require.NoError(t, err)
		input := sourceCSV(t, "00123456,Source,SMA,,,,KAB. BOGOR,,,New address,,\nK7654321,Valid,KURSUS,,,,KOTA BOGOR,,,Address,,\n")
		r, err := execute(ctx, conn, options{Out: out()}, input, nil)
		require.NoError(t, err)
		_, err = execute(ctx, conn, options{Out: out(), Apply: true, ExpectedPlan: r.PlanSHA256}, input, nil)
		require.Error(t, err)
		var missing bool
		require.NoError(t, conn.QueryRow(ctx, `SELECT kota_id IS NULL FROM school WHERE id=$1`, schoolID).Scan(&missing))
		require.True(t, missing)
		require.Equal(t, before+1, count())
	})
	t.Run("bulk insert and repeat", func(t *testing.T) {
		var csv strings.Builder
		for i := 0; i < 10000; i++ {
			fmt.Fprintf(&csv, "T%07d,School %d,SD,,,,KAB. BOGOR,,,Address,,\n", i, i)
		}
		bulk := sourceCSV(t, csv.String())
		r, err := execute(ctx, conn, options{Out: out()}, bulk, nil)
		require.NoError(t, err)
		require.Equal(t, 10000, r.Counts["insert"])
		_, err = execute(ctx, conn, options{Out: out(), Apply: true, ExpectedPlan: r.PlanSHA256}, bulk, nil)
		require.NoError(t, err)
		repeat, err := execute(ctx, conn, options{Out: out()}, bulk, nil)
		require.NoError(t, err)
		require.Equal(t, 10000, repeat.Counts["unchanged"])
	})
	if path := os.Getenv("PUSDATIN_TEST_CSV"); path != "" {
		t.Run("national source dry run", func(t *testing.T) {
			f, err := os.Open(path)
			require.NoError(t, err)
			defer f.Close()
			input, err := readSource(f)
			require.NoError(t, err)
			dir := os.Getenv("PUSDATIN_TEST_REPORT")
			if dir == "" {
				dir = out()
			}
			before := count()
			r, err := execute(ctx, conn, options{Out: dir}, input, nil)
			if r != nil && r.Counts["conflict"] > 0 {
				require.ErrorContains(t, err, "unresolved")
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, r)
			require.Equal(t, before, count())
			t.Logf("source rows=%d distinct=%d counts=%v report=%s", r.SourceRows, r.DistinctNPSNs, r.Counts, dir)
		})
	}
	t.Run("missing unique index blocks writes", func(t *testing.T) {
		_, err := conn.Exec(ctx, `DROP INDEX uq_school_npsn_normalized`)
		require.NoError(t, err)
		_, err = execute(ctx, conn, options{Out: out()}, s, nil)
		require.ErrorContains(t, err, "unique index")
	})
}
