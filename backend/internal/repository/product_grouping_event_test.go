package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
)

func newEvent(name string) *model.ProductGrouping {
	return &model.ProductGrouping{Name: name, Status: model.ProductGroupingStatusDraft, Slug: "ev-" + uuid.NewString()}
}

func strp(s string) *string { return &s }

func intp(i int) *int { return &i }

func TestProductGrouping_EventFieldsRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := New(newGradingTestPool(t))

	g := newEvent("Round Trip")
	g.ShortName = strp("RT")
	g.Scope = strp("international")
	g.Subjects = []string{"math", "science"}
	g.Tone = "teal"
	g.Illustration = "math"
	g.LogoKey = strp("events/rt.png")
	g.OfficialURL = strp("https://example.org")
	g.Content = map[string]model.EventNarrative{
		"id": {FullName: "Round Trip Penuh", Story: []string{"a", "b"}, Rounds: []string{}, Categories: []model.EventCategory{{Name: "SD", Detail: "kelas 1-6"}}, Facts: []model.EventFact{{Label: "Biaya", Value: "0"}}},
		"en": {FullName: "Round Trip Full", Story: []string{}, Rounds: []string{"r1"}, Categories: []model.EventCategory{}, Facts: []model.EventFact{}},
	}
	g.ParticipantPrefix = strp("RTRP")
	require.NoError(t, repo.CreateProductGrouping(ctx, g))

	got, err := repo.GetProductGroupingByID(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, g.Slug, got.Slug)
	require.Equal(t, "RT", *got.ShortName)
	require.Equal(t, "international", *got.Scope)
	require.Equal(t, []string{"math", "science"}, got.Subjects)
	require.Equal(t, "teal", got.Tone)
	require.Equal(t, "math", got.Illustration)
	require.Equal(t, "events/rt.png", *got.LogoKey)
	require.Equal(t, "https://example.org", *got.OfficialURL)
	require.Equal(t, g.Content, got.Content)
	require.False(t, got.IsVisible)
	require.Equal(t, "RTRP", *got.ParticipantPrefix)

	bySlug, err := repo.GetProductGroupingBySlug(ctx, g.Slug)
	require.NoError(t, err)
	require.Equal(t, g.ID, bySlug.ID)
	_, err = repo.GetProductGroupingBySlug(ctx, "no-such-slug")
	require.ErrorIs(t, err, ErrNotFound)

	got.Status = model.ProductGroupingStatusPublished
	got.IsVisible = true
	got.Tone = "gold"
	got.Scope = nil
	got.Content["en"] = model.EventNarrative{FullName: "Changed"}
	require.NoError(t, repo.UpdateProductGrouping(ctx, g.ID, got))

	updated, err := repo.GetProductGroupingByID(ctx, g.ID)
	require.NoError(t, err)
	require.True(t, updated.IsVisible)
	require.Equal(t, "gold", updated.Tone)
	require.Nil(t, updated.Scope)
	require.Equal(t, "Changed", updated.Content["en"].FullName)
	require.Equal(t, "RTRP", *updated.ParticipantPrefix, "untouched fields survive a full update")

	listed, err := repo.ListProductGroupings(ctx, false)
	require.NoError(t, err)
	var found bool
	for _, l := range listed {
		if l.ID == g.ID {
			found = true
			require.Equal(t, "Changed", l.Content["en"].FullName)
		}
	}
	require.True(t, found)
}

