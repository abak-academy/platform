package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/model"
)

// fakeAnnounceRepo implements announceRepo for testing.
type fakeAnnounceRepo struct {
	announcements map[string]*model.Announcement
	emails        map[string][]string // recipient group -> email list
	seq           int
}

func newFakeAnnounceRepo() *fakeAnnounceRepo {
	return &fakeAnnounceRepo{
		announcements: make(map[string]*model.Announcement),
		emails:        make(map[string][]string),
	}
}

func (f *fakeAnnounceRepo) CreateAnnouncement(_ context.Context, a *model.Announcement) error {
	f.seq++
	a.ID = fmt.Sprintf("ann-%d", f.seq)
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	cp := *a
	f.announcements[a.ID] = &cp
	return nil
}

func (f *fakeAnnounceRepo) GetAnnouncementByID(_ context.Context, id string) (*model.Announcement, error) {
	a, ok := f.announcements[id]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (f *fakeAnnounceRepo) UpdateAnnouncement(_ context.Context, id string, a *model.Announcement) error {
	existing, ok := f.announcements[id]
	if !ok {
		return nil
	}
	existing.Title = a.Title
	existing.Message = a.Message
	existing.Type = a.Type
	existing.Recipients = a.Recipients
	existing.ScheduledAt = a.ScheduledAt
	existing.UpdatedAt = time.Now()
	return nil
}

func (f *fakeAnnounceRepo) DeleteAnnouncement(_ context.Context, id string) error {
	delete(f.announcements, id)
	return nil
}

func (f *fakeAnnounceRepo) ListAnnouncements(_ context.Context) ([]model.Announcement, error) {
	var result []model.Announcement
	for _, a := range f.announcements {
		result = append(result, *a)
	}
	return result, nil
}

func (f *fakeAnnounceRepo) ListActiveUserEmails(_ context.Context, recipients string) ([]string, error) {
	emails, ok := f.emails[recipients]
	if !ok {
		return nil, nil
	}
	out := make([]string, len(emails))
	copy(out, emails)
	return out, nil
}

func (f *fakeAnnounceRepo) MarkAnnouncementSent(_ context.Context, id string, sentAt time.Time, recipientCount int) error {
	a, ok := f.announcements[id]
	if !ok {
		return nil
	}
	a.Status = "sent"
	a.SentAt = &sentAt
	a.RecipientCount = &recipientCount
	a.UpdatedAt = time.Now()
	return nil
}

func (f *fakeAnnounceRepo) ScheduleAnnouncementNow(_ context.Context, id string, at time.Time) error {
	a, ok := f.announcements[id]
	if !ok {
		return nil
	}
	a.Status = "scheduled"
	a.ScheduledAt = &at
	a.SentAt = nil
	a.RecipientCount = nil
	a.UpdatedAt = time.Now()
	return nil
}

func (f *fakeAnnounceRepo) ClaimDueAnnouncements(_ context.Context, now time.Time, limit int) ([]model.Announcement, error) {
	var due []model.Announcement
	for _, a := range f.announcements {
		if a.Status == "scheduled" && a.ScheduledAt != nil && !a.ScheduledAt.After(now) {
			due = append(due, *a)
		}
	}
	if len(due) > limit {
		due = due[:limit]
	}
	for _, a := range due {
		cp := a
		now := time.Now()
		cp.Status = "sent"
		cp.SentAt = &now
		f.announcements[cp.ID] = &cp
	}
	return due, nil
}

// setEmails configures what ListActiveUserEmails returns for a recipient group.
func (f *fakeAnnounceRepo) setEmails(recipients string, emails []string) {
	f.emails[recipients] = emails
}

// seedSent inserts an already-sent announcement directly, bypassing
// CreateAnnouncement (whose "sent" status now routes through the scheduler).
func (f *fakeAnnounceRepo) seedSent(title string) (string, error) {
	a := &model.Announcement{
		Title: title, Message: "Message", Type: "announcement",
		Recipients: "all", Status: "sent", CreatedBy: "admin-1",
	}
	if err := f.CreateAnnouncement(context.Background(), a); err != nil {
		return "", err
	}
	return a.ID, nil
}

// fakeEmailProvider records sent emails and can fail on configured addresses.
type fakeEmailProvider struct {
	sent  []emailRecord
	fails map[string]bool
}

type emailRecord struct {
	to      string
	subject string
	body    string
}

func (f *fakeEmailProvider) SendEmail(_ context.Context, to, subject, body string) error {
	f.sent = append(f.sent, emailRecord{to, subject, body})
	if f.fails != nil && f.fails[to] {
		return errors.New("send failed")
	}
	return nil
}

// newTestAnnounceService creates a Service with test doubles for announcement tests.
func newTestAnnounceService(t *testing.T, repo *fakeAnnounceRepo, email *fakeEmailProvider) *Service {
	t.Helper()
	return &Service{
		announceRepo:  repo,
		emailProvider: email,
	}
}

func TestCreateAnnouncement_Draft(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Test Title", "Test Message", "announcement", "all", "draft", nil)
	if err != nil {
		t.Fatalf("CreateAnnouncement(draft): %v", err)
	}
	if a.ID == "" {
		t.Error("want non-empty ID")
	}
	if a.Status != "draft" {
		t.Errorf("want status draft, got %s", a.Status)
	}
	if a.SentAt != nil {
		t.Error("want nil SentAt for draft")
	}
	if a.ScheduledAt != nil {
		t.Error("want nil ScheduledAt for draft")
	}
	if len(email.sent) != 0 {
		t.Errorf("want 0 emails sent for draft, got %d", len(email.sent))
	}
}

func TestCreateAnnouncement_Scheduled(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	future := time.Now().Add(24 * time.Hour)
	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Scheduled Title", "Scheduled Message", "promo", "students", "scheduled", &future)
	if err != nil {
		t.Fatalf("CreateAnnouncement(scheduled): %v", err)
	}
	if a.Status != "scheduled" {
		t.Errorf("want status scheduled, got %s", a.Status)
	}
	if a.ScheduledAt == nil {
		t.Fatal("want non-nil ScheduledAt")
	}
	if !a.ScheduledAt.Equal(future) {
		t.Errorf("want ScheduledAt %v, got %v", future, a.ScheduledAt)
	}
	if len(email.sent) != 0 {
		t.Errorf("want 0 emails sent for scheduled, got %d", len(email.sent))
	}
}

func TestCreateAnnouncement_Scheduled_RejectsMissingScheduledAt(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	_, err := svc.CreateAnnouncement(ctx, "admin-1", "Title", "Message", "announcement", "all", "scheduled", nil)
	if err == nil {
		t.Fatal("want error for missing scheduled_at")
	}
}

func TestCreateAnnouncement_Scheduled_RejectsPastScheduledAt(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	past := time.Now().Add(-1 * time.Hour)
	_, err := svc.CreateAnnouncement(ctx, "admin-1", "Title", "Message", "announcement", "all", "scheduled", &past)
	if err == nil {
		t.Fatal("want error for past scheduled_at")
	}
}

func TestCreateAnnouncement_SentNow_RoutesThroughScheduler(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	repo.setEmails("all", []string{"alice@test.com", "bob@test.com", "charlie@test.com"})
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Send Now Title", "Send Now Message", "announcement", "all", "sent", nil)
	if err != nil {
		t.Fatalf("CreateAnnouncement(sent): %v", err)
	}
	if a.Status != "scheduled" {
		t.Errorf("want status scheduled (queued for the paced worker loop), got %s", a.Status)
	}
	if a.ScheduledAt == nil {
		t.Fatal("want non-nil ScheduledAt")
	}
	if a.ScheduledAt.After(time.Now()) {
		t.Errorf("want scheduled_at ~now, got %v", a.ScheduledAt)
	}
	if a.SentAt != nil {
		t.Error("want nil SentAt — no email may leave the request path")
	}
	if len(email.sent) != 0 {
		t.Errorf("want 0 emails sent from the API request, got %d", len(email.sent))
	}
}

func TestCreateAnnouncement_InvalidType(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	_, err := svc.CreateAnnouncement(ctx, "admin-1", "Title", "Message", "invalid-type", "all", "draft", nil)
	if err == nil {
		t.Fatal("want error for invalid type")
	}
}

func TestCreateAnnouncement_InvalidRecipients(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	_, err := svc.CreateAnnouncement(ctx, "admin-1", "Title", "Message", "announcement", "invalid-group", "draft", nil)
	if err == nil {
		t.Fatal("want error for invalid recipients")
	}
}

func TestUpdateAnnouncement_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	_, err := svc.UpdateAnnouncement(ctx, "nonexistent", "Title", "Message", "announcement", "all", nil)
	if !errors.Is(err, ErrAnnouncementNotFound) {
		t.Errorf("want ErrAnnouncementNotFound, got %v", err)
	}
}

