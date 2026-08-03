package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

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
