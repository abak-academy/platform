package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// setProductAgreementCheckboxes writes a single-row agreement_checkboxes
// array directly, bypassing the handler/service normalization path so the
// test controls the checkbox ID.
func setProductAgreementCheckboxes(t *testing.T, repo *repository.Repository, productID, checkboxID, title, body string) {
	t.Helper()
	data := fmt.Sprintf(`[{"id":%q,"title":%q,"body":%q}]`, checkboxID, title, body)
	_, err := repo.Pool().Exec(context.Background(),
		`UPDATE product SET agreement_checkboxes = $1 WHERE id = $2`, data, productID)
	if err != nil {
		t.Fatalf("set agreement_checkboxes: %v", err)
	}
}

// TestCheckout_AgreementCheckbox_RequiresAcceptance proves storefront checkout
// 400s (via ErrAgreementNotAccepted) until every configured checkbox on the
// order's product is agreed, and snapshots checkbox id/title/body/agreed_at
// onto the order item once it succeeds.
func TestCheckout_AgreementCheckbox_RequiresAcceptance(t *testing.T) {
	svc, repo := newCheckoutTestService(t)
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ('course', $1, 0, 0, 'published') RETURNING id`,
		"Agreement Course "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}
	linkFreshCourse(t, repo, productID)

	checkboxID := uuid.NewString()
	setProductAgreementCheckboxes(t, repo, productID, checkboxID, "Syarat", "Saya menyetujui syarat ini")

	studentID := insertCheckoutStudent(t, repo, "Agreement Student", "agreestu_")
	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if _, err := svc.Checkout(ctx, studentID, order.ID.String(), "agree-key-missing-"+uniqueSuffix(), "", nil, false); !errors.Is(err, ErrAgreementNotAccepted) {
		t.Fatalf("want ErrAgreementNotAccepted with no agreements, got %v", err)
	}

	if _, err := svc.Checkout(ctx, studentID, order.ID.String(), "agree-key-false-"+uniqueSuffix(), "",
		CheckoutAgreements{productID: {checkboxID: false}}, false); !errors.Is(err, ErrAgreementNotAccepted) {
		t.Fatalf("want ErrAgreementNotAccepted with agreed=false, got %v", err)
	}

	result, err := svc.Checkout(ctx, studentID, order.ID.String(), "agree-key-ok-"+uniqueSuffix(), "",
		CheckoutAgreements{productID: {checkboxID: true}}, false)
	if err != nil {
		t.Fatalf("Checkout with agreement accepted: %v", err)
	}
	if !result.Free {
		t.Fatalf("want free digital checkout, got %+v", result)
	}

	var snapshotRaw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT agreement_snapshot FROM order_item WHERE order_id = $1`, order.ID).Scan(&snapshotRaw); err != nil {
		t.Fatalf("read agreement_snapshot: %v", err)
	}
	var snapshot []model.OrderItemAgreement
	if err := json.Unmarshal(snapshotRaw, &snapshot); err != nil {
		t.Fatalf("decode agreement_snapshot: %v", err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("want 1 snapshot row, got %d (%+v)", len(snapshot), snapshot)
	}
	if snapshot[0].CheckboxID != checkboxID || snapshot[0].Title != "Syarat" || snapshot[0].Body != "Saya menyetujui syarat ini" {
		t.Fatalf("snapshot content mismatch: %+v", snapshot[0])
	}
	if snapshot[0].AgreedAt.IsZero() {
		t.Fatalf("want a non-zero AgreedAt, got zero")
	}
}

// TestCheckout_AgreementSnapshot_SurvivesProductEdit proves a later edit to
// product.agreement_checkboxes does not rewrite an order already checked out.
func TestCheckout_AgreementSnapshot_SurvivesProductEdit(t *testing.T) {
	svc, repo := newCheckoutTestService(t)
	ctx := context.Background()

	var productID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, stock, status)
		 VALUES ('course', $1, 0, 0, 'published') RETURNING id`,
		"Agreement Edit Course "+uuid.New().String(),
	).Scan(&productID); err != nil {
		t.Fatalf("create product: %v", err)
	}
	linkFreshCourse(t, repo, productID)

	checkboxID := uuid.NewString()
	setProductAgreementCheckboxes(t, repo, productID, checkboxID, "Judul Asli", "Isi Asli")

	studentID := insertCheckoutStudent(t, repo, "Agreement Edit Student", "agreeeditstu_")
	order, _, err := svc.MintCart(ctx, studentID)
	if err != nil {
		t.Fatalf("MintCart: %v", err)
	}
	if err := svc.AddItem(ctx, studentID, order.ID.String(), productID, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if _, err := svc.Checkout(ctx, studentID, order.ID.String(), "agree-edit-key-"+uniqueSuffix(), "",
		CheckoutAgreements{productID: {checkboxID: true}}, false); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	setProductAgreementCheckboxes(t, repo, productID, checkboxID, "Judul Baru", "Isi Baru")

	var snapshotRaw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT agreement_snapshot FROM order_item WHERE order_id = $1`, order.ID).Scan(&snapshotRaw); err != nil {
		t.Fatalf("read agreement_snapshot: %v", err)
	}
	var snapshot []model.OrderItemAgreement
	if err := json.Unmarshal(snapshotRaw, &snapshot); err != nil {
		t.Fatalf("decode agreement_snapshot: %v", err)
	}
	if len(snapshot) != 1 || snapshot[0].Title != "Judul Asli" || snapshot[0].Body != "Isi Asli" {
		t.Fatalf("snapshot must keep the text as agreed, got %+v", snapshot)
	}
}