func TestUpdateAnnouncement_Sent_Rejected(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	// Seed a sent announcement directly; CreateAnnouncement no longer produces
	// one from the API (status "sent" routes through the scheduler).
	id, err := repo.seedSent("Original")
	if err != nil {
		t.Fatalf("seed sent announcement: %v", err)
	}

	_, err = svc.UpdateAnnouncement(ctx, id, "Updated", "Updated", "announcement", "all", nil)
	if !errors.Is(err, ErrAnnouncementImmutable) {
		t.Errorf("want ErrAnnouncementImmutable, got %v", err)
	}
}

func TestUpdateAnnouncement_Draft_Allowed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Original", "Original", "announcement", "all", "draft", nil)
	if err != nil {
		t.Fatalf("setup CreateAnnouncement: %v", err)
	}

	result, err := svc.UpdateAnnouncement(ctx, a.ID, "Updated Title", "Updated Message", "promo", "students", nil)
	if err != nil {
		t.Fatalf("UpdateAnnouncement(draft): %v", err)
	}
	if result.Title != "Updated Title" {
		t.Errorf("want title 'Updated Title', got %s", result.Title)
	}
	if result.Message != "Updated Message" {
		t.Errorf("want message 'Updated Message', got %s", result.Message)
	}
	if result.Type != "promo" {
		t.Errorf("want type 'promo', got %s", result.Type)
	}
	if result.Recipients != "students" {
		t.Errorf("want recipients 'students', got %s", result.Recipients)
	}
}

