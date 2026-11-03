package service

import (
	"testing"

	"akademi-bimbel/internal/model"
)

func TestEffectiveExamRules(t *testing.T) {
	tests := []struct {
		name           string
		formatVersion  int
		mode           string
		navigation     string
		timer          string
		wantNavigation string
		wantTimer      string
	}{
		{"format 1 standard", 1, "standard", "free", "overall", "free", "overall"},
		{"format 1 utbk ignores raw columns", 1, "utbk", "free", "overall", "sequential", "per_test"},
		{"format 1 ielts ignores raw columns", 1, "ielts", "free", "overall", "sequential", "per_test"},
		{"format 1 standard ignores raw columns", 1, "standard", "sequential", "per_test", "free", "overall"},
		{"format 2 free overall", 2, "standard", "free", "overall", "free", "overall"},
		{"format 2 sequential overall", 2, "standard", "sequential", "overall", "sequential", "overall"},
		{"format 2 sequential per_test", 2, "standard", "sequential", "per_test", "sequential", "per_test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNavigation, gotTimer := effectiveExamRules(tt.formatVersion, tt.mode, tt.navigation, tt.timer)
			if gotNavigation != tt.wantNavigation || gotTimer != tt.wantTimer {
				t.Errorf("effectiveExamRules(%d, %q, %q, %q) = (%q, %q), want (%q, %q)",
					tt.formatVersion, tt.mode, tt.navigation, tt.timer, gotNavigation, gotTimer, tt.wantNavigation, tt.wantTimer)
			}
		})
	}
}

func TestEffectiveExamRules_matchesRuntimeHelpers(t *testing.T) {
	for _, formatVersion := range []int{1, 2} {
		for _, mode := range []string{"standard", "utbk", "ielts"} {
			for _, navigation := range []string{"free", "sequential"} {
				for _, timer := range []string{"overall", "per_test"} {
					exam := &model.Exam{FormatVersion: formatVersion, Mode: mode, NavigationPolicy: navigation, TimerMode: timer}
					gotNavigation, gotTimer := effectiveExamRules(formatVersion, mode, navigation, timer)
					if (gotNavigation == "sequential") != sequentialSession(exam) {
						t.Errorf("fv=%d mode=%s nav=%s timer=%s: navigation %q disagrees with sequentialSession=%v",
							formatVersion, mode, navigation, timer, gotNavigation, sequentialSession(exam))
					}
					if (gotTimer == "per_test") != sectionTimer(exam) {
						t.Errorf("fv=%d mode=%s nav=%s timer=%s: timer %q disagrees with sectionTimer=%v",
							formatVersion, mode, navigation, timer, gotTimer, sectionTimer(exam))
					}
				}
			}
		}
	}
}
