package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

func TestTestimonialsPublic_ListShapeLangAndCache(t *testing.T) {
	pool := newTemplatedPool(t)
	t.Cleanup(pool.Close)
	store := repository.New(pool)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{JWTSecret: "testimonials-test", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, OTPTTL: time.Minute}
	svc := service.NewWithStore(store, store, rdb, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{}, nil, cfg, nil)
	h := handler.New(svc)
	e := echo.New()
	e.GET("/api/v1/testimonials", h.ListTestimonials)

	photo := "question/u/p.jpg"
	note := "izin tertulis"
	mk := func(name string, visible bool, sort int, enChips []string) {
		tm := model.Testimonial{
			Name: name, RoleLabel: "Alumni", PhotoKey: &photo, IsVisible: visible, SortOrder: sort,
			HasConsent: true, ConsentNote: &note,
			Content: map[string]model.TestimonialContent{
				"id": {Quote: "Kutipan " + name, Achievements: []string{"Juara"}},
				"en": {Quote: "Quote " + name, Achievements: enChips},
			},
		}
		if err := store.CreateTestimonial(context.Background(), &tm); err != nil {
			t.Fatal(err)
		}
	}
	mk("Second", true, 2, []string{})
	mk("First", true, 1, []string{"Gold"})
	mk("Hidden", false, 0, []string{"x"})

	get := func(path string) (*httptest.ResponseRecorder, []map[string]any) {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		var body struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v body=%s", err, rec.Body)
		}
		return rec, body.Data
	}

	rec, data := get("/api/v1/testimonials")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("status=%d cc=%q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if len(data) != 2 || data[0]["name"] != "First" || data[1]["name"] != "Second" {
		t.Fatalf("data = %v, want First, Second (draft excluded, sort_order)", data)
	}
	if data[0]["quote"] != "Kutipan First" {
		t.Fatalf("no lang must be Indonesian, got %v", data[0]["quote"])
	}
	want := map[string]bool{"id": true, "name": true, "role_label": true, "photo_url": true, "quote": true, "achievements": true}
	for _, item := range data {
		for k := range item {
			if !want[k] {
				t.Fatalf("unexpected key %q in public item %v", k, item)
			}
		}
		for k := range want {
			if _, ok := item[k]; !ok {
				t.Fatalf("missing key %q in %v", k, item)
			}
		}
		if item["photo_url"] != photo {
			t.Fatalf("photo_url = %v", item["photo_url"])
		}
	}

	_, en := get("/api/v1/testimonials?lang=en")
	if en[0]["quote"] != "Quote First" || en[0]["achievements"].([]any)[0] != "Gold" {
		t.Fatalf("en first = %v", en[0])
	}
	chips, ok := en[1]["achievements"].([]any)
	if !ok || len(chips) != 0 {
		t.Fatalf("empty EN chips must be [] (no id fallback, not null): %v", en[1]["achievements"])
	}
}
