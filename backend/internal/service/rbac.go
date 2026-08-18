package service

import "strings"

const (
	RoleStudent     = "student"
	RoleAdminSchool = "admin_school"
	RoleAdminExam   = "admin_exam"
	RoleAdminStore  = "admin_store"
	RoleSuperAdmin  = "super_admin"
)

var roleCapabilities = map[string][]string{
	RoleStudent:     {},
	RoleAdminExam:   {"questions:*", "tests:*", "products(exam):*", "products(course):*", "sections:*", "sessions:*", "uploads:write", "results:read"},
	// No revenue:read — store admins fulfil orders; revenue reporting is
	// super_admin's. Per-order totals stay visible to them on the orders page,
	// because confirming a manual transfer means matching the receipt to the
	// amount. The restriction is on aggregates.
	RoleAdminStore:  {"products(book|course):write", "sections:*", "orders:*", "promos:*", "notifications:*", "uploads:write"},
	RoleAdminSchool: {"students:*", "results:read", "bulk-exam-orders:write", "products(exam):read"},
	RoleSuperAdmin:  {"*", "schools:write"},
}

func Capabilities(role string) []string {
	caps, ok := roleCapabilities[role]
	if !ok {
		return []string{}
	}
	return caps
}

func HasCapability(role, required string) bool {
	caps, ok := roleCapabilities[role]
	if !ok {
		return false
	}
	for _, cap := range caps {
		if cap == "*" {
			return true
		}
		if cap == required {
			return true
		}
		if strings.HasSuffix(cap, ":*") {
			prefix := strings.TrimSuffix(cap, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}
