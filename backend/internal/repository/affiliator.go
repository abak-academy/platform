package repository

import (
	"encoding/json"

	"akademi-bimbel/internal/model"
)

const (
	affiliatorColumns = `a.user_id, COALESCE(a.referral_code, ''), a.program_status, a.rejection_reason, a.level,
		a.missed_quarters, a.last_evaluated_quarter, a.terms_version, a.bank_account, u.created_at`
	affiliatorFrom = `FROM affiliators a JOIN users u ON u.id = a.user_id`
)

// affiliatorBankAccount is the JSON shape of affiliators.bank_account.
type affiliatorBankAccount struct {
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}

// affiliatorScan reads the affiliatorColumns list; bank_account arrives as raw
// JSON and is unpacked by result.
type affiliatorScan struct {
	a    model.Affiliator
	bank []byte
}

func (s *affiliatorScan) dest() []any {
	return []any{&s.a.ID, &s.a.Code, &s.a.Status, &s.a.RejectionReason, &s.a.Level,
		&s.a.WarnQuarters, &s.a.LastEvaluatedQuarter, &s.a.TermsVersion, &s.bank, &s.a.CreatedAt}
}

func (s *affiliatorScan) result() (model.Affiliator, error) {
	if len(s.bank) > 0 {
		var bank affiliatorBankAccount
		if err := json.Unmarshal(s.bank, &bank); err != nil {
			return model.Affiliator{}, err
		}
		s.a.BankName, s.a.BankAccountNumber, s.a.BankAccountHolder = &bank.BankName, &bank.AccountNumber, &bank.AccountHolder
	}
	return s.a, nil
}

type affiliatorRow interface {
	Scan(dest ...any) error
}

// scanAffiliator reads one affiliatorColumns row; a missing row surfaces the
// driver's not-found error for the caller's isNotFound check.
func scanAffiliator(row affiliatorRow) (*model.Affiliator, error) {
	var s affiliatorScan
	if err := row.Scan(s.dest()...); err != nil {
		return nil, err
	}
	a, err := s.result()
	if err != nil {
		return nil, err
	}
	return &a, nil
}
