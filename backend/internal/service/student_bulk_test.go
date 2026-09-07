package service

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

func TestParseStudentBulkCSV(t *testing.T) {
	t.Run("valid CSV with jenjang, school, and email", func(t *testing.T) {
		data := []byte("name,school,jenjang,email\nBudi,SMAN 1 Jakarta,sma,budi@example.com\nSiti,SMAN 1 Jakarta,sma,\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("want 2 rows, got %d", len(rows))
		}
		if rows[0].Name != "Budi" || rows[0].School != "SMAN 1 Jakarta" || rows[0].Jenjang != "sma" || rows[0].Email == nil || *rows[0].Email != "budi@example.com" {
			t.Errorf("unexpected row 0: %+v", rows[0])
		}
		if rows[1].Email != nil {
			t.Errorf("want nil email for empty column, got %v", *rows[1].Email)
		}
	})

	t.Run("school-only CSV (no email)", func(t *testing.T) {
		data := []byte("name,school,jenjang\nBudi,SMAN 1 Jakarta,sma\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 || rows[0].Name != "Budi" || rows[0].School != "SMAN 1 Jakarta" || rows[0].Jenjang != "sma" {
			t.Errorf("unexpected rows: %+v", rows)
		}
	})

	t.Run("missing school header returns error", func(t *testing.T) {
		data := []byte("name,jenjang\nBudi,sma\n")
		_, err := ParseStudentBulkCSV(data)
		if !errors.Is(err, ErrMissingCSVHeader) {
			t.Errorf("want ErrMissingCSVHeader, got %v", err)
		}
	})

	t.Run("school header case-insensitive", func(t *testing.T) {
		data := []byte("Name,SCHOOL,Jenjang\nBudi,SMAN 1 Jakarta,sma\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 || rows[0].Name != "Budi" || rows[0].School != "SMAN 1 Jakarta" || rows[0].Jenjang != "sma" {
			t.Errorf("unexpected rows: %+v", rows)
		}
	})

	t.Run("missing name header returns error", func(t *testing.T) {
		data := []byte("school,jenjang,email\nSMAN 1 Jakarta,sma,a@b.com\n")
		_, err := ParseStudentBulkCSV(data)
		if !errors.Is(err, ErrMissingCSVHeader) {
			t.Errorf("want ErrMissingCSVHeader, got %v", err)
		}
	})

	t.Run("nis header ignored, school and jenjang still required", func(t *testing.T) {
		data := []byte("name,nis,email\nBudi,1001,budi@example.com\n")
		_, err := ParseStudentBulkCSV(data)
		if !errors.Is(err, ErrMissingCSVHeader) {
			t.Errorf("want ErrMissingCSVHeader (school+jenjang missing), got %v", err)
		}
	})

	t.Run("nis header present with school and jenjang is ignored", func(t *testing.T) {
		data := []byte("name,school,jenjang,nis,email\nBudi,SMAN 1 Jakarta,sma,1001,budi@example.com\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 || rows[0].Name != "Budi" || rows[0].School != "SMAN 1 Jakarta" || rows[0].Jenjang != "sma" {
			t.Errorf("unexpected rows: %+v", rows)
		}
	})

	t.Run("optional address columns parsed when present", func(t *testing.T) {
		data := []byte("name,school,jenjang,email,provinsi,kota,kecamatan,kode_pos\nBudi,SMAN 1 Jakarta,sma,b@b.com,Jawa Barat,Bandung,Coblong,40131\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d", len(rows))
		}
		if rows[0].Name != "Budi" || rows[0].School != "SMAN 1 Jakarta" || rows[0].Jenjang != "sma" {
			t.Errorf("unexpected name/school/jenjang: %+v", rows[0])
		}
		if rows[0].Provinsi == nil || *rows[0].Provinsi != "Jawa Barat" {
			t.Errorf("want provinsi 'Jawa Barat', got %v", rows[0].Provinsi)
		}
		if rows[0].Kota == nil || *rows[0].Kota != "Bandung" {
			t.Errorf("want kota 'Bandung', got %v", rows[0].Kota)
		}
		if rows[0].Kecamatan == nil || *rows[0].Kecamatan != "Coblong" {
			t.Errorf("want kecamatan 'Coblong', got %v", rows[0].Kecamatan)
		}
		if rows[0].KodePos == nil || *rows[0].KodePos != "40131" {
			t.Errorf("want kode_pos '40131', got %v", rows[0].KodePos)
		}
	})

	t.Run("optional dob/gender/grade/alamat_domisili/target_exam columns parsed when present", func(t *testing.T) {
		data := []byte("name,school,jenjang,dob,gender,grade,alamat_domisili,target_exam\nBudi,SMAN 1 Jakarta,sma,2008-05-14,male,11,Jl. Melati No. 3,UTBK\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d", len(rows))
		}
		if rows[0].DOB == nil || *rows[0].DOB != "2008-05-14" {
			t.Errorf("want dob '2008-05-14', got %v", rows[0].DOB)
		}
		if rows[0].Gender == nil || *rows[0].Gender != "male" {
			t.Errorf("want gender 'male', got %v", rows[0].Gender)
		}
		if rows[0].Grade == nil || *rows[0].Grade != "11" {
			t.Errorf("want grade '11', got %v", rows[0].Grade)
		}
		if rows[0].AlamatDomisili == nil || *rows[0].AlamatDomisili != "Jl. Melati No. 3" {
			t.Errorf("want alamat_domisili 'Jl. Melati No. 3', got %v", rows[0].AlamatDomisili)
		}
		if rows[0].TargetExam == nil || *rows[0].TargetExam != "UTBK" {
			t.Errorf("want target_exam 'UTBK', got %v", rows[0].TargetExam)
		}
	})

	t.Run("optional dob/gender/grade/alamat_domisili/target_exam columns absent not an error", func(t *testing.T) {
		data := []byte("name,school,jenjang\nBudi,SMAN 1 Jakarta,sma\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d", len(rows))
		}
		if rows[0].DOB != nil || rows[0].Gender != nil || rows[0].Grade != nil || rows[0].AlamatDomisili != nil || rows[0].TargetExam != nil {
			t.Errorf("optional fields should be nil when columns absent, got %+v", rows[0])
		}
	})

	t.Run("optional address columns absent not an error", func(t *testing.T) {
		data := []byte("name,school,jenjang\nBudi,SMAN 1 Jakarta,sma\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 || rows[0].Name != "Budi" || rows[0].School != "SMAN 1 Jakarta" || rows[0].Jenjang != "sma" {
			t.Errorf("unexpected rows: %+v", rows)
		}
		if rows[0].Provinsi != nil || rows[0].Kota != nil || rows[0].Kecamatan != nil || rows[0].KodePos != nil {
			t.Errorf("optional address fields should be nil when columns absent, got %+v", rows[0])
		}
	})

	t.Run("unparseable bytes returns ErrInvalidCSV", func(t *testing.T) {
		data := []byte("name,school,jenjang\n\"Budi,sma\n")
		_, err := ParseStudentBulkCSV(data)
		if !errors.Is(err, ErrInvalidCSV) {
			t.Errorf("want ErrInvalidCSV, got %v", err)
		}
	})

	t.Run("ragged row shorter than header is accepted", func(t *testing.T) {
		data := []byte("name,school,jenjang\nBudi\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d", len(rows))
		}
		if rows[0].Name != "Budi" || rows[0].School != "" || rows[0].Jenjang != "" {
			t.Errorf("want truncated cells empty, got %+v", rows[0])
		}
		if rows[0].Row != 2 {
			t.Errorf("want spreadsheet line 2, got %d", rows[0].Row)
		}
	})

	t.Run("UTF-8 BOM on header is stripped", func(t *testing.T) {
		data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("name,school,jenjang\nBudi,SMAN 1 Jakarta,sma\n")...)
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 || rows[0].Name != "Budi" {
			t.Errorf("BOM should not break header match, got %+v err=%v", rows, err)
		}
	})

	t.Run("cell values are trimmed", func(t *testing.T) {
		data := []byte("name,school,jenjang,gender\n Budi , SMAN 1 Jakarta , sma , male \n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d", len(rows))
		}
		if rows[0].Name != "Budi" || rows[0].School != "SMAN 1 Jakarta" || rows[0].Jenjang != "sma" {
			t.Errorf("want trimmed required cells, got %+v", rows[0])
		}
		if rows[0].Gender == nil || *rows[0].Gender != "male" {
			t.Errorf("want trimmed gender male, got %v", rows[0].Gender)
		}
	})

	t.Run("blank trailing rows are skipped", func(t *testing.T) {
		data := []byte("name,school,jenjang\nBudi,SMAN 1 Jakarta,sma\n\n\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 data row after skipping blanks, got %d", len(rows))
		}
	})

	t.Run("exactly 1000 data rows is fine", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("name,school,jenjang\n")
		for i := 0; i < maxBulkRows; i++ {
			sb.WriteString("Student,School,sma\n")
		}
		rows, err := ParseStudentBulkCSV([]byte(sb.String()))
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if len(rows) != maxBulkRows {
			t.Errorf("want %d rows, got %d", maxBulkRows, len(rows))
		}
	})

	t.Run("1001 data rows exceeds limit", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("name,school,jenjang\n")
		for i := 0; i < maxBulkRows+1; i++ {
			sb.WriteString("Student,School,sma\n")
		}
		_, err := ParseStudentBulkCSV([]byte(sb.String()))
		if !errors.Is(err, ErrRowLimitExceeded) {
			t.Errorf("want ErrRowLimitExceeded, got %v", err)
		}
	})

	t.Run("optional password header is case-insensitive and blank cells are omitted", func(t *testing.T) {
		data := []byte("name,school,jenjang,PASSWORD\nBudi,SMAN 1 Jakarta,sma,chosenPass123\nSiti,SMAN 1 Jakarta,sma,\n")
		rows, err := ParseStudentBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if rows[0].Password == nil || *rows[0].Password != "chosenPass123" {
			t.Fatalf("want parsed explicit password, got %+v", rows[0].Password)
		}
		if rows[1].Password != nil {
			t.Fatalf("blank password cell should be nil, got %q", *rows[1].Password)
		}
	})

	t.Run("missing password header leaves password nil", func(t *testing.T) {
		rows, err := ParseStudentBulkCSV([]byte("name,school,jenjang\nBudi,SMAN 1 Jakarta,sma\n"))
		if err != nil {
			t.Fatalf("ParseStudentBulkCSV: %v", err)
		}
		if rows[0].Password != nil {
			t.Fatalf("missing password header should leave nil, got %q", *rows[0].Password)
		}
	})
}

