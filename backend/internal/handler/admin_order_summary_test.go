package handler_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/repository"
)

// moneyTags are JSON field names that must never reach admin_store. Note
// `total` is absent deliberately: OrderBucketCounts.Total is an order count,
// not an amount.
var moneyTags = []string{
	"product_revenue", "revenue", "shipping_total", "discount_total",
	"subtotal", "unit_price", "price", "jumlah", "amount",
}

func jsonTagsOf(t *testing.T, v interface{}) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	tags := []string{}
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		tags = append(tags, strings.Split(tag, ",")[0])
	}
	return tags
}

// The strict rule: admin_store fulfils orders but must never receive a revenue
// figure. Enforcing it in the type — a separate projection with no money field —
// beats hiding it at render time, which still ships the bytes.
func TestOrderSummaryProduct_carriesNoMoneyField(t *testing.T) {
	tags := jsonTagsOf(t, handler.OrderSummaryProduct{})
	for _, tag := range tags {
		for _, bad := range moneyTags {
			if tag == bad {
				t.Errorf("OrderSummaryProduct exposes money field %q", tag)
			}
		}
	}
}

// Guards the guard. If the check above cannot fail, it proves nothing — so the
// repository type it is protecting against must trip the very same assertion.
func TestMoneyTagCheck_firesOnTheRepositoryType(t *testing.T) {
	tags := jsonTagsOf(t, repository.TopProduct{})
	found := false
	for _, tag := range tags {
		for _, bad := range moneyTags {
			if tag == bad {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("repository.TopProduct no longer carries a money field — " +
			"the money-tag check can no longer fail, so it proves nothing")
	}
}

// Serialising the whole response is the belt to the type check's braces: it
// catches a money field arriving via an embedded or nested struct.
func TestOrderSummaryResponse_serialisesWithoutMoney(t *testing.T) {
	body, err := json.Marshal(handler.OrderSummaryResponse{
		Buckets:     repository.OrderBucketCounts{NeedsConfirm: 4, Total: 1284},
		TopProducts: []handler.OrderSummaryProduct{{Name: "Buku TKA Saintek", QtySold: 124}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, bad := range moneyTags {
		if strings.Contains(string(body), `"`+bad+`"`) {
			t.Errorf("summary payload contains money key %q: %s", bad, body)
		}
	}
}
