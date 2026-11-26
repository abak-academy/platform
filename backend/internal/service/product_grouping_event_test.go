package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func grpStr(s string) *string { return &s }

func grpBool(b bool) *bool { return &b }

// uniquePrefix is a fresh 6-letter participant prefix; the real-DB fixture is shared and the column is unique.
func uniquePrefix() string {
	hex := strings.ReplaceAll(uuid.NewString(), "-", "")[:6]
	b := []byte(hex)
	for i, c := range b {
		if c >= 'a' {
			b[i] = 'A' + (c - 'a') + 10
		} else {
			b[i] = 'A' + (c - '0')
		}
	}
	return string(b)
}

func eventNarrative(tagline string, story ...string) model.EventNarrative {
	return model.EventNarrative{Tagline: tagline, Story: story}
}

func eventCompleteContent() map[string]model.EventNarrative {
	return map[string]model.EventNarrative{"id": eventNarrative("Tagline", "Cerita"), "en": eventNarrative("Tagline", "Story")}
}

func newEventPublished(t *testing.T, svc *Service) model.ProductGrouping {
	t.Helper()
	g, err := svc.AdminCreateProductGrouping(context.Background(), model.ProductGrouping{Name: "Event " + uniqueSuffix(), ParticipantPrefix: grpStr(uniquePrefix())})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := svc.AdminUpdateProductGrouping(context.Background(), g.ID, ProductGroupingPatch{Status: grpStr(model.ProductGroupingStatusPublished)}); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return g
}

func eventMissing(t *testing.T, err error) []string {
	t.Helper()
	var inc *GroupingIncompleteError
	if !errors.As(err, &inc) {
		t.Fatalf("err = %v, want *GroupingIncompleteError", err)
	}
	return inc.Missing
}

func TestAdminCreateProductGrouping_SlugAndPrefix(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	sfx := uniqueSuffix()

	first, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Olimpiade Sains " + sfx})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if first.Slug != "olimpiade-sains-"+sfx {
		t.Fatalf("Slug = %q, want olimpiade-sains-%s", first.Slug, sfx)
	}
	if first.IsVisible || first.Status != model.ProductGroupingStatusDraft {
		t.Fatalf("new event = visible %v status %q, want hidden draft", first.IsVisible, first.Status)
	}

	second, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Olimpiade Sains " + sfx})
	if err != nil {
		t.Fatalf("create duplicate name: %v", err)
	}
	third, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Olimpiade  Sains  " + sfx + "!!"})
	if err != nil {
		t.Fatalf("create third: %v", err)
	}
	if second.Slug != first.Slug+"-2" || third.Slug != first.Slug+"-3" {
		t.Fatalf("collision slugs = %q, %q; want -2, -3 suffixes of %q", second.Slug, third.Slug, first.Slug)
	}

	symbols, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "!!!"})
	if err != nil || !strings.HasPrefix(symbols.Slug, "event") {
		t.Fatalf("symbols-only slug = %q err=%v, want event[-N]", symbols.Slug, err)
	}
}

func TestAdminCreateProductGrouping_DefaultPrefix(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	p := uniquePrefix()
	g, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: strings.ToLower(p) + " 12 Kompetisi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if g.ParticipantPrefix == nil || *g.ParticipantPrefix != p[:4] {
		t.Fatalf("ParticipantPrefix = %v, want first four letters %q", g.ParticipantPrefix, p[:4])
	}

	again, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: strings.ToLower(p) + " 12 Kompetisi Lain"})
	if err != nil {
		t.Fatalf("create with taken default prefix must still succeed: %v", err)
	}
	if again.ParticipantPrefix != nil {
		t.Fatalf("taken default prefix = %q, want NULL", *again.ParticipantPrefix)
	}

	short, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "7 x 99"})
	if err != nil || short.ParticipantPrefix != nil {
		t.Fatalf("one-letter name prefix = %v err=%v, want NULL", short.ParticipantPrefix, err)
	}

	q := uniquePrefix()
	viaShort, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Nama Panjang Sekali", ShortName: grpStr(q + "zz")})
	if err != nil || viaShort.ParticipantPrefix == nil || *viaShort.ParticipantPrefix != q[:4] {
		t.Fatalf("short_name basis prefix = %v err=%v, want %q", viaShort.ParticipantPrefix, err, q[:4])
	}
}

