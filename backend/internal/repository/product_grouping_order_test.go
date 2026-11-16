package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
)

// Admin's naik/turun reorder (ReplaceGroupingProducts) writes grouping_order;
// ListProducts is what the student catalog reads through, and must sort by
// that column too, or the reorder has no visible effect for students.
func TestListProducts_GroupingOrder_SortsByGroupingOrderNotID(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	tag := uuid.NewString()[:8]
	grouping := &model.ProductGrouping{Name: tag + "-grouping", Status: model.ProductGroupingStatusPublished}
	if err := r.CreateProductGrouping(ctx, grouping); err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	groupingID, err := uuid.Parse(grouping.ID)
	if err != nil {
		t.Fatalf("parse grouping id: %v", err)
	}

	mk := func(name string) uuid.UUID {
		t.Helper()
		p := model.Product{Type: "book", Name: tag + "-" + name, Price: 1000, Stock: 1, Status: "published"}
		if err := r.CreateProduct(ctx, &p); err != nil {
			t.Fatalf("create product %s: %v", name, err)
		}
		id, err := uuid.Parse(p.ID)
		if err != nil {
			t.Fatalf("parse product id: %v", err)
		}
		return id
	}

	// Created in id order a, b, c; assigned grouping_order so the intended
	// display order (c, a, b) disagrees with id order — a result that happens
	// to be right by id-order coincidence would not catch the bug.
	a := mk("a")
	b := mk("b")
	c := mk("c")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := r.ReplaceGroupingProducts(ctx, tx, groupingID, []uuid.UUID{c, a, b}); err != nil {
		t.Fatalf("replace grouping products: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	got, _, err := r.ListProducts(ctx, ProductFilter{GroupingID: grouping.ID, VisibleOnly: true, Limit: 100})
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 grouping members, got %d", len(got))
	}
	wantOrder := []string{tag + "-c", tag + "-a", tag + "-b"}
	for i, p := range got {
		if p.Name != wantOrder[i] {
			t.Errorf("position %d = %q, want %q (student catalog must follow grouping_order, not id)", i, p.Name, wantOrder[i])
		}
	}
}

// The compound keyset comparison added alongside the ORDER BY change must
// keep pagination exact: no row skipped or repeated across a page boundary.
func TestListProducts_GroupingOrder_PaginatesWithoutSkippingOrRepeating(t *testing.T) {
	ctx := context.Background()
	pool := newGradingTestPool(t)
	r := New(pool)

	tag := uuid.NewString()[:8]
	grouping := &model.ProductGrouping{Name: tag + "-grouping", Status: model.ProductGroupingStatusPublished}
	if err := r.CreateProductGrouping(ctx, grouping); err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	groupingID, err := uuid.Parse(grouping.ID)
	if err != nil {
		t.Fatalf("parse grouping id: %v", err)
	}

	// Created e, d, c, b, a — the reverse of the intended display order — so a
	// correct result cannot come from id order by coincidence.
	names := []string{"e", "d", "c", "b", "a"}
	ids := make([]uuid.UUID, len(names))
	for i, n := range names {
		p := model.Product{Type: "book", Name: tag + "-" + n, Price: 1000, Stock: 1, Status: "published"}
		if err := r.CreateProduct(ctx, &p); err != nil {
			t.Fatalf("create product %s: %v", n, err)
		}
		id, err := uuid.Parse(p.ID)
		if err != nil {
			t.Fatalf("parse product id: %v", err)
		}
		ids[i] = id
	}
	wantOrder := []uuid.UUID{ids[4], ids[3], ids[2], ids[1], ids[0]} // a, b, c, d, e

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := r.ReplaceGroupingProducts(ctx, tx, groupingID, wantOrder); err != nil {
		t.Fatalf("replace grouping products: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var seen []uuid.UUID
	cursor := ""
	for page := range 10 {
		got, next, err := r.ListProducts(ctx, ProductFilter{GroupingID: grouping.ID, VisibleOnly: true, Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatalf("list products page %d: %v", page, err)
		}
		for _, p := range got {
			id, err := uuid.Parse(p.ID)
			if err != nil {
				t.Fatalf("parse returned product id: %v", err)
			}
			seen = append(seen, id)
		}
		if next == "" {
			break
		}
		cursor = next
	}

	if len(seen) != len(wantOrder) {
		t.Fatalf("paginated through %d rows, want %d (skip or repeat across a page boundary)", len(seen), len(wantOrder))
	}
	for i, id := range seen {
		if id != wantOrder[i] {
			t.Errorf("position %d = %s, want %s", i, id, wantOrder[i])
		}
	}
}
