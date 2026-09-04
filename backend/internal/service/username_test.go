package service

import (
	"context"
	"errors"
	"testing"
	"unicode/utf8"

	"akademi-bimbel/internal/model"
)

// collisionRepo embeds fakeUserRepo and overrides GetUserByUsername to simulate
// username collisions for testing the retry loop.
type collisionRepo struct {
	fakeUserRepo
	blockUntil int // first blockUntil calls return a taken user
	n          int // call counter
}

func (r *collisionRepo) GetUserByUsername(_ context.Context, _ string) (*model.User, error) {
	r.n++
	if r.n <= r.blockUntil {
		dummy := "taken"
		return &model.User{Username: &dummy}, nil
	}
	return nil, nil
}

// errGetUserByUsernameRepo returns a fixed error from GetUserByUsername.
type errGetUserByUsernameRepo struct {
	fakeUserRepo
	err error
}

func (r *errGetUserByUsernameRepo) GetUserByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, r.err
}

func TestGenerateUsername(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string // expected base (without trailing 4 digits)
		wantRunes int    // expected number of runes
	}{
		{"full name", "Budi Santoso", "budi", 4},
		{"short name", "Ali", "ali", 3},
		{"single word", "Budi", "budi", 4},
		{"mixed case", "Dwi Hartono", "dwih", 4},
		{"leading/trailing spaces", "  Budi  ", "budi", 4},
		{"already lowercase", "budi", "budi", 4},
		{"multiple spaces", "Budi   Santoso", "budi", 4},
		{"very short", "Ai", "ai", 2},
		{"single char", "X", "x", 1},
		{"hyphenated", "Jean-Paul", "jean", 4},
		{"word joiner prefix", "\u2060Budi Santoso", "budi", 4},
		{"zero width space", "Budi\u200bSantoso", "budi", 4},
		{"bom prefix", "\ufeffBudi", "budi", 4},
		{"soft hyphen", "Bu\u00addi", "budi", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateUsername(tt.input)
			if n := utf8.RuneCountInString(got); n != tt.wantRunes {
				t.Errorf("GenerateUsername(%q) = %q, want %d runes, got %d runes", tt.input, got, tt.wantRunes, n)
			}
			if got != tt.want {
				t.Errorf("GenerateUsername(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateUniqueUsername_retriesOnCollision(t *testing.T) {
	// First 3 calls to GetUserByUsername return a user (collision),
	// then the 4th call succeeds. Retry up to 10, so this should work.
	repo := &collisionRepo{fakeUserRepo: *newFakeUserRepo(), blockUntil: 3}
	svc, _ := newTestService(t, repo)

	username, err := svc.generateUniqueUsername(context.Background(), "Budi Santoso")
	if err != nil {
		t.Fatalf("generateUniqueUsername: want nil err, got %v", err)
	}
	if username == "" {
		t.Fatal("generateUniqueUsername: got empty username")
	}
	// The first 4 runes should be "budi"
	runes := []rune(username)
	if len(runes) < 4 {
		t.Fatalf("username %q: too short (len %d)", username, len(runes))
	}
	if base := string(runes[:4]); base != "budi" {
		t.Errorf("username %q: expected base 'budi', got %q", username, base)
	}
	// Last 4 chars should be digits
	suffix := string(runes[4:])
	if len(suffix) != 4 {
		t.Errorf("username %q: expected 4-digit suffix, got %q (len %d)", username, suffix, len(suffix))
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			t.Errorf("username %q: suffix %q contains non-digit %c", username, suffix, c)
		}
	}
	// The retry loop should have been called more than once (3 collisions + 1 success)
	if repo.n <= 1 {
		t.Errorf("GetUserByUsername called %d times, expected > 1 (retries)", repo.n)
	}
}

func TestGenerateUniqueUsername_exhaustedRetries(t *testing.T) {
	// All 10 attempts collide -> ErrUsernameGenerationExhausted
	repo := &collisionRepo{fakeUserRepo: *newFakeUserRepo(), blockUntil: 99}
	svc, _ := newTestService(t, repo)

	_, err := svc.generateUniqueUsername(context.Background(), "Budi Santoso")
	if err == nil {
		t.Fatal("generateUniqueUsername: want ErrUsernameGenerationExhausted, got nil")
	}
	if !errors.Is(err, ErrUsernameGenerationExhausted) {
		t.Fatalf("generateUniqueUsername: want ErrUsernameGenerationExhausted, got %v", err)
	}
	// Should have called GetUserByUsername exactly 10 times
	if repo.n != 10 {
		t.Errorf("GetUserByUsername called %d times, want 10", repo.n)
	}
}

func TestGenerateUniqueUsername_propagatesRepoError(t *testing.T) {
	// If GetUserByUsername returns an error (not nil user), propagate it.
	wantErr := errors.New("db connection failed")
	repo := &errGetUserByUsernameRepo{fakeUserRepo: *newFakeUserRepo(), err: wantErr}
	svc, _ := newTestService(t, repo)

	_, err := svc.generateUniqueUsername(context.Background(), "Budi Santoso")
	if err == nil {
		t.Fatal("generateUniqueUsername: want error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("generateUniqueUsername: want %v, got %v", wantErr, err)
	}
}
