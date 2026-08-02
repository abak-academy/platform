package service

// Findings 2/3 (external review of PR #69, 2026-08 review): saving a
// certificate design/template or enabling certificates must reach
// already-submitted sessions, not just future ones. These tests drive the
// real transactional fan-out through the actual service call and assert on
// a real outbox row — not a struct round-trip, the exact shape of proof
// Finding 1 shows can pass while the real column stays untouched.
//
// They stop at "an unprocessed CertificateNeeded row now targets this
// session" rather than also driving GenerateCertificatePDF through to a
// persisted certificate_key: uploadCertificatePDF makes a real PutObject
// call against s.storage (a concrete *minio.Client, no fake seam), which
// needs a live, correctly-credentialed MinIO — exactly what
// certificate_render_gate_test.go's gotenberg_integration build tag exists
// to keep out of the plain `go test ./...` suite. GenerateCertificatePDF's
// own correctness once an event fires is already proven there
// (TestCertificateRender_ThroughWorker) and in this file's
// TestGenerateCertificatePDF_* no-op cases; what Findings 2/3 were missing
// was the event ever being enqueued for an already-submitted session, which
// is what these tests prove.

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// countUnprocessedCertificateNeeded returns how many unprocessed
// "CertificateNeeded" outbox rows target sessionID.
func countUnprocessedCertificateNeeded(t *testing.T, svc *Service, sessionID uuid.UUID) int {
	t.Helper()
	var n int
	err := svc.storeRepo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND event_type = 'CertificateNeeded' AND processed_at IS NULL`,
		sessionID,
	).Scan(&n)
	if err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	return n
}

// TestUpdateExam_SavingTemplate_FansOutToAlreadySubmittedSessions proves
// Finding 2: a session that submitted while the exam had no saved
// certificate template must still get a certificate once an admin saves one
// — not just sessions that submit afterward.
func TestUpdateExam_SavingTemplate_FansOutToAlreadySubmittedSessions(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Fanout Design " + uuid.NewString(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	// Submitted before any template was ever saved — the exact session the
	// old code path abandoned forever.
	sessID := genCertSeedRow(t, pool, exam.ID, true, 8, true)

	if got := countUnprocessedCertificateNeeded(t, svc, sessID); got != 0 {
		t.Fatalf("outbox CertificateNeeded rows before save = %d, want 0", got)
	}

	detail, err := svc.GetExam(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetExam: %v", err)
	}
	overlay := detail.Exam
	html := `<div>{{student_name}}</div>`
	overlay.CertificateTemplateHTML = &html
	if _, err := svc.UpdateExam(ctx, exam.ID, overlay); err != nil {
		t.Fatalf("UpdateExam: %v", err)
	}

	if got := countUnprocessedCertificateNeeded(t, svc, sessID); got != 1 {
		t.Fatalf("outbox CertificateNeeded rows after save = %d, want 1 (fan-out to the already-submitted session) — without it, this session is abandoned forever", got)
	}
}

// TestUpdateExam_UnrelatedFieldEdit_DoesNotFanOut proves the fan-out is
// scoped to an actual design/template change — an unrelated PATCH (e.g.
// title) must not spam the outbox for every submitted session on every save.
func TestUpdateExam_UnrelatedFieldEdit_DoesNotFanOut(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Fanout Noop " + uuid.NewString(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	sessID := genCertSeedRow(t, pool, exam.ID, true, 8, true)

	detail, err := svc.GetExam(ctx, exam.ID)
	if err != nil {
		t.Fatalf("GetExam: %v", err)
	}
	overlay := detail.Exam
	overlay.Title = "Fanout Noop Renamed " + uuid.NewString()
	if _, err := svc.UpdateExam(ctx, exam.ID, overlay); err != nil {
		t.Fatalf("UpdateExam: %v", err)
	}

	if got := countUnprocessedCertificateNeeded(t, svc, sessID); got != 0 {
		t.Fatalf("outbox CertificateNeeded rows after an unrelated field edit = %d, want 0", got)
	}
}

// TestSetExamCertificateEnabled_FansOutToAlreadySubmittedSessions proves
// Finding 3: a session submitted while certificates were disabled must
// still end up with a certificate once an admin enables them — the exact
// "not-ready session silently abandoned forever" scenario the review named.
func TestSetExamCertificateEnabled_FansOutToAlreadySubmittedSessions(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Fanout Enable " + uuid.NewString(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	html := `<div>{{student_name}}</div>`
	if _, err := pool.Exec(ctx, `UPDATE exam SET certificate_template_html = $1 WHERE id = $2`, html, exam.ID); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	// certificate_enabled=false at submission time — the exam gains
	// certificates only after this session already exists.
	sessID := genCertSeedRow(t, pool, exam.ID, false, 8, true)

	if got := countUnprocessedCertificateNeeded(t, svc, sessID); got != 0 {
		t.Fatalf("outbox CertificateNeeded rows before enabling = %d, want 0", got)
	}

	if _, err := svc.SetExamCertificateEnabled(ctx, exam.ID, true); err != nil {
		t.Fatalf("SetExamCertificateEnabled: %v", err)
	}

	if got := countUnprocessedCertificateNeeded(t, svc, sessID); got != 1 {
		t.Fatalf("outbox CertificateNeeded rows after enabling = %d, want 1 (fan-out to the already-submitted session) — without it, this session is abandoned forever", got)
	}
}

// TestSetExamCertificateEnabled_RedundantEnable_DoesNotReFanOut proves the
// off->on transition guard: enabling an already-enabled exam a second time
// must not re-flood the outbox for every submitted session.
func TestSetExamCertificateEnabled_RedundantEnable_DoesNotReFanOut(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Fanout Redundant " + uuid.NewString(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	sessID := genCertSeedRow(t, pool, exam.ID, true, 8, true)

	if _, err := svc.SetExamCertificateEnabled(ctx, exam.ID, true); err != nil {
		t.Fatalf("SetExamCertificateEnabled (first): %v", err)
	}
	// Drain the first fan-out's row so the second call's effect is isolated.
	if _, err := pool.Exec(ctx, `UPDATE outbox SET processed_at = now() WHERE aggregate_id = $1`, sessID); err != nil {
		t.Fatalf("drain outbox: %v", err)
	}

	if _, err := svc.SetExamCertificateEnabled(ctx, exam.ID, true); err != nil {
		t.Fatalf("SetExamCertificateEnabled (second, redundant): %v", err)
	}

	if got := countUnprocessedCertificateNeeded(t, svc, sessID); got != 0 {
		t.Fatalf("outbox CertificateNeeded rows after a redundant enable = %d, want 0", got)
	}
}
