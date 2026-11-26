package repository

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// insertSchoolWithLocation creates a school with a provinsi_id/kota_id set —
// insertSchool leaves both NULL. provinsiID/kotaID must be real seeded region
// ids (db/migrations/0029_seed_province_city_district.up.sql) since both
// columns are FK-constrained.
func insertSchoolWithLocation(t *testing.T, pool *pgxpool.Pool, name, code, provinsiID, kotaID string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO school (name, code, provinsi_id, kota_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		name, code, provinsiID, kotaID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// TestExamRegistrantDiversity_CountsAndRosterFilterStayConsistent seeds a
// roster across 2 provinces/cities and 3 schools, then proves (1) the
// distinct province/city/school counts on CountExamRegistrationsByStatus are
// correct, and (2) ListExamRoster's same ProvinceID/CityID/SchoolID filter
// narrows the roster to exactly the rows the scoped counts describe.
func TestExamRegistrantDiversity_CountsAndRosterFilterStayConsistent(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolA := insertSchoolWithLocation(t, pool, "Diversity School A", "div-a", "11", "1101")
	schoolB := insertSchoolWithLocation(t, pool, "Diversity School B", "div-b", "12", "1201")
	schoolC := insertSchoolWithLocation(t, pool, "Diversity School C", "div-c", "11", "1101")

	var examID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity Exam', 'draft') RETURNING id`,
	).Scan(&examID))

	studentA := insertSchoolUser(t, pool, "student", "Student A", schoolA)
	studentB := insertSchoolUser(t, pool, "student", "Student B", schoolB)
	studentC := insertSchoolUser(t, pool, "student", "Student C", schoolC)

	for _, sid := range []uuid.UUID{studentA, studentB, studentC} {
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
			sid, examID, uuid.NewString(),
		)
		require.NoError(t, err)
	}

	counts, err := repo.CountExamRegistrationsByStatus(ctx, examID, nil, model.ExamRosterFilter{})
	require.NoError(t, err)
	require.Equal(t, 3, counts.Total)
	require.Equal(t, 2, counts.ProvinceCount, "provinces 11 and 12")
	require.Equal(t, 2, counts.CityCount, "cities 1101 and 1201")
	require.Equal(t, 3, counts.SchoolCount)

	provinceFilter := "11"
	scoped, err := repo.CountExamRegistrationsByStatus(ctx, examID, nil, model.ExamRosterFilter{ProvinceID: &provinceFilter})
	require.NoError(t, err)
	require.Equal(t, 2, scoped.Total, "schools A and C are both in province 11")
	require.Equal(t, 1, scoped.ProvinceCount)
	require.Equal(t, 1, scoped.CityCount)
	require.Equal(t, 2, scoped.SchoolCount)

	rosterRows, _, err := repo.ListExamRoster(ctx, examID, nil, model.ExamRosterFilter{Limit: 10, ProvinceID: &provinceFilter})
	require.NoError(t, err)
	require.Len(t, rosterRows, scoped.Total, "roster row count must match the same-filter counts")
	for _, row := range rosterRows {
		require.NotEqual(t, studentB, row.StudentID, "province-11 filter must exclude the province-12 student")
	}

	schoolCFilter := schoolC
	schoolScoped, err := repo.CountExamRegistrationsByStatus(ctx, examID, nil, model.ExamRosterFilter{SchoolID: &schoolCFilter})
	require.NoError(t, err)
	require.Equal(t, 1, schoolScoped.Total)
	require.Equal(t, 1, schoolScoped.ProvinceCount)
	require.Equal(t, 1, schoolScoped.CityCount)
	require.Equal(t, 1, schoolScoped.SchoolCount)
}

