package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
)

func newTestimonial(name string) *model.Testimonial {
	return &model.Testimonial{Name: name, RoleLabel: "SMP Negeri 1"}
}

func publishable(name string) *model.Testimonial {
	now := time.Now()
	t := newTestimonial(name)
	t.PhotoKey = strp("question/" + uuid.NewString() + ".jpg")
	t.HasConsent = true
	t.ConsentNote = strp("WA orang tua 2026-11-20")
	t.ConsentAt = &now
	t.IsVisible = true
	return t
}

func requireCheckViolation(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23514", pgErr.Code)
}

func TestTestimonial_CRUDRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := New(newGradingTestPool(t))

	in := newTestimonial("Budi")
	in.Content = map[string]model.TestimonialContent{
		"id": {Quote: "Bagus", Achievements: []string{"Juara 1"}},
		"en": {Quote: "Great", Achievements: []string{}},
	}
	require.NoError(t, repo.CreateTestimonial(ctx, in))
	require.NotEmpty(t, in.ID)

	got, err := repo.GetTestimonial(ctx, in.ID)
	require.NoError(t, err)
	require.Equal(t, "Budi", got.Name)
	require.Equal(t, in.Content, got.Content)
	require.Nil(t, got.PhotoKey)
	require.False(t, got.IsVisible)

	got.Name = "Budi S"
	got.SortOrder = 4
	require.NoError(t, inTestimonialTx(t, repo, func(tx pgx.Tx) error { return repo.UpdateTestimonialTx(ctx, tx, got.ID, got) }))
	again, err := repo.GetTestimonial(ctx, got.ID)
	require.NoError(t, err)
	require.Equal(t, "Budi S", again.Name)
	require.Equal(t, 4, again.SortOrder)

	require.NoError(t, inTestimonialTx(t, repo, func(tx pgx.Tx) error { return repo.DeleteTestimonialTx(ctx, tx, got.ID) }))
	_, err = repo.GetTestimonial(ctx, got.ID)
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, inTestimonialTx(t, repo, func(tx pgx.Tx) error { return repo.DeleteTestimonialTx(ctx, tx, got.ID) }), ErrNotFound)
	require.ErrorIs(t, inTestimonialTx(t, repo, func(tx pgx.Tx) error { return repo.UpdateTestimonialTx(ctx, tx, got.ID, got) }), ErrNotFound)
}

func TestTestimonial_ChecksRejectUnpublishableVisible(t *testing.T) {
	ctx := context.Background()
	repo := New(newGradingTestPool(t))

	noConsent := publishable("No Consent")
	noConsent.HasConsent = false
	requireCheckViolation(t, repo.CreateTestimonial(ctx, noConsent))

	noNote := publishable("No Note")
	noNote.ConsentNote = nil
	requireCheckViolation(t, repo.CreateTestimonial(ctx, noNote))

	noPhoto := publishable("No Photo")
	noPhoto.PhotoKey = nil
	requireCheckViolation(t, repo.CreateTestimonial(ctx, noPhoto))

	draft := newTestimonial("Draft")
	require.NoError(t, repo.CreateTestimonial(ctx, draft), "a hidden draft needs no photo or consent")

	draft.IsVisible = true
	requireCheckViolation(t, inTestimonialTx(t, repo, func(tx pgx.Tx) error { return repo.UpdateTestimonialTx(ctx, tx, draft.ID, draft) }))
}

func TestTestimonial_ListVisibleAndConsentByName(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	_, err := pool.Exec(ctx, `DELETE FROM testimonial`)
	require.NoError(t, err)

	var adminID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, username) VALUES ($1, 'super_admin', 'Ibu Admin', $2) RETURNING id`,
		"admin-"+uuid.NewString()[:8]+"@test.local", "adm-"+uuid.NewString()[:8]).Scan(&adminID))

	second := publishable("Second")
	second.SortOrder = 2
	second.ConsentBy = strp(adminID.String())
	first := publishable("First")
	first.SortOrder = 1
	hidden := newTestimonial("Hidden")
	hidden.SortOrder = 0
	for _, x := range []*model.Testimonial{second, first, hidden} {
		require.NoError(t, repo.CreateTestimonial(ctx, x))
	}
	require.Equal(t, "Ibu Admin", *second.ConsentByName)

	all, err := repo.ListTestimonials(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"Hidden", "First", "Second"}, []string{all[0].Name, all[1].Name, all[2].Name})
	require.Equal(t, "Ibu Admin", *all[2].ConsentByName)
	require.Nil(t, all[1].ConsentByName)

	visible, err := repo.ListVisibleTestimonials(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"First", "Second"}, []string{visible[0].Name, visible[1].Name})
}

func inTestimonialTx(t *testing.T, repo *Repository, fn func(pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
