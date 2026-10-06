package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"akademi-bimbel/internal/handler"
	"github.com/stretchr/testify/require"
)

func TestAdminOrders_TypeCourierParamsReachListAndSummary(t *testing.T) {
	h, svc, repo := newShipHandlerTestService(t, &fakeShipHandlerLogistics{})
	ctx := context.Background()
	id := createShippableOrderForHandler(t, svc, repo, "paid", true)
	_, err := repo.Pool().Exec(ctx, `UPDATE order_item SET product_type='medal' WHERE order_id=$1`, id)
	require.NoError(t, err)
	for _, tc := range []struct {
		params string
		want   int
	}{
		{"type=medal&courier=jne", 1}, {"type=book&courier=jne", 0}, {"type=medal&courier=jnt", 0},
	} {
		t.Run(tc.params, func(t *testing.T) {
			params := tc.params + "&q=" + id.String()[24:]
			c, rec := newAdminOrdersGetCtx("/admin/orders?status=paid&" + params)
			require.NoError(t, h.AdminListOrders(c))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var list orderIDListResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
			require.Len(t, list.Data, tc.want)
			c, rec = newAdminOrdersGetCtx("/admin/orders/summary?" + params)
			require.NoError(t, h.AdminOrdersSummary(c))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var summary handler.OrderSummaryResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summary))
			require.Equal(t, tc.want, summary.Buckets.Total)
			require.Equal(t, tc.want, summary.Buckets.ReadyToShip)
			require.Contains(t, summary.Couriers, "jne", "couriers remain unfiltered even when counts are zero")
		})
	}
}
