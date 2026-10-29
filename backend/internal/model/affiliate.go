package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Affiliator struct {
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id"`
	Code              string    `json:"code"`
	Status            string    `json:"status"`
	RejectionReason   *string   `json:"rejection_reason"`
	Level             string    `json:"level"`
	WarnQuarters      int       `json:"warn_quarters"`
	TermsVersion      string    `json:"terms_version"`
	TermsAcceptedAt   time.Time `json:"terms_accepted_at"`
	BankName          *string   `json:"bank_name"`
	BankAccountNumber *string   `json:"bank_account_number"`
	BankAccountHolder *string   `json:"bank_account_holder"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AffiliateBinding struct {
	ID             uuid.UUID  `json:"id"`
	AffiliatorID   uuid.UUID  `json:"affiliator_id"`
	StudentID      uuid.UUID  `json:"student_id"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at"`
	LastExtendedAt *time.Time `json:"last_extended_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AffiliateCommission struct {
	ID              uuid.UUID  `json:"id"`
	AffiliatorID    uuid.UUID  `json:"affiliator_id"`
	StudentID       uuid.UUID  `json:"student_id"`
	OrderID         uuid.UUID  `json:"order_id"`
	OrderItemID     uuid.UUID  `json:"order_item_id"`
	ProductType     string     `json:"product_type"`
	Quantity        int        `json:"quantity,omitempty"`
	BaseAmount      float64    `json:"base_amount"`
	CommissionType  string     `json:"commission_type"`
	CommissionValue float64    `json:"commission_value"`
	LevelSnapshot   string     `json:"level_snapshot"`
	Amount          float64    `json:"amount"`
	Status          string     `json:"status"`
	WaitUntil       *time.Time `json:"wait_until"`
	PaidAt          *time.Time `json:"paid_at"`
	ReceivedAt      *time.Time `json:"received_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Append-only: UPDATE/DELETE are rejected by the DB trigger (0070).
type AffiliateLedgerEntry struct {
	ID           uuid.UUID  `json:"id"`
	AffiliatorID uuid.UUID  `json:"affiliator_id"`
	EntryType    string     `json:"entry_type"`
	Amount       float64    `json:"amount"`
	CommissionID *uuid.UUID `json:"commission_id"`
	PayoutID     *uuid.UUID `json:"payout_id"`
	AdjustmentID *uuid.UUID `json:"adjustment_id"`
	Note         *string    `json:"note"`
	// nil means the mutation was made by an automated job, not an admin.
	ActorID   *uuid.UUID `json:"actor_id"`
	CreatedAt time.Time  `json:"created_at"`
}

type AffiliatePayout struct {
	ID                uuid.UUID  `json:"id"`
	AffiliatorID      uuid.UUID  `json:"affiliator_id"`
	Amount            float64    `json:"amount"`
	Status            string     `json:"status"`
	BankName          string     `json:"bank_name"`
	BankAccountNumber string     `json:"bank_account_number"`
	BankAccountHolder string     `json:"bank_account_holder"`
	ProofURL          *string    `json:"proof_url"`
	RejectReason      *string    `json:"reject_reason"`
	ProcessedBy       *uuid.UUID `json:"processed_by"`
	ProcessedAt       *time.Time `json:"processed_at"`
	RequestedAt       time.Time  `json:"requested_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type AffiliateAdjustment struct {
	ID             uuid.UUID  `json:"id"`
	AffiliatorID   uuid.UUID  `json:"affiliator_id"`
	Category       string     `json:"category"`
	Amount         float64    `json:"amount"`
	Reason         string     `json:"reason"`
	Note           *string    `json:"note"`
	ApprovalStatus *string    `json:"approval_status"`
	ApproverID     *uuid.UUID `json:"approver_id"`
	ApprovedAt     *time.Time `json:"approved_at"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
}

type AffiliateLevelHistory struct {
	ID           uuid.UUID  `json:"id"`
	AffiliatorID uuid.UUID  `json:"affiliator_id"`
	Quarter      string     `json:"quarter"`
	FromLevel    *string    `json:"from_level"`
	ToLevel      string     `json:"to_level"`
	NetRevenue   float64    `json:"net_revenue"`
	Source       string     `json:"source"`
	Reason       *string    `json:"reason"`
	CreatedBy    *uuid.UUID `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Singleton settings row: the DB pins the primary key to 1 (0070).
type AffiliateSettings struct {
	ID                   int        `json:"id"`
	CookieDays           int        `json:"cookie_days"`
	BindingMonths        int        `json:"binding_months"`
	MinExtendAmount      float64    `json:"min_extend_amount"`
	DigitalWaitDays      int        `json:"digital_wait_days"`
	PhysicalWaitDays     int        `json:"physical_wait_days"`
	MinPayout            float64    `json:"min_payout"`
	PayoutDayOfMonth     *int       `json:"payout_day_of_month"`
	LaunchDate           *time.Time `json:"launch_date"`
	EvalDay              int        `json:"eval_day"`
	SilverThreshold      float64    `json:"silver_threshold"`
	GoldThreshold        float64    `json:"gold_threshold"`
	DailyRegistrationCap *int       `json:"daily_registration_cap"`
	UpdatedBy            *uuid.UUID `json:"updated_by"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type AffiliateLevelSettings struct {
	Level            string          `json:"level"`
	CommissionRule   json.RawMessage `json:"commission_rule"`
	BindingMonths    *int            `json:"binding_months"`
	DigitalWaitDays  *int            `json:"digital_wait_days"`
	PhysicalWaitDays *int            `json:"physical_wait_days"`
	MinPayout        *float64        `json:"min_payout"`
	PayoutDayOfMonth *int            `json:"payout_day_of_month"`
	RiseThreshold    *float64        `json:"rise_threshold"`
	HoldThreshold    *float64        `json:"hold_threshold"`
}

// AffiliateCommissionRule is one commission setting: a percentage of the item
// base, or a fixed rupiah amount per unit.
type AffiliateCommissionRule struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

type AffiliateProductOverride struct {
	ProductID uuid.UUID  `json:"product_id"`
	Type      string     `json:"type"`
	Value     float64    `json:"value"`
	UpdatedBy *uuid.UUID `json:"updated_by"`
	UpdatedAt time.Time  `json:"updated_at"`
}
