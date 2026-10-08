package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"akademi-bimbel/internal/repository"
)

// AuditLogEntry is the response shape for a single audit log entry.
type AuditLogEntry struct {
	ID         int64             `json:"id"`
	ActorID    *string           `json:"actor_id"`
	ActorName  *string           `json:"actor_name"`
	ActorEmail *string           `json:"actor_email"`
	TargetType string            `json:"target_type"`
	TargetID   string            `json:"target_id"`
	Action     string            `json:"action"`
	Metadata   *json.RawMessage  `json:"metadata"`
	CreatedAt  string            `json:"created_at"`
}

// AuditLogFilter carries optional filters for ListAuditLog.
type AuditLogFilter struct {
	ActorID    string
	From       string
	To         string
	TargetType string
	Q          string
	Cursor     string
	Limit      int
}

// ListAuditLog returns audit log entries with optional filters, cursor-paginated.
func (s *Service) ListAuditLog(ctx context.Context, filter AuditLogFilter) ([]AuditLogEntry, string, error) {
	// Validate date formats — accept RFC3339 or YYYY-MM-DD
	if filter.From != "" {
		if _, err := time.Parse(time.RFC3339, filter.From); err != nil {
			if _, err2 := time.Parse("2006-01-02", filter.From); err2 != nil {
				return nil, "", fmt.Errorf("%w: %s", ErrInvalidDateFormat, filter.From)
			}
		}
	}
	if filter.To != "" {
		if _, err := time.Parse(time.RFC3339, filter.To); err != nil {
			if _, err2 := time.Parse("2006-01-02", filter.To); err2 != nil {
				return nil, "", fmt.Errorf("%w: %s", ErrInvalidDateFormat, filter.To)
			}
		}
	}

	// Validate actor_id as UUID if provided
	if filter.ActorID != "" {
		if _, err := parseUUID(filter.ActorID); err != nil {
			return nil, "", ErrInvalidUUID
		}
	}

	rows, nextCursor, err := s.storeRepo.ListAuditLog(ctx, repository.AuditLogFilter{
		ActorID:    filter.ActorID,
		From:       filter.From,
		To:         filter.To,
		TargetType: filter.TargetType,
		Q:          filter.Q,
		Cursor:     filter.Cursor,
		Limit:      filter.Limit,
	})
	if err != nil {
		return nil, "", err
	}

	entries := make([]AuditLogEntry, len(rows))
	for i, r := range rows {
		entries[i] = AuditLogEntry{
			ID:         r.ID,
			ActorID:    r.ActorID,
			ActorName:  r.ActorName,
			ActorEmail: r.ActorEmail,
			TargetType: r.TargetType,
			TargetID:   r.TargetID,
			Action:     r.Action,
			Metadata:   r.Metadata,
			CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		}
	}
	return entries, nextCursor, nil
}
