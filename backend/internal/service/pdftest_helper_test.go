package service

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pdfVisualInspectionDir names where rasterized PNGs are kept for the NFR-1
// manual visual-inspection gate: page orientation, field centering, and edge
// overflow are confirmed by looking at these images, not by the automated
// geometry/ink assertions that use renderToPNG (memory:
// pdf-layout-needs-visual-verification).
//
// Opt-in via PDF_VISUAL_DIR — the images are only useful to someone about to
// look at them, and t.TempDir() (used for the render itself) is wiped when the
// test ends. Returns "" when unset, which skips persisting entirely.
func pdfVisualInspectionDir() string { return os.Getenv("PDF_VISUAL_DIR") }

// renderToPNG writes pdf to disk and rasterizes page 1 at 150dpi with
// pdftoppm, decoding the result as an image.Image, so correctness
// assertions run against rendered geometry and ink rather than PDF byte
// substrings (FR-30): a bytes.Contains(pdf, "(Test)")-style check is true of
// a blank page, an inverted page, and a correct page alike. It also fails if
// a second page is produced — certificates and exam cards are exactly one
// page by construction, and a stray second page is the direct regression
// signature of R1's historical bug. Skips cleanly (not fails) when pdftoppm
// is unavailable (NFR-7), so CI without poppler degrades rather than breaks.
func renderToPNG(t *testing.T, pdf []byte) image.Image {
	t.Helper()
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed; cannot verify rasterized output")
	}

	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(pdfPath, pdf, 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	prefix := filepath.Join(dir, "doc")
	cmd := exec.Command("pdftoppm", "-r", "150", "-png", pdfPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %v\n%s", err, out)
	}

	page1 := prefix + "-1.png"
	if _, err := os.Stat(page1); err != nil {
		t.Fatalf("expected rasterized page 1: %v", err)
	}
	if _, err := os.Stat(prefix + "-2.png"); err == nil {
		t.Fatalf("expected exactly one page, found a second page")
	}

	data, err := os.ReadFile(page1)
	if err != nil {
		t.Fatalf("read rasterized png: %v", err)
	}

	if visualDir := pdfVisualInspectionDir(); visualDir != "" {
		if err := os.MkdirAll(visualDir, 0o755); err == nil {
			name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
			persisted := filepath.Join(visualDir, name+".png")
			if werr := os.WriteFile(persisted, data, 0o644); werr == nil {
				t.Logf("rasterized for visual inspection: %s", persisted)
			}
		}
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode rasterized png: %v", err)
	}
	return img
}

// avgColorAt samples a small block centered on (xMm,yMm) and returns its
// average RGB, converting from the mm coordinate space of a pageWidthMm x
// pageHeightMm page into the image's own pixel resolution.
func avgColorAt(img image.Image, pageWidthMm, pageHeightMm, xMm, yMm float64) (r, g, b float64) {
	bounds := img.Bounds()
	pxPerMmX := float64(bounds.Dx()) / pageWidthMm
	pxPerMmY := float64(bounds.Dy()) / pageHeightMm
	cx := int(xMm * pxPerMmX)
	cy := int(yMm * pxPerMmY)

	const half = 4
	var sr, sg, sb, n float64
	for dy := -half; dy <= half; dy++ {
		for dx := -half; dx <= half; dx++ {
			x, y := cx+dx, cy+dy
			if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
				continue
			}
			cr, cg, cb, _ := img.At(x, y).RGBA()
			sr += float64(cr >> 8)
			sg += float64(cg >> 8)
			sb += float64(cb >> 8)
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return sr / n, sg / n, sb / n
}

func colorDistance(r1, g1, b1, r2, g2, b2 float64) float64 {
	dr, dg, db := r1-r2, g1-g2, b1-b2
	return dr*dr + dg*dg + db*db
}

// regionMinBrightness scans a grid over [xMinMm,xMaxMm]x[yMinMm,yMaxMm] and
// returns the darkest pixel found, as an R+G+B sum (0=black, 765=white). It
// detects glyph ink against a much brighter background without needing to
// know a field's exact text color or glyph footprint — used to confirm a
// stamped field actually landed in its expected mm-region rather than
// somewhere else on the page (e.g. after a Y-axis inversion).
func regionMinBrightness(img image.Image, pageWidthMm, pageHeightMm, xMinMm, yMinMm, xMaxMm, yMaxMm float64) float64 {
	bounds := img.Bounds()
	pxPerMmX := float64(bounds.Dx()) / pageWidthMm
	pxPerMmY := float64(bounds.Dy()) / pageHeightMm
	minSum := 3 * 255.0
	for yMm := yMinMm; yMm <= yMaxMm; yMm += 0.5 {
		for xMm := xMinMm; xMm <= xMaxMm; xMm += 0.5 {
			x, y := int(xMm*pxPerMmX), int(yMm*pxPerMmY)
			if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
				continue
			}
			r, g, b, _ := img.At(x, y).RGBA()
			sum := float64(r>>8) + float64(g>>8) + float64(b>>8)
			if sum < minSum {
				minSum = sum
			}
		}
	}
	return minSum
}
