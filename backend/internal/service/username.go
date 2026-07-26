package service

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
)

// GenerateUsername creates a base username from a full name: lowercase, strip
// all whitespace, take the first 4 runes (fewer if the stripped name is shorter).
// The caller appends 4 random digits for uniqueness.
func GenerateUsername(fullName string) string {
	name := strings.TrimSpace(fullName)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "\t", "")

	runes := []rune(name)
	if len(runes) > 4 {
		runes = runes[:4]
	}
	return string(runes)
}

// generateUniqueUsername generates a unique username from fullName by appending
// 4 random digits to the base from GenerateUsername and checking for existing
// collisions via GetUserByUsername. Retries up to 10 total attempts before
// returning ErrUsernameGenerationExhausted.
func (s *Service) generateUniqueUsername(ctx context.Context, fullName string) (string, error) {
	base := GenerateUsername(fullName)
	for i := 0; i < 10; i++ {
		digits, err := randomDigits(4)
		if err != nil {
			return "", err
		}
		username := base + digits
		existing, err := s.repo.GetUserByUsername(ctx, username)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return username, nil
		}
	}
	return "", ErrUsernameGenerationExhausted
}

// randomDigits returns a string of n random decimal digits using crypto/rand.
func randomDigits(n int) (string, error) {
	const charset = "0123456789"
	result := make([]byte, n)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}
