package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// FR8 / site d: admin_store may manage merchandise; admin_exam may not.
func TestCheckTypeRBAC_Merchandise(t *testing.T) {
	if err := checkTypeRBAC(RoleAdminStore, "merchandise"); err != nil {
		t.Errorf("admin_store managing merchandise: want nil, got %v", err)
	}
	if err := checkTypeRBAC(RoleSuperAdmin, "merchandise"); err != nil {
		t.Errorf("super_admin managing merchandise: want nil, got %v", err)
	}
	if err := checkTypeRBAC(RoleAdminExam, "merchandise"); !errors.Is(err, ErrForbidden) {
		t.Errorf("admin_exam managing merchandise: want ErrForbidden, got %v", err)
	}
}

func TestMedalIsPhysicalAndStoreManaged(t *testing.T) {
	require.True(t, isPhysicalType("medal"))
	require.NoError(t, checkTypeRBAC(RoleAdminStore, "medal"))
	require.ErrorIs(t, checkTypeRBAC(RoleAdminExam, "medal"), ErrForbidden)
}

// FR4.e / site e: the Midtrans category switch labels merchandise items.
func TestBuildPaymentRequest_MerchandiseCategory(t *testing.T) {
	order := model.Order{
		Total: 100,
		Items: []model.OrderItem{
			{ProductID: uuid.New(), ProductType: "merchandise", Name: "Academy Tee", UnitPrice: 100, Qty: 1},
		},
	}
	req := buildPaymentRequest("order-1", order, CustomerInfo{})
	require.Len(t, req.Items, 1)
	if req.Items[0].Category != "Merchandise" {
		t.Errorf("merchandise item category = %q, want %q", req.Items[0].Category, "Merchandise")
	}
}

func TestBuildPaymentRequest_MedalCategory(t *testing.T) {
	order := model.Order{Total: 100, Items: []model.OrderItem{{ProductID: uuid.New(), ProductType: "medal", Name: "Gold Medal", UnitPrice: 100, Qty: 1}}}
	req := buildPaymentRequest("order-1", order, CustomerInfo{})
	require.Equal(t, "Medal", req.Items[0].Category)
}

// FR6 / site b: a merchandise product with stock 0 is stock-guarded on AddItem.
func TestAddItem_Merchandise_OutOfStock(t *testing.T) {
	ctx := context.Background()
	svc, repo := newRealDBService(t)

	studentID := seedMerchStudent(t, repo)
	productID := seedMerchProduct(t, repo, 0)

	order, _, err := svc.MintCart(ctx, studentID)
	require.NoError(t, err)

	err = svc.AddItem(ctx, studentID, order.ID.String(), productID, 1)
	if !errors.Is(err, ErrOutOfStock) {
		t.Errorf("adding out-of-stock merchandise: want ErrOutOfStock, got %v", err)
	}
}

func TestDeleteProduct_MerchandiseHardDeletes(t *testing.T) {
	ctx := context.Background()
	svc, repo := newRealDBService(t)

	productID := seedMerchProduct(t, repo, 10)
	require.NoError(t, svc.DeleteProduct(ctx, productID, RoleAdminStore))

	_, err := repo.GetProductByID(ctx, productID)
	require.ErrorIs(t, err, repository.ErrNotFound)
}

// Gate (b) / site c: a processing order containing merchandise cannot be completed
// until it has been shipped.
func TestAdminCompleteOrder_ProcessingMerchandise_RejectedUntilShipped(t *testing.T) {
	ctx := context.Background()
	svc, repo := newRealDBService(t)

	studentID := seedMerchStudent(t, repo)
	productID := seedMerchProduct(t, repo, 10)

	order, _, err := svc.MintCart(ctx, studentID)
	require.NoError(t, err)
	require.NoError(t, svc.AddItem(ctx, studentID, order.ID.String(), productID, 1))

	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.SetOrderStatus(ctx, tx, order.ID, "processing", ""))
	require.NoError(t, tx.Commit(ctx))

	require.ErrorIs(t, svc.AdminCompleteOrder(ctx, order.ID.String()), ErrMustShipBeforeComplete)

	require.NoError(t, svc.AdminShipOrderManual(ctx, order.ID.String(), "JNE-123"))
	require.NoError(t, svc.AdminCompleteOrder(ctx, order.ID.String()))

	completed, err := repo.GetOrderByID(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", completed.Status)
}

func seedMerchStudent(t *testing.T, repo *repository.Repository) string {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO users (email, name, role, status) VALUES ($1, $2, 'student', 'active') RETURNING id`,
		"merch-"+uniqueSuffix()+"@test.local", "Merch Buyer",
	).Scan(&id))
	return id.String()
}

func seedMerchProduct(t *testing.T, repo *repository.Repository, stock int) string {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status) VALUES ('merchandise', $1, 100, $2, 'published') RETURNING id`,
		"Academy Tee "+uniqueSuffix(), stock,
	).Scan(&id))
	return id.String()
}
