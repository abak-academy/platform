package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

type eventFixture struct {
	svc  *Service
	repo *repository.Repository
	g    model.ProductGrouping
}

func fullNarrative(name string) model.EventNarrative {
	return model.EventNarrative{
		FullName: name, Tagline: name + " tagline", LevelsLabel: name + " levels",
		Story: []string{name + " story"}, Rounds: []string{"r1"},
		Categories: []model.EventCategory{{Name: "SD", Detail: "d"}}, Facts: []model.EventFact{{Label: "l", Value: "v"}},
	}
}

func newVisibleEvent(t *testing.T, visible bool) eventFixture {
	t.Helper()
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	suffix := uniqueSuffix()
	g := model.ProductGrouping{
		Name: "Event " + suffix, Slug: "ev-" + suffix, Status: model.ProductGroupingStatusPublished,
		Subjects: []string{"math"}, Tone: "teal", Illustration: "math", IsVisible: visible,
		Content: map[string]model.EventNarrative{"id": fullNarrative("Indonesia"), "en": fullNarrative("English")},
	}
	pfx := randomPrefix()
	g.ParticipantPrefix = &pfx
	logo := "question/u/logo.png"
	g.LogoKey = &logo
	if err := repo.CreateProductGrouping(ctx, &g); err != nil {
		t.Fatalf("create grouping: %v", err)
	}
	return eventFixture{svc: svc, repo: repo, g: g}
}

func randomPrefix() string {
	id := uuid.New()
	b := make([]byte, 6)
	for i := range b {
		b[i] = 'A' + id[i]%26
	}
	return string(b)
}

type editionSeed struct {
	name        string
	productType string
	status      string
	from, until *time.Time
	exams       [][2]*time.Time
}

