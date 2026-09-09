package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestResolveSchoolParticipantSet_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolAID := createTestSchool(t, svc)
	schoolBID := createTestSchool(t, svc)

	// Insert students directly via raw SQL (RegisterStudent references nis
	// which was dropped by migration 0030).
	grade10 := 10
	grade11 := 11

	// School A: 3 students at grade 10
	studentIDs := make([]string, 3)
	for i := range studentIDs {
		username := "pstu_" + uniqueSuffix()
		var id string
		err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, grade, username, password_hash)
			 VALUES ($1, $2, 'student', 'active', $3, $4, '')
			 RETURNING id`,
			"Participant Student "+uniqueSuffix(), schoolAID, grade10, username,
		).Scan(&id)
		if err != nil {
			t.Fatalf("insert student %d: %v", i, err)
		}
		studentIDs[i] = id
	}

	// School B: 1 student at grade 11 (cross-school id for testing)
	var crossSchoolID string
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, school_id, role, status, grade, username, password_hash)
		 VALUES ($1, $2, 'student', 'active', $3, $4, '')
		 RETURNING id`,
		"Cross School Student", schoolBID, grade11, "pcross_"+uniqueSuffix(),
	).Scan(&crossSchoolID)
	if err != nil {
		t.Fatalf("insert cross-school student: %v", err)
	}

	t.Run("individual ids all in-school resolve to themselves", func(t *testing.T) {
		result, err := svc.ResolveSchoolParticipantSet(ctx, schoolAID, ParticipantSelector{
			StudentIDs: studentIDs,
		})
		if err != nil {
			t.Fatalf("ResolveSchoolParticipantSet: %v", err)
		}
		if len(result) != len(studentIDs) {
			t.Fatalf("want %d results, got %d", len(studentIDs), len(result))
		}
		for i, id := range studentIDs {
			if result[i].String() != id {
				t.Errorf("result[%d]: want %s, got %s", i, id, result[i].String())
			}
		}
	})

	t.Run("cross-school student id causes the whole call to fail", func(t *testing.T) {
		_, err := svc.ResolveSchoolParticipantSet(ctx, schoolAID, ParticipantSelector{
			StudentIDs: []string{studentIDs[0], crossSchoolID},
		})
		if err == nil {
			t.Fatal("expected error for cross-school student, got nil")
		}
		if !errors.Is(err, ErrCrossSchoolStudent) {
			t.Errorf("want ErrCrossSchoolStudent, got %v", err)
		}
	})

	t.Run("duplicate student ids in selector causes error", func(t *testing.T) {
		_, err := svc.ResolveSchoolParticipantSet(ctx, schoolAID, ParticipantSelector{
			StudentIDs: []string{studentIDs[0], studentIDs[1], studentIDs[0]},
		})
		if err == nil {
			t.Fatal("expected error for duplicate student id, got nil")
		}
		if !errors.Is(err, ErrDuplicateParticipant) {
			t.Errorf("want ErrDuplicateParticipant, got %v", err)
		}
	})

	t.Run("Grade selects exactly that grade's students in-school", func(t *testing.T) {
		g := 10
		result, err := svc.ResolveSchoolParticipantSet(ctx, schoolAID, ParticipantSelector{
			Grade: &g,
		})
		if err != nil {
			t.Fatalf("ResolveSchoolParticipantSet: %v", err)
		}
		if len(result) != len(studentIDs) {
			t.Errorf("want %d grade-10 students, got %d", len(studentIDs), len(result))
		}
	})

	t.Run("All selects every in-school student and matches collectAllStudentIDs cap behavior", func(t *testing.T) {
		// Add a grade-11 student to school A so All returns more than just grade-10 students.
		var extraID string
		err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, grade, username, password_hash)
			 VALUES ($1, $2, 'student', 'active', $3, $4, '')
			 RETURNING id`,
			"Extra Grade 11", schoolAID, grade11, "pextra_"+uniqueSuffix(),
		).Scan(&extraID)
		if err != nil {
			t.Fatalf("insert extra student: %v", err)
		}

		result, err := svc.ResolveSchoolParticipantSet(ctx, schoolAID, ParticipantSelector{
			All: true,
		})
		if err != nil {
			t.Fatalf("ResolveSchoolParticipantSet: %v", err)
		}
		// Should include all students in school A (3 grade-10 + 1 grade-11 = 4).
		if len(result) != 4 {
			t.Errorf("want 4 students (All), got %d", len(result))
		}
	})

	t.Run("empty selector returns ErrEmptySelector", func(t *testing.T) {
		_, err := svc.ResolveSchoolParticipantSet(ctx, schoolAID, ParticipantSelector{})
		if !errors.Is(err, ErrEmptySelector) {
			t.Errorf("want ErrEmptySelector, got %v", err)
		}
	})
}

func TestListStudentsWithGradeAndJenjang_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)

	grade10 := 10
	jenjangSMA := "sma"
	jenjangSMP := "smp"

	// Insert students with different grades and jenjang values.
	insertStudent := func(name, username string, grade int, jenjang string) string {
		var id string
		err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, grade, jenjang, username, password_hash)
			 VALUES ($1, $2, 'student', 'active', $3, $4, $5, '')
			 RETURNING id`,
			name, schoolID, grade, jenjang, username,
		).Scan(&id)
		if err != nil {
			t.Fatalf("insert student %s: %v", name, err)
		}
		return id
	}

	_ = insertStudent("SMA Grade 10 A", "sma10a_"+uniqueSuffix(), grade10, jenjangSMA)
	_ = insertStudent("SMA Grade 10 B", "sma10b_"+uniqueSuffix(), grade10, jenjangSMA)
	_ = insertStudent("SMP Grade 10 A", "smp10a_"+uniqueSuffix(), grade10, jenjangSMP)
	_ = insertStudent("SMP Grade 8 A", "smp8a_"+uniqueSuffix(), 8, jenjangSMP)

	t.Run("grade filter returns only matching grade", func(t *testing.T) {
		g := 10
		rows, _, _, err := svc.ListStudents(ctx, schoolID, "", "", 100, "", &g, "")
		if err != nil {
			t.Fatalf("ListStudents: %v", err)
		}
		if len(rows) != 3 {
			t.Errorf("want 3 grade-10 students, got %d", len(rows))
		}
	})

	t.Run("jenjang filter returns only matching jenjang", func(t *testing.T) {
		rows, _, _, err := svc.ListStudents(ctx, schoolID, "", "", 100, "", nil, jenjangSMA)
		if err != nil {
			t.Fatalf("ListStudents: %v", err)
		}
		if len(rows) != 2 {
			t.Errorf("want 2 SMA students, got %d", len(rows))
		}
	})

	t.Run("grade + jenjang combined returns intersection", func(t *testing.T) {
		g := 10
		rows, _, _, err := svc.ListStudents(ctx, schoolID, "", "", 100, "", &g, jenjangSMP)
		if err != nil {
			t.Fatalf("ListStudents: %v", err)
		}
		if len(rows) != 1 {
			t.Errorf("want 1 SMP grade-10 student, got %d", len(rows))
		}
	})

	t.Run("grade + jenjang + q combined returns intersection", func(t *testing.T) {
		g := 10
		rows, _, _, err := svc.ListStudents(ctx, schoolID, "", "SMA Grade 10 A", 100, "", &g, jenjangSMA)
		if err != nil {
			t.Fatalf("ListStudents: %v", err)
		}
		if len(rows) != 1 {
			t.Errorf("want 1 SMA grade-10 student matching search, got %d", len(rows))
		}
	})

	t.Run("no filters returns all students", func(t *testing.T) {
		rows, _, _, err := svc.ListStudents(ctx, schoolID, "", "", 100, "", nil, "")
		if err != nil {
			t.Fatalf("ListStudents: %v", err)
		}
		if len(rows) != 4 {
			t.Errorf("want 4 students (no filters), got %d", len(rows))
		}
	})
}

