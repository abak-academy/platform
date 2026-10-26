package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAffiliateMeLists_SearchFilterAndPaginate(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Filters "+uniqueSuffix(), "active", "bronze")

	unique := uniqueSuffix()
	sariName := "Sari" + unique + " Wulandari"
	dimasName := "Dimas" + unique + " Pratama"
	sari := uuid.MustParse(seedAffiliateMeStudent(t, repo, sariName))
	dimas := uuid.MustParse(seedAffiliateMeStudent(t, repo, dimasName))
	seedAffiliateMeBindingSource(t, repo, aff.ID, sari, "active", "link")
	seedAffiliateMeBindingSource(t, repo, aff.ID, dimas, "ended", "checkout_code")

	now := time.Now().UTC()
	commission := func(student uuid.UUID, productType, status string, paidDaysAgo int) uuid.UUID {
		return seedAffiliateJobCommission(t, repo, aff.ID, student, affiliateJobCommissionSeed{
			productType: productType, base: 100000, pct: 10, amount: 10000, status: status, level: "bronze",
			createdAt: now, paidAt: now.AddDate(0, 0, -paidDaysAgo),
		}).ID
	}
	cSariCourse := commission(sari, "course", "available", 2)
	commission(sari, "book", "pending", 20)
	commission(dimas, "exam", "cancelled", 40)
	seedCommissionLedgerEntry(t, repo, aff.ID, cSariCourse, 10000)
	// An order item carries the product's name at purchase time.
	_, err := repo.Pool().Exec(ctx,
		`UPDATE order_item SET name = 'Kursus Matematika '||$2::text
		 WHERE id = (SELECT order_item_id FROM affiliate_commissions WHERE id = $1)`, cSariCourse, unique)
	require.NoError(t, err)
	var adjustmentID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO affiliate_adjustments (affiliator_id, category, amount, reason, created_by)
		 VALUES ($1, 'bonus', 5000, 'Bonus akhir tahun '||$2::text, $3) RETURNING id`,
		aff.ID, unique, uuid.MustParse(seedAffiliateMeUser(t, repo, "Admin Ledger "+uniqueSuffix(), "super_admin"))).Scan(&adjustmentID))
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, adjustment_id, note, created_at) VALUES
		 ($1, 'adjustment', 5000, $3, 'Bonus akhir tahun '||$2::text, now() - interval '1 day')`,
		aff.ID, unique, adjustmentID)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, note, created_at) VALUES
		 ($1, 'payout', -3000, 'payout requested order '||$2::text, now() - interval '3 days')`,
		aff.ID, uuid.NewString())
	require.NoError(t, err)

	page := func(f AffiliateMeListFilter) AffiliateMeCommissionsPage {
		t.Helper()
		f.Page, f.PerPage = 1, 20
		out, err := svc.ListMyAffiliateCommissions(ctx, userID, f)
		require.NoError(t, err)
		return out
	}

	t.Run("commission search matches the student name, case-insensitively, and shows initials", func(t *testing.T) {
		got := page(AffiliateMeListFilter{Query: "wulan"})
		require.Equal(t, 2, got.Total)
		require.Len(t, got.Data, 2)
		require.Equal(t, "Sari"+unique+" W.", got.Data[0].StudentName)
		require.NotEmpty(t, got.Data[0].ProductName)
	})
	t.Run("commission search matches the product name", func(t *testing.T) {
		got := page(AffiliateMeListFilter{Query: "kursus matematika " + unique})
		require.Equal(t, 1, got.Total)
	})
	t.Run("filters combine", func(t *testing.T) {
		require.Equal(t, 1, page(AffiliateMeListFilter{Query: unique, Status: "pending"}).Total)
		require.Equal(t, 1, page(AffiliateMeListFilter{Query: unique, ProductType: "course"}).Total)
	})
	t.Run("the date range limits by payment date", func(t *testing.T) {
		from := now.AddDate(0, 0, -5)
		require.Equal(t, 1, page(AffiliateMeListFilter{Query: unique, From: &from}).Total)
		to := now.AddDate(0, 0, -30)
		require.Equal(t, 1, page(AffiliateMeListFilter{Query: unique, To: &to}).Total)
	})
	t.Run("percent and underscore search literally", func(t *testing.T) {
		require.Equal(t, 0, page(AffiliateMeListFilter{Query: "%"}).Total)
		require.Equal(t, 0, page(AffiliateMeListFilter{Query: "_"}).Total)
	})
	t.Run("total counts every match while a page holds only its slice", func(t *testing.T) {
		got, err := svc.ListMyAffiliateCommissions(ctx, userID, AffiliateMeListFilter{Page: 2, PerPage: 2})
		require.NoError(t, err)
		require.Equal(t, 3, got.Total)
		require.Len(t, got.Data, 1)
	})

	t.Run("bindings filter by name, status and source", func(t *testing.T) {
		list := func(f AffiliateMeListFilter) AffiliateMeBindingsPage {
			f.Page, f.PerPage = 1, 20
			out, err := svc.ListMyAffiliateBindings(ctx, userID, f)
			require.NoError(t, err)
			return out
		}
		require.Equal(t, 1, list(AffiliateMeListFilter{Query: "dimas" + unique}).Total)
		require.Equal(t, 1, list(AffiliateMeListFilter{Status: "ended"}).Total)
		require.Equal(t, 1, list(AffiliateMeListFilter{Type: "link"}).Total)
		require.Equal(t, 0, list(AffiliateMeListFilter{Status: "ended", Type: "link"}).Total)
		_, err := svc.ListMyAffiliateBindings(ctx, userID, AffiliateMeListFilter{Page: 1, PerPage: 20, Type: "bogus"})
		require.ErrorIs(t, err, ErrInvalidStatusFilter)
	})

	t.Run("ledger: type filter, search, and no internal notes", func(t *testing.T) {
		ledger := func(f AffiliateMeListFilter) AffiliateMeLedgerPage {
			f.Page, f.PerPage = 1, 20
			out, err := svc.GetMyAffiliateLedger(ctx, userID, f)
			require.NoError(t, err)
			return out
		}
		all := ledger(AffiliateMeListFilter{})
		require.Equal(t, 3, all.Total)
		require.InDelta(t, 12000, all.Saldo, 0.001, "the saldo is the whole ledger, not the filtered page")

		bySari := ledger(AffiliateMeListFilter{Query: "sari" + unique})
		require.Equal(t, 1, bySari.Total)
		require.Equal(t, "commission", bySari.Data[0].EntryType)
		require.NotNil(t, bySari.Data[0].StudentName)
		require.Equal(t, "Sari"+unique+" W.", *bySari.Data[0].StudentName)
		require.NotNil(t, bySari.Data[0].ProductName)
		require.Nil(t, bySari.Data[0].Note, "a commission entry's note holds internal text and must not reach the affiliator")

		byBonus := ledger(AffiliateMeListFilter{Query: "bonus akhir tahun"})
		require.Equal(t, 1, byBonus.Total)
		require.NotNil(t, byBonus.Data[0].Note)
		require.Contains(t, *byBonus.Data[0].Note, "Bonus akhir tahun")

		payouts := ledger(AffiliateMeListFilter{Type: "payout"})
		require.Equal(t, 1, payouts.Total)
		require.Nil(t, payouts.Data[0].Note, "the payout note carries an order id and must be hidden")

		from := now.AddDate(0, 0, -2)
		require.Equal(t, 2, ledger(AffiliateMeListFilter{From: &from}).Total)

		_, err := svc.GetMyAffiliateLedger(ctx, userID, AffiliateMeListFilter{Page: 1, PerPage: 20, Type: "bogus"})
		require.ErrorIs(t, err, ErrInvalidStatusFilter)
	})
}

func TestAffiliateMeLists_PayoutsAdjustmentsAndViolationsFilters(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Filters B "+uniqueSuffix(), "active", "bronze")
	deciderID := uuid.MustParse(seedAffiliateMeUser(t, repo, "Admin Filters "+uniqueSuffix(), "super_admin"))
	now := time.Now().UTC()

	seedAffiliateMePayoutRow(t, repo, aff.ID, "paid", 100000, now.AddDate(0, 0, -30))
	seedAffiliateMePayoutRow(t, repo, aff.ID, "rejected", 100000, now.AddDate(0, 0, -5))
	seedAffiliateMePayoutRow(t, repo, aff.ID, "requested", 100000, now.AddDate(0, 0, -1))
	seedAffiliateMeViolation(t, repo, aff.ID, deciderID, "open", "none", now.AddDate(0, 0, -2))
	seedAffiliateMeViolation(t, repo, aff.ID, deciderID, "resolved", "upheld", now.AddDate(0, 0, -20))
	unique := uniqueSuffix()
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO affiliate_adjustments (affiliator_id, category, amount, reason, created_by, created_at) VALUES
		 ($1, 'bonus', 50000, 'Bonus '||$3::text, $2, now() - interval '2 days'),
		 ($1, 'correction', -20000, 'Koreksi '||$3::text, $2, now() - interval '20 days')`,
		aff.ID, deciderID, unique)
	require.NoError(t, err)

	base := AffiliateMeListFilter{Page: 1, PerPage: 20}
	with := func(f func(*AffiliateMeListFilter)) AffiliateMeListFilter { out := base; f(&out); return out }

	payouts, err := svc.ListMyAffiliatePayouts(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Status = "paid" }))
	require.NoError(t, err)
	require.Equal(t, 1, payouts.Total)
	from := now.AddDate(0, 0, -7)
	payouts, err = svc.ListMyAffiliatePayouts(ctx, userID, with(func(f *AffiliateMeListFilter) { f.From = &from }))
	require.NoError(t, err)
	require.Equal(t, 2, payouts.Total)
	_, err = svc.ListMyAffiliatePayouts(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Status = "bogus" }))
	require.ErrorIs(t, err, ErrInvalidStatusFilter)

	adj, err := svc.ListMyAffiliateAdjustments(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Type = "bonus" }))
	require.NoError(t, err)
	require.Equal(t, 1, adj.Total)
	adj, err = svc.ListMyAffiliateAdjustments(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Query = "koreksi " + unique }))
	require.NoError(t, err)
	require.Equal(t, 1, adj.Total)
	_, err = svc.ListMyAffiliateAdjustments(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Type = "bogus" }))
	require.ErrorIs(t, err, ErrInvalidStatusFilter)

	viol, err := svc.ListMyAffiliateViolations(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Status = "open" }))
	require.NoError(t, err)
	require.Equal(t, 1, viol.Total)
	viol, err = svc.ListMyAffiliateViolations(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Status = "resolved" }))
	require.NoError(t, err)
	require.Equal(t, 1, viol.Total)
	_, err = svc.ListMyAffiliateViolations(ctx, userID, with(func(f *AffiliateMeListFilter) { f.Type = "bogus" }))
	require.ErrorIs(t, err, ErrInvalidStatusFilter)
}
