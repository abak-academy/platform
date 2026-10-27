package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAffiliateStudentDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want string
	}{
		{"Budi Santoso", "Budi S."},
		{"Budi Santoso Putra", "Budi S. P."},
		{"Budi", "Budi"},
		{"", ""},
		{"  Budi   Santoso  ", "Budi S."},
		{"budi santoso", "budi S."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, affiliateStudentDisplayName(tt.name))
		})
	}
}

func setAffiliateMeFlag(t *testing.T, repo *repository.Repository, value string) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', $1, false)
		 ON CONFLICT (key) DO UPDATE SET value = $1`, value)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `DELETE FROM system_config WHERE key = 'affiliate_enabled'`)
	})
}

func seedAffiliateMeUser(t *testing.T, repo *repository.Repository, name, role string) string {
	t.Helper()
	var id string
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (name, username, role, status, otp_enabled)
		 VALUES ($1, $2, $3, 'active', false) RETURNING id`,
		name, role+"_"+uniqueSuffix(), role,
	).Scan(&id))
	return id
}

func seedAffiliateMeAffiliator(t *testing.T, repo *repository.Repository, name, status, level string) (string, model.Affiliator) {
	t.Helper()
	userID := seedAffiliateMeUser(t, repo, name, "affiliator")
	code := "AF" + strings.ToUpper(uniqueSuffix())
	var aff model.Affiliator
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliators (user_id, code, status, level, terms_version)
		 VALUES ($1, $2, $3, $4, 'v1') RETURNING id, user_id, code, status, level, warn_quarters`,
		userID, code, status, level,
	).Scan(&aff.ID, &aff.UserID, &aff.Code, &aff.Status, &aff.Level, &aff.WarnQuarters))
	return userID, aff
}

func seedAffiliateMeStudent(t *testing.T, repo *repository.Repository, name string) string {
	t.Helper()
	return seedAffiliateMeUser(t, repo, name, "student")
}

func seedAffiliateMeBinding(t *testing.T, repo *repository.Repository, affiliatorID, studentID uuid.UUID, status string) model.AffiliateBinding {
	t.Helper()
	return seedAffiliateMeBindingSource(t, repo, affiliatorID, studentID, status, "link")
}

func seedAffiliateMeBindingSource(t *testing.T, repo *repository.Repository, affiliatorID, studentID uuid.UUID, status, source string) model.AffiliateBinding {
	t.Helper()
	startedAt := time.Now().UTC().AddDate(0, 0, -10)
	endedAt := startedAt.AddDate(0, 3, 0)
	var b model.AffiliateBinding
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliate_bindings (affiliator_id, student_id, status, source, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, affiliator_id, student_id, status, source, started_at, ended_at, created_at`,
		affiliatorID, studentID, status, source, startedAt, endedAt,
	).Scan(&b.ID, &b.AffiliatorID, &b.StudentID, &b.Status, &b.Source, &b.StartedAt, &b.EndedAt, &b.CreatedAt))
	return b
}

func seedAffiliateMeOrder(t *testing.T, repo *repository.Repository, studentID string, status string, subtotal, discount float64, paidAt *time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, paid_at)
		 VALUES ($1, $2, $3, $4, 0, $5, $6) RETURNING id`,
		studentID, status, subtotal, discount, subtotal-discount, paidAt,
	).Scan(&id))
	return id
}

func seedAffiliateMeLedgerEntry(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, entryType string, amount float64, createdAt time.Time) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_ledger_entries (affiliator_id, entry_type, amount, created_at)
		 VALUES ($1, $2, $3, $4)`,
		affiliatorID, entryType, amount, createdAt)
	require.NoError(t, err)
}