func TestResolveSchoolParticipantSet_AllRowCap_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)

	// Insert enough students to exceed the cap. Use a lower cap for testing.
	originalCap := bulkAllRowCap
	bulkAllRowCap = 5
	t.Cleanup(func() { bulkAllRowCap = originalCap })

	for i := 0; i < 10; i++ {
		_, err := repo.Pool().Exec(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash)
			 VALUES ($1, $2, 'student', 'active', $3, '')`,
			"Cap Test "+uniqueSuffix(), schoolID, "pcap_"+uniqueSuffix(),
		)
		if err != nil {
			t.Fatalf("insert student %d: %v", i, err)
		}
	}

	_, err := svc.ResolveSchoolParticipantSet(ctx, schoolID, ParticipantSelector{All: true})
	if err == nil {
		t.Fatal("expected ErrRowLimitExceeded, got nil")
	}
	if !errors.Is(err, ErrRowLimitExceeded) {
		t.Errorf("want ErrRowLimitExceeded, got %v", err)
	}
}

func TestResolveSchoolParticipantSet_GradeEmpty_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	otherSchoolID := createTestSchool(t, svc)

	// Insert one student at grade 10 in school, none at grade 11.
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO users (name, school_id, role, status, grade, username, password_hash)
		 VALUES ($1, $2, 'student', 'active', $3, $4, '')`,
		"Grade 10 Only", schoolID, 10, "pempty_"+uniqueSuffix(),
	)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}

	t.Run("grade with no matching students returns empty", func(t *testing.T) {
		g := 11
		result, err := svc.ResolveSchoolParticipantSet(ctx, schoolID, ParticipantSelector{Grade: &g})
		if err != nil {
			t.Fatalf("ResolveSchoolParticipantSet: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("want 0 grade-11 students, got %d", len(result))
		}
	})

	t.Run("other school has no grade-10 students", func(t *testing.T) {
		g := 10
		result, err := svc.ResolveSchoolParticipantSet(ctx, otherSchoolID, ParticipantSelector{Grade: &g})
		if err != nil {
			t.Fatalf("ResolveSchoolParticipantSet: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("want 0 grade-10 students in other school, got %d", len(result))
		}
	})
}

func TestResolveSchoolParticipantSet_InvalidUUID_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	schoolID := "00000000-0000-0000-0000-000000000000" // non-existent UUID

	t.Run("non-existent school with student ids fails with cross-school error", func(t *testing.T) {
		_, err := svc.ResolveSchoolParticipantSet(ctx, schoolID, ParticipantSelector{
			// uuid.MustParse won't panic because we provide valid UUID strings.
			// GetStudentByID will return nil since no such student exists.
			StudentIDs: []string{uuid.New().String()},
		})
		if err == nil {
			t.Fatal("expected error for non-existent school/student, got nil")
		}
		if !errors.Is(err, ErrCrossSchoolStudent) {
			t.Errorf("want ErrCrossSchoolStudent, got %v", err)
		}
	})
}
