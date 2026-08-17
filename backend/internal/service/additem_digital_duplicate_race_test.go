package service

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// TestAddItem_ConcurrentDuplicateDigitalProduct pins the P2 finding: two
// concurrent Service.AddItem calls for the same digital product on the same
// cart both pass the service's own duplicate pre-check — an unlocked read of
// order.Items — so without a repo-level re-check against the locked item set
// immediately before the INSERT, both would insert and the cart would carry
// two lines for a product meant to be unique. Run across n independent carts
// so at least one trial genuinely overlaps in Postgres; every trial must
// land at exactly one success and one ErrDigitalQtyLimit, and the cart must
// end with exactly one line for the product.
func TestAddItem_ConcurrentDuplicateDigitalProduct(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("needs real parallelism to race goroutines")
	}
	svc, repo := newCheckoutRaceTestService(t)
	ctx := context.Background()

	const n = 16

	type pair struct {
		orderID   uuid.UUID
		studentID string
		productID string
	}
	pairs := make([]pair, n)
	for i := 0; i < n; i++ {
		studentID := insertCheckoutStudent(t, repo, "Digital Dup Race Student", "digitaldup_")
		productID := insertRaceTestProduct(t, repo, "course", 25000, 0)
		order, _, err := svc.MintCart(ctx, studentID)
		if err != nil {
			t.Fatalf("MintCart: %v", err)
		}
		pairs[i] = pair{orderID: order.ID, studentID: studentID, productID: productID}
	}

	var ready sync.WaitGroup
	ready.Add(n * 2)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(n * 2)

	errs := make([][2]error, n)

	for i, p := range pairs {
		i, p := i, p
		for g := 0; g < 2; g++ {
			g := g
			go func() {
				defer done.Done()
				ready.Done()
				<-start
				errs[i][g] = svc.AddItem(ctx, p.studentID, p.orderID.String(), p.productID, 1)
			}()
		}
	}

	ready.Wait()
	close(start)
	done.Wait()

	duplicateHits := 0
	for i, p := range pairs {
		successCount, limitCount := 0, 0
		for _, e := range errs[i] {
			switch {
			case e == nil:
				successCount++
			case errors.Is(e, ErrDigitalQtyLimit):
				limitCount++
			default:
				t.Errorf("trial %d: unexpected error %v", i, e)
			}
		}
		if successCount != 1 || limitCount != 1 {
			t.Errorf("trial %d: want exactly one success and one ErrDigitalQtyLimit, got success=%d limit=%d (errs=%v)",
				i, successCount, limitCount, errs[i])
		}

		got, err := repo.GetOrderByID(ctx, p.orderID)
		if err != nil {
			t.Fatalf("trial %d: GetOrderByID: %v", i, err)
		}
		count := 0
		for _, item := range got.Items {
			if item.ProductID.String() == p.productID {
				count++
			}
		}
		if count != 1 {
			duplicateHits++
			t.Errorf("trial %d: cart has %d lines for product %s, want exactly 1", i, count, p.productID)
		}
	}
	t.Logf("duplicate-insert hit rate this run: %d/%d trials produced a duplicate line", duplicateHits, n)
}
