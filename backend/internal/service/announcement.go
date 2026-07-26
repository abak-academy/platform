package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"akademi-bimbel/internal/model"
)

// AnnounceRepo defines the repository methods the announcement service needs.
type AnnounceRepo interface {
	CreateAnnouncement(ctx context.Context, a *model.Announcement) error
	GetAnnouncementByID(ctx context.Context, id string) (*model.Announcement, error)
	ListAnnouncements(ctx context.Context) ([]model.Announcement, error)
	UpdateAnnouncement(ctx context.Context, id string, a *model.Announcement) error
	DeleteAnnouncement(ctx context.Context, id string) error
	ListActiveUserEmails(ctx context.Context, recipients string) ([]string, error)
	MarkAnnouncementSent(ctx context.Context, id string, sentAt time.Time, recipientCount int) error
	ClaimDueAnnouncements(ctx context.Context, now time.Time, limit int) ([]model.Announcement, error)
}

// validAnnouncementTypes is the set of valid announcement type values.
var validAnnouncementTypes = map[string]bool{
	"announcement": true,
	"promo":        true,
	"exam":         true,
}

// validRecipientGroups is the set of valid recipient group values.
var validRecipientGroups = map[string]bool{
	"all":      true,
	"students": true,
	"admins":   true,
}

// validAnnouncementStatuses is the set of valid announcement status values.
var validAnnouncementStatuses = map[string]bool{
	"draft":     true,
	"scheduled": true,
	"sent":      true,
}

// CreateAnnouncement creates a new announcement after validating enums and
// status-specific rules. For status "scheduled", scheduledAt must be non-nil
// and in the future. For status "sent", the announcement is sent immediately
// (recipients resolved, emails sent, row updated).
func (s *Service) CreateAnnouncement(
	ctx context.Context,
	actorID, title, message, typ, recipients, status string,
	scheduledAt *time.Time,
) (model.Announcement, error) {
	if !validAnnouncementTypes[typ] {
		return model.Announcement{}, fmt.Errorf("%w: invalid announcement type: %q", ErrInvalidAnnouncementField, typ)
	}
	if !validRecipientGroups[recipients] {
		return model.Announcement{}, fmt.Errorf("%w: invalid recipient group: %q", ErrInvalidAnnouncementField, recipients)
	}
	if !validAnnouncementStatuses[status] {
		return model.Announcement{}, fmt.Errorf("%w: invalid announcement status: %q", ErrInvalidAnnouncementField, status)
	}

	if status == "scheduled" {
		if scheduledAt == nil {
			return model.Announcement{}, fmt.Errorf("scheduled_at is required for scheduled announcements")
		}
		if !scheduledAt.After(time.Now()) {
			return model.Announcement{}, fmt.Errorf("scheduled_at must be in the future")
		}
	}

	a := &model.Announcement{
		Title:       title,
		Message:     message,
		Type:        typ,
		Recipients:  recipients,
		Status:      status,
		ScheduledAt: scheduledAt,
		CreatedBy:   actorID,
	}

	if err := s.announceRepo.CreateAnnouncement(ctx, a); err != nil {
		return model.Announcement{}, err
	}

	// For status "sent", trigger immediate send via the same path as manual send.
	if status == "sent" {
		if err := s.sendAnnouncement(ctx, a); err != nil {
			return model.Announcement{}, err
		}
	}

	// Re-fetch to get updated state (e.g. after send-now marking).
	result, err := s.announceRepo.GetAnnouncementByID(ctx, a.ID)
	if err != nil {
		return model.Announcement{}, err
	}
	if result == nil {
		return model.Announcement{}, ErrAnnouncementNotFound
	}
	return *result, nil
}

