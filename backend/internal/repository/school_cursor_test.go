package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNameCursor_roundTrip(t *testing.T) {
	id := uuid.New()

	// A name containing a comma is the case the previous "split on first
	// comma" convention would have broken; DecodeNameCursor must split on the
	// *last* comma instead, since a UUID never contains one.
	for _, name := range []string{"SMA Negeri 1", "SMA 1, Jakarta Selatan"} {
		gotName, gotID, err := DecodeNameCursor(EncodeNameCursor(name, id.String()))
		require.NoError(t, err)
		require.Equal(t, name, gotName)
		require.Equal(t, id, gotID)
	}
}

func TestNameCursor_rejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "no-comma-here", "name,not-a-uuid"} {
		_, _, err := DecodeNameCursor(bad)
		require.ErrorIs(t, err, ErrInvalidCursor, "input %q", bad)
	}
}

// seedSchoolRow inserts a school with an explicit status, bypassing
// CreateSchool (which always inserts 'active') so paging/filter tests can mix
// statuses without an extra UpdateSchoolStatus round trip per row.
func seedSchoolRow(t *testing.T, repo *Repository, name, code, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.pool.QueryRow(context.Background(),
		`INSERT INTO school (name, code, school_types, status) VALUES ($1, $2, '{"SMA"}', $3) RETURNING id`,
		name, code, status,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// The previous keyset filtered `id > cursor` while ordering by name ASC —
// unrelated to the sort key, since ids are random UUIDs. Names are chosen
// here so name order and UUID-insertion order disagree, which is exactly the
// case that made bulk-imported schools disappear from "load more"
// (docs/backlog/school-bulk-list-pagination.md). Paging must visit every
// school exactly once, in stable name order.
func TestListSchoolsAdmin_pagingVisitsEverySchoolExactlyOnce(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	suffix := uuid.New().String()[:8]
	q := "pgsch_" + suffix

	names := []string{
		"SMP Negeri 9 " + q, "SD Islam Terpadu " + q, "SMK Kartika " + q,
		"MA Al Hikmah " + q, "SMA Negeri 3 " + q, "SDN Percobaan " + q,
		"SMA Negeri 1 " + q, "TK Aisyiyah " + q, "SMP Muhammadiyah " + q,
		"SMA Negeri 2 " + q, "SD Kristen " + q, "SMK Negeri 1 " + q,
		"MI Nurul Iman " + q, "SMA Katolik " + q, "SD Marsudirini " + q,
		"SMP Kristen " + q, "SMK Farmasi " + q, "SMA Plus " + q,
		"SD Muhammadiyah " + q, "SMP Islam " + q, "TK Kartika " + q,
		"SMA Unggulan " + q, "SD Negeri 1 " + q, "SMK Bina Karya " + q,
		"SMP Negeri 1 " + q,
	}

	want := map[uuid.UUID]bool{}
	for _, name := range names {
		id := seedSchoolRow(t, repo, name, "c_"+uuid.New().String()[:12], "active")
		want[id] = true
	}

	seen := map[uuid.UUID]int{}
	var lastName string
	cursor := ""
	for page := 0; page < len(names)+2; page++ {
		rows, next, err := repo.ListSchoolsAdmin(ctx, SchoolAdminFilter{Q: q, Limit: 3, Cursor: cursor})
		require.NoError(t, err)
		for _, r := range rows {
			id, err := uuid.Parse(r.ID)
			require.NoError(t, err)
			seen[id]++
			require.GreaterOrEqual(t, r.Name, lastName, "name order must be non-decreasing across pages")
			lastName = r.Name
		}
		if next == "" {
			break
		}
		cursor = next
	}

	require.Len(t, seen, len(want), "every school visited")
	for id := range want {
		require.Equal(t, 1, seen[id], "school %s visited exactly once", id)
	}
}

func TestListSchoolsAdmin_malformedCursorReturnsErrInvalidCursor(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	for _, bad := range []string{"no-comma", "name,not-a-uuid"} {
		_, _, err := repo.ListSchoolsAdmin(ctx, SchoolAdminFilter{Cursor: bad})
		require.ErrorIs(t, err, ErrInvalidCursor, "cursor %q", bad)
	}
}

func TestListSchoolsAdmin_statusFilter(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	suffix := uuid.New().String()[:8]
	q := "statfilt_" + suffix
	activeID := seedSchoolRow(t, repo, "Active School "+q, "a_"+uuid.New().String()[:12], "active")
	deactivatedID := seedSchoolRow(t, repo, "Deactivated School "+q, "d_"+uuid.New().String()[:12], "deactivated")

	rows, _, err := repo.ListSchoolsAdmin(ctx, SchoolAdminFilter{Q: q, Status: "active", Limit: 10})
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	require.True(t, ids[activeID.String()], "active school should be included")
	require.False(t, ids[deactivatedID.String()], "deactivated school should be excluded")
}

func TestCountSchoolsAdmin_matchesFilteredTotal(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	suffix := uuid.New().String()[:8]
	q := "cntsch_" + suffix
	seedSchoolRow(t, repo, "Counted Active 1 "+q, "ca1_"+uuid.New().String()[:10], "active")
	seedSchoolRow(t, repo, "Counted Active 2 "+q, "ca2_"+uuid.New().String()[:10], "active")
	seedSchoolRow(t, repo, "Counted Inactive "+q, "ci1_"+uuid.New().String()[:10], "deactivated")

	counts, err := repo.CountSchoolsAdmin(ctx, SchoolAdminFilter{Q: q})
	require.NoError(t, err)
	require.Equal(t, 3, counts.Total)
	require.Equal(t, 2, counts.Active)
}

func TestListSchoolOptions_excludesDeactivated(t *testing.T) {
	pool := newGradingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	suffix := uuid.New().String()[:8]
	activeID := seedSchoolRow(t, repo, "Option Active "+suffix, "oa_"+uuid.New().String()[:10], "active")
	deactivatedID := seedSchoolRow(t, repo, "Option Deactivated "+suffix, "od_"+uuid.New().String()[:10], "deactivated")

	options, err := repo.ListSchoolOptions(ctx)
	require.NoError(t, err)

	byID := map[string]SchoolOption{}
	for _, o := range options {
		byID[o.ID] = o
	}
	active, ok := byID[activeID.String()]
	require.True(t, ok, "active school should be in options")
	require.Equal(t, []string{"SMA"}, active.SchoolTypes, "school_types should be carried through for the jenjang picker")
	_, ok = byID[deactivatedID.String()]
	require.False(t, ok, "deactivated school should not be in options")
}
