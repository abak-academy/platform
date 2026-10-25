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
		`INSERT INTO affiliators (user_id, code, status, level, terms_version) VALUES ($1, $2, $3, $4, 'v1') RETURNING id`,
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
		`INSERT INTO affiliators (user_id, code, status, terms_version) VALUES ($1, $2, 'active', 'v1') RETURNING id`,
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
	days := 7

	t.Run("suspend sets suspended_until from WIB now plus clamped days", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-suspend-"+uniqueSuffix()+"@test.local", "active", "bronze")
		long := 1000
		aff, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "suspended", "tos pelanggaran", &long)
		require.NoError(t, err)
		require.Equal(t, "suspended", aff.Status)
		want := time.Now().In(affiliateWIBLocation()).AddDate(0, 0, 365)
		require.WithinDuration(t, want, *aff.SuspendedUntil, 2*time.Minute)

		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.status_change"))
		meta := affiliateAdminAuditMeta(t, repo, id.String(), "affiliate.status_change")
		require.Equal(t, "active", meta["from"])
		require.Equal(t, "suspended", meta["to"])
		require.Equal(t, float64(365), meta["days"])
	})

	t.Run("a one-day clamp keeps the suspension", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-suspend1-"+uniqueSuffix()+"@test.local", "active", "bronze")
		zero := 0
		aff, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "suspended", "ringan", &zero)
		require.NoError(t, err)
		want := time.Now().In(affiliateWIBLocation()).AddDate(0, 0, 1)
		require.WithinDuration(t, want, *aff.SuspendedUntil, 2*time.Minute)
	})

	t.Run("suspend without days is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-suspendn-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "suspended", "tanpa hari", nil)
		require.ErrorIs(t, err, ErrAffiliateInvalidDays)
	})

	t.Run("reason is mandatory", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-noreason-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "suspended", "   ", &days)
		require.ErrorIs(t, err, ErrAffiliateReasonRequired)
	})

	t.Run("an unknown status value is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-bogusstat-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "bogus", "alasan", nil)
		require.ErrorIs(t, err, ErrAffiliateInvalidStatus)
	})

	t.Run("deactivate sets the status and leaves pending commissions untouched (BR-PLG-07 deferred)", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-deact-"+uniqueSuffix()+"@test.local", "active", "bronze")
		student := seedAffiliateTestStudent(t, repo, "student-deact-"+uniqueSuffix()+"@test.local", "")
		commission := seedAffiliateJobCommission(t, repo, id, uuid.MustParse(student.ID), affiliateJobCommissionSeed{
			productType: "exam", base: 100000, pct: 10, amount: 10000, status: "pending", level: "bronze",
			createdAt: time.Now(), paidAt: time.Now(),
		})

		aff, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "deactivated", "penipuan", nil)
		require.NoError(t, err)
		require.Equal(t, "deactivated", aff.Status)

		var status string
		require.NoError(t, repo.Pool().QueryRow(context.Background(),
			`SELECT status FROM affiliate_commissions WHERE id = $1`, commission.ID).Scan(&status))
		require.Equal(t, "pending", status, "deactivation must not auto-cancel pending commissions")

		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.status_change"))
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		_, err := svc.ChangeAffiliateStatus(ctx, actor, uuid.NewString(), "active", "alasan", nil)
		require.ErrorIs(t, err, ErrAffiliateNotFound)
	})
}

func TestAffiliateAdminFreeze(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := uuid.NewString()

	seedAffiliateAdminFreezeSettings(t, repo, 5)

	id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-freeze-"+uniqueSuffix()+"@test.local", "active", "bronze")

	t.Run("freeze caps days at freeze_max_days", func(t *testing.T) {
		aff, err := svc.FreezeAffiliateBalance(ctx, actor, id.String(), 30, "fraud diteliti")
		require.NoError(t, err)
		want := time.Now().In(affiliateWIBLocation()).AddDate(0, 0, 5)
		require.WithinDuration(t, want, *aff.FrozenUntil, 2*time.Minute)

		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.freeze"))
		meta := affiliateAdminAuditMeta(t, repo, id.String(), "affiliate.freeze")
		require.Equal(t, float64(5), meta["days"])
	})

	t.Run("freeze needs a reason", func(t *testing.T) {
		_, err := svc.FreezeAffiliateBalance(ctx, actor, id.String(), 3, " ")
		require.ErrorIs(t, err, ErrAffiliateReasonRequired)
	})

	t.Run("freeze with non-positive days is rejected", func(t *testing.T) {
		_, err := svc.FreezeAffiliateBalance(ctx, actor, id.String(), 0, "alasan")
		require.ErrorIs(t, err, ErrAffiliateInvalidDays)
	})

	t.Run("unfreeze clears frozen_until", func(t *testing.T) {
		aff, err := svc.UnfreezeAffiliateBalance(ctx, actor, id.String(), "banding diterima")
		require.NoError(t, err)
		require.Nil(t, aff.FrozenUntil)

		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.unfreeze"))
	})

	t.Run("unfreeze needs a reason", func(t *testing.T) {
		_, err := svc.UnfreezeAffiliateBalance(ctx, actor, id.String(), "")
		require.ErrorIs(t, err, ErrAffiliateReasonRequired)
	})

	t.Run("freeze on an unknown id is not found", func(t *testing.T) {
		_, err := svc.FreezeAffiliateBalance(ctx, actor, uuid.NewString(), 3, "alasan")
		require.ErrorIs(t, err, ErrAffiliateNotFound)
	})
}

