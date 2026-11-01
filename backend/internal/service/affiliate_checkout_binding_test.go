package service

import (
	"context"
	"testing"

	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

func seedAffiliateCheckoutOrder(t *testing.T, svc *Service, repo *repository.Repository, buyerID string, code string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	course := seedCourseTarget(t, repo)
	product := seedDigitalProduct(t, repo, "course", 50000, course)
	orderID := cartOf(t, svc, uuid.MustParse(buyerID), product)
	key := "aff-checkout-" + uniqueSuffix()
	var err error
	if code == "" {
		_, err = svc.Checkout(ctx, buyerID, orderID.String(), key, "")
	} else {
		_, err = svc.Checkout(ctx, buyerID, orderID.String(), key, code)
	}
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return orderID
}

func TestCheckoutPaid_WithAffiliateCode_BindingBornAtPaymentOnce(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-checkout-once-"+uniqueSuffix()+"@test.local", "", "active")
	buyerID := insertCheckoutStudent(t, repo, "Aff Checkout Once", "affco1_")

	orderID := seedAffiliateCheckoutOrder(t, svc, repo, buyerID, owner.code)

	// Checkout alone must not bind: the binding is born when the order is paid.
	if binding := getActiveAffiliateBinding(t, repo, buyerID); binding != nil {
		t.Fatalf("binding created at checkout, want none until payment: %+v", binding)
	}
	var storedCode *string
	if err := repo.Pool().QueryRow(ctx, `SELECT affiliate_code FROM orders WHERE id = $1`, orderID).Scan(&storedCode); err != nil {
		t.Fatalf("read order affiliate_code: %v", err)
	}
	if storedCode == nil || *storedCode != owner.code {
		t.Fatalf("orders.affiliate_code = %v, want %q", storedCode, owner.code)
	}

	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-aff-once"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	binding := getActiveAffiliateBinding(t, repo, buyerID)
	if binding == nil {
		t.Fatal("want a binding born at payment")
	}
	if binding.AffiliatorID != owner.affiliatorID {
		t.Errorf("binding affiliator = %v, want %v", binding.AffiliatorID, owner.affiliatorID)
	}
	if binding.Source != affiliateSourceCheckoutCode {
		t.Errorf("binding source = %q, want %q", binding.Source, affiliateSourceCheckoutCode)
	}

	// A duplicate webhook delivery must not rebind or re-extend.
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-aff-once"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("duplicate webhook: %v", err)
	}
	if n := countAffiliateBindings(t, repo, buyerID); n != 1 {
		t.Errorf("bindings after duplicate webhook = %d, want 1", n)
	}
}

func TestPaidOrder_WithoutCode_ExtendsExistingBinding(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-extend-paid-"+uniqueSuffix()+"@test.local", "", "active")
	buyerID := insertCheckoutStudent(t, repo, "Aff Extend Paid", "affext_")

	createAffiliateBindingCommitted(t, svc, repo, buyerID, owner.affiliatorID, affiliateSourceManualCode)
	before := getActiveAffiliateBinding(t, repo, buyerID)
	if before == nil {
		t.Fatal("pre-existing binding missing")
	}

	orderID := seedAffiliateCheckoutOrder(t, svc, repo, buyerID, "")
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-aff-extend"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	after := getActiveAffiliateBinding(t, repo, buyerID)
	if after == nil {
		t.Fatal("binding must stay active after a paid order")
	}
	if !after.ExpiresAt.After(*before.ExpiresAt) {
		t.Errorf("expires_at = %v, want re-anchored beyond %v by the payment", after.ExpiresAt, before.ExpiresAt)
	}

	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-aff-extend"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("duplicate webhook: %v", err)
	}
	dup := getActiveAffiliateBinding(t, repo, buyerID)
	if !dup.ExpiresAt.Equal(*after.ExpiresAt) {
		t.Errorf("expires_at = %v, want unchanged %v after a duplicate delivery", dup.ExpiresAt, after.ExpiresAt)
	}
}

