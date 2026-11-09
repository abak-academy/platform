package service

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const partnerOnlyPrograms = `{"affiliate": false, "partner": {"school_ids": []}}`
const bothPrograms = `{"affiliate": true, "partner": {"school_ids": []}}`

// seedProgramAccount creates an affiliator user with a profile. An empty
// programs value leaves the column NULL, which is a plain Affiliator account.
func seedProgramAccount(t *testing.T, repo *repository.Repository, userStatus, programStatus, code, programs string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	suffix := uniqueSuffix()
	var userID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO users (email, username, role, name, status) VALUES ($1, $2, 'affiliator', 'Program Account', $3) RETURNING id`,
		"prog-"+suffix+"@test.local", "prog"+suffix, userStatus,
	).Scan(&userID))
	var programsValue, codeValue any
	if programs != "" {
		programsValue = programs
	}
	if code != "" {
		codeValue = code
	}
	_, err := repo.Pool().Exec(ctx,
		`INSERT INTO affiliators (user_id, referral_code, program_status, terms_version, programs) VALUES ($1, $2, $3, 'v1', $4::jsonb)`,
		userID, codeValue, programStatus, programsValue)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(ctx, `DELETE FROM affiliators WHERE user_id = $1`, userID)
		_, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

func TestAffiliatorProgramStates_coversEveryMembershipAndStatus(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	rejected := "tidak lengkap"

	tests := []struct {
		name          string
		userStatus    string
		programStatus string
		programs      string
		wantAffiliate ProgramState
		wantPartner   ProgramState
		wantActive    bool
	}{
		{"a legacy account is an Affiliator only", "active", "active", "", ProgramActive, ProgramNotEnrolled, true},
		{"a Mitra-only account is not in the Affiliator program", "active", "active", partnerOnlyPrograms, ProgramNotEnrolled, ProgramActive, true},
		{"both programs", "active", "active", bothPrograms, ProgramActive, ProgramActive, true},
		{"a pending application is pending in its program", "active", "pending_approval", partnerOnlyPrograms, ProgramNotEnrolled, ProgramPending, true},
		{"a rejected application is rejected in its program", "active", "rejected", partnerOnlyPrograms, ProgramNotEnrolled, ProgramRejected, true},
		{"a suspended account keeps its programs but is not active", "deactivated", "suspended", bothPrograms, ProgramActive, ProgramActive, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := seedProgramAccount(t, repo, tt.userStatus, tt.programStatus, "", tt.programs)
			if tt.programStatus == "rejected" {
				_, err := repo.Pool().Exec(ctx, `UPDATE affiliators SET rejection_reason = $2 WHERE user_id = $1`, id, rejected)
				require.NoError(t, err)
			}

			got, err := svc.AffiliatorProgramStates(ctx, id.String())
			require.NoError(t, err)
			require.Equal(t, tt.wantAffiliate, got.Affiliate)
			require.Equal(t, tt.wantPartner, got.Partner)
			require.Equal(t, tt.wantActive, got.AccountActive)
			if tt.programStatus == "rejected" {
				require.NotNil(t, got.RejectionReason)
				require.Equal(t, rejected, *got.RejectionReason)
			}
		})
	}

	t.Run("a user without a profile belongs to neither program", func(t *testing.T) {
		student := seedAffiliateTestStudent(t, repo, "prog-student-"+uniqueSuffix()+"@test.local", "")
		got, err := svc.AffiliatorProgramStates(ctx, student.ID)
		require.NoError(t, err)
		require.Equal(t, ProgramNotEnrolled, got.Affiliate)
		require.Equal(t, ProgramNotEnrolled, got.Partner)
		require.False(t, got.AccountActive)
	})
}

func TestChangeAffiliateStatus_keepsTheUserStatusInStep(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "prog-actor-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })
	id := seedProgramAccount(t, repo, "active", "active", "AFP"+uniqueSuffix()[:5], bothPrograms)

	userStatus := func() string {
		var status string
		require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, id).Scan(&status))
		return status
	}

	_, err := svc.ChangeAffiliateStatus(ctx, actor, id.String(), "suspended", "pelanggaran")
	require.NoError(t, err)
	require.Equal(t, "deactivated", userStatus(), "a suspended account is a deactivated user: read-only, not locked out")
	states, err := svc.AffiliatorProgramStates(ctx, id.String())
	require.NoError(t, err)
	require.False(t, states.AccountActive)
	require.Equal(t, ProgramActive, states.Partner, "suspension does not remove a program")

	_, err = svc.ChangeAffiliateStatus(ctx, actor, id.String(), "active", "selesai")
	require.NoError(t, err)
	require.Equal(t, "active", userStatus())
}

func TestDecideAffiliateApproval_partnerOnlyGetsNoReferralCode(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "prog-appr-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })

	t.Run("a Mitra application is approved without a code", func(t *testing.T) {
		id := seedProgramAccount(t, repo, "active", "pending_approval", "", partnerOnlyPrograms)
		aff, err := svc.DecideAffiliateApproval(ctx, actor, id.String(), "approve", "", nil)
		require.NoError(t, err)
		require.Equal(t, "active", aff.Status)
		require.Empty(t, aff.Code)
		require.True(t, aff.Programs.HasPartner())
		require.False(t, aff.Programs.HasAffiliate())
	})

	t.Run("the admin can change the requested program on approval", func(t *testing.T) {
		id := seedProgramAccount(t, repo, "active", "pending_approval", "", partnerOnlyPrograms)
		aff, err := svc.DecideAffiliateApproval(ctx, actor, id.String(), "approve", "", []string{"affiliate", "partner"})
		require.NoError(t, err)
		require.NotEmpty(t, aff.Code, "an Affiliator gets a referral code")
		require.True(t, aff.Programs.HasAffiliate())
		require.True(t, aff.Programs.HasPartner())
	})

	t.Run("an unknown program is refused and nothing changes", func(t *testing.T) {
		id := seedProgramAccount(t, repo, "active", "pending_approval", "", partnerOnlyPrograms)
		_, err := svc.DecideAffiliateApproval(ctx, actor, id.String(), "approve", "", []string{"reseller"})
		require.ErrorIs(t, err, ErrAffiliateInvalidProgram)
		states, err := svc.AffiliatorProgramStates(ctx, id.String())
		require.NoError(t, err)
		require.Equal(t, ProgramPending, states.Partner)
	})
}

func TestAdminAddAffiliatorProgram(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "prog-add-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })

	t.Run("adding Mitra to an Affiliator keeps the Affiliator program", func(t *testing.T) {
		id := seedProgramAccount(t, repo, "active", "active", "AFA"+uniqueSuffix()[:5], "")
		aff, err := svc.AdminAddAffiliatorProgram(ctx, actor, id.String(), "partner")
		require.NoError(t, err)
		require.True(t, aff.Programs.HasPartner())
		require.True(t, aff.Programs.HasAffiliate())
	})

	t.Run("adding Affiliator to a Mitra creates the referral code once", func(t *testing.T) {
		id := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms)
		aff, err := svc.AdminAddAffiliatorProgram(ctx, actor, id.String(), "affiliate")
		require.NoError(t, err)
		require.True(t, aff.Programs.HasAffiliate())
		require.NotEmpty(t, aff.Code)

		again, err := svc.AdminAddAffiliatorProgram(ctx, actor, id.String(), "affiliate")
		require.NoError(t, err, "adding a program the account has is a no-op")
		require.Equal(t, aff.Code, again.Code, "the code is not regenerated")
	})

	t.Run("a pending or rejected account cannot take a new program", func(t *testing.T) {
		id := seedProgramAccount(t, repo, "active", "pending_approval", "", partnerOnlyPrograms)
		_, err := svc.AdminAddAffiliatorProgram(ctx, actor, id.String(), "affiliate")
		require.ErrorIs(t, err, ErrAffiliatorNotApproved)
	})

	t.Run("only the two programs exist", func(t *testing.T) {
		id := seedProgramAccount(t, repo, "active", "active", "AFB"+uniqueSuffix()[:5], "")
		_, err := svc.AdminAddAffiliatorProgram(ctx, actor, id.String(), "reseller")
		require.ErrorIs(t, err, ErrAffiliateInvalidProgram)
	})
}

func TestAdminSetPartnerSchools(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "prog-sch-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })
	partner := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms)
	schoolA := createTestSchool(t, svc)
	schoolB := createTestSchool(t, svc)

	aff, err := svc.AdminSetPartnerSchools(ctx, actor, partner.String(), []string{schoolA, schoolB, schoolA})
	require.NoError(t, err)
	require.Equal(t, []string{schoolA, schoolB}, aff.Programs.Partner.SchoolIDs, "duplicates are dropped, order is kept")
	states, err := svc.AffiliatorProgramStates(ctx, partner.String())
	require.NoError(t, err)
	require.Equal(t, []string{schoolA, schoolB}, states.PartnerSchools)

	aff, err = svc.AdminSetPartnerSchools(ctx, actor, partner.String(), nil)
	require.NoError(t, err)
	require.Empty(t, aff.Programs.Partner.SchoolIDs, "the list is replaced, so it can be emptied")

	t.Run("an unknown school is refused", func(t *testing.T) {
		_, err := svc.AdminSetPartnerSchools(ctx, actor, partner.String(), []string{uuid.NewString()})
		require.ErrorIs(t, err, ErrSchoolNotFound)
	})

	t.Run("a deactivated school is refused", func(t *testing.T) {
		_, err := repo.Pool().Exec(ctx, `UPDATE school SET status = 'deactivated' WHERE id = $1`, schoolB)
		require.NoError(t, err)
		_, err = svc.AdminSetPartnerSchools(ctx, actor, partner.String(), []string{schoolB})
		require.ErrorIs(t, err, ErrSchoolDeactivated)
	})

	t.Run("an account outside the Mitra program has no schools to set", func(t *testing.T) {
		plain := seedProgramAccount(t, repo, "active", "active", "AFC"+uniqueSuffix()[:5], "")
		_, err := svc.AdminSetPartnerSchools(ctx, actor, plain.String(), []string{schoolA})
		require.ErrorIs(t, err, ErrAffiliatorNotPartner)
	})
}

func TestCanHoldSession_onlyAffiliatorsStayReadable(t *testing.T) {
	tests := []struct {
		role, status string
		want         bool
	}{
		{RoleStudent, "active", true},
		{RoleAffiliator, "active", true},
		{RoleAffiliator, "deactivated", true},
		{RoleStudent, "deactivated", false},
		{RoleAdminSchool, "deactivated", false},
		{RoleAffiliator, "pending_verification", false},
		{RoleAffiliator, "deleted", false},
	}
	for _, tt := range tests {
		got := canHoldSession(&model.User{Role: tt.role, Status: tt.status})
		require.Equal(t, tt.want, got, "%s/%s", tt.role, tt.status)
	}
}

func TestRegisterAffiliator_picksTheProgramAndNeedsItSwitchedOn(t *testing.T) {
	svc, repo, _ := newAffiliateActivationService(t)
	ctx := context.Background()
	register := func(program string) error {
		_, err := svc.RegisterAffiliator(ctx, "reg-"+uniqueSuffix()+"@test.local", "password123", "Budi Santoso", "0812"+uniqueSuffix()[:6], "2026-11", program, "127.0.0.1")
		return err
	}
	setPartnerFlag := func(value string) {
		_, err := repo.Pool().Exec(ctx,
			`INSERT INTO system_config (key, value, is_secret) VALUES ('partner_enabled', $1, false) ON CONFLICT (key) DO UPDATE SET value = $1`, value)
		require.NoError(t, err)
	}
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM system_config WHERE key = 'partner_enabled'`) })

	setAffiliateFlagForCommission(t, repo, "true")
	setPartnerFlag("false")
	require.ErrorIs(t, register("partner"), ErrPartnerRegistrationClosed, "the Mitra program is off")
	require.NoError(t, register(""), "an empty program is the Affiliator program, as before")
	require.ErrorIs(t, register("reseller"), ErrAffiliateInvalidProgram)

	setPartnerFlag("true")
	setAffiliateFlagForCommission(t, repo, "false")
	require.ErrorIs(t, register("affiliate"), ErrAffiliateRegistrationClosed, "the two switches are separate")
	require.NoError(t, register("partner"))

	var programs []byte
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`SELECT a.programs FROM affiliators a JOIN users u ON u.id = a.user_id
		 WHERE u.role = 'affiliator' AND a.programs IS NOT NULL ORDER BY u.created_at DESC LIMIT 1`).Scan(&programs))
	require.JSONEq(t, `{"affiliate": false, "partner": {"school_ids": []}}`, string(programs))
}

