# Go Style and Tests

Repository-wide Go implementation and unit-test guidance. This document is durable engineering documentation; agent skills reference it but do not own it.

Authority order:

1. Existing style in the package being changed.
2. `AGENTS.md` and [Go Patterns](go-patterns.md).
3. This document.

## Code style

Clarity over cleverness; match the package before applying a general preference. Interfaces only at a real boundary, owned by the consumer. No `init()`, no shadowed built-ins, `time.Duration` for periods.

```go
// Interface as a value, declared where it is consumed.
type Renderer interface {
	Render(ctx context.Context, html []byte) ([]byte, error)
}

func NewService(r Renderer) *Service { ... } // not *Renderer

// Copy mutable internal state before exposing it.
func (e *Exam) Tags() []string {
	return slices.Clone(e.tags)
}

// Explicit tags on anything persisted or transported.
type PresignedUploadURL struct {
	URL    string            `json:"url"`
	Method string            `json:"method"`
	Fields map[string]string `json:"fields"`
	Key    string            `json:"key"`
}

// Release at the acquisition boundary.
obj, err := s.storage.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
if err != nil {
	return fmt.Errorf("get %s: %w", key, err)
}
defer obj.Close()
```

## Errors and failure behavior

Sentinels for `errors.Is`, typed errors for `errors.As`. Wrap with `%w`, handle once, never panic in request/worker/repository/service paths. Every new sentinel gets a `mapServiceError` case.

```go
// service/errors.go
var ErrSchoolDeactivated = errors.New("school is deactivated")

// service: wrap, don't log-and-return.
if school.Status != "active" {
	return nil, ErrSchoolDeactivated
}
if err := s.storeRepo.CreateStudent(ctx, st); err != nil {
	return nil, fmt.Errorf("create student: %w", err)
}

// handler/errors.go: map once at the boundary.
case errors.Is(err, service.ErrSchoolDeactivated):
	status, apiErr = http.StatusConflict, APIError{Code: "school_deactivated", Message: err.Error()}

// Comma-ok assertions only.
claims, ok := c.Get("claims").(*infra.Claims)
if !ok {
	return echo.ErrUnauthorized
}
```

```go
// Don't
log.Error("register failed", "err", err)
return err                                   // logged twice upstream
if strings.Contains(err.Error(), "deactivated") { ... } // branch on string
```

## Unit-test style

Test behavior, not implementation steps. Table-driven when one behavior has several cases; one focused case needs no table. Exercise the real service method — no shims. Mock only real external boundaries.

```go
func TestMatchesAnyAccepted_pinsTheConfirmedRule(t *testing.T) {
	tests := []struct {
		name      string
		submitted string
		accepted  []string
		want      bool
	}{
		{"trim + lowercase matches", " DUA ", []string{"dua"}, true},
		{"accent is not folded", "dúa", []string{"dua"}, false},
		{"empty submitted never matches", "", []string{"dua"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAnyAccepted(tt.submitted, tt.accepted)
			if got != tt.want {
				t.Errorf("matchesAnyAccepted(%q, %v) = %v, want %v", tt.submitted, tt.accepted, got, tt.want)
			}
		})
	}
}
```

```go
// Assert the contract and the persisted side effect, not just "no error".
err := svc.RegisterStudent(ctx, req)
if !errors.Is(err, service.ErrSchoolDeactivated) {
	t.Fatalf("RegisterStudent() err = %v, want ErrSchoolDeactivated", err)
}
if n := countStudents(t, db, schoolID); n != 0 {
	t.Fatalf("students persisted = %d, want 0", n)
}

// Pass time in instead of asserting around time.Now().
func isExamOpen(e Exam, now time.Time) bool { ... }

got := isExamOpen(exam, time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC))
```

Also:

- `t.Parallel()` only when fixtures, env, DB rows and globals are isolated.
- Repository / state-transition behavior: use the existing Postgres testcontainer pattern and check persisted rows.
- Migration-dependent behavior: paired up/down migrations plus integration coverage.
- External providers: test against a captured real response fixture; if none exists, mark `TODO: uncertain` and say so in the PR.

## Red-proof quality

A valid red test compiles, reaches the intended assertion, and fails because the behavior is missing or wrong — not because of setup, missing infrastructure, or a broken mock. It passes after the smallest correct implementation and stays meaningful if internals change.

```text
--- FAIL: TestRegisterStudent/deactivated_school_is_rejected
    student_test.go:88: RegisterStudent() err = <nil>, want ErrSchoolDeactivated   ✅ behavior red
--- FAIL: TestRegisterStudent/deactivated_school_is_rejected
    student_test.go:41: seed school: connection refused                          ❌ setup red
```

Before reporting green, run the focused test, the relevant package suite, and the broader commands required by `.agents/verification.md`.
