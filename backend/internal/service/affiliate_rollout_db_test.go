package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExtendBinding_AFreeOrderNeverExtends(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3, minExtend: 0})
	owner := seedAffiliatorOwner(t, repo, "aff-free-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-free-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "manual_code")
	binding := getActiveAffiliateBinding(t, repo, student.ID)

	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.ExtendBinding(ctx, tx, student.ID, binding.StartedAt.Add(time.Hour), 0))
	require.NoError(t, tx.Commit(ctx))

	row := getActiveAffiliateBinding(t, repo, student.ID)
	require.True(t, row.ExpiresAt.Equal(*binding.ExpiresAt), "a free order never extends, even with min_extend_amount=0")
}

func TestAffiliateValidate_IsCaseAndSpaceInsensitiveAndReportsCookieDays(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO affiliate_settings (id, cookie_days) VALUES (1, 45)
		 ON CONFLICT (id) DO UPDATE SET cookie_days = 45`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `UPDATE affiliate_settings SET cookie_days = 30 WHERE id = 1`) })
	owner := seedAffiliatorOwner(t, repo, "aff-case-"+uniqueSuffix()+"@test.local", "", "active")

	typed := "  " + strings.ToLower(owner.code) + " "
	res, err := svc.ValidateAffiliateCode(ctx, typed)
	require.NoError(t, err)
	require.True(t, res.Valid, "a lowercase link must still resolve")
	require.Equal(t, 45, res.CookieDays, "the page needs the admin's cookie lifetime")

	unknown, err := svc.ValidateAffiliateCode(ctx, "NOSUCHCD")
	require.NoError(t, err)
	require.False(t, unknown.Valid)
	require.Equal(t, 45, unknown.CookieDays, "the cookie is still sized for a code that is not active yet")
}

func TestRegisterAffiliator_DailyCap(t *testing.T) {
	svc, repo, _ := newAffiliateActivationService(t)
	ctx := context.Background()
	setAffiliateFlagForCommission(t, repo, "true")
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(ctx, `UPDATE affiliate_settings SET daily_registration_cap = NULL WHERE id = 1`)
	})

	register := func(label string) error {
		_, err := svc.RegisterAffiliator(ctx, "cap-"+label+"-"+uniqueSuffix()+"@test.local", "password123", "Budi Santoso", "0812"+uniqueSuffix()[:6], "v1", "127.0.0.1")
		return err
	}

	// The shared fixture already holds profiles created today; allow exactly one more.
	nowWIB := time.Now().In(affiliateWIBLocation())
	dayStart := time.Date(nowWIB.Year(), nowWIB.Month(), nowWIB.Day(), 0, 0, 0, 0, nowWIB.Location())
	today, err := repo.CountAffiliatorsCreatedSince(ctx, dayStart)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO affiliate_settings (id, daily_registration_cap) VALUES (1, $1)
		 ON CONFLICT (id) DO UPDATE SET daily_registration_cap = $1`, today+1)
	require.NoError(t, err)

	require.NoError(t, register("first"))
	require.ErrorIs(t, register("second"), ErrAffiliateRegistrationCap)

	_, err = repo.Pool().Exec(ctx, `UPDATE affiliate_settings SET daily_registration_cap = NULL WHERE id = 1`)
	require.NoError(t, err)
	require.NoError(t, register("uncapped"))
}

// A failed flag read must surface on the money paths instead of reading as
// "off", which would skip a binding or drop a commission without a trace.
func TestAffiliateEnabledErr_SurfacesReadFailures(t *testing.T) {
	svc, repo := newRealDBService(t)
	setAffiliateFlagForCommission(t, repo, "true")

	on, err := svc.affiliateEnabledErr(context.Background())
	require.NoError(t, err)
	require.True(t, on)

	broken, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = svc.affiliateEnabledErr(broken)
	require.Error(t, err)
	require.False(t, svc.affiliateEnabled(broken), "read paths treat a failed read as off")

	tx, err := repo.BeginTx(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	require.Error(t, svc.paidOrderBindingTx(broken, tx, seedCommissionOrder(t, repo,
		seedAffiliateTestStudent(t, repo, "student-flagerr-"+uniqueSuffix()+"@test.local", "").ID,
		time.Now().UTC(), 0, commissionSeedItem{productType: "exam", jumlah: 1000})), "settlement must fail, not skip")
	require.Error(t, svc.CreateCommissionsForOrder(broken, tx, seedCommissionOrder(t, repo,
		seedAffiliateTestStudent(t, repo, "student-flagerr2-"+uniqueSuffix()+"@test.local", "").ID,
		time.Now().UTC(), 0, commissionSeedItem{productType: "exam", jumlah: 1000}), nil), "commission creation must fail so it is retried")
}

func TestSeedDevAffiliateDefaults(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	reset := func() {
		_, _ = repo.Pool().Exec(ctx, `DELETE FROM system_config WHERE key = 'affiliate_enabled'`)
		_, _ = repo.Pool().Exec(ctx, `UPDATE affiliate_settings SET launch_date = NULL WHERE id = 1`)
	}
	reset()
	t.Cleanup(reset)

	flag := func() (string, bool) {
		v, found, err := repo.GetSystemConfigValue(ctx, "affiliate_enabled")
		require.NoError(t, err)
		return v, found
	}
	launch := func() *time.Time {
		var d *time.Time
		err := repo.Pool().QueryRow(ctx, `SELECT launch_date FROM affiliate_settings WHERE id = 1`).Scan(&d)
		if err != nil {
			return nil
		}
		return d
	}

	for _, env := range []string{"prod", "staging", ""} {
		require.NoError(t, svc.SeedDevAffiliateDefaults(ctx, env))
		_, found := flag()
		require.False(t, found, "env %q must stay off", env)
		require.Nil(t, launch(), "env %q must not set a launch date", env)
	}

	require.NoError(t, svc.SeedDevAffiliateDefaults(ctx, "dev"))
	v, found := flag()
	require.True(t, found)
	require.Equal(t, "true", v)
	require.NotNil(t, launch())
	require.True(t, launch().Before(time.Now().AddDate(-10, 0, 0)))

	require.NoError(t, svc.SeedDevAffiliateDefaults(ctx, "dev"))

	t.Run("a hand-set value survives a restart", func(t *testing.T) {
		require.NoError(t, repo.UpsertSystemConfig(ctx, "affiliate_enabled", "false", false))
		custom := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		_, err := repo.Pool().Exec(ctx, `UPDATE affiliate_settings SET launch_date = $1 WHERE id = 1`, custom)
		require.NoError(t, err)

		require.NoError(t, svc.SeedDevAffiliateDefaults(ctx, "dev"))

		v, _ := flag()
		require.Equal(t, "false", v)
		require.True(t, launch().Equal(custom))
	})
}
