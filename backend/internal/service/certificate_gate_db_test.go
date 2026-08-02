package service

// Regression test for issue #55 (spec FR-1..FR-6, NFR-P1): resolveCertificateURL
// must apply certificateLayoutAllowed on every access, not only inside the
// needsRegeneration branch. It runs against the real service.Service (not
// shimSessionService, which has no gate at all — see certificate_test.go:121)
// because Service.storeRepo is the concrete *repository.Repository, so the gate
// can only be proven with a live Postgres. Harness shape copied from
// exam_grading_test.go:20.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// gateQueryCounter is a pgx.QueryTracer that counts calls to the exact SQL
// GetSessionWithQuestions issues, identified by its "SELECT et.test_id" text
// (unique across the repository package as of this writing — grep before
// trusting it after a refactor). Service.storeRepo is the concrete
// *repository.Repository, so there is no fake repo to assert call counts on;
// this is the substitute proof that a non-score layout on the cache path
// performs zero extra queries (NFR-P1).
type gateQueryCounter struct {
	mu sync.Mutex
	n  int
}

func (c *gateQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "SELECT et.test_id") {
		c.mu.Lock()
		c.n++
		c.mu.Unlock()
	}
	return ctx
}

func (c *gateQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (c *gateQueryCounter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func (c *gateQueryCounter) reset() {
	c.mu.Lock()
	c.n = 0
	c.mu.Unlock()
}

// gateFakeRenderer is the PDFGenerator seam, injected so this test never
// talks to a real Gotenberg. Its call count is the proof that a cache hit
// never regenerates (NFR-P2) — resolveCertificateURL is read-only and must
// never call the renderer at all (async redesign 2026-08-02).
type gateFakeRenderer struct {
	calls int
}

func (r *gateFakeRenderer) RenderHTML(context.Context, []byte) ([]byte, error) {
	r.calls++
	return []byte("%PDF-1.4"), nil
}

// gateTestPool starts an ephemeral Postgres, applies migrations, and wires a
// query tracer so tests can prove how many times GetSessionWithQuestions ran.
func gateTestPool(t *testing.T) (*pgxpool.Pool, *gateQueryCounter) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("akademi_gate_test"),
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

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	counter := &gateQueryCounter{}
	poolCfg.ConnConfig.Tracer = counter

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool, counter
}

// gateTestService builds a real service.Service backed by the testcontainers
// pool, a locally-signing MinIO client (region set explicitly so presigning
// never needs a reachable endpoint, mirroring
// exam_certificate_handler_test.go:115), a miniredis-backed rdb (Task 13's
// renderCertificateThroughPrintRoute mints a print token via s.rdb before it
// ever reaches the renderer), and the given fake PDF renderer.
func gateTestService(t *testing.T, pool *pgxpool.Pool, renderer PDFGenerator) *Service {
	t.Helper()
	store := repository.New(pool)
	storage, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("test-access", "test-secret", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{
		ObjectStorageBucketName: "test-bucket",
		ObjectStorageRegion:     "us-east-1",
	}
	return NewWithStore(
		store, store, rdb, nil,
		&NoopOTPProvider{}, &NoopEmailProvider{},
		&NoopPaymentClient{}, &NoopLogisticsClient{},
		storage, cfg, renderer,
	)
}

// gateCachedSession builds a submitted session whose certificate is already
// cached (certificate_key/certificate_generated_at set, nothing stale) — the
// exact state that let the pre-fix gate be skipped entirely. ExamID is a fresh
// UUID with no matching exam_test rows; GetSessionWithQuestions still runs a
// real query against it (proving the tracer sees it), it just returns none.
func gateCachedSession() *model.ExamSession {
	submittedAt := time.Now().Add(-time.Hour)
	generatedAt := time.Now().Add(-30 * time.Minute)
	score := 8.0
	certKey := "certificates/" + uuid.NewString() + ".pdf"
	return &model.ExamSession{
		ID:                     uuid.New(),
		ExamID:                 uuid.New(),
		StudentID:              uuid.New(),
		StartedAt:              submittedAt,
		SubmittedAt:            &submittedAt,
		Status:                 "submitted",
		Score:                  &score,
		CertificateKey:         &certKey,
		CertificateGeneratedAt: &generatedAt,
	}
}

// freshUncachedSession builds a submitted session with no cached certificate
// (CertificateKey nil) — the path that would normally regenerate — so a
// certificate_enabled=false test proves the gate short-circuits before any
// generation work, not just on the cache-hit path.
func freshUncachedSession() *model.ExamSession {
	submittedAt := time.Now().Add(-time.Hour)
	score := 8.0
	return &model.ExamSession{
		ID:          uuid.New(),
		ExamID:      uuid.New(),
		StudentID:   uuid.New(),
		StartedAt:   submittedAt,
		SubmittedAt: &submittedAt,
		Status:      "submitted",
		Score:       &score,
	}
}

func TestResolveCertificateURL_CachedGate(t *testing.T) {
	pool, counter := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	t.Run("FR-1: cached score-bearing certificate is denied when result_config is hidden", func(t *testing.T) {
		counter.reset()
		exam := &model.Exam{Title: "Gate Exam", ResultConfig: "hidden", CertificateDesign: certDesignJSON("modern"), CertificateEnabled: true}
		sess := gateCachedSession()

		url, err := svc.resolveCertificateURL(ctx, exam, sess, nil)
		if err != nil {
			t.Fatalf("resolveCertificateURL: %v", err)
		}
		if url != nil {
			t.Fatalf("certificate_url = %q, want nil — a hidden result_config must gate an already-cached score-bearing certificate at access time, not only at generation time", *url)
		}
		if got := counter.count(); got != 1 {
			t.Errorf("GetSessionWithQuestions calls = %d, want 1 (one query to evaluate the gate for a score-bearing layout)", got)
		}
	})

	t.Run("FR-2: cached score-bearing certificate is served without regeneration when result_config permits", func(t *testing.T) {
		counter.reset()
		renderer.calls = 0
		exam := &model.Exam{Title: "Gate Exam", ResultConfig: "score_only", CertificateDesign: certDesignJSON("modern"), CertificateEnabled: true}
		sess := gateCachedSession()

		url, err := svc.resolveCertificateURL(ctx, exam, sess, nil)
		if err != nil {
			t.Fatalf("resolveCertificateURL: %v", err)
		}
		if url == nil || *url == "" {
			t.Fatalf("certificate_url = %v, want a presigned URL", url)
		}
		if renderer.calls != 0 {
			t.Errorf("PDF renderer calls = %d, want 0 (a cached certificate must never regenerate, NFR-P2)", renderer.calls)
		}
	})

	t.Run("FR-3/NFR-P1: cached non-score layout skips the gate and issues zero extra queries", func(t *testing.T) {
		counter.reset()
		exam := &model.Exam{Title: "Gate Exam", ResultConfig: "hidden", CertificateDesign: certDesignJSON("classic"), CertificateEnabled: true}
		sess := gateCachedSession()

		url, err := svc.resolveCertificateURL(ctx, exam, sess, nil)
		if err != nil {
			t.Fatalf("resolveCertificateURL: %v", err)
		}
		if url == nil || *url == "" {
			t.Fatalf("certificate_url = %v, want a presigned URL — a non-score layout is never gated, even under result_config=hidden", url)
		}
		if got := counter.count(); got != 0 {
			t.Errorf("GetSessionWithQuestions calls = %d, want 0 — a non-score layout on the cache path must not read session questions", got)
		}
	})

	t.Run("FR-10: certificate_enabled=false denies an uncached session before any regeneration work", func(t *testing.T) {
		renderer.calls = 0
		exam := &model.Exam{Title: "Disabled Cert Exam", ResultConfig: "score_only", CertificateDesign: certDesignJSON("classic"), CertificateEnabled: false}
		sess := freshUncachedSession()

		url, err := svc.resolveCertificateURL(ctx, exam, sess, nil)
		if err != nil {
			t.Fatalf("resolveCertificateURL: %v", err)
		}
		if url != nil {
			t.Fatalf("certificate_url = %q, want nil — certificate_enabled=false must gate access regardless of result_config", *url)
		}
		if renderer.calls != 0 {
			t.Errorf("PDF renderer calls = %d, want 0 — a disabled exam must never generate a certificate", renderer.calls)
		}
	})

	t.Run("FR-10: certificate_enabled=false denies an already-cached, permitted certificate", func(t *testing.T) {
		renderer.calls = 0
		exam := &model.Exam{Title: "Disabled Cert Exam", ResultConfig: "score_only", CertificateDesign: certDesignJSON("classic"), CertificateEnabled: false}
		sess := gateCachedSession()

		url, err := svc.resolveCertificateURL(ctx, exam, sess, nil)
		if err != nil {
			t.Fatalf("resolveCertificateURL: %v", err)
		}
		if url != nil {
			t.Fatalf("certificate_url = %q, want nil — certificate_enabled=false must gate even an already-cached certificate", *url)
		}
	})
}

// TestResolveCertificateURL_NeverRenders proves the read path never
// generates (async redesign 2026-08-02, superseding this file's old
// TestRenderCertificateThroughPrintRoute_* tests, which exercised
// renderCertificateThroughPrintRoute/RenderURL — both deleted along with the
// print-token/print-route architecture). An uncached session (no
// certificate_key) must resolve to nil without ever calling the renderer;
// GenerateCertificatePDF (called only by the worker, off the
// CertificateNeeded outbox event) is the only place a certificate is ever
// generated.
func TestResolveCertificateURL_NeverRenders(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam := &model.Exam{Title: "Never Renders Exam", ResultConfig: "score_only", CertificateDesign: certDesignJSON("classic"), CertificateEnabled: true}
	sess := freshUncachedSession()

	url, err := svc.resolveCertificateURL(ctx, exam, sess, nil)
	if err != nil {
		t.Fatalf("resolveCertificateURL: %v", err)
	}
	if url != nil {
		t.Fatalf("certificate_url = %q, want nil — an uncached session has nothing to presign yet", *url)
	}
	if renderer.calls != 0 {
		t.Errorf("PDF renderer calls = %d, want 0 — the read path must never generate", renderer.calls)
	}
}
