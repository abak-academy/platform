package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newNotifTestService(t *testing.T) (*Service, *redis.Client, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis setup failed: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &Service{rdb: rdb}, rdb, func() {
		rdb.Close()
		mr.Close()
	}
}

func seedNotif(t *testing.T, rdb *redis.Client, n PurchaseNotification) {
	t.Helper()
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	score := float64(n.CreatedAt.UnixMilli())
	if err := rdb.ZAdd(context.Background(), purchaseNotifKey, redis.Z{Score: score, Member: string(data)}).Err(); err != nil {
		t.Fatalf("failed to add notification: %v", err)
	}
}

func TestPushPurchaseNotification(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	orderId := uuid.New()
	notif := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     orderId,
		StudentName: "John Doe",
		Amount:      100000,
		CreatedAt:   time.Now(),
		Read:        false,
	}

	if err := svc.PushPurchaseNotification(ctx, notif); err != nil {
		t.Fatalf("PushPurchaseNotification failed: %v", err)
	}

	members, err := rdb.ZRange(ctx, purchaseNotifKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("failed to get sorted set members: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 member in sorted set, got %d", len(members))
	}

	var retrieved PurchaseNotification
	if err := json.Unmarshal([]byte(members[0]), &retrieved); err != nil {
		t.Fatalf("failed to unmarshal notification: %v", err)
	}

	if retrieved.ID != notif.ID {
		t.Errorf("notification ID mismatch: expected %s, got %s", notif.ID, retrieved.ID)
	}
	if retrieved.StudentName != notif.StudentName {
		t.Errorf("student name mismatch: expected %s, got %s", notif.StudentName, retrieved.StudentName)
	}
}

// The feed is a single shared key, not one per role — a super_admin reading it
// must see what an order confirmation pushed.
func TestPushPurchaseNotificationIsVisibleToAnyReader(t *testing.T) {
	svc, _, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	notif := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "John Doe",
		Amount:      100000,
		CreatedAt:   time.Now(),
	}
	if err := svc.PushPurchaseNotification(ctx, notif); err != nil {
		t.Fatalf("PushPurchaseNotification failed: %v", err)
	}

	for _, userID := range []string{"super-admin-user", "store-admin-user"} {
		got, _, err := svc.ListNotifications(ctx, userID, NotifFilter{Limit: 10})
		if err != nil {
			t.Fatalf("ListNotifications(%s) failed: %v", userID, err)
		}
		if len(got) != 1 {
			t.Fatalf("user %s: expected 1 notification, got %d", userID, len(got))
		}
		if got[0].ID != notif.ID {
			t.Errorf("user %s: expected %s, got %s", userID, notif.ID, got[0].ID)
		}
	}
}

func TestPushPurchaseNotificationTrimsToRetention(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	total := notifRetention + 25
	var newest, oldest string
	for i := 0; i < total; i++ {
		n := PurchaseNotification{
			ID:          uuid.New().String(),
			Type:        "order_confirmed",
			OrderID:     uuid.New(),
			StudentName: "Student",
			Amount:      1000,
			CreatedAt:   now.Add(time.Duration(i) * time.Millisecond),
		}
		if i == 0 {
			oldest = n.ID
		}
		if i == total-1 {
			newest = n.ID
		}
		if err := svc.PushPurchaseNotification(ctx, n); err != nil {
			t.Fatalf("push %d failed: %v", i, err)
		}
	}

	card, err := rdb.ZCard(ctx, purchaseNotifKey).Result()
	if err != nil {
		t.Fatalf("ZCard failed: %v", err)
	}
	if card != int64(notifRetention) {
		t.Fatalf("expected feed trimmed to %d, got %d", notifRetention, card)
	}

	got, _, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}
	if len(got) != 1 || got[0].ID != newest {
		t.Errorf("expected newest %s to survive the trim, got %+v", newest, got)
	}

	all, _, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: notifMaxLim})
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}
	for _, n := range all {
		if n.ID == oldest {
			t.Errorf("oldest notification %s should have been trimmed", oldest)
		}
	}
}

