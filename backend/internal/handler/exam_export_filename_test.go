package handler

import (
	"testing"
	"time"
)

func TestSanitizeFilenameSegment(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Try Out UTBK 2026", "Try-Out-UTBK-2026"},
		{"slash and quotes", `Ujian/"Khusus"`, "Ujian-Khusus"},
		{"collapses runs", "A   /\\  B", "A-B"},
		{"trims leading and trailing", "  Ujian  ", "Ujian"},
		{"unicode and emoji dropped, letters kept", "Ujian 😀 Spesial", "Ujian-Spesial"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeFilenameSegment(tc.in); got != tc.want {
				t.Errorf("sanitizeFilenameSegment(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExamExportTimestamp_ParsesAsWIBWallClock(t *testing.T) {
	ts := examExportTimestamp()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load Asia/Jakarta: %v", err)
	}
	parsed, err := time.ParseInLocation("20060102-150405", ts, loc)
	if err != nil {
		t.Fatalf("examExportTimestamp() = %q, not parseable as 20060102-150405: %v", ts, err)
	}
	if since := time.Since(parsed); since < 0 || since > time.Minute {
		t.Errorf("examExportTimestamp() = %q, too far from now (%v)", ts, since)
	}
}
