package service

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
)

func TestParseSchoolBulkCSV(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		data := []byte("name,code,npsn,school_types,alamat\nSMAN 1 Jakarta,sman1jkt,20000001,sma|smk,Jl. Merdeka No.1\n")
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d", len(rows))
		}
		r := rows[0]
		if r.Name != "SMAN 1 Jakarta" || r.Code != "sman1jkt" {
			t.Errorf("unexpected row: %+v", r)
		}
		if r.NPSN == nil || *r.NPSN != "20000001" {
			t.Errorf("want npsn 20000001, got %v", r.NPSN)
		}
		if r.Alamat == nil || *r.Alamat != "Jl. Merdeka No.1" {
			t.Errorf("want alamat, got %v", r.Alamat)
		}
		if len(r.SchoolTypes) != 2 || r.SchoolTypes[0] != "sma" || r.SchoolTypes[1] != "smk" {
			t.Errorf("want [sma smk], got %v", r.SchoolTypes)
		}
	})

	t.Run("missing name header returns ErrMissingSchoolCSVHeader", func(t *testing.T) {
		data := []byte("code,npsn\nsman1,2000\n")
		_, err := ParseSchoolBulkCSV(data)
		if !errors.Is(err, ErrMissingSchoolCSVHeader) {
			t.Errorf("want ErrMissingSchoolCSVHeader, got %v", err)
		}
	})

	t.Run("missing code header returns ErrMissingSchoolCSVHeader", func(t *testing.T) {
		data := []byte("name,npsn\nSMAN 1,2000\n")
		_, err := ParseSchoolBulkCSV(data)
		if !errors.Is(err, ErrMissingSchoolCSVHeader) {
			t.Errorf("want ErrMissingSchoolCSVHeader, got %v", err)
		}
	})

	t.Run("empty file returns ErrMissingSchoolCSVHeader", func(t *testing.T) {
		_, err := ParseSchoolBulkCSV([]byte(""))
		if !errors.Is(err, ErrMissingSchoolCSVHeader) {
			t.Errorf("want ErrMissingSchoolCSVHeader, got %v", err)
		}
	})

	t.Run("malformed CSV returns ErrInvalidCSV", func(t *testing.T) {
		data := []byte("name,code\n\"Budi,x1\n")
		_, err := ParseSchoolBulkCSV(data)
		if !errors.Is(err, ErrInvalidCSV) {
			t.Errorf("want ErrInvalidCSV, got %v", err)
		}
	})

	t.Run("1001 data rows exceeds limit", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("name,code\n")
		for i := 0; i < maxBulkRows+1; i++ {
			sb.WriteString("School,code1\n")
		}
		_, err := ParseSchoolBulkCSV([]byte(sb.String()))
		if !errors.Is(err, ErrRowLimitExceeded) {
			t.Errorf("want ErrRowLimitExceeded, got %v", err)
		}
	})

	t.Run("pipe-separated school_types", func(t *testing.T) {
		data := []byte("name,code,school_types\nS,c1,sma|smk\n")
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if len(rows[0].SchoolTypes) != 2 || rows[0].SchoolTypes[0] != "sma" || rows[0].SchoolTypes[1] != "smk" {
			t.Errorf("want [sma smk], got %v", rows[0].SchoolTypes)
		}
	})

	t.Run("quoted comma-separated school_types", func(t *testing.T) {
		data := []byte("name,code,school_types\nS,c1,\"sma, smk\"\n")
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if len(rows[0].SchoolTypes) != 2 || rows[0].SchoolTypes[0] != "sma" || rows[0].SchoolTypes[1] != "smk" {
			t.Errorf("want [sma smk], got %v", rows[0].SchoolTypes)
		}
	})

	t.Run("empty school_types cell is empty slice not nil", func(t *testing.T) {
		data := []byte("name,code,school_types\nS,c1,\n")
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if rows[0].SchoolTypes == nil {
			t.Error("want empty slice, got nil")
		}
		if len(rows[0].SchoolTypes) != 0 {
			t.Errorf("want empty, got %v", rows[0].SchoolTypes)
		}
	})

	t.Run("school_types column absent is empty slice not nil", func(t *testing.T) {
		data := []byte("name,code\nS,c1\n")
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if rows[0].SchoolTypes == nil || len(rows[0].SchoolTypes) != 0 {
			t.Errorf("want empty slice, got %v", rows[0].SchoolTypes)
		}
	})

	t.Run("unknown extra column ignored", func(t *testing.T) {
		data := []byte("name,code,foo\nS,c1,bar\n")
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if rows[0].Name != "S" || rows[0].Code != "c1" {
			t.Errorf("unexpected row: %+v", rows[0])
		}
	})

	t.Run("UTF-8 BOM on header is stripped", func(t *testing.T) {
		data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("name,code\nS,c1\n")...)
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if len(rows) != 1 || rows[0].Name != "S" || rows[0].Code != "c1" {
			t.Errorf("BOM should not break header match, got %+v", rows)
		}
	})

	t.Run("cell values are trimmed and blank rows skipped", func(t *testing.T) {
		data := []byte("name,code,npsn\n SMAN 1 , C1 , 20100001 \n\n")
		rows, err := ParseSchoolBulkCSV(data)
		if err != nil {
			t.Fatalf("ParseSchoolBulkCSV: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row, got %d", len(rows))
		}
		if rows[0].Name != "SMAN 1" || rows[0].Code != "C1" {
			t.Errorf("want trimmed cells, got %+v", rows[0])
		}
		if rows[0].NPSN == nil || *rows[0].NPSN != "20100001" {
			t.Errorf("want trimmed npsn, got %v", rows[0].NPSN)
		}
		if rows[0].Row != 2 {
			t.Errorf("want row 2, got %d", rows[0].Row)
		}
	})
}

