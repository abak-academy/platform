package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// One order, five copies. qty_sold must be 5 while order_count is 1 — if the
// two ever collapse into the same number the aggregation is wrong.
func TestTopProducts_qtyAndOrderCountAreDifferentNumbers(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Bulk Buyer")
	book := seedProduct(t, pool, "Buku Borongan", "book", 20000)
	at := time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, student, "paid", at, 100000, 0, 0, 100000,
		[]seedItem{{book, "Buku Borongan", "book", 20000, 5}})

	// February is used by no other test in this package; the pool is shared.
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.TopProducts(ctx, from, to, "revenue", 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, book, got[0].ProductID)
	require.Equal(t, "Buku Borongan", got[0].Name)
	require.Equal(t, "book", got[0].ProductType)
	require.Equal(t, 5, got[0].QtySold)
	require.Equal(t, 1, got[0].OrderCount)
	require.Equal(t, 100000.0, got[0].ProductRevenue)
}

// orderBy is interpolated into the SQL, so it has to be an allowlist.
func TestTopProducts_orderByRejectsUnknownKey(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	_, err := repo.TopProducts(context.Background(), from, to, "total; DROP TABLE orders", 10)
	require.ErrorIs(t, err, ErrInvalidOrderBy)
}

func TestTopProducts_ordersByQtyWhenAsked(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Ranking Buyer")
	cheap := seedProduct(t, pool, "Pin Murah", "merchandise", 1000)
	pricey := seedProduct(t, pool, "Medali Mahal", "medal", 500000)
	at := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

	// Many cheap units versus one expensive one: the two orderings disagree,
	// which is the whole point of having both.
	seedOrder(t, pool, student, "paid", at, 100000, 0, 0, 100000,
		[]seedItem{{cheap, "Pin Murah", "merchandise", 1000, 100}})
	seedOrder(t, pool, student, "paid", at, 500000, 0, 0, 500000,
		[]seedItem{{pricey, "Medali Mahal", "medal", 500000, 1}})

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	byRevenue, err := repo.TopProducts(ctx, from, to, "revenue", 10)
	require.NoError(t, err)
	require.Equal(t, pricey, byRevenue[0].ProductID)

	byQty, err := repo.TopProducts(ctx, from, to, "qty", 10)
	require.NoError(t, err)
	require.Equal(t, cheap, byQty[0].ProductID)
}

// The store dashboard used to count with .length of a page the API caps at 20.
func TestCountOrdersByBucket_countsBeyondOnePage(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Bucket Buyer")
	book := seedProduct(t, pool, "Buku Bucket", "book", 10000)
	at := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)

	for i := 0; i < 25; i++ {
		seedOrder(t, pool, student, "payment_pending", at, 10000, 0, 0, 10000,
			[]seedItem{{book, "Buku Bucket", "book", 10000, 1}})
	}
	seedOrder(t, pool, student, "completed", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Bucket", "book", 10000, 1}})

	monthStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{StudentID: &student, ExcludeCart: true},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)

	require.Equal(t, 25, got.NeedsConfirm, "counts are not capped by page size")
	require.Equal(t, 26, got.Total)
	require.Equal(t, 26, got.CreatedThisMonth)
	require.Equal(t, 1, got.CompletedThisMonth)
	require.Equal(t, 0, got.ShipmentFailed)
}

// The counts have to describe the same rows the table is showing, so they run
// through the same predicate builder as ListOrders.
func TestCountOrdersByBucket_honoursTheSearchFilter(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	wanted := seedStudent(t, pool, "Terpilih Sekali")
	other := seedStudent(t, pool, "Bukan Dia")
	book := seedProduct(t, pool, "Buku Filter", "book", 10000)
	at := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, wanted, "payment_pending", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Filter", "book", 10000, 1}})
	seedOrder(t, pool, other, "payment_pending", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Filter", "book", 10000, 1}})

	monthStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{ExcludeCart: true, Search: "terpilih sekali"},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)
	require.Equal(t, 1, got.NeedsConfirm, "search narrows the counts too")
}

// "Siap kirim" must mean a parcel exists. The orders page gates its Ship action
// on a physical item, so counting digital-only orders here promised work the
// row could not offer.
func TestCountOrdersByBucket_readyToShipRequiresAPhysicalItem(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Mixed Cart Buyer")
	book := seedProduct(t, pool, "Buku Fisik", "book", 10000)
	course := seedProduct(t, pool, "Kursus Digital", "course", 10000)
	at := time.Date(2025, 8, 12, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Fisik", "book", 10000, 1}})
	seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{course, "Kursus Digital", "course", 10000, 1}})

	monthStart := time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)

	got, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{StudentID: &student, ExcludeCart: true},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)

	require.Equal(t, 1, got.ReadyToShip, "only the order with a physical item")
	require.Equal(t, 2, got.Total, "both orders still exist")
}