func TestProductGrouping_DefaultsAndUniqueErrors(t *testing.T) {
	ctx := context.Background()
	repo := New(newGradingTestPool(t))

	g := newEvent("Defaults")
	require.NoError(t, repo.CreateProductGrouping(ctx, g))
	require.Equal(t, "brand", g.Tone)
	require.Equal(t, "trophy", g.Illustration)
	require.Empty(t, g.Subjects)
	require.NotNil(t, g.Content)
	require.Empty(t, g.Content)

	dupSlug := newEvent("Dup Slug")
	dupSlug.Slug = g.Slug
	require.ErrorIs(t, repo.CreateProductGrouping(ctx, dupSlug), ErrGroupingSlugTaken)

	a := newEvent("Prefix A")
	a.ParticipantPrefix = strp("DUPA")
	require.NoError(t, repo.CreateProductGrouping(ctx, a))
	b := newEvent("Prefix B")
	b.ParticipantPrefix = strp("DUPA")
	require.ErrorIs(t, repo.CreateProductGrouping(ctx, b), ErrGroupingPrefixTaken)

	b.ParticipantPrefix = strp("DUPB")
	require.NoError(t, repo.CreateProductGrouping(ctx, b))
	b.ParticipantPrefix = strp("DUPA")
	require.ErrorIs(t, repo.UpdateProductGrouping(ctx, b.ID, b), ErrGroupingPrefixTaken)
	b.ParticipantPrefix = strp("DUPB")
	b.Slug = a.Slug
	require.ErrorIs(t, repo.UpdateProductGrouping(ctx, b.ID, b), ErrGroupingSlugTaken)
}

func TestProductGrouping_Checks(t *testing.T) {
	ctx := context.Background()
	repo := New(newGradingTestPool(t))

	visibleDraft := newEvent("Visible Draft")
	visibleDraft.IsVisible = true
	visibleDraft.ParticipantPrefix = strp("VDRF")
	require.Error(t, repo.CreateProductGrouping(ctx, visibleDraft), "visible requires published")

	visibleNoPrefix := newEvent("Visible No Prefix")
	visibleNoPrefix.IsVisible = true
	visibleNoPrefix.Status = model.ProductGroupingStatusPublished
	require.Error(t, repo.CreateProductGrouping(ctx, visibleNoPrefix), "visible requires a prefix")

	ok := newEvent("Visible Ok")
	ok.IsVisible = true
	ok.Status = model.ProductGroupingStatusPublished
	ok.ParticipantPrefix = strp("VOK")
	require.NoError(t, repo.CreateProductGrouping(ctx, ok))

	for _, bad := range []string{"A", "ABCDEFG", "abc", "AB1", "A-B"} {
		g := newEvent("Bad Prefix " + bad)
		g.ParticipantPrefix = strp(bad)
		err := repo.CreateProductGrouping(ctx, g)
		require.Errorf(t, err, "prefix %q must violate the format CHECK", bad)
		require.NotErrorIs(t, err, ErrGroupingPrefixTaken)
	}
	for _, good := range []string{"GOODAB", "GD"} {
		g := newEvent("Good Prefix " + good)
		g.ParticipantPrefix = strp(good)
		require.NoErrorf(t, repo.CreateProductGrouping(ctx, g), "prefix %q is valid", good)
	}

	badTone := newEvent("Bad Tone")
	badTone.Tone = "pink"
	require.Error(t, repo.CreateProductGrouping(ctx, badTone))
	badScope := newEvent("Bad Scope")
	badScope.Scope = strp("galactic")
	require.Error(t, repo.CreateProductGrouping(ctx, badScope))
	badIll := newEvent("Bad Illustration")
	badIll.Illustration = "pie"
	require.Error(t, repo.CreateProductGrouping(ctx, badIll))
}

