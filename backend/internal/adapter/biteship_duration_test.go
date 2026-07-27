package adapter

import "testing"

// Biteship's duration field is prose, and the wording varies by carrier and by
// language. strconv.Atoi returned 0 for every one of these, which is why the
// storefront showed "0 days" against every rate.
func TestParseDurationDays(t *testing.T) {
	cases := []struct {
		name     string
		duration string
		want     int
	}{
		{"english range", "1 - 2 days", 2},
		{"indonesian range", "2 - 3 hari", 3},
		{"no spaces around dash", "1-2 days", 2},
		{"single english", "3 days", 3},
		{"single indonesian", "4 hari", 4},
		{"bare number", "2", 2},
		{"same day has no day count", "Same day", 0},
		{"empty", "", 0},
		{"words only", "Besok sampai tujuan", 0},
		{"two digits", "10 - 14 days", 14},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseDurationDays(c.duration); got != c.want {
				t.Errorf("parseDurationDays(%q) = %d, want %d", c.duration, got, c.want)
			}
		})
	}
}

// A range must resolve to its upper bound. Showing the lower one promises the
// buyer a delivery date the carrier did not commit to, and the storefront prints
// this number next to a price the buyer is about to pay.
func TestParseDurationDays_RangeTakesUpperBound(t *testing.T) {
	if got := parseDurationDays("1 - 5 days"); got != 5 {
		t.Errorf("want the upper bound 5, got %d — quoting the lower bound over-promises", got)
	}
}

func TestParsePricing_MapsDurationOntoEstimatedDays(t *testing.T) {
	c := &BiteshipClient{}
	rates := c.parsePricing([]biteshipPricingItem{
		{CourierName: "JNE", CourierServiceName: "Reguler", Price: 10000, Duration: "1 - 2 days"},
		{CourierName: "Tiki", CourierServiceName: "Same Day Service", Price: 30000, Duration: "Same day"},
	})

	if len(rates) != 2 {
		t.Fatalf("want 2 rates, got %d", len(rates))
	}
	if rates[0].EstimatedDays != 2 {
		t.Errorf("JNE: want 2 days from %q, got %d", "1 - 2 days", rates[0].EstimatedDays)
	}
	// Left at 0 on purpose: the storefront renders no estimate rather than
	// "0 days", which would read as a delivery promise nobody made.
	if rates[1].EstimatedDays != 0 {
		t.Errorf("Tiki same-day: want 0 (no day count), got %d", rates[1].EstimatedDays)
	}
}