// TestListExamRegistrantDiversity_PerExamCountsScopedToFilter proves the
// Opsi A on-demand list function computes registration_count/province_count/
// city_count/school_count per exam, scoped to the same filter, across more
// than one exam.
func TestListExamRegistrantDiversity_PerExamCountsScopedToFilter(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolA := insertSchoolWithLocation(t, pool, "Diversity List School A", "divlist-a", "11", "1101")
	schoolB := insertSchoolWithLocation(t, pool, "Diversity List School B", "divlist-b", "12", "1201")

	var exam1, exam2 uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity List Exam 1', 'draft') RETURNING id`,
	).Scan(&exam1))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity List Exam 2', 'draft') RETURNING id`,
	).Scan(&exam2))

	studentA1 := insertSchoolUser(t, pool, "student", "Exam1 Student A", schoolA)
	studentB1 := insertSchoolUser(t, pool, "student", "Exam1 Student B", schoolB)
	studentA2 := insertSchoolUser(t, pool, "student", "Exam2 Student A", schoolA)

	insertReg := func(studentID, examID uuid.UUID) {
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
			studentID, examID, uuid.NewString(),
		)
		require.NoError(t, err)
	}
	insertReg(studentA1, exam1)
	insertReg(studentB1, exam1)
	insertReg(studentA2, exam2)

	provinceFilter := "11"
	items, nextCursor, err := repo.ListExamRegistrantDiversity(ctx, ExamRegistrantDiversityFilter{
		Limit:      10,
		ProvinceID: &provinceFilter,
	})
	require.NoError(t, err)
	require.Empty(t, nextCursor)
	require.Len(t, items, 2)

	byExam := map[uuid.UUID]model.ExamRegistrantDiversityItem{}
	for _, item := range items {
		byExam[item.ExamID] = item
	}

	exam1Item, ok := byExam[exam1]
	require.True(t, ok)
	require.Equal(t, 1, exam1Item.RegistrationCount, "exam1 has only one province-11 registrant (studentA1)")
	require.Equal(t, 1, exam1Item.ProvinceCount)
	require.Equal(t, 1, exam1Item.CityCount)
	require.Equal(t, 1, exam1Item.SchoolCount)

	exam2Item, ok := byExam[exam2]
	require.True(t, ok)
	require.Equal(t, 1, exam2Item.RegistrationCount)
	require.Equal(t, 1, exam2Item.ProvinceCount)
	require.Equal(t, 1, exam2Item.SchoolCount)
}

// TestListExamRegistrantDiversity_ExcludesExamsWithNoMatchingRegistrant is
// the review-reported bug ("ada yg 0 sekolah masih muncul ketika filter
// aktif"): an exam whose only registrants are OUTSIDE the active filter must
// not appear in the result at all — not appear with zero counts.
func TestListExamRegistrantDiversity_ExcludesExamsWithNoMatchingRegistrant(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolInFilter := insertSchoolWithLocation(t, pool, "Diversity Zero School In", "divzero-in", "11", "1101")
	schoolOutOfFilter := insertSchoolWithLocation(t, pool, "Diversity Zero School Out", "divzero-out", "12", "1201")

	var examMatching, examNonMatching, examNoRegistrants uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity Zero Exam Matching', 'draft') RETURNING id`,
	).Scan(&examMatching))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity Zero Exam Non-matching', 'draft') RETURNING id`,
	).Scan(&examNonMatching))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity Zero Exam No Registrants', 'draft') RETURNING id`,
	).Scan(&examNoRegistrants))

	studentIn := insertSchoolUser(t, pool, "student", "Matching Student", schoolInFilter)
	studentOut := insertSchoolUser(t, pool, "student", "Non-matching Student", schoolOutOfFilter)

	_, err := pool.Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
		studentIn, examMatching, uuid.NewString(),
	)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
		studentOut, examNonMatching, uuid.NewString(),
	)
	require.NoError(t, err)
	// examNoRegistrants gets no registration rows at all.

	provinceFilter := "11"
	items, _, err := repo.ListExamRegistrantDiversity(ctx, ExamRegistrantDiversityFilter{
		Limit:      10,
		ProvinceID: &provinceFilter,
	})
	require.NoError(t, err)

	byExam := map[uuid.UUID]model.ExamRegistrantDiversityItem{}
	for _, item := range items {
		byExam[item.ExamID] = item
	}

	_, matchingPresent := byExam[examMatching]
	require.True(t, matchingPresent, "exam with a filter-matching registrant must appear")

	_, nonMatchingPresent := byExam[examNonMatching]
	require.False(t, nonMatchingPresent, "exam whose only registrant is outside the filter must not appear at all")

	_, noRegistrantsPresent := byExam[examNoRegistrants]
	require.False(t, noRegistrantsPresent, "exam with zero registrants must not appear once a filter is active")
}

