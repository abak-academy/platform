package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func tmBool(b bool) *bool { return &b }

func tmStr(s string) *string { return &s }

func tmKey(s string) NullableString { return NullableString{Set: true, Value: &s} }

func tmContent(enQuote string, achievements ...string) *map[string]model.TestimonialContent {
	return &map[string]model.TestimonialContent{
		"id": {Quote: "Kutipan", Achievements: []string{}},
		"en": {Quote: enQuote, Achievements: achievements},
	}
}

func tmActor(t *testing.T, repo *repository.Repository) string {
	t.Helper()
	email := "tm-" + uniqueSuffix() + "@test.com"
	u := &model.User{Email: &email, Name: "Super " + uniqueSuffix(), PasswordHash: "x", Role: RoleSuperAdmin, Status: "active", AuthProvider: "google"}
	if err := repo.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u.ID
}

func tmFull() TestimonialPatch {
	return TestimonialPatch{
		Name: tmStr("Ayu " + uniqueSuffix()), RoleLabel: tmStr("SMA Negeri 1"),
		PhotoKey: tmKey("question/ayu.jpg"), Content: tmContent("Great"),
		HasConsent: tmBool(true), ConsentNote: NullableString{Set: true, Value: tmStr("Formulir WA")},
	}
}

func tmCreate(t *testing.T, svc *Service, actor string, p TestimonialPatch) *model.Testimonial {
	t.Helper()
	got, err := svc.AdminCreateTestimonial(context.Background(), actor, p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return got
}

type tmAudit struct {
	Action   string
	Old, New any
}

func tmAuditRows(t *testing.T, repo *repository.Repository, actor, id string) []tmAudit {
	t.Helper()
	rows, _, err := repo.ListAuditLog(context.Background(), repository.AuditLogFilter{ActorID: actor, TargetType: "testimonial", Limit: 100})
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	var out []tmAudit
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].TargetID != id {
			continue
		}
		var meta struct{ Old, New any }
		if rows[i].Metadata != nil {
			if err := json.Unmarshal(*rows[i].Metadata, &meta); err != nil {
				t.Fatalf("metadata: %v", err)
			}
		}
		out = append(out, tmAudit{rows[i].Action, meta.Old, meta.New})
	}
	return out
}

func tmMissing(t *testing.T, err error) []string {
	t.Helper()
	var inc *TestimonialIncompleteError
	if !errors.Is(err, ErrTestimonialIncomplete) || !errors.As(err, &inc) {
		t.Fatalf("want TestimonialIncompleteError, got %v", err)
	}
	return inc.Missing
}

func TestAdminTestimonial_VisibleRejectedPerMissingPart(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := tmActor(t, repo)
	ctx := context.Background()

	cases := []struct {
		name    string
		mutate  func(p *TestimonialPatch)
		missing []string
	}{
		{"no consent", func(p *TestimonialPatch) { p.HasConsent = nil }, []string{"has_consent"}},
		{"no note", func(p *TestimonialPatch) { p.ConsentNote = NullableString{} }, []string{"consent_note"}},
		{"no en quote", func(p *TestimonialPatch) { p.Content = tmContent("") }, []string{"content.en.quote"}},
		{"no id quote", func(p *TestimonialPatch) {
			p.Content = &map[string]model.TestimonialContent{"id": {Quote: " "}, "en": {Quote: "Great"}}
		}, []string{"content.id.quote"}},
		{"no photo", func(p *TestimonialPatch) { p.PhotoKey = NullableString{} }, []string{"photo_key"}},
		{"all missing in order", func(p *TestimonialPatch) {
			p.HasConsent, p.ConsentNote, p.Content, p.PhotoKey = nil, NullableString{}, nil, NullableString{}
		}, []string{"has_consent", "consent_note", "content.id.quote", "content.en.quote", "photo_key"}},
	}
	for _, c := range cases {
		t.Run("create "+c.name, func(t *testing.T) {
			p := tmFull()
			c.mutate(&p)
			p.IsVisible = tmBool(true)
			_, err := svc.AdminCreateTestimonial(ctx, actor, p)
			if got := tmMissing(t, err); !slices.Equal(got, c.missing) {
				t.Errorf("missing = %v, want %v", got, c.missing)
			}
		})
		t.Run("patch "+c.name, func(t *testing.T) {
			draft := tmCreate(t, svc, actor, TestimonialPatch{Name: tmStr("Draft"), RoleLabel: tmStr("Guru")})
			p := tmFull()
			c.mutate(&p)
			p.IsVisible = tmBool(true)
			_, err := svc.AdminUpdateTestimonial(ctx, actor, draft.ID, p)
			if got := tmMissing(t, err); !slices.Equal(got, c.missing) {
				t.Errorf("missing = %v, want %v", got, c.missing)
			}
			after, _ := repo.GetTestimonial(ctx, draft.ID)
			if after.IsVisible {
				t.Error("rejected patch must not publish")
			}
		})
	}
}

