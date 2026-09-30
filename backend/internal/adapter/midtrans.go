package adapter

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"akademi-bimbel/internal/service"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

// MidtransClient wraps Midtrans Snap and CoreAPI clients.
type MidtransClient struct {
	serverKey string
	snap      snap.Client
	core      coreapi.Client
}

// NewMidtransClient creates a real MidtransClient. Unknown env values default to sandbox.
func NewMidtransClient(serverKey, clientKey, env string) *MidtransClient {
	mtEnv := midtrans.Sandbox
	if strings.EqualFold(env, "production") {
		mtEnv = midtrans.Production
	}

	snapClient := &snap.Client{}
	snapClient.New(serverKey, mtEnv)

	coreClient := &coreapi.Client{}
	coreClient.New(serverKey, mtEnv)

	return &MidtransClient{
		serverKey: serverKey,
		snap:      *snapClient,
		core:      *coreClient,
	}
}

func (m *MidtransClient) CreatePayment(ctx context.Context, req service.PaymentRequest) (service.PaymentResponse, error) {
	snapReq := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.OrderID,
			GrossAmt: req.Amount,
		},
	}

	if len(req.Items) > 0 {
		items := make([]midtrans.ItemDetails, len(req.Items))
		for i, it := range req.Items {
			items[i] = midtrans.ItemDetails{
				ID:       it.ID,
				Name:     it.Name,
				Price:    it.Price,
				Qty:      it.Qty,
				Category: it.Category,
			}
		}
		snapReq.Items = &items
	}

	if req.Customer.Name != "" || req.Customer.Email != "" {
		snapReq.CustomerDetail = &midtrans.CustomerDetails{
			FName: req.Customer.Name,
			Email: req.Customer.Email,
			Phone: req.Customer.Phone,
		}
	}

	if req.CallbackURL != "" {
		snapReq.Callbacks = &snap.Callbacks{Finish: req.CallbackURL}
	}

	snapReq.Expiry = &snap.ExpiryDetails{
		Duration: int64(req.ExpiresIn.Hours()),
		Unit:     "hour",
	}

	resp, err := m.snap.CreateTransaction(snapReq)
	if err != nil {
		return service.PaymentResponse{}, err
	}

	return service.PaymentResponse{
		GatewayRef: req.OrderID,
		SnapToken:  resp.Token,
		PaymentURL: resp.RedirectURL,
		ExpiresAt:  time.Now().Add(req.ExpiresIn),
	}, nil
}

func (m *MidtransClient) QueryStatus(ctx context.Context, reference string) (service.PaymentStatus, error) {
	resp, err := m.core.CheckTransaction(reference)
	if err != nil {
		return service.PaymentStatus{Reference: reference, Paid: false}, err
	}

	status := resp.TransactionStatus
	return service.PaymentStatus{
		Reference: reference,
		Paid:      status == "settlement" || status == "capture",
	}, nil
}

func (m *MidtransClient) VerifySignature(payload []byte, signature string) bool {
	var notif struct {
		OrderID     string `json:"order_id"`
		StatusCode  string `json:"status_code"`
		GrossAmount string `json:"gross_amount"`
	}

	if err := json.Unmarshal(payload, &notif); err != nil {
		return false
	}

	computed := sha512Hex(notif.OrderID + notif.StatusCode + notif.GrossAmount + m.serverKey)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}

func sha512Hex(input string) string {
	sum := sha512.Sum512([]byte(input))
	return hex.EncodeToString(sum[:])
}
