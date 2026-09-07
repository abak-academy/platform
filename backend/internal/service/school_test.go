package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeSchoolNPSN(t *testing.T) {
	tests := []struct {
		name string
		in   *string
		want *string
		err  error
	}{
		{name: "omitted remains null"},
		{name: "blank becomes null", in: stringPtr(" \t ")},
		{name: "numeric accepted", in: stringPtr(" 20101234 "), want: stringPtr("20101234")},
		{name: "letter prefixed accepted and uppercased", in: stringPtr(" p1234567 "), want: stringPtr("P1234567")},
		{name: "too short rejected", in: stringPtr("1234567"), err: ErrInvalidSchoolNPSN},
		{name: "too long rejected", in: stringPtr("123456789"), err: ErrInvalidSchoolNPSN},
		{name: "punctuation rejected", in: stringPtr("1234-678"), err: ErrInvalidSchoolNPSN},
		{name: "non ASCII rejected", in: stringPtr("É1234567"), err: ErrInvalidSchoolNPSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSchoolNPSN(tt.in)
			if !errors.Is(err, tt.err) {
				t.Fatalf("normalizeSchoolNPSN: want error %v, got %v", tt.err, err)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("want nil, got %q", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("want %q, got %v", *tt.want, got)
			}
		})
	}
}

