package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func seedStudentPageRow(t *testing.T, r *Repository, schoolID, name, status, jenjang string, grade int, createdAt time.Time) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, school_id, status, jenjang, grade, otp_enabled, created_at)
		 VALUES ($1, $2, 'student', $3, $4, $5, $6, false, $7) RETURNING id`,
		name, "page_"+uuid.NewString()[:12], schoolID, status, jenjang, grade, createdAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert student page row: %v", err)
	}
	return id
}

func seedStudentEligibilityExam(t *testing.T, r *Repository, studentID string) string {
	t.Helper()
	ctx := context.Background()
	var examID string
	if err := r.pool.QueryRow(ctx, `INSERT INTO exam (title) VALUES ($1) RETURNING id`, "Eligibility "+uuid.NewString()).Scan(&examID); err != nil {
		t.Fatalf("insert exam: %v", err)
	}
	if studentID != "" {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
			studentID, examID, "elig_"+uuid.NewString(),
		); err != nil {
			t.Fatalf("insert registration: %v", err)
		}
	}
	return examID
}

func walkSchoolStudentPages(t *testing.T, r *Repository, schoolID string, filter StudentFilter) []string {
	t.Helper()
	var ids []string
	for {
		rows, next, err := r.ListStudentsBySchool(context.Background(), schoolID, filter)
		if err != nil {
			t.Fatalf("ListStudentsBySchool: %v", err)
		}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		if next == "" {
			return ids
		}
		filter.Cursor = next
	}
}

func walkCrossSchoolStudentPages(t *testing.T, r *Repository, filter StudentFilter) []string {
	t.Helper()
	var ids []string
	for {
		rows, next, err := r.SearchStudentsAcrossSchools(context.Background(), filter)
		if err != nil {
			t.Fatalf("SearchStudentsAcrossSchools: %v", err)
		}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		if next == "" {
			return ids
		}
		filter.Cursor = next
	}
}

func assertStudentIDsExactlyOnce(t *testing.T, got, want []string) {
	t.Helper()
	counts := make(map[string]int, len(got))
	for _, id := range got {
		counts[id]++
	}
	if len(got) != len(want) {
		t.Fatalf("student count: want %d, got %d (%v)", len(want), len(got), got)
	}
	for _, id := range want {
		if counts[id] != 1 {
			t.Errorf("student %s: want exactly once, got %d", id, counts[id])
		}
	}
}

func TestStudentLists_ExamEligibilityAndCompositeCursor(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	schoolID := insertSchoolForAdminStudentsTest(t, r, "Eligibility "+suffix, "elig_"+suffix)
	tiedAt := time.Now().UTC().Truncate(time.Microsecond)

	eligibleA := seedStudentPageRow(t, r, schoolID, "Eligible A "+suffix, "active", "sma", 10, tiedAt)
	eligibleB := seedStudentPageRow(t, r, schoolID, "Eligible B "+suffix, "active", "sma", 10, tiedAt)
	registered := seedStudentPageRow(t, r, schoolID, "Registered "+suffix, "active", "sma", 10, tiedAt)
	seedStudentPageRow(t, r, schoolID, "Deactivated "+suffix, "deactivated", "sma", 10, tiedAt)
	seedStudentPageRow(t, r, schoolID, "Wrong Grade "+suffix, "active", "sma", 11, tiedAt)
	seedStudentPageRow(t, r, schoolID, "Wrong Jenjang "+suffix, "active", "smp", 10, tiedAt)
	examID := seedStudentEligibilityExam(t, r, registered)
	grade := 10
	filter := StudentFilter{Limit: 1, Q: suffix, Grade: &grade, Jenjang: "sma", ExamID: examID}

	t.Run("school endpoint walks tied timestamps without ineligible rows", func(t *testing.T) {
		got := walkSchoolStudentPages(t, r, schoolID, filter)
		assertStudentIDsExactlyOnce(t, got, []string{eligibleA, eligibleB})
	})

	t.Run("cross-school endpoint walks tied timestamps without ineligible rows", func(t *testing.T) {
		filter.SchoolID = &schoolID
		got := walkCrossSchoolStudentPages(t, r, filter)
		assertStudentIDsExactlyOnce(t, got, []string{eligibleA, eligibleB})
	})

	t.Run("malformed composite cursors are rejected", func(t *testing.T) {
		filter.Cursor = "not-a-composite-cursor"
		if _, _, err := r.ListStudentsBySchool(ctx, schoolID, filter); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("ListStudentsBySchool: want ErrInvalidCursor, got %v", err)
		}
		if _, _, err := r.SearchStudentsAcrossSchools(ctx, filter); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("SearchStudentsAcrossSchools: want ErrInvalidCursor, got %v", err)
		}
	})
}

// insertNullSchoolStudent inserts a student with school_id IS NULL and a
// free-text unlisted_school_name, mirroring how a self-registering user
// without a listed school ends up in the users table.
func insertNullSchoolStudent(t *testing.T, r *Repository, name, unlistedName string) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, school_id, unlisted_school_name, status, jenjang, grade, otp_enabled)
		 VALUES ($1, $2, 'student', NULL, $3, 'active', 'sma', 10, false)
		 RETURNING id`,
		name, "ns_"+uuid.NewString()[:12], unlistedName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert null-school student: %v", err)
	}
	return id
}

