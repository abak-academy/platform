package service

import "fmt"

// ValidateKodePos enforces that a postal code is digits and nothing else.
//
// It is stored and carried as text — leading zeros are significant, so it can
// never become a number — but every character still has to be a digit. Letters
// and punctuation reach the courier API as a destination it cannot resolve, and
// an unresolvable destination is not reported: the quote silently degrades to
// the flat rate (see docs/backlog/ and the Biteship adapter), so a typo becomes
// a wrong shipping price rather than an error.
//
// Length is deliberately not checked here: Indonesian postal codes are five
// digits, but that rule belongs with the region data rather than with a string
// guard, and rejecting a length would also reject the shorter codes already
// stored on existing rows.
func ValidateKodePos(kodePos string) error {
	if kodePos == "" {
		return nil
	}
	for _, r := range kodePos {
		if r < '0' || r > '9' {
			return fmt.Errorf("%w: %q", ErrInvalidKodePos, kodePos)
		}
	}
	return nil
}