func TestAffiliateAdminLevel(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	// affiliate_level_history.created_by is FK'd to users, so the actor must
	// be a seeded user row, unlike the audit-log-only actions.
	actor := insertAdminActor(t, repo, "affadm")
	reason := "penyesuaian manual"

	t.Run("manual change writes history with from/to and resets warn_quarters", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvl-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := repo.Pool().Exec(context.Background(), `UPDATE affiliators SET warn_quarters = 2 WHERE id = $1`, id)
		require.NoError(t, err)

		aff, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "gold", "", reason)
		require.NoError(t, err)
		require.Equal(t, "gold", aff.Level)
		require.Equal(t, 0, aff.WarnQuarters)

		rows := affiliateJobHistoryRows(t, repo, id)
		require.Len(t, rows, 1)
		require.NotNil(t, rows[0].FromLevel)
		require.Equal(t, "bronze", *rows[0].FromLevel)
		require.Equal(t, "gold", rows[0].ToLevel)
		require.Equal(t, "manual", rows[0].Source)
		require.Equal(t, reason, *rows[0].Reason)
		require.Equal(t, affiliateQuarterKey(time.Now().In(affiliateWIBLocation())), rows[0].Quarter)
		require.NotNil(t, rows[0].CreatedBy)
		require.Equal(t, actor, rows[0].CreatedBy.String())

		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.level_change"))
		meta := affiliateAdminAuditMeta(t, repo, id.String(), "affiliate.level_change")
		require.Equal(t, "bronze", meta["from"])
		require.Equal(t, "gold", meta["to"])
	})

	t.Run("a pure lock sets level_locked_until and writes no history", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lock-"+uniqueSuffix()+"@test.local", "active", "silver")

		aff, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "", "2027-01-01", "kunci evaluasi")
		require.NoError(t, err)
		require.Equal(t, "silver", aff.Level)
		require.True(t, aff.LevelLockedUntil.Equal(time.Date(2027, 1, 2, 0, 0, 0, 0, affiliateWIBLocation())),
			"lock_until = %v, want end of 2027-01-01 WIB", aff.LevelLockedUntil)
		require.Empty(t, affiliateJobHistoryRows(t, repo, id), "a pure lock writes no history row")
		require.Equal(t, 1, countAffiliateAdminAudit(t, repo, id.String(), "affiliate.level_change"))
	})

	t.Run("an empty change is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvle-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "", "", reason)
		require.ErrorIs(t, err, ErrMissingField)
	})

	t.Run("an unknown level is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvlx-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "platinum", "", reason)
		require.ErrorIs(t, err, ErrAffiliateInvalidLevel)
	})

	t.Run("a lock on a malformed date is rejected", func(t *testing.T) {
		id, _ := seedAffiliateAdminAffiliator(t, repo, "Affiliate Seed", "aff-lvld-"+uniqueSuffix()+"@test.local", "active", "bronze")
		_, err := svc.ChangeAffiliateLevel(ctx, actor, id.String(), "", "01-2027", reason)
		require.ErrorIs(t, err, ErrInvalidDateFormat)
	})

	t.Run("a level change on an unknown id is not found", func(t *testing.T) {
		_, err := svc.ChangeAffiliateLevel(ctx, actor, uuid.NewString(), "gold", "", reason)
		require.ErrorIs(t, err, ErrAffiliateNotFound)
	})
}

func TestAffiliateAdminSentinels_areDistinct(t *testing.T) {
	errs := []error{
		ErrAffiliateNotPending, ErrAffiliateInvalidDecision, ErrAffiliateRejectReasonRequired,
		ErrAffiliateInvalidStatus, ErrAffiliateReasonRequired, ErrAffiliateInvalidDays,
		ErrAffiliateInvalidLevel, ErrAffiliateCodeGenerationExhausted,
	}
	seen := map[string]bool{}
	for _, err := range errs {
		require.False(t, seen[err.Error()], "sentinel message duplicated: %s", err.Error())
		seen[err.Error()] = true
	}
}
