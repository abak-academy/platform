package handler_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdminUnitSalesRangesAndCSV(t *testing.T) {
	h, svc, repo := newShipHandlerTestService(t, &fakeShipHandlerLogistics{})
	id := createShippableOrderForHandler(t, svc, repo, "paid", true)
	name := "unit-handler-" + uuid.NewString() + ", \"buku\""
	_, err := repo.Pool().Exec(context.Background(), `UPDATE orders SET paid_at='2045-06-10 16:59:00+00', status='cancelled', cancellation_reason='refunded', cancelled_at='2045-07-01 12:00:00+07' WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = repo.Pool().Exec(context.Background(), `UPDATE order_item SET name=$2, qty=7 WHERE order_id=$1`, id, name)
	require.NoError(t, err)
	for _, suffix := range []string{"-a", "-b"} {
		var product string
		require.NoError(t, repo.Pool().QueryRow(context.Background(), `INSERT INTO product(name,type,price,status) VALUES($1,'book',123,'published') RETURNING id`, name+suffix).Scan(&product))
		_, err := repo.Pool().Exec(context.Background(), `INSERT INTO order_item(order_id,product_id,product_type,name,qty,unit_price,jumlah) VALUES($1,$2,'book',$3,7,123,861)`, id, product, name+suffix)
		require.NoError(t, err)
	}
	for _, tc := range []struct {
		query  string
		status int
		units  int
	}{
		{"from=2045-06-10&to=2045-06-10", 200, 7}, {"from=2045-06-11&to=2045-06-11", 200, 0},
		{"from=2045-07-01&to=2045-07-01", 200, 0}, {"from=2045-06-10", 200, 7}, {"to=2045-06-10", 200, 7}, {"", 200, 7},
		{"from=2045-06-11&to=2045-06-10", 400, 0}, {"from=invalid", 400, 0}, {"to=invalid", 400, 0},
	} {
		t.Run(tc.query, func(t *testing.T) {
			c, rec := newAdminOrdersGetCtx("/admin/orders/unit-sales?" + tc.query)
			require.NoError(t, h.AdminUnitSales(c))
			require.Equal(t, tc.status, rec.Code)
			if tc.status != 200 {
				return
			}
			var body model.UnitSalesResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Len(t, body.Types, 5)
			for i, typ := range []string{"book", "merchandise", "medal", "course", "exam"} {
				require.Equal(t, typ, body.Types[i].Type)
			}
			found, refunded, net := 0, 0, 0
			for _, typ := range body.Types {
				for _, p := range typ.Products {
					if p.Name == name {
						found += p.PaidUnits
						refunded += p.RefundUnits
						net += p.NetUnits
					}
				}
			}
			require.Equal(t, tc.units, found)
			wantRefund := 0
			if tc.query == "" || tc.query == "from=2045-06-10" || tc.query == "from=2045-07-01&to=2045-07-01" {
				wantRefund = 7
			}
			require.Equal(t, wantRefund, refunded)
			require.Equal(t, tc.units-wantRefund, net)
			if tc.query == "from=2045-06-11&to=2045-06-11" {
				for _, typ := range body.Types {
					require.Empty(t, typ.Products)
					require.Equal(t, model.UnitSalesFigures{}, typ.UnitSalesFigures)
				}
			}
			if tc.query == "from=2045-06-10&to=2045-06-10" {
				require.Equal(t, 1, body.Types[0].PaidOrders)
				productOrders := 0
				for _, p := range body.Types[0].Products {
					productOrders += p.PaidOrders
				}
				require.Greater(t, productOrders, body.Types[0].PaidOrders)
			}
			for _, typ := range body.Types {
				c, rec := newAdminOrdersGetCtx("/admin/orders/unit-sales/export?type=" + typ.Type + "&" + tc.query)
				require.NoError(t, h.AdminUnitSalesExport(c))
				require.Equal(t, 200, rec.Code)
				require.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
				start, end := c.QueryParam("from"), c.QueryParam("to")
				if start == "" {
					start = "semua-waktu"
				}
				if end == "" {
					end = "semua-waktu"
				}
				require.Equal(t, fmt.Sprintf(`attachment; filename="unit-terjual-%s-%s_%s.csv"`, typ.Type, start, end), rec.Header().Get("Content-Disposition"))
				require.True(t, strings.HasPrefix(rec.Body.String(), "\xef\xbb\xbf"))
				rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\xef\xbb\xbf"))).ReadAll()
				require.NoError(t, err)
				require.Len(t, rows, len(typ.Products)+1)
				require.Equal(t, []string{"Produk", "Order dibayar", "Order refund", "Unit dibayar", "Unit refund", "Unit neto"}, rows[0])
				paid, refund, net := 0, 0, 0
				productPaid, productRefund := 0, 0
				for _, p := range typ.Products {
					productPaid += p.PaidUnits
					productRefund += p.RefundUnits
				}
				require.Equal(t, typ.PaidUnits, productPaid)
				require.Equal(t, typ.RefundUnits, productRefund)
				for i, row := range rows[1:] {
					require.Len(t, row, 6)
					if i > 0 {
						previous, current := typ.Products[i-1], typ.Products[i]
						require.True(t, previous.NetUnits > current.NetUnits || previous.NetUnits == current.NetUnits && (previous.Name < current.Name || previous.Name == current.Name && previous.ProductID < current.ProductID))
					}
					require.Equal(t, typ.Products[i].Name, row[0])
					a, e := strconv.Atoi(row[3])
					require.NoError(t, e)
					paid += a
					b, e := strconv.Atoi(row[4])
					require.NoError(t, e)
					refund += b
					n, e := strconv.Atoi(row[5])
					require.NoError(t, e)
					net += n
				}
				require.Equal(t, typ.PaidUnits, paid)
				require.Equal(t, typ.RefundUnits, refund)
				require.Equal(t, typ.NetUnits, net)
			}
		})
	}
	c, rec := newAdminOrdersGetCtx("/admin/orders/unit-sales/export?type=unknown")
	require.NoError(t, h.AdminUnitSalesExport(c))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
