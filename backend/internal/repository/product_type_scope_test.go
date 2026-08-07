package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// ProductFilter.Types is the read-side role boundary (see
// productListFilterForRole). The service tests drive it through a fake repo, so
// this pins the behaviour against real SQL — a fake that agrees with itself
// proves nothing about the query that actually runs.
func TestListProducts_TypesAllowlist(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	// The pool is shared across the package with no truncation, so names are
	// unique per run and assertions are membership/invariant based, never counts
	// over the whole table.
	tag := uuid.NewString()[:8]
	mk := func(productType, name string) {
		t.Helper()
		p := model.Product{Type: productType, Name: tag + "-" + name, Price: 1000, Stock: 1, Status: "draft"}
		if err := r.CreateProduct(ctx, &p); err != nil {
			t.Fatalf("create %s %s: %v", productType, name, err)
		}
	}
	mk("book", "book-a")
	mk("book", "book-b")
	mk("merchandise", "merch-a")
	mk("course", "course-a")
	mk("course", "course-b")
	mk("exam", "exam-a")

	got, _, err := r.ListProducts(ctx, ProductFilter{Types: []string{"course", "exam"}, Limit: 1000})
	if err != nil {
		t.Fatalf("list with Types allowlist: %v", err)
	}
	mine := map[string]bool{}
	for _, p := range got {
		// Holds across every row in the shared table, not just the seeded ones.
		if p.Type != "course" && p.Type != "exam" {
			t.Errorf("allowlist leaked a %s product (%s)", p.Type, p.Name)
		}
		mine[p.Name] = true
	}
	for _, want := range []string{"course-a", "course-b", "exam-a"} {
		if !mine[tag+"-"+want] {
			t.Errorf("allowlist dropped %s, which it should admit", want)
		}
	}
	for _, notWant := range []string{"book-a", "book-b", "merch-a"} {
		if mine[tag+"-"+notWant] {
			t.Errorf("allowlist admitted %s", notWant)
		}
	}
}

// A non-nil empty allowlist means the role may see no type at all. SQL cannot
// express `IN ()`, so this must short-circuit rather than fall through to an
// unfiltered query.
func TestListProducts_EmptyTypesAllowlistReturnsNothing(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	p := model.Product{Type: "course", Name: uuid.NewString()[:8] + "-c", Price: 1000, Stock: 1, Status: "draft"}
	if err := r.CreateProduct(ctx, &p); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, cursor, err := r.ListProducts(ctx, ProductFilter{Types: []string{}, Limit: 1000})
	if err != nil {
		t.Fatalf("list with empty allowlist: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty allowlist must return no rows, got %d", len(got))
	}
	if cursor != "" {
		t.Errorf("empty allowlist must not hand out a cursor, got %q", cursor)
	}
}

// The boundary has to live in the WHERE clause: applied after LIMIT picked a
// page, a table full of physical rows would crowd the digital ones out of it
// and the caller would see an empty page instead of its own products.
func TestListProducts_TypesAppliedBeforeLimit(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	tag := uuid.NewString()[:8]
	for i := 0; i < 30; i++ {
		p := model.Product{Type: "book", Name: tag + "-bulk", Price: 1000, Stock: 1, Status: "draft"}
		if err := r.CreateProduct(ctx, &p); err != nil {
			t.Fatalf("create bulk book %d: %v", i, err)
		}
	}
	for _, n := range []string{"c1", "c2", "c3"} {
		p := model.Product{Type: "course", Name: tag + "-" + n, Price: 1000, Stock: 1, Status: "draft"}
		if err := r.CreateProduct(ctx, &p); err != nil {
			t.Fatalf("create course %s: %v", n, err)
		}
	}

	got, _, err := r.ListProducts(ctx, ProductFilter{Types: []string{"course", "exam"}, Limit: 3})
	if err != nil {
		t.Fatalf("list with small limit: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want a full page of 3 digital rows, got %d", len(got))
	}
	for _, p := range got {
		if p.Type != "course" && p.Type != "exam" {
			t.Errorf("physical row %s consumed a slot in the limited page", p.Name)
		}
	}
}