func TestAdminTestimonial_ConsentOffAndPhotoRemovalHide(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := tmActor(t, repo)
	ctx := context.Background()

	t.Run("consent false hides in the same write", func(t *testing.T) {
		tm := tmCreate(t, svc, actor, func() TestimonialPatch { p := tmFull(); p.IsVisible = tmBool(true); return p }())
		got, err := svc.AdminUpdateTestimonial(ctx, actor, tm.ID, TestimonialPatch{HasConsent: tmBool(false)})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if got.IsVisible || got.HasConsent || got.ConsentBy != nil || got.ConsentAt != nil {
			t.Errorf("want hidden and consent cleared, got visible=%v consent=%v by=%v at=%v", got.IsVisible, got.HasConsent, got.ConsentBy, got.ConsentAt)
		}
	})
	t.Run("photo null hides in the same write", func(t *testing.T) {
		tm := tmCreate(t, svc, actor, func() TestimonialPatch { p := tmFull(); p.IsVisible = tmBool(true); return p }())
		got, err := svc.AdminUpdateTestimonial(ctx, actor, tm.ID, TestimonialPatch{PhotoKey: NullableString{Set: true}})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if got.IsVisible || got.PhotoKey != nil {
			t.Errorf("want hidden with no photo, got visible=%v photo=%v", got.IsVisible, got.PhotoKey)
		}
	})
	t.Run("absent fields stay unchanged", func(t *testing.T) {
		tm := tmCreate(t, svc, actor, func() TestimonialPatch { p := tmFull(); p.IsVisible = tmBool(true); return p }())
		got, err := svc.AdminUpdateTestimonial(ctx, actor, tm.ID, TestimonialPatch{SortOrder: new(int)})
		if err != nil || !got.IsVisible || got.PhotoKey == nil || !got.HasConsent {
			t.Errorf("unrelated patch changed state: %v %+v", err, got)
		}
	})
}

func TestAdminTestimonial_ConsentFieldsAreServerSet(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := tmActor(t, repo)
	ctx := context.Background()

	tm := tmCreate(t, svc, actor, tmFull())
	if tm.ConsentBy == nil || *tm.ConsentBy != actor || tm.ConsentAt == nil || tm.ConsentByName == nil {
		t.Fatalf("consent_by/at not set by server: %+v", tm)
	}
	firstAt := *tm.ConsentAt

	other := tmActor(t, repo)
	got, err := svc.AdminUpdateTestimonial(ctx, other, tm.ID, TestimonialPatch{HasConsent: tmBool(true), RoleLabel: tmStr("Baru")})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if *got.ConsentBy != actor || !got.ConsentAt.Equal(firstAt) {
		t.Errorf("already-true consent must keep its original by/at, got %v %v", *got.ConsentBy, got.ConsentAt)
	}

	got, err = svc.AdminUpdateTestimonial(ctx, other, tm.ID, TestimonialPatch{HasConsent: tmBool(false)})
	if err != nil || got.ConsentBy != nil || got.ConsentAt != nil {
		t.Fatalf("consent off must clear by/at: %v %+v", err, got)
	}
	got, err = svc.AdminUpdateTestimonial(ctx, other, tm.ID, TestimonialPatch{HasConsent: tmBool(true)})
	if err != nil || got.ConsentBy == nil || *got.ConsentBy != other {
		t.Fatalf("consent on again must record the new actor: %v %+v", err, got)
	}
}

