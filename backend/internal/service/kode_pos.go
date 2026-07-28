package service

import "fmt"

// ValidateKodePos enforces that a postal code is digits and nothing else.
//
// It is stored and carried as text — leading zeros are significant, so it can
// never become a number — but every character still has to be a digit. Letters
// and punctuation reach the courier API as a destination it cannot resolve, and
// an unresolvable destination is not reported: the quote silently degrades to
// the flat rate (see the Biteship adapter), so a typo becomes a wrong shipping
// price rather than an error.
//
// This checks the characters, not whether there are any. An empty string passes,
// because empty means "not provided" and only two callers can even reach it with
// one: profile update and admin registration, where the postal code is optional.
// The two paths where a postal code is required reject empty before they get
// here — the quote handler with a 400, and PatchCart via nilIfEmpty.
//
// Length is deliberately not checked: Indonesian postal codes are five digits,
// but that rule belongs with the region data rather than with a string guard,
// and rejecting a length would also reject the shorter codes already stored.
func ValidateKodePos(kodePos string) error {
	for _, r := range kodePos {
		if r < '0' || r > '9' {
			return fmt.Errorf("%w: %q", ErrInvalidKodePos, kodePos)
		}
	}
	return nil
}
