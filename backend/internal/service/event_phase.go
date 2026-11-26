package service

import (
	"sort"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

const (
	PhaseOpen   = "open"
	PhaseLive   = "live"
	PhaseClosed = "closed"
	PhaseDone   = "done"

	MilestoneDone     = "done"
	MilestoneOngoing  = "ongoing"
	MilestoneUpcoming = "upcoming"
)

func jakartaDay(t time.Time) string { return t.In(jakarta).Format("2006-01-02") }

// milestoneState compares Jakarta calendar days, so today is "YYYY-MM-DD".
func milestoneState(item model.ProductGroupingScheduleItem, today string) string {
	end := item.StartsOn
	if item.EndsOn != nil {
		end = *item.EndsOn
	}
	switch {
	case end < today:
		return MilestoneDone
	case item.StartsOn <= today:
		return MilestoneOngoing
	}
	return MilestoneUpcoming
}

// nextMilestone is the first non-done milestone in date order, or nil.
func nextMilestone(items []model.ProductGroupingScheduleItem, now time.Time) (*model.ProductGroupingScheduleItem, string) {
	today := jakartaDay(now)
	sorted := sortedSchedule(items)
	for i := range sorted {
		if st := milestoneState(sorted[i], today); st != MilestoneDone {
			return &sorted[i], st
		}
	}
	return nil, ""
}

func sortedSchedule(items []model.ProductGroupingScheduleItem) []model.ProductGroupingScheduleItem {
	out := append([]model.ProductGroupingScheduleItem{}, items...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartsOn != out[j].StartsOn {
			return out[i].StartsOn < out[j].StartsOn
		}
		ei, ej := out[i].StartsOn, out[j].StartsOn
		if out[i].EndsOn != nil {
			ei = *out[i].EndsOn
		}
		if out[j].EndsOn != nil {
			ej = *out[j].EndsOn
		}
		return ei < ej
	})
	return out
}

// editionSchedule is a product's earliest start and latest end across its exams.
type editionSchedule struct {
	Start, End *time.Time
}

// aggregateEditionSchedule: an exam without an end runs to the end of its
// Jakarta day, so a mixed product stays live until its last exam is over.
func aggregateEditionSchedule(exams []repository.EditionExam) editionSchedule {
	var s editionSchedule
	for _, x := range exams {
		if x.ScheduledAt == nil {
			continue
		}
		e := endOfJakartaDay(*x.ScheduledAt)
		if x.ScheduledEndAt != nil {
			e = *x.ScheduledEndAt
		}
		if s.Start == nil || x.ScheduledAt.Before(*s.Start) {
			st := *x.ScheduledAt
			s.Start = &st
		}
		if s.End == nil || e.After(*s.End) {
			ee := e
			s.End = &ee
		}
	}
	return s
}

func endOfJakartaDay(t time.Time) time.Time {
	y, m, d := t.In(jakarta).Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, jakarta).Add(-time.Nanosecond)
}

func productInWindow(p model.Product, now time.Time) bool {
	return (p.AvailableFrom == nil || !p.AvailableFrom.After(now)) &&
		(p.AvailableUntil == nil || !p.AvailableUntil.Before(now))
}

// editionPhase never returns done/live for an unscheduled edition.
func editionPhase(s editionSchedule, inWindow bool, now time.Time) string {
	if s.Start != nil && s.End != nil {
		switch {
		case now.After(*s.End):
			return PhaseDone
		case !now.Before(*s.Start):
			return PhaseLive
		}
	}
	if inWindow {
		return PhaseOpen
	}
	return PhaseClosed
}

// isOpenEdition applies the GET /products guest window plus published status;
// non-exam products are never editions.
func isOpenEdition(p model.Product, phase string, now time.Time) bool {
	return p.Type == "exam" && p.Status == "published" && productInWindow(p, now) &&
		(phase == PhaseOpen || phase == PhaseLive)
}

func sortOpenEditions(eds []PublicEdition) {
	sort.SliceStable(eds, func(i, j int) bool {
		a, b := eds[i], eds[j]
		if (a.Phase == PhaseLive) != (b.Phase == PhaseLive) {
			return a.Phase == PhaseLive
		}
		if (a.ScheduledAt == nil) != (b.ScheduledAt == nil) {
			return a.ScheduledAt != nil
		}
		if a.ScheduledAt != nil && !a.ScheduledAt.Equal(*b.ScheduledAt) {
			return a.ScheduledAt.Before(*b.ScheduledAt)
		}
		return a.Title < b.Title
	})
}
