package service

import (
	"context"
	"slices"
	"testing"
	"time"

	"akademi-bimbel/internal/model"

	"github.com/stretchr/testify/require"
)

func TestAffiliateBroadcastRecipientGroupIsValid(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	a, err := svc.CreateAnnouncement(ctx, "admin-1", "Judul Affiliator", "Pesan program", "announcement", "affiliators", "draft", nil)
	if err != nil {
		t.Fatalf("CreateAnnouncement(affiliators): %v", err)
	}
	if a.Recipients != "affiliators" {
		t.Errorf("recipients = %q, want affiliators", a.Recipients)
	}

	_, err = svc.CreateAnnouncement(ctx, "admin-1", "Judul", "Pesan", "announcement", "affiliator", "draft", nil)
	if err == nil {
		t.Error("singular 'affiliator' must stay invalid, got nil error")
	}
}

func TestAffiliateBroadcastDispatchesToAffiliators(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAnnounceRepo()
	repo.setEmails("affiliators", []string{"aff1@test.com", "aff2@test.com"})
	email := &fakeEmailProvider{}
	svc := newTestAnnounceService(t, repo, email)

	due := time.Now().Add(-time.Hour)
	_ = repo.CreateAnnouncement(ctx, &model.Announcement{
		Title: "Judul", Message: "Pesan", Type: "announcement",
		Recipients: "affiliators", Status: "scheduled", CreatedBy: "admin-1",
		ScheduledAt: &due,
	})

	dispatched, err := svc.DispatchDueAnnouncements(ctx, 10)
	if err != nil {
		t.Fatalf("DispatchDueAnnouncements: %v", err)
	}
	if dispatched != 1 {
		t.Fatalf("dispatched = %d, want 1", dispatched)
	}
	var got []string
	for _, rec := range email.sent {
		got = append(got, rec.to)
	}
	if !slices.Equal(got, []string{"aff1@test.com", "aff2@test.com"}) {
		t.Errorf("emails sent = %v, want affiliator group only", got)
	}
}

// TestAffiliateBroadcastRecipients_resolvesAffiliatorEmails pins the worker
// fan-out SQL branch (BR-NTF-01): only active users holding an active
// affiliators row get the broadcast email.
func TestAffiliateBroadcastRecipients_resolvesAffiliatorEmails(t *testing.T) {
	_, repo := newRealDBService(t)
	ctx := context.Background()
	pool := repo.Pool()

	activeEmail := "affbcast-active-" + uniqueSuffix() + "@test.local"
	suspendedEmail := "affbcast-susp-" + uniqueSuffix() + "@test.local"
	active := seedAffiliatorOwner(t, repo, activeEmail, "", "active")
	suspended := seedAffiliatorOwner(t, repo, suspendedEmail, "", "suspended")
	student := seedAffiliateTestStudent(t, repo, "affbcast-student-"+uniqueSuffix()+"@test.local", "")
	var bareID string
	bareEmail := "affbcast-bare-" + uniqueSuffix() + "@test.local"
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (name, email, username, role, status) VALUES ('Bare Affiliator', $1, $2, 'affiliator', 'active') RETURNING id`,
		bareEmail, "affbcast_"+uniqueSuffix()).Scan(&bareID))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM affiliators WHERE user_id IN ($1, $2)`, active.userID, suspended.userID)
	})
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3, $4)`, active.userID, suspended.userID, student.ID, bareID)
	})

	emails, err := repo.ListActiveUserEmails(ctx, "affiliators")
	require.NoError(t, err)
	if !slices.Contains(emails, activeEmail) {
		t.Errorf("ListActiveUserEmails(affiliators) misses the active affiliator %q: %v", activeEmail, emails)
	}
	if slices.Contains(emails, suspendedEmail) {
		t.Errorf("suspended affiliator received the broadcast: %v", emails)
	}
	if student.Email != nil && slices.Contains(emails, *student.Email) {
		t.Errorf("student received the affiliator broadcast: %v", emails)
	}
	if slices.Contains(emails, bareEmail) {
		t.Errorf("user with affiliator role but no affiliators row received the broadcast: %v", emails)
	}

	if _, err := repo.ListActiveUserEmails(ctx, "affiliator"); err == nil {
		t.Error("unknown group must error, got nil")
	}
}
