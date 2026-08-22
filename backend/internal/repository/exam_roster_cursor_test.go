package repository

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestListExamRoster_CursorWalksScopedRowsBothDirections(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	schoolA := insertSchool(t, pool, "Roster School A", "roster-a")
	schoolB := insertSchool(t, pool, "Roster School B", "roster-b")

	var examID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ('Cursor Roster', 'draft') RETURNING id`,
	).Scan(&examID))

	for i := 0; i < 5; i++ {
		studentID := insertSchoolUser(t, pool, "student", "Roster Student", schoolA)
		var participantNumber *int
		if i < 3 {
			n := i + 1
			participantNumber = &n
		}
		_, err := pool.Exec(ctx,
			`INSERT INTO exam_registration (student_id, exam_id, token, participant_number)
			 VALUES ($1, $2, $3, $4)`,
			studentID, examID, uuid.NewString(), participantNumber,
		)
		require.NoError(t, err)
	}
	otherStudent := insertSchoolUser(t, pool, "student", "Other School Student", schoolB)
	_, err := pool.Exec(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, participant_number)
		 VALUES ($1, $2, $3, 4)`,
		otherStudent, examID, uuid.NewString(),
	)
	require.NoError(t, err)

	for direction, wantNumbers := range map[string][]int{"asc": {1, 2, 3}, "desc": {3, 2, 1}} {
		t.Run(direction, func(t *testing.T) {
			cursor := ""
			seen := map[uuid.UUID]bool{}
			var gotNumbers []int
			var nilCount int
			for page := 0; page < 4; page++ {
				rows, nextCursor, err := repo.ListExamRoster(ctx, examID, &schoolA, model.ExamRosterFilter{
					Cursor: cursor,
					Limit:  2,
					Sort:   direction,
				})
				require.NoError(t, err)
				require.LessOrEqual(t, len(rows), 2)
				for _, row := range rows {
					require.False(t, seen[row.RegistrationID], "registration returned twice")
					seen[row.RegistrationID] = true
					require.NotEqual(t, otherStudent, row.StudentID)
					if row.ParticipantNumber == nil {
						nilCount++
					} else {
						gotNumbers = append(gotNumbers, *row.ParticipantNumber)
					}
				}
				cursor = nextCursor
				if cursor == "" {
					break
				}
			}
			require.Len(t, seen, 5)
			require.Equal(t, wantNumbers, gotNumbers)
			require.Equal(t, 2, nilCount)
		})
	}

	legacyRows, err := repo.GetExamRoster(ctx, examID, nil)
	require.NoError(t, err)
	require.Len(t, legacyRows, 6)
	_, _, err = repo.ListExamRoster(ctx, examID, nil, model.ExamRosterFilter{Cursor: "broken", Limit: 20})
	require.True(t, errors.Is(err, ErrInvalidCursor))
}
