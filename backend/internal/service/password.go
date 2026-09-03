package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// 10 is bcrypt's own default and the deployed floor for generated and operator-set passwords.
const productionBcryptCost = 10

// A cost-12 hash takes ~2.5s under -race, which put the auth tests on the CI critical path.
func hashPassword(password string) ([]byte, error) {
	cost := productionBcryptCost
	if testing.Testing() {
		cost = bcrypt.MinCost
	}
	return bcrypt.GenerateFromPassword([]byte(password), cost)
}