const frontendStudentBulkTemplateCSV = "name,school,jenjang,email,dob,gender,grade,target_exam,alamat_domisili,provinsi,kota,kecamatan,kode_pos\n" +
	"Budi Santoso,SMAN 1 Jakarta,SMA,budi@example.com,2008-05-14,male,11,UTBK,\"Jl. Melati No. 3, RT 04\",JAWA BARAT,KOTA BANDUNG,COBLONG,40132\n" +
	"Siti Aminah,SMAN 1 Jakarta,SMA,,,,,,,,,,\n"

func TestFrontendStudentTemplateParsesUnmodified(t *testing.T) {
	rows, err := ParseStudentBulkCSV([]byte(frontendStudentBulkTemplateCSV))
	if err != nil {
		t.Fatalf("the frontend template must parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 example rows, got %d", len(rows))
	}
	if rows[0].Jenjang != "SMA" || rows[0].Provinsi == nil || *rows[0].Provinsi != "JAWA BARAT" {
		t.Errorf("unexpected first row: %+v", rows[0])
	}
	if rows[0].Kota == nil || *rows[0].Kota != "KOTA BANDUNG" {
		t.Errorf("want KOTA BANDUNG, got %v", rows[0].Kota)
	}
	if rows[0].AlamatDomisili == nil || *rows[0].AlamatDomisili != "Jl. Melati No. 3, RT 04" {
		t.Errorf("want quoted address unquoted, got %v", rows[0].AlamatDomisili)
	}
	if rows[1].Name != "Siti Aminah" || rows[1].Email != nil {
		t.Errorf("unexpected minimal row: %+v", rows[1])
	}
}

