package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// One shared feed. Access is already gated by the notifications:read
	// capability, which only admin_store and super_admin hold.
	purchaseNotifKey = "notif:purchase"
	notifReadPrefix  = "notif_read:"

	notifRetention  = 500
	notifReadTTL    = 90 * 24 * time.Hour
	notifScanBatch  = 100
	notifDefaultLim = 20
	notifMaxLim     = 100
)

type PurchaseNotification struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	OrderID     uuid.UUID `json:"order_id"`
	StudentName string    `json:"student_name"`
	Amount      int64     `json:"amount"`
	CreatedAt   time.Time `json:"created_at"`
	Read        bool      `json:"read"`
}

type NotifFilter struct {
	Type       string
	UnreadOnly bool
	Cursor     string
	Limit      int
}

func notifReadKey(userID string) string { return notifReadPrefix + userID }

// encodeNotifCursor pairs the sort score with the id so resuming stays exact
// when several notifications share a millisecond.
func encodeNotifCursor(n PurchaseNotification) string {
	return strconv.FormatInt(n.CreatedAt.UnixMilli(), 10) + "_" + n.ID
}

func decodeNotifCursor(cursor string) (score int64, id string, ok bool) {
	sep := strings.Index(cursor, "_")
	if sep <= 0 || sep == len(cursor)-1 {
		return 0, "", false
	}
	score, err := strconv.ParseInt(cursor[:sep], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return score, cursor[sep+1:], true
}

func (s *Service) PushPurchaseNotification(ctx context.Context, notif PurchaseNotification) error {
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}

	score := float64(notif.CreatedAt.UnixMilli())
	if err := s.rdb.ZAdd(ctx, purchaseNotifKey, redis.Z{Score: score, Member: string(data)}).Err(); err != nil {
		return err
	}

	// Drop everything past the newest notifRetention so the feed cannot grow
	// without bound; nothing else ever prunes this key.
	return s.rdb.ZRemRangeByRank(ctx, purchaseNotifKey, 0, -(notifRetention + 1)).Err()
}

func (s *Service) ListNotifications(ctx context.Context, userID string, filter NotifFilter) ([]PurchaseNotification, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = notifDefaultLim
	}
	if filter.Limit > notifMaxLim {
		filter.Limit = notifMaxLim
	}

	readIDs, err := s.rdb.SMembers(ctx, notifReadKey(userID)).Result()
	if err != nil && err != redis.Nil {
		return nil, "", err
	}
	read := make(map[string]struct{}, len(readIDs))
	for _, id := range readIDs {
		read[id] = struct{}{}
	}

	max := "+inf"
	cursorID := ""
	if filter.Cursor != "" {
		if score, id, ok := decodeNotifCursor(filter.Cursor); ok {
			// Inclusive max, then skip forward to the cursor id — an exclusive
			// bound would swallow ties on the same millisecond.
			max = strconv.FormatInt(score, 10)
			cursorID = id
		}
	}
	resumed := cursorID == ""

	out := make([]PurchaseNotification, 0, filter.Limit)
	var offset int64

	for {
		members, err := s.rdb.ZRevRangeByScore(ctx, purchaseNotifKey, &redis.ZRangeBy{
			Min:    "-inf",
			Max:    max,
			Offset: offset,
			Count:  notifScanBatch,
		}).Result()
		if err != nil {
			return nil, "", err
		}
		if len(members) == 0 {
			return out, "", nil
		}
		offset += int64(len(members))

		for _, member := range members {
			var notif PurchaseNotification
			if err := json.Unmarshal([]byte(member), &notif); err != nil {
				continue
			}

			if !resumed {
				if notif.ID == cursorID {
					resumed = true
				}
				continue
			}

			if filter.Type != "" && notif.Type != filter.Type {
				continue
			}

			_, notif.Read = read[notif.ID]
			if filter.UnreadOnly && notif.Read {
				continue
			}

			// A cursor is only handed back once a further *matching* item is in
			// hand, so filtered-out tails never masquerade as another page.
			if len(out) == filter.Limit {
				return out, encodeNotifCursor(out[len(out)-1]), nil
			}
			out = append(out, notif)
		}

		if len(members) < notifScanBatch {
			return out, "", nil
		}
	}
}

func (s *Service) MarkNotificationRead(ctx context.Context, userID, id string) error {
	key := notifReadKey(userID)
	if err := s.rdb.SAdd(ctx, key, id).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, key, notifReadTTL).Err()
}