func TestListNotifications(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	notif1 := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "John Doe",
		Amount:      100000,
		CreatedAt:   now.Add(-2 * time.Second),
	}
	notif2 := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_paid",
		OrderID:     uuid.New(),
		StudentName: "Jane Smith",
		Amount:      200000,
		CreatedAt:   now.Add(-1 * time.Second),
	}
	notif3 := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "Bob Jones",
		Amount:      150000,
		CreatedAt:   now,
	}

	for _, n := range []PurchaseNotification{notif1, notif2, notif3} {
		seedNotif(t, rdb, n)
	}

	notifications, nextCursor, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}

	if len(notifications) != 3 {
		t.Fatalf("expected 3 notifications, got %d", len(notifications))
	}

	if notifications[0].ID != notif3.ID {
		t.Errorf("first notification should be notif3, got %s", notifications[0].ID)
	}
	if notifications[1].ID != notif2.ID {
		t.Errorf("second notification should be notif2, got %s", notifications[1].ID)
	}
	if notifications[2].ID != notif1.ID {
		t.Errorf("third notification should be notif1, got %s", notifications[2].ID)
	}

	if nextCursor != "" {
		t.Errorf("expected empty next cursor, got %s", nextCursor)
	}
}

func TestListNotificationsWithTypeFilter(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	notif1 := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "John Doe",
		Amount:      100000,
		CreatedAt:   now.Add(-1 * time.Second),
	}
	notif2 := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_paid",
		OrderID:     uuid.New(),
		StudentName: "Jane Smith",
		Amount:      200000,
		CreatedAt:   now,
	}

	seedNotif(t, rdb, notif1)
	seedNotif(t, rdb, notif2)

	notifications, _, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Type: "order_confirmed", Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Type != "order_confirmed" {
		t.Errorf("expected type order_confirmed, got %s", notifications[0].Type)
	}
}

func TestMarkNotificationRead(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	notifId := uuid.New().String()
	userID := "user-1"

	if err := svc.MarkNotificationRead(ctx, userID, notifId); err != nil {
		t.Fatalf("MarkNotificationRead failed: %v", err)
	}

	isMember, err := rdb.SIsMember(ctx, notifReadKey(userID), notifId).Result()
	if err != nil {
		t.Fatalf("failed to check read set: %v", err)
	}
	if !isMember {
		t.Error("expected notification id in the user's read set")
	}

	ttl, err := rdb.TTL(ctx, notifReadKey(userID)).Result()
	if err != nil {
		t.Fatalf("failed to read TTL: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("expected the read set to carry a TTL, got %v", ttl)
	}
}

func TestMarkNotificationReadIdempotent(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	notifId := uuid.New().String()
	userID := "user-1"

	if err := svc.MarkNotificationRead(ctx, userID, notifId); err != nil {
		t.Fatalf("first MarkNotificationRead failed: %v", err)
	}
	if err := svc.MarkNotificationRead(ctx, userID, notifId); err != nil {
		t.Fatalf("second MarkNotificationRead failed: %v", err)
	}

	card, err := rdb.SCard(ctx, notifReadKey(userID)).Result()
	if err != nil {
		t.Fatalf("SCard failed: %v", err)
	}
	if card != 1 {
		t.Errorf("expected 1 member after marking twice, got %d", card)
	}
}

// Read state is per user: one admin clearing their inbox must not clear it for
// everyone else sharing the role.
func TestMarkNotificationReadIsPerUser(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	notif := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "John Doe",
		Amount:      100000,
		CreatedAt:   time.Now(),
	}
	seedNotif(t, rdb, notif)

	if err := svc.MarkNotificationRead(ctx, "user-a", notif.ID); err != nil {
		t.Fatalf("MarkNotificationRead failed: %v", err)
	}

	forA, _, err := svc.ListNotifications(ctx, "user-a", NotifFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications(user-a) failed: %v", err)
	}
	if len(forA) != 1 || !forA[0].Read {
		t.Errorf("expected user-a to see the notification as read, got %+v", forA)
	}

	forB, _, err := svc.ListNotifications(ctx, "user-b", NotifFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications(user-b) failed: %v", err)
	}
	if len(forB) != 1 || forB[0].Read {
		t.Errorf("expected user-b to still see the notification as unread, got %+v", forB)
	}

	unreadForB, _, err := svc.ListNotifications(ctx, "user-b", NotifFilter{UnreadOnly: true, Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications(user-b, unread) failed: %v", err)
	}
	if len(unreadForB) != 1 {
		t.Errorf("expected user-b's unread filter to still return the notification, got %d", len(unreadForB))
	}
}

func TestListNotificationsUnreadOnlyFilter(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	userID := "user-1"
	now := time.Now()
	notif1 := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "John Doe",
		Amount:      100000,
		CreatedAt:   now.Add(-1 * time.Second),
	}
	notif2 := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_paid",
		OrderID:     uuid.New(),
		StudentName: "Jane Smith",
		Amount:      200000,
		CreatedAt:   now,
	}

	seedNotif(t, rdb, notif1)
	seedNotif(t, rdb, notif2)

	if err := svc.MarkNotificationRead(ctx, userID, notif1.ID); err != nil {
		t.Fatalf("MarkNotificationRead failed: %v", err)
	}

	notifications, _, err := svc.ListNotifications(ctx, userID, NotifFilter{UnreadOnly: true, Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 unread notification, got %d", len(notifications))
	}
	if notifications[0].ID != notif2.ID {
		t.Errorf("expected notif2 (unread), got %s", notifications[0].ID)
	}
}

