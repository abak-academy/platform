package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func seedRegistration0063(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()

	var studentID, examID, registrationID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', $2) RETURNING id`,
		"migration-0063-"+suffix+"@test.local", "Migration 0063",
	).Scan(&studentID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, status) VALUES ($1, 'draft') RETURNING id`,
		"Migration 0063 "+suffix,
	).Scan(&examID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		studentID, examID, "migration-0063-"+suffix, status,
	).Scan(&registrationID))

	return registrationID
}

func seedSession0063(t *testing.T, pool *pgxpool.Pool, registrationID uuid.UUID, attempt int, status string) {
	t.Helper()
	ctx := context.Background()

	var studentID, examID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT student_id, exam_id FROM exam_registration WHERE id = $1`, registrationID,
	).Scan(&studentID, &examID))
	_, err := pool.Exec(ctx,
		`INSERT INTO exam_session (registration_id, student_id, exam_id, attempt_number, started_at, status)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		registrationID, studentID, examID, attempt, time.Now(), status,
	)
	require.NoError(t, err)
}

func TestMigration0063_ReconcilesRegistrationFromLatestAttempt(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0062_school_name_id_index.up.sql")

	latestSubmitted := seedRegistration0063(t, pool, "in_progress")
	seedSession0063(t, pool, latestSubmitted, 1, "submitted")

	latestInProgress := seedRegistration0063(t, pool, "checked_in")
	seedSession0063(t, pool, latestInProgress, 1, "in_progress")

	multipleAttempts := seedRegistration0063(t, pool, "submitted")
	seedSession0063(t, pool, multipleAttempts, 1, "submitted")
	seedSession0063(t, pool, multipleAttempts, 2, "in_progress")

	latestOfMultipleSubmitted := seedRegistration0063(t, pool, "in_progress")
	seedSession0063(t, pool, latestOfMultipleSubmitted, 1, "in_progress")
	seedSession0063(t, pool, latestOfMultipleSubmitted, 2, "submitted")

	noSessionRegistered := seedRegistration0063(t, pool, "registered")
	noSessionCheckedIn := seedRegistration0063(t, pool, "checked_in")

	expected := map[uuid.UUID]string{
		latestSubmitted:           "submitted",
		latestInProgress:          "in_progress",
		multipleAttempts:          "in_progress",
		latestOfMultipleSubmitted: "submitted",
		noSessionRegistered:       "registered",
		noSessionCheckedIn:        "checked_in",
	}
	assertStatuses := func() {
		t.Helper()
		for registrationID, want := range expected {
			var got string
			require.NoError(t, pool.QueryRow(ctx,
				`SELECT status FROM exam_registration WHERE id = $1`, registrationID,
			).Scan(&got))
			require.Equal(t, want, got)
		}
	}

	applyMigrationFile(t, pool, "0063_exam_registration_status_reconciliation.up.sql")
	assertStatuses()
	applyMigrationFile(t, pool, "0063_exam_registration_status_reconciliation.up.sql")
	assertStatuses()
	applyMigrationFile(t, pool, "0063_exam_registration_status_reconciliation.down.sql")
	assertStatuses()
}