func seedAffiliateMeClick(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID, code, ipHash string) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_clicks (affiliator_id, code, ip_hash, click_day)
		 VALUES ($1, $2, $3, date_trunc('day', now()))`,
		affiliatorID, code, ipHash)
	require.NoError(t, err)
}

func TestGetMyAffiliateProfile_showsLevelProgress(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")
	seedAffiliateJobLevelThresholds(t, repo, "silver", 200000, 50000)
	seedAffiliateJobLevelThresholds(t, repo, "gold", 300000, 150000)

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Me "+uniqueSuffix(), "active", "silver")
	studentID := seedAffiliateMeStudent(t, repo, "Budi Santoso")
	seedAffiliateMeBinding(t, repo, aff.ID, uuid.MustParse(studentID), "active")
	seedAffiliateJobCommission(t, repo, aff.ID, uuid.MustParse(studentID), affiliateJobCommissionSeed{
		productType: "course",
		base:        120000,
		pct:         10,
		amount:      12000,
		status:      "available",
		level:       "silver",
		createdAt:   time.Now().UTC(),
		paidAt:      time.Now().UTC(),
	})

	profile, err := svc.GetMyAffiliateProfile(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, aff.Code, profile.Code)
	require.Equal(t, "/r/"+aff.Code, profile.ReferralLink)
	require.Equal(t, "silver", profile.Level)
	require.Equal(t, "active", profile.Status)
	require.Equal(t, "v1", profile.TermsVersion)

	nowWIB := time.Now().In(affiliateWIBLocation())
	progress := profile.LevelProgress
	require.NotNil(t, progress)
	require.Equal(t, affiliateQuarterKey(nowWIB), progress.Quarter)
	require.InDelta(t, 120000, progress.NetRevenue, 0.001)
	require.Equal(t, "gold", progress.NextLevel)
	require.NotNil(t, progress.RiseThreshold)
	require.InDelta(t, 300000, *progress.RiseThreshold, 0.001)
	require.NotNil(t, progress.HoldThreshold)
	require.InDelta(t, 50000, *progress.HoldThreshold, 0.001)
}

func TestGetMyAffiliateProfile_pendingHasNoCodeOrLink(t *testing.T) {
	svc, repo := newRealDBService(t)
	setAffiliateMeFlag(t, repo, "true")

	userID := seedAffiliateMeUser(t, repo, "Affiliator Pending "+uniqueSuffix(), "affiliator")
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliators (user_id, status, terms_version) VALUES ($1, 'pending_approval', 'v1')`, userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `DELETE FROM affiliators WHERE user_id = $1`, userID)
		_, _ = repo.Pool().Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	profile, err := svc.GetMyAffiliateProfile(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, "pending_approval", profile.Status)
	require.Empty(t, profile.Code)
	require.Empty(t, profile.ReferralLink)
}

