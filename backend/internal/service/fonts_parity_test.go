package service

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// webFontDir is the browser-side copy loaded by the exam package layout.
const webFontDir = "../../../web/public/fonts"

// TestFontParity_BackendMatchesWeb guards a duplication that cannot be removed
// without moving certificate rendering itself (see
// docs/backlog/certificate-rendering-consolidation.md): the same font files must
// exist twice, once embedded in the binary for Gotenberg and once served to the
// browser for the WYSIWYG design editor.
//
// If they drift, the editor previews a typeface the PDF cannot render — or the
// reverse — and nothing else in the suite notices, because no test compares a
// browser preview against a rendered PDF.
func TestFontParity_BackendMatchesWeb(t *testing.T) {
	if _, err := os.Stat(webFontDir); os.IsNotExist(err) {
		t.Skipf("web font dir not present at %s (backend checked out alone?)", webFontDir)
	}

	embedded := hashTreeFS(t)
	served := hashTreeDisk(t, webFontDir)

	// Without this both maps could go empty — from a moved directory or a
	// changed extension — and the comparison below would pass vacuously.
	if len(embedded) < 8 {
		t.Fatalf("expected at least 8 embedded font files, found %d — parity check would be vacuous", len(embedded))
	}

	for name, want := range embedded {
		got, ok := served[name]
		if !ok {
			t.Errorf("%s is embedded in the backend but missing from %s", name, webFontDir)
			continue
		}
		if got != want {
			t.Errorf("%s differs between backend and web (backend=%s web=%s)", name, want[:12], got[:12])
		}
	}
	for name := range served {
		if _, ok := embedded[name]; !ok {
			t.Errorf("%s is served to the browser but not embedded in the backend", name)
		}
	}
}

// hashTreeFS digests every font in the embedded FS, keyed by path relative to
// the fonts root so both sides use the same key space.
func hashTreeFS(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := fs.WalkDir(fontFS, "fonts", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isFontFile(path) {
			return err
		}
		b, err := fontFS.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[strings.TrimPrefix(path, "fonts/")] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded fonts: %v", err)
	}
	return out
}

func hashTreeDisk(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isFontFile(path) {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk web fonts: %v", err)
	}
	return out
}

// isFontFile deliberately ignores OFL.txt and any README the families ship with:
// licence text is expected to sit alongside the fonts and is not what drifts.
func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".woff", ".woff2":
		return true
	}
	return false
}
