package repository

import (
	"context"
	"time"

	"akademi-bimbel/internal/model"
)

// Compile-time check: *Repository must implement all announcement methods.
var _ interface {
	CreateAnnouncement(context.Context, *model.Announcement) error
	GetAnnouncementByID(context.Context, string) (*model.Announcement, error)
	ListAnnouncements(context.Context) ([]model.Announcement, error)
	UpdateAnnouncement(context.Context, string, *model.Announcement) error
	DeleteAnnouncement(context.Context, string) error
	ClaimDueAnnouncements(context.Context, time.Time, int) ([]model.Announcement, error)
	MarkAnnouncementSent(context.Context, string, time.Time, int) error
	ListActiveUserEmails(context.Context, string) ([]string, error)
} = (*Repository)(nil)
