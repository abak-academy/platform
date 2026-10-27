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
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM affiliate_flags WHERE affiliator_id = $1`, owner.affiliatorID) })

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
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM affiliate_violations WHERE affiliator_id = $1`, owner.affiliatorID) })
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
