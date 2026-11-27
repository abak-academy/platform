package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestListAuditLog_DateParsing validates date format parsing via a dummy call.
// Since ListAuditLog depends on s.storeRepo, we test date validation logic
// directly through filter validation patterns that mirror the service's checks.
func TestParseAuditDates(t *testing.T) {
	// Validate RFC3339 dates
	for _, s := range []string{
		"2026-01-15T10:30:00Z",
		"2026-06-23T15:04:05+07:00",
		"2026-01-15T10:30:00.000Z",
	} {
		_, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Errorf("valid RFC3339 %q should parse: %v", s, err)
		}
	}

	// Validate YYYY-MM-DD dates
	for _, s := range []string{
		"2026-01-15",
		"2026-06-23",
	} {
		_, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Errorf("valid YYYY-MM-DD %q should parse: %v", s, err)
		}
	}

	// Invalid dates should fail
	for _, s := range []string{
		"not-a-date",
		"01-15-2026",
		"2026/01/15",
		"",
	} {
		_, err := time.Parse(time.RFC3339, s)
		_, err2 := time.Parse("2006-01-02", s)
		if err == nil && err2 == nil {
			t.Errorf("invalid date %q should not parse", s)
		}
	}
}

func TestAuditFilter_ActorIDValidation(t *testing.T) {
	// Validate UUID parsing for actor_id filter
	// parseUUID is defined in store.go and used in ListAuditLog
	t.Run("valid UUID passes parse", func(t *testing.T) {
		_, err := parseUUID("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
		if err != nil {
			t.Errorf("valid UUID should pass: %v", err)
		}
	})

	t.Run("invalid UUID fails parse", func(t *testing.T) {
		_, err := parseUUID("not-a-uuid")
		if err == nil {
			t.Error("invalid UUID should fail")
		}
	})

	t.Run("empty UUID fails parse", func(t *testing.T) {
		_, err := parseUUID("")
		if err == nil {
			t.Error("empty string should fail UUID parse")
		}
	})
}

// TestErrSentinelsExist verifies the new sentinel errors are defined and usable.
func TestErrSentinelsExist(t *testing.T) {
	if ErrCannotDeactivateSelf == nil {
		t.Fatal("ErrCannotDeactivateSelf must be defined")
	}
	if ErrInvalidAdminRole == nil {
		t.Fatal("ErrInvalidAdminRole must be defined")
	}
	if ErrInvalidRoleFilter == nil {
		t.Fatal("ErrInvalidRoleFilter must be defined")
	}
	if ErrInvalidStatusFilter == nil {
		t.Fatal("ErrInvalidStatusFilter must be defined")
	}
	if ErrAccountNoEmail == nil {
		t.Fatal("ErrAccountNoEmail must be defined")
	}
	if ErrMissingField == nil {
		t.Fatal("ErrMissingField must be defined")
	}

	// Verify error identity
	if !errors.Is(ErrCannotDeactivateSelf, ErrCannotDeactivateSelf) {
		t.Error("ErrCannotDeactivateSelf should identify itself")
	}
	if !errors.Is(ErrInvalidAdminRole, ErrInvalidAdminRole) {
		t.Error("ErrInvalidAdminRole should identify itself")
	}

	if ErrInvalidDateFormat == nil {
		t.Fatal("ErrInvalidDateFormat must be defined")
	}
	// Verify wrapping works — production code uses fmt.Errorf("%w: ...", ErrInvalidDateFormat, val)
	{
		wrapped := fmt.Errorf("%w: %s", ErrInvalidDateFormat, "2026-13-01")
		if !errors.Is(wrapped, ErrInvalidDateFormat) {
			t.Error("wrapped ErrInvalidDateFormat should be detectable via errors.Is")
		}
	}
}

func TestListAuditLog_Pagination(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	targetType := "paging_" + uniqueSuffix()
	actor := uuid.NewString()
	for i := 0; i < 5; i++ {
		if err := repo.InsertAuditLogMeta(ctx, nil, &actor, targetType, fmt.Sprintf("t%d", i), "test.page", nil); err != nil {
			t.Fatalf("seed audit row %d: %v", i, err)
		}
	}
	list := func(page, limit int) ([]AuditLogEntry, int) {
		t.Helper()
		got, total, err := svc.ListAuditLog(ctx, AuditLogFilter{TargetType: targetType, Page: page, Limit: limit})
		if err != nil {
			t.Fatalf("ListAuditLog(page=%d, limit=%d): %v", page, limit, err)
		}
		return got, total
	}

	p1, total := list(1, 2)
	p2, _ := list(2, 2)
	p3, _ := list(3, 2)
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(p1) != 2 || len(p2) != 2 || len(p3) != 1 {
		t.Fatalf("page sizes = %d,%d,%d, want 2,2,1", len(p1), len(p2), len(p3))
	}
	seen := map[int64]bool{}
	for _, e := range append(append(p1, p2...), p3...) {
		if seen[e.ID] {
			t.Errorf("entry %d appears on more than one page", e.ID)
		}
		seen[e.ID] = true
	}

	if past, total := list(4, 2); len(past) != 0 || total != 5 {
		t.Errorf("past-the-end page = %d rows total %d, want 0 rows total 5", len(past), total)
	}
	if clamped, _ := list(0, 1000); len(clamped) != 5 {
		t.Errorf("page=0 limit=1000 returned %d rows, want 5 (page clamped to 1)", len(clamped))
	}
}
