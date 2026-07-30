package service

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
)

// The payload from B-8's acceptance criteria.
const evilName = `=cmd|' /C calc'!A0`

func TestCsvSafeField(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"equals", "=1+1", "'=1+1"},
		{"plus", "+1", "'+1"},
		{"minus", "-1", "'-1"},
		{"at", "@SUM(A1)", "'@SUM(A1)"},
		{"tab", "\tx", "'\tx"},
		{"carriage return", "\rx", "'\rx"},
		{"dde payload", evilName, "'" + evilName},
		{"benign name", "Budi Santoso", "Budi Santoso"},
		{"empty", "", ""},
		{"formula char not leading", "a=1", "a=1"},
		{"utf8 leading rune untouched", "Ärger", "Ärger"},
		// A multi-byte rune must not be mistaken for a sentinel byte.
		{"utf8 em dash is not minus", "—dash", "—dash"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := csvSafeField(tc.in); got != tc.want {
				t.Errorf("csvSafeField(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// firstDataRecord parses CSV bytes and returns the record after the header.
func firstDataRecord(t *testing.T, b []byte) []string {
	t.Helper()
	recs, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		t.Fatalf("export is not parseable CSV: %v", err)
	}
	if len(recs) < 2 {
		t.Fatalf("expected a header and one data row, got %d records", len(recs))
	}
	return recs[1]
}

// Writer 1 of 3 — the results export behind the E3 results tab.
func TestBuildSchoolResultsCSV_neutralisesFormulaLead(t *testing.T) {
	username := evilName
	score := 42.5
	submitted := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	out := BuildSchoolResultsCSV([]model.AdminResultRow{{
		StudentName: evilName,
		Username:    &username,
		Score:       &score,
		SubmittedAt: &submitted,
	}})

	rec := firstDataRecord(t, out)
	if rec[0] != "'"+evilName {
		t.Errorf("name not neutralised: got %q", rec[0])
	}
	if rec[1] != "'"+evilName {
		t.Errorf("username not neutralised: got %q", rec[1])
	}
}

// A negative score is machine-generated (point_wrong subtracts) and must stay a
// number, not become text — this is why the writer sanitises per field.
func TestBuildSchoolResultsCSV_leavesNegativeScoreTyped(t *testing.T) {
	score := -5.0
	out := BuildSchoolResultsCSV([]model.AdminResultRow{{
		StudentName: "Budi",
		Score:       &score,
	}})

	rec := firstDataRecord(t, out)
	if strings.HasPrefix(rec[2], "'") {
		t.Errorf("negative score was sanitised into text: got %q", rec[2])
	}
	if rec[2] != "-5" {
		t.Errorf("score = %q, want %q", rec[2], "-5")
	}
}

// Writer 2 of 3.
func TestBuildCredentialsResultCSV_neutralisesFormulaLead(t *testing.T) {
	out := BuildCredentialsResultCSV([]StudentBulkResultRow{{
		Name:         evilName,
		Username:     "+budi",
		TempPassword: "-secret",
		Error:        "@err",
	}})

	rec := firstDataRecord(t, out)
	for i, want := range []string{"'" + evilName, "'+budi", "'-secret", "'@err"} {
		if rec[i] != want {
			t.Errorf("field %d = %q, want %q", i, rec[i], want)
		}
	}
}

// Writer 3 of 3 — the pattern E4's school importer is told to copy.
func TestBuildStudentBulkResultCSV_neutralisesFormulaLead(t *testing.T) {
	out := BuildStudentBulkResultCSV([]StudentBulkResultRow{{
		Name:         evilName,
		School:       "=SUM(A1)",
		Email:        "budi@example.com",
		Status:       "ok",
		Username:     "budi",
		TempPassword: "pw",
		Error:        "",
	}})

	rec := firstDataRecord(t, out)
	if rec[0] != "'"+evilName {
		t.Errorf("name not neutralised: got %q", rec[0])
	}
	if rec[1] != "'=SUM(A1)" {
		t.Errorf("school not neutralised: got %q", rec[1])
	}
	if rec[2] != "budi@example.com" {
		t.Errorf("benign email was altered: got %q", rec[2])
	}
}

// A comma or quote in a name must still round-trip — encoding/csv owns quoting,
// and the sanitiser must not interfere with it.
func TestCsvWriters_preserveRFC4180Quoting(t *testing.T) {
	out := BuildCredentialsResultCSV([]StudentBulkResultRow{{
		Name:     `Santoso, Budi "BS"`,
		Username: "budi",
	}})

	rec := firstDataRecord(t, out)
	if rec[0] != `Santoso, Budi "BS"` {
		t.Errorf("name did not round-trip: got %q", rec[0])
	}
}
