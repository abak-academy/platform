package handler_test

import (
	"os"
	"testing"
)

// The DB-backed envs in this package share one Postgres container
// (sharedpg_test.go); each holds a pool on its own database cloned from the
// template. Close the pools and fake-redis instances each env owns, then
// terminate the single container — without that it leaks after the run.
func TestMain(m *testing.M) {
	code := m.Run()

	if adminStuDBEnv != nil {
		closeEnv(adminStuDBEnv.pool, adminStuDBEnv.rdb, adminStuDBEnv.mr)
	}
	if adminResultsDBEnv != nil {
		closeEnv(adminResultsDBEnv.pool, adminResultsDBEnv.rdb, adminResultsDBEnv.mr)
	}
	if adminProductDBEnv != nil {
		closeEnv(adminProductDBEnv.pool, adminProductDBEnv.rdb, adminProductDBEnv.mr)
	}
	if regionDBEnv != nil {
		closeEnv(regionDBEnv.pool, regionDBEnv.rdb, regionDBEnv.mr)
	}
	if searchGrantDBEnv != nil {
		closeEnv(searchGrantDBEnv.pool, searchGrantDBEnv.rdb, searchGrantDBEnv.mr)
	}

	terminateSharedPG()

	os.Exit(code)
}
