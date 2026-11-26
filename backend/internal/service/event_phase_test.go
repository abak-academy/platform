package service

import (
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func wib(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, jakarta)
}

func tp(t time.Time) *time.Time { return &t }

func sp(s string) *string { return &s }

func TestMilestoneState_Boundaries(t *testing.T) {
	item := func(start string, end *string) model.ProductGroupingScheduleItem {
		return model.ProductGroupingScheduleItem{StartsOn: start, EndsOn: end}
	}
	cases := []struct {
		name  string
		it    model.ProductGroupingScheduleItem
		today string
		want  string
	}{
		{"today is start of a range", item("2026-12-10", sp("2026-12-12")), "2026-12-10", MilestoneOngoing},
		{"today is end of a range", item("2026-12-10", sp("2026-12-12")), "2026-12-12", MilestoneOngoing},
		{"day after end", item("2026-12-10", sp("2026-12-12")), "2026-12-13", MilestoneDone},
		{"day before start", item("2026-12-10", sp("2026-12-12")), "2026-12-09", MilestoneUpcoming},
		{"no end, today is start", item("2026-12-10", nil), "2026-12-10", MilestoneOngoing},
		{"no end, day after start", item("2026-12-10", nil), "2026-12-11", MilestoneDone},
		{"no end, before start", item("2026-12-10", nil), "2026-12-09", MilestoneUpcoming},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := milestoneState(c.it, c.today); got != c.want {
				t.Fatalf("milestoneState = %q, want %q", got, c.want)
			}
		})
	}
}

func TestNextMilestone_SkipsDoneInDateOrder(t *testing.T) {
	items := []model.ProductGroupingScheduleItem{
		{Label: "Babak final", StartsOn: "2026-12-20"},
		{Label: "Pendaftaran", StartsOn: "2026-11-01", EndsOn: sp("2026-11-30")},
		{Label: "Technical meeting", StartsOn: "2026-12-05"},
	}
	got, state := nextMilestone(items, wib(2026, 12, 1, 9, 0))
	if got == nil || got.Label != "Technical meeting" || state != MilestoneUpcoming {
		t.Fatalf("next = %+v %q, want Technical meeting upcoming", got, state)
	}
	if got, _ := nextMilestone(items, wib(2027, 1, 1, 9, 0)); got != nil {
		t.Fatalf("next after all done = %+v, want nil", got)
	}
}

func exam(start, end *time.Time) repository.EditionExam {
	return repository.EditionExam{ScheduledAt: start, ScheduledEndAt: end}
}

func TestEditionPhase_Boundaries(t *testing.T) {
	start := wib(2026, 12, 10, 8, 0)
	end := wib(2026, 12, 10, 12, 0)
	withEnd := aggregateEditionSchedule([]repository.EditionExam{exam(&start, &end)})
	noEnd := aggregateEditionSchedule([]repository.EditionExam{exam(&start, nil)})
	late := wib(2026, 12, 10, 23, 30)
	lateNoEnd := aggregateEditionSchedule([]repository.EditionExam{exam(&late, nil)})
	early := wib(2026, 12, 10, 0, 30)
	earlyNoEnd := aggregateEditionSchedule([]repository.EditionExam{exam(&early, nil)})

	cases := []struct {
		name     string
		s        editionSchedule
		inWindow bool
		now      time.Time
		want     string
	}{
		{"before start, in window", withEnd, true, start.Add(-time.Second), PhaseOpen},
		{"before start, window closed", withEnd, false, start.Add(-time.Second), PhaseClosed},
		{"now == scheduled_at is live", withEnd, true, start, PhaseLive},
		{"live even when window closed", withEnd, false, start, PhaseLive},
		{"now == scheduled_end_at still live", withEnd, true, end, PhaseLive},
		{"just after scheduled_end_at is done", withEnd, true, end.Add(time.Nanosecond), PhaseDone},
		{"done wins over open window", withEnd, true, end.Add(time.Hour), PhaseDone},
		{"no end: live later the same Jakarta day", noEnd, true, wib(2026, 12, 10, 23, 59), PhaseLive},
		{"no end: done at next Jakarta midnight", noEnd, true, wib(2026, 12, 11, 0, 0), PhaseDone},
		{"23:30 WIB no end: still live at 23:59", lateNoEnd, true, wib(2026, 12, 10, 23, 59), PhaseLive},
		{"23:30 WIB no end: done at 00:00", lateNoEnd, true, wib(2026, 12, 11, 0, 0), PhaseDone},
		{"00:30 WIB no end: live at 00:30", earlyNoEnd, true, early, PhaseLive},
		{"00:30 WIB no end: done at next midnight", earlyNoEnd, true, wib(2026, 12, 11, 0, 0), PhaseDone},
		{"unscheduled in window is open", editionSchedule{}, true, start, PhaseOpen},
		{"unscheduled out of window is closed", editionSchedule{}, false, start, PhaseClosed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := editionPhase(c.s, c.inWindow, c.now); got != c.want {
				t.Fatalf("editionPhase = %q, want %q", got, c.want)
			}
		})
	}
}

