package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newSessionService(t *testing.T, repo *repository.Repository) *Service {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{JWTSecret: "affiliator-test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 168 * time.Hour}
	return NewWithStore(repo, repo, rdb, infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL),
		&NoopOTPProvider{}, &NoopEmailProvider{}, nil, nil, nil, cfg, nil)
}

func TestNormalizeSchoolIDs(t *testing.T) {
	a, b := uuid.NewString(), uuid.NewString()
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr error
	}{
		{"keeps order and drops duplicates", []string{a, b, a}, []string{a, b}, nil},
		{"trims and lowercases", []string{" " + a + " "}, []string{a}, nil},
		{"blank entries are ignored", []string{"", a}, []string{a}, nil},
		{"empty list is rejected", nil, nil, ErrAffiliatorSchoolsRequired},
		{"only blanks is rejected", []string{" "}, nil, ErrAffiliatorSchoolsRequired},
		{"non uuid is rejected", []string{"nope"}, nil, ErrInvalidUUID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSchoolIDs(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("normalizeSchoolIDs(%v) err = %v, want %v", tt.in, err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("normalizeSchoolIDs(%v) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("normalizeSchoolIDs(%v)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAffiliatorLifecycle_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newSessionService(t, repo)
	ctx := context.Background()
	actorID := uuid.NewString()

	newSchool := func(t *testing.T) string {
		t.Helper()
		code := "aff_" + uniqueSuffix()
		school, err := svc.CreateSchool(ctx, "Affiliate School "+code, code, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		return school.ID
	}
	newAffiliator := func(t *testing.T, schoolIDs ...string) (*AffiliatorResponse, string) {
		t.Helper()
		email := "aff-" + uniqueSuffix() + "@test.com"
		acc, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{Email: email, Name: "Aff", Password: "password123", SchoolIDs: schoolIDs})
		if err != nil {
			t.Fatalf("CreateAffiliator: %v", err)
		}
		return acc, email
	}

	t.Run("create stores role, schools and no single school binding", func(t *testing.T) {
		s1, s2 := newSchool(t), newSchool(t)
		acc, _ := newAffiliator(t, s1, s2, s1)
		u, err := repo.GetAdminUserByID(ctx, acc.ID)
		if err != nil || u == nil {
			t.Fatalf("GetAdminUserByID: %v, %v", u, err)
		}
		if u.Role != RoleAffiliator {
			t.Errorf("role = %q, want affiliator", u.Role)
		}
		if u.SchoolID != nil {
			t.Errorf("school_id = %v, want nil", *u.SchoolID)
		}
		ids, err := repo.GetAffiliatorSchoolIDs(ctx, acc.ID)
		if err != nil {
			t.Fatalf("GetAffiliatorSchoolIDs: %v", err)
		}
		if len(ids) != 2 || ids[0] != s1 || ids[1] != s2 {
			t.Errorf("school_ids = %v, want [%s %s]", ids, s1, s2)
		}
	})

	t.Run("create rejects missing, unknown and deactivated schools", func(t *testing.T) {
		in := AffiliatorInput{Email: "aff-" + uniqueSuffix() + "@test.com", Name: "Aff", Password: "password123"}
		if _, err := svc.CreateAffiliator(ctx, actorID, in); !errors.Is(err, ErrAffiliatorSchoolsRequired) {
			t.Errorf("no schools: err = %v, want ErrAffiliatorSchoolsRequired", err)
		}
		in.SchoolIDs = []string{uuid.NewString()}
		if _, err := svc.CreateAffiliator(ctx, actorID, in); !errors.Is(err, ErrSchoolNotFound) {
			t.Errorf("unknown school: err = %v, want ErrSchoolNotFound", err)
		}
	})

	t.Run("create rejects a taken email", func(t *testing.T) {
		s1 := newSchool(t)
		_, email := newAffiliator(t, s1)
		_, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{Email: email, Name: "Dup", Password: "password123", SchoolIDs: []string{s1}})
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("err = %v, want ErrEmailTaken", err)
		}
	})

	t.Run("bulk reports each row and keeps going after a failure", func(t *testing.T) {
		s1 := newSchool(t)
		good := "aff-" + uniqueSuffix() + "@test.com"
		results, err := svc.BulkCreateAffiliators(ctx, actorID, []AffiliatorInput{
			{Email: good, Name: "Good", Password: "password123", SchoolIDs: []string{s1}},
			{Email: "aff-" + uniqueSuffix() + "@test.com", Name: "Weak", Password: "short", SchoolIDs: []string{s1}},
		})
		if err != nil {
			t.Fatalf("BulkCreateAffiliators: %v", err)
		}
		if len(results) != 2 || !results[0].OK || results[1].OK || results[1].Error == "" {
			t.Errorf("results = %+v, want first ok and second failed with a message", results)
		}
		if _, err := svc.BulkCreateAffiliators(ctx, actorID, nil); !errors.Is(err, ErrMissingField) {
			t.Errorf("empty bulk: err = %v, want ErrMissingField", err)
		}
	})

	t.Run("promote converts only an admin_school and clears its school binding", func(t *testing.T) {
		s1, s2 := newSchool(t), newSchool(t)
		email := "promo-" + uniqueSuffix() + "@test.com"
		admin, err := svc.CreateAdminAccount(ctx, actorID, email, "School Admin", RoleAdminSchool, "password123", &s1)
		if err != nil {
			t.Fatalf("CreateAdminAccount: %v", err)
		}
		if err := svc.PromoteToAffiliator(ctx, actorID, admin.ID, []string{s2}); err != nil {
			t.Fatalf("PromoteToAffiliator: %v", err)
		}
		u, _ := repo.GetAdminUserByID(ctx, admin.ID)
		if u.Role != RoleAffiliator || u.SchoolID != nil {
			t.Errorf("after promote role=%q school_id=%v, want affiliator and nil", u.Role, u.SchoolID)
		}
		ids, _ := repo.GetAffiliatorSchoolIDs(ctx, admin.ID)
		if len(ids) != 1 || ids[0] != s2 {
			t.Errorf("school_ids = %v, want [%s]", ids, s2)
		}
		if err := svc.PromoteToAffiliator(ctx, actorID, admin.ID, []string{s2}); !errors.Is(err, ErrNotAdminSchool) {
			t.Errorf("second promote: err = %v, want ErrNotAdminSchool", err)
		}
		store, _ := svc.CreateAdminAccount(ctx, actorID, "store-"+uniqueSuffix()+"@test.com", "Store", RoleAdminStore, "password123", nil)
		if err := svc.PromoteToAffiliator(ctx, actorID, store.ID, []string{s2}); !errors.Is(err, ErrNotAdminSchool) {
			t.Errorf("promote admin_store: err = %v, want ErrNotAdminSchool", err)
		}
	})

	t.Run("set schools replaces the list and rejects a non affiliator", func(t *testing.T) {
		s1, s2 := newSchool(t), newSchool(t)
		acc, _ := newAffiliator(t, s1)
		if err := svc.SetAffiliatorSchools(ctx, actorID, acc.ID, []string{s2}); err != nil {
			t.Fatalf("SetAffiliatorSchools: %v", err)
		}
		ids, _ := repo.GetAffiliatorSchoolIDs(ctx, acc.ID)
		if len(ids) != 1 || ids[0] != s2 {
			t.Errorf("school_ids = %v, want [%s]", ids, s2)
		}
		store, _ := svc.CreateAdminAccount(ctx, actorID, "store-"+uniqueSuffix()+"@test.com", "Store", RoleAdminStore, "password123", nil)
		if err := svc.SetAffiliatorSchools(ctx, actorID, store.ID, []string{s2}); !errors.Is(err, ErrAffiliatorNotFound) {
			t.Errorf("non affiliator: err = %v, want ErrAffiliatorNotFound", err)
		}
	})

	t.Run("deactivated affiliator can still log in and refresh, others cannot", func(t *testing.T) {
		s1 := newSchool(t)
		acc, email := newAffiliator(t, s1)
		if err := svc.ChangeAccountStatus(ctx, actorID, acc.ID, "deactivated"); err != nil {
			t.Fatalf("deactivate: %v", err)
		}
		active, err := svc.AffiliatorActive(ctx, acc.ID)
		if err != nil || active {
			t.Fatalf("AffiliatorActive = %v, %v, want false", active, err)
		}
		access, refresh, _, err := svc.Login(ctx, email, "password123", "1.1.1.1")
		if err != nil || access == "" {
			t.Fatalf("deactivated affiliator Login: access=%q err=%v, want a session", access, err)
		}
		if _, _, err := svc.Refresh(ctx, refresh); err != nil {
			t.Errorf("deactivated affiliator Refresh: %v, want a session", err)
		}

		storeEmail := "store-" + uniqueSuffix() + "@test.com"
		store, _ := svc.CreateAdminAccount(ctx, actorID, storeEmail, "Store", RoleAdminStore, "password123", nil)
		if err := svc.ChangeAccountStatus(ctx, actorID, store.ID, "deactivated"); err != nil {
			t.Fatalf("deactivate store: %v", err)
		}
		if _, _, _, err := svc.Login(ctx, storeEmail, "password123", "1.1.1.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("deactivated admin_store Login err = %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("list filters by status and profile returns the schools", func(t *testing.T) {
		s1 := newSchool(t)
		acc, _ := newAffiliator(t, s1)
		rows, _, _, err := svc.ListAffiliators(ctx, "active", "", 100, "")
		if err != nil {
			t.Fatalf("ListAffiliators: %v", err)
		}
		found := false
		for _, r := range rows {
			if r.ID == acc.ID {
				found = len(r.SchoolIDs) == 1 && r.SchoolIDs[0] == s1
			}
		}
		if !found {
			t.Errorf("affiliator %s missing from the active list or wrong schools", acc.ID)
		}
		if _, _, _, err := svc.ListAffiliators(ctx, "deleted", "", 10, ""); !errors.Is(err, ErrInvalidStatusFilter) {
			t.Errorf("bad status: err = %v, want ErrInvalidStatusFilter", err)
		}
		p, err := svc.GetAffiliatorProfile(ctx, acc.ID)
		if err != nil || len(p.Schools) != 1 || p.Schools[0].ID != s1 {
			t.Errorf("GetAffiliatorProfile = %+v, %v, want the one school", p, err)
		}
	})
}

func TestAffiliatorStudents_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newSessionService(t, repo)
	ctx := context.Background()
	actorID := uuid.NewString()

	newSchool := func(t *testing.T) (id, code string) {
		t.Helper()
		code = "afs_" + uniqueSuffix()
		school, err := svc.CreateSchool(ctx, "Aff Students "+code, code, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		return school.ID, code
	}
	own, ownCode := newSchool(t)
	other, otherCode := newSchool(t)
	acc, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{
		Email: "afs-" + uniqueSuffix() + "@test.com", Name: "Aff", Password: "password123", SchoolIDs: []string{own},
	})
	if err != nil {
		t.Fatalf("CreateAffiliator: %v", err)
	}

	t.Run("register records registered_by and refuses a foreign school", func(t *testing.T) {
		resp, err := svc.RegisterStudent(ctx, acc.ID, own, "Siswa Satu", "SMA", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}
		mine, _, _, err := svc.ListAffiliatorStudents(ctx, acc.ID, own, true, false, "", "", 100, "", nil, "")
		if err != nil {
			t.Fatalf("ListAffiliatorStudents: %v", err)
		}
		if len(mine) != 1 || mine[0].ID != resp.ID || mine[0].RegisteredBy == nil || *mine[0].RegisteredBy != acc.ID {
			t.Fatalf("registered by me = %+v, want the new student marked %s", mine, acc.ID)
		}
		if _, err := svc.RegisterStudent(ctx, acc.ID, other, "Siswa Dua", "SMA", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); !errors.Is(err, ErrAffiliatorSchoolNotAllowed) {
			t.Errorf("foreign school: err = %v, want ErrAffiliatorSchoolNotAllowed", err)
		}
		if _, err := svc.RegisterStudent(ctx, acc.ID, "", "Siswa Tiga", "SMA", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); !errors.Is(err, ErrAffiliatorSchoolNotAllowed) {
			t.Errorf("no school: err = %v, want ErrAffiliatorSchoolNotAllowed", err)
		}
	})

	t.Run("other actors are not restricted and leave registered_by empty", func(t *testing.T) {
		resp, err := svc.RegisterStudent(ctx, actorID, other, "Siswa Admin", "SMA", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}
		rows, _, _, err := svc.ListStudents(ctx, other, "", "", 100, "", nil, "")
		if err != nil {
			t.Fatalf("ListStudents: %v", err)
		}
		if len(rows) != 1 || rows[0].ID != resp.ID || rows[0].RegisteredBy != nil {
			t.Errorf("rows = %+v, want the new student with registered_by empty", rows)
		}
	})

	t.Run("bulk rows outside the assigned schools fail, inside succeed", func(t *testing.T) {
		rows := []StudentBulkRow{
			{Row: 1, Name: "Bulk Dalam", Jenjang: "SMA", SchoolCode: ownCode},
			{Row: 2, Name: "Bulk Luar", Jenjang: "SMA", SchoolCode: otherCode},
		}
		results, ok, err := svc.ProcessStudentBulkRows(ctx, acc.ID, nil, RoleAffiliator, rows, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if ok != 1 || results[0].Status != "success" || results[1].Status != "failed" || results[1].Error != ErrAffiliatorSchoolNotAllowed.Error() {
			t.Errorf("ok=%d results=%+v, want first success and second refused as not assigned", ok, results)
		}
	})

	t.Run("list covers only assigned schools and can narrow to own registrations", func(t *testing.T) {
		all, _, counts, err := svc.ListAffiliatorStudents(ctx, acc.ID, "", false, false, "", "", 100, "", nil, "")
		if err != nil {
			t.Fatalf("ListAffiliatorStudents: %v", err)
		}
		if len(all) != 2 || counts.Total != 2 {
			t.Errorf("all = %d (count %d), want the 2 students of the assigned school", len(all), counts.Total)
		}
		mine, _, _, _ := svc.ListAffiliatorStudents(ctx, acc.ID, own, true, false, "", "", 100, "", nil, "")
		if len(mine) != 2 {
			t.Errorf("registered by me = %d, want 2 (single and bulk)", len(mine))
		}
		if _, _, _, err := svc.ListAffiliatorStudents(ctx, acc.ID, other, false, false, "", "", 100, "", nil, ""); !errors.Is(err, ErrAffiliatorSchoolNotAllowed) {
			t.Errorf("foreign school filter: err = %v, want ErrAffiliatorSchoolNotAllowed", err)
		}
	})
}

func TestAffiliatorListing_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newSessionService(t, repo)
	ctx := context.Background()
	actorID := uuid.NewString()

	code := "lst_" + uniqueSuffix()
	school, err := svc.CreateSchool(ctx, "Listing School "+code, code, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool: %v", err)
	}
	tag := "Pagetag" + uniqueSuffix()
	var created []string
	for i := 0; i < 3; i++ {
		acc, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{
			Email: fmt.Sprintf("%s-%d@test.com", strings.ToLower(tag), i), Name: fmt.Sprintf("%s %d", tag, i), Password: "password123", SchoolIDs: []string{school.ID},
		})
		if err != nil {
			t.Fatalf("CreateAffiliator: %v", err)
		}
		created = append(created, acc.ID)
	}

	t.Run("search narrows by name or email and reports the total", func(t *testing.T) {
		rows, _, total, err := svc.ListAffiliators(ctx, "", tag, 100, "")
		if err != nil {
			t.Fatalf("ListAffiliators: %v", err)
		}
		if len(rows) != 3 || total != 3 {
			t.Errorf("rows=%d total=%d, want 3 and 3 for the tag %q", len(rows), total, tag)
		}
		byEmail, _, total, _ := svc.ListAffiliators(ctx, "", strings.ToLower(tag)+"-1@", 100, "")
		if len(byEmail) != 1 || total != 1 {
			t.Errorf("email search rows=%d total=%d, want 1 and 1", len(byEmail), total)
		}
		if none, _, total, _ := svc.ListAffiliators(ctx, "", "no-such-"+tag, 100, ""); len(none) != 0 || total != 0 {
			t.Errorf("unmatched search returned %d rows, total %d", len(none), total)
		}
	})

	t.Run("pages follow the cursor without gaps or repeats, newest first", func(t *testing.T) {
		first, next, total, err := svc.ListAffiliators(ctx, "", tag, 2, "")
		if err != nil || len(first) != 2 || next == "" || total != 3 {
			t.Fatalf("page 1 = %d rows, next=%q, total=%d, err=%v; want 2 rows, a cursor and total 3", len(first), next, total, err)
		}
		second, next2, _, err := svc.ListAffiliators(ctx, "", tag, 2, next)
		if err != nil || len(second) != 1 || next2 != "" {
			t.Fatalf("page 2 = %d rows, next=%q, err=%v; want the last row and no cursor", len(second), next2, err)
		}
		seen := map[string]bool{}
		for _, r := range append(first, second...) {
			if seen[r.ID] {
				t.Errorf("row %s appears on two pages", r.ID)
			}
			seen[r.ID] = true
		}
		for _, id := range created {
			if !seen[id] {
				t.Errorf("affiliator %s missing from the pages", id)
			}
		}
		if first[0].ID != created[2] {
			t.Errorf("first row = %s, want the newest (%s)", first[0].ID, created[2])
		}
	})

	t.Run("promotable list holds only active school admins, with their school", func(t *testing.T) {
		admin, err := svc.CreateAdminAccount(ctx, actorID, strings.ToLower(tag)+"-adm@test.com", tag+" Admin", RoleAdminSchool, "password123", &school.ID)
		if err != nil {
			t.Fatalf("CreateAdminAccount: %v", err)
		}
		off, err := svc.CreateAdminAccount(ctx, actorID, strings.ToLower(tag)+"-off@test.com", tag+" Off", RoleAdminSchool, "password123", &school.ID)
		if err != nil {
			t.Fatalf("CreateAdminAccount: %v", err)
		}
		if err := svc.ChangeAccountStatus(ctx, actorID, off.ID, "deactivated"); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		rows, _, total, err := svc.ListPromotableAdmins(ctx, tag, 50, "")
		if err != nil {
			t.Fatalf("ListPromotableAdmins: %v", err)
		}
		if len(rows) != 1 || total != 1 || rows[0].ID != admin.ID {
			t.Fatalf("promotable = %+v (total %d), want only the active admin", rows, total)
		}
		if rows[0].SchoolID == nil || *rows[0].SchoolID != school.ID || rows[0].SchoolName == nil || *rows[0].SchoolName != school.Name {
			t.Errorf("school = %v / %v, want %s / %s", rows[0].SchoolID, rows[0].SchoolName, school.ID, school.Name)
		}
		bySchool, _, _, _ := svc.ListPromotableAdmins(ctx, school.Name, 50, "")
		if len(bySchool) != 1 {
			t.Errorf("search by school name returned %d rows, want 1", len(bySchool))
		}

		if err := svc.PromoteToAffiliator(ctx, actorID, admin.ID, []string{school.ID}); err != nil {
			t.Fatalf("PromoteToAffiliator: %v", err)
		}
		after, _, total, _ := svc.ListPromotableAdmins(ctx, tag, 50, "")
		if len(after) != 0 || total != 0 {
			t.Errorf("a promoted admin is still promotable: %+v", after)
		}
	})
}
