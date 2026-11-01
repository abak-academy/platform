package repository

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestListOrders_TypeCourierCombinedFilters(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	student := seedStudent(t, pool, "Courier Filter Buyer")
	medal := seedProduct(t, pool, "Filter Medal", "medal", 10000)
	book := seedProduct(t, pool, "Filter Book", "book", 10000)
	at := time.Date(2028, 1, 10, 0, 0, 0, 0, time.UTC)
	var want []uuid.UUID
	for _, row := range []struct {
		status, courier, ptype string
		product                uuid.UUID
	}{
		{"paid", "jne", "medal", medal}, {"paid", "jne", "book", book},
		{"payment_pending", "jne", "medal", medal}, {"paid", "jnt", "medal", medal},
	} {
		id := seedOrder(t, pool, student, row.status, at, 10000, 0, 0, 10000,
			[]seedItem{{row.product, "Filter Item", row.ptype, 10000, 1}})
		_, err := pool.Exec(ctx, `UPDATE orders SET courier_code=$1 WHERE id=$2`, row.courier, id)
		require.NoError(t, err)
		want = append(want, id)
	}
	for _, tc := range []struct {
		name   string
		filter OrderFilter
		ids    []uuid.UUID
	}{
		{"type", OrderFilter{ProductType: "medal"}, []uuid.UUID{want[0], want[2], want[3]}},
		{"courier", OrderFilter{CourierCode: "jne"}, want[:3]},
		{"combined", OrderFilter{ProductType: "medal", CourierCode: "jne", Status: "paid", Search: "Courier Filter Buyer", CreatedFrom: &at, CreatedTo: ptrTime(at.AddDate(0, 0, 1))}, want[:1]},
		{"queue", OrderFilter{ProductType: "medal", CourierCode: "jne", ReadyToShip: true, Search: "Courier Filter Buyer"}, want[:1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := tc.filter
			filter.StudentID, filter.ExcludeCart, filter.Limit = &student, true, 100
			rows, _, err := repo.ListOrders(ctx, filter)
			require.NoError(t, err)
			ids := []uuid.UUID{}
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			require.ElementsMatch(t, tc.ids, ids)
		})
	}
}

func TestListOrderCouriers_DistinctSortedNonCart(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	student := seedStudent(t, pool, "Courier List Buyer")
	at := time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)
	before, err := repo.ListOrderCouriers(ctx)
	require.NoError(t, err)
	want := append([]string{}, before...)
	for _, row := range []struct {
		status string
		code   *string
	}{
		{"paid", ptrString("zz_test_jne")}, {"paid", ptrString("zz_test_jne")},
		{"paid", ptrString("zz_test_aaa")}, {"cart", ptrString("zz_cart_only")},
		{"paid", nil}, {"paid", ptrString("")},
	} {
		id := seedOrder(t, pool, student, row.status, at, 0, 0, 0, 0, nil)
		_, err := pool.Exec(ctx, `UPDATE orders SET courier_code=$1 WHERE id=$2`, row.code, id)
		require.NoError(t, err)
	}
	want = append(want, "zz_test_aaa", "zz_test_jne")
	sort.Strings(want)
	got, err := repo.ListOrderCouriers(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func ptrString(value string) *string { return &value }
