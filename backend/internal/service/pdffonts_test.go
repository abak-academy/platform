package service

import (
	"testing"
)

// allFontFamilies lists the FR-7a closed set in a fixed order.
var allFontFamilies = []string{
	FontSourceSerif4,
	FontPublicSans,
	FontCinzel,
	FontPlayfairDisplay,
	FontCormorantGaramond,
	FontGreatVibes,
}

func TestResolveFontFamily_KnownFamiliesResolveToThemselves(t *testing.T) {
	t.Parallel()
	for _, family := range allFontFamilies {
		if got := ResolveFontFamily(family); got != family {
			t.Errorf("ResolveFontFamily(%q) = %q, want %q", family, got, family)
		}
	}
}

func TestResolveFontFamily_UnknownFallsBackToBrandDefault(t *testing.T) {
	t.Parallel()
	got := ResolveFontFamily("comic_sans")
	if got != defaultFontFamily {
		t.Errorf("ResolveFontFamily(unknown) = %q, want brand default %q", got, defaultFontFamily)
	}
}
