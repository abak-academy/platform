package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// PrintTokenKind is the document kind a print token is scoped to.
type PrintTokenKind string

const (
	PrintTokenKindCertificate PrintTokenKind = "certificate"
	PrintTokenKindCard        PrintTokenKind = "card"
)

const printTokenTTL = 60 * time.Second

// ErrPrintTokenInvalid covers every redemption failure mode (not found,
// expired, already redeemed, wrong kind, wrong subject) with a single
// sentinel so callers can't distinguish them and probe for valid scope.
var ErrPrintTokenInvalid = errors.New("print token invalid or already redeemed")

// printTokenRecord is the JSON envelope stored in Redis for every print
// token. An admin certificate preview (FR-29) carries its unsaved
// template/layout override here so it travels in the token payload and never
// in the rendered URL (NFR-S5) — Preview/TemplateOverride/LayoutOverride are
// always zero for a real certificate or card token.
type printTokenRecord struct {
	Kind             PrintTokenKind `json:"kind"`
	SubjectID        string         `json:"subject_id"`
	Preview          bool           `json:"preview,omitempty"`
	TemplateOverride string         `json:"template_override,omitempty"`
	LayoutOverride   *Layout        `json:"layout_override,omitempty"`
}

// MintPrintToken issues an opaque, single-use token bound to kind and
// subjectID, valid for at most 60 seconds (FR-13, NFR-S3).
func (s *Service) MintPrintToken(ctx context.Context, kind PrintTokenKind, subjectID string) (string, error) {
	if kind != PrintTokenKindCertificate && kind != PrintTokenKindCard {
		return "", fmt.Errorf("invalid print token kind: %q", kind)
	}
	if subjectID == "" {
		return "", errors.New("print token subject id required")
	}
	return s.mintPrintTokenRecord(ctx, printTokenRecord{Kind: kind, SubjectID: subjectID})
}

// MintCertificatePreviewToken issues a print token scoped to the certificate
// document kind and examID, carrying an admin's unsaved template/layout
// override (FR-29). layoutOverride may be nil to preview the exam's saved (or
// default) layout.
func (s *Service) MintCertificatePreviewToken(ctx context.Context, examID, templateOverride string, layoutOverride *Layout) (string, error) {
	if examID == "" {
		return "", errors.New("print token subject id required")
	}
	return s.mintPrintTokenRecord(ctx, printTokenRecord{
		Kind:             PrintTokenKindCertificate,
		SubjectID:        examID,
		Preview:          true,
		TemplateOverride: templateOverride,
		LayoutOverride:   layoutOverride,
	})
}

func (s *Service) mintPrintTokenRecord(ctx context.Context, record printTokenRecord) (string, error) {
	token, err := randomPrintToken()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, printTokenKey(token), raw, printTokenTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// RedeemPrintToken atomically consumes token and validates it was minted for
// kind and subjectID. Redemption uses GETDEL so concurrent callers racing on
// the same token can never both succeed (FR-14, FR-15, FR-17).
func (s *Service) RedeemPrintToken(ctx context.Context, token string, kind PrintTokenKind, subjectID string) error {
	record, err := s.redeemPrintTokenRecord(ctx, token)
	if err != nil {
		return err
	}
	if record.Kind != kind || record.SubjectID != subjectID {
		return ErrPrintTokenInvalid
	}
	return nil
}

// RedeemCertificateToken redeems a print token scoped to the certificate
// document kind and subjectID — a session id for a real certificate, an exam
// id for an admin preview (FR-29) — and returns the decoded record so the
// caller can dispatch between a real session render and a preview render.
func (s *Service) RedeemCertificateToken(ctx context.Context, token, subjectID string) (*printTokenRecord, error) {
	record, err := s.redeemPrintTokenRecord(ctx, token)
	if err != nil {
		return nil, err
	}
	if record.Kind != PrintTokenKindCertificate || record.SubjectID != subjectID {
		return nil, ErrPrintTokenInvalid
	}
	return record, nil
}

func (s *Service) redeemPrintTokenRecord(ctx context.Context, token string) (*printTokenRecord, error) {
	val, err := s.rdb.GetDel(ctx, printTokenKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrPrintTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	var record printTokenRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return nil, ErrPrintTokenInvalid
	}
	return &record, nil
}

func printTokenKey(token string) string {
	return "printtoken:" + token
}

func randomPrintToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
