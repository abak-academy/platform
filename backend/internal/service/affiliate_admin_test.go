package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedAffiliateAdminAffiliator(t *testing.T, repo *repository.Repository, name, email, status, level string) (uuid.UUID, string) {
	t.Helper()
	var userID string
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, email, username, role, status) VALUES ($1, $2, $3, 'affiliator', 'active') RETURNING id`,
		name, email, "affadm_"+uniqueSuffix(),
	).Scan(&userID))
	code := "AF" + strings.ToUpper(uniqueSuffix())
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliators (user_id, referral_code, program_status, level, terms_version) VALUES ($1, $2, $3, $4, 'v1') RETURNING user_id`,
		userID, code, status, level,
	).Scan(&id))
	return id, code
}

func countAffiliateAdminAudit(t *testing.T, repo *repository.Repository, targetID, action string) int {
	t.Helper()
	var n int
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE target_type = 'affiliator' AND target_id = $1 AND action = $2`,
		targetID, action,
	).Scan(&n))
	return n
}

func affiliateAdminAuditMeta(t *testing.T, repo *repository.Repository, targetID, action string) map[string]any {
	t.Helper()
	var raw []byte
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT metadata FROM audit_log WHERE target_type = 'affiliator' AND target_id = $1 AND action = $2 ORDER BY id DESC LIMIT 1`,
		targetID, action,
	).Scan(&raw))
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func seedAffiliateAdminFreezeSettings(t *testing.T, repo *repository.Repository, maxDays int) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_settings (id, freeze_max_days) VALUES (1, $1)
		 ON CONFLICT (id) DO UPDATE SET freeze_max_days = $1, updated_at = now()`,
		maxDays,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(),
			`INSERT INTO affiliate_settings (id, freeze_max_days) VALUES (1, 14)
			 ON CONFLICT (id) DO UPDATE SET freeze_max_days = 14, updated_at = now()`)
	})
}

func TestAffiliateAdminCodeGeneration_charset(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := generateAffiliateReferralCode()
		require.NoError(t, err)
		require.Len(t, code, 8)
		for _, r := range code {
			require.False(t, strings.ContainsRune("O0I1", r), "code %q carries an ambiguous glyph", code)
			require.True(t, strings.ContainsRune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789", r), "code %q carries %q outside the charset", code, r)
		}
	}
}

func TestAffiliateAdminCodeGeneration_uniqueAgainstExisting(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	seeded := "AF" + strings.ToUpper(uniqueSuffix())
	var userID string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, role, status) VALUES ($1, $2, 'affiliator', 'active') RETURNING id`,
		"Affiliate Seed", "affgen_"+uniqueSuffix(),
	).Scan(&userID))
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO affiliators (user_id, referral_code, program_status, terms_version) VALUES ($1, $2, 'active', 'v1') RETURNING user_id`,
		userID, seeded,
	).Scan(new(uuid.UUID)))

	code, err := svc.generateUniqueAffiliateCode(ctx)
	require.NoError(t, err)
	require.Len(t, code, 8)
	require.NotEqual(t, seeded, code)
	taken, err := repo.AffiliateCodeExists(ctx, code)
	require.NoError(t, err)
	require.False(t, taken)
}

func TestAffiliateAdminApprovalMatrix(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := uuid.NewString()

	tests := []struct {
		name        string
		status      string
		decision    string
		reason      string
		wantErr     error
		wantStatus  string
		wantCode    bool
		wantReason  string
		wantAuditNo int
	}{
		{"approve from pending activates and generates code", "pending_approval", "approve", "", nil, "active", true, "", 1},
		{"approve from active is an invalid transition", "active", "approve", "", ErrAffiliateNotPending, "active", false, "", 0},
		{"approve from suspended is an invalid transition", "suspended", "approve", "", ErrAffiliateNotPending, "suspended", false, "", 0},
		{"approve from deactivated is an invalid transition", "deactivated", "approve", "", ErrAffiliateNotPending, "deactivated", false, "", 0},
		{"approve from rejected is an invalid transition", "rejected", "approve", "", ErrAffiliateNotPending, "rejected", false, "", 0},
		{"reject from pending needs a reason", "pending_approval", "reject", "", ErrAffiliateRejectReasonRequired, "pending_approval", false, "", 0},
		{"reject from pending rejects with reason", "pending_approval", "reject", "dokumen tidak lengkap", nil, "rejected", false, "dokumen tidak lengkap", 1},
		{"reject from active is an invalid transition", "active", "reject", "alasan", ErrAffiliateNotPending, "active", false, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, code := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-approval-"+uniqueSuffix()+"@test.local", tt.status, "bronze")

			aff, err := svc.DecideAffiliateApproval(ctx, actor, id.String(), tt.decision, tt.reason)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, aff)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantStatus, aff.Status)
				if tt.wantCode {
					require.Len(t, aff.Code, 8)
					require.NotEqual(t, code, aff.Code, "approval must generate a fresh code once")
				} else {
					require.Equal(t, code, aff.Code, "code must stay untouched on reject")
				}
			}
			if tt.wantReason != "" {
				require.Equal(t, tt.wantReason, *aff.RejectionReason)
			}

			auditNo := countAffiliateAdminAudit(t, repo, id.String(), "affiliate.approval")
			require.Equal(t, tt.wantAuditNo, auditNo)
			if tt.wantAuditNo == 1 {
				meta := affiliateAdminAuditMeta(t, repo, id.String(), "affiliate.approval")
				require.Equal(t, tt.decision, meta["decision"])
			}
		})
	}

	t.Run("approve on an unknown id is not found", func(t *testing.T) {
		_, err := svc.DecideAffiliateApproval(ctx, actor, uuid.NewString(), "approve", "")
		require.ErrorIs(t, err, ErrAffiliateNotFound)
	})
	t.Run("an unknown decision is rejected", func(t *testing.T) {
		_, err := svc.DecideAffiliateApproval(ctx, actor, uuid.NewString(), "defer", "")
		require.ErrorIs(t, err, ErrAffiliateInvalidDecision)
	})
	t.Run("a malformed id is rejected", func(t *testing.T) {
		_, err := svc.DecideAffiliateApproval(ctx, actor, "not-a-uuid", "approve", "")
		require.ErrorIs(t, err, ErrInvalidUUID)
	})
}

// Two approvals racing on the same pending profile must hand out one code: the
// loser sees the profile already approved instead of overwriting the code that
// was just given to the affiliator.
func TestAffiliateAdminApproval_ConcurrentSubmitsKeepTheFirstCode(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedRevokeActor(t, repo).String()
	id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-race-"+uniqueSuffix()+"@test.local", "pending_approval", "bronze")

	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.DecideAffiliateApproval(ctx, actor, id.String(), "approve", "")
			results <- err
		}()
	}
	var failures, successes int
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			require.ErrorIs(t, err, ErrAffiliateNotPending)
			failures++
		} else {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, failures)
	require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.approval"))
}

func TestAffiliateAdminList(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()

	pending, _ := seedAffiliateAdminAffiliator(t, repo, "Budi Santoso "+suffix, "budi-"+suffix+"@test.local", "pending_approval", "bronze")
	active, _ := seedAffiliateAdminAffiliator(t, repo, "Siti Aminah "+suffix, "siti-"+suffix+"@test.local", "active", "silver")
	gold, goldCode := seedAffiliateAdminAffiliator(t, repo, "Andi Wijaya "+suffix, "andi-"+suffix+"@test.local", "deactivated", "gold")

	t.Run("status filter", func(t *testing.T) {
		list, err := svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Status: "active"})
		require.NoError(t, err)
		require.GreaterOrEqual(t, list.Total, 1)
		var found bool
		for _, row := range list.Data {
			require.Equal(t, "active", row.Status)
			if row.ID == active {
				found = true
			}
		}
		require.True(t, found, "the seeded active affiliator is listed")
	})

	t.Run("level filter", func(t *testing.T) {
		list, err := svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Level: "gold"})
		require.NoError(t, err)
		require.GreaterOrEqual(t, list.Total, 1)
		var found bool
		for _, row := range list.Data {
			if row.ID == pending {
				require.Fail(t, "pending affiliator must not match a gold filter")
			}
			if row.ID == gold {
				found = true
			}
		}
		require.True(t, found)
	})

	t.Run("q matches a name substring", func(t *testing.T) {
		list, err := svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Query: "Aminah " + suffix})
		require.NoError(t, err)
		require.Equal(t, 1, list.Total)
		require.Equal(t, active, list.Data[0].ID)
	})

	t.Run("q matches an email substring", func(t *testing.T) {
		list, err := svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Query: "budi-" + suffix + "@"})
		require.NoError(t, err)
		require.Equal(t, 1, list.Total)
		require.Equal(t, pending, list.Data[0].ID)
	})

	t.Run("q matches a code substring", func(t *testing.T) {
		list, err := svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Query: strings.ToLower(goldCode)})
		require.NoError(t, err)
		require.GreaterOrEqual(t, list.Total, 1)
		var found bool
		for _, row := range list.Data {
			if row.Code == goldCode {
				found = true
			}
		}
		require.True(t, found)
	})

	t.Run("defaults and per_page cap", func(t *testing.T) {
		list, err := svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{PerPage: 0})
		require.NoError(t, err)
		require.Equal(t, 20, list.PerPage, "a missing per_page defaults to 20")
		require.LessOrEqual(t, len(list.Data), 20)

		list, err = svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{PerPage: 100})
		require.NoError(t, err)
		require.Equal(t, 50, list.PerPage, "per_page is capped at 50")

		list, err = svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Page: 0})
		require.NoError(t, err)
		require.Equal(t, 1, list.Page, "page defaults to 1")
	})

	t.Run("invalid status and level filters are rejected", func(t *testing.T) {
		_, err := svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Status: "bogus"})
		require.ErrorIs(t, err, ErrInvalidStatusFilter)
		_, err = svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Level: "platinum"})
		require.ErrorIs(t, err, ErrAffiliateInvalidLevel)
	})
}

func TestAffiliateAdminStatusChange(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := uuid.NewString()

	t.Run("suspend has no end date and is recorded in the audit log", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-suspend-"+uniqueSuffix()+"@test.local", "active", "bronze")
		aff, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "suspended", "tos pelanggaran")
		require.NoError(t, err)
		require.Equal(t, "suspended", aff.Status)

		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.status_change"))
		meta := affiliateAdminAuditMeta(t, repo, id.String(), "affiliate.status_change")
		require.Equal(t, "active", meta["from"])
		require.Equal(t, "suspended", meta["to"])
		require.NotContains(t, meta, "days")
	})

	t.Run("reason is mandatory", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-noreason-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "suspended", "   ")
		require.ErrorIs(t, err, ErrAffiliateReasonRequired)
	})

	t.Run("an unknown status value is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-bogusstat-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "bogus", "alasan")
		require.ErrorIs(t, err, ErrAffiliateInvalidStatus)
	})

	t.Run("deactivate cancels pending commissions and leaves available saldo alone (BR-PLG-07)", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-deact-"+uniqueSuffix()+"@test.local", "active", "bronze")
		student := seedAffiliateTestStudent(t, repo, "student-deact-"+uniqueSuffix()+"@test.local", "")
		seed := func(status string) uuid.UUID {
			return seedAffiliateJobCommission(t, repo, id, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
				productType: "exam", base: 100000, pct: 10, amount: 10000, status: status, level: "bronze",
				createdAt: time.Now(), paidAt: time.Now(),
			}).ID
		}
		pending, available := seed("pending"), seed("available")

		aff, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "deactivated", "penipuan")
		require.NoError(t, err)
		require.Equal(t, "deactivated", aff.Status)

		require.Equal(t, "cancelled", commissionStatusByID(t, repo, pending))
		require.Equal(t, "available", commissionStatusByID(t, repo, available))
		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.status_change"))
		meta := affiliateAdminAuditMeta(t, repo, id.String(), "affiliate.status_change")
		require.EqualValues(t, 1, meta["commissions_cancelled"])
	})

	t.Run("a pending or rejected profile cannot be activated (it has no referral code)", func(t *testing.T) {
		for _, status := range []string{"pending_approval", "rejected"} {
			id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-badtrans-"+uniqueSuffix()+"@test.local", status, "bronze")
			_, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "active", "alasan")
			require.ErrorIs(t, err, ErrAffiliateInvalidTransition, status)
			var got string
			require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT program_status FROM affiliators WHERE user_id = $1`, id).Scan(&got))
			require.Equal(t, status, got)
		}
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		_, err := svc.ChangeAffiliateStatus(ctx, actor, uuid.NewString(), "active", "alasan")
		require.ErrorIs(t, err, ErrAffiliateNotFound)
	})
}

