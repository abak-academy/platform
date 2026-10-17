package integration_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUnitSalesRouteRolesAndWhitelist(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	student := seedUser(t, env, "student", "active", false)
	name := "unit-route-" + uuid.NewString()
	var order string
	require.NoError(t, env.pool.QueryRow(ctx, `INSERT INTO orders(student_id,status,subtotal,total,paid_at) VALUES($1,'paid',123,123,'2098-01-01 12:00:00+07') RETURNING id`, student).Scan(&order))
	for _, typ := range []string{"book", "course"} {
		var product string
		require.NoError(t, env.pool.QueryRow(ctx, `INSERT INTO product(name,type,price,status) VALUES($1,$2,123,'published') RETURNING id`, name+typ, typ).Scan(&product))
		_, err := env.pool.Exec(ctx, `INSERT INTO order_item(order_id,product_id,product_type,name,qty,unit_price,jumlah) VALUES($1,$2,$3,$4,2,123,246)`, order, product, typ, name+typ)
		require.NoError(t, err)
	}
	admin := seedUser(t, env, "admin_store", "active", false)
	resp := env.doJSON(t, http.MethodGet, "/api/v1/admin/orders/unit-sales/export?type=unknown", nil, authToken(t, env, admin, "admin_store"))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	drainClose(resp)

	for _, role := range []string{"admin_store", "super_admin", "admin_exam", "admin_school", "student", ""} {
		t.Run(role, func(t *testing.T) {
			token := ""
			if role != "" {
				id := seedUser(t, env, role, "active", false)
				token = authToken(t, env, id, role)
			}
			for _, path := range []string{"/unit-sales?from=2098-01-01&to=2098-01-01", "/unit-sales/export?type=book&from=2098-01-01&to=2098-01-01"} {
				resp := env.doJSON(t, http.MethodGet, "/api/v1/admin/orders"+path, nil, token)
				want := http.StatusForbidden
				if role == "" {
					want = http.StatusUnauthorized
				}
				if role == "admin_store" || role == "super_admin" {
					want = http.StatusOK
				}
				require.Equal(t, want, resp.StatusCode)
				if want == 200 && path[11] == '?' {
					var body map[string]json.RawMessage
					require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
					require.ElementsMatch(t, []string{"from", "to", "types"}, mapKeys(body))
					var types []map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(body["types"], &types))
					require.Len(t, types, 5)
					for _, typ := range types {
						require.ElementsMatch(t, []string{"type", "paid_orders", "refund_orders", "paid_units", "refund_units", "net_units", "products"}, mapKeys(typ))
						var products []map[string]json.RawMessage
						require.NoError(t, json.Unmarshal(typ["products"], &products))
						for _, p := range products {
							require.ElementsMatch(t, []string{"product_id", "name", "paid_orders", "refund_orders", "paid_units", "refund_units", "net_units"}, mapKeys(p))
						}
					}
				}
				if want == 200 && strings.Contains(path, "/export") {
					rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(readUnitSalesBody(t, resp), "\xef\xbb\xbf"))).ReadAll()
					require.NoError(t, err)
					require.Equal(t, []string{"Produk", "Order dibayar", "Order refund", "Unit dibayar", "Unit refund", "Unit neto"}, rows[0])
					found := false
					for _, row := range rows[1:] {
						require.NotEqual(t, name+"course", row[0])
						if row[0] == name+"book" {
							found = true
						}
					}
					require.True(t, found)
				}
				drainClose(resp)
			}
		})
	}
}

func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func readUnitSalesBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(data)
}
