package service

import (
	"context"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func parseAdminCSV(t *testing.T, data []byte) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))).ReadAll()
	require.NoError(t, err)
	return records
}

func TestAdminAffiliateFlags_filtersAndCSV(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()
	owner := seedAffiliatorOwner(t, repo, "aff-flaglist-"+uniqueSuffix()+"@test.local", "", "active")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_flags WHERE affiliator_id = $1`, owner.affiliatorID)
	})

	for _, status := range []string{"open", "valid", "ignored"} {
		_, err := pool.Exec(ctx,
			`INSERT INTO affiliate_flags (affiliator_id, rule, payload, status) VALUES ($1, 'self_purchase', '{"note":"=cmd"}', $2)`,
			owner.affiliatorID, status)
		require.NoError(t, err)
	}

	all, total, err := svc.AdminListAffiliateFlags(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "all", Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, all, 3)

	valid, total, err := svc.AdminListAffiliateFlags(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "valid", Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "valid", valid[0].Status)

	future := time.Now().Add(24 * time.Hour)
	_, total, err = svc.AdminListAffiliateFlags(ctx, AffiliateAdminQueueFilter{Q: owner.code, From: &future, Limit: 20})
	require.NoError(t, err)
	require.Zero(t, total)

	paged, total, err := svc.AdminListAffiliateFlags(ctx, AffiliateAdminQueueFilter{Q: owner.code, Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, paged, 1)

	data, err := svc.AdminAffiliateFlagsCSV(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "open"})
	require.NoError(t, err)
	records := parseAdminCSV(t, data)
	require.Len(t, records, 2)
	require.Equal(t, "affiliator_name", records[0][0])
	require.Equal(t, owner.code, records[1][1])
	require.Equal(t, "open", records[1][3])
	require.True(t, strings.HasPrefix(records[1][4], "'") || !strings.HasPrefix(records[1][4], "="), "detail must be formula-safe: %q", records[1][4])
}

func TestAdminAffiliateAppeals_filtersAndCSV(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()
	owner := seedAffiliatorOwner(t, repo, "aff-appeallist-"+uniqueSuffix()+"@test.local", "", "active")
	actor := seedAffiliateAdminActor(t, repo, "aff-appealadm-"+uniqueSuffix()+"@test.local")
	actorUUID := uuid.MustParse(actor)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliate_violations WHERE affiliator_id = $1`, owner.affiliatorID)
	})
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, owner.userID, actor) })

	seedAffiliateMeViolation(t, repo, owner.affiliatorID, actorUUID, "open", "pending", time.Now())
	seedAffiliateMeViolation(t, repo, owner.affiliatorID, actorUUID, "resolved", "rejected", time.Now())
	seedAffiliateMeViolation(t, repo, owner.affiliatorID, actorUUID, "open", "none", time.Now())

	all, total, err := svc.AdminListAffiliateAppeals(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "all", Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 2, total, "a violation without an appeal is not an appeal")
	require.Len(t, all, 2)

	pending, total, err := svc.AdminListAffiliateAppeals(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "pending", Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "pending", pending[0].AppealStatus)

	_, _, err = svc.AdminListAffiliateAppeals(ctx, AffiliateAdminQueueFilter{Status: "none", Limit: 20})
	require.ErrorIs(t, err, ErrValidation)

	data, err := svc.AdminAffiliateAppealsCSV(ctx, AffiliateAdminQueueFilter{Q: owner.code, Status: "all"})
	require.NoError(t, err)
	require.Len(t, parseAdminCSV(t, data), 3)
}

