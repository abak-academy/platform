package service

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
)

func TestBuildCredentialsResultCSV(t *testing.T) {
	rows := []StudentBulkResultRow{
		{Name: "Budi", Username: "sch_1001", TempPassword: "abc123"},
		{Name: "Siti", Error: "student_not_found"},
	}
	data := BuildCredentialsResultCSV(rows)

	r := csv.NewReader(strings.NewReader(string(data)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read back csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 records (header + 2 rows), got %d", len(records))
	}
	wantHeader := []string{"name", "username", "temp_password", "error"}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Errorf("header[%d]: want %s, got %s", i, h, records[0][i])
		}
	}
	wantRow1 := []string{"Budi", "sch_1001", "abc123", ""}
	for i, v := range wantRow1 {
		if records[1][i] != v {
			t.Errorf("row1[%d]: want %s, got %s", i, v, records[1][i])
		}
	}
	wantRow2 := []string{"Siti", "", "", "student_not_found"}
	for i, v := range wantRow2 {
		if records[2][i] != v {
			t.Errorf("row2[%d]: want %s, got %s", i, v, records[2][i])
		}
	}
}

func TestReissueStudentCredentialsBulk_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	t.Run("explicit ids: success and not-found rows coexist, batch does not abort", func(t *testing.T) {
		schoolID := createTestSchool(t, svc)
		reg, err := svc.RegisterStudent(ctx, testActorID, schoolID, "Budi Reissue", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}

		missingID := "00000000-0000-0000-0000-000000000000"
		data, err := svc.ReissueStudentCredentialsBulk(ctx, testActorID, schoolID, []string{reg.ID, missingID}, false)
		if err != nil {
			t.Fatalf("ReissueStudentCredentialsBulk: %v", err)
		}

		r := csv.NewReader(strings.NewReader(string(data)))
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("read back csv: %v", err)
		}
		if len(records) != 3 {
			t.Fatalf("want 3 records (header + 2 rows), got %d", len(records))
		}
		successRow := records[1]
		if successRow[0] != "Budi Reissue" || successRow[1] == "" || successRow[2] == "" || successRow[3] != "" {
			t.Errorf("unexpected success row: %+v", successRow)
		}
		if successRow[2] == reg.TempPassword {
			t.Error("reissued temp_password should differ from the original registration temp password (proves reissue, not a no-op)")
		}
		notFoundRow := records[2]
		if notFoundRow[3] != "student_not_found" {
			t.Errorf("want error=student_not_found for missing id, got %+v", notFoundRow)
		}
	})

	t.Run("cross-school id is treated as not found", func(t *testing.T) {
		schoolA := createTestSchool(t, svc)
		schoolB := createTestSchool(t, svc)
		reg, err := svc.RegisterStudent(ctx, testActorID, schoolA, "Cross School", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("RegisterStudent: %v", err)
		}

		data, err := svc.ReissueStudentCredentialsBulk(ctx, testActorID, schoolB, []string{reg.ID}, false)
		if err != nil {
			t.Fatalf("ReissueStudentCredentialsBulk: %v", err)
		}
		r := csv.NewReader(strings.NewReader(string(data)))
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("read back csv: %v", err)
		}
		if len(records) != 2 || records[1][3] != "student_not_found" {
			t.Errorf("want single not-found row, got %+v", records)
		}
	})

	t.Run("explicit ids over the row cap error out before any reissue call", func(t *testing.T) {
		ids := make([]string, maxBulkRows+1)
		for i := range ids {
			ids[i] = "00000000-0000-0000-0000-000000000000"
		}
		_, err := svc.ReissueStudentCredentialsBulk(ctx, testActorID, "some-school", ids, false)
		if !errors.Is(err, ErrRowLimitExceeded) {
			t.Errorf("want ErrRowLimitExceeded, got %v", err)
		}
	})

	t.Run("all=true paginates the whole school", func(t *testing.T) {
		schoolID := createTestSchool(t, svc)
		var regs []*StudentRegistrationResponse
		for i := 0; i < 3; i++ {
			reg, err := svc.RegisterStudent(ctx, testActorID, schoolID, "All Student", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("RegisterStudent: %v", err)
			}
			regs = append(regs, reg)
		}

		data, err := svc.ReissueStudentCredentialsBulk(ctx, testActorID, schoolID, nil, true)
		if err != nil {
			t.Fatalf("ReissueStudentCredentialsBulk: %v", err)
		}
		r := csv.NewReader(strings.NewReader(string(data)))
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("read back csv: %v", err)
		}
		if len(records) != len(regs)+1 {
			t.Fatalf("want %d records (header + %d rows), got %d", len(regs)+1, len(regs), len(records))
		}
		for _, rec := range records[1:] {
			if rec[0] == "" || rec[1] == "" || rec[2] == "" || rec[3] != "" {
				t.Errorf("unexpected row in all=true batch: %+v", rec)
			}
		}
	})

	t.Run("all=true school over the row cap errors out via pagination, not just explicit ids", func(t *testing.T) {
		origCap := bulkAllRowCap
		bulkAllRowCap = 2
		t.Cleanup(func() { bulkAllRowCap = origCap })

		schoolID := createTestSchool(t, svc)
		for i := 0; i < 3; i++ {
			if _, err := svc.RegisterStudent(ctx, testActorID, schoolID, "Cap Student", "sma", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
				t.Fatalf("RegisterStudent: %v", err)
			}
		}

		_, err := svc.ReissueStudentCredentialsBulk(ctx, testActorID, schoolID, nil, true)
		if !errors.Is(err, ErrRowLimitExceeded) {
			t.Errorf("want ErrRowLimitExceeded, got %v", err)
		}
	})
}
