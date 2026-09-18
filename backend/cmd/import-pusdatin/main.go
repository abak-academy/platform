package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"akademi-bimbel/internal/model"
	"github.com/jackc/pgx/v5"
)

type options struct {
	CSV, CityMap, Out, ExpectedPlan string
	Apply                           bool
}

type result struct {
	Status        string         `json:"status"`
	SourceSHA256  string         `json:"source_sha256"`
	PlanSHA256    string         `json:"plan_sha256"`
	SourceRows    int            `json:"source_rows"`
	DistinctNPSNs int            `json:"distinct_npsns"`
	Counts        map[string]int `json:"counts"`
}

func main() {
	var opts options
	flag.StringVar(&opts.CSV, "csv", "", "Pusdatin source CSV (required)")
	flag.StringVar(&opts.CityMap, "city-map", "", "reviewed kabupaten,kota_id CSV for unresolved cities")
	flag.StringVar(&opts.Out, "out", "", "new directory for private plan and result files (required)")
	flag.BoolVar(&opts.Apply, "apply", false, "write the reviewed plan; default is a read-only dry-run")
	flag.StringVar(&opts.ExpectedPlan, "expect-plan", "", "SHA256 from the reviewed dry-run (required with --apply)")
	flag.Parse()
	if opts.CSV == "" || opts.Out == "" || flag.NArg() != 0 || opts.Apply && len(opts.ExpectedPlan) != 64 {
		fmt.Fprintln(os.Stderr, "require --csv and --out; --apply also requires --expect-plan SHA256")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if err := run(ctx, opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts options) error {
	f, err := os.Open(opts.CSV)
	if err != nil {
		return err
	}
	s, err := readSource(f)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	var aliases map[string]string
	if opts.CityMap != "" {
		f, err := os.Open(opts.CityMap)
		if err != nil {
			return err
		}
		aliases, err = readAliases(f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required; no default database is selected")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("invalid DATABASE_URL")
	}
	cfg.ConnectTimeout = 10 * time.Second
	cfg.RuntimeParams["application_name"] = "pusdatin-manual-import"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer conn.Close(context.Background())
	outcome, err := execute(ctx, conn, opts, s, aliases)
	if outcome != nil {
		if outputErr := json.NewEncoder(os.Stdout).Encode(outcome); err == nil {
			err = outputErr
		}
	}
	return err
}

func execute(ctx context.Context, conn *pgx.Conn, opts options, s source, aliases map[string]string) (*result, error) {
	if opts.Apply && len(opts.ExpectedPlan) != 64 {
		return nil, fmt.Errorf("apply requires reviewed plan SHA256")
	}
	if err := os.Mkdir(opts.Out, 0700); err != nil {
		return nil, fmt.Errorf("create new report directory: %w", err)
	}
	txOpts := pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	if opts.Apply {
		txOpts = pgx.TxOptions{IsoLevel: pgx.ReadCommitted}
	}
	tx, err := conn.BeginTx(ctx, txOpts)
	if err != nil {
		return nil, err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '10min'; SET LOCAL idle_in_transaction_session_timeout = '5min'`); err != nil {
		return nil, err
	}
	if opts.Apply {
		// Lock before reading so a concurrent writer cannot invalidate the reviewed snapshot.
		if _, err = tx.Exec(ctx, `LOCK TABLE school IN SHARE ROW EXCLUSIVE MODE; LOCK TABLE province, city IN SHARE MODE`); err != nil {
			return nil, err
		}
	}
	var ready bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM pg_index i WHERE i.indrelid='school'::regclass AND i.indisunique AND i.indisvalid
 AND i.indnkeyatts=1 AND pg_get_expr(i.indexprs,i.indrelid)='upper(btrim(npsn))'
 AND (i.indpred IS NULL OR pg_get_expr(i.indpred,i.indrelid)='(npsn IS NOT NULL)'))
 AND NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='school'::regclass AND attname='category' AND NOT attisdropped)`).Scan(&ready)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, fmt.Errorf("deploy PR 171 schema and the normalized-NPSN unique index before importing")
	}
	rows, err := tx.Query(ctx, `SELECT c.id,c.province_id,c.name FROM city c JOIN province p ON p.id=c.province_id ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	cities, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (model.City, error) {
		var c model.City
		err := r.Scan(&c.ID, &c.ProvinceID, &c.Name)
		return c, err
	})
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,name,code,npsn,school_types,alamat,provinsi_id,kota_id,status,created_at,updated_at FROM school ORDER BY id`)
	if err != nil {
		return nil, err
	}
	schools, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (model.School, error) {
		var s model.School
		err := r.Scan(&s.ID, &s.Name, &s.Code, &s.NPSN, &s.SchoolTypes, &s.Alamat, &s.ProvinsiID, &s.KotaID, &s.Status, &s.CreatedAt, &s.UpdatedAt)
		return s, err
	})
	if err != nil {
		return nil, err
	}
	plan := makePlan(s, cities, schools, aliases)
	outcome, err := writePlan(opts.Out, s, plan)
	if err != nil {
		return nil, err
	}
	save := func() error { return writeResult(opts.Out, outcome) }
	if outcome.Counts["conflict"] > 0 {
		outcome.Status = "blocked"
		if err := save(); err != nil {
			return outcome, err
		}
		return outcome, fmt.Errorf("%d unresolved NPSNs; nothing written to database; inspect plan.jsonl", outcome.Counts["conflict"])
	}
	if !opts.Apply {
		outcome.Status = "dry_run"
		return outcome, save()
	}
	if outcome.PlanSHA256 != opts.ExpectedPlan {
		outcome.Status = "stale_plan"
		if err := save(); err != nil {
			return outcome, err
		}
		return outcome, fmt.Errorf("plan changed; review the new report and rerun with its SHA256; nothing written")
	}
	outcome.Status = "prepared"
	if err := save(); err != nil {
		return outcome, err
	}
	if err := applyPlan(ctx, tx, plan); err != nil {
		return outcome, err
	}
	// Persist an uncertain state before COMMIT so a lost connection never looks like a successful import.
	outcome.Status = "commit_unknown"
	if err := save(); err != nil {
		return outcome, err
	}
	if err := tx.Commit(ctx); err != nil {
		return outcome, fmt.Errorf("commit not confirmed; run a fresh dry-run before any retry: %w", err)
	}
	outcome.Status = "applied"
	if err := save(); err != nil {
		return outcome, fmt.Errorf("database committed but final report write failed: %w", err)
	}
	return outcome, nil
}

func writePlan(dir string, s source, plan []change) (*result, error) {
	f, err := os.OpenFile(filepath.Join(dir, "plan.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	enc := json.NewEncoder(io.MultiWriter(f, h))
	if err := enc.Encode(struct {
		SourceSHA256 string `json:"source_sha256"`
	}{s.SHA256}); err != nil {
		return nil, err
	}
	out := &result{SourceSHA256: s.SHA256, SourceRows: s.Rows, DistinctNPSNs: len(plan), Counts: map[string]int{"insert": 0, "update": 0, "unchanged": 0, "conflict": 0}}
	for _, c := range plan {
		out.Counts[c.Action]++
		if err := enc.Encode(c); err != nil {
			return nil, err
		}
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	out.PlanSHA256 = hex.EncodeToString(h.Sum(nil))
	return out, nil
}

func writeResult(dir string, outcome *result) error {
	data, err := json.MarshalIndent(outcome, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "result.json")
	if err := os.WriteFile(path+".tmp", append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func applyPlan(ctx context.Context, tx pgx.Tx, plan []change) error {
	_, err := tx.Exec(ctx, `CREATE TEMP TABLE pusdatin_import_stage (
 action text NOT NULL, id uuid, name text NOT NULL, code text NOT NULL, npsn text NOT NULL,
 school_types text[] NOT NULL, alamat text, provinsi_id text NOT NULL, kota_id text NOT NULL) ON COMMIT DROP`)
	if err != nil {
		return err
	}
	changes := make([][]any, 0)
	inserts, updates := int64(0), int64(0)
	for _, c := range plan {
		if c.Action != "insert" && c.Action != "update" {
			continue
		}
		s := c.After
		var id any
		if c.Action == "update" {
			id = s.ID
			updates++
		} else {
			inserts++
		}
		changes = append(changes, []any{c.Action, id, s.Name, s.Code, s.NPSN, s.SchoolTypes, s.Alamat, s.ProvinsiID, s.KotaID})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"pusdatin_import_stage"}, []string{"action", "id", "name", "code", "npsn", "school_types", "alamat", "provinsi_id", "kota_id"}, pgx.CopyFromRows(changes)); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE school s SET school_types=p.school_types,alamat=p.alamat,
 provinsi_id=p.provinsi_id,kota_id=p.kota_id,updated_at=now()
 FROM pusdatin_import_stage p WHERE p.action='update' AND s.id=p.id`)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != updates {
		return errors.New("update count mismatch; rolling back")
	}
	tag, err = tx.Exec(ctx, `INSERT INTO school (name,code,npsn,school_types,alamat,provinsi_id,kota_id,status)
 SELECT name,code,npsn,school_types,alamat,provinsi_id,kota_id,'active' FROM pusdatin_import_stage WHERE action='insert'`)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != inserts {
		return errors.New("insert count mismatch; rolling back")
	}
	var mismatches int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM pusdatin_import_stage p
 LEFT JOIN school s ON upper(btrim(s.npsn))=upper(btrim(p.npsn))
 WHERE s.id IS NULL OR s.name IS DISTINCT FROM p.name OR s.code IS DISTINCT FROM p.code
 OR s.school_types IS DISTINCT FROM p.school_types OR s.alamat IS DISTINCT FROM p.alamat
 OR s.provinsi_id IS DISTINCT FROM p.provinsi_id OR s.kota_id IS DISTINCT FROM p.kota_id
 OR (p.action='update' AND s.id<>p.id)`).Scan(&mismatches)
	if err != nil {
		return err
	}
	if mismatches != 0 {
		return fmt.Errorf("%d persisted rows differ from plan; rolling back", mismatches)
	}
	return nil
}
