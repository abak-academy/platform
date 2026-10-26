package service

import (
	"context"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Commission tests share the package real-DB fixture (rows never reset), so
// each test seeds its own users/products/orders with uniqueSuffix names and
// its own flag row; tests never run in parallel.
type commissionSeedItem struct {
	productType string
	jumlah      float64
}

func setAffiliateFlagForCommission(t *testing.T, repo *repository.Repository, value string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO system_config (key, value, is_secret) VALUES ('affiliate_enabled', $1, false)
		 ON CONFLICT (key) DO UPDATE SET value = $1`, value); err != nil {
		t.Fatalf("set affiliate flag %q: %v", value, err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `DELETE FROM system_config WHERE key = 'affiliate_enabled'`)
	})
}

func seedCommissionProduct(t *testing.T, repo *repository.Repository, productType string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status) VALUES ($1, $2, 100000, NULL, 'published') RETURNING id`,
		productType, "Commission "+productType+" "+uniqueSuffix(),
	).Scan(&id))
	return id
}

func seedCommissionOrder(t *testing.T, repo *repository.Repository, studentID string, paidAt time.Time, discount float64, items ...commissionSeedItem) model.Order {
	t.Helper()
	ctx := context.Background()
	subtotal := 0.0
	for _, it := range items {
		subtotal += it.jumlah
	}
	var orderID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO orders (student_id, status, subtotal, discount, shipping_cost, total, paid_at)
		 VALUES ($1, 'paid', $2, $3, 0, $4, $5) RETURNING id`,
		studentID, subtotal, discount, subtotal-discount, paidAt,
	).Scan(&orderID))

	order := model.Order{
		ID:        orderID,
		StudentID: uuid.MustParse(studentID),
		Status:    "paid",
		Subtotal:  subtotal,
		Discount:  discount,
		Total:     subtotal - discount,
		PaidAt:    &paidAt,
	}
	for _, it := range items {
		productID := seedCommissionProduct(t, repo, it.productType)
		item := model.OrderItem{
			OrderID:     orderID,
			ProductID:   productID,
			ProductType: it.productType,
			Name:        "Item " + it.productType,
			UnitPrice:   it.jumlah,
			Qty:         1,
			Jumlah:      it.jumlah,
		}
		require.NoError(t, repo.Pool().QueryRow(ctx,
			`INSERT INTO order_item (order_id, product_id, product_type, name, unit_price, qty, jumlah)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			orderID, productID, it.productType, item.Name, item.UnitPrice, item.Qty, item.Jumlah,
		).Scan(&item.ID))
		order.Items = append(order.Items, item)
	}
	return order
}

func runCreateCommissions(t *testing.T, svc *Service, repo *repository.Repository, order model.Order) error {
	t.Helper()
	ctx := context.Background()
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	err = svc.CreateCommissionsForOrder(ctx, tx, order, order.Items)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	require.NoError(t, tx.Commit(ctx))
	return nil
}

func affiliateCommissionsForOrder(t *testing.T, repo *repository.Repository, orderID uuid.UUID) []model.AffiliateCommission {
	t.Helper()
	rows, err := repo.Pool().Query(context.Background(),
		`SELECT id, affiliator_id, student_id, order_id, order_item_id, product_type, base_amount, percentage,
		        level_snapshot, amount, status, wait_until, paid_at
		 FROM affiliate_commissions WHERE order_id = $1 ORDER BY order_item_id`, orderID)
	require.NoError(t, err)
	defer rows.Close()

	var out []model.AffiliateCommission
	for rows.Next() {
		var c model.AffiliateCommission
		require.NoError(t, rows.Scan(&c.ID, &c.AffiliatorID, &c.StudentID, &c.OrderID, &c.OrderItemID,
			&c.ProductType, &c.BaseAmount, &c.Percentage, &c.LevelSnapshot, &c.Amount, &c.Status, &c.WaitUntil, &c.PaidAt))
		out = append(out, c)
	}
	require.NoError(t, rows.Err())
	return out
}

func commissionsByType(rows []model.AffiliateCommission) map[string]model.AffiliateCommission {
	byType := make(map[string]model.AffiliateCommission, len(rows))
	for _, c := range rows {
		byType[c.ProductType] = c
	}
	return byType
}