func TestCheckout_FreeOrder_WithAffiliateCode_BindsAtSettlement(t *testing.T) {
	svc, repo := newCheckoutTestService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")
	owner := seedAffiliatorOwner(t, repo, "aff-free-"+uniqueSuffix()+"@test.local", "", "active")
	buyerID := insertCheckoutStudent(t, repo, "Aff Free Buyer", "afffree_")

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ('course', $1, 0, 0, 'published') RETURNING id`,
		"Free Course "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}
	linkFreshCourse(t, repo, productID)

	order, _, err := svc.MintCart(ctx, buyerID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, buyerID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if _, err := svc.Checkout(ctx, buyerID, order.ID.String(), "free-aff-"+uniqueSuffix(), owner.code); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	got, err := svc.GetStudentOrder(ctx, buyerID, order.ID.String())
	if err != nil {
		t.Fatalf("GetStudentOrder: %v", err)
	}
	if got.Status != "paid" {
		t.Fatalf("order status = %q, want paid", got.Status)
	}
	binding := getActiveAffiliateBinding(t, repo, buyerID)
	if binding == nil {
		t.Fatal("want a binding created at the free-order settlement")
	}
	if binding.AffiliatorID != owner.affiliatorID {
		t.Errorf("binding affiliator = %v, want %v", binding.AffiliatorID, owner.affiliatorID)
	}
	if binding.Source != affiliateSourceCheckoutCode {
		t.Errorf("binding source = %q, want %q", binding.Source, affiliateSourceCheckoutCode)
	}
}

func TestPaidOrder_FlagOff_NoBindingNoExtension(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "false")
	owner := seedAffiliatorOwner(t, repo, "aff-flagoff-"+uniqueSuffix()+"@test.local", "", "active")
	buyerID := insertCheckoutStudent(t, repo, "Aff Flag Off", "affflag_")

	createAffiliateBindingCommitted(t, svc, repo, buyerID, owner.affiliatorID, affiliateSourceManualCode)
	before := getActiveAffiliateBinding(t, repo, buyerID)

	orderID := seedAffiliateCheckoutOrder(t, svc, repo, buyerID, owner.code)
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-aff-off"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	after := getActiveAffiliateBinding(t, repo, buyerID)
	if after == nil {
		t.Fatal("existing binding must survive a flag-off settlement")
	}
	if !after.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Errorf("expires_at = %v, want untouched %v with the flag off", after.ExpiresAt, before.ExpiresAt)
	}
}

func TestPaidOrder_SelfPurchase_NoBinding(t *testing.T) {
	svc, repo, _ := newLifecycleService(t)
	ctx := context.Background()
	seedPostLaunchAffiliateSettings(t, repo)
	setAffiliateFlag(t, repo, "true")

	// The schema keeps emails unique per user, so self-purchase is detectable
	// via the shared phone number.
	phone := "+62812" + uniqueSuffix()[:6]
	var buyerID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (email, name, role, status, username, phone)
		 VALUES ($1, 'Self Buyer', 'student', 'active', $2, $3) RETURNING id`,
		"self-buyer-"+uniqueSuffix()+"@test.local", "selfbuy_"+uniqueSuffix(), phone,
	).Scan(&buyerID); err != nil {
		t.Fatalf("insert buyer: %v", err)
	}
	owner := seedAffiliatorOwner(t, repo, "self-owner-"+uniqueSuffix()+"@test.local", phone, "active")

	orderID := seedAffiliateCheckoutOrder(t, svc, repo, buyerID, owner.code)
	if err := svc.HandlePaymentWebhook(ctx, settlementBody(orderID, "tx-aff-self"), "sig", uuid.NewString()); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if n := countAffiliateBindings(t, repo, buyerID); n != 0 {
		t.Errorf("bindings after self-purchase = %d, want 0", n)
	}
}