// TestListExamRegistrantDiversity_ExamIDFilterNarrowsToOneExam proves the
// review-requested "filter ujian tertentu" (Opsi A, point 3): passing
// ExamID returns only that exam, even though it would otherwise also match
// the province filter alongside other exams.
func TestListExamRegistrantDiversity_ExamIDFilterNarrowsToOneExam(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	school := insertSchoolWithLocation(t, pool, "Diversity ExamID School", "divexamid", "11", "1101")

	var exam1, exam2 uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity ExamID Exam 1', 'draft') RETURNING id`,
	).Scan(&exam1))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity ExamID Exam 2', 'draft') RETURNING id`,
	).Scan(&exam2))

	student1 := insertSchoolUser(t, pool, "student", "ExamID Student 1", school)
	student2 := insertSchoolUser(t, pool, "student", "ExamID Student 2", school)
	for studentID, examID := range map[uuid.UUID]uuid.UUID{student1: exam1, student2: exam2} {
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
			studentID, examID, uuid.NewString(),
		)
		require.NoError(t, err)
	}

	provinceFilter := "11"
	exam1Str := exam1.String()
	items, _, err := repo.ListExamRegistrantDiversity(ctx, ExamRegistrantDiversityFilter{
		Limit:      10,
		ProvinceID: &provinceFilter,
		ExamID:     &exam1Str,
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, exam1, items[0].ExamID)
}

// TestAggregateExamRegistrantDiversity_TrueDistinctAcrossExams proves the
// box-summary (review point 2) computes a TRUE distinct province/city/school
// count across every matching exam, not a sum of each exam's own distinct
// count — two exams sharing the same province must count that province once.
func TestAggregateExamRegistrantDiversity_TrueDistinctAcrossExams(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolA := insertSchoolWithLocation(t, pool, "Diversity Agg School A", "divagg-a", "11", "1101")
	schoolB := insertSchoolWithLocation(t, pool, "Diversity Agg School B", "divagg-b", "11", "1102")

	var exam1, exam2 uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity Agg Exam 1', 'draft') RETURNING id`,
	).Scan(&exam1))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Diversity Agg Exam 2', 'draft') RETURNING id`,
	).Scan(&exam2))

	studentA := insertSchoolUser(t, pool, "student", "Agg Student A", schoolA)
	studentB := insertSchoolUser(t, pool, "student", "Agg Student B", schoolB)
	for studentID, examID := range map[uuid.UUID]uuid.UUID{studentA: exam1, studentB: exam2} {
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3)`,
			studentID, examID, uuid.NewString(),
		)
		require.NoError(t, err)
	}

	provinceFilter := "11"
	summary, err := repo.AggregateExamRegistrantDiversity(ctx, ExamRegistrantDiversityFilter{
		ProvinceID: &provinceFilter,
	})
	require.NoError(t, err)
	require.Equal(t, 2, summary.RegistrationCount, "one registrant on each of the two exams")
	require.Equal(t, 1, summary.ProvinceCount, "both schools share province 11 — must count once, not twice")
	require.Equal(t, 2, summary.CityCount)
	require.Equal(t, 2, summary.SchoolCount)
}