func insertSchoolForAdminStudentsTest(t *testing.T, r *Repository, name, code string) string {
	t.Helper()
	var id string
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO school (name, code) VALUES ($1, $2) RETURNING id`,
		name, code,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert school: %v", err)
	}
	return id
}

func TestCountStudentsAdmin_matchesFilteredTotals(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	suffix := uuid.NewString()[:8]
	schoolID := insertSchoolForAdminStudentsTest(t, r, "Count School "+suffix, "csa_"+suffix)
	seedStudentPageRow(t, r, schoolID, "Counted Active 1 "+suffix, "active", "sma", 10, time.Now().UTC())
	seedStudentPageRow(t, r, schoolID, "Counted Active 2 "+suffix, "active", "sma", 11, time.Now().UTC())
	seedStudentPageRow(t, r, schoolID, "Counted Inactive "+suffix, "deactivated", "sma", 12, time.Now().UTC())
	// A different school's student with the same name pattern must not leak
	// into the counts of a school-scoped query.
	otherSchool := insertSchoolForAdminStudentsTest(t, r, "Other Count School "+suffix, "csb_"+suffix)
	seedStudentPageRow(t, r, otherSchool, "Counted Active 3 "+suffix, "active", "sma", 10, time.Now().UTC())

	filter := StudentFilter{Q: suffix, Limit: 1, Cursor: "would-be-ignored-if-it-were-valid"}
	counts, err := r.CountStudentsAdmin(ctx, schoolID, filter)
	if err != nil {
		t.Fatalf("CountStudentsAdmin: %v", err)
	}
	if counts.Total != 3 {
		t.Errorf("Total: want 3, got %d", counts.Total)
	}
	if counts.Active != 2 {
		t.Errorf("Active: want 2, got %d", counts.Active)
	}
	if counts.Deactivated != 1 {
		t.Errorf("Deactivated: want 1, got %d", counts.Deactivated)
	}

	// Counts are filter-aware: narrowing to active makes total == active and
	// zeroes the deactivated bucket (same behavior as CountSchoolsAdmin).
	counts, err = r.CountStudentsAdmin(ctx, schoolID, StudentFilter{Q: suffix, Status: "active"})
	if err != nil {
		t.Fatalf("CountStudentsAdmin(active): %v", err)
	}
	if counts.Total != 2 || counts.Active != 2 || counts.Deactivated != 0 {
		t.Errorf("active-filtered counts: want 2/2/0, got %d/%d/%d", counts.Total, counts.Active, counts.Deactivated)
	}
}

func TestSearchStudentsAcrossSchools_NullSchool(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	suffix := uuid.NewString()[:8]
	studentName := "NullSchoolKid " + suffix
	unlistedName := "Unlisted School " + suffix
	studentID := insertNullSchoolStudent(t, r, studentName, unlistedName)

	t.Run("no school filter includes the null-school student with nil SchoolID/SchoolName", func(t *testing.T) {
		rows, _, err := r.SearchStudentsAcrossSchools(ctx, StudentFilter{Limit: 100, Q: suffix})
		if err != nil {
			t.Fatalf("SearchStudentsAcrossSchools: %v", err)
		}
		found := false
		for _, row := range rows {
			if row.ID != studentID {
				continue
			}
			found = true
			if row.SchoolID != nil {
				t.Errorf("SchoolID: want nil, got %v", *row.SchoolID)
			}
			if row.SchoolName != nil {
				t.Errorf("SchoolName: want nil, got %v", *row.SchoolName)
			}
			if row.UnlistedSchoolName == nil || *row.UnlistedSchoolName != unlistedName {
				t.Errorf("UnlistedSchoolName: want %s, got %v", unlistedName, row.UnlistedSchoolName)
			}
		}
		if !found {
			t.Fatalf("student %s not found in results scoped by q=%s", studentID, suffix)
		}
	})

	t.Run("NoSchool true returns only null-school students", func(t *testing.T) {
		schooledID := insertSchoolForAdminStudentsTest(t, r, "School For "+suffix, "sfa_"+suffix)
		_, err := r.pool.Exec(ctx,
			`INSERT INTO users (name, username, role, school_id, status, jenjang, grade, otp_enabled)
			 VALUES ($1, $2, 'student', $3, 'active', 'sma', 10, false)`,
			"SchooledKid "+suffix, "s_"+suffix, schooledID,
		)
		if err != nil {
			t.Fatalf("insert schooled student: %v", err)
		}

		rows, _, err := r.SearchStudentsAcrossSchools(ctx, StudentFilter{Limit: 100, Q: suffix, NoSchool: true})
		if err != nil {
			t.Fatalf("SearchStudentsAcrossSchools: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 null-school student, got %d", len(rows))
		}
		if rows[0].ID != studentID {
			t.Errorf("ID: want %s, got %s", studentID, rows[0].ID)
		}
		if rows[0].SchoolID != nil {
			t.Errorf("SchoolID: want nil, got %v", *rows[0].SchoolID)
		}
	})
}

func TestGetStudentByID_EmptySchoolID(t *testing.T) {
	pool := newGradingTestPool(t)
	r := New(pool)
	ctx := context.Background()

	suffix := uuid.NewString()[:8]
	schoolID := insertSchoolForAdminStudentsTest(t, r, "School "+suffix, "gs_"+suffix)
	var withSchool string
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, school_id, status, jenjang, otp_enabled)
		 VALUES ($1, $2, 'student', $3, 'active', 'sma', false) RETURNING id`,
		"Has School "+suffix, "gsu_"+suffix, schoolID,
	).Scan(&withSchool); err != nil {
		t.Fatalf("insert schooled student: %v", err)
	}
	nullID := insertNullSchoolStudent(t, r, "Null "+suffix, "Unlisted "+suffix)

	t.Run("empty schoolID finds a schooled student", func(t *testing.T) {
		u, err := r.GetStudentByID(ctx, withSchool, "")
		if err != nil {
			t.Fatalf("GetStudentByID: %v", err)
		}
		if u == nil || u.ID != withSchool {
			t.Fatalf("want student %s, got %+v", withSchool, u)
		}
	})

	t.Run("empty schoolID finds a null-school student", func(t *testing.T) {
		u, err := r.GetStudentByID(ctx, nullID, "")
		if err != nil {
			t.Fatalf("GetStudentByID: %v", err)
		}
		if u == nil || u.ID != nullID {
			t.Fatalf("want student %s, got %+v", nullID, u)
		}
	})

	t.Run("wrong school returns nil", func(t *testing.T) {
		other := insertSchoolForAdminStudentsTest(t, r, "Other "+suffix, "go_"+suffix)
		u, err := r.GetStudentByID(ctx, withSchool, other)
		if err != nil {
			t.Fatalf("GetStudentByID: %v", err)
		}
		if u != nil {
			t.Errorf("want nil for other school, got %+v", u)
		}
	})

	t.Run("owning school finds the student", func(t *testing.T) {
		u, err := r.GetStudentByID(ctx, withSchool, schoolID)
		if err != nil {
			t.Fatalf("GetStudentByID: %v", err)
		}
		if u == nil {
			t.Fatal("want student under owning school")
		}
	})
}