// UpdateAnnouncement updates a draft or scheduled announcement. Returns
// ErrAnnouncementNotFound if the id doesn't exist, and ErrAnnouncementImmutable
// if the announcement has already been sent.
func (s *Service) UpdateAnnouncement(
	ctx context.Context, id, title, message, typ, recipients string,
	scheduledAt *time.Time,
) (*model.Announcement, error) {
	existing, err := s.announceRepo.GetAnnouncementByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrAnnouncementNotFound
	}
	if existing.Status == "sent" {
		return nil, ErrAnnouncementImmutable
	}

	updates := &model.Announcement{
		Title:       title,
		Message:     message,
		Type:        typ,
		Recipients:  recipients,
		ScheduledAt: scheduledAt,
	}
	if err := s.announceRepo.UpdateAnnouncement(ctx, id, updates); err != nil {
		return nil, err
	}

	result, err := s.announceRepo.GetAnnouncementByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteAnnouncement deletes a draft or scheduled announcement. Returns
// ErrAnnouncementNotFound if the id doesn't exist, and ErrAnnouncementImmutable
// if the announcement has already been sent.
func (s *Service) DeleteAnnouncement(ctx context.Context, id string) error {
	existing, err := s.announceRepo.GetAnnouncementByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrAnnouncementNotFound
	}
	if existing.Status == "sent" {
		return ErrAnnouncementImmutable
	}

	return s.announceRepo.DeleteAnnouncement(ctx, id)
}

// SendAnnouncementNow triggers immediate send of a draft or scheduled
// announcement. Returns ErrAnnouncementNotFound if the id doesn't exist,
// and ErrAnnouncementImmutable if already sent.
func (s *Service) SendAnnouncementNow(ctx context.Context, id string) (model.Announcement, error) {
	existing, err := s.announceRepo.GetAnnouncementByID(ctx, id)
	if err != nil {
		return model.Announcement{}, err
	}
	if existing == nil {
		return model.Announcement{}, ErrAnnouncementNotFound
	}
	if existing.Status == "sent" {
		return model.Announcement{}, ErrAnnouncementImmutable
	}

	if err := s.sendAnnouncement(ctx, existing); err != nil {
		return model.Announcement{}, err
	}

	result, err := s.announceRepo.GetAnnouncementByID(ctx, id)
	if err != nil {
		return model.Announcement{}, err
	}
	return *result, nil
}

// DispatchDueAnnouncements claims and sends all due scheduled announcements.
// One announcement's failure does not stop the rest. Returns the number of
// announcements dispatched.
func (s *Service) DispatchDueAnnouncements(ctx context.Context, limit int) (int, error) {
	announcements, err := s.announceRepo.ClaimDueAnnouncements(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}

	dispatched := 0
	for _, a := range announcements {
		if err := s.sendAnnouncement(ctx, &a); err != nil {
			log.Printf("failed to dispatch announcement %s: %v", a.ID, err)
			continue
		}
		dispatched++
	}
	return dispatched, nil
}

// sendAnnouncement resolves recipients, sends one email per address (best-effort),
// and marks the announcement as sent with the attempted recipient count.
func (s *Service) sendAnnouncement(ctx context.Context, a *model.Announcement) error {
	emails, err := s.announceRepo.ListActiveUserEmails(ctx, a.Recipients)
	if err != nil {
		return fmt.Errorf("resolve recipients: %w", err)
	}

	count := len(emails)
	for _, email := range emails {
		if err := s.emailProvider.SendEmail(ctx, email, a.Title, a.Message); err != nil {
			// One failure logs and continues — never aborts the loop.
			log.Printf("failed to send announcement %s to %s: %v", a.ID, email, err)
		}
	}

	return s.announceRepo.MarkAnnouncementSent(ctx, a.ID, time.Now(), count)
}

// ListAnnouncements returns all announcements ordered by created_at DESC.
func (s *Service) ListAnnouncements(ctx context.Context) ([]model.Announcement, error) {
	return s.announceRepo.ListAnnouncements(ctx)
}

// SetAnnounceRepo sets the announcement repository. Used in tests.
func (s *Service) SetAnnounceRepo(repo AnnounceRepo) {
	s.announceRepo = repo
}
