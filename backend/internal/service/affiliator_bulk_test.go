package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseAffiliatorBulkCSV(t *testing.T) {
	t.Run("reads one school per line by column name, in any order", func(t *testing.T) {
		rows, err := ParseAffiliatorBulkCSV([]byte("\xEF\xBB\xBFemail,Name,school_code,school_npsn,password\n" +
			"a@example.com,Andi,KODEA,12345678,secret123\n" +
			"\n" +
			"a@example.com,Andi,KODEB,,\n"))
		if err != nil {
			t.Fatalf("ParseAffiliatorBulkCSV: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2 (blank line skipped)", len(rows))
		}
		a, b := rows[0], rows[1]
		if a.Row != 2 || a.Name != "Andi" || a.Email != "a@example.com" || a.Password != "secret123" || a.SchoolCode != "KODEA" || a.SchoolNPSN != "12345678" {
			t.Errorf("row a = %+v", a)
		}
		if b.Row != 3 || b.Password != "" || b.SchoolCode != "KODEB" || b.SchoolNPSN != "" {
			t.Errorf("row b = %+v, want record 3 (blank lines are skipped), no password, code only", b)
		}
	})

	t.Run("does not split a cell on | or commas: that is no longer a list", func(t *testing.T) {
		rows, err := ParseAffiliatorBulkCSV([]byte("name,email,school_code\nAndi,a@example.com,KODEA|KODEB\n"))
		if err != nil || len(rows) != 1 || rows[0].SchoolCode != "KODEA|KODEB" {
			t.Errorf("rows=%+v err=%v, want the cell kept whole so it fails as an unknown school", rows, err)
		}
	})

	t.Run("the password column is optional", func(t *testing.T) {
		rows, err := ParseAffiliatorBulkCSV([]byte("name,email,school_code\nAndi,a@example.com,KODEA\n"))
		if err != nil || len(rows) != 1 || rows[0].Password != "" {
			t.Errorf("rows=%+v err=%v, want one row with no password", rows, err)
		}
	})

	for name, csv := range map[string]string{
		"no name":          "email,school_code\na@example.com,K\n",
		"no email":         "name,school_code\nAndi,K\n",
		"no school column": "name,email,password\nAndi,a@example.com,secret123\n",
		"empty file":       "",
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			if _, err := ParseAffiliatorBulkCSV([]byte(csv)); !errors.Is(err, ErrMissingAffiliatorCSVHeader) {
				t.Errorf("err = %v, want ErrMissingAffiliatorCSVHeader", err)
			}
		})
	}

	t.Run("rejects a file over the row limit", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("name,email,school_code\n")
		for i := 0; i <= maxBulkRows; i++ {
			sb.WriteString("A,a@example.com,K\n")
		}
		if _, err := ParseAffiliatorBulkCSV([]byte(sb.String())); !errors.Is(err, ErrRowLimitExceeded) {
			t.Errorf("err = %v, want ErrRowLimitExceeded", err)
		}
	})
}

func TestBuildAffiliatorBulkResultCSV_neutralisesFormulas(t *testing.T) {
	out := string(BuildAffiliatorBulkResultCSV([]AffiliatorBulkResultRow{
		{Row: 2, Name: "=HYPERLINK(\"x\")", Email: "a@example.com", School: "SMA", Status: "success", TempPassword: "Abc12345"},
	}))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if lines[0] != "row,name,email,school,status,temp_password,error" {
		t.Errorf("header = %q", lines[0])
	}
	if strings.Contains(lines[1], ",=HYPERLINK") || !strings.Contains(lines[1], "Abc12345") {
		t.Errorf("row = %q, want the formula neutralised and the generated password present", lines[1])
	}
}

