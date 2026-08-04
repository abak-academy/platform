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
func EncodeOrderCursor(createdAt time.Time, id uuid.UUID) string {
	return createdAt.UTC().Format(time.RFC3339Nano) + "_" + id.String()
}

func DecodeOrderCursor(s string) (time.Time, uuid.UUID, error) {
	sep := strings.LastIndex(s, "_")
	if sep <= 0 {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	at, err := time.Parse(time.RFC3339Nano, s[:sep])
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(s[sep+1:])
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return at, id, nil
}