func TestProcessSchoolBulkRows_Integration(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	t.Run("duplicate code fails at row level, later row still succeeds, order preserved", func(t *testing.T) {
		existingCode := "sb_" + uniqueSuffix()
		if _, err := svc.CreateSchool(ctx, "Existing School", existingCode, nil, nil, nil); err != nil {
			t.Fatalf("seed CreateSchool: %v", err)
		}
		newCode := "sb_" + uniqueSuffix()
		rows := []SchoolBulkRow{
			{Name: "First", Code: "sb_first_" + uniqueSuffix()},
			{Name: "Duplicate", Code: existingCode},
			{Name: "Later", Code: newCode},
		}
		results, successCount, err := svc.ProcessSchoolBulkRows(ctx, rows, nil)
		if err != nil {
			t.Fatalf("ProcessSchoolBulkRows: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("want 3 results, got %d", len(results))
		}
		if results[0].Status != "success" || results[0].Error != "" {
			t.Errorf("want row0 success, got %+v", results[0])
		}
		if results[1].Status != "failed" || results[1].Error != ErrSchoolCodeTaken.Error() {
			t.Errorf("want row1 failed with ErrSchoolCodeTaken, got %+v", results[1])
		}
		if results[2].Status != "success" || results[2].Error != "" {
			t.Errorf("want row2 success (batch continues after row1 failure), got %+v", results[2])
		}
		if results[0].Name != "First" || results[1].Name != "Duplicate" || results[2].Name != "Later" {
			t.Errorf("order not preserved: %+v", results)
		}
		if successCount != 2 {
			t.Errorf("want successCount=2, got %d", successCount)
		}
	})

	t.Run("invalid name fails at row level and later row still succeeds", func(t *testing.T) {
		validCode := "sb_" + uniqueSuffix()
		rows := []SchoolBulkRow{
			{Row: 2, Name: "...", Code: "sb_" + uniqueSuffix()},
			{Row: 3, Name: "Valid Bulk School", Code: validCode},
		}
		results, successCount, err := svc.ProcessSchoolBulkRows(ctx, rows, nil)
		if err != nil {
			t.Fatalf("ProcessSchoolBulkRows: %v", err)
		}
		if successCount != 1 {
			t.Errorf("want successCount=1, got %d", successCount)
		}
		if len(results) != 2 {
			t.Fatalf("want 2 results, got %d", len(results))
		}
		if results[0].Status != "failed" || results[0].Error != ErrInvalidSchoolName.Error() {
			t.Errorf("want row0 invalid-name failure, got %+v", results[0])
		}
		if results[1].Status != "success" || results[1].Error != "" {
			t.Errorf("want row1 success, got %+v", results[1])
		}
		if results[0].Row != 2 || results[1].Row != 3 {
			t.Errorf("row order not preserved: %+v", results)
		}
		found := findSchoolByCode(t, svc, validCode)
		if found.Code != validCode {
			t.Errorf("created valid row code: want %q, got %q", validCode, found.Code)
		}
	})

	t.Run("onProgress reaches 100 at end", func(t *testing.T) {
		rows := []SchoolBulkRow{
			{Name: "A", Code: "sb_" + uniqueSuffix()},
		}
		var calls []int
		_, _, err := svc.ProcessSchoolBulkRows(ctx, rows, func(pct int) { calls = append(calls, pct) })
		if err != nil {
			t.Fatalf("ProcessSchoolBulkRows: %v", err)
		}
		if len(calls) == 0 || calls[len(calls)-1] != 100 {
			t.Errorf("want final progress 100, got %v", calls)
		}
	})
}

func TestBuildSchoolBulkResultCSV(t *testing.T) {
	results := []SchoolBulkResultRow{
		{Row: 2, Name: "SMAN 1 Jakarta", Code: "sman1", NPSN: "2000", SchoolTypes: "sma|smk", Alamat: "Jl. X", Status: "success"},
		{Row: 3, Name: "=cmd|'/c calc'!A1", Code: "sman2", Status: "failed", Error: ErrSchoolCodeTaken.Error()},
		{Row: 4, Name: "...", Code: "sman3", Status: "failed", Error: ErrInvalidSchoolName.Error()},
	}
	data := BuildSchoolBulkResultCSV(results)

	r := csv.NewReader(strings.NewReader(string(data)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read back csv: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("want 4 records (header + 3 rows), got %d", len(records))
	}
	wantHeader := []string{"row", "name", "code", "npsn", "school_types", "alamat", "status", "error"}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Errorf("header[%d]: want %s, got %s", i, h, records[0][i])
		}
	}
	wantRow1 := []string{"2", "SMAN 1 Jakarta", "sman1", "2000", "sma|smk", "Jl. X", "success", ""}
	for i, v := range wantRow1 {
		if records[1][i] != v {
			t.Errorf("row1[%d]: want %s, got %s", i, v, records[1][i])
		}
	}
	// NFR-2: a school name that looks like a spreadsheet formula must be
	// neutralised in the export, never left as a live leading '='.
	if strings.HasPrefix(records[2][1], "=") {
		t.Errorf("formula-injection guard did not fire: name %q still starts with '=' in the exported CSV", records[2][1])
	}
	if records[2][1] != "'=cmd|'/c calc'!A1" {
		t.Errorf("want neutralised name, got %q", records[2][1])
	}
	if records[3][7] != ErrInvalidSchoolName.Error() {
		t.Errorf("want invalid name error in error column, got %q", records[3][7])
	}
}