func TestAffiliatorBulk_Integration(t *testing.T) {
	_, repo := newRealDBService(t)
	svc := newSessionService(t, repo)
	ctx := context.Background()
	actor, err := svc.CreateAdminAccount(ctx, uuid.NewString(), "bulk-actor-"+uniqueSuffix()+"@test.com", "Bulk Actor", RoleSuperAdmin, "password123", nil)
	if err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}

	newSchool := func(t *testing.T) (id, name, code, npsn string) {
		t.Helper()
		code = "ab_" + uniqueSuffix()
		npsn = "B" + uniqueSuffix()[:7]
		sch, err := svc.CreateSchool(ctx, "Bulk School "+code, code, &npsn, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateSchool: %v", err)
		}
		return sch.ID, sch.Name, code, npsn
	}
	id1, name1, code1, _ := newSchool(t)
	id2, name2, _, npsn2 := newSchool(t)
	_, name3, code3, _ := newSchool(t)
	suffix := uniqueSuffix()
	email := func(label string) string { return "bulk-" + label + "-" + suffix + "@test.com" }

	if _, err := svc.CreateAffiliator(ctx, actor.ID, AffiliatorInput{Email: email("taken"), Name: "Existing", Password: "password123", SchoolIDs: []string{id1}}); err != nil {
		t.Fatalf("seed an existing account: %v", err)
	}

	rows := []AffiliatorBulkRow{
		// One account over three lines: by code, by NPSN, and a repeated school.
		{Row: 2, Name: "Three Lines", Email: email("a"), Password: "password123", SchoolCode: code1},
		{Row: 3, Name: "", Email: email("a"), SchoolNPSN: npsn2},
		{Row: 4, Name: "Three Lines", Email: email("a"), Password: "password123", SchoolCode: code1},
		// A generated password, one school.
		{Row: 5, Name: "Generated", Email: email("b"), SchoolCode: code3},
		// Both NPSN and code given: the NPSN wins.
		{Row: 6, Name: "Both Columns", Email: email("c"), SchoolNPSN: npsn2, SchoolCode: "ignored_code"},
		// A bad school makes the whole account fail, including its good line.
		{Row: 7, Name: "Half Bad", Email: email("d"), Password: "password123", SchoolCode: code1},
		{Row: 8, Name: "Half Bad", Email: email("d"), SchoolCode: "does_not_exist"},
		// Conflicting name or password between lines of one account.
		{Row: 9, Name: "Conflict", Email: email("e"), Password: "password123", SchoolCode: code1},
		{Row: 10, Name: "Someone Else", Email: email("e"), SchoolCode: code3},
		// Row-level failures.
		{Row: 11, Name: "Weak Password", Email: email("f"), Password: "short", SchoolCode: code1},
		{Row: 12, Name: "Bad NPSN", Email: email("g"), SchoolNPSN: "x"},
		{Row: 13, Name: "No School", Email: email("h")},
		{Row: 14, Name: "Taken Email", Email: email("taken"), SchoolCode: code3},
		{Row: 15, Name: "No Email", Email: "", SchoolCode: code1},
	}
	var progress []int
	results, ok, err := svc.ProcessAffiliatorBulkRows(ctx, actor.ID, rows, func(p int) { progress = append(progress, p) })
	if err != nil {
		t.Fatalf("ProcessAffiliatorBulkRows: %v", err)
	}
	if len(results) != len(rows) {
		t.Fatalf("results = %d, want one per line (%d)", len(results), len(rows))
	}
	// 3 lines of account a + 1 of b + 1 of c.
	if ok != 5 {
		t.Fatalf("successful lines = %d, want 5: %+v", ok, results)
	}
	if len(progress) == 0 || progress[len(progress)-1] != 100 {
		t.Errorf("progress = %v, want it to end at 100", progress)
	}
	byRow := map[int]AffiliatorBulkResultRow{}
	for _, r := range results {
		byRow[r.Row] = r
	}
	stored := func(label string) []string {
		list, _, _, _ := svc.ListAffiliators(ctx, "", "bulk-"+label+"-"+suffix, 10, "")
		if len(list) != 1 {
			return nil
		}
		return list[0].SchoolIDs
	}

	t.Run("lines sharing an email become one account with each line's school, repeats collapsed", func(t *testing.T) {
		for _, n := range []int{2, 3, 4} {
			if byRow[n].Status != "success" {
				t.Errorf("row %d = %+v, want success", n, byRow[n])
			}
		}
		if byRow[2].School != name1 || byRow[3].School != name2 {
			t.Errorf("schools reported = %q / %q, want %q / %q", byRow[2].School, byRow[3].School, name1, name2)
		}
		if got := stored("a"); len(got) != 2 || got[0] != id1 || got[1] != id2 {
			t.Errorf("stored schools = %v, want [%s %s] in line order", got, id1, id2)
		}
		if byRow[2].TempPassword != "" {
			t.Errorf("a given password must not be echoed back, got %q", byRow[2].TempPassword)
		}
		if _, _, _, err := svc.Login(ctx, email("a"), "password123", "1.1.1.1"); err != nil {
			t.Errorf("Login with the given password: %v", err)
		}
	})

	t.Run("a blank password is generated and shown on the first line only", func(t *testing.T) {
		r := byRow[5]
		if r.Status != "success" || len(r.TempPassword) < minPasswordLen || r.School != name3 {
			t.Fatalf("row 5 = %+v, want success with a generated password", r)
		}
		if _, _, _, err := svc.Login(ctx, email("b"), r.TempPassword, "1.1.1.1"); err != nil {
			t.Errorf("Login with the generated password: %v", err)
		}
	})

	t.Run("the NPSN wins when both columns are filled", func(t *testing.T) {
		if byRow[6].Status != "success" || byRow[6].School != name2 {
			t.Errorf("row 6 = %+v, want success at %q", byRow[6], name2)
		}
		if got := stored("c"); len(got) != 1 || got[0] != id2 {
			t.Errorf("stored schools = %v, want [%s]", got, id2)
		}
	})

	t.Run("one bad line fails the whole account and says which", func(t *testing.T) {
		if r := byRow[8]; r.Status != "failed" || !strings.Contains(r.Error, "school not found by code") {
			t.Errorf("row 8 = %+v, want the unknown school", r)
		}
		if r := byRow[7]; r.Status != "failed" || !strings.Contains(r.Error, "not created: row 8") {
			t.Errorf("row 7 = %+v, want it to point at row 8", r)
		}
		if got := stored("d"); got != nil {
			t.Errorf("a half-bad account was created with %v", got)
		}
	})

	t.Run("a line that contradicts the first line's name fails the account", func(t *testing.T) {
		if r := byRow[10]; r.Status != "failed" || !strings.Contains(r.Error, "name differs from row 9") {
			t.Errorf("row 10 = %+v, want a name conflict with row 9", r)
		}
		if r := byRow[9]; r.Status != "failed" || !strings.Contains(r.Error, "not created: row 10") {
			t.Errorf("row 9 = %+v, want it to point at row 10", r)
		}
		if got := stored("e"); got != nil {
			t.Errorf("a conflicting account was created with %v", got)
		}
	})

	t.Run("row level failures name their reason and create nothing", func(t *testing.T) {
		want := map[int]string{
			11: "password too weak",
			12: "NPSN",
			13: "at least one school",
			14: "email already registered",
			15: "missing required field",
		}
		for n, sub := range want {
			if r := byRow[n]; r.Status != "failed" || !strings.Contains(r.Error, sub) {
				t.Errorf("row %d = %+v, want failed mentioning %q", n, r, sub)
			}
		}
		for _, label := range []string{"f", "g", "h"} {
			if got := stored(label); got != nil {
				t.Errorf("a failed row created an account for %s: %v", label, got)
			}
		}
	})

	t.Run("a deactivated school fails the account", func(t *testing.T) {
		_, _, offCode, _ := newSchool(t)
		sch, _ := repo.GetSchoolByCode(ctx, offCode)
		if _, err := repo.Pool().Exec(ctx, "UPDATE school SET status = 'deactivated' WHERE id = $1", sch.ID); err != nil {
			t.Fatalf("deactivate school: %v", err)
		}
		res, n, _ := svc.ProcessAffiliatorBulkRows(ctx, actor.ID, []AffiliatorBulkRow{
			{Row: 2, Name: "Off", Email: email("i"), SchoolCode: offCode},
		}, nil)
		if n != 0 || !strings.Contains(res[0].Error, "deactivated") {
			t.Errorf("result = %+v, want a deactivated-school failure", res)
		}
	})

	t.Run("enqueue validates the header and queues an affiliator_bulk job", func(t *testing.T) {
		key := "affiliator-bulk/" + uniqueSuffix() + "-a.csv"
		if _, err := svc.enqueueAffiliatorBulkJobFromData(ctx, actor.ID, key, []byte("name,email\nA,a@example.com\n")); !errors.Is(err, ErrMissingAffiliatorCSVHeader) {
			t.Fatalf("bad header: err = %v, want ErrMissingAffiliatorCSVHeader", err)
		}
		jobID, err := svc.enqueueAffiliatorBulkJobFromData(ctx, actor.ID, key, []byte("name,email,school_code\nA,a@example.com,K\n"))
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		job, err := svc.storeRepo.GetJobByID(ctx, jobID)
		if err != nil || job == nil {
			t.Fatalf("GetJobByID: %v, %v", job, err)
		}
		if job.Type != "affiliator_bulk" || job.Status != "queued" || job.CreatedBy != actor.ID || job.InputURL == nil || *job.InputURL != key {
			t.Errorf("job = %+v, want a queued affiliator_bulk job created by the actor for %s", job, key)
		}
	})
}