func TestAdminTestimonial_InputValidation(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := tmActor(t, repo)
	ctx := context.Background()

	bad := map[string]TestimonialPatch{
		"photo outside question/": {PhotoKey: tmKey("avatars/x.jpg")},
		"photo with ..":           {PhotoKey: tmKey("question/../avatars/x.jpg")},
		"photo is a URL":          {PhotoKey: tmKey("https://evil.example/x.jpg")},
		"unknown locale": {Content: &map[string]model.TestimonialContent{
			"fr": {Quote: "x"},
		}},
		"4 achievements": {Content: tmContent("q", "a", "b", "c", "d")},
		"empty name":     {Name: tmStr("  ")},
	}
	for name, p := range bad {
		t.Run(name, func(t *testing.T) {
			base := TestimonialPatch{Name: tmStr("N"), RoleLabel: tmStr("R")}
			tm := tmCreate(t, svc, actor, base)
			if _, err := svc.AdminUpdateTestimonial(ctx, actor, tm.ID, p); !errors.Is(err, ErrValidation) {
				t.Errorf("update err = %v, want ErrValidation", err)
			}
			if p.Name == nil {
				p.Name = tmStr("N")
			}
			p.RoleLabel = tmStr("R")
			if _, err := svc.AdminCreateTestimonial(ctx, actor, p); !errors.Is(err, ErrValidation) {
				t.Errorf("create err = %v, want ErrValidation", err)
			}
		})
	}

	t.Run("3 achievements ok", func(t *testing.T) {
		p := TestimonialPatch{Name: tmStr("N"), RoleLabel: tmStr("R"), Content: tmContent("q", "a", "b", "c")}
		got := tmCreate(t, svc, actor, p)
		if len(got.Content["en"].Achievements) != 3 {
			t.Errorf("achievements = %v", got.Content["en"].Achievements)
		}
	})
	t.Run("unknown id and bad uuid", func(t *testing.T) {
		if _, err := svc.AdminUpdateTestimonial(ctx, actor, "00000000-0000-0000-0000-000000000000", TestimonialPatch{}); !errors.Is(err, ErrTestimonialNotFound) {
			t.Errorf("err = %v, want ErrTestimonialNotFound", err)
		}
		if err := svc.AdminDeleteTestimonial(ctx, actor, "nope"); !errors.Is(err, ErrInvalidUUID) {
			t.Errorf("err = %v, want ErrInvalidUUID", err)
		}
	})
}

func TestAdminTestimonial_AuditRows(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := tmActor(t, repo)
	ctx := context.Background()

	p := tmFull()
	p.IsVisible = tmBool(true)
	tm := tmCreate(t, svc, actor, p)
	if _, err := svc.AdminUpdateTestimonial(ctx, actor, tm.ID, TestimonialPatch{RoleLabel: tmStr("tanpa perubahan flag")}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdminUpdateTestimonial(ctx, actor, tm.ID, TestimonialPatch{HasConsent: tmBool(false)}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AdminDeleteTestimonial(ctx, actor, tm.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetTestimonial(ctx, tm.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("row still present: %v", err)
	}

	got := tmAuditRows(t, repo, actor, tm.ID)
	want := []tmAudit{
		{"testimonial.consent_change", false, true},
		{"testimonial.visibility_change", false, true},
		{"testimonial.consent_change", true, false},
		{"testimonial.visibility_change", true, false},
	}
	if len(got) != len(want)+1 {
		t.Fatalf("audit rows = %+v, want %d consent/visibility rows + delete", got, len(want))
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("missing audit row %+v in %+v", w, got)
		}
	}
	del := got[len(got)-1]
	if del.Action != "testimonial.delete" {
		t.Errorf("last audit action = %q, want testimonial.delete", del.Action)
	}
	old, ok := del.Old.(map[string]any)
	if !ok || old["name"] != tm.Name || old["has_consent"] != false || old["is_visible"] != false {
		t.Errorf("delete audit old = %#v, want the row's name and flags as last saved", del.Old)
	}
	if del.New != nil {
		t.Errorf("delete audit new = %#v, want nil", del.New)
	}
}

func TestAdminTestimonial_ConcurrentPatchesSerialize(t *testing.T) {
	svc, repo := newRealDBService(t)
	actor := tmActor(t, repo)
	ctx := context.Background()

	p := tmFull()
	p.IsVisible = tmBool(true)
	tm := tmCreate(t, svc, actor, p)

	holder, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := repo.GetTestimonialForUpdateTx(ctx, holder, tm.ID); err != nil {
		t.Fatal(err)
	}

	three := 3
	patches := []TestimonialPatch{
		{PhotoKey: NullableString{Set: true}},
		{SortOrder: &three},
	}
	errs := make([]error, len(patches))
	var wg sync.WaitGroup
	for i := range patches {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = svc.AdminUpdateTestimonial(ctx, actor, tm.ID, patches[i])
		}()
	}
	time.Sleep(500 * time.Millisecond)
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("patch %d: %v", i, err)
		}
	}
	got, err := repo.GetTestimonial(ctx, tm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsVisible || got.PhotoKey != nil || got.SortOrder != 3 {
		t.Errorf("lost update: visible=%v photo=%v sort=%d, want hidden, no photo, sort 3", got.IsVisible, got.PhotoKey, got.SortOrder)
	}
}