// frontendSchoolBulkTemplateCSV is the exact byte sequence the "download
// template" button hands the admin. Copied verbatim from
// web/components/admin/SchoolBulkImportModal.tsx (TEMPLATE_HEADER +
// TEMPLATE_EXAMPLE_ROW, joined with "\n" and newline-terminated by
// buildTemplateCSV). That file's matching test asserts the same literal and
// names this constant, so a divergence fails on one side or the other.
const frontendSchoolBulkTemplateCSV = "name,code,npsn,school_types,alamat\n" +
	"SMAN 1 Jakarta,SMAN1JKT,20100001,SMA|SMK,\"Jl. Sudirman No. 1\"\n" +
	"SMPN 5 Bandung,SMPN5BDG,,SMP,\n"

// TestFrontendTemplateParsesUnmodified is FR-31: the template as downloaded and
// re-uploaded untouched must parse cleanly — never a header error, never a
// parse error.
func TestFrontendTemplateParsesUnmodified(t *testing.T) {
	rows, err := ParseSchoolBulkCSV([]byte(frontendSchoolBulkTemplateCSV))
	if err != nil {
		t.Fatalf("the frontend template must parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 example rows, got %d", len(rows))
	}
	r := rows[0]
	if r.Name != "SMAN 1 Jakarta" || r.Code != "SMAN1JKT" {
		t.Errorf("unexpected example row: %+v", r)
	}
	if r.NPSN == nil || *r.NPSN != "20100001" {
		t.Errorf("want npsn 20100001, got %v", r.NPSN)
	}
	if r.Alamat == nil || *r.Alamat != "Jl. Sudirman No. 1" {
		t.Errorf("want alamat, got %v", r.Alamat)
	}
	// §D-4: the pipe encoding must survive as two distinct school types, not
	// one cell containing a literal "sma|smk".
	if len(r.SchoolTypes) != 2 {
		t.Fatalf("want 2 school types from the pipe-encoded cell, got %d (%v)", len(r.SchoolTypes), r.SchoolTypes)
	}
	if r.SchoolTypes[0] != "SMA" || r.SchoolTypes[1] != "SMK" {
		t.Errorf("want [SMA SMK], got %v", r.SchoolTypes)
	}
}
