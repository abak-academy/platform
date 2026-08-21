package service

import (
	"context"
	"sort"
	"time"

	"akademi-bimbel/internal/model"
)

// monitorEndedVisibleFor is how long an ended exam stays on the Session
// Monitor's available-exams list after its window closes, so an admin can
// still check what happened right after it wrapped.
const monitorEndedVisibleFor = 24 * time.Hour

// examMonitorWindow decides whether an exam belongs on the Session Monitor's
// available-exams list right now, and if so whether it's "live" or "ended".
//
// The window opens check_in_window_minutes before scheduled_at and stays open
// through duration_minutes + grace_window_minutes past the latest possible
// start (scheduled_end_at when set, else scheduled_at itself — a student can
// start any time up to scheduled_end_at per FR on flexible-window exams).
func examMonitorWindow(now time.Time, c model.ExamMonitorCandidate) (state string, ok bool) {
	if c.ScheduledAt == nil {
		return "", false
	}

	checkIn := 0
	if c.CheckInWindowMinutes != nil {
		checkIn = *c.CheckInWindowMinutes
	}
	start := c.ScheduledAt.Add(-time.Duration(checkIn) * time.Minute)
	if now.Before(start) {
		return "", false
	}

	latestStart := *c.ScheduledAt
	if c.ScheduledEndAt != nil {
		latestStart = *c.ScheduledEndAt
	}
	duration := 0
	if c.DurationMinutes != nil {
		duration = *c.DurationMinutes
	}
	grace := 0
	if c.GraceWindowMinutes != nil {
		grace = *c.GraceWindowMinutes
	}
	end := latestStart.Add(time.Duration(duration+grace) * time.Minute)

	if !now.After(end) {
		return "live", true
	}
	if now.After(end.Add(monitorEndedVisibleFor)) {
		return "", false
	}
	return "ended", true
}

// ListExamsForMonitor returns exams currently within their scheduled window (or
// recently ended), each with registration counts, for the Session Monitor's
// available-exams list (FR: replaces has_published_product gating, which
// misses exams granted directly without a Product).
func (s *Service) ListExamsForMonitor(ctx context.Context) ([]model.ExamMonitorAvailable, error) {
	candidates, err := s.storeRepo.ListExamMonitorCandidates(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]model.ExamMonitorAvailable, 0, len(candidates))
	for _, c := range candidates {
		state, ok := examMonitorWindow(now, c)
		if !ok {
			continue
		}

		rows, err := s.storeRepo.GetSessionMonitorRows(ctx, c.ID)
		if err != nil {
			return nil, err
		}

		active, notStarted := 0, 0
		for i := range rows {
			switch deriveStatus(rows[i], now, c.DurationMinutes, c.GraceWindowMinutes) {
			case "registered":
				notStarted++
			case "checked_in", "in_progress", "overdue":
				active++
			}
		}

		out = append(out, model.ExamMonitorAvailable{
			ID:              c.ID,
			Title:           c.Title,
			ScheduledAt:     c.ScheduledAt,
			ScheduledEndAt:  c.ScheduledEndAt,
			State:           state,
			TotalRegistered: len(rows),
			ActiveCount:     active,
			NotStartedCount: notStarted,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ScheduledAt.Before(*out[j].ScheduledAt)
	})

	return out, nil
}