func TestAdminAffiliateCommissions_fullNamesFiltersAndCSV(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	_, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator AdminComm "+uniqueSuffix(), "active", "bronze")

	unique := uniqueSuffix()
	sariName := "Sari" + unique + " Wulandari"
	sari := uuid.MustParse(seedAffiliateMeStudent(t, repo, sariName))
	dimas := uuid.MustParse(seedAffiliateMeStudent(t, repo, "Dimas"+unique+" Pratama"))
	seedAffiliateMeBindingSource(t, repo, aff.ID, sari, "active", "link")
	seedAffiliateMeBindingSource(t, repo, aff.ID, dimas, "active", "link")

	now := time.Now().UTC()
	commission := func(student uuid.UUID, productType, status string) {
		seedAffiliateJobCommission(t, repo, aff.ID, student, affiliateJobCommissionSeed{
			productType: productType, base: 100000, pct: 10, amount: 10000, status: status, level: "bronze",
			createdAt: now, paidAt: now.AddDate(0, 0, -2),
		})
	}
	commission(sari, "course", "available")
	commission(dimas, "book", "pending")

	all, total, err := svc.AdminListAffiliateCommissions(ctx, aff.ID.String(), AffiliateAdminQueueFilter{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	names := []string{all[0].StudentName, all[1].StudentName}
	require.Contains(t, names, sariName, "an admin sees the full student name, not first name + initials")

	byStudent, total, err := svc.AdminListAffiliateCommissions(ctx, aff.ID.String(), AffiliateAdminQueueFilter{Q: "dimas" + unique, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "book", byStudent[0].ProductType)

	_, total, err = svc.AdminListAffiliateCommissions(ctx, aff.ID.String(), AffiliateAdminQueueFilter{Status: "available", Type: "course", Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)

	future := now.Add(24 * time.Hour)
	_, total, err = svc.AdminListAffiliateCommissions(ctx, aff.ID.String(), AffiliateAdminQueueFilter{From: &future, Limit: 20})
	require.NoError(t, err)
	require.Zero(t, total)

	page2, total, err := svc.AdminListAffiliateCommissions(ctx, aff.ID.String(), AffiliateAdminQueueFilter{Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, page2, 1)

	_, _, err = svc.AdminListAffiliateCommissions(ctx, uuid.NewString(), AffiliateAdminQueueFilter{Limit: 20})
	require.ErrorIs(t, err, ErrAffiliateNotFound)
	_, _, err = svc.AdminListAffiliateCommissions(ctx, "not-a-uuid", AffiliateAdminQueueFilter{Limit: 20})
	require.ErrorIs(t, err, ErrInvalidUUID)
	_, _, err = svc.AdminListAffiliateCommissions(ctx, aff.ID.String(), AffiliateAdminQueueFilter{Status: "bogus", Limit: 20})
	require.ErrorIs(t, err, ErrInvalidStatusFilter)
	_, _, err = svc.AdminListAffiliateCommissions(ctx, aff.ID.String(), AffiliateAdminQueueFilter{Type: "bogus", Limit: 20})
	require.ErrorIs(t, err, ErrValidation)

	data, err := svc.AdminAffiliateCommissionsCSV(ctx, aff.ID.String(), AffiliateAdminQueueFilter{Status: "all"})
	require.NoError(t, err)
	records := parseAdminCSV(t, data)
	require.Len(t, records, 3)
	require.Equal(t, "student_name", records[0][2])
}

func TestAdminAffiliateReportSummary_searchLevelAndPaging(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	_, a := seedAffiliateMeAffiliator(t, repo, "Report Alpha "+uniqueSuffix(), "active", "bronze")
	_, b := seedAffiliateMeAffiliator(t, repo, "Report Beta "+uniqueSuffix(), "active", "silver")
	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	byCode, err := svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{Q: strings.ToLower(a.Code)})
	require.NoError(t, err)
	require.Equal(t, 1, byCode.Total)
	require.Equal(t, a.ID, byCode.Data[0].AffiliatorID)

	silver, err := svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{Q: b.Code, Level: "silver"})
	require.NoError(t, err)
	require.Equal(t, 1, silver.Total)
	none, err := svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{Q: b.Code, Level: "gold"})
	require.NoError(t, err)
	require.Zero(t, none.Total)

	all, err := svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, all.Total, 2)
	paged, err := svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{Page: 1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, paged.Data, 1)
	require.Equal(t, all.Total, paged.Total, "the total counts every matching row, not the page")
	beyond, err := svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{Page: 9999, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, beyond.Data)

	_, err = svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{Level: "platinum"})
	require.ErrorIs(t, err, ErrValidation)
}

func TestAdminAffiliateReportOrders_acrossAffiliators(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	_, a := seedAffiliateMeAffiliator(t, repo, "Orders Alpha "+uniqueSuffix(), "active", "bronze")
	_, b := seedAffiliateMeAffiliator(t, repo, "Orders Beta "+uniqueSuffix(), "active", "bronze")

	unique := uniqueSuffix()
	sari := uuid.MustParse(seedAffiliateMeStudent(t, repo, "Sari"+unique+" Wulandari"))
	dimas := uuid.MustParse(seedAffiliateMeStudent(t, repo, "Dimas"+unique+" Pratama"))
	seedAffiliateMeBindingSource(t, repo, a.ID, sari, "active", "link")
	seedAffiliateMeBindingSource(t, repo, b.ID, dimas, "active", "link")
	now := time.Now().UTC()
	for _, c := range []struct {
		aff     uuid.UUID
		student uuid.UUID
		kind    string
		status  string
	}{{a.ID, sari, "course", "available"}, {b.ID, dimas, "book", "pending"}} {
		seedAffiliateJobCommission(t, repo, c.aff, c.student, affiliateJobCommissionSeed{
			productType: c.kind, base: 100000, pct: 10, amount: 10000, status: c.status, level: "bronze",
			createdAt: now, paidAt: now.AddDate(0, 0, -1),
		})
	}

	byCode, total, err := svc.AdminListAffiliateReportOrders(ctx, AffiliateAdminQueueFilter{Q: strings.ToLower(a.Code), Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, a.Code, byCode[0].AffiliatorCode)
	require.NotEmpty(t, byCode[0].AffiliatorName)
	require.Contains(t, byCode[0].StudentName, "Wulandari", "the full student name, not first name + initials")

	byStudent, total, err := svc.AdminListAffiliateReportOrders(ctx, AffiliateAdminQueueFilter{Q: "dimas" + unique, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, b.Code, byStudent[0].AffiliatorCode)

	_, total, err = svc.AdminListAffiliateReportOrders(ctx, AffiliateAdminQueueFilter{Q: unique, Type: "course", Status: "available", Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)

	_, _, err = svc.AdminListAffiliateReportOrders(ctx, AffiliateAdminQueueFilter{Status: "bogus", Limit: 20})
	require.ErrorIs(t, err, ErrInvalidStatusFilter)

	data, err := svc.AdminAffiliateReportOrdersCSV(ctx, AffiliateAdminQueueFilter{Q: unique})
	require.NoError(t, err)
	require.Len(t, parseAdminCSV(t, data), 3)
}
