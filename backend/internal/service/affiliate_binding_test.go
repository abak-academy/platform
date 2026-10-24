package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The real-DB fixture is shared and rows are never reset, so every test seeds
// its own settings row (singleton upsert) and unique users/codes, and never
// runs in parallel.
type affiliateSettingsSeed struct {
	launchDate    *time.Time
	bindingMonths int
	minExtend     float64
}

func seedAffiliateSettingsForBinding(t *testing.T, repo *repository.Repository, seed affiliateSettingsSeed) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_settings (id, launch_date, binding_months, min_extend_amount)
		 VALUES (1, $1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET launch_date = $1, binding_months = $2, min_extend_amount = $3, updated_at = now()`,
		seed.launchDate, seed.bindingMonths, seed.minExtend,
	)
	require.NoError(t, err)
}

func seedAffiliateBindingUser(t *testing.T, repo *repository.Repository, email, phone, role string) string {
	t.Helper()
	var id string
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (email, phone, role, name) VALUES ($1, NULLIF($2, ''), $3, 'Affiliate Seed') RETURNING id`,
		email, phone, role,
	).Scan(&id))
	return id
}

type affiliateTestOwner struct {
	affiliatorID uuid.UUID
	userID       string
	code         string
}

func seedAffiliatorOwner(t *testing.T, repo *repository.Repository, email, phone, status string) affiliateTestOwner {
	t.Helper()
	userID := seedAffiliateBindingUser(t, repo, email, phone, "affiliator")
	code := "AF" + strings.ToUpper(uniqueSuffix())
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO affiliators (user_id, code, terms_version, status) VALUES ($1, $2, 'v1', $3) RETURNING id`,
		userID, code, status,
	).Scan(&id))
	return affiliateTestOwner{affiliatorID: id, userID: userID, code: code}
}

func seedAffiliateTestStudent(t *testing.T, repo *repository.Repository, email, phone string) *model.User {
	t.Helper()
	id := seedAffiliateBindingUser(t, repo, email, phone, "student")
	return &model.User{ID: id, Email: &email, Phone: &phone, CreatedAt: time.Now().UTC()}
}

func createAffiliateBindingCommitted(t *testing.T, svc *Service, repo *repository.Repository, studentID string, affiliatorID uuid.UUID, source string) {
	t.Helper()
	tx, err := repo.BeginTx(context.Background())
	require.NoError(t, err)
	require.NoError(t, svc.CreateBinding(context.Background(), tx, studentID, affiliatorID, source))
	require.NoError(t, tx.Commit(context.Background()))
}

func countAffiliateBindings(t *testing.T, repo *repository.Repository, studentID string) int {
	t.Helper()
	var n int
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM affiliate_bindings WHERE student_id = $1`, studentID,
	).Scan(&n))
	return n
}

