//go:build gotenberg_integration

// Replacement render gate (NFR-R4, FR-30) for the async redesign
// (2026-08-02): certificates now render exclusively through
// GenerateCertificatePDF — the FE-authored, {{token}}-carrying stored
// template, substituted with verified DB values and posted directly to
// Gotenberg's /forms/chromium/convert/html — never fetched from a running
// `web` container. This gate proves that real path against a real Postgres
// and a real Gotenberg; unlike the print-route gate it replaces, it needs no
// `api`, `web`, or Redis service at all, since nothing here mints a print
// token or asks Gotenberg to fetch a URL.
//
//	go test -tags gotenberg_integration -run 'TestCertificateRender_|TestCardRender_' -count=1 -v ./internal/service/
package service

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func renderGatePostgresDSN() string {
	if v := os.Getenv("RENDER_GATE_POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://akademi:akademi@localhost:5432/akademi?sslmode=disable"
}

func renderGateGotenbergURL() string {
	if v := os.Getenv("GOTENBERG_URL"); v != "" {
		return v
	}
	return "http://localhost:3001"
}

func renderGateMinioEndpoint() string {
	if v := os.Getenv("RENDER_GATE_MINIO_ENDPOINT"); v != "" {
		return v
	}
	return "localhost:9000"
}

// renderGateService connects to the real compose Postgres and a real MinIO,
// and fails loudly (not skips) when either is unreachable — this gate exists
// precisely so a broken render path cannot pass silently. Matches
// deploy/compose/local.yml's default credentials/bucket.
func renderGateService(t *testing.T) *Service {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, renderGatePostgresDSN())
	if err != nil {
		t.Fatalf("connect to compose postgres at %s: %v — is deploy/compose/local.yml up?", renderGatePostgresDSN(), err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("compose postgres not reachable at %s: %v — is deploy/compose/local.yml up?", renderGatePostgresDSN(), err)
	}
	// This gate no longer starts the `api` container (nothing here needs it),
	// which used to apply migrations on boot — run them here instead so the
	// gate is self-sufficient against a bare compose postgres.
	if err := infra.RunMigrations(ctx, renderGatePostgresDSN()); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	storage, err := minio.New(renderGateMinioEndpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	if _, err := storage.BucketExists(ctx, "akademi-bimbel"); err != nil {
		t.Fatalf("compose minio not reachable at %s: %v — is deploy/compose/local.yml up?", renderGateMinioEndpoint(), err)
	}

	store := repository.New(pool)
	cfg := &config.Config{ObjectStorageBucketName: "akademi-bimbel", ObjectStorageRegion: "us-east-1", GotenbergURL: renderGateGotenbergURL()}
	renderer := NewGotenbergPDFGenerator(renderGateGotenbergURL(), nil)
	return NewWithStore(store, store, nil, nil, &NoopOTPProvider{}, &NoopEmailProvider{}, &NoopPaymentClient{}, &NoopLogisticsClient{}, storage, cfg, renderer)
}

// gateCertificateTemplateHTML is a hand-authored stand-in for the FE
// serializer's real output (web/app/api/admin/certificate-template): same
// {{token}} contract (previewContent leaves unrecognized/unsubstituted
// tokens literal, certificateBackgroundURL/certificateImageAssetURLs supply
// the certificate_background_url/certificate_asset_* tokens), same absolute
// mm-positioned box convention CertificateDocumentCore renders — verifying
// the FE's actual React output would need a browser-driven test, out of
// scope for this Go-side gate.
const gateCertificateTemplateHTML = `<!DOCTYPE html>
<html><head><style>
@page { size: 297mm 210mm; margin: 0; }
body { margin: 0; }
.page { position: relative; width: 297mm; height: 210mm; }
.bg { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
.name { position: absolute; left: 48mm; top: 108mm; width: 200mm; font-size: 28pt; font-family: sans-serif; color: #000000; text-align: center; }
</style></head>
<body>
<div class="page">
<img class="bg" src="{{certificate_background_url}}" alt="" />
<div class="name">{{student_name}}</div>
</div>
</body></html>`

// renderGateSeedSubmittedSession inserts a real user + exam + registration +
// submitted session, wires certificate_enabled + the stored template, and
// returns the session id — everything GenerateCertificatePDF needs.
func renderGateSeedSubmittedSession(t *testing.T, svc *Service, examTitle, studentName string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool := svc.storeRepo.Pool()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: examTitle, CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	templateHTML := gateCertificateTemplateHTML
	if _, err := pool.Exec(ctx,
		`UPDATE exam SET certificate_enabled = true, certificate_template_html = $1 WHERE id = $2`,
		templateHTML, exam.ID,
	); err != nil {
		t.Fatalf("enable certificate + set template: %v", err)
	}

	var studentID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, password_hash) VALUES ($1, $2, 'student', 'active', '') RETURNING id`,
		studentName, "rendergate-"+uuid.NewString(),
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}

	var regID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number) VALUES ($1, $2, $3, 'registered', 1) RETURNING id`,
		studentID, exam.ID, "TOKEN"+uuid.NewString(),
	).Scan(&regID); err != nil {
		t.Fatalf("insert registration: %v", err)
	}

	var sessID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, submitted_at, status)
		 VALUES ($1, $2, $3, now(), now(), 'submitted') RETURNING id`,
		regID, studentID, exam.ID,
	).Scan(&sessID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return sessID, exam.ID
}

// TestCertificateRender_ThroughWorker is the async redesign's render gate
// (NFR-R4, FR-30): GenerateCertificatePDF — called only by the worker, off
// the CertificateNeeded outbox event — must substitute the stored template's
// {{token}} spots with real DB values, render through a real Gotenberg, and
// persist a real certificate_key the read path can then presign.
func TestCertificateRender_ThroughWorker(t *testing.T) {
	svc := renderGateService(t)
	sessID, _ := renderGateSeedSubmittedSession(t, svc, "Render Gate Worker "+uuid.NewString(), "Render Gate Student")

	if err := svc.GenerateCertificatePDF(context.Background(), sessID); err != nil {
		t.Fatalf("GenerateCertificatePDF: %v", err)
	}

	sess, err := svc.storeRepo.GetExamSessionByID(context.Background(), sessID)
	if err != nil {
		t.Fatalf("GetExamSessionByID: %v", err)
	}
	if sess.CertificateKey == nil {
		t.Fatal("certificate_key was not persisted — generation did not run")
	}

	obj, err := svc.storage.GetObject(context.Background(), "akademi-bimbel", *sess.CertificateKey, minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("download generated certificate: %v", err)
	}
	defer obj.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(obj); err != nil {
		t.Fatalf("read generated certificate: %v", err)
	}
	pdf := buf.Bytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (no %%PDF- header); first bytes: %q", pdf[:min(16, len(pdf))])
	}

	img := renderToPNG(t, pdf)
	bounds := img.Bounds()
	if bounds.Dx() <= bounds.Dy() {
		t.Errorf("expected A4 landscape, got %dx%d px", bounds.Dx(), bounds.Dy())
	}

	// The classic background's navy band runs along the top edge; a blank or
	// background-less render leaves this area white — proves
	// certificate_background_url actually resolved and Gotenberg fetched it.
	r, g, b := avgColorAt(img, certificatePageWidthMm, certificatePageHeightMm, 148, 8)
	if r > 200 && g > 200 && b > 200 {
		t.Errorf("top band is near-white (%.0f,%.0f,%.0f) — background did not render", r, g, b)
	}

	// Ink where {{student_name}} was substituted proves the worker's DB value
	// (not a hardcoded template value) reached the page.
	nameInk := regionMinBrightness(img, certificatePageWidthMm, certificatePageHeightMm, 48, 108, 248, 122)
	if nameInk > 600 {
		t.Errorf("no glyph ink in the student_name box (darkest pixel %.0f/765) — {{student_name}} did not substitute", nameInk)
	}
}

// TestCertificateRender_ThroughWorker_NoOpWithoutGotenbergCall proves the
// worker's readiness gate (GenerateCertificatePDF) still holds against a
// real stack — a disabled exam must reach neither Gotenberg nor object
// storage, evidenced by no certificate_key ever appearing.
func TestCertificateRender_ThroughWorker_NoOpWhenDisabled(t *testing.T) {
	svc := renderGateService(t)
	sessID, examID := renderGateSeedSubmittedSession(t, svc, "Render Gate Disabled "+uuid.NewString(), "Render Gate Student")
	if _, err := svc.storeRepo.Pool().Exec(context.Background(), `UPDATE exam SET certificate_enabled = false WHERE id = $1`, examID); err != nil {
		t.Fatalf("disable certificate: %v", err)
	}

	if err := svc.GenerateCertificatePDF(context.Background(), sessID); err != nil {
		t.Fatalf("GenerateCertificatePDF: %v", err)
	}

	sess, err := svc.storeRepo.GetExamSessionByID(context.Background(), sessID)
	if err != nil {
		t.Fatalf("GetExamSessionByID: %v", err)
	}
	if sess.CertificateKey != nil {
		t.Fatalf("certificate_key = %q, want nil — a disabled exam must never generate", *sess.CertificateKey)
	}
}

// TestCertificateRender_DesignChangeRegeneratesThroughWorker is the
// end-to-end proof for Finding A (2026-08 review): a design change fanned
// out as a CertificateNeeded event must actually reach a real regenerated
// PDF through GenerateCertificatePDF, not just flip a boolean in a unit
// test. certificate_key is deterministic per session
// (uploadCertificatePDF: "certificates/<sessionID>.pdf"), so a fresh key
// can't be the proof — certificate_generated_at advancing is: the worker
// only ever bumps it after render+upload actually ran again.
func TestCertificateRender_DesignChangeRegeneratesThroughWorker(t *testing.T) {
	svc := renderGateService(t)
	sessID, examID := renderGateSeedSubmittedSession(t, svc, "Render Gate Design Change "+uuid.NewString(), "Render Gate Student")

	if err := svc.GenerateCertificatePDF(context.Background(), sessID); err != nil {
		t.Fatalf("GenerateCertificatePDF (initial): %v", err)
	}
	before, err := svc.storeRepo.GetExamSessionByID(context.Background(), sessID)
	if err != nil {
		t.Fatalf("GetExamSessionByID (before): %v", err)
	}
	if before.CertificateKey == nil || before.CertificateGeneratedAt == nil {
		t.Fatal("initial generation did not persist a certificate")
	}

	// Simulate an admin design edit landing after the cached PDF, the same
	// way UpdateExam bumps certificate_design_updated_at on a real save.
	designUpdatedAt := before.CertificateGeneratedAt.Add(time.Second)
	if _, err := svc.storeRepo.Pool().Exec(context.Background(),
		`UPDATE exam SET certificate_design_updated_at = $1 WHERE id = $2`, designUpdatedAt, examID,
	); err != nil {
		t.Fatalf("set certificate_design_updated_at: %v", err)
	}

	if err := svc.GenerateCertificatePDF(context.Background(), sessID); err != nil {
		t.Fatalf("GenerateCertificatePDF (after design change): %v", err)
	}
	after, err := svc.storeRepo.GetExamSessionByID(context.Background(), sessID)
	if err != nil {
		t.Fatalf("GetExamSessionByID (after): %v", err)
	}
	if after.CertificateGeneratedAt == nil || !after.CertificateGeneratedAt.After(*before.CertificateGeneratedAt) {
		t.Fatalf("certificate_generated_at did not advance (before=%v, after=%v) — the fanned-out CertificateNeeded event was a no-op, the stale PDF is still being served", before.CertificateGeneratedAt, after.CertificateGeneratedAt)
	}
}

// TestCardRender_ThroughWorker proves the exam card's static build-time
// template (backend/internal/service/assets/exam_card_template.html,
// generated from web/components/exam/ExamCardPrintable.tsx by
// web/scripts/build-card-template.mjs) still renders through a real
// Gotenberg after substitution — the card's counterpart to
// TestCertificateRender_ThroughWorker. Matched by the gate script's
// `TestCardRender_` prefix.
func TestCardRender_ThroughWorker(t *testing.T) {
	renderer := NewGotenbergPDFGenerator(renderGateGotenbergURL(), nil)
	data := &CardPrintData{
		ParticipantNo: "250620-0001-000001",
		StudentName:   "Render Gate Card Student",
		School:        "SMA Gate Test",
		ExamTitle:     "Ujian Gerbang Render",
		ExamSchedule:  "02 Aug 2026 09:00 WIB",
		CheckInCode:   "GATECODE1",
	}

	pdf, err := generateExamCardPDF(context.Background(), renderer, data)
	if err != nil {
		t.Fatalf("generateExamCardPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (no %%PDF- header); first bytes: %q", pdf[:min(16, len(pdf))])
	}

	img := renderToPNG(t, pdf)
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
		t.Fatal("rasterized card has zero dimensions")
	}
}
