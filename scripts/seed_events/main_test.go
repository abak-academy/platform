package main

import (
	"encoding/json"
	"os"
	"testing"
)

func loadSeed(t *testing.T) []seedEvent {
	t.Helper()
	raw, err := os.ReadFile("../../docs/prototypes/redesign-v2/event-series-seed.json")
	if err != nil {
		t.Fatal(err)
	}
	var seed []seedEvent
	if err := json.Unmarshal(raw, &seed); err != nil {
		t.Fatal(err)
	}
	return seed
}

func bySlug(t *testing.T, seed []seedEvent, slug string) seedEvent {
	t.Helper()
	for _, s := range seed {
		if s.Slug == slug {
			return s
		}
	}
	t.Fatalf("seed has no %q", slug)
	return seedEvent{}
}

func TestMatchGrouping(t *testing.T) {
	existing := []grouping{
		{ID: "1", Slug: "stemco", Name: "Something Else"},
		{ID: "2", Slug: "einstein-olympiad", Name: "EINSTEIN OLYMPIAD"},
		{ID: "3", Slug: "other", Name: "stemco"},
	}
	cases := []struct {
		seed seedEvent
		want string
	}{
		{seedEvent{Slug: "stemco", Name: "STEMCO"}, "1"},
		{seedEvent{Slug: "einstein", Name: "Einstein Olympiad"}, "2"},
		{seedEvent{Slug: "hippo", Name: "HIPPO"}, ""},
	}
	for _, c := range cases {
		got := matchGrouping(c.seed, existing)
		switch {
		case c.want == "" && got != nil:
			t.Errorf("%s: want no match, got %s", c.seed.Slug, got.ID)
		case c.want != "" && (got == nil || got.ID != c.want):
			t.Errorf("%s: want %s, got %v", c.seed.Slug, c.want, got)
		}
	}
}

func TestPayloadsNeverSendProtectedKeys(t *testing.T) {
	seed := loadSeed(t)
	for _, s := range seed {
		for name, p := range map[string]map[string]any{"create": buildCreate(s), "patch": buildPatch(s)} {
			for _, k := range []string{"is_visible", "status", "participant_prefix"} {
				if _, ok := p[k]; ok {
					t.Errorf("%s %s sends %s", s.Slug, name, k)
				}
			}
		}
	}
}

func TestPayloadContent(t *testing.T) {
	seed := loadSeed(t)

	if got := buildPatch(bySlug(t, seed, "einstein"))["short_name"]; got != "Einstein" {
		t.Errorf("einstein short_name = %v", got)
	}
	if _, ok := buildPatch(bySlug(t, seed, "stemco"))["short_name"]; ok {
		t.Error("stemco must not get a short_name")
	}
	if n := len(buildPatch(bySlug(t, seed, "esolio"))["schedule"].([]json.RawMessage)); n != 4 {
		t.Errorf("esolio schedule len = %d, want 4", n)
	}
	if _, ok := buildPatch(bySlug(t, seed, "stemco"))["schedule"]; ok {
		t.Error("empty schedule must be omitted, not sent as a wipe")
	}
	if _, ok := buildCreate(bySlug(t, seed, "esolio"))["schedule"]; ok {
		t.Error("create body has no schedule; it is PATCHed after")
	}

	c := buildPatch(bySlug(t, seed, "inysia"))["content"].(map[string]json.RawMessage)
	if _, ok := c["en"]; ok || c["id"] == nil {
		t.Errorf("inysia content keys wrong: %v", c)
	}
	if buildCreate(bySlug(t, seed, "stemco"))["name"] != "STEMCO" {
		t.Error("create must carry name")
	}
}