func TestAffiliateCommission_CreatesPendingCommissionsForBoundStudent(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-comm-a-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-a-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

	paidAt := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	order := seedCommissionOrder(t, repo, student.ID, paidAt, 0,
		commissionSeedItem{productType: "exam", jumlah: 100000},
		commissionSeedItem{productType: "course", jumlah: 50000},
	)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))

	rows := affiliateCommissionsForOrder(t, repo, order.ID)
	require.Len(t, rows, 2)
	byType := commissionsByType(rows)

	exam := byType["exam"]
	require.Equal(t, owner.affiliatorID, exam.AffiliatorID)
	require.Equal(t, order.StudentID, exam.StudentID)
	require.Equal(t, order.ID, exam.OrderID)
	require.Equal(t, 100000.0, exam.BaseAmount)
	require.Equal(t, 10.0, exam.Percentage)
	require.Equal(t, 10000.0, exam.Amount)
	require.Equal(t, "bronze", exam.LevelSnapshot)
	require.Equal(t, "pending", exam.Status)
	require.NotNil(t, exam.WaitUntil)
	require.WithinDuration(t, paidAt.AddDate(0, 0, 7), *exam.WaitUntil, time.Second)
	require.NotNil(t, exam.PaidAt)
	require.WithinDuration(t, paidAt, *exam.PaidAt, time.Second)

	course := byType["course"]
	require.Equal(t, 50000.0, course.BaseAmount)
	require.Equal(t, 10.0, course.Percentage)
	require.Equal(t, 5000.0, course.Amount)
	require.Equal(t, "bronze", course.LevelSnapshot)
	require.Equal(t, "pending", course.Status)
	require.NotNil(t, course.WaitUntil)
	require.WithinDuration(t, paidAt.AddDate(0, 0, 7), *course.WaitUntil, time.Second)
}

func TestAffiliateCommission_ReplayedCallIsIdempotent(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-comm-b-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-b-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

	order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000},
	)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))
	require.NoError(t, runCreateCommissions(t, svc, repo, order))

	rows := affiliateCommissionsForOrder(t, repo, order.ID)
	require.Len(t, rows, 1)
}

func TestAffiliateCommission_FlagOffCreatesNothing(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "false")
	owner := seedAffiliatorOwner(t, repo, "aff-comm-c-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-c-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

	order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000},
	)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))
	require.Empty(t, affiliateCommissionsForOrder(t, repo, order.ID))
}

func TestAffiliateCommission_UnboundStudentCreatesNothing(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	seedAffiliatorOwner(t, repo, "aff-comm-d-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-d-"+uniqueSuffix()+"@test.local", "")

	order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000},
	)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))
	require.Empty(t, affiliateCommissionsForOrder(t, repo, order.ID))
}

func TestAffiliateCommission_SuspendedAffiliatorCreatesNothingButFrozenStillEarns(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")

	suspendUntil := time.Now().UTC().Add(24 * time.Hour)
	freezeUntil := time.Now().UTC().Add(24 * time.Hour)

	t.Run("suspended affiliator earns nothing", func(t *testing.T) {
		owner := seedAffiliatorOwner(t, repo, "aff-comm-e1-"+uniqueSuffix()+"@test.local", "", "suspended")
		_, err := repo.Pool().Exec(ctx, `UPDATE affiliators SET suspended_until = $1 WHERE id = $2`, suspendUntil, owner.affiliatorID)
		require.NoError(t, err)
		student := seedAffiliateTestStudent(t, repo, "student-comm-e1-"+uniqueSuffix()+"@test.local", "")
		createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

		order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
			commissionSeedItem{productType: "exam", jumlah: 100000},
		)
		require.NoError(t, runCreateCommissions(t, svc, repo, order))
		require.Empty(t, affiliateCommissionsForOrder(t, repo, order.ID))
	})

	// BR-PLG-05: a balance freeze only holds withdrawals; purchases made while it
	// lasts must still earn, because the commission is created once from the event.
	t.Run("frozen affiliator still earns", func(t *testing.T) {
		owner := seedAffiliatorOwner(t, repo, "aff-comm-e2-"+uniqueSuffix()+"@test.local", "", "active")
		_, err := repo.Pool().Exec(ctx, `UPDATE affiliators SET frozen_until = $1 WHERE id = $2`, freezeUntil, owner.affiliatorID)
		require.NoError(t, err)
		student := seedAffiliateTestStudent(t, repo, "student-comm-e2-"+uniqueSuffix()+"@test.local", "")
		createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

		order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
			commissionSeedItem{productType: "exam", jumlah: 100000},
		)
		require.NoError(t, runCreateCommissions(t, svc, repo, order))
		require.Len(t, affiliateCommissionsForOrder(t, repo, order.ID), 1)
	})
}