func TestAdminCreateProductGrouping_ExplicitPrefix(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	p := uniquePrefix()
	if _, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "A " + uniqueSuffix(), ParticipantPrefix: &p}); err != nil {
		t.Fatalf("create with explicit prefix: %v", err)
	}
	if _, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "B " + uniqueSuffix(), ParticipantPrefix: &p}); !errors.Is(err, repository.ErrGroupingPrefixTaken) {
		t.Fatalf("duplicate explicit prefix err = %v, want ErrGroupingPrefixTaken", err)
	}
	if _, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "C " + uniqueSuffix(), ParticipantPrefix: grpStr("abc")}); !errors.Is(err, ErrValidation) {
		t.Fatalf("lowercase explicit prefix err = %v, want ErrValidation", err)
	}
}

func TestAdminUpdateProductGrouping_PrefixFormatAndDuplicate(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	g := newEventPublished(t, svc)
	other := newEventPublished(t, svc)

	for _, bad := range []string{"abcd", "A", "ABCDEFG", "AB1", "A-B"} {
		if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{ParticipantPrefix: grpStr(bad)}); !errors.Is(err, ErrValidation) {
			t.Errorf("prefix %q err = %v, want ErrValidation", bad, err)
		}
	}
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{ParticipantPrefix: other.ParticipantPrefix}); !errors.Is(err, repository.ErrGroupingPrefixTaken) {
		t.Fatalf("duplicate prefix err = %v, want ErrGroupingPrefixTaken", err)
	}
	ok := uniquePrefix()
	got, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{ParticipantPrefix: &ok})
	if err != nil || got.ParticipantPrefix == nil || *got.ParticipantPrefix != ok {
		t.Fatalf("valid prefix update = %+v err=%v", got.ParticipantPrefix, err)
	}
}

func TestAdminUpdateProductGrouping_VisibilityRules(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	draft, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "Draft " + uniqueSuffix(), ParticipantPrefix: grpStr(uniquePrefix())})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.AdminUpdateProductGrouping(ctx, draft.ID, ProductGroupingPatch{IsVisible: grpBool(true), Content: grpPtr(eventCompleteContent())})
	if m := eventMissing(t, err); len(m) != 1 || m[0] != "status" {
		t.Fatalf("draft visibility missing = %v, want [status]", m)
	}

	noPrefix, err := svc.AdminCreateProductGrouping(ctx, model.ProductGrouping{Name: "7 x 9"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.AdminUpdateProductGrouping(ctx, noPrefix.ID, ProductGroupingPatch{Status: grpStr("published"), IsVisible: grpBool(true), Content: grpPtr(eventCompleteContent())})
	if m := eventMissing(t, err); len(m) != 1 || m[0] != "participant_prefix" {
		t.Fatalf("no-prefix visibility missing = %v, want [participant_prefix]", m)
	}

	g := newEventPublished(t, svc)
	_, err = svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{IsVisible: grpBool(true), Content: grpPtr(map[string]model.EventNarrative{
		"id": eventNarrative("Tagline", "Cerita"),
		"en": eventNarrative("  ", " ", ""),
	})})
	if m := eventMissing(t, err); strings.Join(m, ",") != "en.tagline,en.story" {
		t.Fatalf("en-incomplete missing = %v, want [en.tagline en.story]", m)
	}
	_, err = svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{IsVisible: grpBool(true), Content: grpPtr(map[string]model.EventNarrative{"en": eventNarrative("T", "S")})})
	if m := eventMissing(t, err); strings.Join(m, ",") != "id.tagline,id.story" {
		t.Fatalf("id-missing missing = %v, want [id.tagline id.story]", m)
	}

	schedule := []model.ProductGroupingScheduleItem{
		{Label: "Babak 1", LabelEN: grpStr("Round 1"), StartsOn: "2026-12-01"},
		{Label: "Babak 2", StartsOn: "2026-12-05"},
		{Label: "Final", LabelEN: grpStr(" "), StartsOn: "2026-12-09"},
	}
	_, err = svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{IsVisible: grpBool(true), Content: grpPtr(eventCompleteContent()), Schedule: &schedule})
	if m := eventMissing(t, err); strings.Join(m, ",") != "schedule[1].label_en,schedule[2].label_en" {
		t.Fatalf("schedule missing = %v, want [schedule[1].label_en schedule[2].label_en]", m)
	}
	after, _, err := svc.AdminGetProductGrouping(ctx, g.ID)
	if err != nil || after.IsVisible {
		t.Fatalf("rejected visibility must not persist: visible=%v err=%v", after.IsVisible, err)
	}

	schedule[1].LabelEN, schedule[2].LabelEN = grpStr("Round 2"), grpStr("Final")
	got, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{IsVisible: grpBool(true), Content: grpPtr(eventCompleteContent()), Schedule: &schedule})
	if err != nil || !got.IsVisible {
		t.Fatalf("complete event visible = %v err=%v, want true", got.IsVisible, err)
	}

	// A visible event cannot be edited into an incomplete state.
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{ParticipantPrefix: grpStr("")}); eventMissing(t, err)[0] != "participant_prefix" {
		t.Fatalf("clearing the prefix of a visible event err = %v", err)
	}

	// Back to draft hides it in the same write.
	back, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Status: grpStr("draft")})
	if err != nil || back.IsVisible {
		t.Fatalf("draft must force is_visible=false: visible=%v err=%v", back.IsVisible, err)
	}
	persisted, _, _ := svc.AdminGetProductGrouping(ctx, g.ID)
	if persisted.IsVisible {
		t.Fatal("is_visible still true in DB after status=draft")
	}
}

