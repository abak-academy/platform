package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestUnitSalesRange(t *testing.T) {
	for _, tc := range []struct {
		query    string
		valid    bool
		from, to string
	}{
		{"", true, "", ""}, {"from=2026-10-17", true, "2026-10-17T00:00:00+07:00", ""},
		{"to=2026-10-17", true, "", "2026-10-18T00:00:00+07:00"},
		{"from=2026-10-17&to=2026-10-17", true, "2026-10-17T00:00:00+07:00", "2026-10-18T00:00:00+07:00"},
		{"from=2026-10-18&to=2026-10-17", false, "", ""}, {"from=no", false, "", ""}, {"to=2026-02-30", false, "", ""},
	} {
		t.Run(tc.query, func(t *testing.T) {
			c := echo.New().NewContext(httptest.NewRequest("GET", "/?"+tc.query, nil), httptest.NewRecorder())
			from, to, err := unitSalesRange(c)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.from == "" {
				require.Nil(t, from)
			} else {
				require.Equal(t, tc.from, from.Format("2006-01-02T15:04:05Z07:00"))
			}
			if tc.to == "" {
				require.Nil(t, to)
			} else {
				require.Equal(t, tc.to, to.Format("2006-01-02T15:04:05Z07:00"))
			}
		})
	}
}