func TestUpdateAnnouncement_Scheduled_Allowed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	future := time.Now().Add(24 * time.Hour)
	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Original", "Original", "announcement", "all", "scheduled", &future)
	if err != nil {
		t.Fatalf("setup CreateAnnouncement: %v", err)
	}

	newFuture := time.Now().Add(48 * time.Hour)
	result, err := svc.UpdateAnnouncement(ctx, a.ID, "Updated", "Updated", "promo", "students", &newFuture)
	if err != nil {
		t.Fatalf("UpdateAnnouncement(scheduled): %v", err)
	}
	if result.Title != "Updated" {
		t.Errorf("want title 'Updated', got %s", result.Title)
	}
}

func TestDeleteAnnouncement_Sent_Rejected(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	id, err := repo.seedSent("Title")
	if err != nil {
		t.Fatalf("seed sent announcement: %v", err)
	}

	err = svc.DeleteAnnouncement(ctx, id)
	if !errors.Is(err, ErrAnnouncementImmutable) {
		t.Errorf("want ErrAnnouncementImmutable, got %v", err)
	}
}

func TestDeleteAnnouncement_Draft_Allowed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Title", "Message", "announcement", "all", "draft", nil)
	if err != nil {
		t.Fatalf("setup CreateAnnouncement: %v", err)
	}

	err = svc.DeleteAnnouncement(ctx, a.ID)
	if err != nil {
		t.Fatalf("DeleteAnnouncement(draft): %v", err)
	}
}

func TestDeleteAnnouncement_Scheduled_Allowed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	future := time.Now().Add(24 * time.Hour)
	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Title", "Message", "announcement", "all", "scheduled", &future)
	if err != nil {
		t.Fatalf("setup CreateAnnouncement: %v", err)
	}

	err = svc.DeleteAnnouncement(ctx, a.ID)
	if err != nil {
		t.Fatalf("DeleteAnnouncement(scheduled): %v", err)
	}
}

func TestDeleteAnnouncement_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	err := svc.DeleteAnnouncement(ctx, "nonexistent")
	if !errors.Is(err, ErrAnnouncementNotFound) {
		t.Errorf("want ErrAnnouncementNotFound, got %v", err)
	}
}

func TestSendAnnouncementNow(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	repo.setEmails("students", []string{"student1@test.com", "student2@test.com"})
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	future := time.Now().Add(24 * time.Hour)
	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Scheduled", "Message", "announcement", "students", "scheduled", &future)
	if err != nil {
		t.Fatalf("setup CreateAnnouncement: %v", err)
	}

	result, err := svc.SendAnnouncementNow(ctx, a.ID)
	if err != nil {
		t.Fatalf("SendAnnouncementNow: %v", err)
	}
	if result.Status != "scheduled" {
		t.Errorf("want status scheduled (queued for the paced worker loop), got %s", result.Status)
	}
	if result.ScheduledAt == nil || result.ScheduledAt.After(time.Now()) {
		t.Errorf("want scheduled_at ~now, got %v", result.ScheduledAt)
	}
	if result.SentAt != nil {
		t.Error("want nil SentAt — dispatch belongs to the worker")
	}
	if len(email.sent) != 0 {
		t.Errorf("want 0 emails sent from the API request, got %d", len(email.sent))
	}
}

func TestSendAnnouncementNow_SentStatusRejected(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	id, err := repo.seedSent("Title")
	if err != nil {
		t.Fatalf("seed sent announcement: %v", err)
	}

	_, err = svc.SendAnnouncementNow(ctx, id)
	if !errors.Is(err, ErrAnnouncementImmutable) {
		t.Errorf("want ErrAnnouncementImmutable, got %v", err)
	}
}

func TestSendAnnouncementNow_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	_, err := svc.SendAnnouncementNow(ctx, "nonexistent")
	if !errors.Is(err, ErrAnnouncementNotFound) {
		t.Errorf("want ErrAnnouncementNotFound, got %v", err)
	}
}

