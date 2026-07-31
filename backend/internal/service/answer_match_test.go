package service

import "testing"

// TestNormalizeAnswer_And_MatchesAnyAccepted pins the confirmed FB-10 matching
// rule at its boundaries (FR-23): trim -> Unicode lowercase -> collapse internal
// whitespace runs to one space -> exact match. No accent folding, no punctuation
// stripping, no number-word equivalence.
func TestMatchesAnyAccepted_pinsTheConfirmedRule(t *testing.T) {
	tests := []struct {
		name      string
		submitted string
		accepted  []string
		want      bool
	}{
		{"trim + lowercase matches", " DUA ", []string{"dua"}, true},
		{"trim + lowercase + collapse whitespace matches", "DUA   KALI", []string{"dua kali"}, true},
		{"accent is not folded", "dúa", []string{"dua"}, false},
		{"punctuation is not stripped", "dua!", []string{"dua"}, false},
		{"no number-word equivalence", "2", []string{"dua"}, false},
		{"empty submitted never matches", "", []string{"dua"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAnyAccepted(tt.submitted, tt.accepted)
			if got != tt.want {
				t.Errorf("matchesAnyAccepted(%q, %v) = %v, want %v", tt.submitted, tt.accepted, got, tt.want)
			}
		})
	}
}

func TestNormalizeAnswer_collapsesWhitespaceAndLowercases(t *testing.T) {
	if got := normalizeAnswer("  DUA   KALI  "); got != "dua kali" {
		t.Errorf("normalizeAnswer = %q, want %q", got, "dua kali")
	}
}