func TestAffiliateCommissionAmount(t *testing.T) {
	cases := []struct {
		name      string
		base, pct float64
		want      float64
	}{
		{"whole percent", 100000, 10, 10000},
		{"floors the remainder", 33333, 10, 3333},
		{"decimal percent that is inexact in float", 20000, 7.35, 1470},
		{"half percent", 99999, 2.5, 2499},
		{"zero percent", 100000, 0, 0},
		{"fractional base after discount split", 33333.33, 5, 1666},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, affiliateCommissionAmount(tc.base, tc.pct))
		})
	}
}

func TestAffiliateCommission_DigitalWaitsPhysicalWaitsNull(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-comm-f-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-f-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

	paidAt := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	order := seedCommissionOrder(t, repo, student.ID, paidAt, 0,
		commissionSeedItem{productType: "book", jumlah: 30000},
		commissionSeedItem{productType: "exam", jumlah: 100000},
	)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))

	byType := commissionsByType(affiliateCommissionsForOrder(t, repo, order.ID))
	require.Len(t, byType, 2)

	book := byType["book"]
	require.Equal(t, "pending", book.Status)
	require.Equal(t, 3.0, book.Percentage)
	require.Equal(t, 900.0, book.Amount)
	require.Nil(t, book.WaitUntil)

	exam := byType["exam"]
	require.Equal(t, "pending", exam.Status)
	require.NotNil(t, exam.WaitUntil)
	require.WithinDuration(t, paidAt.AddDate(0, 0, 7), *exam.WaitUntil, time.Second)
}

func TestAffiliateCommission_MedalDefaultZeroEarnsNothing(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-comm-g-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-g-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

	order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
		commissionSeedItem{productType: "medal", jumlah: 50000},
	)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))
	require.Empty(t, affiliateCommissionsForOrder(t, repo, order.ID))
}

func TestAffiliateCommission_DiscountSplitAcrossItems(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-comm-h-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-h-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

	paidAt := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	order := seedCommissionOrder(t, repo, student.ID, paidAt, 20000,
		commissionSeedItem{productType: "exam", jumlah: 100000},
		commissionSeedItem{productType: "course", jumlah: 100000},
	)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))

	byType := commissionsByType(affiliateCommissionsForOrder(t, repo, order.ID))
	require.Len(t, byType, 2)
	require.Equal(t, 90000.0, byType["exam"].BaseAmount)
	require.Equal(t, 9000.0, byType["exam"].Amount)
	require.Equal(t, 90000.0, byType["course"].BaseAmount)
	require.Equal(t, 9000.0, byType["course"].Amount)
}

func TestAffiliateCommission_ProductOverrideWins(t *testing.T) {
	svc, repo := newRealDBService(t)
	launch := time.Now().UTC().Add(-24 * time.Hour)
	seedAffiliateSettingsForBinding(t, repo, affiliateSettingsSeed{launchDate: &launch, bindingMonths: 3})
	setAffiliateFlagForCommission(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-comm-o-"+uniqueSuffix()+"@test.local", "", "active")
	student := seedAffiliateTestStudent(t, repo, "student-comm-o-"+uniqueSuffix()+"@test.local", "")
	createAffiliateBindingCommitted(t, svc, repo, student.ID, owner.affiliatorID, "checkout_code")

	order := seedCommissionOrder(t, repo, student.ID, time.Now().UTC(), 0,
		commissionSeedItem{productType: "exam", jumlah: 100000},
	)
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO affiliate_product_overrides (product_id, percentage) VALUES ($1, 15)`,
		order.Items[0].ProductID)
	require.NoError(t, err)

	require.NoError(t, runCreateCommissions(t, svc, repo, order))

	rows := affiliateCommissionsForOrder(t, repo, order.ID)
	require.Len(t, rows, 1)
	require.Equal(t, 15.0, rows[0].Percentage)
	require.Equal(t, 15000.0, rows[0].Amount)
}