func TestMapSchoolWriteError_OnlyNamedUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "named NPSN unique violation",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: schoolNPSNUniqueIndex},
			want: ErrSchoolNPSNTaken,
		},
		{
			name: "different unique violation unchanged",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "school_code_key"},
		},
		{
			name: "named constraint with different SQLSTATE unchanged",
			err:  &pgconn.PgError{Code: "23514", ConstraintName: schoolNPSNUniqueIndex},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapSchoolWriteError(tt.err)
			if tt.want != nil {
				if !errors.Is(got, tt.want) {
					t.Fatalf("want %v, got %v", tt.want, got)
				}
				return
			}
			if got != tt.err {
				t.Fatalf("want original error, got %v", got)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }

// findSchool pages through AdminListSchools looking for a school by ID. The
// real-DB fixture is shared across every test in this package, so a single
// page isn't guaranteed to contain a school created by this test.
func findSchool(t *testing.T, svc *Service, id string) SchoolResponse {
	t.Helper()
	ctx := context.Background()
	cursor := ""
	for {
		rows, next, _, err := svc.AdminListSchools(ctx, AdminListSchoolsParams{Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatalf("AdminListSchools: %v", err)
		}
		for _, r := range rows {
			if r.ID == id {
				return r
			}
		}
		if next == "" {
			t.Fatalf("school %s not found in AdminListSchools", id)
		}
		cursor = next
	}
}

func findSchoolByCode(t *testing.T, svc *Service, code string) SchoolResponse {
	t.Helper()
	ctx := context.Background()
	cursor := ""
	for {
		rows, next, _, err := svc.AdminListSchools(ctx, AdminListSchoolsParams{Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatalf("AdminListSchools: %v", err)
		}
		for _, r := range rows {
			if r.Code == code {
				return r
			}
		}
		if next == "" {
			t.Fatalf("school with code %s not found in AdminListSchools", code)
		}
		cursor = next
	}
}

func TestCreateSchool_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	t.Run("happy path creates active school with zero student count", func(t *testing.T) {
		code := "cs_" + uniqueSuffix()
		npsn := "20000001"
		alamat := "Jl. Test No.1"
		resp, err := svc.CreateSchool(ctx, "Test School "+code, code, &npsn, []string{"SMA"}, &alamat)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		if resp.Status != "active" {
			t.Errorf("Status: want active, got %q", resp.Status)
		}
		if resp.StudentCount != 0 {
			t.Errorf("StudentCount: want 0, got %d", resp.StudentCount)
		}
		if resp.ID == "" {
			t.Error("want non-empty ID")
		}
		if resp.Code != code {
			t.Errorf("Code: want %s, got %s", code, resp.Code)
		}
	})

	t.Run("omitted school_types defaults to empty slice not null", func(t *testing.T) {
		code := "cs_" + uniqueSuffix()
		resp, err := svc.CreateSchool(ctx, "No Types School "+code, code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool with omitted school_types: %v", err)
		}
		if resp.SchoolTypes == nil {
			t.Error("SchoolTypes: want empty slice, got nil")
		}
		if len(resp.SchoolTypes) != 0 {
			t.Errorf("SchoolTypes: want empty, got %v", resp.SchoolTypes)
		}

		// Persisted row must also carry {} not NULL, and remain listable.
		found := findSchool(t, svc, resp.ID)
		if found.SchoolTypes == nil || len(found.SchoolTypes) != 0 {
			t.Errorf("persisted SchoolTypes: want empty slice, got %v", found.SchoolTypes)
		}
	})

	t.Run("missing name returns ErrInvalidSchoolName", func(t *testing.T) {
		_, err := svc.CreateSchool(ctx, "", "somecode", nil, nil, nil)
		if !errors.Is(err, ErrInvalidSchoolName) {
			t.Errorf("want ErrInvalidSchoolName, got %v", err)
		}
	})

	t.Run("missing code returns ErrMissingField", func(t *testing.T) {
		_, err := svc.CreateSchool(ctx, "Some School", "", nil, nil, nil)
		if !errors.Is(err, ErrMissingField) {
			t.Errorf("want ErrMissingField, got %v", err)
		}
	})

	t.Run("rejects names without letters or digits", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			want error
		}{
			{name: "", want: ErrInvalidSchoolName},
			{name: "   ", want: ErrInvalidSchoolName},
			{name: " ...--- ", want: ErrInvalidSchoolName},
		} {
			_, err := svc.CreateSchool(ctx, tc.name, "cs_"+uniqueSuffix(), nil, nil, nil)
			if !errors.Is(err, tc.want) {
				t.Errorf("CreateSchool(%q): want %v, got %v", tc.name, tc.want, err)
			}
		}
	})

	t.Run("accepts names with digits punctuation and unicode letters", func(t *testing.T) {
		for _, name := range []string{"12345", "SMA Harapan-1", "Al-Ma'ruf", "École Internationale", "東京学園"} {
			code := "cs_" + uniqueSuffix()
			resp, err := svc.CreateSchool(ctx, name, code, nil, nil, nil)
			if err != nil {
				t.Fatalf("CreateSchool(%q): %v", name, err)
			}
			if resp.Name != name {
				t.Errorf("Name: want %q, got %q", name, resp.Name)
			}
		}
	})

	t.Run("duplicate code returns ErrSchoolCodeTaken", func(t *testing.T) {
		code := "cs_" + uniqueSuffix()
		if _, err := svc.CreateSchool(ctx, "First", code, nil, nil, nil); err != nil {
			t.Fatalf("CreateSchool (first): %v", err)
		}
		_, err := svc.CreateSchool(ctx, "Second", code, nil, nil, nil)
		if !errors.Is(err, ErrSchoolCodeTaken) {
			t.Errorf("want ErrSchoolCodeTaken, got %v", err)
		}
	})

	t.Run("normalizes NPSN and stores blank as null", func(t *testing.T) {
		code := "cs_" + uniqueSuffix()
		npsn := " p1234567 "
		created, err := svc.CreateSchool(ctx, "Normalized NPSN School", code, &npsn, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		if created.NPSN == nil || *created.NPSN != "P1234567" {
			t.Fatalf("NPSN: want P1234567, got %v", created.NPSN)
		}

		blank := "  "
		cleared, err := svc.UpdateSchool(ctx, created.ID, nil, &blank, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdateSchool blank NPSN: %v", err)
		}
		if cleared.NPSN != nil {
			t.Fatalf("NPSN: want nil after blank update, got %q", *cleared.NPSN)
		}
	})

	t.Run("rejects malformed NPSN before create", func(t *testing.T) {
		code := "cs_" + uniqueSuffix()
		invalid := "1234-678"
		_, err := svc.CreateSchool(ctx, "Invalid NPSN School", code, &invalid, nil, nil)
		if !errors.Is(err, ErrInvalidSchoolNPSN) {
			t.Fatalf("want ErrInvalidSchoolNPSN, got %v", err)
		}
		for _, row := range mustListSchools(t, svc) {
			if row.Code == code {
				t.Fatalf("school %q was written despite invalid NPSN", code)
			}
		}
	})

	t.Run("rejects duplicate normalized NPSN and allows multiple nulls", func(t *testing.T) {
		npsn := "Q1234567"
		if _, err := svc.CreateSchool(ctx, "First NPSN", "cs_"+uniqueSuffix(), &npsn, nil, nil); err != nil {
			t.Fatalf("CreateSchool first: %v", err)
		}
		duplicate := " q1234567 "
		_, err := svc.CreateSchool(ctx, "Duplicate NPSN", "cs_"+uniqueSuffix(), &duplicate, nil, nil)
		if !errors.Is(err, ErrSchoolNPSNTaken) {
			t.Fatalf("want ErrSchoolNPSNTaken, got %v", err)
		}
		for _, name := range []string{"Null NPSN One", "Null NPSN Two"} {
			if _, err := svc.CreateSchool(ctx, name, "cs_"+uniqueSuffix(), nil, nil, nil); err != nil {
				t.Fatalf("CreateSchool %q with nil NPSN: %v", name, err)
			}
		}
	})
}

func mustListSchools(t *testing.T, svc *Service) []SchoolResponse {
	t.Helper()
	var all []SchoolResponse
	cursor := ""
	for {
		rows, next, _, err := svc.AdminListSchools(context.Background(), AdminListSchoolsParams{Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatalf("AdminListSchools: %v", err)
		}
		all = append(all, rows...)
		if next == "" {
			return all
		}
		cursor = next
	}
}

func TestUpdateSchool_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	t.Run("happy path patches fields", func(t *testing.T) {
		code := "us_" + uniqueSuffix()
		created, err := svc.CreateSchool(ctx, "Before Update", code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		newName := "After Update"
		updated, err := svc.UpdateSchool(ctx, created.ID, &newName, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdateSchool: %v", err)
		}
		if updated.Name != newName {
			t.Errorf("Name: want %s, got %s", newName, updated.Name)
		}
		if updated.Code != code {
			t.Errorf("Code should be unchanged: want %s, got %s", code, updated.Code)
		}
	})

	t.Run("omitted name leaves existing name unchanged", func(t *testing.T) {
		code := "us_" + uniqueSuffix()
		created, err := svc.CreateSchool(ctx, "Name Stays", code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		alamat := "Jl. Updated"
		updated, err := svc.UpdateSchool(ctx, created.ID, nil, nil, &alamat, nil, nil)
		if err != nil {
			t.Fatalf("UpdateSchool: %v", err)
		}
		if updated.Name != created.Name {
			t.Errorf("Name: want unchanged %q, got %q", created.Name, updated.Name)
		}
		if updated.Alamat == nil || *updated.Alamat != alamat {
			t.Errorf("Alamat: want %q, got %v", alamat, updated.Alamat)
		}
	})

	t.Run("normalizes NPSN on update", func(t *testing.T) {
		code := "us_" + uniqueSuffix()
		created, err := svc.CreateSchool(ctx, "Normalize Update", code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		npsn := " u" + uniqueSuffix()[:7] + " "
		updated, err := svc.UpdateSchool(ctx, created.ID, nil, &npsn, nil, nil, nil)
		if err != nil {
			t.Fatalf("UpdateSchool: %v", err)
		}
		want := strings.ToUpper(strings.TrimSpace(npsn))
		if updated.NPSN == nil || *updated.NPSN != want {
			t.Fatalf("NPSN: want %q, got %v", want, updated.NPSN)
		}
		persisted := findSchool(t, svc, created.ID)
		if persisted.NPSN == nil || *persisted.NPSN != want {
			t.Fatalf("persisted NPSN: want %q, got %v", want, persisted.NPSN)
		}
	})

	t.Run("rejects malformed NPSN before update and preserves the row", func(t *testing.T) {
		code := "us_" + uniqueSuffix()
		npsn := "V" + uniqueSuffix()[:7]
		created, err := svc.CreateSchool(ctx, "Invalid NPSN Update", code, &npsn, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		invalid := "1234-678"
		_, err = svc.UpdateSchool(ctx, created.ID, nil, &invalid, nil, nil, nil)
		if !errors.Is(err, ErrInvalidSchoolNPSN) {
			t.Fatalf("want ErrInvalidSchoolNPSN, got %v", err)
		}
		persisted := findSchool(t, svc, created.ID)
		if persisted.NPSN == nil || *persisted.NPSN != *created.NPSN {
			t.Fatalf("NPSN changed after rejected update: want %v, got %v", created.NPSN, persisted.NPSN)
		}
	})

	t.Run("rejects duplicate normalized NPSN on update and preserves the row", func(t *testing.T) {
		takenNPSN := "W" + uniqueSuffix()[:7]
		taken, err := svc.CreateSchool(ctx, "Taken NPSN", "us_"+uniqueSuffix(), &takenNPSN, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool taken: %v", err)
		}
		originalNPSN := "X" + uniqueSuffix()[:7]
		target, err := svc.CreateSchool(ctx, "Duplicate Update Target", "us_"+uniqueSuffix(), &originalNPSN, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool target: %v", err)
		}
		duplicate := " " + strings.ToLower(*taken.NPSN) + " "
		_, err = svc.UpdateSchool(ctx, target.ID, nil, &duplicate, nil, nil, nil)
		if !errors.Is(err, ErrSchoolNPSNTaken) {
			t.Fatalf("want ErrSchoolNPSNTaken, got %v", err)
		}
		persisted := findSchool(t, svc, target.ID)
		if persisted.NPSN == nil || *persisted.NPSN != *target.NPSN {
			t.Fatalf("NPSN changed after duplicate update: want %v, got %v", target.NPSN, persisted.NPSN)
		}
	})

	t.Run("invalid name is rejected and row remains unchanged", func(t *testing.T) {
		code := "us_" + uniqueSuffix()
		created, err := svc.CreateSchool(ctx, "Still Valid", code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		invalidName := "..."
		newCode := "us_" + uniqueSuffix()
		_, err = svc.UpdateSchool(ctx, created.ID, &invalidName, nil, nil, nil, &newCode)
		if !errors.Is(err, ErrInvalidSchoolName) {
			t.Fatalf("want ErrInvalidSchoolName, got %v", err)
		}
		found := findSchool(t, svc, created.ID)
		if found.Name != created.Name {
			t.Errorf("Name changed after rejected update: want %q, got %q", created.Name, found.Name)
		}
		if found.Code != code {
			t.Errorf("Code changed after rejected update: want %q, got %q", code, found.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		newName := "Doesn't Matter"
		_, err := svc.UpdateSchool(ctx, "00000000-0000-0000-0000-000000000000", &newName, nil, nil, nil, nil)
		if !errors.Is(err, ErrSchoolNotFound) {
			t.Errorf("want ErrSchoolNotFound, got %v", err)
		}
	})

	t.Run("code uniqueness on update", func(t *testing.T) {
		codeA := "us_" + uniqueSuffix()
		codeB := "us_" + uniqueSuffix()
		if _, err := svc.CreateSchool(ctx, "School A", codeA, nil, nil, nil); err != nil {
			t.Fatalf("CreateSchool A: %v", err)
		}
		schoolB, err := svc.CreateSchool(ctx, "School B", codeB, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool B: %v", err)
		}
		_, err = svc.UpdateSchool(ctx, schoolB.ID, nil, nil, nil, nil, &codeA)
		if !errors.Is(err, ErrSchoolCodeTaken) {
			t.Errorf("want ErrSchoolCodeTaken, got %v", err)
		}
	})

	t.Run("code change succeeds when students exist (lock removed)", func(t *testing.T) {
		code := "us_" + uniqueSuffix()
		npsn := "S" + uniqueSuffix()[:7]
		school, err := svc.CreateSchool(ctx, "School With Students", code, &npsn, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		if _, err := svc.RegisterStudent(ctx, school.ID, "Stu Dent", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}
		newCode := "us_" + uniqueSuffix()
		updated, err := svc.UpdateSchool(ctx, school.ID, nil, nil, nil, nil, &newCode)
		if err != nil {
			t.Errorf("code change should succeed (lock removed), got %v", err)
		}
		if updated != nil && updated.Code != newCode {
			t.Errorf("Code: want %s, got %s", newCode, updated.Code)
		}
	})

	t.Run("code uniqueness still enforced on update", func(t *testing.T) {
		codeA := "us_" + uniqueSuffix()
		codeB := "us_" + uniqueSuffix()
		_, err := svc.CreateSchool(ctx, "School A", codeA, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool A: %v", err)
		}
		npsn := "S" + uniqueSuffix()[:7]
		schoolB, err := svc.CreateSchool(ctx, "School B", codeB, &npsn, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool B: %v", err)
		}
		if _, err := svc.RegisterStudent(ctx, schoolB.ID, "Stu Dent", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}
		_, err = svc.UpdateSchool(ctx, schoolB.ID, nil, nil, nil, nil, &codeA)
		if !errors.Is(err, ErrSchoolCodeTaken) {
			t.Errorf("want ErrSchoolCodeTaken, got %v", err)
		}
	})
}

func TestChangeSchoolStatus_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	t.Run("happy path toggles status", func(t *testing.T) {
		code := "st_" + uniqueSuffix()
		school, err := svc.CreateSchool(ctx, "Status School", code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		updated, err := svc.ChangeSchoolStatus(ctx, school.ID, "deactivated")
		if err != nil {
			t.Fatalf("ChangeSchoolStatus: %v", err)
		}
		if updated.Status != "deactivated" {
			t.Errorf("Status: want deactivated, got %s", updated.Status)
		}
		updated, err = svc.ChangeSchoolStatus(ctx, school.ID, "active")
		if err != nil {
			t.Fatalf("ChangeSchoolStatus back to active: %v", err)
		}
		if updated.Status != "active" {
			t.Errorf("Status: want active, got %s", updated.Status)
		}
	})

	t.Run("invalid status value", func(t *testing.T) {
		code := "st_" + uniqueSuffix()
		school, err := svc.CreateSchool(ctx, "Invalid Status School", code, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		_, err = svc.ChangeSchoolStatus(ctx, school.ID, "pending")
		if !errors.Is(err, ErrInvalidStatusFilter) {
			t.Errorf("want ErrInvalidStatusFilter, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.ChangeSchoolStatus(ctx, "00000000-0000-0000-0000-000000000000", "active")
		if !errors.Is(err, ErrSchoolNotFound) {
			t.Errorf("want ErrSchoolNotFound, got %v", err)
		}
	})
}

func TestAdminListSchools_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	code := "ls_" + uniqueSuffix()
	name := "Listable School " + code
	npsn := "S" + uniqueSuffix()[:7]
	school, err := svc.CreateSchool(ctx, name, code, &npsn, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.RegisterStudent(ctx, school.ID, "Stu Dent", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}
	}

	found := findSchool(t, svc, school.ID)
	if found.Name != name {
		t.Errorf("Name: want %s, got %s", name, found.Name)
	}
	if found.StudentCount != 2 {
		t.Errorf("StudentCount: want 2, got %d", found.StudentCount)
	}
}

func TestAdminListSchools_QAndStatusFilter_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	suffix := uniqueSuffix()
	q := "qfilt_" + suffix
	active, err := svc.CreateSchool(ctx, "Active "+q, "qa_"+suffix, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool active: %v", err)
	}
	deactivated, err := svc.CreateSchool(ctx, "Deactivated "+q, "qd_"+suffix, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool deactivated: %v", err)
	}
	if _, err := svc.ChangeSchoolStatus(ctx, deactivated.ID, "deactivated"); err != nil {
		t.Fatalf("ChangeSchoolStatus: %v", err)
	}

	rows, _, counts, err := svc.AdminListSchools(ctx, AdminListSchoolsParams{Limit: 100, Q: q})
	if err != nil {
		t.Fatalf("AdminListSchools with q: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows matching q=%q, got %d", q, len(rows))
	}
	if counts.Total != 2 || counts.Active != 1 {
		t.Errorf("counts: want total=2 active=1, got %+v", counts)
	}

	rows, _, _, err = svc.AdminListSchools(ctx, AdminListSchoolsParams{Limit: 100, Q: q, Status: "active"})
	if err != nil {
		t.Fatalf("AdminListSchools with status=active: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != active.ID {
		t.Errorf("want only the active school, got %+v", rows)
	}

	if _, _, _, err := svc.AdminListSchools(ctx, AdminListSchoolsParams{Status: "pending"}); !errors.Is(err, ErrInvalidStatusFilter) {
		t.Errorf("want ErrInvalidStatusFilter for status=pending, got %v", err)
	}
}

func TestSchoolOptions_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	suffix := uniqueSuffix()
	active, err := svc.CreateSchool(ctx, "Option Active "+suffix, "oa_"+suffix, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool: %v", err)
	}
	deactivated, err := svc.CreateSchool(ctx, "Option Deactivated "+suffix, "od_"+suffix, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateSchool: %v", err)
	}
	if _, err := svc.ChangeSchoolStatus(ctx, deactivated.ID, "deactivated"); err != nil {
		t.Fatalf("ChangeSchoolStatus: %v", err)
	}

	options, err := svc.SchoolOptions(ctx)
	if err != nil {
		t.Fatalf("SchoolOptions: %v", err)
	}
	var sawActive, sawDeactivated bool
	for _, o := range options {
		if o.ID == active.ID {
			sawActive = true
		}
		if o.ID == deactivated.ID {
			sawDeactivated = true
		}
	}
	if !sawActive {
		t.Error("want active school in options")
	}
	if sawDeactivated {
		t.Error("deactivated school should not be in options")
	}
}

func TestSchoolSentinelErrors(t *testing.T) {
	if ErrSchoolNotFound == nil {
		t.Error("ErrSchoolNotFound is nil")
	}
	if ErrSchoolCodeTaken == nil {
		t.Error("ErrSchoolCodeTaken is nil")
	}
}

func TestSchoolResponseMapping(t *testing.T) {
	row := repository.SchoolAdminRow{
		School: model.School{
			ID:   "s1",
			Name: "Test School",
			Code: "test",
		},
		StudentCount: 5,
	}
	resp := toSchoolResponse(row)
	if resp.ID != "s1" {
		t.Errorf("ID: want s1, got %s", resp.ID)
	}
	if resp.Name != "Test School" {
		t.Errorf("Name: want Test School, got %s", resp.Name)
	}
	if resp.Code != "test" {
		t.Errorf("Code: want test, got %s", resp.Code)
	}
	if resp.StudentCount != 5 {
		t.Errorf("StudentCount: want 5, got %d", resp.StudentCount)
	}
}
