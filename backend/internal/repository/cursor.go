package repository

import (
	"strconv"
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

func EncodePaidOrderCursor(at *time.Time, id uuid.UUID) string {
	if at == nil {
		return "null," + id.String()
	}
	return EncodeOrderCursor(*at, id)
}

func DecodePaidOrderCursor(s string) (*time.Time, uuid.UUID, error) {
	key, idStr, found := strings.Cut(s, ",")
	if found && key == "null" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, uuid.Nil, ErrInvalidCursor
		}
		return nil, id, nil
	}
	at, id, err := DecodeOrderCursor(s)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return &at, id, nil
}

func EncodeItemQtyOrderCursor(qty int64, id uuid.UUID) string {
	return strconv.FormatInt(qty, 10) + "," + id.String()
}

func DecodeItemQtyOrderCursor(s string) (int64, uuid.UUID, error) {
	key, idStr, found := strings.Cut(s, ",")
	if !found {
		return 0, uuid.Nil, ErrInvalidCursor
	}
	qty, err := strconv.ParseInt(key, 10, 64)
	if err != nil || qty < 0 {
		return 0, uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return 0, uuid.Nil, ErrInvalidCursor
	}
	return qty, id, nil
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