func getActiveAffiliateBinding(t *testing.T, repo *repository.Repository, studentID string) *model.AffiliateBinding {
	t.Helper()
	var b model.AffiliateBinding
	err := repo.Pool().QueryRow(context.Background(),
		`SELECT id, affiliator_id, student_id, status, source, started_at, ended_at, last_extended_at, created_at, updated_at
		 FROM affiliate_bindings WHERE student_id = $1 AND status = 'active'`,
		studentID,
	).Scan(&b.ID, &b.AffiliatorID, &b.StudentID, &b.Status, &b.Source, &b.StartedAt, &b.EndedAt, &b.LastExtendedAt, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	require.NoError(t, err)
	return &b
}

func endActiveAffiliateBinding(t *testing.T, repo *repository.Repository, studentID string) {
	t.Helper()
	_, err := repo.Pool().Exec(context.Background(),
		`UPDATE affiliate_bindings SET status = 'ended', ended_at = now() WHERE student_id = $1 AND status = 'active'`,
		studentID,
	)
	require.NoError(t, err)
}

func latestAffiliateFlag(t *testing.T, repo *repository.Repository, affiliatorID uuid.UUID) (rule, payload string) {
	t.Helper()
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT rule, payload::text FROM affiliate_flags WHERE affiliator_id = $1 ORDER BY created_at DESC LIMIT 1`,
		affiliatorID,
	).Scan(&rule, &payload))
	return rule, payload
}

func TestResolveBinding_ManualWinsOverCookie(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})

	manual := seedAffiliatorOwner(t, repo, "aff-manual-"+uniqueSuffix()+"@test.local", "", "active")
	cookie := seedAffiliatorOwner(t, repo, "aff-cookie-"+uniqueSuffix()+"@test.local", "", "active")

	t.Run("manual code beats cookie", func(t *testing.T) {
		student := seedAffiliateTestStudent(t, repo, "student-manual-"+uniqueSuffix()+"@test.local", "")
		manualCode, cookieCode := manual.code, cookie.code
		decision, err := svc.ResolveBinding(ctx, student, &manualCode, &cookieCode)
		require.NoError(t, err)
		require.NotNil(t, decision)
		require.Equal(t, manual.affiliatorID, decision.AffiliatorID)
		require.Equal(t, "manual_code", decision.Source)
	})

	t.Run("cookie only binds as link", func(t *testing.T) {
		student := seedAffiliateTestStudent(t, repo, "student-cookie-"+uniqueSuffix()+"@test.local", "")
		cookieCode := cookie.code
		decision, err := svc.ResolveBinding(ctx, student, nil, &cookieCode)
		require.NoError(t, err)
		require.NotNil(t, decision)
		require.Equal(t, cookie.affiliatorID, decision.AffiliatorID)
		require.Equal(t, "link", decision.Source)
	})
}

func TestResolveBinding_UnknownOrInactiveCode_NoBinding(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})

	pending := seedAffiliatorOwner(t, repo, "aff-pending-"+uniqueSuffix()+"@test.local", "", "pending_approval")
	unknownCode := "ZZ" + uniqueSuffix()
	pendingCode := pending.code
	student := seedAffiliateTestStudent(t, repo, "student-unknown-"+uniqueSuffix()+"@test.local", "")

	tests := []struct {
		name   string
		manual *string
		cookie *string
	}{
		{"unknown code", &unknownCode, nil},
		{"inactive affiliator code", &pendingCode, nil},
		{"no codes at all", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := svc.ResolveBinding(ctx, student, tt.manual, tt.cookie)
			require.NoError(t, err)
			require.Nil(t, decision)
		})
	}
	require.Equal(t, 0, countAffiliateBindings(t, repo, student.ID))
}

func TestResolveBinding_SelfPurchase_NoBindingAndFlag(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})

	t.Run("same email", func(t *testing.T) {
		email := "self-email-" + uniqueSuffix() + "@test.local"
		owner := seedAffiliatorOwner(t, repo, email, "", "active")
		student := &model.User{ID: uuid.NewString(), Email: &email, CreatedAt: time.Now().UTC()}
		code := owner.code
		decision, err := svc.ResolveBinding(ctx, student, &code, nil)
		require.NoError(t, err)
		require.Nil(t, decision)
		rule, payload := latestAffiliateFlag(t, repo, owner.affiliatorID)
		require.Equal(t, "self_purchase", rule)
		var fields map[string]string
		require.NoError(t, json.Unmarshal([]byte(payload), &fields))
		require.Equal(t, student.ID, fields["user_id"])
		require.Equal(t, "email", fields["matched_field"])
	})

	t.Run("same phone", func(t *testing.T) {
		phone := "+62812" + uniqueSuffix()[:6]
		owner := seedAffiliatorOwner(t, repo, "self-phone-"+uniqueSuffix()+"@test.local", phone, "active")
		student := &model.User{ID: uuid.NewString(), Phone: &phone, CreatedAt: time.Now().UTC()}
		code := owner.code
		decision, err := svc.ResolveBinding(ctx, student, &code, nil)
		require.NoError(t, err)
		require.Nil(t, decision)
		rule, payload := latestAffiliateFlag(t, repo, owner.affiliatorID)
		require.Equal(t, "self_purchase", rule)
		var fields map[string]string
		require.NoError(t, json.Unmarshal([]byte(payload), &fields))
		require.Equal(t, "phone", fields["matched_field"])
	})
}

func TestResolveBinding_PreLaunchStudent_NoBinding(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	t.Run("student created before launch date", func(t *testing.T) {
		launch := time.Now().UTC().Add(24 * time.Hour)
		seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
		owner := seedAffiliatorOwner(t, repo, "aff-pre1-"+uniqueSuffix()+"@test.local", "", "active")
		student := seedAffiliateTestStudent(t, repo, "student-pre1-"+uniqueSuffix()+"@test.local", "")
		code := owner.code
		decision, err := svc.ResolveBinding(ctx, student, &code, nil)
		require.NoError(t, err)
		require.Nil(t, decision)
		require.Equal(t, 0, countAffiliateBindings(t, repo, student.ID))
	})

	t.Run("launch date not set", func(t *testing.T) {
		seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: nil, bindingMonths: 3})
		owner := seedAffiliatorOwner(t, repo, "aff-pre2-"+uniqueSuffix()+"@test.local", "", "active")
		student := seedAffiliateTestStudent(t, repo, "student-pre2-"+uniqueSuffix()+"@test.local", "")
		code := owner.code
		decision, err := svc.ResolveBinding(ctx, student, &code, nil)
		require.NoError(t, err)
		require.Nil(t, decision)
	})
}

func TestResolveBinding_KeepsExistingActiveBinding(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})

	holder := seedAffiliatorOwner(t, repo, "aff-holder-"+uniqueSuffix()+"@test.local", "", "active")
	other := seedAffiliatorOwner(t, repo, "aff-other-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-bound-"+uniqueSuffix()+"@test.local", "")

	createAffiliateBindingCommitted(t, svc, repo, student.ID, holder.affiliatorID, "manual_code")

	otherCode := other.code
	decision, err := svc.ResolveBinding(ctx, student, &otherCode, nil)
	require.NoError(t, err)
	require.Nil(t, decision)
	require.Equal(t, 1, countAffiliateBindings(t, repo, student.ID))
	require.Equal(t, holder.affiliatorID, getActiveAffiliateBinding(t, repo, student.ID).AffiliatorID)
}

func TestCreateBinding_SecondActiveRefusedEndedAllowsRebind(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})

	owner := seedAffiliatorOwner(t, repo, "aff-rebind-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-rebind-"+uniqueSuffix()+"@test.local", "")

	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "manual_code")
	require.Equal(t, 1, countAffiliateBindings(t, repo, student.ID))

	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")
	require.Equal(t, 1, countAffiliateBindings(t, repo, student.ID), "a second active binding must not be inserted")

	endActiveAffiliateBinding(t, repo, student.ID)
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "link")
	require.Equal(t, 2, countAffiliateBindings(t, repo, student.ID), "an ended binding must allow a new active one")
	binding := getActiveAffiliateBinding(t, repo, student.ID)
	require.NotNil(t, binding)
	require.Equal(t, "link", binding.Source)
}

func TestExtendBinding_AmountThresholdAndReanchor(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3, minExtend: 100000})

	owner := seedAffiliatorOwner(t, repo, "aff-extend-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-extend-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "manual_code")
	binding := getActiveAffiliateBinding(t, repo, student.ID)
	require.NotNil(t, binding)

	t.Run("below threshold does not extend", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		require.NoError(t, err)
		require.NoError(t, svc.ExtendBinding(ctx, tx, student.ID, binding.StartedAt.Add(2*time.Second), 99999))
		require.NoError(t, tx.Commit(ctx))
		row := getActiveAffiliateBinding(t, repo, student.ID)
		require.True(t, row.EndedAt.Equal(binding.StartedAt.AddDate(0, 3, 0)))
		require.Nil(t, row.LastExtendedAt)
	})

	t.Run("payment at threshold extends from paidAt", func(t *testing.T) {
		paidAt := binding.StartedAt.Add(2 * time.Second)
		tx, err := repo.BeginTx(ctx)
		require.NoError(t, err)
		require.NoError(t, svc.ExtendBinding(ctx, tx, student.ID, paidAt, 100000))
		require.NoError(t, tx.Commit(ctx))
		row := getActiveAffiliateBinding(t, repo, student.ID)
		require.NotNil(t, row.EndedAt)
		require.True(t, row.EndedAt.Equal(paidAt.AddDate(0, 3, 0)))
		require.NotNil(t, row.LastExtendedAt)
		require.True(t, row.LastExtendedAt.Equal(paidAt))
	})

	t.Run("earlier payment keeps the longer window", func(t *testing.T) {
		paidAt := binding.StartedAt.Add(1 * time.Second)
		tx, err := repo.BeginTx(ctx)
		require.NoError(t, err)
		require.NoError(t, svc.ExtendBinding(ctx, tx, student.ID, paidAt, 100000))
		require.NoError(t, tx.Commit(ctx))
		row := getActiveAffiliateBinding(t, repo, student.ID)
		require.True(t, row.EndedAt.Equal(binding.StartedAt.Add(2*time.Second).AddDate(0, 3, 0)))
		require.True(t, row.LastExtendedAt.Equal(paidAt))
	})

	t.Run("no active binding is a no-op", func(t *testing.T) {
		endActiveAffiliateBinding(t, repo, student.ID)
		tx, err := repo.BeginTx(ctx)
		require.NoError(t, err)
		require.NoError(t, svc.ExtendBinding(ctx, tx, student.ID, time.Now().UTC(), 100000))
		require.NoError(t, tx.Commit(ctx))
		require.Nil(t, getActiveAffiliateBinding(t, repo, student.ID))
	})
}

func TestExpireBindings_DueOnlyAndIdempotent(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})

	owner := seedAffiliatorOwner(t, repo, "aff-expire-"+uniqueSuffix()+"@test.local", "", "active")
	due := seedAffiliateTestStudent(t, repo, "student-due-"+uniqueSuffix()+"@test.local", "")
	live := seedAffiliateTestStudent(t, repo, "student-live-"+uniqueSuffix()+"@test.local", "")

	createAffiliateBindingCommitted(t, svc, repo, due.ID, owner.affiliatorID, "manual_code")
	createAffiliateBindingCommitted(t, svc, repo, live.ID, owner.affiliatorID, "manual_code")
	_, err := repo.Pool().Exec(ctx,
		`UPDATE affiliate_bindings SET ended_at = now() - interval '1 hour' WHERE student_id = $1 AND status = 'active'`,
		due.ID,
	)
	require.NoError(t, err)

	expired, err := svc.ExpireBindings(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, 1, expired)
	require.Nil(t, getActiveAffiliateBinding(t, repo, due.ID))
	require.NotNil(t, getActiveAffiliateBinding(t, repo, live.ID))

	expiredAgain, err := svc.ExpireBindings(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, 0, expiredAgain)
}
