package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

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
	// Nil DurationMinutes means timer_mode=per_test (UTBK/IELTS): there's no single
	// exam-level clock, so the window's duration comes from the sum of the exam's
	// section durations instead — otherwise the window closes after just the grace
	// period and the exam vanishes from the list while students are still mid-section.
	duration := c.SectionsDurationMinutes
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

		// GetSessionMonitorRows returns one row per exam_session, and a retake
		// (max_attempts >= 2) gives one registration several sessions — so counts
		// are aggregated per RegistrationID, not per row, or a retaking student
		// would be counted more than once.
		type regState struct {
			active     bool
			registered bool
		}
		byReg := make(map[uuid.UUID]*regState, len(rows))
		for i := range rows {
			rs := byReg[rows[i].RegistrationID]
			if rs == nil {
				rs = &regState{}
				byReg[rows[i].RegistrationID] = rs
			}
			switch deriveStatus(rows[i], now, c.DurationMinutes, c.GraceWindowMinutes) {
			case "registered":
				rs.registered = true
			case "checked_in", "in_progress", "overdue":
				rs.active = true
			}
		}

		active, notStarted := 0, 0
		for _, rs := range byReg {
			switch {
			case rs.active:
				active++
			case rs.registered:
				notStarted++
			}
		}

		out = append(out, model.ExamMonitorAvailable{
			ID:              c.ID,
			Title:           c.Title,
			ScheduledAt:     c.ScheduledAt,
			ScheduledEndAt:  c.ScheduledEndAt,
			State:           state,
			TotalRegistered: len(byReg),
			ActiveCount:     active,
			NotStartedCount: notStarted,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ScheduledAt.Before(*out[j].ScheduledAt)
	})

	return out, nil
}
