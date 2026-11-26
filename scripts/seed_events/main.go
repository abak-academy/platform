// Seed events — loads docs/prototypes/redesign-v2/event-series-seed.json into
// the admin API (docs/specs/event-series.md § Seed content).
//
// Matches an existing event by slug, then case-insensitive name, and PATCHes it;
// unmatched seed events are POSTed (the API creates them hidden and draft).
// Never sends is_visible, status or participant_prefix.
//
//	ABAK_ADMIN_IDENTIFIER=... ABAK_ADMIN_PASSWORD=... \
//	  go run scripts/seed_events/main.go -base http://localhost:8080 -dry-run
//
// Or set ABAK_ADMIN_TOKEN instead of the identifier/password pair.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type seedEvent struct {
	Slug         string                     `json:"slug"`
	Name         string                     `json:"name"`
	ShortName    *string                    `json:"short_name"`
	Scope        *string                    `json:"scope"`
	Subjects     []string                   `json:"subjects"`
	Tone         string                     `json:"tone"`
	Illustration string                     `json:"illustration"`
	OfficialURL  *string                    `json:"official_url"`
	Content      map[string]json.RawMessage `json:"content"`
	Schedule     []json.RawMessage          `json:"schedule"`
}

type grouping struct {
	ID                string  `json:"id"`
	Slug              string  `json:"slug"`
	Name              string  `json:"name"`
	ParticipantPrefix *string `json:"participant_prefix"`
}

// Einstein Olympiad's hero label is set by hand per the spec; the seed has no short_name.
var shortNameOverrides = map[string]string{"einstein": "Einstein"}

func matchGrouping(s seedEvent, existing []grouping) *grouping {
	for i := range existing {
		if existing[i].Slug == s.Slug {
			return &existing[i]
		}
	}
	for i := range existing {
		if strings.EqualFold(existing[i].Name, s.Name) {
			return &existing[i]
		}
	}
	return nil
}

func eventFields(s seedEvent) map[string]any {
	m := map[string]any{"subjects": s.Subjects}
	if s.Subjects == nil {
		m["subjects"] = []string{}
	}
	if s.Tone != "" {
		m["tone"] = s.Tone
	}
	if s.Illustration != "" {
		m["illustration"] = s.Illustration
	}
	if s.Scope != nil {
		m["scope"] = *s.Scope
	}
	if s.OfficialURL != nil {
		m["official_url"] = *s.OfficialURL
	}
	if short := s.ShortName; short != nil {
		m["short_name"] = *short
	} else if v, ok := shortNameOverrides[s.Slug]; ok {
		m["short_name"] = v
	}
	content := map[string]json.RawMessage{}
	for k, v := range s.Content {
		if string(bytes.TrimSpace(v)) != "null" {
			content[k] = v
		}
	}
	if len(content) > 0 {
		m["content"] = content
	}
	return m
}

func buildCreate(s seedEvent) map[string]any {
	m := eventFields(s)
	m["name"] = s.Name
	return m
}

func buildPatch(s seedEvent) map[string]any {
	m := eventFields(s)
	if len(s.Schedule) > 0 {
		m["schedule"] = s.Schedule
	}
	return m
}

type client struct {
	base, token string
	http        *http.Client
}

func (c *client) do(method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func prefixStr(p *string) string {
	if p == nil {
		return "-"
	}
	return *p
}

func run(base, seedPath string, dryRun bool) error {
	raw, err := os.ReadFile(seedPath)
	if err != nil {
		return err
	}
	var seed []seedEvent
	if err := json.Unmarshal(raw, &seed); err != nil {
		return fmt.Errorf("parse %s: %w", seedPath, err)
	}

	c := &client{base: strings.TrimRight(base, "/"), token: os.Getenv("ABAK_ADMIN_TOKEN"), http: &http.Client{Timeout: 30 * time.Second}}
	if c.token == "" {
		id, pw := os.Getenv("ABAK_ADMIN_IDENTIFIER"), os.Getenv("ABAK_ADMIN_PASSWORD")
		if id == "" || pw == "" {
			return fmt.Errorf("set ABAK_ADMIN_TOKEN, or ABAK_ADMIN_IDENTIFIER and ABAK_ADMIN_PASSWORD")
		}
		var login struct {
			AccessToken string `json:"access_token"`
		}
		if err := c.do("POST", "/api/v1/auth/login", map[string]string{"identifier": id, "password": pw}, &login); err != nil {
			return err
		}
		c.token = login.AccessToken
	}

	var list struct {
		Data []grouping `json:"data"`
	}
	if err := c.do("GET", "/api/v1/admin/product-groupings", nil, &list); err != nil {
		return err
	}

	fmt.Printf("%-20s %-8s %-22s %s\n", "SEED", "RESULT", "SLUG", "PREFIX")
	for _, s := range seed {
		g := matchGrouping(s, list.Data)
		result, slug, prefix := "created", s.Slug, "(derived on create)"
		if g != nil {
			result, slug, prefix = "matched", g.Slug, prefixStr(g.ParticipantPrefix)
		}
		if !dryRun {
			if g == nil {
				var created grouping
				if err := c.do("POST", "/api/v1/admin/product-groupings", buildCreate(s), &created); err != nil {
					return fmt.Errorf("%s: %w", s.Name, err)
				}
				g = &created
				slug, prefix = created.Slug, prefixStr(created.ParticipantPrefix)
			}
			var patched struct {
				grouping
			}
			if err := c.do("PATCH", "/api/v1/admin/product-groupings/"+g.ID, buildPatch(s), &patched); err != nil {
				return fmt.Errorf("%s: %w", s.Name, err)
			}
			slug, prefix = patched.Slug, prefixStr(patched.ParticipantPrefix)
		}
		fmt.Printf("%-20s %-8s %-22s %s\n", s.Name, result, slug, prefix)
	}
	if dryRun {
		fmt.Println("dry run: nothing written")
	}
	return nil
}

func main() {
	base := flag.String("base", os.Getenv("ABAK_BASE_URL"), "API origin, e.g. http://localhost:8080 (or ABAK_BASE_URL)")
	seedPath := flag.String("seed", "docs/prototypes/redesign-v2/event-series-seed.json", "seed file")
	dryRun := flag.Bool("dry-run", false, "print the match table and write nothing")
	flag.Parse()
	if *base == "" {
		fmt.Fprintln(os.Stderr, "-base or ABAK_BASE_URL is required")
		os.Exit(2)
	}
	if err := run(*base, *seedPath, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
