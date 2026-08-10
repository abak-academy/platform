package integration_test

import (
	"context"
	"testing"

	"akademi-bimbel/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The course tests in internal/service/course_test.go run against shimService,
// which answers the role question itself — deleting canAuthorCourses from a
// production method leaves them all green. Everything here exercises the REAL
// service against Postgres, so the gates have to exist to pass.

// courseAuthorRoles mirrors canAuthorCourses. Reads and writes share one
// boundary, so both halves of this file assert against the same two lists —
// that is what stops them drifting apart again.
var (
	courseAuthorRoles    = []string{service.RoleAdminExam, service.RoleAdminStore, service.RoleSuperAdmin}
	courseForbiddenRoles = []string{service.RoleStudent, service.RoleAdminSchool}
)

func TestCreateSection_RoleGate_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	course, err := env.svc.CreateCourse(ctx, "Persiapan UTBK", "SMA", "Matematika", "Sari", service.RoleAdminExam)
	require.NoError(t, err)

	_, err = env.svc.CreateSection(ctx, course.ID.String(), "Bab 1", service.RoleAdminSchool)
	require.ErrorIs(t, err, service.ErrForbidden, "admin_school must not author course sections")

	sec, err := env.svc.CreateSection(ctx, course.ID.String(), "Bab 1", service.RoleAdminExam)
	require.NoError(t, err, "admin_exam must be able to author course sections")
	assert.Equal(t, "Bab 1", sec.Title)

	var count int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM section WHERE course_id = $1`, course.ID).Scan(&count))
	assert.Equal(t, 1, count, "exactly one section persisted — the forbidden call must write nothing")
}

// TestListCourses_RoleGate_RealService covers the first of the three reads that
// were mounted under /admin/courses with no role argument at all.
func TestListCourses_RoleGate_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	for _, role := range courseForbiddenRoles {
		_, _, err := env.svc.ListCourses(ctx, 20, "", role)
		require.ErrorIs(t, err, service.ErrForbidden, "role %s should be forbidden", role)
	}

	for _, role := range courseAuthorRoles {
		_, _, err := env.svc.ListCourses(ctx, 20, "", role)
		require.NoError(t, err, "role %s should be allowed", role)
	}
}

// TestGetCourse_RoleGate_RealService is the GetCourse half of C-1's three-handler fix.
func TestGetCourse_RoleGate_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	courseID := seedCourse(t, env, "Math")

	for _, role := range courseForbiddenRoles {
		_, _, _, err := env.svc.GetCourse(ctx, courseID, role)
		require.ErrorIs(t, err, service.ErrForbidden, "role %s should be forbidden", role)
	}

	for _, role := range courseAuthorRoles {
		_, _, _, err := env.svc.GetCourse(ctx, courseID, role)
		require.NoError(t, err, "role %s should be allowed", role)
	}
}

// TestListSections_RoleGate_RealService is the third handler named in spec C-1:
// GET /admin/courses/:id/sections was left unguarded alongside the other two.
func TestListSections_RoleGate_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	courseID := seedCourse(t, env, "Math")
	seedSection(t, env, courseID)

	for _, role := range courseForbiddenRoles {
		_, err := env.svc.ListSections(ctx, courseID, role)
		require.ErrorIs(t, err, service.ErrForbidden, "role %s should be forbidden", role)
	}

	for _, role := range courseAuthorRoles {
		_, err := env.svc.ListSections(ctx, courseID, role)
		require.NoError(t, err, "role %s should be allowed", role)
	}
}

// TestCourseReadWriteBoundariesMatch is the regression guard for the defect this
// merge resolved: PR #83 widened authoring to admin_exam while PR #85 gated reads
// to a separate, narrower predicate. Both suites were green in isolation. Asserting
// the two boundaries agree is what makes a future divergence fail loudly.
func TestCourseReadWriteBoundariesMatch(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	courseID := seedCourse(t, env, "Math")

	for _, role := range append(append([]string{}, courseAuthorRoles...), courseForbiddenRoles...) {
		_, _, _, readErr := env.svc.GetCourse(ctx, courseID, role)
		_, writeErr := env.svc.CreateSection(ctx, courseID, "Bab", role)

		readForbidden := readErr != nil
		writeForbidden := writeErr != nil
		assert.Equal(t, writeForbidden, readForbidden,
			"role %s: read and write boundaries disagree (read err=%v, write err=%v)", role, readErr, writeErr)
	}
}
