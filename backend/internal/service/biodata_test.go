package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
)

// TestBiodataMissingFields_OnlyGradeMissing pins that a student with a school
// and dob already set, but no grade, is reported as missing only "grade" —
// not the other two fields that are actually present.
func TestBiodataMissingFields_OnlyGradeMissing(t *testing.T) {
	dob := time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)
	school := "school-uuid"
	u := &model.User{SchoolID: &school, DOB: &dob}

	missing := biodataMissingFields(u)

	if len(missing) != 1 || missing[0] != "grade" {
		t.Fatalf("want [grade], got %v", missing)
	}
}

// TestBiodataIncompleteError_NamesOnlyMissingFields is the RED case for the
// defect: the old ErrBiodataIncomplete was a single static sentence naming
// school/grade/dob regardless of what was actually missing, which told a
// student with a school on file to go fill in a school.
func TestBiodataIncompleteError_NamesOnlyMissingFields(t *testing.T) {
	err := &BiodataIncompleteError{Missing: []string{"grade"}}

	msg := err.Error()

	if !strings.Contains(msg, "kelas") {
		t.Errorf("want message to name grade (kelas), got %q", msg)
	}
	if strings.Contains(msg, "sekolah") {
		t.Errorf("want message NOT to name school (sekolah) when only grade is missing, got %q", msg)
	}
	if strings.Contains(msg, "tanggal lahir") {
		t.Errorf("want message NOT to name dob (tanggal lahir) when only grade is missing, got %q", msg)
	}
}

func TestBiodataIncompleteError_SatisfiesErrBiodataIncomplete(t *testing.T) {
	err := &BiodataIncompleteError{Missing: []string{"dob"}}
	if !errors.Is(err, ErrBiodataIncomplete) {
		t.Error("want BiodataIncompleteError to satisfy errors.Is(err, ErrBiodataIncomplete)")
	}
}
