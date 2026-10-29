package service

import (
	"context"
	"time"

	"testing"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// seedAffiliateTxPayout inserts a payout transaction. amount is the positive
// rupiah requested; the stored row is the negative debit that took effect at.
func seedAffiliateTxPayout(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, status string, amount float64, holder string, at time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, created_at, detail)
		 VALUES ($1, 'payout', $2, -$3::numeric, $4, $4,
		     jsonb_build_object('bank_account', jsonb_build_object(
		         'bank_name', 'BCA', 'account_number', '1234567890', 'account_holder', $5::text)))
		 RETURNING id`,
		affiliatorID, status, amount, at, holder,
	).Scan(&id))
	return id
}

// seedAffiliateTxAdjustment inserts a posted adjustment that took effect at.
func seedAffiliateTxAdjustment(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, category string, amount float64, reason string, actorID *uuid.UUID, at time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliate_transactions (affiliator_id, type, status, amount, effective_at, created_at, actor_id, note, detail)
		 VALUES ($1, 'adjustment', 'posted', $2, $3, $3, $4, $5, jsonb_build_object('category', $6::text))
		 RETURNING id`,
		affiliatorID, amount, at, actorID, reason, category,
	).Scan(&id))
	return id
}

// seedAffiliateTxBalance gives the affiliator a balance of amount through a
// posted adjustment, for tests that only need money to spend.
func seedAffiliateTxBalance(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, amount float64) {
	t.Helper()
	seedAffiliateTxAdjustment(t, repo, affiliatorID, "other", amount, "seed balance", nil, time.Now().UTC())
}
