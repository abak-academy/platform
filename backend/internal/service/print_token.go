package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

// MintPrintToken issues an opaque, single-use token bound to kind and
// subjectID, valid for at most 60 seconds (FR-13, NFR-S3).
func (s *Service) MintPrintToken(ctx context.Context, kind PrintTokenKind, subjectID string) (string, error) {
	if kind != PrintTokenKindCertificate && kind != PrintTokenKindCard {
		return "", fmt.Errorf("invalid print token kind: %q", kind)
	}
	if subjectID == "" {
		return "", errors.New("print token subject id required")
	}

	token, err := randomPrintToken()
	if err != nil {
		return "", err
	}

	if err := s.rdb.Set(ctx, printTokenKey(token), printTokenValue(kind, subjectID), printTokenTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// RedeemPrintToken atomically consumes token and validates it was minted for
// kind and subjectID. Redemption uses GETDEL so concurrent callers racing on
// the same token can never both succeed (FR-14, FR-15, FR-17).
func (s *Service) RedeemPrintToken(ctx context.Context, token string, kind PrintTokenKind, subjectID string) error {
	val, err := s.rdb.GetDel(ctx, printTokenKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return ErrPrintTokenInvalid
	}
	if err != nil {
		return err
	}
	if val != printTokenValue(kind, subjectID) {
		return ErrPrintTokenInvalid
	}
	return nil
}

func printTokenKey(token string) string {
	return "printtoken:" + token
}

func printTokenValue(kind PrintTokenKind, subjectID string) string {
	return string(kind) + ":" + subjectID
}

func randomPrintToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