func TestAffiliateAdminLevel(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := uuid.NewString()
	reason := "penyesuaian manual"

	t.Run("manual change resets warn_quarters and is recorded in the audit log", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvl-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := repo.Pool().Exec(context.Background(), `UPDATE affiliators SET missed_quarters = 2 WHERE user_id = $1`, id)
		require.NoError(t, err)

		aff, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "gold", reason)
		require.NoError(t, err)
		require.Equal(t, "gold", aff.Level)
		require.Equal(t, 0, aff.WarnQuarters)

		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.level_change"))
		meta := affiliateAdminAuditMeta(t, repo, id.String(), "affiliate.level_change")
		require.Equal(t, "bronze", meta["from"])
		require.Equal(t, "gold", meta["to"])
		require.Equal(t, reason, meta["reason"])
		require.Equal(t, "", affiliateJobEvaluatedQuarter(t, repo, id), "a manual change does not settle a quarter")
	})

	t.Run("a missing level is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvle-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "", reason)
		require.ErrorIs(t, err, ErrMissingField)
	})

	t.Run("a reason is mandatory", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvlr-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "gold", "  ")
		require.ErrorIs(t, err, ErrAffiliateReasonRequired)
	})

	t.Run("an unknown level is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvlx-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "platinum", reason)
		require.ErrorIs(t, err, ErrAffiliateInvalidLevel)
	})

	t.Run("a level change on an unknown id is not found", func(t *testing.T) {
		_, err := svc.ChangeAffiliateLevel(ctx, actor, uuid.NewString(), "gold", reason)
		require.ErrorIs(t, err, ErrAffiliateNotFound)
	})
}

func TestAffiliateAdminSentinels_areDistinct(t *testing.T) {
	errs := []error{
		ErrAffiliateNotPending, ErrAffiliateInvalidDecision, ErrAffiliateRejectReasonRequired,
		ErrAffiliateInvalidStatus, ErrAffiliateReasonRequired,
		ErrAffiliateInvalidLevel, ErrAffiliateCodeGenerationExhausted, ErrAffiliateInvalidTransition,
	}
	seen := map[string]bool{}
	for _, err := range errs {
		require.False(t, seen[err.Error()], "sentinel message duplicated: %s", err.Error())
		seen[err.Error()] = true
	}
}