func TestListNotificationsWithPagination(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	notifs := make([]PurchaseNotification, 5)
	for i := 0; i < 5; i++ {
		notifs[i] = PurchaseNotification{
			ID:          uuid.New().String(),
			Type:        "order_confirmed",
			OrderID:     uuid.New(),
			StudentName: "Student " + string(rune('0'+i)),
			Amount:      100000 * int64(i+1),
			CreatedAt:   now.Add(time.Duration(-i) * time.Second),
		}
		seedNotif(t, rdb, notifs[i])
	}

	filter := NotifFilter{Limit: 2}
	page1, cursor1, err := svc.ListNotifications(ctx, "user-1", filter)
	if err != nil {
		t.Fatalf("ListNotifications page 1 failed: %v", err)
	}

	if len(page1) != 2 {
		t.Fatalf("expected 2 notifications on page 1, got %d", len(page1))
	}
	if page1[0].ID != notifs[0].ID {
		t.Errorf("expected notifs[0] first, got %s", page1[0].ID)
	}
	if page1[1].ID != notifs[1].ID {
		t.Errorf("expected notifs[1] second, got %s", page1[1].ID)
	}

	if cursor1 == "" {
		t.Fatalf("expected non-empty cursor for page 2")
	}

	page2, cursor2, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 2, Cursor: cursor1})
	if err != nil {
		t.Fatalf("ListNotifications page 2 failed: %v", err)
	}

	if len(page2) != 2 {
		t.Fatalf("expected 2 notifications on page 2, got %d", len(page2))
	}
	// The old offset cursor could not catch this: page 2 must continue the
	// sequence rather than repeat or skip items.
	if page2[0].ID != notifs[2].ID {
		t.Errorf("expected notifs[2] to open page 2, got %s", page2[0].ID)
	}
	if page2[1].ID != notifs[3].ID {
		t.Errorf("expected notifs[3] to close page 2, got %s", page2[1].ID)
	}

	page3, cursor3, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 2, Cursor: cursor2})
	if err != nil {
		t.Fatalf("ListNotifications page 3 failed: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("expected 1 notification on the final page, got %d", len(page3))
	}
	if page3[0].ID != notifs[4].ID {
		t.Errorf("expected notifs[4] on the final page, got %s", page3[0].ID)
	}
	if cursor3 != "" {
		t.Errorf("expected no cursor past the last item, got %s", cursor3)
	}
}

