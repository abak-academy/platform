package service

import (
	"context"
	"time"
)

type ItemDetail struct {
	ID       string
	Name     string
	Price    int64
	Qty      int32
	Category string
}

type CustomerInfo struct {
	Name  string
	Email string
	Phone string
}

type PaymentRequest struct {
	OrderID     string
	Reference   string
	Amount      int64
	ExpiresIn   time.Duration
	Items       []ItemDetail
	Customer    CustomerInfo
	CallbackURL string
}

type PaymentResponse struct {
	GatewayRef string
	PaymentURL string
	SnapToken  string
	ExpiresAt  time.Time
}

type PaymentStatus struct {
	Reference string
	Paid      bool
	PaidAt    *time.Time
}

// PaymentClient is the payment-gateway port. Midtrans is the real implementation
// (internal/adapter); NoopPaymentClient is the fallback when no gateway is
// configured.
type PaymentClient interface {
	CreatePayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error)
	QueryStatus(ctx context.Context, reference string) (PaymentStatus, error)
	VerifySignature(payload []byte, signature string) bool
}

type NoopPaymentClient struct{}

func (n *NoopPaymentClient) CreatePayment(ctx context.Context, req PaymentRequest) (PaymentResponse, error) {
	return PaymentResponse{
		GatewayRef: "noop-" + req.OrderID,
		PaymentURL: "https://noop.payment/pay/" + req.OrderID,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}, nil
}

func (n *NoopPaymentClient) QueryStatus(ctx context.Context, reference string) (PaymentStatus, error) {
	return PaymentStatus{Reference: reference, Paid: false}, nil
}

// VerifySignature rejects everything: NoopPaymentClient means no gateway is
// configured, so no signature could be authentic. Returning true would let anyone
// settle any order via the unauthenticated webhook before keys are entered.
func (n *NoopPaymentClient) VerifySignature(payload []byte, signature string) bool {
	return false
}
