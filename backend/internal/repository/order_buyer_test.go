package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedSchool(t *testing.T, name string) uuid.UUID {
	t.Helper()
	pool := newReportingTestPool(t)
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO school (name, code) VALUES ($1, $2) RETURNING id`,
		name, "SCH-"+uuid.NewString()[:8],
	).Scan(&id))
	return id
}

// The orders table shows the buyer's school and grade beneath their name — a
// student id told an admin nothing. Both come from users, so they are resolved
// in the same batched lookup as the name rather than a query per row.
func TestGetBuyersByIDs_resolvesSchoolAndGrade(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	schoolID := seedSchool(t, "SMAN 3 Bogor")

	var withSchool uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, school_id, grade)
		 VALUES ($1, 'student', 'Rani Puspita', $2, 12) RETURNING id`,
		"buyer-"+uuid.NewString()+"@test.local", schoolID,
	).Scan(&withSchool))

	got, err := repo.GetBuyersByIDs(ctx, []string{withSchool.String()})
	require.NoError(t, err)

	b := got[withSchool.String()]
	require.Equal(t, "Rani Puspita", b.Name)
	require.Equal(t, "SMAN 3 Bogor", b.School)
	require.NotNil(t, b.Grade)
	require.Equal(t, 12, *b.Grade)
}

// Plenty of buyers have neither — the join must not drop them from the result,
// or their orders would render with the "student not found" fallback name.
func TestGetBuyersByIDs_keepsBuyersWithNoSchoolOrGrade(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	var bare uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', 'Tanpa Sekolah') RETURNING id`,
		"bare-"+uuid.NewString()+"@test.local",
	).Scan(&bare))

	got, err := repo.GetBuyersByIDs(ctx, []string{bare.String()})
	require.NoError(t, err)

	b, ok := got[bare.String()]
	require.True(t, ok, "a buyer with no school must still resolve")
	require.Equal(t, "Tanpa Sekolah", b.Name)
	require.Equal(t, "", b.School)
	require.Nil(t, b.Grade)
}

func TestGetBuyersByIDs_emptyInput(t *testing.T) {
	repo := New(newReportingTestPool(t))
	got, err := repo.GetBuyersByIDs(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, got)
}