func (f eventFixture) addEdition(t *testing.T, e editionSeed) string {
	t.Helper()
	ctx := context.Background()
	if e.productType == "" {
		e.productType = "exam"
	}
	if e.status == "" {
		e.status = "published"
	}
	var pid string
	if err := f.repo.Pool().QueryRow(ctx,
		`INSERT INTO product (type, name, price, status, available_from, available_until, grouping_id, stages, grade_min, grade_max, image_url)
		 VALUES ($1,$2,5000,$3,$4,$5,$6,'{SD}',4,6,'product/img.png') RETURNING id`,
		e.productType, e.name, e.status, e.from, e.until, f.g.ID).Scan(&pid); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	for i, x := range e.exams {
		var eid string
		if err := f.repo.Pool().QueryRow(ctx,
			`INSERT INTO exam (title, status, scheduled_at, scheduled_end_at) VALUES ($1,'draft',$2,$3) RETURNING id`,
			e.name+" exam "+string(rune('a'+i)), x[0], x[1]).Scan(&eid); err != nil {
			t.Fatalf("insert exam: %v", err)
		}
		if _, err := f.repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1,$2)`, pid, eid); err != nil {
			t.Fatalf("link exam: %v", err)
		}
	}
	return pid
}

func (f eventFixture) addSchedule(t *testing.T, items ...model.ProductGroupingScheduleItem) {
	t.Helper()
	tx, err := f.repo.Pool().Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err := f.repo.ReplaceGroupingScheduleTx(context.Background(), tx, f.g.ID, items); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func find(events []PublicEvent, slug string) *PublicEvent {
	for i := range events {
		if events[i].Slug == slug {
			return &events[i]
		}
	}
	return nil
}

func TestEventSeriesPublic_OpenEditionsFilterAndOrder(t *testing.T) {
	f := newVisibleEvent(t, true)
	now := wib(2026, 12, 1, 9, 0)
	past, future := now.Add(-48*time.Hour), now.Add(48*time.Hour)
	one := func(s, e *time.Time) [][2]*time.Time { return [][2]*time.Time{{s, e}} }
	liveStart, liveEnd := now.Add(-time.Hour), now.Add(time.Hour)
	d1, d2 := wib(2026, 12, 5, 8, 0), wib(2026, 12, 9, 8, 0)
	doneStart, doneEnd := now.Add(-5*time.Hour), now.Add(-4*time.Hour)

	liveID := f.addEdition(t, editionSeed{name: "Live", exams: one(&liveStart, &liveEnd)})
	f.addEdition(t, editionSeed{name: "Later", exams: one(&d2, nil)})
	f.addEdition(t, editionSeed{name: "Sooner", exams: one(&d1, nil)})
	f.addEdition(t, editionSeed{name: "Unscheduled", exams: one(nil, nil)})
	f.addEdition(t, editionSeed{name: "Draft", status: "draft", exams: one(&d1, nil)})
	f.addEdition(t, editionSeed{name: "NotYet", from: &future, exams: one(&d1, nil)})
	f.addEdition(t, editionSeed{name: "WindowEnded", until: &past, exams: one(&d1, nil)})
	f.addEdition(t, editionSeed{name: "Done", exams: one(&doneStart, &doneEnd)})
	f.addEdition(t, editionSeed{name: "Book", productType: "book"})

	ev, err := f.svc.getPublicEventSeries(context.Background(), f.g.Slug, "id", now)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got := titles(ev.OpenEditions)
	want := []string{"Live", "Sooner", "Later", "Unscheduled"}
	if len(got) != len(want) {
		t.Fatalf("open editions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("open editions = %v, want %v", got, want)
		}
	}
	live := ev.OpenEditions[0]
	if live.ProductID != liveID || live.Phase != PhaseLive || len(live.ExamIDs) != 1 || live.Price != 5000 ||
		live.ImageURL != "product/img.png" || live.GradeMin == nil || *live.GradeMin != 4 || live.Stages[0] != "SD" ||
		!live.ScheduledAt.Equal(liveStart) || !live.ScheduledEndAt.Equal(liveEnd) {
		t.Fatalf("live edition = %+v", live)
	}
	if ev.OpenEditions[3].ScheduledAt != nil || ev.OpenEditions[3].Phase != PhaseOpen {
		t.Fatalf("unscheduled edition = %+v", ev.OpenEditions[3])
	}
}

func TestEventSeriesPublic_LangAndSchedule(t *testing.T) {
	f := newVisibleEvent(t, true)
	now := wib(2026, 12, 6, 9, 0)
	f.addSchedule(t,
		model.ProductGroupingScheduleItem{Label: "Pendaftaran", LabelEN: sp("Registration"), StartsOn: "2026-12-01", EndsOn: sp("2026-12-05")},
		model.ProductGroupingScheduleItem{Label: "Final", LabelEN: sp("Finals"), StartsOn: "2026-12-06"},
		model.ProductGroupingScheduleItem{Label: "Pengumuman", StartsOn: "2026-12-20"},
	)
	ctx := context.Background()

	id, err := f.svc.getPublicEventSeries(ctx, f.g.Slug, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if id.FullName != "Indonesia" || id.Schedule[0].Label != "Pendaftaran" {
		t.Fatalf("default lang = %q / %q, want Indonesian", id.FullName, id.Schedule[0].Label)
	}
	if bogus, _ := f.svc.getPublicEventSeries(ctx, f.g.Slug, "fr", now); bogus.FullName != "Indonesia" {
		t.Fatalf("unknown lang = %q, want Indonesian", bogus.FullName)
	}

	en, err := f.svc.getPublicEventSeries(ctx, f.g.Slug, "en", now)
	if err != nil {
		t.Fatal(err)
	}
	if en.FullName != "English" || en.Tagline != "English tagline" || en.Story[0] != "English story" {
		t.Fatalf("en narrative = %+v", en)
	}
	labels := []string{en.Schedule[0].Label, en.Schedule[1].Label, en.Schedule[2].Label}
	states := []string{en.Schedule[0].State, en.Schedule[1].State, en.Schedule[2].State}
	if labels[0] != "Registration" || labels[1] != "Finals" || labels[2] != "Pengumuman" {
		t.Fatalf("en labels = %v (missing label_en falls back to id)", labels)
	}
	if states[0] != MilestoneDone || states[1] != MilestoneOngoing || states[2] != MilestoneUpcoming {
		t.Fatalf("states = %v", states)
	}
	if en.LogoURL != "question/u/logo.png" || en.Subjects[0] != "math" {
		t.Fatalf("logo/subjects = %q %v", en.LogoURL, en.Subjects)
	}
}

func TestEventSeriesPublic_HiddenEventsNeverReturned(t *testing.T) {
	hidden := newVisibleEvent(t, false)
	shown := newVisibleEvent(t, true)
	ctx := context.Background()
	now := time.Now()

	if _, err := hidden.svc.getPublicEventSeries(ctx, hidden.g.Slug, "id", now); !errors.Is(err, ErrProductGroupingNotFound) {
		t.Fatalf("hidden get err = %v, want ErrProductGroupingNotFound", err)
	}
	if _, err := hidden.svc.getPublicEventSeries(ctx, "no-such-"+uniqueSuffix(), "id", now); !errors.Is(err, ErrProductGroupingNotFound) {
		t.Fatalf("unknown slug err = %v, want ErrProductGroupingNotFound", err)
	}
	list, err := shown.svc.listPublicEventSeries(ctx, "id", now)
	if err != nil {
		t.Fatal(err)
	}
	if find(list, hidden.g.Slug) != nil {
		t.Fatal("hidden event present in list")
	}
	if find(list, shown.g.Slug) == nil {
		t.Fatal("visible event missing from list")
	}
}

func TestEventSeriesPublic_ListFollowsDisplayOrder(t *testing.T) {
	a := newVisibleEvent(t, true)
	b := newVisibleEvent(t, true)
	ctx := context.Background()
	if err := a.repo.ReorderProductGroupings(ctx, []uuid.UUID{uuid.MustParse(b.g.ID), uuid.MustParse(a.g.ID)}); err != nil {
		t.Fatal(err)
	}
	list, err := a.svc.listPublicEventSeries(ctx, "id", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ia, ib := -1, -1
	for i, e := range list {
		switch e.Slug {
		case a.g.Slug:
			ia = i
		case b.g.Slug:
			ib = i
		}
	}
	if ia < 0 || ib < 0 || ib > ia {
		t.Fatalf("order b=%d a=%d, want b before a", ib, ia)
	}
}

func TestEventSeriesSummary_PhasesCountsAndNextMilestone(t *testing.T) {
	f := newVisibleEvent(t, true)
	now := wib(2026, 12, 1, 9, 0)
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	doneS, doneE := now.Add(-5*time.Hour), now.Add(-4*time.Hour)
	d := now.Add(72 * time.Hour)
	liveID := f.addEdition(t, editionSeed{name: "Live", exams: [][2]*time.Time{{&start, &end}}})
	openID := f.addEdition(t, editionSeed{name: "Open", exams: [][2]*time.Time{{&d, nil}}})
	doneID := f.addEdition(t, editionSeed{name: "Done", exams: [][2]*time.Time{{&doneS, &doneE}}})
	draftID := f.addEdition(t, editionSeed{name: "Draft", status: "draft", exams: [][2]*time.Time{{&d, nil}}})
	bookID := f.addEdition(t, editionSeed{name: "Book", productType: "book"})
	f.addSchedule(t,
		model.ProductGroupingScheduleItem{Label: "Lama", StartsOn: "2026-11-01"},
		model.ProductGroupingScheduleItem{Label: "Berikutnya", StartsOn: "2026-12-10"},
	)

	sum, err := f.svc.GroupingEventSummaryAt(context.Background(), f.g.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if sum.EditionCount != 4 || sum.OpenEditionCount != 2 {
		t.Fatalf("counts = %d/%d, want 4/2", sum.EditionCount, sum.OpenEditionCount)
	}
	for id, want := range map[string]string{liveID: PhaseLive, openID: PhaseOpen, doneID: PhaseDone, draftID: PhaseOpen} {
		if sum.Phases[id] != want {
			t.Fatalf("phase[%s] = %q, want %q", id, sum.Phases[id], want)
		}
	}
	if _, ok := sum.Phases[bookID]; ok {
		t.Fatal("non-exam product must not have a phase")
	}
	if sum.NextMilestone == nil || sum.NextMilestone.Label != "Berikutnya" || sum.NextMilestone.State != MilestoneUpcoming {
		t.Fatalf("next milestone = %+v", sum.NextMilestone)
	}
}