func TestBuildStudentBulkResultCSV(t *testing.T) {
	results := []StudentBulkResultRow{
		{Row: 2, Name: "Budi", Jenjang: "sma", School: "SMAN 1 Jakarta", Email: "budi@example.com", DOB: "2008-05-12", Gender: "L", Grade: "11", Provinsi: "Jawa Barat", Kota: "Bandung", Kecamatan: "Cicendo", Status: "success", Username: "budi123", TempPassword: "abc123"},
		{Row: 3, Name: "Siti", Jenjang: "sma", School: "SMAN 1 Jakarta", Status: "failed", Error: "some error"},
	}
	data := BuildStudentBulkResultCSV(results)

	r := csv.NewReader(strings.NewReader(string(data)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read back csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 records (header + 2 rows), got %d", len(records))
	}
	wantHeader := []string{
		"row", "name", "jenjang", "school", "email", "dob", "gender", "grade",
		"alamat_domisili", "target_exam", "provinsi", "kota", "kecamatan", "kode_pos",
		"status", "username", "temp_password", "error",
	}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Errorf("header[%d]: want %s, got %s", i, h, records[0][i])
		}
	}
	wantRow1 := []string{"2", "Budi", "sma", "SMAN 1 Jakarta", "budi@example.com", "2008-05-12", "L", "11", "", "", "Jawa Barat", "Bandung", "Cicendo", "", "success", "budi123", "abc123", ""}
	for i, v := range wantRow1 {
		if records[1][i] != v {
			t.Errorf("row1[%d]: want %s, got %s", i, v, records[1][i])
		}
	}
	wantRow2 := []string{"3", "Siti", "sma", "SMAN 1 Jakarta", "", "", "", "", "", "", "", "", "", "", "failed", "", "", "some error"}
	for i, v := range wantRow2 {
		if records[2][i] != v {
			t.Errorf("row2[%d]: want %s, got %s", i, v, records[2][i])
		}
	}
}

