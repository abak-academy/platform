package service

import (
	"akademi-bimbel/config"
	"context"
	"errors"
	"testing"
	"time"

	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestIsValidAdminRole(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleSuperAdmin, true},
		{RoleAdminStore, true},
		{RoleAdminExam, true},
		{RoleAdminSchool, true},
		{"student", false},
		{"ghost", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.role, func(t *testing.T) {
			got := isValidAdminRole(tc.role)
			if got != tc.want {
				t.Errorf("isValidAdminRole(%q): want %v, got %v", tc.role, tc.want, got)
			}
		})
	}
}

func TestIsValidStatusFilter(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"active", true},
		{"deactivated", true},
		{"deleted", false},
		{"pending", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			got := isValidStatusFilter(tc.status)
			if got != tc.want {
				t.Errorf("isValidStatusFilter(%q): want %v, got %v", tc.status, tc.want, got)
			}
		})
	}
}

// -- self-deactivation guard --

func TestCheckSelfDeactivation(t *testing.T) {
	t.Run("same user self-deactivation returns ErrCannotDeactivateSelf", func(t *testing.T) {
		err := checkSelfDeactivation("u1", "u1", "deactivated")
		if !errors.Is(err, ErrCannotDeactivateSelf) {
			t.Errorf("want ErrCannotDeactivateSelf, got %v", err)
		}
	})

	t.Run("different user deactivation returns nil", func(t *testing.T) {
		err := checkSelfDeactivation("u2", "u1", "deactivated")
		if err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("self activation is allowed", func(t *testing.T) {
		err := checkSelfDeactivation("u1", "u1", "active")
		if err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("self status change to non-deactivated returns nil", func(t *testing.T) {
		err := checkSelfDeactivation("u1", "u1", "active")
		if err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})
}

// -- email uniqueness --

func TestCheckEmailUniqueness(t *testing.T) {
	ctx := context.Background()

	t.Run("taken email returns ErrEmailTaken", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			Email:  strptr("taken@example.com"),
			Status: "active",
			Role:   RoleSuperAdmin,
		})
		err := checkEmailUniqueness(ctx, repo, "taken@example.com")
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("want ErrEmailTaken, got %v", err)
		}
	})

	t.Run("available email returns nil", func(t *testing.T) {
		repo := newFakeUserRepo()
		err := checkEmailUniqueness(ctx, repo, "new@example.com")
		if err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("deleted user does not block email", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			Email:  strptr("deleted@example.com"),
			Status: "deleted",
			Role:   RoleSuperAdmin,
		})
		err := checkEmailUniqueness(ctx, repo, "deleted@example.com")
		if err != nil {
			t.Errorf("deleted user should not block email, got %v", err)
		}
	})

	t.Run("case-insensitive match returns ErrEmailTaken", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.seed(&model.User{
			Email:  strptr("Case@Example.com"),
			Status: "active",
			Role:   RoleSuperAdmin,
		})
		// Different case should match normalized
		err := checkEmailUniqueness(ctx, repo, "case@example.com")
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("want ErrEmailTaken for case-insensitive match, got %v", err)
		}
	})
}

func TestAdminAccountResponse_SchoolID(t *testing.T) {
	sid := "s-1"
	row := repository.AdminUserRow{
		ID:       "u1",
		Name:     "Admin",
		Role:     RoleAdminSchool,
		Status:   "active",
		SchoolID: &sid,
	}
	resp := toAdminAccountResponse(row)
	if resp.SchoolID == nil || *resp.SchoolID != "s-1" {
		t.Errorf("SchoolID: want s-1, got %v", resp.SchoolID)
	}
}

func TestAdminAccountResponse_SchoolIDNil(t *testing.T) {
	row := repository.AdminUserRow{
		ID:     "u2",
		Name:   "Super Admin",
		Role:   RoleSuperAdmin,
		Status: "active",
	}
	resp := toAdminAccountResponse(row)
	if resp.SchoolID != nil {
		t.Errorf("SchoolID: want nil for non-admin_school role, got %v", resp.SchoolID)
	}
}

func TestSchoolBindingSentinels(t *testing.T) {
	if ErrSchoolRequired == nil {
		t.Error("ErrSchoolRequired is nil")
	}
	if ErrSchoolNotAllowed == nil {
		t.Error("ErrSchoolNotAllowed is nil")
	}
}

