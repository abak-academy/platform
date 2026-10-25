package repository

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/model"
)

func (r *Repository) CreateAnnouncement(ctx context.Context, a *model.Announcement) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO announcement (
			title, message, type, recipients, status, scheduled_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING id, created_at, updated_at`,
		a.Title, a.Message, a.Type, a.Recipients, a.Status, a.ScheduledAt, a.CreatedBy,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *Repository) GetAnnouncementByID(ctx context.Context, id string) (*model.Announcement, error) {
	a := &model.Announcement{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, message, type, recipients, status,
			scheduled_at, sent_at, recipient_count, created_by,
			created_at, updated_at
		FROM announcement
		WHERE id = $1`,
		id,
	).Scan(
		&a.ID, &a.Title, &a.Message, &a.Type, &a.Recipients, &a.Status,
		&a.ScheduledAt, &a.SentAt, &a.RecipientCount, &a.CreatedBy,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (r *Repository) ListAnnouncements(ctx context.Context) ([]model.Announcement, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, message, type, recipients, status,
			scheduled_at, sent_at, recipient_count, created_by,
			created_at, updated_at
		FROM announcement
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	announcements := []model.Announcement{}
	for rows.Next() {
		var a model.Announcement
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Message, &a.Type, &a.Recipients, &a.Status,
			&a.ScheduledAt, &a.SentAt, &a.RecipientCount, &a.CreatedBy,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		announcements = append(announcements, a)
	}
	return announcements, rows.Err()
}

func (r *Repository) UpdateAnnouncement(ctx context.Context, id string, a *model.Announcement) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE announcement
		SET title = $1, message = $2, type = $3, recipients = $4,
		    scheduled_at = $5, updated_at = now()
		WHERE id = $6`,
		a.Title, a.Message, a.Type, a.Recipients, a.ScheduledAt, id,
	)
	return err
}

func (r *Repository) DeleteAnnouncement(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM announcement WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) ClaimDueAnnouncements(ctx context.Context, now time.Time, limit int) ([]model.Announcement, error) {
	// SELECT ... FOR UPDATE SKIP LOCKED claims rows without marking them sent.
	// MarkAnnouncementSent is called by the service layer only after a successful
	// send, so a transient failure leaves the row as 'scheduled' for retry.
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, message, type, recipients, status,
			scheduled_at, sent_at, recipient_count, created_by,
			created_at, updated_at
		FROM announcement
		WHERE status = 'scheduled' AND scheduled_at <= $1
		ORDER BY scheduled_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED`,
		now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	announcements := []model.Announcement{}
	for rows.Next() {
		var a model.Announcement
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Message, &a.Type, &a.Recipients, &a.Status,
			&a.ScheduledAt, &a.SentAt, &a.RecipientCount, &a.CreatedBy,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		announcements = append(announcements, a)
	}
	return announcements, rows.Err()
}

func (r *Repository) MarkAnnouncementSent(ctx context.Context, id string, sentAt time.Time, recipientCount int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE announcement
		SET status = 'sent', sent_at = $1, recipient_count = $2, updated_at = now()
		WHERE id = $3`,
		sentAt, recipientCount, id,
	)
	return err
}

// ScheduleAnnouncementNow flips a draft/scheduled announcement back to
// 'scheduled' with the given scheduled_at, clearing any stale sent metadata,
// so the worker's paced loop dispatches it on the next tick.
func (r *Repository) ScheduleAnnouncementNow(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE announcement
		SET status = 'scheduled', scheduled_at = $1, sent_at = NULL, recipient_count = NULL, updated_at = now()
		WHERE id = $2`,
		at, id,
	)
	return err
}

func (r *Repository) ListActiveUserEmails(ctx context.Context, recipients string) ([]string, error) {
	query := `SELECT email FROM users
		WHERE status = 'active'
		AND email IS NOT NULL AND email != ''`

	switch recipients {
	case "students":
		query += " AND role = 'student'"
	case "admins":
		query += " AND role LIKE 'admin_%'"
	case "affiliators":
		// Active affiliator accounts only: the user row alone is not enough.
		query += " AND role = 'affiliator' AND EXISTS (SELECT 1 FROM affiliators a WHERE a.user_id = users.id AND a.status = 'active')"
	case "all":
		// no additional filter
	default:
		return nil, fmt.Errorf("unknown recipient group: %s", recipients)
	}

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}
	return emails, rows.Err()
}