// The "ready to ship" queue card links to ?queue=ready_to_ship, which must
// return exactly the rows CountOrdersByBucket's ready_to_ship bucket counts —
// not the bigger status=paid set the card used to link to. Scoped by
// StudentID, so this test does not need a reserved date window.
func TestListOrders_readyToShipMatchesTheBucketDefinition(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Ready Queue Buyer")
	book := seedProduct(t, pool, "Buku Siap Kirim", "book", 10000)
	course := seedProduct(t, pool, "Kursus Digital Saja", "course", 10000)
	at := time.Date(2025, 8, 20, 8, 0, 0, 0, time.UTC)

	paidPhysical := seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Siap Kirim", "book", 10000, 1}})
	processingPhysical := seedOrder(t, pool, student, "processing", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Siap Kirim", "book", 10000, 1}})
	// Neither belongs in the queue: digital-only has nothing to ship, and
	// shipped physical has already left it.
	seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{course, "Kursus Digital Saja", "course", 10000, 1}})
	seedOrder(t, pool, student, "shipped", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Siap Kirim", "book", 10000, 1}})

	orders, _, err := repo.ListOrders(ctx, OrderFilter{StudentID: &student, ReadyToShip: true, Limit: 10})
	require.NoError(t, err)

	gotIDs := make([]uuid.UUID, len(orders))
	for i, o := range orders {
		gotIDs[i] = o.ID
	}
	require.ElementsMatch(t, []uuid.UUID{paidPhysical, processingPhysical}, gotIDs)

	monthStart := time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	counts, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{StudentID: &student, ExcludeCart: true},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)
	require.Equal(t, len(gotIDs), counts.ReadyToShip, "the link's rows must match the card's count")
}

// The orders page's "All" tab and the store dashboard both use "all" as a
// legitimate AdminOrderFilterStatus value. If it reaches the predicate builder
// as a literal `status = 'all'`, it matches zero rows instead of every row.
func TestCountOrdersByBucket_StatusAllMeansNoFilter(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Status All Buyer")
	book := seedProduct(t, pool, "Buku Status All", "book", 10000)
	at := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Status All", "book", 10000, 1}})
	seedOrder(t, pool, student, "payment_pending", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Status All", "book", 10000, 1}})

	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	noFilter, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{StudentID: &student, ExcludeCart: true},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)

	allFilter, err := repo.CountOrdersByBucket(ctx,
		OrderFilter{StudentID: &student, ExcludeCart: true, Status: "all"},
		[]string{"courierNotFound"}, monthStart, monthEnd)
	require.NoError(t, err)

	require.Equal(t, noFilter, allFilter, "status=all must count the same rows as no status filter")
	require.Equal(t, 2, allFilter.Total, "both orders must be counted, not zero")
}

// Same sentinel, the list side. AdminListOrders passes ?status= straight
// through to the same OrderFilter, so ListOrders must treat "all" identically.
func TestListOrders_StatusAllMeansNoFilter(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	student := seedStudent(t, pool, "Status All List Buyer")
	book := seedProduct(t, pool, "Buku Status All List", "book", 10000)
	at := time.Date(2026, 6, 11, 8, 0, 0, 0, time.UTC)

	seedOrder(t, pool, student, "paid", at, 10000, 0, 0, 10000,
		[]seedItem{{book, "Buku Status All List", "book", 10000, 1}})

	noFilter, _, err := repo.ListOrders(ctx, OrderFilter{StudentID: &student, Limit: 10})
	require.NoError(t, err)
	allFilter, _, err := repo.ListOrders(ctx, OrderFilter{StudentID: &student, Status: "all", Limit: 10})
	require.NoError(t, err)

	require.Len(t, noFilter, 1)
	require.Len(t, allFilter, 1, "status=all must return the row, not filter it out")
}