// FR-ACC-03/FR-ACC-04: ChangeAccountRole's school_id binding, exercised through
// the real Service against a valid UUID target (the pre-existing handler tests
// for this used a non-UUID id, so they short-circuited on ErrInvalidUUID before
// ever reaching this validation — a coverage gap, not a behavior defect).
func TestChangeAccountRole_SchoolBinding_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	actorID := uuid.NewString()

	newTarget := func(t *testing.T) string {
		t.Helper()
		email := "acc-" + uniqueSuffix() + "@test.com"
		acc, err := svc.CreateAdminAccount(ctx, actorID, email, "Target Admin", RoleAdminStore, "password123", nil)
		if err != nil {
			t.Fatalf("CreateAdminAccount: %v", err)
		}
		return acc.ID
	}

	newSchool := func(t *testing.T) string {
		t.Helper()
		code := "acc_" + uniqueSuffix()
		school, err := svc.CreateSchool(ctx, "Binding School "+code, code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		return school.ID
	}

	t.Run("schoolID required when role changes to admin_school", func(t *testing.T) {
		targetID := newTarget(t)
		err := svc.ChangeAccountRole(ctx, actorID, targetID, RoleAdminSchool, nil)
		if !errors.Is(err, ErrSchoolRequired) {
			t.Errorf("want ErrSchoolRequired, got %v", err)
		}
	})

	t.Run("valid schoolID sets school_id on the account", func(t *testing.T) {
		targetID := newTarget(t)
		schoolID := newSchool(t)
		if err := svc.ChangeAccountRole(ctx, actorID, targetID, RoleAdminSchool, &schoolID); err != nil {
			t.Fatalf("ChangeAccountRole: %v", err)
		}
		u, err := repo.GetAdminUserByID(ctx, targetID)
		if err != nil {
			t.Fatalf("GetAdminUserByID: %v", err)
		}
		if u.SchoolID == nil || *u.SchoolID != schoolID {
			t.Errorf("SchoolID: want %s, got %v", schoolID, u.SchoolID)
		}
	})

	t.Run("schoolID cleared when role changes away from admin_school", func(t *testing.T) {
		targetID := newTarget(t)
		schoolID := newSchool(t)
		if err := svc.ChangeAccountRole(ctx, actorID, targetID, RoleAdminSchool, &schoolID); err != nil {
			t.Fatalf("ChangeAccountRole (to admin_school): %v", err)
		}
		if err := svc.ChangeAccountRole(ctx, actorID, targetID, RoleAdminStore, nil); err != nil {
			t.Fatalf("ChangeAccountRole (away from admin_school): %v", err)
		}
		u, err := repo.GetAdminUserByID(ctx, targetID)
		if err != nil {
			t.Fatalf("GetAdminUserByID: %v", err)
		}
		if u.SchoolID != nil {
			t.Errorf("SchoolID: want nil after moving away from admin_school, got %v", *u.SchoolID)
		}
	})
}

func TestChangeAccountStatus_ReactivatesDeactivatedAccount(t *testing.T) {
	_, repo := newRealDBService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{
		JWTSecret:       "admin-reactivation-test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
	}
	svc := NewWithStore(
		repo,
		repo,
		rdb,
		infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL),
		&NoopOTPProvider{},
		&NoopEmailProvider{},
		nil,
		nil,
		nil,
		cfg,
		nil,
	)
	ctx := context.Background()
	actorID := uuid.NewString()
	email := "reactivate-" + uniqueSuffix() + "@test.com"

	account, err := svc.CreateAdminAccount(
		ctx,
		actorID,
		email,
		"Reactivation Target",
		RoleAdminStore,
		"password123",
		nil,
	)
	if err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	if err := svc.ChangeAccountStatus(ctx, actorID, account.ID, "deactivated"); err != nil {
		t.Fatalf("ChangeAccountStatus (deactivate): %v", err)
	}
	if err := svc.ChangeAccountStatus(ctx, actorID, account.ID, "active"); err != nil {
		t.Fatalf("ChangeAccountStatus (reactivate): %v", err)
	}

	user, err := repo.GetAdminUserByID(ctx, account.ID)
	if err != nil {
		t.Fatalf("GetAdminUserByID: %v", err)
	}
	if user == nil || user.Status != "active" {
		t.Errorf("want status active after reactivation, got %v", user)
	}
	access, refresh, _, err := svc.Login(ctx, email, "password123")
	if err != nil {
		t.Fatalf("Login after reactivation: %v", err)
	}
	if access == "" || refresh == "" {
		t.Errorf("want session tokens after reactivation, got access=%q refresh=%q", access, refresh)
	}
}
