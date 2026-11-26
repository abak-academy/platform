package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

type eventSeriesEnv struct {
	pool *pgxpool.Pool
	repo *repository.Repository
	e    *echo.Echo
}

func newEventSeriesEnv(t *testing.T) *eventSeriesEnv {
	t.Helper()
	pool := newTemplatedPool(t)
	store := repository.New(pool)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{JWTSecret: "event-series-test", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, OTPTTL: time.Minute}
	svc := service.NewWithStore(store, store, rdb, nil, &service.NoopOTPProvider{}, &service.NoopEmailProvider{},
		&service.NoopPaymentClient{}, &service.NoopLogisticsClient{}, nil, cfg, nil)
	h := handler.New(svc)
	e := echo.New()
	e.HideBanner = true
	v1 := e.Group("/api/v1")
	v1.GET("/event-series", h.ListEventSeries)
	v1.GET("/event-series/:slug", h.GetEventSeries)
	t.Cleanup(pool.Close)
	return &eventSeriesEnv{pool: pool, repo: store, e: e}
}

func (env *eventSeriesEnv) seedEvent(t *testing.T, slug, prefix string, visible bool) string {
	t.Helper()
	narrative := func(n string) model.EventNarrative {
		return model.EventNarrative{FullName: n, Tagline: n + " tag", Story: []string{"s"}, Rounds: []string{}, Categories: []model.EventCategory{}, Facts: []model.EventFact{}}
	}
	g := model.ProductGrouping{
		Name: "Event " + slug, Slug: slug, Status: model.ProductGroupingStatusPublished, Tone: "brand", Illustration: "trophy",
		IsVisible: visible, ParticipantPrefix: &prefix,
		Content: map[string]model.EventNarrative{"id": narrative("Nama Indonesia"), "en": narrative("English Name")},
	}
	if err := env.repo.CreateProductGrouping(context.Background(), &g); err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	return g.ID
}

func (env *eventSeriesEnv) get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestEventSeriesHandler_ListAndLangAndCache(t *testing.T) {
	env := newEventSeriesEnv(t)
	ctx := context.Background()
	gid := env.seedEvent(t, "shown", "SHWN", true)
	env.seedEvent(t, "secret", "SCRT", false)
	if _, err := env.pool.Exec(ctx, `INSERT INTO product_grouping_schedule (grouping_id, label, label_en, starts_on) VALUES ($1,'Final','Finals','2099-01-01')`, gid); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	var pid string
	if err := env.pool.QueryRow(ctx, `INSERT INTO product (type, name, price, status, grouping_id) VALUES ('exam','Edisi Satu',0,'published',$1) RETURNING id`, gid).Scan(&pid); err != nil {
		t.Fatal(err)
	}

	rec := env.get("/api/v1/event-series")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0]["slug"] != "shown" {
		t.Fatalf("list = %v, want only the visible event", body.Data)
	}
	if body.Data[0]["full_name"] != "Nama Indonesia" {
		t.Fatalf("default lang full_name = %v, want Indonesian", body.Data[0]["full_name"])
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("hidden event leaked into list")
	}
	eds := body.Data[0]["open_editions"].([]any)
	if len(eds) != 1 || eds[0].(map[string]any)["phase"] != "open" {
		t.Fatalf("open_editions = %v", eds)
	}
	for _, banned := range []string{"count", "registration", "sales", "student", "participant_prefix", "is_visible"} {
		if strings.Contains(rec.Body.String(), banned) {
			t.Fatalf("response contains %q: %s", banned, rec.Body)
		}
	}

	en := env.get("/api/v1/event-series?lang=en")
	var enBody struct {
		Data []struct {
			FullName string `json:"full_name"`
			Schedule []struct {
				Label string `json:"label"`
				State string `json:"state"`
			} `json:"schedule"`
		} `json:"data"`
	}
	if err := json.Unmarshal(en.Body.Bytes(), &enBody); err != nil {
		t.Fatal(err)
	}
	if enBody.Data[0].FullName != "English Name" || enBody.Data[0].Schedule[0].Label != "Finals" || enBody.Data[0].Schedule[0].State != "upcoming" {
		t.Fatalf("en list = %+v", enBody.Data[0])
	}
	if id := env.get("/api/v1/event-series?lang=xx"); !strings.Contains(id.Body.String(), "Nama Indonesia") {
		t.Fatalf("unknown lang must serve Indonesian: %s", id.Body)
	}
}

func TestEventSeriesHandler_GetBySlug(t *testing.T) {
	env := newEventSeriesEnv(t)
	env.seedEvent(t, "shown", "SHWN", true)
	env.seedEvent(t, "secret", "SCRT", false)

	ok := env.get("/api/v1/event-series/shown?lang=en")
	if ok.Code != http.StatusOK || ok.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("status=%d cache=%q", ok.Code, ok.Header().Get("Cache-Control"))
	}
	var ev map[string]any
	if err := json.Unmarshal(ok.Body.Bytes(), &ev); err != nil {
		t.Fatal(err)
	}
	if ev["slug"] != "shown" || ev["full_name"] != "English Name" {
		t.Fatalf("event = %v", ev)
	}
	for _, path := range []string{"/api/v1/event-series/secret", "/api/v1/event-series/nope"} {
		rec := env.get(path)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
		if rec.Header().Get("Cache-Control") != "" {
			t.Fatalf("%s must not be cacheable", path)
		}
	}
}
