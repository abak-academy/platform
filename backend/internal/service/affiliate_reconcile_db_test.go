package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Level revenue is attributed to the quarter a commission became final, so a
// physical order that settles after the evaluation day lands in the next
// quarter instead of in none, and a refunded (cancelled) commission never counts.
func TestAffiliateNetRevenue_CountsCommissionsInTheQuarterTheyBecomeFinal(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()
	owner := seedAffiliatorOwner(t, repo, "aff-netrev-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-netrev-"+uniqueSuffix()+"@test.local", "")
	studentID := uuid.MustParse(student.ID)

	wib := affiliateWIBLocation()
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 9, 0, 0, 0, wib) }
	q3Start := time.Date(2026, time.July, 1, 0, 0, 0, 0, wib)
	q3End := time.Date(2026, time.October, 1, 0, 0, 0, 0, wib)
	q4End := time.Date(2027, time.January, 1, 0, 0, 0, 0, wib)

	seed := func(status string, base float64, paid, final time.Time) {
		f := final
		seedAffiliateJobCommission(t, repo, owner.affiliatorID, studentID, affiliateJobCommissionSeed{
			productType: "book", base: base, pct: 5, amount: base * 0.05, status: status,
			level: "bronze", createdAt: paid, paidAt: paid, waitUntil: &f,
		})
	}
	seed("available", 1_000_000, day(time.September, 10), day(time.September, 17)) // final in Q3
	seed("available", 2_000_000, day(time.September, 28), day(time.October, 17))   // physical, final in Q4
	seed("cancelled", 1_500_000, day(time.August, 5), day(time.August, 12))        // refunded
	seed("pending", 700_000, day(time.September, 20), day(time.October, 20))       // not final yet

	q3, err := repo.GetAffiliateQuarterNetRevenue(ctx, owner.affiliatorID, q3Start, q3End)
	require.NoError(t, err)
	require.Equal(t, 1_000_000.0, q3)

	q4, err := repo.GetAffiliateQuarterNetRevenue(ctx, owner.affiliatorID, q3End, q4End)
	require.NoError(t, err)
	require.Equal(t, 2_000_000.0, q4)
}

func TestReconcileMissingCommissions_RecreatesWhatTheWorkerDropped(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-30 * 24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-recon-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-recon-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")
	_, err := repo.Pool().Exec(ctx, `UPDATE affiliate_bindings SET started_at = now() - interval '5 days' WHERE student_id = $1`, student.ID)
	require.NoError(t, err)

	dropped := seedCommissionOrder(t, repo, student.ID, time.Now().UTC().Add(-48*time.Hour), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000})
	tooFresh := seedCommissionOrder(t, repo, student.ID, time.Now().UTC().Add(-10*time.Minute), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000})
	tooOld := seedCommissionOrder(t, repo, student.ID, time.Now().UTC().Add(-20*24*time.Hour), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000})

	res, err := svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.OrdersReconciled, 1)

	require.Len(t, affiliateCommissionsForOrder(t, repo, dropped.ID), 1)
	require.Empty(t, affiliateCommissionsForOrder(t, repo, tooFresh.ID), "an order the worker may still be handling is left alone")
	require.Empty(t, affiliateCommissionsForOrder(t, repo, tooOld.ID), "orders outside the window are left alone")

	_, err = svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)
	require.Len(t, affiliateCommissionsForOrder(t, repo, dropped.ID), 1, "a rerun must not duplicate")
}

func TestReconcileMissingCommissions_SkipsInactiveAffiliatorAndFlagOff(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-30 * 24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")

	suspended := seedAffiliatorOwner(t, repo, "aff-recon-sus-"+uniqueSuffix()+"@test.local", "", "suspended")
	student := seedAffiliateTestStudent(t, repo, "student-recon-sus-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, suspended.affiliatorID, "checkout_code")
	_, err := repo.Pool().Exec(ctx, `UPDATE affiliate_bindings SET started_at = now() - interval '5 days' WHERE student_id = $1`, student.ID)
	require.NoError(t, err)
	order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC().Add(-48*time.Hour), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000})

	_, err = svc.RunAffiliateJob(ctx, "commissions", nil)
	require.NoError(t, err)
	require.Empty(t, affiliateCommissionsForOrder(t, repo, order.ID))
}
