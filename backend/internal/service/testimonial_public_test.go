package service

import (
	"context"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func seedTestimonial(t *testing.T, f *testimonialFixture, name string, visible bool, sort int, photo *string, en model.TestimonialContent) {
	t.Helper()
	note := "izin tertulis"
	tm := model.Testimonial{
		Name: name, RoleLabel: "Alumni", PhotoKey: photo, IsVisible: visible, SortOrder: sort,
		HasConsent: true, ConsentNote: &note,
		Content: map[string]model.TestimonialContent{
			"id": {Quote: "Kutipan " + name, Achievements: []string{"Juara " + name}},
			"en": en,
		},
	}
	if err := f.repo.CreateTestimonial(context.Background(), &tm); err != nil {
		t.Fatalf("create testimonial: %v", err)
	}
}

type testimonialFixture struct {
	svc  *Service
	repo interface {
		CreateTestimonial(context.Context, *model.Testimonial) error
	}
}

func TestPublicTestimonials_VisibleOnlyOrderedLocalised(t *testing.T) {
	svc, repo := newRealDBService(t)
	f := &testimonialFixture{svc: svc, repo: repo}
	ctx := context.Background()
	photo := "question/u/p.jpg"
	suffix := uniqueSuffix()
	seedTestimonial(t, f, "B"+suffix, true, 20, &photo, model.TestimonialContent{Quote: "EN B", Achievements: []string{}})
	seedTestimonial(t, f, "A"+suffix, true, 10, &photo, model.TestimonialContent{Quote: "EN A", Achievements: []string{"Gold A"}})
	seedTestimonial(t, f, "Draft"+suffix, false, 5, &photo, model.TestimonialContent{Quote: "EN D"})

	id, err := svc.ListPublicTestimonials(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	mine := filterByName(id, suffix)
	if len(mine) != 2 || mine[0].Name != "A"+suffix || mine[1].Name != "B"+suffix {
		t.Fatalf("got %+v, want A then B only (draft excluded, sort_order)", mine)
	}
	if mine[0].Quote != "Kutipan A"+suffix || mine[0].Achievements[0] != "Juara A"+suffix {
		t.Fatalf("default lang not Indonesian: %+v", mine[0])
	}
	if mine[0].PhotoURL != photo {
		t.Fatalf("photo_url = %q, want %q", mine[0].PhotoURL, photo)
	}

	en, err := svc.ListPublicTestimonials(ctx, "en")
	if err != nil {
		t.Fatal(err)
	}
	mine = filterByName(en, suffix)
	if mine[0].Quote != "EN A" || len(mine[0].Achievements) != 1 || mine[0].Achievements[0] != "Gold A" {
		t.Fatalf("en A = %+v", mine[0])
	}
	if mine[1].Achievements == nil || len(mine[1].Achievements) != 0 {
		t.Fatalf("empty EN chips must be [] not nil/id fallback: %#v", mine[1].Achievements)
	}

	fr, _ := svc.ListPublicTestimonials(ctx, "fr")
	if got := filterByName(fr, suffix); got[0].Quote != "Kutipan A"+suffix {
		t.Fatalf("unknown lang must fall back to id, got %q", got[0].Quote)
	}
}

func filterByName(in []PublicTestimonial, suffix string) []PublicTestimonial {
	out := []PublicTestimonial{}
	for _, p := range in {
		if strings.HasSuffix(p.Name, suffix) {
			out = append(out, p)
		}
	}
	return out
}
