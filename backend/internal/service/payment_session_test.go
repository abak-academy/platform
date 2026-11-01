package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestPaymentSession(t *testing.T) {
	tests := []struct {
		name       string
		expiresIn  time.Duration
		writeError bool
	}{
		{"round trip with bounded TTL", time.Hour, false},
		{"expired session stores nothing", -time.Second, false},
		{"zero TTL stores nothing", 0, false},
		{"Redis write error is swallowed", time.Hour, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			svc := &Service{rdb: rdb}
			ctx := context.Background()
			orderID := "session-" + uniqueSuffix()
			session := paymentSession{GatewayRef: "ref", SnapToken: "token", PaymentURL: "https://payment.test/session", ExpiresAt: time.Now().Add(tt.expiresIn).UTC()}
			before := time.Until(session.ExpiresAt)
			if tt.writeError {
				mr.SetError("ERR write unavailable")
			}
			svc.storePaymentSession(ctx, orderID, session)
			mr.SetError("")
			got, ok := svc.loadPaymentSession(ctx, orderID)
			if tt.expiresIn <= 0 || tt.writeError {
				if ok || mr.Exists("payment:session:"+orderID) {
					t.Fatalf("session was stored: %+v", got)
				}
				return
			}
			if !ok || got != session {
				t.Fatalf("loaded = %+v, %v; want %+v, true", got, ok, session)
			}
			ttl := mr.TTL("payment:session:" + orderID)
			if ttl <= 0 || ttl > before {
				t.Errorf("TTL = %v, want > 0 and <= %v", ttl, before)
			}
			raw, err := mr.Get("payment:session:" + orderID)
			if err != nil {
				t.Fatal(err)
			}
			var stored paymentSession
			if err := json.Unmarshal([]byte(raw), &stored); err != nil || stored != session {
				t.Fatalf("stored JSON = %q, err = %v", raw, err)
			}
			mr.FastForward(ttl)
			if _, ok := svc.loadPaymentSession(ctx, orderID); ok {
				t.Fatal("session survives its TTL")
			}
		})
	}
}
