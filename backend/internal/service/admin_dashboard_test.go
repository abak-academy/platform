package service

import (
	"testing"
	"time"
)

func TestPhysicalTypeValuesMatchIsPhysicalType(t *testing.T) {
	// The rule has one home. If a fourth physical type is added to
	// isPhysicalType and not here, the dashboard silently reclassifies it.
	for _, tp := range []string{"book", "merchandise", "medal"} {
		if !isPhysicalType(tp) {
			t.Errorf("%q should be physical", tp)
		}
	}
	for _, tp := range []string{"exam", "course"} {
		if isPhysicalType(tp) {
			t.Errorf("%q should be digital", tp)
		}
	}
	got := PhysicalTypeValues()
	if len(got) != 3 {
		t.Errorf("PhysicalTypeValues() = %v, want the three physical types", got)
	}
	for _, tp := range got {
		if !isPhysicalType(tp) {
			t.Errorf("PhysicalTypeValues() returned %q, which isPhysicalType rejects", tp)
		}
	}
}

func TestPreviousWindowIsEqualLengthImmediatelyBefore(t *testing.T) {
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 7, 8, 0, 0, 0, 0, jkt)
	to := time.Date(2026, 8, 7, 0, 0, 0, 0, jkt) // 30 days

	pFrom, pTo := previousWindow(from, to)

	if !pTo.Equal(from) {
		t.Errorf("prev window should end where the current one starts: %v vs %v", pTo, from)
	}
	if to.Sub(from) != pTo.Sub(pFrom) {
		t.Errorf("prev window length %v != current %v", pTo.Sub(pFrom), to.Sub(from))
	}
}

func TestKPIOmitsPrevWhenPreviousWindowIsEmpty(t *testing.T) {
	// A delta against zero is noise, not information.
	k := makeKPI(126, 0, false)
	if k.Prev != nil {
		t.Errorf("prev = %v, want nil when the previous window had no data", *k.Prev)
	}

	k2 := makeKPI(126, 117, true)
	if k2.Prev == nil || *k2.Prev != 117 {
		t.Errorf("prev = %v, want 117", k2.Prev)
	}
}

func TestResolveBucketDefaultsByRangeLength(t *testing.T) {
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, jkt)

	if got := resolveBucket("", from, from.AddDate(0, 0, 30)); got != "day" {
		t.Errorf("30 days = %q, want day", got)
	}
	if got := resolveBucket("", from, from.AddDate(0, 0, 31)); got != "day" {
		t.Errorf("31 days = %q, want day", got)
	}
	if got := resolveBucket("", from, from.AddDate(0, 0, 90)); got != "week" {
		t.Errorf("90 days = %q, want week", got)
	}
	if got := resolveBucket("day", from, from.AddDate(0, 0, 90)); got != "day" {
		t.Errorf("explicit bucket must win, got %q", got)
	}
}
