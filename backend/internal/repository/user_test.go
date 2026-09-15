package repository

import (
	"context"
	"testing"

	"akademi-bimbel/internal/model"
)

// Compile-time check: *Repository must implement all user methods.
var _ interface {
	CreateUser(context.Context, *model.User) error
	GetUserByEmail(context.Context, string) (*model.User, error)
	GetUserByID(context.Context, string) (*model.User, error)
	UpdatePasswordHash(context.Context, string, string) error
	TombstoneUser(context.Context, string) error
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