func TestBuildStudentBulkResultCSV_DoesNotLeakExplicitPassword(t *testing.T) {
	explicitPassword := "chosenPass123"
	hash, err := hashPassword(explicitPassword)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	results := []StudentBulkResultRow{
		{Row: 2, Name: "Budi", School: "SMAN 1 Jakarta", Email: "budi@example.com", Status: "success", Username: "budi123", TempPassword: ""},
		{Row: 3, Name: "Siti", School: "SMAN 1 Jakarta", Status: "success", Username: "siti123", TempPassword: "generated123"},
	}
	data := string(BuildStudentBulkResultCSV(results))
	if !strings.Contains(data, "temp_password") || !strings.Contains(data, "generated123") {
		t.Fatalf("legacy temp_password column/value must remain, got %s", data)
	}
	if strings.Contains(data, explicitPassword) || strings.Contains(data, string(hash)) {
		t.Fatalf("result CSV leaked explicit password or hash: %s", data)
	}
}

// schoolNameByID is a test helper that retrieves the school name for a given ID.
func schoolNameByID(t *testing.T, repo *repository.Repository, schoolID string) string {
	t.Helper()
	ctx := context.Background()
	school, err := repo.GetSchoolByID(ctx, schoolID)
	if err != nil || school == nil {
		t.Fatalf("GetSchoolByID(%s): %v", schoolID, err)
	}
	return school.Name
}

