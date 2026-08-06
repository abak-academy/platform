package service

import "testing"

func TestCapabilities_knownRoles(t *testing.T) {
	cases := []struct {
		role string
		want int // minimum expected length; 0 means empty
	}{
		{RoleStudent, 0},
		{RoleAdminExam, 5},
		{RoleAdminStore, 6},
		{RoleAdminSchool, 3},
		{RoleSuperAdmin, 1},
	}
	for _, tc := range cases {
		caps := Capabilities(tc.role)
		if tc.want == 0 && len(caps) != 0 {
			t.Errorf("Capabilities(%q): want empty slice, got %v", tc.role, caps)
		}
		if tc.want > 0 && len(caps) < tc.want {
			t.Errorf("Capabilities(%q): want at least %d entries, got %d", tc.role, tc.want, len(caps))
		}
	}
}

func TestCapabilities_unknownRole(t *testing.T) {
	caps := Capabilities("ghost")
	if len(caps) != 0 {
		t.Errorf("Capabilities(unknown): want empty slice, got %v", caps)
	}
}

func TestHasCapability_superAdminMatchesAll(t *testing.T) {
	required := []string{"questions:create", "orders:delete", "students:read", "anything:write", "*"}
	for _, r := range required {
		if !HasCapability(RoleSuperAdmin, r) {
			t.Errorf("HasCapability(super_admin, %q): want true", r)
		}
	}
}

func TestHasCapability_adminExamWildcard(t *testing.T) {
	// admin_exam has "questions:*" — should match any questions: action
	if !HasCapability(RoleAdminExam, "questions:create") {
		t.Error("HasCapability(admin_exam, questions:create): want true")
	}
	if !HasCapability(RoleAdminExam, "questions:delete") {
		t.Error("HasCapability(admin_exam, questions:delete): want true")
	}
	// but NOT orders:read
	if HasCapability(RoleAdminExam, "orders:read") {
		t.Error("HasCapability(admin_exam, orders:read): want false")
	}
}

func TestHasCapability_adminStoreWildcard(t *testing.T) {
	if !HasCapability(RoleAdminStore, "orders:read") {
		t.Error("HasCapability(admin_store, orders:read): want true")
	}
	if !HasCapability(RoleAdminStore, "orders:delete") {
		t.Error("HasCapability(admin_store, orders:delete): want true")
	}
	// admin_store cannot touch questions
	if HasCapability(RoleAdminStore, "questions:create") {
		t.Error("HasCapability(admin_store, questions:create): want false")
	}
}

func TestHasCapability_adminSchool(t *testing.T) {
	if !HasCapability(RoleAdminSchool, "students:create") {
		t.Error("HasCapability(admin_school, students:create): want true")
	}
	if HasCapability(RoleAdminSchool, "questions:create") {
		t.Error("HasCapability(admin_school, questions:create): want false")
	}
}

func TestHasCapability_adminSchoolExamReadOnly(t *testing.T) {
	if !HasCapability(RoleAdminSchool, "products(exam):read") {
		t.Error("HasCapability(admin_school, products(exam):read): want true — needed for the Registrations tab")
	}
	if HasCapability(RoleAdminSchool, "products(exam):write") {
		t.Error("HasCapability(admin_school, products(exam):write): want false — admin_school must not manage exam content")
	}
}

func TestHasCapability_studentSatisfiesNothing(t *testing.T) {
	caps := []string{"questions:read", "orders:read", "students:read", "*"}
	for _, c := range caps {
		if HasCapability(RoleStudent, c) {
			t.Errorf("HasCapability(student, %q): want false", c)
		}
	}
}

func TestHasCapability_unknownRoleSatisfiesNothing(t *testing.T) {
	if HasCapability("ghost", "questions:read") {
		t.Error("HasCapability(unknown, questions:read): want false")
	}
}

func TestHasCapability_SuperAdminSchoolsWrite(t *testing.T) {
	if !HasCapability(RoleSuperAdmin, "schools:write") {
		t.Error("HasCapability(super_admin, schools:write): want true")
	}
}

func TestCapabilities_SuperAdminCount(t *testing.T) {
	caps := Capabilities(RoleSuperAdmin)
	if len(caps) < 2 {
		t.Errorf("Capabilities(super_admin): want at least 2 capabilities (has * and schools:write), got %d", len(caps))
	}
}

// admin_store fulfils orders but must not read revenue aggregates. Per-order
// amounts stay visible elsewhere — confirming a bank transfer means matching
// the receipt against the figure — so the restriction is on aggregates only.
func TestHasCapability_adminStoreCannotReadRevenue(t *testing.T) {
	if HasCapability(RoleAdminStore, "revenue:read") {
		t.Error("HasCapability(admin_store, revenue:read): want false")
	}
	if !HasCapability(RoleSuperAdmin, "revenue:read") {
		t.Error("HasCapability(super_admin, revenue:read): want true")
	}
	if !HasCapability(RoleAdminStore, "orders:write") {
		t.Error("HasCapability(admin_store, orders:write): want true — fulfilment is untouched")
	}
}

// TestAdminExamCapabilities pins the admin_exam capability list. Its TypeScript
// counterpart is web/lib/hooks/use-capability.test.ts, which asserts the same
// literal strings. The two suites share no definition — this only makes a
// divergence between the two languages fail loudly.
func TestAdminExamCapabilities(t *testing.T) {
	want := []string{
		"questions:*",
		"tests:*",
		"products(exam):*",
		"products(course):*",
		"sections:*",
		"sessions:*",
		"uploads:write",
	}
	got := Capabilities(RoleAdminExam)
	if len(got) != len(want) {
		t.Fatalf("admin_exam caps = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cap[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAdminExamRetainsExamReadCapability guards the collapse that looks tidy and
// is not: HasCapability matches exact strings and ":*" prefixes only, so a
// merged "products(exam|course):*" entry would stop matching this and lock the
// exam admin out of the /admin/exams read group (routes.go:293).
func TestAdminExamRetainsExamReadCapability(t *testing.T) {
	for _, required := range []string{"products(exam):read", "products(exam):write", "products(course):write"} {
		if !HasCapability(RoleAdminExam, required) {
			t.Errorf("HasCapability(admin_exam, %q) = false, want true", required)
		}
	}
	for _, required := range []string{"orders:write", "revenue:read", "schools:write"} {
		if HasCapability(RoleAdminExam, required) {
			t.Errorf("HasCapability(admin_exam, %q) = true, want false", required)
		}
	}
}
