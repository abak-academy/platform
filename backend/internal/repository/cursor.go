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

// EncodeNameCursor and DecodeNameCursor carry a "<name>,<uuid>" keyset cursor
// for lists ordered by a text column plus id tiebreaker (e.g. ListSchoolsAdmin,
// ORDER BY name ASC, id ASC). Split on the *last* comma rather than the first:
// a school/user-facing name can legitimately contain a comma, but a UUID
// never does, so this is unambiguous without needing to escape the name.
func EncodeNameCursor(name, id string) string {
	return name + "," + id
}

func DecodeNameCursor(s string) (string, uuid.UUID, error) {
	idx := strings.LastIndex(s, ",")
	if idx < 0 {
		return "", uuid.Nil, ErrInvalidCursor
	}
	name, idStr := s[:idx], s[idx+1:]
	id, err := uuid.Parse(idStr)
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	return name, id, nil
}