func TestProductGrouping_ScheduleReplaceInOneTx(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	g := newEvent("Schedule")
	require.NoError(t, repo.CreateProductGrouping(ctx, g))

	replace := func(items []model.ProductGroupingScheduleItem, commit bool) error {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		if err := repo.ReplaceGroupingScheduleTx(ctx, tx, g.ID, items); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if !commit {
			return tx.Rollback(ctx)
		}
		return tx.Commit(ctx)
	}

	first := []model.ProductGroupingScheduleItem{
		{Label: "Penutupan", StartsOn: "2026-09-01"},
		{Label: "Pendaftaran", LabelEN: strp("Registration"), StartsOn: "2026-03-01", EndsOn: strp("2026-04-30")},
	}
	require.NoError(t, replace(first, true))
	got, err := repo.ListGroupingSchedule(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "Pendaftaran", got[0].Label, "ordered by starts_on")
	require.Equal(t, "2026-03-01", got[0].StartsOn)
	require.Equal(t, "2026-04-30", *got[0].EndsOn)
	require.Equal(t, "Registration", *got[0].LabelEN)
	require.Nil(t, got[1].EndsOn)
	require.Nil(t, got[1].LabelEN)
	require.NotEmpty(t, got[1].ID)

	require.NoError(t, replace(nil, false), "rolled-back delete")
	got, err = repo.ListGroupingSchedule(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, got, 2, "rollback leaves the old rows")

	bad := []model.ProductGroupingScheduleItem{
		{Label: "Ok", StartsOn: "2026-05-01"},
		{Label: "Backwards", StartsOn: "2026-05-10", EndsOn: strp("2026-05-01")},
	}
	require.Error(t, replace(bad, true), "ends_on before starts_on violates the CHECK")
	got, err = repo.ListGroupingSchedule(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, got, 2, "a failed replace leaves the old rows")

	require.NoError(t, replace([]model.ProductGroupingScheduleItem{{Label: "Only", StartsOn: "2026-06-01"}}, true))
	got, err = repo.ListGroupingSchedule(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Only", got[0].Label)
}

func TestProductGrouping_CountProductsAndProductLevel(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	repo := New(pool)

	g := newEvent("Count")
	require.NoError(t, repo.CreateProductGrouping(ctx, g))
	n, err := repo.CountGroupingProducts(ctx, g.ID)
	require.NoError(t, err)
	require.Zero(t, n)

	p := &model.Product{Type: "exam", Name: "Edition", Status: "published", Stages: []string{"SD"}, GradeMin: intp(4), GradeMax: intp(6)}
	require.NoError(t, repo.CreateProduct(ctx, p))
	plain := &model.Product{Type: "exam", Name: "Plain", Status: "published"}
	require.NoError(t, repo.CreateProduct(ctx, plain))

	got, err := repo.GetProductByID(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"SD"}, got.Stages)
	require.Equal(t, 4, *got.GradeMin)
	require.Equal(t, 6, *got.GradeMax)
	gotPlain, err := repo.GetProductByID(ctx, plain.ID)
	require.NoError(t, err)
	require.Equal(t, []string{}, gotPlain.Stages, "no stages reads as an empty slice, not nil")
	require.Nil(t, gotPlain.GradeMin)

	pid, err := uuid.Parse(p.ID)
	require.NoError(t, err)
	gid, err := uuid.Parse(g.ID)
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceGroupingProducts(ctx, tx, gid, []uuid.UUID{pid}))
	require.NoError(t, tx.Commit(ctx))
	n, err = repo.CountGroupingProducts(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	got.Stages = []string{"SMP", "SMA"}
	got.GradeMin = nil
	got.GradeMax = intp(12)
	require.NoError(t, repo.UpdateProduct(ctx, p.ID, got))
	updated, err := repo.GetProductByID(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"SMP", "SMA"}, updated.Stages)
	require.Nil(t, updated.GradeMin)
	require.Equal(t, 12, *updated.GradeMax)

	listed, err := repo.ListProductsByGrouping(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, []string{"SMP", "SMA"}, listed[0].Stages)

	for name, mutate := range map[string]func(*model.Product){
		"unknown stage": func(x *model.Product) { x.Stages = []string{"SMK"} },
		"grade_min 0":   func(x *model.Product) { x.GradeMin = intp(0) },
		"grade_max 13":  func(x *model.Product) { x.GradeMax = intp(13) },
		"max below min": func(x *model.Product) { x.GradeMin = intp(8); x.GradeMax = intp(7) },
	} {
		bad := *updated
		mutate(&bad)
		require.Errorf(t, repo.UpdateProduct(ctx, p.ID, &bad), "%s must violate a CHECK", name)
	}
}
