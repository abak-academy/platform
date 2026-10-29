package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestAffiliateAdminDetailAggregatesSummaries seeds one affiliator with every
// detail ingredient and asserts the DTO numbers plus the BR-PRV-02 payload:
// students appear as first name + initials, without email or phone.
func TestAffiliateAdminDetailAggregatesSummaries(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	id, code := seedAffiliateAdminAffiliator(t, repo, "Budi Santoso "+suffix, "budi-detail-"+suffix+"@test.local", "active", "gold")

	studentEmail := "student-detail-" + suffix + "@test.local"
	studentPhone := "0812" + suffix
	var studentID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, email, phone, role, status) VALUES ($1, $2, $3, 'student', 'active') RETURNING id`,
		"Siti Aminah Rahayu", studentEmail, studentPhone,
	).Scan(&studentID))
	seedAffiliateJobBinding(t, repo, id, studentID, nil)
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, status, source, started_at, ended_at)
		 VALUES ($1, $2, 'ended', 'link', now() - interval '90 days', now() - interval '10 days')`,
		id, studentID)
	require.NoError(t, err)

	now := time.Now().UTC()
	seedAffiliateJobCommission(t, repo, id, studentID, affiliateJobCommissionSeed{
		productType: "exam", base: 100000, pct: 10, amount: 10000, status: "pending", level: "gold",
		createdAt: now.AddDate(0, 0, -2), paidAt: now.AddDate(0, 0, -2),
	})
	seedAffiliateJobCommission(t, repo, id, studentID, affiliateJobCommissionSeed{
		productType: "course", base: 50000, pct: 10, amount: 5000, status: "available", level: "gold",
		createdAt: now.AddDate(0, 0, -3), paidAt: now.AddDate(0, 0, -3),
	})
	seedAffiliateJobCommission(t, repo, id, studentID, affiliateJobCommissionSeed{
		productType: "book", base: 70000, pct: 10, amount: 7000, status: "available", level: "gold",
		createdAt: now.AddDate(0, 0, -4), paidAt: now.AddDate(0, 0, -4),
	})
	seedAffiliateJobCommission(t, repo, id, studentID, affiliateJobCommissionSeed{
		productType: "merchandise", base: 30000, pct: 10, amount: 3000, status: "cancelled", level: "gold",
		createdAt: now.AddDate(0, 0, -5), paidAt: now.AddDate(0, 0, -5),
	})

	_, err = repo.Pool().Exec(ctx,
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, note)
		 VALUES ($1, 'commission', 25000, 'seed detail'), ($1, 'payout', -5000, 'seed detail')`,
		id)
	require.NoError(t, err)

	detail, err := svc.GetAffiliateAdminDetail(ctx, id.String())
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.NotNil(t, detail.Affiliate)
	require.Equal(t, id, detail.Affiliate.ID)
	require.Equal(t, code, detail.Affiliate.Code)
	require.Equal(t, "active", detail.Affiliate.Status)
	require.Equal(t, "Budi Santoso "+suffix, detail.Name)
	require.NotNil(t, detail.Email)
	require.Equal(t, "budi-detail-"+suffix+"@test.local", *detail.Email)

	require.Equal(t, 1, detail.Bindings.Active)
	require.Equal(t, 1, detail.Bindings.Ended)
	require.Len(t, detail.Bindings.Data, 2, "both bindings fit inside the last-10 cap")
	require.Equal(t, "Siti A. R.", detail.Bindings.Data[0].Name)
	require.Equal(t, "Siti A. R.", detail.Bindings.Data[1].Name)

	raw, err := json.Marshal(detail)
	require.NoError(t, err)
	payload := string(raw)
	require.NotContains(t, payload, studentEmail, "bindings must not carry the student email (BR-PRV-02)")
	require.NotContains(t, payload, studentPhone, "bindings must not carry the student phone (BR-PRV-02)")
	require.NotContains(t, payload, "Aminah", "student last words stay initialled")

	require.Equal(t, map[string]int{"pending": 1, "available": 2, "cancelled": 1}, detail.Commissions.Counts)
	require.InDelta(t, 25000.0, detail.Commissions.TotalAmount, 0.001)
	require.InDelta(t, 20000.0, detail.Saldo, 0.001)
}

func TestAffiliateAdminDetail_unknownOrMalformedID(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	_, err := svc.GetAffiliateAdminDetail(ctx, uuid.NewString())
	require.ErrorIs(t, err, ErrAffiliateNotFound)

	_, err = svc.GetAffiliateAdminDetail(ctx, "not-a-uuid")
	require.ErrorIs(t, err, ErrInvalidUUID)
}

// TestAffiliateAdminDetail_emptyListsAreArrays keeps the detail body
// frontend-friendly when the affiliator has no history yet.
func TestAffiliateAdminDetail_emptyListsAreArrays(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-empty-"+uniqueSuffix()+"@test.local", "active", "bronze")
	detail, err := svc.GetAffiliateAdminDetail(ctx, id.String())
	require.NoError(t, err)

	raw, err := json.Marshal(detail)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "level_history")
	require.Contains(t, string(raw), `"data":[]`)
}
