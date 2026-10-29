package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

func TestAffiliatorSchoolList_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newSessionService(t, repo)
	ctx := context.Background()
	actorID := uuid.NewString()
	tag := "Asl" + uniqueSuffix()

	type school struct{ id, name, code, npsn string }
	newSchool := func(label string) school {
		t.Helper()
		code := strings.ToLower(tag) + label
		npsn := "L" + uniqueSuffix()[:7]
		name := "School " + tag + " " + label
		sch, err := svc.CreateSchool(ctx, name, code, &npsn, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		return school{sch.ID, name, code, *sch.NPSN}
	}
	s1, s2 := newSchool("A"), newSchool("B")

	newAff := func(label string, schools ...string) string {
		t.Helper()
		acc, err := svc.CreateAffiliator(ctx, actorID, AffiliatorInput{
			Email: strings.ToLower(tag) + "-" + strings.ToLower(label) + "@test.com", Name: tag + " " + label, Password: "password123", SchoolIDs: schools,
		})
		if err != nil {
			t.Fatalf("CreateAffiliator: %v", err)
		}
		return acc.ID
	}
	a1, a2, a3 := newAff("One", s1.id, s2.id), newAff("Two", s1.id), newAff("Three", s2.id)
	if err := svc.ChangeAccountStatus(ctx, actorID, a3, "deactivated"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := repo.Pool().Exec(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang) VALUES ('Siswa', $1, 'student', $2, $3, '', 'sma')`,
			s1.id, map[bool]string{true: "deleted", false: "active"}[i == 2], "aslstu_"+uniqueSuffix()); err != nil {
			t.Fatalf("insert student: %v", err)
		}
	}
	// A deleted affiliator, and a school id that no longer exists, must not show.
	a4 := newAff("Four", s1.id)
	if _, err := repo.Pool().Exec(ctx, `UPDATE users SET status = 'deleted' WHERE id = $1`, a4); err != nil {
		t.Fatalf("delete affiliator: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE users SET school_ids = school_ids || $2::uuid WHERE id = $1`, a2, uuid.NewString()); err != nil {
		t.Fatalf("add orphan school id: %v", err)
	}

	list := func(q repository.AffiliatorSchoolQuery) ([]repository.AffiliatorSchoolRow, int) {
		t.Helper()
		if q.Q == "" {
			q.Q = tag
		}
		rows, total, err := svc.ListAffiliatorSchools(ctx, q)
		if err != nil {
			t.Fatalf("ListAffiliatorSchools: %v", err)
		}
		return rows, total
	}
	pairs := func(rows []repository.AffiliatorSchoolRow) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = strings.TrimPrefix(r.AffiliatorName, tag+" ") + "@" + strings.TrimPrefix(r.SchoolName, "School "+tag+" ")
		}
		return out
	}
	equal := func(got, want []string) bool { return strings.Join(got, ",") == strings.Join(want, ",") }

	t.Run("one row per assignment, by school then affiliator, without deleted accounts or missing schools", func(t *testing.T) {
		rows, total := list(repository.AffiliatorSchoolQuery{})
		want := []string{"One@A", "Two@A", "One@B", "Three@B"}
		if total != 4 || !equal(pairs(rows), want) {
			t.Errorf("pairs = %v (total %d), want %v", pairs(rows), total, want)
		}
	})

	t.Run("carries the affiliator's state, the school's identifiers and its active student count", func(t *testing.T) {
		rows, _ := list(repository.AffiliatorSchoolQuery{AffiliatorID: a3})
		if len(rows) != 1 {
			t.Fatalf("rows = %+v, want one", rows)
		}
		r := rows[0]
		if r.AffiliatorStatus != "deactivated" || r.SchoolID != s2.id || r.SchoolCode != s2.code || r.SchoolNPSN == nil || *r.SchoolNPSN != s2.npsn || r.StudentCount != 0 {
			t.Errorf("row = %+v, want a deactivated affiliator at school B with no students", r)
		}
		a, _ := list(repository.AffiliatorSchoolQuery{SchoolID: s1.id, AffiliatorID: a1})
		if len(a) != 1 || a[0].StudentCount != 2 {
			t.Errorf("school A row = %+v, want 2 students (the deleted one is not counted)", a)
		}
	})

	t.Run("filters by affiliator, by school, and by both", func(t *testing.T) {
		byAff, total := list(repository.AffiliatorSchoolQuery{AffiliatorID: a1})
		if total != 2 || !equal(pairs(byAff), []string{"One@A", "One@B"}) {
			t.Errorf("by affiliator = %v (total %d), want both schools of One", pairs(byAff), total)
		}
		bySchool, total := list(repository.AffiliatorSchoolQuery{SchoolID: s1.id})
		if total != 2 || !equal(pairs(bySchool), []string{"One@A", "Two@A"}) {
			t.Errorf("by school = %v (total %d), want the two affiliators of A", pairs(bySchool), total)
		}
		both, total := list(repository.AffiliatorSchoolQuery{AffiliatorID: a2, SchoolID: s1.id})
		if total != 1 || !equal(pairs(both), []string{"Two@A"}) {
			t.Errorf("both = %v (total %d), want one pair", pairs(both), total)
		}
		none, total := list(repository.AffiliatorSchoolQuery{AffiliatorID: a2, SchoolID: s2.id})
		if total != 0 || len(none) != 0 {
			t.Errorf("an unassigned pair = %v, want none", pairs(none))
		}
	})

	t.Run("search reaches the affiliator's name and email and the school's name, code and NPSN", func(t *testing.T) {
		for name, q := range map[string]string{
			"affiliator name":  tag + " Three",
			"affiliator email": strings.ToLower(tag) + "-three@",
			"school name":      "School " + tag + " B",
			"school code":      s2.code,
			"school npsn":      s2.npsn,
		} {
			rows, _, err := svc.ListAffiliatorSchools(ctx, repository.AffiliatorSchoolQuery{Q: q, Limit: 50})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			found := false
			for _, r := range rows {
				found = found || (r.AffiliatorID == a3 && r.SchoolID == s2.id)
			}
			if !found {
				t.Errorf("search by %s (%q) did not find Three@B", name, q)
			}
		}
		if rows, total := list(repository.AffiliatorSchoolQuery{Q: "no-such-" + tag}); len(rows) != 0 || total != 0 {
			t.Errorf("unmatched search returned %v", pairs(rows))
		}
	})

	t.Run("pages keep the total and do not repeat rows", func(t *testing.T) {
		first, total := list(repository.AffiliatorSchoolQuery{Limit: 3})
		second, _ := list(repository.AffiliatorSchoolQuery{Limit: 3, Offset: 3})
		all := append(pairs(first), pairs(second)...)
		if total != 4 || len(first) != 3 || len(second) != 1 || !equal(all, []string{"One@A", "Two@A", "One@B", "Three@B"}) {
			t.Errorf("pages = %v + %v (total %d)", pairs(first), pairs(second), total)
		}
	})

	t.Run("a malformed id is rejected rather than reaching the database", func(t *testing.T) {
		for _, q := range []repository.AffiliatorSchoolQuery{{AffiliatorID: "nope"}, {SchoolID: "1234"}} {
			if _, _, err := svc.ListAffiliatorSchools(ctx, q); !errors.Is(err, ErrInvalidUUID) {
				t.Errorf("query %+v: err = %v, want ErrInvalidUUID", q, err)
			}
			if _, err := svc.ExportAffiliatorSchools(ctx, q); !errors.Is(err, ErrInvalidUUID) {
				t.Errorf("export %+v: err = %v, want ErrInvalidUUID", q, err)
			}
		}
	})

	t.Run("export is the same list, whole, as CSV with a BOM", func(t *testing.T) {
		data, err := svc.ExportAffiliatorSchools(ctx, repository.AffiliatorSchoolQuery{Q: tag, Limit: 1})
		if err != nil {
			t.Fatalf("ExportAffiliatorSchools: %v", err)
		}
		if !strings.HasPrefix(string(data), "\xEF\xBB\xBF") {
			t.Error("export lacks the UTF-8 BOM")
		}
		lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(data), "\xEF\xBB\xBF")), "\n")
		if lines[0] != "affiliator,email,status affiliator,sekolah,NPSN,kode sekolah,status sekolah,siswa" {
			t.Errorf("header = %q", lines[0])
		}
		if len(lines) != 5 {
			t.Errorf("export has %d lines, want a header and all 4 assignments regardless of the page size", len(lines))
		}
		if !strings.Contains(lines[1], tag+" One") || !strings.Contains(lines[1], s1.npsn) || !strings.Contains(lines[1], ",2") {
			t.Errorf("first row = %q, want One at school A with its NPSN and 2 students", lines[1])
		}

		filtered, _ := svc.ExportAffiliatorSchools(ctx, repository.AffiliatorSchoolQuery{AffiliatorID: a3})
		if got := strings.Count(strings.TrimSpace(string(filtered)), "\n"); got != 1 {
			t.Errorf("a filtered export has %d data rows, want 1", got)
		}
	})
}
