package service

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/stretchr/testify/require"
)

func TestAdminCreateAffiliator_activeWithCodeAndTempPassword(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "admin-create-"+uniqueSuffix()+"@test.local")
	email := "new-aff-" + uniqueSuffix() + "@test.local"

	created, err := svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{Name: "Pak Baru", Email: email, Phone: "081234567890"})
	require.NoError(t, err)
	require.NotEmpty(t, created.Code)
	require.NotEmpty(t, created.TempPassword)

	var status, programStatus string
	var temporary bool
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT u.status, a.program_status, u.password_temporary FROM users u JOIN affiliators a ON a.user_id = u.id WHERE u.email = $1`, email,
	).Scan(&status, &programStatus, &temporary))
	require.Equal(t, "active", status)
	require.Equal(t, "active", programStatus)
	require.True(t, temporary, "the affiliator must change the temporary password at first login")

	_, err = svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{Name: "Pak Baru", Email: email, Phone: "081234567890"})
	require.ErrorIs(t, err, ErrEmailTaken)
	chosen, err := svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{Name: "Pilih Sandi", Email: "pw-" + uniqueSuffix() + "@test.local", Phone: "0811", Password: "SandiPilihan1"})
	require.NoError(t, err)
	require.Equal(t, "SandiPilihan1", chosen.TempPassword, "an admin-chosen password is used as given")
	_, err = svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{Name: "Sandi Pendek", Email: "pw2-" + uniqueSuffix() + "@test.local", Phone: "0811", Password: "short"})
	require.ErrorIs(t, err, ErrWeakPassword)
	_, err = svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{Name: "Tanpa Telepon", Email: "np-" + uniqueSuffix() + "@test.local"})
	require.ErrorIs(t, err, ErrAffiliatePhoneRequired, "the phone number is mandatory")
}

func TestAdminBulkCreateAffiliators_reportsEachRow(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "admin-bulk-"+uniqueSuffix()+"@test.local")
	taken := "bulk-taken-" + uniqueSuffix() + "@test.local"
	_, err := svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{Name: "Sudah Ada", Email: taken, Phone: "0811"})
	require.NoError(t, err)

	res, err := svc.AdminBulkCreateAffiliators(ctx, actor, []AdminCreateAffiliatorInput{
		{Name: "Satu", Email: "bulk1-" + uniqueSuffix() + "@test.local", Phone: "0812"},
		{Name: "Dua", Email: taken, Phone: "0813"},
		{Name: "Tiga", Email: "bulk3-" + uniqueSuffix() + "@test.local"},
	})
	require.NoError(t, err)
	require.Len(t, res, 3)
	require.NotNil(t, res[0].Created)
	require.Equal(t, "email_taken", res[1].Error)
	require.Equal(t, "phone_required", res[2].Error)

	_, err = svc.AdminBulkCreateAffiliators(ctx, actor, make([]AdminCreateAffiliatorInput, affiliateBulkCreateMax+1))
	require.ErrorIs(t, err, ErrAffiliateBulkTooLarge)
}

func TestDecideAffiliateApproval_reopenRejected(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "admin-reopen-"+uniqueSuffix()+"@test.local")
	rejected := seedAffiliatorOwner(t, repo, "reopen-"+uniqueSuffix()+"@test.local", "", "rejected")

	aff, err := svc.DecideAffiliateApproval(ctx, actor, rejected.affiliatorID.String(), "reopen", "")
	require.NoError(t, err)
	require.Equal(t, "pending_approval", aff.Status)
	require.Nil(t, aff.RejectionReason)

	_, err = svc.DecideAffiliateApproval(ctx, actor, rejected.affiliatorID.String(), "reopen", "")
	require.ErrorIs(t, err, ErrAffiliateInvalidTransition, "only a rejected profile can be reopened")
}

func TestAffiliatorOnlyPromos_hiddenFromStudentsListedForAffiliators(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	userID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator Promo "+uniqueSuffix(), "active", "bronze")
	suspendedID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator Promo S "+uniqueSuffix(), "suspended", "bronze")
	pct := 10.0
	code := "AFFPROMO" + uniqueSuffix()

	_, err := svc.AdminCreatePromoCode(ctx, model.PromoCode{Code: code, DiscountPercent: &pct, IsPublic: true, AffiliatorOnly: true})
	require.NoError(t, err)

	mine, err := svc.ListMyAffiliatePromos(ctx, userID)
	require.NoError(t, err)
	require.Contains(t, promoCodes(mine), code)
	none, err := svc.ListMyAffiliatePromos(ctx, suspendedID)
	require.NoError(t, err)
	require.Empty(t, none, "a suspended affiliator does not get promos to share")

	public, err := svc.ListActivePublicPromos(ctx)
	require.NoError(t, err)
	require.NotContains(t, promoCodes(public), code, "students never see an affiliator-only promo in the listing")
}

func promoCodes(promos []model.PromoCode) []string {
	out := make([]string, 0, len(promos))
	for _, p := range promos {
		out = append(out, p.Code)
	}
	return out
}
