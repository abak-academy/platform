# Go Patterns

Target backend structure for new code and scoped cleanup. Existing drift is technical debt, not permission to spread it. Do not refactor adjacent code unless the task explicitly includes that cleanup.

## Layer ownership

Each business rule lives in one layer. Handlers translate HTTP, services decide, repositories persist, adapters own provider wire shapes. The domain lives in `internal/model`.

```go
// handler: parse + validate input, call the service, map the error. No policy.
func (h *Handler) AdminUploadImage(c echo.Context) error {
	claims := claimsFromContext(c)
	filename := c.QueryParam("filename")
	if filename == "" {
		return badRequest(c, "filename is required")
	}
	resp, err := h.svc.GeneratePresignedUploadURL(c.Request().Context(), claims.Sub, "question", filename, c.QueryParam("content_type"))
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// service: the decision.
if school.Status != "active" {
	return nil, ErrSchoolDeactivated
}

// repository: query and persist only.
func (r *Repository) CreateStudent(ctx context.Context, u *model.User) error {
	return r.pool.QueryRow(ctx, `INSERT INTO users (...) VALUES (...) RETURNING id`, ...).Scan(&u.ID)
}
```

```go
// Don't: policy hidden in SQL, where the service can't see or test it.
`SELECT ... FROM schools WHERE id = $1 AND status = 'active'` // "not found" now also means "deactivated"
```

Schema changes ship as paired files: `db/migrations/0068_<name>.up.sql` + `.down.sql`, with integration coverage when behavior depends on them.

Review question: if one business rule changes, is its decision concentrated in one clear layer?

## Error direction

Reusable sentinels live in `service/errors.go`. HTTP mapping happens only in `handler/errors.go`. Provider errors stay wrapped inside `internal/adapter` unless the service must branch on them.

```go
// service/errors.go
var ErrSchoolDeactivated = errors.New("school is deactivated")

// handler/errors.go (the only place a service error becomes a status code)
case errors.Is(err, service.ErrSchoolDeactivated):
	status, apiErr = http.StatusConflict, APIError{Code: "school_deactivated", Message: err.Error()}

// adapter: wrap the provider failure; the service sees an error, not an HTTP body.
if resp.StatusCode >= 300 {
	return fmt.Errorf("fazpass: unexpected status %d", resp.StatusCode)
}
```

```go
// Don't
return c.JSON(http.StatusConflict, ...)                // status decided in the service path
if strings.Contains(err.Error(), "deactivated") { ... } // string match across layers
```

Existing scattered errors are cleanup candidates. New code adds none unless the error is truly local and unshared.

## Model and DTO direction

Stored or domain state lives in `internal/model`. Handler-only request bodies stay unexported in the handler. Shared response shapes get a deliberate name and home. Never create a package for one struct.

```go
// internal/model/school.go: persistent concept
type School struct { ... }

// handler/admin_announcements.go: request body only this handler reads
type createAnnouncementRequest struct {
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	ScheduledAt *time.Time `json:"scheduled_at"`
}

// service/student.go: response contract shared by several upload endpoints and the frontend
type PresignedUploadURL struct {
	URL    string            `json:"url"`
	Method string            `json:"method"`
	Fields map[string]string `json:"fields"`
	Key    string            `json:"key"`
}
```

When a struct becomes reused or starts representing stored business state, move it toward `internal/model` in a scoped cleanup.

## Interface and abstraction restraint

Interfaces only at a real seam: an external provider, or a dependency a unit test must replace. Keep them narrow and owned by the service that consumes them. No abstractions for hypothetical variants.

```go
// service/ports_notify.go: consumer-owned, one method, implemented by adapter.FazpassProvider
type OTPProvider interface {
	SendOTP(ctx context.Context, channel, to, code string) error
}

// service/pdf_generator.go: Gotenberg is the only implementation;
// the seam exists so certificate/card generation is unit-testable.
type pdfGenerator interface {
	RenderHTML(ctx context.Context, html []byte) ([]byte, error)
}
```

```go
// Don't
type SchoolRepository interface { /* 20 methods mirroring *Repository */ } // no second implementation, no test need
package schoolutil                                                         // new package for one helper
```

## Cleanup workflow

1. Name one structural problem: errors, model placement, DTO shape, repository logic, or provider contract.
2. Pin current behavior with tests first.
3. Move structure without changing behavior, in its own commit.
4. Run focused package tests, then the broader gates when the change crosses handler, service, repository, adapter, or migration boundaries.

```text
refactor: move exam status check from repository into service   ← structure only, tests unchanged
fix: reject grading when exam is archived                        ← behavior change, separate commit
```

## Non-goals

- No backend re-architecture during feature work.
- No struct moves or new interfaces for aesthetics.
- No behavior change hidden inside a cleanup.
- No normalizing unrelated legacy code while touching one path.
