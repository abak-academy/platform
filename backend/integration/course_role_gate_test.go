package integration_test

import (
	"context"
	"testing"

	"akademi-bimbel/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The 22 course tests in internal/service/course_test.go run against shimService,
// which answers the role question itself — deleting canAuthorCourses from a
// production method leaves them all green. This one exercises the REAL service
// against Postgres, so the gate on CreateSection has to exist to pass.
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
