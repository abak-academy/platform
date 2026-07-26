package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"akademi-bimbel/internal/model"
)

// insertMCQQuestion inserts a non-essay question for leaderboard/analytics tests
// where the fullyGradedFilter should always pass (no essay to check).
func insertMCQQuestion(t *testing.T, pool *pgxpool.Pool, testID uuid.UUID, body string, pointCorrect, sortOrder int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong)
		VALUES ('mcq', $1, $2, 0) RETURNING id`,
		body, pointCorrect,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert mcq question: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`,
		testID, id, sortOrder,
	); err != nil {
		t.Fatalf("insert test_question: %v", err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Leaderboard
// ---------------------------------------------------------------------------

// TestLeaderboard_ListExamLeaderboard verifies score-descending order, ties sharing
// rank, exclusion of ungraded/non-submitted sessions, and cursor-pagination metadata.
func TestLeaderboard_ListExamLeaderboard(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	admin := insertGradingUser(t, pool, "admin_exam", "Grader Admin")
	studentA := insertGradingUser(t, pool, "student", "Student A")
	studentB := insertGradingUser(t, pool, "student", "Student B")
	studentC := insertGradingUser(t, pool, "student", "Student C")
	studentD := insertGradingUser(t, pool, "student", "Student D")
	studentE := insertGradingUser(t, pool, "student", "Student E")

	testID := insertGradingTest(t, pool)
	essayQID := insertGradingEssayQuestion(t, pool, testID, "Explain gravity", 10, 1)
	examID := insertGradingExam(t, pool, testID)

	now := time.Now()

	// Session A: submitted, score 90, essay graded → fully graded, appears.
	sessionA := insertGradingSession(t, pool, studentA, examID, "submitted", &now, f64PtrG(90))
	insertGradingAnswer(t, pool, sessionA, essayQID, strPtrG("a answer"), f64PtrG(10), &admin, timePtrG(now))

	// Session B: submitted, score 80, essay graded → fully graded, appears.
	sessionB := insertGradingSession(t, pool, studentB, examID, "submitted", &now, f64PtrG(80))
	insertGradingAnswer(t, pool, sessionB, essayQID, strPtrG("b answer"), f64PtrG(10), &admin, timePtrG(now))

	// Session C: submitted, score 90, essay graded → fully graded, ties with A.
	sessionC := insertGradingSession(t, pool, studentC, examID, "submitted", &now, f64PtrG(90))
	insertGradingAnswer(t, pool, sessionC, essayQID, strPtrG("c answer"), f64PtrG(10), &admin, timePtrG(now))

	// Session D: in_progress → excluded (not submitted).
	sessionD := insertGradingSession(t, pool, studentD, examID, "in_progress", nil, nil)
	insertGradingAnswer(t, pool, sessionD, essayQID, strPtrG("d draft"), nil, nil, nil)

	// Session E: submitted, score 70, essay UNGRADED → excluded by fullyGradedFilter.
	sessionE := insertGradingSession(t, pool, studentE, examID, "submitted", &now, f64PtrG(70))
	insertGradingAnswer(t, pool, sessionE, essayQID, strPtrG("e answer"), nil, nil, nil)

	// Reference: 3 eligible entries.
	_ = sessionA
	_ = sessionB
	_ = sessionC
	_ = sessionD
	_ = sessionE

	t.Run("full dataset: ties share rank, ungraded/in_progress excluded", func(t *testing.T) {
		entries, nextCursor, err := repo.ListExamLeaderboard(ctx, examID, "", 20)
		if err != nil {
			t.Fatalf("ListExamLeaderboard: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("want 3 entries (A/B/C), got %d: %+v", len(entries), entries)
		}

		// Score-descending: first two at 90, third at 80.
		if entries[0].Score != 90 || entries[1].Score != 90 {
			t.Errorf("first two entries should have score 90, got [%f, %f]",
				entries[0].Score, entries[1].Score)
		}
		if entries[2].Score != 80 {
			t.Errorf("third entry should have score 80, got %f", entries[2].Score)
		}

		// Ties share rank 1; next distinct score gets rank 3 (dense skip).
		if entries[0].Rank != 1 || entries[1].Rank != 1 {
			t.Errorf("first two should share rank 1, got ranks %d, %d",
				entries[0].Rank, entries[1].Rank)
		}
		if entries[2].Rank != 3 {
			t.Errorf("third entry should have rank 3 (dense skip), got %d", entries[2].Rank)
		}

		// All three students must be present.
		names := map[string]int{}
		for _, e := range entries {
			names[e.StudentName]++
		}
		if names["Student A"] != 1 || names["Student B"] != 1 || names["Student C"] != 1 {
			t.Errorf("expected exactly one of each student, got %v", names)
		}

		// No cursor since all fit within limit.
		if nextCursor != "" {
			t.Errorf("expected empty cursor (all rows returned), got %q", nextCursor)
		}
	})

	t.Run("cursor present when rows exceed limit, absent otherwise", func(t *testing.T) {
		// More rows than limit → cursor present.
		entries, cursor, err := repo.ListExamLeaderboard(ctx, examID, "", 1)
		if err != nil {
			t.Fatalf("ListExamLeaderboard(limit=1): %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("want 1 entry with limit=1, got %d", len(entries))
		}
		if cursor == "" {
			t.Errorf("expected non-empty cursor when more rows exist")
		} else {
			parts := strings.Split(cursor, ",")
			if len(parts) != 2 {
				t.Errorf("cursor format should be 'score,id', got %q", cursor)
			} else {
				if _, err := strconv.ParseFloat(parts[0], 64); err != nil {
					t.Errorf("cursor score not parseable: %v", err)
				}
				if _, err := uuid.Parse(parts[1]); err != nil {
					t.Errorf("cursor id not parseable: %v", err)
				}
			}
		}

		// All rows fit → cursor empty.
		entries2, cursor2, err := repo.ListExamLeaderboard(ctx, examID, "", 3)
		if err != nil {
			t.Fatalf("ListExamLeaderboard(limit=3): %v", err)
		}
		if len(entries2) != 3 {
			t.Fatalf("want 3 entries, got %d", len(entries2))
		}
		if cursor2 != "" {
			t.Errorf("expected empty cursor (all rows returned), got %q", cursor2)
		}
	})

	t.Run("page 2 resumes after a tied page 1 without loss or duplication", func(t *testing.T) {
		page1, cursor, err := repo.ListExamLeaderboard(ctx, examID, "", 2)
		if err != nil {
			t.Fatalf("page 1: %v", err)
		}
		if len(page1) != 2 || page1[0].Score != 90 || page1[1].Score != 90 {
			t.Fatalf("page 1 should be the two tied 90s, got %+v", page1)
		}
		if cursor == "" {
			t.Fatal("page 1 should return a cursor")
		}

		page2, cursor2, err := repo.ListExamLeaderboard(ctx, examID, cursor, 2)
		if err != nil {
			t.Fatalf("page 2: %v", err)
		}
		if len(page2) != 1 {
			t.Fatalf("page 2 should have exactly 1 entry (Student B, 80), got %d: %+v", len(page2), page2)
		}
		if page2[0].StudentName != "Student B" || page2[0].Score != 80 || page2[0].Rank != 3 {
			t.Errorf("page 2 entry should be Student B score=80 rank=3, got %+v", page2[0])
		}
		if cursor2 != "" {
			t.Errorf("page 2 is the last page, cursor should be empty, got %q", cursor2)
		}
	})

	t.Run("malformed cursor returns ErrInvalidCursor", func(t *testing.T) {
		for _, bad := range []string{"90,notauuid", "notanumber,00000000-0000-0000-0000-000000000001", "nocomma"} {
			_, _, err := repo.ListExamLeaderboard(ctx, examID, bad, 20)
			if !errors.Is(err, ErrInvalidCursor) {
				t.Errorf("cursor %q: want ErrInvalidCursor, got %v", bad, err)
			}
		}
	})

	t.Run("limit=1 walk visits every row exactly once", func(t *testing.T) {
		seenNames := map[string]int{}
		seenSessions := map[uuid.UUID]int{}
		var ranks []int
		cursor := ""
		for page := 0; page < 10; page++ {
			entries, next, err := repo.ListExamLeaderboard(ctx, examID, cursor, 1)
			if err != nil {
				t.Fatalf("walk page %d: %v", page, err)
			}
			for _, e := range entries {
				seenNames[e.StudentName]++
				if e.SessionID == uuid.Nil {
					t.Errorf("entry %q has zero session_id", e.StudentName)
				}
				seenSessions[e.SessionID]++
				ranks = append(ranks, e.Rank)
			}
			if next == "" {
				break
			}
			cursor = next
		}

		if len(seenNames) != 3 {
			t.Errorf("walk should visit 3 distinct students, got %v", seenNames)
		}
		for _, name := range []string{"Student A", "Student B", "Student C"} {
			if seenNames[name] != 1 {
				t.Errorf("%s should appear exactly once, got %d (all: %v)", name, seenNames[name], seenNames)
			}
		}
		for sid, n := range seenSessions {
			if n != 1 {
				t.Errorf("session %s appeared %d times", sid, n)
			}
		}
		wantRanks := []int{1, 1, 3}
		if len(ranks) != 3 || ranks[0] != wantRanks[0] || ranks[1] != wantRanks[1] || ranks[2] != wantRanks[2] {
			t.Errorf("ranks across walk should be %v, got %v", wantRanks, ranks)
		}
	})
}

// ---------------------------------------------------------------------------
// Analytics — GetExamCompletionStats
// ---------------------------------------------------------------------------

// TestAnalytics_GetExamCompletionStats verifies total/submitted counts for an exam.
func TestAnalytics_GetExamCompletionStats(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	studentA := insertGradingUser(t, pool, "student", "Student A")
	studentB := insertGradingUser(t, pool, "student", "Student B")
	studentC := insertGradingUser(t, pool, "student", "Student C")

	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	now := time.Now()

	// 2 submitted, 1 in_progress → total=3, submitted=2.
	insertGradingSession(t, pool, studentA, examID, "submitted", &now, f64PtrG(80))
	insertGradingSession(t, pool, studentB, examID, "submitted", &now, f64PtrG(90))
	insertGradingSession(t, pool, studentC, examID, "in_progress", nil, nil)

	// Separate exam with zero sessions.
	emptyExamID := insertGradingExam(t, pool, testID)

	t.Run("counts all sessions regardless of status", func(t *testing.T) {
		total, submitted, err := repo.GetExamCompletionStats(ctx, examID)
		if err != nil {
			t.Fatalf("GetExamCompletionStats: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if submitted != 2 {
			t.Errorf("submitted = %d, want 2", submitted)
		}
	})

	t.Run("zero sessions returns (0, 0, nil)", func(t *testing.T) {
		total, submitted, err := repo.GetExamCompletionStats(ctx, emptyExamID)
		if err != nil {
			t.Fatalf("GetExamCompletionStats on empty exam: %v", err)
		}
		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}
		if submitted != 0 {
			t.Errorf("submitted = %d, want 0", submitted)
		}
	})
}

// ---------------------------------------------------------------------------
// Analytics — GetFullyGradedScores
// ---------------------------------------------------------------------------

// TestAnalytics_GetFullyGradedScores verifies only fully-graded submitted sessions
// are included and that an empty result is an empty slice (not nil, not error).
func TestAnalytics_GetFullyGradedScores(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	admin := insertGradingUser(t, pool, "admin_exam", "Grader Admin")
	studentA := insertGradingUser(t, pool, "student", "Student A")
	studentB := insertGradingUser(t, pool, "student", "Student B")
	studentC := insertGradingUser(t, pool, "student", "Student C")

	testID := insertGradingTest(t, pool)
	essayQID := insertGradingEssayQuestion(t, pool, testID, "Explain", 10, 1)
	examID := insertGradingExam(t, pool, testID)
	now := time.Now()

	// Session A: submitted, score 85, essay graded → fully graded.
	sessionA := insertGradingSession(t, pool, studentA, examID, "submitted", &now, f64PtrG(85))
	insertGradingAnswer(t, pool, sessionA, essayQID, strPtrG("a answer"), f64PtrG(8), &admin, timePtrG(now))

	// Session B: submitted, score 92, essay graded → fully graded.
	sessionB := insertGradingSession(t, pool, studentB, examID, "submitted", &now, f64PtrG(92))
	insertGradingAnswer(t, pool, sessionB, essayQID, strPtrG("b answer"), f64PtrG(9), &admin, timePtrG(now))

	// Session C: submitted, score 70, essay UNGRADED → excluded.
	sessionC := insertGradingSession(t, pool, studentC, examID, "submitted", &now, f64PtrG(70))
	insertGradingAnswer(t, pool, sessionC, essayQID, strPtrG("c answer"), nil, nil, nil)

	// Exam with no sessions at all.
	emptyExamID := insertGradingExam(t, pool, testID)

	_ = sessionA
	_ = sessionB
	_ = sessionC

	t.Run("returns only fully-graded submitted session scores", func(t *testing.T) {
		scores, err := repo.GetFullyGradedScores(ctx, examID)
		if err != nil {
			t.Fatalf("GetFullyGradedScores: %v", err)
		}
		if len(scores) != 2 {
			t.Fatalf("want 2 scores (A=85, B=92), got %d: %v", len(scores), scores)
		}
		seen := map[float64]bool{}
		for _, s := range scores {
			seen[s] = true
		}
		if !seen[85] || !seen[92] {
			t.Errorf("want scores {85, 92}, got %v", scores)
		}
	})

	t.Run("empty slice when none qualify (not nil, not error)", func(t *testing.T) {
		scores, err := repo.GetFullyGradedScores(ctx, emptyExamID)
		if err != nil {
			t.Fatalf("GetFullyGradedScores: %v", err)
		}
		if scores == nil {
			t.Error("want empty slice, got nil")
		}
		if len(scores) != 0 {
			t.Errorf("want empty slice, got %d elements", len(scores))
		}
	})
}

// ---------------------------------------------------------------------------
// Certificate — UpdateSessionCertificate
// ---------------------------------------------------------------------------

// TestCertificate_UpdateSessionCertificate verifies that both certificate_key and
// certificate_generated_at are persisted and that a second call overwrites both.
func TestCertificate_UpdateSessionCertificate(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := insertGradingUser(t, pool, "student", "Student Cert")
	testID := insertGradingTest(t, pool)
	examID := insertGradingExam(t, pool, testID)
	now := time.Now()
	sessionID := insertGradingSession(t, pool, student, examID, "submitted", &now, f64PtrG(85))

	t.Run("persists certificate_key and certificate_generated_at", func(t *testing.T) {
		key := "certificates/abc123.pdf"
		genAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
		if err := repo.UpdateSessionCertificate(ctx, sessionID, key, genAt); err != nil {
			t.Fatalf("UpdateSessionCertificate: %v", err)
		}

		sess, err := repo.GetExamSessionByID(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetExamSessionByID: %v", err)
		}
		if sess.CertificateKey == nil || *sess.CertificateKey != key {
			t.Errorf("CertificateKey = %v, want %q", sess.CertificateKey, key)
		}
		if sess.CertificateGeneratedAt == nil || !sess.CertificateGeneratedAt.Equal(genAt) {
			t.Errorf("CertificateGeneratedAt = %v, want %v", sess.CertificateGeneratedAt, genAt)
		}
	})

	t.Run("second call overwrites both fields", func(t *testing.T) {
		key2 := "certificates/def456.pdf"
		genAt2 := time.Date(2026, 7, 2, 14, 30, 0, 0, time.UTC)
		if err := repo.UpdateSessionCertificate(ctx, sessionID, key2, genAt2); err != nil {
			t.Fatalf("UpdateSessionCertificate (2nd): %v", err)
		}

		sess, err := repo.GetExamSessionByID(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetExamSessionByID: %v", err)
		}
		if sess.CertificateKey == nil || *sess.CertificateKey != key2 {
			t.Errorf("CertificateKey = %v, want %q", sess.CertificateKey, key2)
		}
		if sess.CertificateGeneratedAt == nil || !sess.CertificateGeneratedAt.Equal(genAt2) {
			t.Errorf("CertificateGeneratedAt = %v, want %v", sess.CertificateGeneratedAt, genAt2)
		}
	})
}

// ---------------------------------------------------------------------------
// Certificate — Exam CRUD round-trip for certificate_design's template key
// ---------------------------------------------------------------------------

// TestCertificateTemplateRoundTrip verifies that creating and updating an exam with
// each of the 3 valid template keys persists (inside certificate_design, FR-26) and
// reads back correctly.
func TestCertificateTemplateRoundTrip(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	testID := insertGradingTest(t, pool)

	for _, template := range []string{"classic", "modern", "elegant"} {
		t.Run("create with "+template, func(t *testing.T) {
			var examID uuid.UUID
			err := pool.QueryRow(ctx,
				`INSERT INTO exam (title, certificate_design) VALUES ($1, $2) RETURNING id`,
				"Create-"+template, fmt.Sprintf(`{"template":%q}`, template),
			).Scan(&examID)
			if err != nil {
				t.Fatalf("insert exam with template %q: %v", template, err)
			}
			if _, err := pool.Exec(ctx,
				`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
				examID, testID,
			); err != nil {
				t.Fatalf("insert exam_test: %v", err)
			}

			exam, err := repo.GetExamByID(ctx, examID)
			if err != nil {
				t.Fatalf("GetExamByID: %v", err)
			}
			if exam.CertificateDesign == nil {
				t.Fatal("CertificateDesign is nil, want the seeded blob")
			}
			var decoded map[string]any
			if err := json.Unmarshal(*exam.CertificateDesign, &decoded); err != nil {
				t.Fatalf("unmarshal certificate_design: %v", err)
			}
			if decoded["template"] != template {
				t.Errorf("certificate_design template = %v, want %q", decoded["template"], template)
			}
		})

		t.Run("update to "+template, func(t *testing.T) {
			var examID uuid.UUID
			err := pool.QueryRow(ctx,
				`INSERT INTO exam (title, certificate_design) VALUES ($1, '{"template":"classic"}') RETURNING id`,
				"UpdateTo-"+template,
			).Scan(&examID)
			if err != nil {
				t.Fatalf("insert exam: %v", err)
			}
			if _, err := pool.Exec(ctx,
				`INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, 1)`,
				examID, testID,
			); err != nil {
				t.Fatalf("insert exam_test: %v", err)
			}

			// Fetch, modify, write back.
			exam, err := repo.GetExamByID(ctx, examID)
			if err != nil {
				t.Fatalf("GetExamByID: %v", err)
			}
			raw := json.RawMessage(fmt.Sprintf(`{"template":%q}`, template))
			exam.CertificateDesign = &raw
			if err := repo.UpdateExam(ctx, examID, exam); err != nil {
				t.Fatalf("UpdateExam: %v", err)
			}

			updated, err := repo.GetExamByID(ctx, examID)
			if err != nil {
				t.Fatalf("GetExamByID after update: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(*updated.CertificateDesign, &decoded); err != nil {
				t.Fatalf("unmarshal certificate_design: %v", err)
			}
			if decoded["template"] != template {
				t.Errorf("certificate_design template = %v, want %q", decoded["template"], template)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Certificate — template validity (DB CHECK constraint removed, Task 8/FR-26)
// ---------------------------------------------------------------------------

// TestCertificateTemplateCHECKConstraint_Removed proves the chk_certificate_template
// DB constraint no longer exists after migration 0042 (certificate_design is a plain
// JSONB blob with no CHECK on its "template" key) — template validity is now enforced
// exclusively at the Go layer by service.validateCertificateTemplate (see
// certificate_test.go's TestValidateCertificateTemplate_* for that coverage).
func TestCertificateTemplateCHECKConstraint_Removed(t *testing.T) {
	pool := newGradingTestPool(t)
	ctx := context.Background()

	// An "invalid" template string is accepted at the DB level — nothing but Go
	// validation stands between the editor and this column now.
	var examID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO exam (title, certificate_design) VALUES ($1, $2) RETURNING id`,
		"Any Template Exam", `{"template":"invalid_template"}`,
	).Scan(&examID)
	if err != nil {
		t.Fatalf("insert with an arbitrary template string should succeed at the DB level, got: %v", err)
	}

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = 'chk_certificate_template')`,
	).Scan(&exists); err != nil {
		t.Fatalf("query pg_constraint: %v", err)
	}
	if exists {
		t.Error("chk_certificate_template should have been dropped by migration 0042")
	}
}

// ---------------------------------------------------------------------------
// Compile-time interface checks
// ---------------------------------------------------------------------------

var _ interface {
	ListExamLeaderboard(context.Context, uuid.UUID, string, int) ([]model.ExamLeaderboardEntry, string, error)
	GetExamCompletionStats(context.Context, uuid.UUID) (int, int, error)
	GetFullyGradedScores(context.Context, uuid.UUID) ([]float64, error)
	UpdateSessionCertificate(context.Context, uuid.UUID, string, time.Time) error
} = (*Repository)(nil)