func TestEditionPhase_MultiExamUsesEarliestStartAndLatestEnd(t *testing.T) {
	s1, e1 := wib(2026, 12, 10, 8, 0), wib(2026, 12, 10, 10, 0)
	s2, e2 := wib(2026, 12, 12, 8, 0), wib(2026, 12, 12, 10, 0)
	s := aggregateEditionSchedule([]repository.EditionExam{exam(&s2, &e2), exam(&s1, &e1), exam(nil, nil)})
	if !s.Start.Equal(s1) || !s.End.Equal(e2) {
		t.Fatalf("schedule = %v..%v, want %v..%v", s.Start, s.End, s1, e2)
	}
	if got := editionPhase(s, true, wib(2026, 12, 11, 9, 0)); got != PhaseLive {
		t.Fatalf("between the two exams = %q, want live", got)
	}
	if got := editionPhase(s, true, e2.Add(time.Minute)); got != PhaseDone {
		t.Fatalf("after last exam = %q, want done", got)
	}
}

func TestEditionPhase_MixedEndAndNoEndStaysLiveUntilLastExam(t *testing.T) {
	earlyStart, earlyEnd := wib(2026, 12, 10, 8, 0), wib(2026, 12, 10, 10, 0)
	lateStart := wib(2026, 12, 12, 9, 0)
	s := aggregateEditionSchedule([]repository.EditionExam{exam(&earlyStart, &earlyEnd), exam(&lateStart, nil)})
	if got := editionPhase(s, true, wib(2026, 12, 12, 20, 0)); got != PhaseLive {
		t.Fatalf("phase = %q, want live (second exam has no end)", got)
	}
}

func TestIsOpenEdition(t *testing.T) {
	now := wib(2026, 12, 1, 9, 0)
	base := model.Product{Type: "exam", Status: "published"}
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	with := func(f func(p *model.Product)) model.Product { p := base; f(&p); return p }

	cases := []struct {
		name  string
		p     model.Product
		phase string
		want  bool
	}{
		{"published exam, open", base, PhaseOpen, true},
		{"published exam, live", base, PhaseLive, true},
		{"unpublished", with(func(p *model.Product) { p.Status = "draft" }), PhaseOpen, false},
		{"not yet available", with(func(p *model.Product) { p.AvailableFrom = &future }), PhaseOpen, false},
		{"availability ended", with(func(p *model.Product) { p.AvailableUntil = &past }), PhaseOpen, false},
		{"available window spans now", with(func(p *model.Product) { p.AvailableFrom, p.AvailableUntil = &past, &future }), PhaseOpen, true},
		{"closed phase", base, PhaseClosed, false},
		{"done phase", base, PhaseDone, false},
		{"non-exam product", with(func(p *model.Product) { p.Type = "book" }), PhaseOpen, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isOpenEdition(c.p, c.phase, now); got != c.want {
				t.Fatalf("isOpenEdition = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSortOpenEditions_LiveFirstThenDateUnscheduledLastThenTitle(t *testing.T) {
	d1, d2 := wib(2026, 12, 5, 8, 0), wib(2026, 12, 9, 8, 0)
	eds := []PublicEdition{
		{Title: "Z-unscheduled", Phase: PhaseOpen},
		{Title: "B-open-d2", Phase: PhaseOpen, ScheduledAt: &d2},
		{Title: "A-unscheduled", Phase: PhaseOpen},
		{Title: "C-live-d2", Phase: PhaseLive, ScheduledAt: &d2},
		{Title: "D-open-d1", Phase: PhaseOpen, ScheduledAt: &d1},
		{Title: "A-open-d2", Phase: PhaseOpen, ScheduledAt: &d2},
		{Title: "E-live-d1", Phase: PhaseLive, ScheduledAt: &d1},
	}
	sortOpenEditions(eds)
	want := []string{"E-live-d1", "C-live-d2", "D-open-d1", "A-open-d2", "B-open-d2", "A-unscheduled", "Z-unscheduled"}
	for i, w := range want {
		if eds[i].Title != w {
			t.Fatalf("order[%d] = %q, want %q (full: %v)", i, eds[i].Title, w, titles(eds))
		}
	}
}

func titles(eds []PublicEdition) []string {
	out := make([]string, len(eds))
	for i, e := range eds {
		out[i] = e.Title
	}
	return out
}