func TestAdminCreateAffiliator_partnerOnlyHasNoReferralCode(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actor := seedAffiliateAdminActor(t, repo, "prog-create-"+uniqueSuffix()+"@test.local")
	t.Cleanup(func() { _, _ = repo.Pool().Exec(ctx, `DELETE FROM users WHERE id = $1`, actor) })

	created, err := svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{
		Name: "Mitra Baru", Email: "mitra-" + uniqueSuffix() + "@test.local", Phone: "0812" + uniqueSuffix()[:6],
		Programs: []string{"partner"},
	})
	require.NoError(t, err)
	require.Empty(t, created.Code)
	states, err := svc.AffiliatorProgramStates(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, ProgramActive, states.Partner)
	require.Equal(t, ProgramNotEnrolled, states.Affiliate)

	both, err := svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{
		Name: "Dua Program", Email: "dua-" + uniqueSuffix() + "@test.local", Phone: "0813" + uniqueSuffix()[:6],
		Programs: []string{"affiliate", "partner"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, both.Code)

	_, err = svc.AdminCreateAffiliator(ctx, actor, AdminCreateAffiliatorInput{
		Name: "Salah", Email: "salah-" + uniqueSuffix() + "@test.local", Phone: "0814" + uniqueSuffix()[:6],
		Programs: []string{"reseller"},
	})
	require.ErrorIs(t, err, ErrAffiliateInvalidProgram)
}

func TestAffiliateJobsSkipAccountsOutsideTheAffiliatorProgram(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()
	affiliator := seedProgramAccount(t, repo, "active", "active", "AFD"+uniqueSuffix()[:5], "")
	partnerOnly := seedProgramAccount(t, repo, "active", "active", "", partnerOnlyPrograms)
	both := seedProgramAccount(t, repo, "active", "active", "AFE"+uniqueSuffix()[:5], bothPrograms)

	list, err := repo.ListAffiliatorsForLevelEval(ctx)
	require.NoError(t, err)
	ids := map[uuid.UUID]bool{}
	for _, a := range list {
		ids[a.ID] = true
	}
	require.True(t, ids[affiliator], "a plain Affiliator is evaluated")
	require.True(t, ids[both], "an account in both programs is evaluated")
	require.False(t, ids[partnerOnly], "a Mitra-only account has no level to evaluate")
}
