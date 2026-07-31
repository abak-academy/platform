package service

import "testing"

// TestIsPhysicalType pins isPhysicalType (store.go:1495) to exactly
// {book, merchandise, medal} and rejects {course, exam}. This is the Go half
// of FR-F-3; its web counterpart is web/lib/shipping.test.ts, which pins the
// TypeScript isPhysicalType/isDigitalType pair over the same ProductType set.
// The two suites do not share a definition — this only makes a divergence
// between the two languages fail loudly.
func TestIsPhysicalType(t *testing.T) {
	physical := []string{"book", "merchandise", "medal"}
	for _, pt := range physical {
		if !isPhysicalType(pt) {
			t.Errorf("isPhysicalType(%q) = false, want true", pt)
		}
	}

	digital := []string{"course", "exam"}
	for _, pt := range digital {
		if isPhysicalType(pt) {
			t.Errorf("isPhysicalType(%q) = true, want false", pt)
		}
	}
}
