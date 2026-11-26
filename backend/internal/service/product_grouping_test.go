package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// seedGroupingProduct creates a minimal "book" product for grouping tests.
func seedGroupingProduct(t *testing.T, repo interface {
	CreateProduct(context.Context, *model.Product) error
}, name string) model.Product {
	t.Helper()
	p := model.Product{Type: "book", Name: name, Price: 1000, Status: "published"}
	if err := repo.CreateProduct(context.Background(), &p); err != nil {
		t.Fatalf("seed product %q: %v", name, err)
	}
	return p
}

func TestAdminCreateProductGrouping_RequiresName(t *testing.T) {
	svc, _ := newRealDBService(t)
	_, err := svc.AdminCreateProductGrouping(context.Background(), model.ProductGrouping{Name: ""})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("AdminCreateProductGrouping() err = %v, want ErrValidation", err)
	}
}

func TestAdminCreateProductGrouping_DefaultsToDraft(t *testing.T) {
	svc, _ := newRealDBService(t)
	g, err := svc.AdminCreateProductGrouping(context.Background(), model.ProductGrouping{Name: "Paket UTBK " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("AdminCreateProductGrouping() err = %v", err)
	}
	if g.Status != model.ProductGroupingStatusDraft {
		t.Fatalf("Status = %q, want draft", g.Status)
	}
}

func TestAdminSetGroupingProducts_AssignsOrdersAndUnassignsRemoved(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	g, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Grouping " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	p1 := seedGroupingProduct(t, repo, "Buku A "+uniqueSuffix())
	p2 := seedGroupingProduct(t, repo, "Buku B "+uniqueSuffix())
	p3 := seedGroupingProduct(t, repo, "Buku C "+uniqueSuffix())

	// Assign p2 then p1, in that order.
	_, products, err := svc.AdminSetGroupingProducts(ctx, g.ID, []string{p2.ID, p1.ID})
	if err != nil {
		t.Fatalf("AdminSetGroupingProducts() err = %v", err)
	}
	if len(products) != 2 || products[0].ID != p2.ID || products[1].ID != p1.ID {
		t.Fatalf("members after first assign = %+v, want [p2, p1] in order", products)
	}

	// Replace membership with p1 and p3; p2 must be unassigned.
	_, products, err = svc.AdminSetGroupingProducts(ctx, g.ID, []string{p1.ID, p3.ID})
	if err != nil {
		t.Fatalf("AdminSetGroupingProducts() err = %v", err)
	}
	if len(products) != 2 || products[0].ID != p1.ID || products[1].ID != p3.ID {
		t.Fatalf("members after replace = %+v, want [p1, p3] in order", products)
	}

	unassigned, err := repo.GetProductByID(ctx, p2.ID)
	if err != nil {
		t.Fatalf("get unassigned product: %v", err)
	}
	if unassigned.GroupingID != nil {
		t.Fatalf("p2.GroupingID = %v, want nil after being dropped from the grouping", *unassigned.GroupingID)
	}
}

func TestAdminDeleteProductGrouping_RefusedWhileMembersExist(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	g, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Grouping " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	p := seedGroupingProduct(t, repo, "Buku "+uniqueSuffix())
	if _, _, err := svc.AdminSetGroupingProducts(ctx, g.ID, []string{p.ID}); err != nil {
		t.Fatalf("assign product: %v", err)
	}

	if err := svc.AdminDeleteProductGrouping(ctx, g.ID); !errors.Is(err, ErrGroupingHasProducts) {
		t.Fatalf("AdminDeleteProductGrouping() with members err = %v, want ErrGroupingHasProducts", err)
	}
	after, err := repo.GetProductByID(ctx, p.ID)
	if err != nil || after.GroupingID == nil {
		t.Fatalf("member must stay grouped after the refused delete: %+v err=%v", after, err)
	}

	if _, _, err := svc.AdminSetGroupingProducts(ctx, g.ID, nil); err != nil {
		t.Fatalf("unassign product: %v", err)
	}
	if err := svc.AdminDeleteProductGrouping(ctx, g.ID); err != nil {
		t.Fatalf("AdminDeleteProductGrouping() when empty err = %v", err)
	}
	if _, _, err := svc.AdminGetProductGrouping(ctx, g.ID); !errors.Is(err, ErrProductGroupingNotFound) {
		t.Fatalf("AdminGetProductGrouping() after delete err = %v, want ErrProductGroupingNotFound", err)
	}
}

// Student catalog browsing filters by grouping_id via ListProducts rather
// than a dedicated grouping-detail read; a draft grouping's id must not let
// that filter surface its (already individually published) members.
func TestListProducts_GroupingIDFilter_RespectsGroupingPublishStatus(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	g, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Grouping " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	p := seedGroupingProduct(t, repo, "Buku "+uniqueSuffix())
	if _, _, err := svc.AdminSetGroupingProducts(ctx, g.ID, []string{p.ID}); err != nil {
		t.Fatalf("assign product: %v", err)
	}

	products, _, err := svc.ListProducts(ctx, repository.ProductFilter{GroupingID: g.ID}, RoleStudent)
	if err != nil {
		t.Fatalf("ListProducts() on draft grouping err = %v", err)
	}
	if len(products) != 0 {
		t.Fatalf("draft grouping: products = %+v, want none", products)
	}

	published := model.ProductGroupingStatusPublished
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Status: &published}); err != nil {
		t.Fatalf("publish grouping: %v", err)
	}
	products, _, err = svc.ListProducts(ctx, repository.ProductFilter{GroupingID: g.ID}, RoleStudent)
	if err != nil {
		t.Fatalf("ListProducts() on published grouping err = %v", err)
	}
	if len(products) != 1 || products[0].ID != p.ID {
		t.Fatalf("published grouping: products = %+v, want only %q", products, p.ID)
	}
}