func TestDispatchDueAnnouncements(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	repo.setEmails("all", []string{"alice@test.com"})
	repo.setEmails("students", []string{"bob@test.com"})
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	// Create two due announcements by seeding the repo directly (past scheduled_at
	// can't go through CreateAnnouncement which validates future-only for "scheduled").
	_ = repo.CreateAnnouncement(ctx, &model.Announcement{
		Title: "Due 1", Message: "Message 1", Type: "announcement",
		Recipients: "all", Status: "scheduled", CreatedBy: "admin-1",
		ScheduledAt: timePtr(time.Now().Add(-1 * time.Hour)),
	})
	_ = repo.CreateAnnouncement(ctx, &model.Announcement{
		Title: "Due 2", Message: "Message 2", Type: "announcement",
		Recipients: "students", Status: "scheduled", CreatedBy: "admin-1",
		ScheduledAt: timePtr(time.Now().Add(-30 * time.Minute)),
	})
	// Future announcement (should not be dispatched)
	_ = repo.CreateAnnouncement(ctx, &model.Announcement{
		Title: "Future", Message: "Message", Type: "announcement",
		Recipients: "all", Status: "scheduled", CreatedBy: "admin-1",
		ScheduledAt: timePtr(time.Now().Add(24 * time.Hour)),
	})

	dispatched, err := svc.DispatchDueAnnouncements(ctx, 10)
	if err != nil {
		t.Fatalf("DispatchDueAnnouncements: %v", err)
	}
	if dispatched != 2 {
		t.Errorf("want 2 dispatched, got %d", dispatched)
	}
	if len(email.sent) != 2 { // 1 email per recipient group
		t.Errorf("want 2 total emails sent, got %d", len(email.sent))
	}
}

func TestDispatchDueAnnouncements_SurvivesOneFailing(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	// One recipient group will resolve with empty list (no emails configured)
	// Another should work fine
	repo.setEmails("all", []string{"alice@test.com"})
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	// Seed directly to bypass scheduled_at validation
	_ = repo.CreateAnnouncement(ctx, &model.Announcement{
		Title: "No Emails", Message: "Message", Type: "announcement",
		Recipients: "students", Status: "scheduled", CreatedBy: "admin-1",
		ScheduledAt: timePtr(time.Now().Add(-1 * time.Hour)),
	})
	_ = repo.CreateAnnouncement(ctx, &model.Announcement{
		Title: "Good", Message: "Message", Type: "announcement",
		Recipients: "all", Status: "scheduled", CreatedBy: "admin-1",
		ScheduledAt: timePtr(time.Now().Add(-30 * time.Minute)),
	})

	dispatched, err := svc.DispatchDueAnnouncements(ctx, 10)
	if err != nil {
		t.Fatalf("DispatchDueAnnouncements: %v", err)
	}
	// The "students" one won't send any emails (empty list) but is still considered dispatched
	// because it resolves, sends 0 emails (no error), and marks sent.
	if dispatched != 2 {
		t.Errorf("want 2 dispatched, got %d", dispatched)
	}
}

func TestDispatchDueAnnouncements_Empty(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	dispatched, err := svc.DispatchDueAnnouncements(ctx, 10)
	if err != nil {
		t.Fatalf("DispatchDueAnnouncements: %v", err)
	}
	if dispatched != 0 {
		t.Errorf("want 0 dispatched, got %d", dispatched)
	}
}

func TestDispatchDueAnnouncements_PacesEmails(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	repo.setEmails("all", []string{"a@test.com", "b@test.com", "c@test.com"})
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)
	// 30 emails/minute → 2s spacing between consecutive sends.
	svc.cfg = &config.Config{EmailSendPerMinute: 30}
	var sleeps []time.Duration
	svc.emailSleep = func(d time.Duration) { sleeps = append(sleeps, d) }

	_ = repo.CreateAnnouncement(ctx, &model.Announcement{
		Title: "Paced", Message: "Message", Type: "announcement",
		Recipients: "all", Status: "scheduled", CreatedBy: "admin-1",
		ScheduledAt: timePtr(time.Now().Add(-1 * time.Hour)),
	})

	if _, err := svc.DispatchDueAnnouncements(ctx, 10); err != nil {
		t.Fatalf("DispatchDueAnnouncements: %v", err)
	}
	if len(email.sent) != 3 {
		t.Fatalf("want 3 emails sent, got %d", len(email.sent))
	}
	// First send is immediate; the remaining two each wait one interval.
	if len(sleeps) != 2 {
		t.Fatalf("want 2 pacing sleeps (3 sends, first immediate), got %d", len(sleeps))
	}
	for i, d := range sleeps {
		if d < 1900*time.Millisecond || d > 2*time.Second {
			t.Errorf("sleep[%d] = %v, want ~2s", i, d)
		}
	}
}
