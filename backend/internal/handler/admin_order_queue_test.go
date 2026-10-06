package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestApplyOrderQueue_InTransit(t *testing.T) {
	filter := repository.OrderFilter{Status: "paid"}
	require.NoError(t, applyOrderQueue(&filter, "in_transit"))
	require.True(t, filter.InTransit)
	require.Equal(t, service.ShipmentFailureStatusValues(), filter.ShipmentStatusNotIn)
}

func TestAdminOrderQueues_UnknownReturns400(t *testing.T) {
	h := &Handler{}
	for _, endpoint := range []string{"list", "summary"} {
		t.Run(endpoint, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/admin/orders?queue=unknown", nil), rec)
			var err error
			if endpoint == "list" {
				err = h.AdminListOrders(c)
			} else {
				err = h.AdminOrdersSummary(c)
			}
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestOrderSummaryResponse_StatusBucketJSON(t *testing.T) {
	body, err := json.Marshal(OrderSummaryResponse{})
	require.NoError(t, err)
	var response struct {
		Buckets map[string]int `json:"buckets"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	for _, key := range []string{"needs_confirm", "ready_to_ship", "shipment_failed", "in_transit", "completed", "payment_expired", "cancelled", "total"} {
		require.Contains(t, response.Buckets, key)
	}
}
