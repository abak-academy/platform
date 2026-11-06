package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDeviceProof(t *testing.T) {
	const validID = "3f2b8c1e-6a4d-4e0b-9d7a-1c5e8f2a9b34"
	tests := []struct {
		name   string
		header string
		wantID string
	}{
		{name: "valid uuid", header: validID, wantID: validID},
		{name: "absent", header: "", wantID: ""},
		{name: "not a uuid", header: "my-laptop", wantID: ""},
		{name: "uuid with trailing junk", header: validID + "x", wantID: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("User-Agent", "ua")
			if tc.header != "" {
				req.Header.Set("X-Exam-Device", tc.header)
			}
			c := echo.New().NewContext(req, httptest.NewRecorder())

			got := deviceProof(c)

			if got.ID != tc.wantID {
				t.Errorf("ID: want %q, got %q", tc.wantID, got.ID)
			}
			if want := fingerprint(c.RealIP(), "ua"); got.LegacyFP != want {
				t.Errorf("LegacyFP: want %q, got %q", want, got.LegacyFP)
			}
		})
	}
}