// TestCheckout_BulkExamOrder_SkipsAgreementCheck proves AdminCheckoutBulkOrder
// (skipAgreementCheck=true) succeeds with no agreements payload even when the
// product has checkboxes configured, while the same order without the opt-out
// would be refused.
func TestCheckout_BulkExamOrder_SkipsAgreementCheck(t *testing.T) {
	svc, repo := newCheckoutTestService(t)
	ctx := context.Background()

	schoolID := createTestSchool(t, svc)
	examID := createTestExamForBulk(t, repo)
	productID := createTestExamProductForBulk(t, repo, examID, 0)

	checkboxID := uuid.NewString()
	setProductAgreementCheckboxes(t, repo, productID, checkboxID, "Syarat Ujian", "Peserta menyetujui")

	var adminID string
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, role, status, username, password_hash, jenjang)
		 VALUES ($1, 'admin_school', 'active', $2, '', 'sma')
		 RETURNING id`,
		"Bulk Agreement Admin", "bulkagreeadm_"+uniqueSuffix(),
	).Scan(&adminID); err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	insertBulkStudent := func(prefix string) string {
		var studentID string
		if err := repo.Pool().QueryRow(ctx,
			`INSERT INTO users (name, school_id, role, status, username, password_hash, jenjang)
			 VALUES ($1, $2, 'student', 'active', $3, '', 'sma')
			 RETURNING id`,
			"Bulk Agreement Student", schoolID, prefix+uniqueSuffix(),
		).Scan(&studentID); err != nil {
			t.Fatalf("insert student: %v", err)
		}
		return studentID
	}

	t.Run("skip=true succeeds with no agreements payload", func(t *testing.T) {
		studentID := insertBulkStudent("bulkagreestu1_")
		order, err := svc.CreateBulkExamOrder(ctx, adminID, schoolID, examID, ParticipantSelector{StudentIDs: []string{studentID}})
		if err != nil {
			t.Fatalf("CreateBulkExamOrder: %v", err)
		}
		result, err := svc.Checkout(ctx, adminID, order.ID.String(), "bulk-agree-skip-"+uniqueSuffix(), "", nil, true)
		if err != nil {
			t.Fatalf("Checkout with skipAgreementCheck=true: want success, got %v", err)
		}
		if !result.Free {
			t.Fatalf("want free checkout, got %+v", result)
		}
	})

	t.Run("skip=false is refused for the same configured checkbox", func(t *testing.T) {
		studentID := insertBulkStudent("bulkagreestu2_")
		order, err := svc.CreateBulkExamOrder(ctx, adminID, schoolID, examID, ParticipantSelector{StudentIDs: []string{studentID}})
		if err != nil {
			t.Fatalf("CreateBulkExamOrder: %v", err)
		}
		if _, err := svc.Checkout(ctx, adminID, order.ID.String(), "bulk-agree-noskip-"+uniqueSuffix(), "", nil, false); !errors.Is(err, ErrAgreementNotAccepted) {
			t.Fatalf("want ErrAgreementNotAccepted without the opt-out, got %v", err)
		}
	})
}

// TestCheckoutPartnerOrder_AgreementCheckbox_RequiresAcceptance mirrors the
// storefront gate for the partner/Mitra checkout path.
func TestCheckoutPartnerOrder_AgreementCheckbox_RequiresAcceptance(t *testing.T) {
	env := newPartnerOrderEnv(t)
	ctx := context.Background()
	product, _ := env.courseProduct(t, 100000)
	env.open(t, product)

	checkboxID := uuid.NewString()
	setProductAgreementCheckboxes(t, env.repo, product.String(), checkboxID, "Syarat Mitra", "Mitra menyetujui")

	a := env.student(t, "Ani", env.schoolA, "")
	full := env.voucher(t, 100, "student", i(2))

	order, err := env.svc.CreatePartnerOrder(ctx, env.partner, []string{product.String()}, []string{a}, full.Code)
	require.NoError(t, err)

	_, err = env.svc.CheckoutPartnerOrder(ctx, env.partner, order.ID.String(), "partner-agree-missing-"+uniqueSuffix(), nil)
	require.ErrorIs(t, err, ErrAgreementNotAccepted)

	res, err := env.svc.CheckoutPartnerOrder(ctx, env.partner, order.ID.String(), "partner-agree-ok-"+uniqueSuffix(),
		CheckoutAgreements{product.String(): {checkboxID: true}})
	require.NoError(t, err)
	require.True(t, res.Free)

	var snapshotRaw []byte
	require.NoError(t, env.repo.Pool().QueryRow(ctx, `SELECT agreement_snapshot FROM order_item WHERE order_id = $1`, order.ID).Scan(&snapshotRaw))
	var snapshot []model.OrderItemAgreement
	require.NoError(t, json.Unmarshal(snapshotRaw, &snapshot))
	require.Len(t, snapshot, 1)
	require.Equal(t, checkboxID, snapshot[0].CheckboxID)
	require.Equal(t, "Syarat Mitra", snapshot[0].Title)
}
