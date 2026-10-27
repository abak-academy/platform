package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseAffiliatorBulkCSV(t *testing.T) {
	t.Run("reads columns by name, in any order, with several schools per cell", func(t *testing.T) {
		rows, err := ParseAffiliatorBulkCSV([]byte("\ufeffemail,Name,school_code,school_npsn,password\n" +
			"a@example.com,Andi,KODEA|KODEB,12345678,secret123\n" +
			"\n" +
			"b@example.com,Bela,,\"87654321, 11112222\",\n"))
		if err != nil {
			t.Fatalf("ParseAffiliatorBulkCSV: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2 (blank line skipped)", len(rows))
		}
		a, b := rows[0], rows[1]
		if a.Row != 2 || a.Name != "Andi" || a.Email != "a@example.com" || a.Password != "secret123" ||
			strings.Join(a.SchoolCode, ",") != "KODEA,KODEB" || strings.Join(a.SchoolNPSN, ",") != "12345678" {
			t.Errorf("row a = %+v", a)
		}
		if b.Row != 3 || b.Password != "" || len(b.SchoolCode) != 0 || strings.Join(b.SchoolNPSN, ",") != "87654321,11112222" {
			t.Errorf("row b = %+v, want record 3 (blank lines are skipped), no password, two NPSNs", b)
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
		{Row: 2, Name: "=HYPERLINK(\"x\")", Email: "a@example.com", Schools: "SMA", Status: "success", TempPassword: "Abc12345"},
	}))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if lines[0] != "row,name,email,schools,status,temp_password,error" {
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
	suffix := uniqueSuffix()

	rows := []AffiliatorBulkRow{
		{Row: 2, Name: "Both Schools", Email: "bulk-a-" + suffix + "@test.com", Password: "password123", SchoolCode: []string{code1}, SchoolNPSN: []string{npsn2}},
		{Row: 3, Name: "Generated", Email: "bulk-b-" + suffix + "@test.com", SchoolCode: []string{code1, code1}},
		{Row: 4, Name: "Weak Password", Email: "bulk-c-" + suffix + "@test.com", Password: "short", SchoolCode: []string{code1}},
		{Row: 5, Name: "Unknown School", Email: "bulk-d-" + suffix + "@test.com", SchoolCode: []string{"does_not_exist"}},
		{Row: 6, Name: "Bad NPSN", Email: "bulk-e-" + suffix + "@test.com", SchoolNPSN: []string{"x"}},
		{Row: 7, Name: "No School", Email: "bulk-f-" + suffix + "@test.com"},
		{Row: 8, Name: "Duplicate Email", Email: "bulk-a-" + suffix + "@test.com", SchoolCode: []string{code1}},
	}
	var progress []int
	results, ok, err := svc.ProcessAffiliatorBulkRows(ctx, actor.ID, rows, func(p int) { progress = append(progress, p) })
	if err != nil {
		t.Fatalf("ProcessAffiliatorBulkRows: %v", err)
	}
	if ok != 2 || len(results) != len(rows) {
		t.Fatalf("ok=%d results=%d, want 2 successes among %d rows: %+v", ok, len(results), len(rows), results)
	}
	if len(progress) == 0 || progress[len(progress)-1] != 100 {
		t.Errorf("progress = %v, want it to end at 100", progress)
	}

	t.Run("a row may name schools by NPSN and by code together (NPSNs first)", func(t *testing.T) {
		r := results[0]
		if r.Status != "success" || r.TempPassword != "" || r.Schools != name2+"|"+name1 {
			t.Fatalf("row 2 = %+v, want success with both schools and no generated password", r)
		}
		list, _, _, _ := svc.ListAffiliators(ctx, "", "bulk-a-"+suffix, 10, "")
		if len(list) != 1 || len(list[0].SchoolIDs) != 2 || list[0].SchoolIDs[0] != id2 || list[0].SchoolIDs[1] != id1 {
			t.Errorf("stored schools = %+v, want [%s %s]", list, id2, id1)
		}
		if _, _, _, err := svc.Login(ctx, "bulk-a-"+suffix+"@test.com", "password123", "1.1.1.1"); err != nil {
			t.Errorf("Login with the given password: %v", err)
		}
	})

	t.Run("a blank password is generated, listed once, and works; repeated schools collapse", func(t *testing.T) {
		r := results[1]
		if r.Status != "success" || len(r.TempPassword) < minPasswordLen || r.Schools != name1 {
			t.Fatalf("row 3 = %+v, want success, a generated password and one school", r)
		}
		if _, _, _, err := svc.Login(ctx, "bulk-b-"+suffix+"@test.com", r.TempPassword, "1.1.1.1"); err != nil {
			t.Errorf("Login with the generated password: %v", err)
		}
	})

	t.Run("each bad row fails alone with a reason and creates nothing", func(t *testing.T) {
		want := map[int]string{
			2: "password too weak",
			3: "school not found by code",
			4: "NPSN",
			5: "at least one school",
			6: "email already registered",
		}
		for i, sub := range want {
			r := results[i]
			if r.Status != "failed" || !strings.Contains(r.Error, sub) {
				t.Errorf("row %d = %+v, want failed mentioning %q", r.Row, r, sub)
			}
		}
		for _, email := range []string{"bulk-c-", "bulk-d-", "bulk-e-", "bulk-f-"} {
			if list, _, _, _ := svc.ListAffiliators(ctx, "", email+suffix, 10, ""); len(list) != 0 {
				t.Errorf("a failed row created an account for %s", email)
			}
		}
	})

	t.Run("a deactivated school fails the row", func(t *testing.T) {
		_, _, offCode, _ := newSchool(t)
		sch, _ := repo.GetSchoolByCode(ctx, offCode)
		if _, err := repo.Pool().Exec(ctx, "UPDATE school SET status = 'deactivated' WHERE id = $1", sch.ID); err != nil {
			t.Fatalf("deactivate school: %v", err)
		}
		res, ok, _ := svc.ProcessAffiliatorBulkRows(ctx, actor.ID, []AffiliatorBulkRow{
			{Row: 2, Name: "Off", Email: "bulk-g-" + suffix + "@test.com", SchoolCode: []string{offCode}},
		}, nil)
		if ok != 0 || !strings.Contains(res[0].Error, "deactivated") {
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
