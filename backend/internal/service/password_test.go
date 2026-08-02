package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// The test-only cost is a CI speedup, not a licence to weaken the shipped hash.
func TestProductionBcryptCostIsNotWeakened(t *testing.T) {
	if productionBcryptCost < 12 {
		t.Fatalf("production bcrypt cost dropped to %d, want >= 12", productionBcryptCost)
	}
}

func TestHashPasswordUsesMinCostUnderTest(t *testing.T) {
	hash, err := hashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	cost, err := bcrypt.Cost(hash)
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost != bcrypt.MinCost {
		t.Errorf("cost = %d, want %d — the CI speedup is not in effect", cost, bcrypt.MinCost)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("correct-horse-battery-staple")); err != nil {
		t.Errorf("hash does not verify: %v", err)
	}
}