func TestProcessStudentBulkRows_Integration(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	// Seed region data for name-resolution tests.
	seedTestRegionData(t, repo)

	t.Run("all-success batch with jenjang only (no address)", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		rows := []StudentBulkRow{
			{Name: "Budi", School: schoolName, Jenjang: "sma"},
			{Name: "Siti", School: schoolName, Jenjang: "sma"},
		}
		var progressCalls []int
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, func(pct int) {
			progressCalls = append(progressCalls, pct)
		})
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != len(rows) {
			t.Errorf("want successCount=%d, got %d", len(rows), successCount)
		}
		for _, r := range results {
			if r.Status != "success" || r.Username == "" || r.TempPassword == "" || r.Error != "" {
				t.Errorf("unexpected result row: %+v", r)
			}
			if r.School != schoolName {
				t.Errorf("want school=%q, got %q", schoolName, r.School)
			}
		}
		if len(progressCalls) == 0 || progressCalls[len(progressCalls)-1] != 100 {
			t.Errorf("want progress calls ending at 100, got %v", progressCalls)
		}
	})

	t.Run("dob/gender/grade/alamat_domisili/target_exam persisted, same as single registration", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		dob := "2008-05-14"
		gender := "male"
		grade := "11"
		alamat := "Jl. Melati No. 3"
		targetExam := "UTBK"
		rows := []StudentBulkRow{
			{Name: "Fields", School: schoolName, Jenjang: "sma", DOB: &dob, Gender: &gender, Grade: &grade, AlamatDomisili: &alamat, TargetExam: &targetExam},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 || results[0].Status != "success" {
			t.Fatalf("want success, got %+v", results[0])
		}

		student, err := repo.GetUserByUsername(ctx, results[0].Username)
		if err != nil || student == nil {
			t.Fatalf("GetUserByUsername(%s): %v", results[0].Username, err)
		}
		if student.DOB == nil || student.DOB.Format("2006-01-02") != dob {
			t.Errorf("want dob %s, got %v", dob, student.DOB)
		}
		// Persisted as 'm', not 'male' — RegisterStudent normalizes gender to
		// match users_gender_check (see normalizeGender in admin_students.go).
		if student.Gender == nil || *student.Gender != "m" {
			t.Errorf("want gender 'm', got %v", student.Gender)
		}
		if student.Grade == nil || *student.Grade != 11 {
			t.Errorf("want grade 11, got %v", student.Grade)
		}
		if student.AlamatDomisili == nil || *student.AlamatDomisili != alamat {
			t.Errorf("want alamat_domisili %q, got %v", alamat, student.AlamatDomisili)
		}
		if student.TargetExam == nil || *student.TargetExam != targetExam {
			t.Errorf("want target_exam %q, got %v", targetExam, student.TargetExam)
		}
	})

	t.Run("invalid dob format produces row-level error, not a batch abort", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		badDOB := "14-05-2008"
		rows := []StudentBulkRow{
			{Name: "BadDOB", School: schoolName, Jenjang: "sma", DOB: &badDOB},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 0 {
			t.Errorf("want successCount=0 for invalid dob, got %d", successCount)
		}
		if results[0].Status != "failed" || results[0].Error != ErrInvalidDOBFormat.Error() {
			t.Errorf("want failed with ErrInvalidDOBFormat, got %+v", results[0])
		}
	})

	t.Run("non-numeric grade produces row-level error, not a batch abort", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		badGrade := "sepuluh"
		rows := []StudentBulkRow{
			{Name: "BadGrade", School: schoolName, Jenjang: "sma", Grade: &badGrade},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 0 {
			t.Errorf("want successCount=0 for invalid grade, got %d", successCount)
		}
		if results[0].Status != "failed" || results[0].Error != ErrInvalidGradeFormat.Error() {
			t.Errorf("want failed with ErrInvalidGradeFormat, got %+v", results[0])
		}
	})

	t.Run("address names resolved correctly", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		sulsel, _ := repo.GetProvinceByName(ctx, "SULAWESI SELATAN")
		if sulsel == nil {
			t.Fatal("SULAWESI SELATAN should exist in seeded data")
		}
		makassar, _ := repo.GetCityByNameInProvince(ctx, "KOTA MAKASSAR", sulsel.ID)
		if makassar == nil {
			t.Fatal("KOTA MAKASSAR should exist")
		}

		sulselProv := "SULAWESI SELATAN"
		makassarKota := "KOTA MAKASSAR"
		mariso := "MARISO"
		kodePos := "90222"
		rows := []StudentBulkRow{
			{Name: "Andi", School: schoolName, Jenjang: "sma", Provinsi: &sulselProv, Kota: &makassarKota, Kecamatan: &mariso, KodePos: &kodePos},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 {
			t.Fatalf("want successCount=1, got %d: %+v", successCount, results)
		}
		if results[0].Status != "success" || results[0].Error != "" {
			t.Errorf("want success, got %+v", results[0])
		}
		if results[0].School != schoolName {
			t.Errorf("want school=%q, got %q", schoolName, results[0].School)
		}
	})

	t.Run("partial address per row produces row-level error", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		sulselProv := "SULAWESI SELATAN"
		rows := []StudentBulkRow{
			{Name: "Partial", School: schoolName, Jenjang: "sma", Provinsi: &sulselProv},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 0 {
			t.Errorf("want successCount=0 for partial address, got %d", successCount)
		}
		if results[0].Status != "failed" || results[0].Error == "" {
			t.Errorf("want failed with error for partial address, got %+v", results[0])
		}
	})

	t.Run("unresolvable province name produces row-level error", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		bogusProv := "NONEXISTENT PROVINCE"
		makassarKota := "KOTA MAKASSAR"
		mariso := "MARISO"
		rows := []StudentBulkRow{
			{Name: "Bogus", School: schoolName, Jenjang: "sma", Provinsi: &bogusProv, Kota: &makassarKota, Kecamatan: &mariso},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 0 {
			t.Errorf("want successCount=0 for unresolvable province, got %d", successCount)
		}
		if results[0].Status != "failed" || results[0].Error == "" {
			t.Errorf("want failed with error for unresolvable province, got %+v", results[0])
		}
	})

	t.Run("deactivated school: every row fails, successCount 0", func(t *testing.T) {
		schoolID := createTestSchool(t, svc)
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		if _, err := svc.ChangeSchoolStatus(ctx, schoolID, "deactivated"); err != nil {
			t.Fatalf("ChangeSchoolStatus: %v", err)
		}
		rows := []StudentBulkRow{
			{Name: "A", School: schoolName, Jenjang: "sma"},
			{Name: "B", School: schoolName, Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, func(int) {})
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 0 {
			t.Errorf("want successCount=0, got %d", successCount)
		}
		for _, r := range results {
			if r.Status != "failed" || r.Error != ErrSchoolDeactivated.Error() {
				t.Errorf("want every row failed with ErrSchoolDeactivated, got %+v", r)
			}
		}
	})

	t.Run("unexpected error (not one of the 3 known sentinels) is a row failure, not a batch abort", func(t *testing.T) {
		schoolID := createTestSchool(t, svc)
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		cancelCtx, cancel := context.WithCancel(ctx)
		rows := []StudentBulkRow{
			{Name: "First", School: schoolName, Jenjang: "sma"},
			{Name: "Second", School: schoolName, Jenjang: "sma"},
			{Name: "Third", School: schoolName, Jenjang: "sma"},
		}

		callCount := 0
		results, successCount, err := svc.ProcessStudentBulkRows(cancelCtx, schoolBound, RoleAdminSchool, rows, nil, func(int) {
			callCount++
			if callCount == 1 {
				cancel()
			}
		})
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: want nil error (row failures must not abort the batch), got %v", err)
		}
		if len(results) != len(rows) {
			t.Fatalf("want a report row for every input row, got %d", len(results))
		}
		if successCount != 1 {
			t.Errorf("want successCount=1 (only the row processed before cancellation), got %d", successCount)
		}
		if results[0].Status != "success" {
			t.Errorf("want first row to succeed before cancellation, got %+v", results[0])
		}
		for _, r := range results[1:] {
			if r.Status != "failed" || r.Error == "" {
				t.Errorf("want rows after cancellation to be reported as failed with a non-empty error, got %+v", r)
			}
			if r.Error == ErrSchoolDeactivated.Error() || r.Error == ErrMissingField.Error() {
				t.Errorf("this row's failure must be the unexpected context-cancellation error, not one of the known sentinels: %+v", r)
			}
		}
	})

	t.Run("progress callback: monotonic non-decreasing, checkpoint every 5 rows for a 50-row batch", func(t *testing.T) {
		schoolID := createTestSchool(t, svc)
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		rows := make([]StudentBulkRow, 50)
		for i := range rows {
			rows[i] = StudentBulkRow{Name: "Student", School: schoolName, Jenjang: "sma"}
		}
		var progressCalls []int
		_, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, func(pct int) {
			progressCalls = append(progressCalls, pct)
		})
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != len(rows) {
			t.Fatalf("want successCount=%d, got %d", len(rows), successCount)
		}
		if len(progressCalls) < 10 {
			t.Fatalf("want at least 10 progress calls for a 50-row batch, got %d: %v", len(progressCalls), progressCalls)
		}
		for i := 1; i < len(progressCalls); i++ {
			if progressCalls[i] < progressCalls[i-1] {
				t.Errorf("want monotonically non-decreasing progress, got %v", progressCalls)
				break
			}
		}
		if progressCalls[len(progressCalls)-1] != 100 {
			t.Errorf("want final progress call to be 100, got %v", progressCalls)
		}
	})

	// --- Task 27: School resolution tests ---

	t.Run("admin_school: row with unknown school fails with school-not-found", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolBound := &schoolID
		rows := []StudentBulkRow{
			{Name: "NoSchool", School: "THIS SCHOOL DOES NOT EXIST", Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 0 {
			t.Errorf("want successCount=0 for unknown school, got %d", successCount)
		}
		if results[0].Status != "failed" || results[0].Error != ErrSchoolNotFoundByName.Error() {
			t.Errorf("want failed with ErrSchoolNotFoundByName, got %+v", results[0])
		}
		if results[0].School != "THIS SCHOOL DOES NOT EXIST" {
			t.Errorf("want raw CSV school value in result, got %q", results[0].School)
		}
	})

	t.Run("admin_school: row with different school fails with cross-school error", func(t *testing.T) {
		schoolA := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolB := createTestSchool(t, svc)
		schoolBName := schoolNameByID(t, repo, schoolB)
		schoolBound := &schoolA

		rows := []StudentBulkRow{
			{Name: "Cross", School: schoolBName, Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 0 {
			t.Errorf("want successCount=0 for cross-school, got %d", successCount)
		}
		if results[0].Status != "failed" || results[0].Error != ErrCrossSchoolBound.Error() {
			t.Errorf("want failed with ErrCrossSchoolBound, got %+v", results[0])
		}
		if results[0].School != schoolBName {
			t.Errorf("want raw CSV school value, got %q", results[0].School)
		}
	})

	t.Run("super_admin: nil schoolBound allows any school", func(t *testing.T) {
		schoolA := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolAName := schoolNameByID(t, repo, schoolA)
		schoolB := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolBName := schoolNameByID(t, repo, schoolB)

		rows := []StudentBulkRow{
			{Name: "FromA", School: schoolAName, Jenjang: "sma"},
			{Name: "FromB", School: schoolBName, Jenjang: "sma"},
		}
		// schoolBound = nil simulates super_admin (unrestricted).
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, nil, RoleSuperAdmin, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 2 {
			t.Fatalf("want successCount=2, got %d: %+v", successCount, results)
		}
		if results[0].Status != "success" || results[1].Status != "success" {
			t.Errorf("want both rows to succeed, got %+v", results)
		}
		// Each result should have the canonical school name.
		if results[0].School != schoolAName {
			t.Errorf("result 0: want school=%q, got %q", schoolAName, results[0].School)
		}
		if results[1].School != schoolBName {
			t.Errorf("result 1: want school=%q, got %q", schoolBName, results[1].School)
		}
	})

	t.Run("nil schoolBound with unknown school still fails per-row", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)

		rows := []StudentBulkRow{
			{Name: "Good", School: schoolName, Jenjang: "sma"},
			{Name: "Bad", School: "NONEXISTENT SCHOOL", Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, nil, RoleSuperAdmin, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 {
			t.Errorf("want successCount=1, got %d", successCount)
		}
		if results[0].Status != "success" {
			t.Errorf("want first row to succeed, got %+v", results[0])
		}
		if results[0].School != schoolName {
			t.Errorf("result 0: want school=%q, got %q", schoolName, results[0].School)
		}
		if results[1].Status != "failed" {
			t.Errorf("want second row to fail, got %+v", results[1])
		}
		if results[1].School != "NONEXISTENT SCHOOL" {
			t.Errorf("result 1: want raw school=%q, got %q", "NONEXISTENT SCHOOL", results[1].School)
		}
	})

	t.Run("super_admin mixed explicit and generated passwords are per-row", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		explicitPassword := "chosenPass123"
		rows := []StudentBulkRow{
			{Name: "Explicit Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma", Password: &explicitPassword},
			{Name: "Generated Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, nil, RoleSuperAdmin, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 2 {
			t.Fatalf("want successCount=2, got %d: %+v", successCount, results)
		}
		if results[0].Status != "success" || results[0].TempPassword != "" {
			t.Fatalf("explicit row should succeed without result temp_password, got %+v", results[0])
		}
		if results[1].Status != "success" || results[1].TempPassword == "" {
			t.Fatalf("generated row should retain result temp_password, got %+v", results[1])
		}
		explicitUser, err := repo.GetUserByUsername(ctx, results[0].Username)
		if err != nil || explicitUser == nil {
			t.Fatalf("read explicit user: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(explicitUser.PasswordHash), []byte(explicitPassword)); err != nil {
			t.Fatalf("explicit row hash does not match supplied password: %v", err)
		}
		generatedUser, err := repo.GetUserByUsername(ctx, results[1].Username)
		if err != nil || generatedUser == nil {
			t.Fatalf("read generated user: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(generatedUser.PasswordHash), []byte(results[1].TempPassword)); err != nil {
			t.Fatalf("generated row hash does not match generated temp password: %v", err)
		}
	})

	t.Run("super_admin weak explicit password fails only that row", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		weak := "short"
		rows := []StudentBulkRow{
			{Name: "Weak Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma", Password: &weak},
			{Name: "Valid Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, nil, RoleSuperAdmin, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 {
			t.Fatalf("want only valid row to succeed, got %d: %+v", successCount, results)
		}
		if results[0].Status != "failed" || results[0].Error != ErrWeakPassword.Error() || results[0].Username != "" || results[0].TempPassword != "" {
			t.Fatalf("weak explicit row should fail without credentials, got %+v", results[0])
		}
		if results[1].Status != "success" || results[1].TempPassword == "" {
			t.Fatalf("valid generated row should succeed, got %+v", results[1])
		}
	})

	t.Run("admin_school explicit password is forbidden per row while blank rows succeed", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		explicitPassword := "chosenPass123"
		explicitName := "Forbidden Bulk " + uniqueSuffix()
		rows := []StudentBulkRow{
			{Name: explicitName, School: schoolName, Jenjang: "sma", Password: &explicitPassword},
			{Name: "Allowed Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, nil, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 {
			t.Fatalf("want only blank-password row to succeed, got %d: %+v", successCount, results)
		}
		if results[0].Status != "failed" || results[0].Error != ErrForbidden.Error() || results[0].Username != "" || results[0].TempPassword != "" {
			t.Fatalf("explicit admin_school row should be forbidden without credentials, got %+v", results[0])
		}
		if results[1].Status != "success" || results[1].TempPassword == "" {
			t.Fatalf("blank admin_school row should succeed unchanged, got %+v", results[1])
		}
		var count int
		if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM users WHERE name = $1`, explicitName).Scan(&count); err != nil {
			t.Fatalf("count forbidden insert: %v", err)
		}
		if count != 0 {
			t.Fatalf("forbidden explicit row inserted %d users", count)
		}
	})

	t.Run("super_admin request-level password applies to rows without CSV password", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		requestPassword := "welcomePass123"
		rows := []StudentBulkRow{
			{Name: "ReqPass Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma"},
			{Name: "ReqPass2 Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma"},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, nil, RoleSuperAdmin, rows, &requestPassword, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 2 {
			t.Fatalf("want successCount=2, got %d: %+v", successCount, results)
		}
		for i, r := range results {
			if r.Status != "success" || r.Username == "" || r.TempPassword != "" {
				t.Fatalf("row %d should succeed on the explicit path without result temp_password, got %+v", i, r)
			}
			u, err := repo.GetUserByUsername(ctx, r.Username)
			if err != nil || u == nil {
				t.Fatalf("read user for row %d: %v", i, err)
			}
			if !u.MustChangePassword {
				t.Errorf("row %d user must have must_change_password set", i)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(requestPassword)); err != nil {
				t.Errorf("row %d hash does not match request-level password: %v", i, err)
			}
		}
	})

	t.Run("admin_school request-level password applies to own-school rows", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		schoolBound := &schoolID
		requestPassword := "schoolPass123"
		rows := []StudentBulkRow{{Name: "SchoolReqPass " + uniqueSuffix(), School: schoolName, Jenjang: "sma"}}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, schoolBound, RoleAdminSchool, rows, &requestPassword, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 || results[0].Status != "success" {
			t.Fatalf("want request-level row to succeed for admin_school, got %d: %+v", successCount, results)
		}
		u, err := repo.GetUserByUsername(ctx, results[0].Username)
		if err != nil || u == nil {
			t.Fatalf("read user: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(requestPassword)); err != nil {
			t.Fatalf("hash does not match request-level password: %v", err)
		}
		if !u.MustChangePassword {
			t.Error("admin_school request-level password must set must_change_password")
		}
	})

	t.Run("CSV password column wins over request-level password for super_admin", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		csvPassword := "csvPass12345"
		requestPassword := "requestPass123"
		rows := []StudentBulkRow{{Name: "Precedence Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma", Password: &csvPassword}}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, nil, RoleSuperAdmin, rows, &requestPassword, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 || results[0].Status != "success" {
			t.Fatalf("want row to succeed, got %d: %+v", successCount, results)
		}
		u, err := repo.GetUserByUsername(ctx, results[0].Username)
		if err != nil || u == nil {
			t.Fatalf("read user: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(csvPassword)); err != nil {
			t.Fatalf("CSV password should win over request-level password: %v", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(requestPassword)) == nil {
			t.Fatal("request-level password must not be used when CSV password present")
		}
	})

	t.Run("weak request-level password fails those rows", func(t *testing.T) {
		schoolID := seedSchoolWithJenjang(t, svc, repo, []string{"sma"})
		schoolName := schoolNameByID(t, repo, schoolID)
		weak := "short"
		rows := []StudentBulkRow{
			{Name: "WeakReq Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma"},
			{Name: "ValidReq Bulk " + uniqueSuffix(), School: schoolName, Jenjang: "sma", Password: strPtr("validPass123")},
		}
		results, successCount, err := svc.ProcessStudentBulkRows(ctx, nil, RoleSuperAdmin, rows, &weak, nil)
		if err != nil {
			t.Fatalf("ProcessStudentBulkRows: %v", err)
		}
		if successCount != 1 {
			t.Fatalf("want only CSV-password row to succeed, got %d: %+v", successCount, results)
		}
		if results[0].Status != "failed" || results[0].Error != ErrWeakPassword.Error() {
			t.Fatalf("weak request-level row should fail ErrWeakPassword, got %+v", results[0])
		}
	})
}

// seedTestRegionData inserts deterministic region data into the shared test DB.
// Uses ON CONFLICT DO NOTHING for idempotency so the function is safe to call
// multiple times when other tests share the DB fixture.
func seedTestRegionData(t *testing.T, repo *repository.Repository) {
	t.Helper()
	ctx := context.Background()
	pool := repo.Pool()

	_, err := pool.Exec(ctx, `INSERT INTO province (id, name) VALUES ('73', 'SULAWESI SELATAN') ON CONFLICT DO NOTHING`)
	if err != nil {
		t.Fatalf("insert province sulsel: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO province (id, name) VALUES ('35', 'JAWA TIMUR') ON CONFLICT DO NOTHING`)
	if err != nil {
		t.Fatalf("insert province jatim: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO city (id, province_id, name) VALUES ('7371', '73', 'KOTA MAKASSAR') ON CONFLICT DO NOTHING`)
	if err != nil {
		t.Fatalf("insert city makassar: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO city (id, province_id, name) VALUES ('3578', '35', 'KOTA SURABAYA') ON CONFLICT DO NOTHING`)
	if err != nil {
		t.Fatalf("insert city surabaya: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO district (id, city_id, name) VALUES ('7371010', '7371', 'MARISO') ON CONFLICT DO NOTHING`)
	if err != nil {
		t.Fatalf("insert district mariso: %v", err)
	}
}
