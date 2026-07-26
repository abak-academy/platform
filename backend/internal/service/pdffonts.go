package service

import "embed"

//go:embed fonts
var fontFS embed.FS

// Closed set of bundled OFL font family identifiers (FR-7a). Any other
// identifier is unknown and must fall back to defaultFontFamily.
const (
	FontSourceSerif4      = "source_serif_4"
	FontPublicSans        = "public_sans"
	FontCinzel            = "cinzel"
	FontPlayfairDisplay   = "playfair_display"
	FontCormorantGaramond = "cormorant_garamond"
	FontGreatVibes        = "great_vibes"

	defaultFontFamily = FontSourceSerif4
)

// fontFiles maps each family to its embedded TTF per style key ("" / "B").
// Families with only one committed weight reuse that weight for both ""
// and "B" so SetFont never fails regardless of requested style.
var fontFiles = map[string]map[string]string{
	FontSourceSerif4: {
		"":  "fonts/source_serif_4/SourceSerif4-SemiBold.ttf",
		"B": "fonts/source_serif_4/SourceSerif4-Bold.ttf",
	},
	FontPublicSans: {
		"":  "fonts/public_sans/PublicSans-Regular.ttf",
		"B": "fonts/public_sans/PublicSans-SemiBold.ttf",
	},
	FontCinzel: {
		"":  "fonts/cinzel/Cinzel-Bold.ttf",
		"B": "fonts/cinzel/Cinzel-Bold.ttf",
	},
	FontPlayfairDisplay: {
		"":  "fonts/playfair_display/PlayfairDisplay-Bold.ttf",
		"B": "fonts/playfair_display/PlayfairDisplay-Bold.ttf",
	},
	FontCormorantGaramond: {
		"":  "fonts/cormorant_garamond/CormorantGaramond-SemiBold.ttf",
		"B": "fonts/cormorant_garamond/CormorantGaramond-SemiBold.ttf",
	},
	FontGreatVibes: {
		"":  "fonts/great_vibes/GreatVibes-Regular.ttf",
		"B": "fonts/great_vibes/GreatVibes-Regular.ttf",
	},
}

// ResolveFontFamily maps a persisted font family identifier to a registered
// family. An unknown identifier falls back to the brand default rather than
// erroring, so a stale or corrupt layout still renders (FR-7a, Invariant 8).
func ResolveFontFamily(family string) string {
	if _, ok := fontFiles[family]; ok {
		return family
	}
	return defaultFontFamily
}
