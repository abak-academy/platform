package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"akademi-bimbel/internal/model"
)

// TestSubstituteTemplateTokens_ReplacesKnownTokenLeavesLiteralTextAlone is the
// Done-when proof for the worker's substitution step (async redesign
// 2026-08-02): {{score}} resolves to the verified DB value, and a literal
// number elsewhere in the template (standing in for a hand-typed/hardcoded
// value in the FE-authored template) is left completely untouched — nothing
// but a recognized {{token}} spot is ever rewritten.
func TestSubstituteTemplateTokens_ReplacesKnownTokenLeavesLiteralTextAlone(t *testing.T) {
	html := `<div>{{score}}</div><div>100 fixed</div>`
	values := map[FieldID]string{"score": "42"}

	got := substituteTemplateTokens(html, values)

	want := `<div>42</div><div>100 fixed</div>`
	if got != want {
		t.Errorf("substituteTemplateTokens = %q, want %q", got, want)
	}
}

// TestSubstituteTemplateTokens_UnknownTokenBlanked proves a token with no
// corresponding DB value is blanked rather than left as literal {{...}} text
// leaking onto the rendered document.
func TestSubstituteTemplateTokens_UnknownTokenBlanked(t *testing.T) {
	got := substituteTemplateTokens(`<div>{{unknown_token}}</div>`, map[FieldID]string{})
	if got != `<div></div>` {
		t.Errorf("substituteTemplateTokens = %q, want the unknown token blanked", got)
	}
}

// TestSubstituteTemplateTokens_EscapesValue proves a value is HTML-escaped
// before insertion — the template is FE-authored and trusted, but the value
// came from a database column (e.g. a student's name) an admin may have
// typed HTML-looking characters into.
func TestSubstituteTemplateTokens_EscapesValue(t *testing.T) {
	got := substituteTemplateTokens(`<div>{{student_name}}</div>`, map[FieldID]string{
		"student_name": `<script>alert(1)</script>`,
	})
	if strings.Contains(got, "<script>") {
		t.Errorf("substituteTemplateTokens did not escape the value: %q", got)
	}
}

// ---------- tests: GenerateCertificatePDF (async redesign 2026-08-02) ----------
//
// GenerateCertificatePDF is the only place a certificate is ever generated —
// these prove it never calls the renderer for a session that isn't ready,
// reusing the testcontainers harness (gateTestPool/gateTestService) from
// certificate_gate_db_test.go.

func TestGenerateCertificatePDF_UnknownSession_NoOp(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)

	if err := svc.GenerateCertificatePDF(context.Background(), uuid.New()); err != nil {
		t.Fatalf("GenerateCertificatePDF: %v", err)
	}
	if renderer.calls != 0 {
		t.Errorf("renderer calls = %d, want 0 — an unknown session has nothing to generate", renderer.calls)
	}
}

// genCertSeedRow inserts a minimal real user + exam + exam_registration +
// exam_session row directly, returning the session id. Kept to raw SQL
// (mirrors seedStudentDirect's pattern in exam_test.go) rather than pulling
// in the full student/registration service flow, which this test has no use
// for beyond satisfying exam_session's foreign keys.
func genCertSeedRow(t *testing.T, pool *pgxpool.Pool, examID uuid.UUID, certEnabled bool, score float64, submitted bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var studentID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status, password_hash) VALUES ($1, $2, 'student', 'active', '') RETURNING id`,
		"Gen Cert Student", "gencert-"+uuid.NewString(),
	).Scan(&studentID)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE exam SET certificate_enabled = $1 WHERE id = $2`, certEnabled, examID); err != nil {
		t.Fatalf("set certificate_enabled: %v", err)
	}

	var regID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status) VALUES ($1, $2, $3, 'registered') RETURNING id`,
		studentID, examID, "TOKEN"+uuid.NewString(),
	).Scan(&regID)
	if err != nil {
		t.Fatalf("insert registration: %v", err)
	}

	status := "in_progress"
	var submittedAt *time.Time
	if submitted {
		status = "submitted"
		now := time.Now()
		submittedAt = &now
	}
	var sessID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, started_at, submitted_at, status, score)
		 VALUES ($1, $2, $3, now(), $4, $5, $6) RETURNING id`,
		regID, studentID, examID, submittedAt, status, score,
	).Scan(&sessID)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return sessID
}

func TestGenerateCertificatePDF_NotSubmitted_NoOp(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Gen Cert Not Submitted " + uuid.NewString(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	sessID := genCertSeedRow(t, pool, exam.ID, true, 0, false)

	if err := svc.GenerateCertificatePDF(ctx, sessID); err != nil {
		t.Fatalf("GenerateCertificatePDF: %v", err)
	}
	if renderer.calls != 0 {
		t.Errorf("renderer calls = %d, want 0 — an in-progress session must never generate", renderer.calls)
	}
}

func TestGenerateCertificatePDF_CertificateDisabled_NoOp(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Gen Cert Disabled " + uuid.NewString(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	sessID := genCertSeedRow(t, pool, exam.ID, false, 8, true)

	if err := svc.GenerateCertificatePDF(ctx, sessID); err != nil {
		t.Fatalf("GenerateCertificatePDF: %v", err)
	}
	if renderer.calls != 0 {
		t.Errorf("renderer calls = %d, want 0 — certificate_enabled=false must never generate", renderer.calls)
	}
}

// TestGenerateCertificatePDF_NoTemplateSaved_NoOp proves a submitted,
// enabled exam with no certificate_template_html saved yet (the admin has
// never saved a design) is a no-op rather than an error — there is nothing
// to substitute into.
func TestGenerateCertificatePDF_NoTemplateSaved_NoOp(t *testing.T) {
	pool, _ := gateTestPool(t)
	renderer := &gateFakeRenderer{}
	svc := gateTestService(t, pool, renderer)
	ctx := context.Background()

	exam, err := svc.CreateExam(ctx, model.Exam{Title: "Gen Cert No Template " + uuid.NewString(), CertificateDesign: certDesignJSON("classic")})
	if err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	sessID := genCertSeedRow(t, pool, exam.ID, true, 8, true)

	if err := svc.GenerateCertificatePDF(ctx, sessID); err != nil {
		t.Fatalf("GenerateCertificatePDF: %v", err)
	}
	if renderer.calls != 0 {
		t.Errorf("renderer calls = %d, want 0 — no saved template means nothing to render", renderer.calls)
	}
}
