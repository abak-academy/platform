package repository

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

// Compile-time check: *Repository must implement all user methods.
var _ interface {
	CreateUser(context.Context, *model.User) error
	GetUserByEmail(context.Context, string) (*model.User, error)
	GetUserByID(context.Context, string) (*model.User, error)
	UpdatePasswordHash(context.Context, string, string) error
	TombstoneUser(context.Context, string) error
	ListSchools(context.Context) ([]*model.School, error)
} = (*Repository)(nil)

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"user@example.com", "user@example.com"},
		{"User@Example.COM", "user@example.com"},
		{"ADMIN@AMARTHA.COM", "admin@amartha.com"},
		{"  User@Example.COM  ", "user@example.com"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		got := normalizeEmail(c.in)
		if got != c.want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeOptionalEmail(t *testing.T) {
	empty := ""
	ws := "  \t"
	real := "  User@Example.COM "
	if got := normalizeOptionalEmail(nil); got != nil {
		t.Errorf("nil: want nil, got %v", got)
	}
	if got := normalizeOptionalEmail(&empty); got != nil {
		t.Errorf("empty: want nil, got %v", got)
	}
	if got := normalizeOptionalEmail(&ws); got != nil {
		t.Errorf("whitespace: want nil, got %v", got)
	}
	got := normalizeOptionalEmail(&real)
	if got == nil || *got != "user@example.com" {
		t.Errorf("real: want user@example.com, got %v", got)
	}
}

// TestListSchools verifies that ListSchools returns id, name, code, AND
// school_types for every active school, ordered by name.
func TestListSchools(t *testing.T) {
	pool := newPristineTestPool(t)
	repo := New(pool)
	ctx := context.Background()

	// Clear seeded school data so the test is deterministic.
	_, err := pool.Exec(ctx, `DELETE FROM school`)
	if err != nil {
		t.Fatalf("clear school table: %v", err)
	}

	// Insert school A with specific school_types.
	var schoolA string
	err = pool.QueryRow(ctx,
		`INSERT INTO school (name, code, school_types) VALUES ($1, $2, $3) RETURNING id`,
		"Zebra Academy", "ZBR", []string{"sma", "smp"},
	).Scan(&schoolA)
	if err != nil {
		t.Fatalf("insert school A: %v", err)
	}
	t.Logf("schoolA id: %s", schoolA)

	// Insert school B with different school_types.
	var schoolB string
	err = pool.QueryRow(ctx,
		`INSERT INTO school (name, code, school_types) VALUES ($1, $2, $3) RETURNING id`,
		"Alpha School", "ALP", []string{"sd"},
	).Scan(&schoolB)
	if err != nil {
		t.Fatalf("insert school B: %v", err)
	}

	// Insert school C (deactivated) — should NOT appear.
	err = pool.QueryRow(ctx,
		`INSERT INTO school (name, code, school_types, status) VALUES ($1, $2, $3, $4) RETURNING id`,
		"Inactive Academy", "INA", []string{"sma"}, "deactivated",
	).Scan(new(uuid.UUID))
	if err != nil {
		t.Fatalf("insert school C: %v", err)
	}

	schools, err := repo.ListSchools(ctx)
	if err != nil {
		t.Fatalf("ListSchools: %v", err)
	}

	if len(schools) != 2 {
		t.Fatalf("expected 2 active schools, got %d", len(schools))
	}

	// First school alphabetically: Alpha School (school B).
	s0 := schools[0]
	if s0.Name != "Alpha School" {
		t.Errorf("expected first school 'Alpha School', got %q", s0.Name)
	}
	if s0.Code != "ALP" {
		t.Errorf("expected code 'ALP', got %q", s0.Code)
	}
	if len(s0.SchoolTypes) != 1 || s0.SchoolTypes[0] != "sd" {
		t.Errorf("expected school_types [sd], got %v", s0.SchoolTypes)
	}

	// Second school alphabetically: Zebra Academy (school A).
	s1 := schools[1]
	if s1.Name != "Zebra Academy" {
		t.Errorf("expected second school 'Zebra Academy', got %q", s1.Name)
	}
	if s1.Code != "ZBR" {
		t.Errorf("expected code 'ZBR', got %q", s1.Code)
	}
	if len(s1.SchoolTypes) != 2 || s1.SchoolTypes[0] != "sma" || s1.SchoolTypes[1] != "smp" {
		t.Errorf("expected school_types [sma smp], got %v", s1.SchoolTypes)
	}
}