func TestAdminUpdateProductGrouping_RejectsInvalidStatus(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	g, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Grouping " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	bogus := "archived"
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Status: &bogus}); !errors.Is(err, ErrValidation) {
		t.Fatalf("AdminUpdateProductGrouping() err = %v, want ErrValidation", err)
	}
}

func TestAdminReorderProductGroupings_SetsDisplayOrder(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	g1, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Grouping " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("create grouping 1: %v", err)
	}
	g2, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Grouping " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("create grouping 2: %v", err)
	}

	if err := svc.AdminReorderProductGroupings(ctx, []string{g2.ID, g1.ID}); err != nil {
		t.Fatalf("AdminReorderProductGroupings() err = %v", err)
	}

	all, err := svc.AdminListProductGroupings(ctx)
	if err != nil {
		t.Fatalf("AdminListProductGroupings() err = %v", err)
	}
	idx := map[string]int{}
	for i, g := range all {
		idx[g.ID] = i
	}
	if idx[g2.ID] >= idx[g1.ID] {
		t.Fatalf("list order = %+v, want g2 before g1", all)
	}
}

// A membership write committed while DELETE waits on the grouping lock must
// make the DELETE refuse, not ungroup the new member via ON DELETE SET NULL.
func TestAdminDeleteProductGrouping_WaitsForConcurrentMembershipWrite(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	g, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Grouping " + uniqueSuffix()})
	if err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	p := seedGroupingProduct(t, repo, "Buku "+uniqueSuffix())

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := repo.GetProductGroupingForUpdateTx(ctx, tx, g.ID); err != nil {
		t.Fatalf("lock grouping: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE product SET grouping_id = $1 WHERE id = $2`, g.ID, p.ID); err != nil {
		t.Fatalf("assign in tx: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- svc.AdminDeleteProductGrouping(ctx, g.ID) }()
	select {
	case err := <-done:
		t.Fatalf("delete returned %v before the membership tx committed; it must wait on the grouping lock", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := <-done; !errors.Is(err, ErrGroupingHasProducts) {
		t.Fatalf("AdminDeleteProductGrouping() err = %v, want ErrGroupingHasProducts", err)
	}
	after, err := repo.GetProductByID(ctx, p.ID)
	if err != nil || after.GroupingID == nil || *after.GroupingID != g.ID {
		t.Fatalf("member must stay grouped: %+v err=%v", after, err)
	}
}