func TestCountOrdersByBucket_AllStatusOptionsMatchPagedRows(t *testing.T) {
	pool := newReportingTestPool(t)
	repo := New(pool)
	ctx := context.Background()
	student := seedStudent(t, pool, "Status Filter Buyer")
	book := seedProduct(t, pool, "Status Filter Book", "book", 10000)
	course := seedProduct(t, pool, "Status Filter Course", "course", 10000)
	at := time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)
	failures := []string{"courier_not_found", "courierNotFound"}
	expected := map[string][]uuid.UUID{}
	types := map[uuid.UUID]string{}
	couriers := map[uuid.UUID]string{}
	add := func(status, productType string, shipmentStatus *string, buckets ...string) {
		product := book
		if productType == "course" {
			product = course
		}
		id := seedOrder(t, pool, student, status, at, 10000, 0, 0, 10000,
			[]seedItem{{product, "Status Filter Item", productType, 10000, 1}})
		types[id] = productType
		courier := "jne"
		if len(types)%2 == 0 {
			courier = "jnt"
		}
		couriers[id] = courier
		_, err := pool.Exec(ctx, `UPDATE orders SET courier_code=$1 WHERE id=$2`, courier, id)
		require.NoError(t, err)
		if shipmentStatus != nil {
			require.NoError(t, repo.SetShipmentStatus(ctx, id, *shipmentStatus))
		}
		if status != "cart" {
			expected["all"] = append(expected["all"], id)
		}
		for _, bucket := range buckets {
			expected[bucket] = append(expected[bucket], id)
		}
	}
	for i := 0; i < 3; i++ {
		add("payment_pending", "book", nil, "pending")
		add("paid", "book", nil, "ready_to_ship", "paid")
		add("processing", "book", nil, "ready_to_ship", "processing")
		add("processing", "course", nil, "processing")
		add("completed", "course", nil, "completed")
		add("payment_expired", "book", nil, "payment_expired")
		add("cancelled", "book", nil, "cancelled")
	}
	add("cart", "book", nil)
	add("paid", "course", nil, "paid")
	add("shipped", "book", nil, "in_transit", "shipped")
	for _, status := range []string{"", "in_transit", "delivered", "on_hold"} {
		add("shipped", "book", &status, "in_transit", "shipped")
	}
	for _, status := range failures {
		add("shipped", "book", &status, "shipment_failed", "shipped")
	}
	for _, scope := range []OrderFilter{{}, {ProductType: "book"}, {CourierCode: "jne"}, {ProductType: "book", CourierCode: "jne"}} {
		scopedExpected := map[string][]uuid.UUID{}
		for name, ids := range expected {
			for _, id := range ids {
				if (scope.ProductType == "" || types[id] == scope.ProductType) && (scope.CourierCode == "" || couriers[id] == scope.CourierCode) {
					scopedExpected[name] = append(scopedExpected[name], id)
				}
			}
		}
		scope.StudentID, scope.ExcludeCart = &student, true
		counts, err := repo.CountOrdersByBucket(ctx, scope, failures, at, at.AddDate(0, 1, 0))
		require.NoError(t, err)
		cases := []struct {
			name   string
			filter OrderFilter
			count  int
		}{
			{"all", OrderFilter{}, counts.Total},
			{"pending", OrderFilter{Status: "payment_pending"}, counts.NeedsConfirm},
			{"ready_to_ship", OrderFilter{ReadyToShip: true}, counts.ReadyToShip},
			{"shipment_failed", OrderFilter{ShipmentStatusIn: failures}, counts.ShipmentFailed},
			{"in_transit", OrderFilter{InTransit: true, ShipmentStatusNotIn: failures}, counts.InTransit},
			{"shipped", OrderFilter{Status: "shipped"}, counts.Shipped},
			{"completed", OrderFilter{Status: "completed"}, counts.Completed},
			{"payment_expired", OrderFilter{Status: "payment_expired"}, counts.PaymentExpired},
			{"cancelled", OrderFilter{Status: "cancelled"}, counts.Cancelled},
			{"paid", OrderFilter{Status: "paid"}, len(scopedExpected["paid"])},
			{"processing", OrderFilter{Status: "processing"}, len(scopedExpected["processing"])},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				filter := tc.filter
				filter.ProductType, filter.CourierCode = scope.ProductType, scope.CourierCode
				filter.StudentID, filter.ExcludeCart, filter.Limit = &student, true, 2
				var ids []uuid.UUID
				for page := 0; ; page++ {
					require.Less(t, page, 30, "paging must terminate")
					rows, cursor, err := repo.ListOrders(ctx, filter)
					require.NoError(t, err)
					for _, row := range rows {
						ids = append(ids, row.ID)
					}
					if cursor == "" {
						break
					}
					filter.Cursor = cursor
				}
				require.ElementsMatch(t, scopedExpected[tc.name], ids)
				require.Equal(t, tc.count, len(ids))
			})
		}

	}
}