func TestAdminReadsSurviveACodelessPendingAffiliator(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	userID := seedAffiliateMeUser(t, repo, "Affiliator Codeless "+uniqueSuffix(), "affiliator")
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO affiliators (user_id, status, terms_version) VALUES ($1, 'pending_approval', 'v1')`, userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(ctx, `DELETE FROM affiliators WHERE user_id = $1`, userID)
		_, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	_, err = svc.AdminAffiliateReportSummary(ctx, from, to, AffiliateReportOptions{})
	require.NoError(t, err)
	_, _, err = svc.AdminListAffiliateFlags(ctx, AffiliateAdminQueueFilter{Status: "open", Limit: 20})
	require.NoError(t, err)
	_, _, err = svc.AdminListAffiliateAppeals(ctx, AffiliateAdminQueueFilter{Status: "pending", Limit: 20})
	require.NoError(t, err)
	_, _, err = svc.AdminListAffiliatePayouts(ctx, AffiliateAdminQueueFilter{Limit: 20})
	require.NoError(t, err)
	_, err = svc.ListAffiliateAdmin(ctx, AffiliateAdminListFilter{Status: "pending_approval"})
	require.NoError(t, err)
}

func TestGetMyAffiliateProfile_goldHasNoRiseThreshold(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator Gold "+uniqueSuffix(), "active", "gold")

	profile, err := svc.GetMyAffiliateProfile(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, "gold", profile.Level)
	require.NotNil(t, profile.LevelProgress)
	require.Empty(t, profile.LevelProgress.NextLevel)
	require.Nil(t, profile.LevelProgress.RiseThreshold)
}

func TestGetMyAffiliateProfile_missingProfileReturnsSentinel(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	userID := seedAffiliateMeUser(t, repo, "Affiliator No Profile "+uniqueSuffix(), "affiliator")

	_, err := svc.GetMyAffiliateProfile(ctx, userID)
	require.ErrorIs(t, err, ErrAffiliateProfileNotFound)
}

func TestGetMyAffiliateProfile_flagOffReturnsSentinel(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "false")

	userID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator FlagOff "+uniqueSuffix(), "active", "bronze")

	_, err := svc.GetMyAffiliateProfile(ctx, userID)
	require.ErrorIs(t, err, ErrAffiliateProfileNotFound)
}

func TestUpdateMyAffiliateBank_persistsAndAudits(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Bank "+uniqueSuffix(), "active", "bronze")

	profile, err := svc.UpdateMyAffiliateBank(ctx, userID, "BCA", "1234567890", "Budi Santoso")
	require.NoError(t, err)
	require.NotNil(t, profile.BankName)
	require.Equal(t, "BCA", *profile.BankName)

	var bankName, number, holder string
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT bank_name, bank_account_number, bank_account_holder FROM affiliators WHERE id = $1`,
		aff.ID,
	).Scan(&bankName, &number, &holder))
	require.Equal(t, "BCA", bankName)
	require.Equal(t, "1234567890", number)
	require.Equal(t, "Budi Santoso", holder)

	var audits int
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE action = 'affiliate.bank_update' AND target_id = $1`,
		aff.ID.String(),
	).Scan(&audits))
	require.Equal(t, 1, audits)
}

func TestUpdateMyAffiliateBank_requiresAllFields(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator Bank2 "+uniqueSuffix(), "active", "bronze")

	_, err := svc.UpdateMyAffiliateBank(ctx, userID, "BCA", "1234567890", "   ")
	require.ErrorIs(t, err, ErrMissingField)
}

func TestUpdateMyAffiliateBank_rejectedWhenNotActive(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator Susp "+uniqueSuffix(), "suspended", "bronze")

	_, err := svc.UpdateMyAffiliateBank(ctx, userID, "BCA", "1234567890", "Budi")
	require.ErrorIs(t, err, ErrAffiliateNotActive)
}

func TestUpdateMyAffiliateBank_unknownUserReturnsSentinel(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID := seedAffiliateMeUser(t, repo, "Affiliator Bank None "+uniqueSuffix(), "affiliator")

	_, err := svc.UpdateMyAffiliateBank(ctx, userID, "BCA", "1234567890", "Budi")
	require.ErrorIs(t, err, ErrAffiliateProfileNotFound)
}

func TestListMyAffiliateBindings_privacyAndSpend(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userIDA, affA := seedAffiliateMeAffiliator(t, repo, "Affiliator A "+uniqueSuffix(), "active", "bronze")
	_, affB := seedAffiliateMeAffiliator(t, repo, "Affiliator B "+uniqueSuffix(), "active", "bronze")

	studentA1 := seedAffiliateMeStudent(t, repo, "Budi Santoso")
	seedAffiliateMeBinding(t, repo, affA.ID, uuid.MustParse(studentA1), "active")
	paidDuring := time.Now().UTC().AddDate(0, 0, -5)
	seedAffiliateMeOrder(t, repo, studentA1, "paid", 100000, 10000, &paidDuring)
	seedAffiliateMeOrder(t, repo, studentA1, "payment_pending", 300000, 0, nil)

	studentA2 := seedAffiliateMeStudent(t, repo, "Citra Melati")
	seedAffiliateMeBinding(t, repo, affA.ID, uuid.MustParse(studentA2), "ended")
	paidBeforeBinding := time.Now().UTC().AddDate(0, 0, -30)
	seedAffiliateMeOrder(t, repo, studentA2, "completed", 50000, 0, &paidBeforeBinding)

	studentB := seedAffiliateMeStudent(t, repo, "Dewi Lestari")
	seedAffiliateMeBinding(t, repo, affB.ID, uuid.MustParse(studentB), "active")
	seedAffiliateMeOrder(t, repo, studentB, "paid", 500000, 0, &paidDuring)

	page, err := svc.ListMyAffiliateBindings(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.NoError(t, err)
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Data, 2)

	byName := make(map[string]float64, len(page.Data))
	for _, item := range page.Data {
		byName[item.Name] = item.TotalSpend
	}
	require.InDelta(t, 90000, byName["Budi S."], 0.001)
	require.InDelta(t, 0, byName["Citra M."], 0.001)

	for _, item := range page.Data {
		require.NotEmpty(t, item.BindingID)
		require.NotEmpty(t, item.RegisteredAt)
		require.NotZero(t, item.StartedAt)
		require.NotNil(t, item.EndedAt)
	}

	active, err := svc.ListMyAffiliateBindings(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20, Status: "active"})
	require.NoError(t, err)
	require.Equal(t, 1, active.Total)
	require.Equal(t, "Budi S.", active.Data[0].Name)
	require.Equal(t, "active", active.Data[0].Status)

	ended, err := svc.ListMyAffiliateBindings(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20, Status: "ended"})
	require.NoError(t, err)
	require.Equal(t, 1, ended.Total)

	pageB, err := svc.ListMyAffiliateBindings(ctx, seedAffiliateMeUser(t, repo, "Affiliator C "+uniqueSuffix(), "affiliator"), AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.ErrorIs(t, err, ErrAffiliateProfileNotFound)
	require.Empty(t, pageB.Data)

	_, err = svc.ListMyAffiliateBindings(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20, Status: "nonsense"})
	require.ErrorIs(t, err, ErrInvalidStatusFilter)

	single, err := svc.ListMyAffiliateBindings(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 1})
	require.NoError(t, err)
	require.Equal(t, 1, len(single.Data))
	require.Equal(t, 2, single.Total)
}

func TestListMyAffiliateCommissions_filtersAndPaging(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userIDA, affA := seedAffiliateMeAffiliator(t, repo, "Affiliator Comm "+uniqueSuffix(), "active", "bronze")
	_, affB := seedAffiliateMeAffiliator(t, repo, "Affiliator Comm B "+uniqueSuffix(), "active", "bronze")
	studentA := seedAffiliateMeStudent(t, repo, "Budi Santoso")
	studentB := seedAffiliateMeStudent(t, repo, "Dewi Lestari")

	oct10 := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	oct20 := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	oct15 := oct20.AddDate(0, 0, -5)
	oct21 := oct20.AddDate(0, 0, 1)
	seedAffiliateJobCommission(t, repo, affA.ID, uuid.MustParse(studentA), affiliateJobCommissionSeed{
		productType: "exam", base: 100000, pct: 10, amount: 10000,
		status: "available", level: "bronze", createdAt: oct10, paidAt: oct10,
	})
	seedAffiliateJobCommission(t, repo, affA.ID, uuid.MustParse(studentA), affiliateJobCommissionSeed{
		productType: "book", base: 200000, pct: 3, amount: 6000,
		status: "available", level: "bronze", createdAt: oct20, paidAt: oct20,
	})
	seedAffiliateJobCommission(t, repo, affA.ID, uuid.MustParse(studentA), affiliateJobCommissionSeed{
		productType: "exam", base: 300000, pct: 10, amount: 30000,
		status: "pending", level: "bronze", createdAt: oct20, paidAt: oct20,
	})
	seedAffiliateJobCommission(t, repo, affB.ID, uuid.MustParse(studentB), affiliateJobCommissionSeed{
		productType: "exam", base: 400000, pct: 10, amount: 40000,
		status: "available", level: "bronze", createdAt: oct20, paidAt: oct20,
	})

	all, err := svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.NoError(t, err)
	require.Equal(t, 3, all.Total)

	available, err := svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20, Status: "available"})
	require.NoError(t, err)
	require.Equal(t, 2, available.Total)

	byType, err := svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20, ProductType: "exam"})
	require.NoError(t, err)
	require.Equal(t, 2, byType.Total)

	both, err := svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20, Status: "available", ProductType: "book"})
	require.NoError(t, err)
	require.Equal(t, 1, both.Total)
	require.InDelta(t, 6000, both.Data[0].Amount, 0.001)

	ranged, err := svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20,
		From: &oct15, To: &oct21})
	require.NoError(t, err)
	require.Equal(t, 2, ranged.Total)

	pageB, err := svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 2, PerPage: 2})
	require.NoError(t, err)
	require.Equal(t, 3, pageB.Total)
	require.Len(t, pageB.Data, 1)
	require.InDelta(t, 10000, pageB.Data[0].Amount, 0.001)

	pageBFull, err := svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 2})
	require.NoError(t, err)
	require.Len(t, pageBFull.Data, 2)
	require.Greater(t, pageBFull.Data[0].PaidAt.Unix(), oct10.Unix())

	_, err = svc.ListMyAffiliateCommissions(ctx, userIDA, AffiliateMeListFilter{Page: 1, PerPage: 20, Status: "weird"})
	require.ErrorIs(t, err, ErrInvalidStatusFilter)
}

func TestGetMyAffiliateLedger_saldoAndOrdering(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Ledger "+uniqueSuffix(), "active", "bronze")
	base := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 12000, base)
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "refund_deduction", -3000, base.Add(24*time.Hour))
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "payout", 5000, base.Add(48*time.Hour))

	desc, err := svc.GetMyAffiliateLedger(ctx, userID, AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.NoError(t, err)
	require.InDelta(t, 14000, desc.Saldo, 0.001)
	require.Equal(t, 3, desc.Total)
	require.Len(t, desc.Data, 3)
	require.Equal(t, "payout", desc.Data[0].EntryType)
	require.Equal(t, "commission", desc.Data[2].EntryType)

	asc, err := svc.GetMyAffiliateLedger(ctx, userID, AffiliateMeListFilter{Page: 1, PerPage: 20, Order: "asc"})
	require.NoError(t, err)
	require.Equal(t, "commission", asc.Data[0].EntryType)

	paged, err := svc.GetMyAffiliateLedger(ctx, userID, AffiliateMeListFilter{Page: 2, PerPage: 2})
	require.NoError(t, err)
	require.Len(t, paged.Data, 1)
	require.InDelta(t, 14000, paged.Saldo, 0.001)

	userIDB, affB := seedAffiliateMeAffiliator(t, repo, "Affiliator Ledger B "+uniqueSuffix(), "active", "bronze")
	ledgerB, err := svc.GetMyAffiliateLedger(ctx, userIDB, AffiliateMeListFilter{Page: 1, PerPage: 20})
	require.NoError(t, err)
	require.Equal(t, 0, ledgerB.Total)
	require.InDelta(t, 0, ledgerB.Saldo, 0.001)
	require.Empty(t, ledgerB.Data)
	_ = affB
}

func TestGetMyAffiliateStats_math(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Stats "+uniqueSuffix(), "active", "bronze")
	for _, ip := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"} {
		seedAffiliateMeClick(t, repo, aff.ID, aff.Code, ip)
	}

	student1 := seedAffiliateMeStudent(t, repo, "Budi Santoso")
	seedAffiliateMeBinding(t, repo, aff.ID, uuid.MustParse(student1), "active")
	student2 := seedAffiliateMeStudent(t, repo, "Citra Melati")
	binding2 := seedAffiliateMeBindingSource(t, repo, aff.ID, uuid.MustParse(student2), "active", "manual_code")
	_, err := repo.Pool().Exec(ctx,
		`UPDATE affiliate_bindings SET status = 'ended', updated_at = now() WHERE id = $1`, binding2.ID)
	require.NoError(t, err)

	stats, err := svc.GetMyAffiliateStats(ctx, userID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 3, stats.Clicks)
	require.Equal(t, 2, stats.Registrations)
	require.Equal(t, 2, stats.TotalStudents)
	require.Equal(t, 1, stats.ActiveBindings)
	require.InDelta(t, 2.0/3.0, stats.Conversion, 0.0001)
	require.Equal(t, 1, stats.RegistrationsBySource["link"])
	require.Equal(t, 1, stats.RegistrationsBySource["manual_code"])
	require.Equal(t, 2, stats.NewStudents)
}

func TestGetMyAffiliateStats_period(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, aff := seedAffiliateMeAffiliator(t, repo, "Affiliator Stats Period "+uniqueSuffix(), "active", "bronze")
	seedAffiliateMeClick(t, repo, aff.ID, aff.Code, "198.51.100.9")
	student := seedAffiliateMeStudent(t, repo, "Budi Santoso")
	seedAffiliateMeBinding(t, repo, aff.ID, uuid.MustParse(student), "active")
	old := time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 12000, old)
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "commission", 8000, recent)
	seedAffiliateMeLedgerEntry(t, repo, aff.ID, "refund_deduction", -3000, recent)

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	inRange, err := svc.GetMyAffiliateStats(ctx, userID, &from, &to)
	require.NoError(t, err)
	require.InDelta(t, 8000, inRange.CommissionEarned, 0.001)
	require.Equal(t, 0, inRange.Clicks)
	require.Equal(t, 0, inRange.NewStudents)
	require.Equal(t, 1, inRange.TotalStudents)

	all, err := svc.GetMyAffiliateStats(ctx, userID, nil, nil)
	require.NoError(t, err)
	require.InDelta(t, 20000, all.CommissionEarned, 0.001)
	require.Equal(t, 1, all.Clicks)
	require.Equal(t, 1, all.NewStudents)
}

func TestGetMyAffiliateStats_empty(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	setAffiliateMeFlag(t, repo, "true")

	userID, _ := seedAffiliateMeAffiliator(t, repo, "Affiliator Stats Empty "+uniqueSuffix(), "active", "bronze")

	stats, err := svc.GetMyAffiliateStats(ctx, userID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 0, stats.Clicks)
	require.Equal(t, 0, stats.Registrations)
	require.InDelta(t, 0, stats.Conversion, 0.0001)
	require.Empty(t, stats.RegistrationsBySource)
}
