package repository

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestListExamRoster_NameSearchFiltersRows(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	school := insertSchool(t, pool, "Roster Search School", "roster-search")

	var examID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Search Roster', 'draft') RETURNING id`,
	).Scan(&examID))

	checkedIn := insertSchoolUser(t, pool, "student", "Andi Saputra", school)
	submitted := insertSchoolUser(t, pool, "student", "Budi Santoso", school)
	revoked := insertSchoolUser(t, pool, "student", "Citra Dewi", school)

	insertRegistration := func(studentID uuid.UUID, participantNumber int, status string) {
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, participant_number, status)
			 VALUES ($1, $2, $3, $4, $5)`,
			studentID, examID, uuid.NewString(), participantNumber, status,
		)
		require.NoError(t, err)
	}
	insertRegistration(checkedIn, 1, "checked_in")
	insertRegistration(submitted, 2, "submitted")
	insertRegistration(revoked, 3, "revoked")

	rows, _, err := repo.ListExamRoster(ctx, examID, nil, model.ExamRosterFilter{Limit: 10, Q: "saputra"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, checkedIn, rows[0].StudentID)

	rows, _, err = repo.ListExamRoster(ctx, examID, nil, model.ExamRosterFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	counts, err := repo.CountExamRegistrationsByStatus(ctx, examID, nil)
	require.NoError(t, err)
	require.Equal(t, model.ExamRegistrationCounts{
		Total: 3, Registered: 0, CheckedIn: 1, Submitted: 1, Revoked: 1,
	}, counts)

	otherSchool := insertSchool(t, pool, "Roster Search Other", "roster-search-b")
	other := insertSchoolUser(t, pool, "student", "Andi Lain", otherSchool)
	insertRegistration(other, 4, "registered")

	scoped, err := repo.CountExamRegistrationsByStatus(ctx, examID, &school)
	require.NoError(t, err)
	require.Equal(t, 3, scoped.Total)

	unscoped, err := repo.CountExamRegistrationsByStatus(ctx, examID, nil)
	require.NoError(t, err)
	require.Equal(t, 4, unscoped.Total)
	require.Equal(t, 1, unscoped.Registered)
}
