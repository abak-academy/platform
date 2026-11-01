package service

import (
	"encoding/csv"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/stretchr/testify/require"
)

func TestUnitSalesCSV(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"=SUM(A1)", "'=SUM(A1)"}, {"+1", "'+1"}, {"-1", "'-1"}, {"@x", "'@x"},
		{"\tx", "'\tx"}, {"\rx", "'\rx"}, {"Buku, \"Satu\"\nDua", "Buku, \"Satu\"\nDua"}, {"", ""}, {"Buku", "Buku"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := UnitSalesCSV([]model.UnitSalesProduct{{Name: tc.name, UnitSalesFigures: model.UnitSalesFigures{PaidOrders: 1, RefundOrders: 2, PaidUnits: 3, RefundUnits: 4, NetUnits: -1}}})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(string(data), "\xef\xbb\xbf"))
			rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))).ReadAll()
			require.NoError(t, err)
			require.Equal(t, [][]string{{"Produk", "Order dibayar", "Order refund", "Unit dibayar", "Unit refund", "Unit neto"}, {tc.want, "1", "2", "3", "4", "-1"}}, rows)
		})
	}
}
