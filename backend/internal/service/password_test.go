package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// The test-only cost is a CI speedup, not a licence to weaken the shipped hash.
//
// The floor moved 12 -> 10 on 2026-08-14, deliberately. 12 was never chosen: it
// was hard-coded in the first auth commit (09e5793, 2026-06-12) with no recorded
// rationale, and only ever moved for CI reasons. The login spike is CPU-bound
// on a 2-vCPU VM where cost 12 costs
// 4x cost 10 — ~18 minutes versus ~4.5 for 3000 students. See issue #94.
//
// 10 is bcrypt's own default and the floor below which this must not go.
// Operator-set passwords make the work-factor trade-off security-sensitive;
// revisit this upward when capacity permits.
func TestProductionBcryptCostIsNotWeakened(t *testing.T) {
	if productionBcryptCost < 10 {
		t.Fatalf("production bcrypt cost dropped to %d, want >= 10", productionBcryptCost)
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
