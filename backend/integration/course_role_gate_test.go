package integration_test

import (
	"context"
	"testing"

	"akademi-bimbel/internal/service"

	"github.com/stretchr/testify/require"
)

// TestListCourses_RoleGate_RealService drives the real Service (not the
// shimCourseService in internal/service/course_test.go, which answers the
// role question itself and would stay green even if this guard were deleted).
func TestListCourses_RoleGate_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	forbiddenRoles := []string{service.RoleStudent, service.RoleAdminSchool}
	for _, role := range forbiddenRoles {
		_, _, err := env.svc.ListCourses(ctx, 20, "", role)
		require.ErrorIs(t, err, service.ErrForbidden, "role %s should be forbidden", role)
	}

	allowedRoles := []string{service.RoleAdminExam, service.RoleAdminStore, service.RoleSuperAdmin}
	for _, role := range allowedRoles {
		_, _, err := env.svc.ListCourses(ctx, 20, "", role)
		require.NoError(t, err, "role %s should be allowed", role)
	}
}

// TestGetCourse_RoleGate_RealService is the GetCourse half of C-1's three-handler fix.
func TestGetCourse_RoleGate_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	courseID := seedCourse(t, env, "Math")

	forbiddenRoles := []string{service.RoleStudent, service.RoleAdminSchool}
	for _, role := range forbiddenRoles {
		_, _, _, err := env.svc.GetCourse(ctx, courseID, role)
		require.ErrorIs(t, err, service.ErrForbidden, "role %s should be forbidden", role)
	}

	allowedRoles := []string{service.RoleAdminExam, service.RoleAdminStore, service.RoleSuperAdmin}
	for _, role := range allowedRoles {
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

	forbiddenRoles := []string{service.RoleStudent, service.RoleAdminSchool}
	for _, role := range forbiddenRoles {
		_, err := env.svc.ListSections(ctx, courseID, role)
		require.ErrorIs(t, err, service.ErrForbidden, "role %s should be forbidden", role)
	}

	allowedRoles := []string{service.RoleAdminExam, service.RoleAdminStore, service.RoleSuperAdmin}
	for _, role := range allowedRoles {
		_, err := env.svc.ListSections(ctx, courseID, role)
		require.NoError(t, err, "role %s should be allowed", role)
	}
}
