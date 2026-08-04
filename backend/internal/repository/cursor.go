package repository

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Order cursors carry both sort keys. created_at alone is not unique — orders
// placed in the same instant would be skipped or repeated — and id alone has no
// relationship to the sort order at all, which is what the previous
// implementation got wrong: it filtered on id while ordering by created_at.
// "<rfc3339nano>,<uuid>" — the same shape ListAdminResults already uses, so the
// two composite cursors in this repository stay one convention.
func EncodeOrderCursor(createdAt time.Time, id uuid.UUID) string {
	return createdAt.UTC().Format(time.RFC3339Nano) + "," + id.String()
}

func DecodeOrderCursor(s string) (time.Time, uuid.UUID, error) {
	timeStr, idStr, found := strings.Cut(s, ",")
	if !found {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	at, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return at, id, nil
}