// A page whose tail is filtered out must still hand back a cursor when older
// matching items exist, instead of silently ending the feed.
func TestListNotificationsCursorSurvivesFilteredTail(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	userID := "user-1"
	now := time.Now()

	newestUnread := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "Newest",
		Amount:      100000,
		CreatedAt:   now,
	}
	seedNotif(t, rdb, newestUnread)

	// A long read run, longer than one scan batch, between the two unread items.
	for i := 0; i < notifScanBatch+10; i++ {
		n := PurchaseNotification{
			ID:          uuid.New().String(),
			Type:        "order_confirmed",
			OrderID:     uuid.New(),
			StudentName: "Read",
			Amount:      1000,
			CreatedAt:   now.Add(-time.Duration(i+1) * time.Second),
		}
		seedNotif(t, rdb, n)
		if err := svc.MarkNotificationRead(ctx, userID, n.ID); err != nil {
			t.Fatalf("MarkNotificationRead failed: %v", err)
		}
	}

	oldestUnread := PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "Oldest",
		Amount:      200000,
		CreatedAt:   now.Add(-time.Duration(notifScanBatch+50) * time.Second),
	}
	seedNotif(t, rdb, oldestUnread)

	page1, cursor, err := svc.ListNotifications(ctx, userID, NotifFilter{UnreadOnly: true, Limit: 1})
	if err != nil {
		t.Fatalf("ListNotifications page 1 failed: %v", err)
	}
	if len(page1) != 1 || page1[0].ID != newestUnread.ID {
		t.Fatalf("expected the newest unread on page 1, got %+v", page1)
	}
	if cursor == "" {
		t.Fatal("expected a cursor: an older unread notification exists past the read run")
	}

	page2, cursor2, err := svc.ListNotifications(ctx, userID, NotifFilter{UnreadOnly: true, Limit: 1, Cursor: cursor})
	if err != nil {
		t.Fatalf("ListNotifications page 2 failed: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != oldestUnread.ID {
		t.Fatalf("expected the oldest unread on page 2, got %+v", page2)
	}
	if cursor2 != "" {
		t.Errorf("expected no cursor after the last unread item, got %s", cursor2)
	}
}

// Notifications minted in the same millisecond must all be reachable — an
// exclusive score bound would drop the ties.
func TestListNotificationsCursorHandlesSameMillisecondTies(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	shared := time.Now().Truncate(time.Millisecond)
	ids := map[string]bool{}
	for i := 0; i < 3; i++ {
		n := PurchaseNotification{
			ID:          uuid.New().String(),
			Type:        "order_confirmed",
			OrderID:     uuid.New(),
			StudentName: "Tie",
			Amount:      int64(1000 * (i + 1)),
			CreatedAt:   shared,
		}
		ids[n.ID] = false
		seedNotif(t, rdb, n)
	}

	cursor := ""
	for page := 0; page < 3; page++ {
		got, next, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatalf("page %d failed: %v", page, err)
		}
		if len(got) != 1 {
			t.Fatalf("page %d: expected 1 notification, got %d", page, len(got))
		}
		seen, known := ids[got[0].ID]
		if !known {
			t.Fatalf("page %d returned an unknown id %s", page, got[0].ID)
		}
		if seen {
			t.Fatalf("page %d repeated id %s", page, got[0].ID)
		}
		ids[got[0].ID] = true
		cursor = next
	}

	for id, seen := range ids {
		if !seen {
			t.Errorf("notification %s was never returned across the three pages", id)
		}
	}
	if cursor != "" {
		t.Errorf("expected no cursor after exhausting the ties, got %s", cursor)
	}
}

func TestListNotificationsLimitBounds(t *testing.T) {
	svc, _, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	if _, _, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 0}); err != nil {
		t.Fatalf("zero limit should fall back to the default: %v", err)
	}
	if _, _, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 5000}); err != nil {
		t.Fatalf("oversized limit should clamp: %v", err)
	}
}

func TestDecodeNotifCursorRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "abc", "_id", "123_", "notanumber_id"} {
		if _, _, ok := decodeNotifCursor(bad); ok {
			t.Errorf("expected %q to be rejected", bad)
		}
	}

	score, id, ok := decodeNotifCursor("1700000000000_abc-def")
	if !ok || score != 1700000000000 || id != "abc-def" {
		t.Errorf("round trip failed: %d %s %v", score, id, ok)
	}
}

// A cursor pointing at a notification that has since been trimmed must not
// resurrect the whole feed as if it were page one.
func TestListNotificationsStaleCursorReturnsEmpty(t *testing.T) {
	svc, rdb, cleanup := newNotifTestService(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	seedNotif(t, rdb, PurchaseNotification{
		ID:          uuid.New().String(),
		Type:        "order_confirmed",
		OrderID:     uuid.New(),
		StudentName: "Live",
		Amount:      1000,
		CreatedAt:   now,
	})

	stale := encodeNotifCursor(PurchaseNotification{
		ID:        uuid.New().String(),
		CreatedAt: now,
	})

	got, next, err := svc.ListNotifications(ctx, "user-1", NotifFilter{Limit: 10, Cursor: stale})
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no items for a stale cursor, got %d", len(got))
	}
	if next != "" {
		t.Errorf("expected no cursor, got %s", next)
	}
}
