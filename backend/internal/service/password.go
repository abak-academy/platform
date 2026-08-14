package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// 10 is bcrypt's own default. Every password this hashes is machine-generated
// (genTempPassword: 10 runes from a 62-rune alphabet via crypto/rand, ~59 bits),
// and no work factor is what protects those — see TestProductionBcryptCostIsNotWeakened.
const productionBcryptCost = 10

// A cost-12 hash takes ~2.5s under -race, which put the auth tests on the CI critical path.
func hashPassword(password string) ([]byte, error) {
	cost := productionBcryptCost
	if testing.Testing() {
		cost = bcrypt.MinCost
	}
	return bcrypt.GenerateFromPassword([]byte(password), cost)
}
