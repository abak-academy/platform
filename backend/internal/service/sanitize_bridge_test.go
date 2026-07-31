package service

import (
	"os"
	"testing"
)

// TestSanitizeQuestionBodyBridge exposes the unexported sanitizeQuestionBody to the
// vitest suite, which must round-trip editor output through the REAL Go sanitiser —
// a JS reimplementation would prove nothing, since the FB-24 P0 existed precisely
// because the two sides disagreed.
//
// It is committed rather than written and deleted at runtime: two vitest workers
// previously created and removed their own bridge files inside this package, so one
// worker's cleanup could land mid-compile of the other's `go test`, producing an
// intermittent failure. Env-gated so a normal `go test ./...` skips it.
func TestSanitizeQuestionBodyBridge(t *testing.T) {
	in := os.Getenv("SANITIZE_BRIDGE_IN")
	out := os.Getenv("SANITIZE_BRIDGE_OUT")
	if in == "" || out == "" {
		t.Skip("SANITIZE_BRIDGE_IN/OUT not set — invoked only from the web test suite")
	}
	data, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte(sanitizeQuestionBody(string(data))), 0o644); err != nil {
		t.Fatal(err)
	}
}
