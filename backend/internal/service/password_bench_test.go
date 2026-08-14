package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// GenerateFromPassword at cost 12 costs about the same as CompareHashAndPassword at cost 12,
// so a single benchmark of the verify path stands in for both.
func BenchmarkPasswordVerify_ProductionCost(b *testing.B) {
	pw := "correct-horse-battery-staple"
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), productionBcryptCost)
	if err != nil {
		b.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	if c, err := bcrypt.Cost(hash); err != nil || c != productionBcryptCost {
		b.Fatalf("hash cost = %d (err %v), want %d — benchmark would measure the wrong cost", c, err, productionBcryptCost)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bcrypt.CompareHashAndPassword(hash, []byte(pw)); err != nil {
			b.Fatalf("bcrypt.CompareHashAndPassword: %v", err)
		}
	}
}
