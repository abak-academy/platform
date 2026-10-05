package handler

import (
	"akademi-bimbel/internal/repository"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateOrderProductType(t *testing.T) {
	for _, value := range []string{"", "book", "merchandise", "medal", "course", "exam"} {
		require.NoError(t, validateOrderProductType(value))
	}
	for _, value := range []string{"unknown", "Book", " book"} {
		require.Error(t, validateOrderProductType(value), "type %q", value)
	}
}

func TestAdminOrders_InvalidTypeReturns400(t *testing.T) {
	h := &Handler{}
	for _, endpoint := range []struct {
		name string
		run  func(echo.Context) error
	}{
		{"list", h.AdminListOrders}, {"summary", h.AdminOrdersSummary},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/admin/orders?type=unknown", nil), rec)
			require.NoError(t, endpoint.run(c))
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestAdminOrderFilter_Params(t *testing.T) {
	c := echo.New().NewContext(httptest.NewRequest(http.MethodGet,
		"/admin/orders?type=medal&courier=jne&status=paid&q=Buyer&from=2026-01-01&to=2026-01-02", nil), httptest.NewRecorder())
	filter, err := adminOrderFilter(c)
	require.NoError(t, err)
	require.Equal(t, "medal", filter.ProductType)
	require.Equal(t, "jne", filter.CourierCode)
	require.Equal(t, "paid", filter.Status)
	require.Equal(t, "Buyer", filter.Search)
	require.NotNil(t, filter.CreatedFrom)
	require.NotNil(t, filter.CreatedTo)
}

func TestAdminOrders_InvalidSortDirectionReturns400(t *testing.T) {
	h := &Handler{}
	for _, params := range []string{"sort=total", "dir=sideways", "sort=paid_at&dir=ASC"} {
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/admin/orders?"+params, nil), rec)
		require.NoError(t, h.AdminListOrders(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

func TestApplyOrderSort(t *testing.T) {
	for _, sort := range []string{"", "order_at", "paid_at", "item_qty"} {
		for _, dir := range []string{"", "asc", "desc"} {
			filter := repository.OrderFilter{}
			require.NoError(t, applyOrderSort(&filter, sort, dir))
			wantSort, wantDir := sort, dir
			if wantSort == "" {
				wantSort = "order_at"
			}
			if wantDir == "" {
				wantDir = "desc"
			}
			require.Equal(t, repository.OrderSort(wantSort), filter.Sort)
			require.Equal(t, repository.OrderSortDirection(wantDir), filter.Direction)
		}
	}
}