func grpPtr[T any](v T) *T { return &v }

func TestAdminUpdateProductGrouping_FieldValidation(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	g := newEventPublished(t, svc)
	cases := map[string]ProductGroupingPatch{
		"unknown locale key": {Content: grpPtr(map[string]model.EventNarrative{"fr": eventNarrative("a", "b")})},
		"bad scope":          {Scope: grpStr("galactic")},
		"bad tone":           {Tone: grpStr("neon")},
		"bad illustration":   {Illustration: grpStr("dragon")},
		"bad official url":   {OfficialURL: grpStr("javascript:alert(1)")},
		"empty name":         {Name: grpStr(" ")},
		"ends before starts": {Schedule: &[]model.ProductGroupingScheduleItem{{Label: "X", StartsOn: "2026-12-05", EndsOn: grpStr("2026-12-04")}}},
		"bad date":           {Schedule: &[]model.ProductGroupingScheduleItem{{Label: "X", StartsOn: "05-12-2026"}}},
		"empty label":        {Schedule: &[]model.ProductGroupingScheduleItem{{Label: " ", StartsOn: "2026-12-05"}}},
	}
	for name, patch := range cases {
		if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, patch); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
	// Equal start and end is a valid one-day range.
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Schedule: &[]model.ProductGroupingScheduleItem{{Label: "X", StartsOn: "2026-12-05", EndsOn: grpStr("2026-12-05")}}}); err != nil {
		t.Errorf("starts_on == ends_on err = %v, want nil", err)
	}
}

func TestAdminUpdateProductGrouping_ScheduleReplaceIsAtomicAndOrdered(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	g := newEventPublished(t, svc)
	other := newEventPublished(t, svc)

	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Schedule: &[]model.ProductGroupingScheduleItem{
		{Label: "Akhir", StartsOn: "2026-12-20"}, {Label: "Awal", LabelEN: grpStr("Start"), StartsOn: "2026-12-01", EndsOn: grpStr("2026-12-03")},
	}}); err != nil {
		t.Fatalf("set schedule: %v", err)
	}
	d, err := svc.AdminGetProductGroupingDetail(ctx, g.ID)
	if err != nil || len(d.Schedule) != 2 || d.Schedule[0].Label != "Awal" || d.Schedule[1].Label != "Akhir" {
		t.Fatalf("schedule = %+v err=%v, want ordered [Awal Akhir]", d.Schedule, err)
	}
	if d.EnComplete {
		t.Fatal("en_complete = true with an untranslated schedule row and no content")
	}

	// The grouping update fails on a duplicate prefix; the schedule and name must stay as they were.
	_, err = svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{
		Name: grpStr("Renamed"), ParticipantPrefix: other.ParticipantPrefix,
		Schedule: &[]model.ProductGroupingScheduleItem{{Label: "Baru", StartsOn: "2027-01-01"}},
	})
	if !errors.Is(err, repository.ErrGroupingPrefixTaken) {
		t.Fatalf("err = %v, want ErrGroupingPrefixTaken", err)
	}
	d, _ = svc.AdminGetProductGroupingDetail(ctx, g.ID)
	if len(d.Schedule) != 2 || d.Schedule[0].Label != "Awal" || d.Name == "Renamed" {
		t.Fatalf("failed patch leaked: name=%q schedule=%+v", d.Name, d.Schedule)
	}

	// Absent schedule preserves; empty array clears.
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Description: grpStr("x")}); err != nil {
		t.Fatalf("patch without schedule: %v", err)
	}
	if d, _ = svc.AdminGetProductGroupingDetail(ctx, g.ID); len(d.Schedule) != 2 {
		t.Fatalf("absent schedule changed it: %+v", d.Schedule)
	}
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Schedule: &[]model.ProductGroupingScheduleItem{}}); err != nil {
		t.Fatalf("clear schedule: %v", err)
	}
	if d, _ = svc.AdminGetProductGroupingDetail(ctx, g.ID); len(d.Schedule) != 0 {
		t.Fatalf("empty schedule array did not clear: %+v", d.Schedule)
	}
}

