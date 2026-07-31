package service

import "strings"

// normalizeAnswer applies the confirmed FB-10 matching rule (FR-23, decided
// 2026-07-30 — see docs/backlog/e2-exam-authoring.md §FB-10): trim leading/
// trailing whitespace, Unicode-lowercase, then collapse every internal run of
// whitespace to one space. Deliberately excludes accent folding, punctuation
// stripping and number-word equivalence — those are a second accepted answer,
// not a grader rule.
func normalizeAnswer(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// matchesAnyAccepted reports whether submitted matches any of the accepted
// answers under normalizeAnswer, by exact string equality of the normalised
// forms only.
func matchesAnyAccepted(submitted string, accepted []string) bool {
	if submitted == "" {
		return false
	}
	norm := normalizeAnswer(submitted)
	for _, a := range accepted {
		if normalizeAnswer(a) == norm {
			return true
		}
	}
	return false
}