func TestAdminGetProductGroupingDetail_EnComplete(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	g := newEventPublished(t, svc)
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Content: grpPtr(eventCompleteContent()), Schedule: &[]model.ProductGroupingScheduleItem{{Label: "A", LabelEN: grpStr("A"), StartsOn: "2026-12-01"}}}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if d, _ := svc.AdminGetProductGroupingDetail(ctx, g.ID); !d.EnComplete {
		t.Fatal("en_complete = false for a complete English side")
	}
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Content: grpPtr(map[string]model.EventNarrative{"id": eventNarrative("T", "S")})}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if d, _ := svc.AdminGetProductGroupingDetail(ctx, g.ID); d.EnComplete {
		t.Fatal("en_complete = true with no English content")
	}
}

func seedEventExam(t *testing.T, repo *repository.Repository, name string) (productID, examID, regID string) {
	t.Helper()
	ctx := context.Background()
	pool := repo.Pool()
	if err := pool.QueryRow(ctx, `INSERT INTO exam (title) VALUES ($1) RETURNING id`, name).Scan(&examID); err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	p := model.Product{Type: "exam", Name: name, Price: 0, Status: "published"}
	if err := repo.CreateProduct(ctx, &p); err != nil {
		t.Fatalf("seed exam product: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, p.ID, examID); err != nil {
		t.Fatalf("link exam: %v", err)
	}
	var studentID string
	if err := pool.QueryRow(ctx, `INSERT INTO users (name, role, status, username, password_hash) VALUES ($1, 'student', 'active', $2, '') RETURNING id`,
		"Siswa "+name, "u_"+uniqueSuffix()).Scan(&studentID); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO exam_registration (student_id, exam_id, token, card_key, participant_number) VALUES ($1, $2, $3, 'cards/x.pdf', 1) RETURNING id`,
		studentID, examID, "tok-"+uuid.NewString()).Scan(&regID); err != nil {
		t.Fatalf("seed registration: %v", err)
	}
	return p.ID, examID, regID
}

func eventCardKey(t *testing.T, repo *repository.Repository, regID string) *string {
	t.Helper()
	var k *string
	if err := repo.Pool().QueryRow(context.Background(), `SELECT card_key FROM exam_registration WHERE id = $1`, regID).Scan(&k); err != nil {
		t.Fatalf("read card_key: %v", err)
	}
	return k
}

func eventRestoreCard(t *testing.T, repo *repository.Repository, regID string) {
	t.Helper()
	if _, err := repo.Pool().Exec(context.Background(), `UPDATE exam_registration SET card_key = 'cards/x.pdf' WHERE id = $1`, regID); err != nil {
		t.Fatalf("restore card_key: %v", err)
	}
}

func TestAdminUpdateProductGrouping_PrefixChangeClearsCachedCards(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	g := newEventPublished(t, svc)
	memberProduct, _, memberReg := seedEventExam(t, repo, "Member "+uniqueSuffix())
	_, _, strangerReg := seedEventExam(t, repo, "Stranger "+uniqueSuffix())
	if _, _, err := svc.AdminSetGroupingProducts(ctx, g.ID, []string{memberProduct}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	eventRestoreCard(t, repo, memberReg)

	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{Name: grpStr("Renamed " + uniqueSuffix())}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if eventCardKey(t, repo, memberReg) == nil {
		t.Fatal("a patch that keeps the prefix must not clear cached cards")
	}

	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{ParticipantPrefix: grpStr(uniquePrefix())}); err != nil {
		t.Fatalf("change prefix: %v", err)
	}
	if eventCardKey(t, repo, memberReg) != nil {
		t.Fatal("prefix change left the member exam's cached card")
	}
	if eventCardKey(t, repo, strangerReg) == nil {
		t.Fatal("prefix change cleared the card of an exam outside the event")
	}

	eventRestoreCard(t, repo, memberReg)
	other := newEventPublished(t, svc)
	if _, err := svc.AdminUpdateProductGrouping(ctx, g.ID, ProductGroupingPatch{ParticipantPrefix: other.ParticipantPrefix}); err == nil {
		t.Fatal("expected duplicate prefix error")
	}
	if eventCardKey(t, repo, memberReg) == nil {
		t.Fatal("a rolled-back prefix change must keep cached cards")
	}
}

func TestAdminSetGroupingProducts_ClearsCardsOfExamsWhoseEventChanges(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	g := newEventPublished(t, svc)
	stayP, _, stayReg := seedEventExam(t, repo, "Stay "+uniqueSuffix())
	goP, _, goReg := seedEventExam(t, repo, "Go "+uniqueSuffix())
	comeP, _, comeReg := seedEventExam(t, repo, "Come "+uniqueSuffix())
	_, _, strangerReg := seedEventExam(t, repo, "Stranger "+uniqueSuffix())
	if _, _, err := svc.AdminSetGroupingProducts(ctx, g.ID, []string{stayP, goP}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	for _, r := range []string{stayReg, goReg, comeReg, strangerReg} {
		eventRestoreCard(t, repo, r)
	}

	if _, _, err := svc.AdminSetGroupingProducts(ctx, g.ID, []string{stayP, comeP}); err != nil {
		t.Fatalf("replace members: %v", err)
	}
	if eventCardKey(t, repo, goReg) != nil {
		t.Error("exam of the removed product kept its cached card")
	}
	if eventCardKey(t, repo, comeReg) != nil {
		t.Error("exam of the added product kept its cached card")
	}
	if eventCardKey(t, repo, stayReg) == nil {
		t.Error("exam of an unchanged member lost its cached card")
	}
	if eventCardKey(t, repo, strangerReg) == nil {
		t.Error("exam outside the event lost its cached card")
	}
}

func TestUpdateProductRelations_ClearsCardsOfExamsMovedInOrOutOfAnEvent(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	g := newEventPublished(t, svc)
	memberP, keepExam, keepReg := seedEventExam(t, repo, "Keep "+uniqueSuffix())
	_, dropExam, dropReg := seedEventExam(t, repo, "Drop "+uniqueSuffix())
	_, addExam, addReg := seedEventExam(t, repo, "Add "+uniqueSuffix())
	_, _, strangerReg := seedEventExam(t, repo, "Stranger "+uniqueSuffix())
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, memberP, dropExam); err != nil {
		t.Fatalf("link drop exam: %v", err)
	}
	if _, _, err := svc.AdminSetGroupingProducts(ctx, g.ID, []string{memberP}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	for _, r := range []string{keepReg, dropReg, addReg, strangerReg} {
		eventRestoreCard(t, repo, r)
	}

	if _, err := svc.UpdateProductRelations(ctx, memberP, []string{keepExam, addExam}, nil, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProductRelations: %v", err)
	}
	if eventCardKey(t, repo, dropReg) != nil {
		t.Error("exam moved out of the event kept its cached card")
	}
	if eventCardKey(t, repo, addReg) != nil {
		t.Error("exam moved into the event kept its cached card")
	}
	if eventCardKey(t, repo, keepReg) == nil || eventCardKey(t, repo, strangerReg) == nil {
		t.Error("exam unaffected by the change lost its cached card")
	}

	// A product outside any event changes nothing.
	loneP, _, loneReg := seedEventExam(t, repo, "Lone "+uniqueSuffix())
	_, swapExam, _ := seedEventExam(t, repo, "Swap "+uniqueSuffix())
	if _, err := svc.UpdateProductRelations(ctx, loneP, []string{swapExam}, nil, RoleSuperAdmin); err != nil {
		t.Fatalf("UpdateProductRelations lone: %v", err)
	}
	if eventCardKey(t, repo, loneReg) == nil {
		t.Error("relations change on an ungrouped product cleared a card")
	}
}

func TestValidateProductLevel(t *testing.T) {
	i := func(v int) *int { return &v }
	cases := []struct {
		name     string
		stages   []string
		min, max *int
		ok       bool
	}{
		{"empty", nil, nil, nil, true},
		{"valid", []string{"SD", "SMP", "SMA"}, i(1), i(12), true},
		{"equal grades", []string{"SD"}, i(4), i(4), true},
		{"unknown stage", []string{"TK"}, nil, nil, false},
		{"lowercase stage", []string{"sd"}, nil, nil, false},
		{"grade 0", nil, i(0), nil, false},
		{"grade 13", nil, nil, i(13), false},
		{"max below min", nil, i(6), i(4), false},
		{"only max", nil, nil, i(6), true},
	}
	for _, c := range cases {
		err := ValidateProductLevel(c.stages, c.min, c.max)
		if c.ok && err != nil || !c.ok && !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}
