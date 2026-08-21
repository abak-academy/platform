package service

import (
	"testing"
	"time"

	"akademi-bimbel/internal/model"
)

func TestExamMonitorWindow_NilScheduledAt_NeverAvailable(t *testing.T) {
	c := model.ExamMonitorCandidate{}
	_, ok := examMonitorWindow(time.Now(), c)
	if ok {
		t.Errorf("want not available, got available")
	}
}

func TestExamMonitorWindow_BeforeCheckInWindow_NotAvailable(t *testing.T) {
	scheduledAt := time.Now().Add(2 * time.Hour)
	c := model.ExamMonitorCandidate{
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: intptr(30),
	}
	_, ok := examMonitorWindow(time.Now(), c)
	if ok {
		t.Errorf("want not available before check-in window opens, got available")
	}
}

func TestExamMonitorWindow_WithinCheckInWindow_BeforeStart_IsLive(t *testing.T) {
	scheduledAt := time.Now().Add(10 * time.Minute)
	c := model.ExamMonitorCandidate{
		ScheduledAt:          &scheduledAt,
		CheckInWindowMinutes: intptr(30),
		DurationMinutes:      intptr(60),
	}
	state, ok := examMonitorWindow(time.Now(), c)
	if !ok || state != "live" {
		t.Errorf("want live, got state=%q ok=%v", state, ok)
	}
}

func TestExamMonitorWindow_DuringExam_IsLive(t *testing.T) {
	scheduledAt := time.Now().Add(-30 * time.Minute)
	c := model.ExamMonitorCandidate{
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(120),
	}
	state, ok := examMonitorWindow(time.Now(), c)
	if !ok || state != "live" {
		t.Errorf("want live, got state=%q ok=%v", state, ok)
	}
}

func TestExamMonitorWindow_AfterEndWithinBuffer_IsEnded(t *testing.T) {
	scheduledAt := time.Now().Add(-3 * time.Hour)
	c := model.ExamMonitorCandidate{
		ScheduledAt:        &scheduledAt,
		DurationMinutes:    intptr(60),
		GraceWindowMinutes: intptr(10),
	}
	state, ok := examMonitorWindow(time.Now(), c)
	if !ok || state != "ended" {
		t.Errorf("want ended, got state=%q ok=%v", state, ok)
	}
}

func TestExamMonitorWindow_AfterBuffer_NotAvailable(t *testing.T) {
	scheduledAt := time.Now().Add(-48 * time.Hour)
	c := model.ExamMonitorCandidate{
		ScheduledAt:     &scheduledAt,
		DurationMinutes: intptr(60),
	}
	_, ok := examMonitorWindow(time.Now(), c)
	if ok {
		t.Errorf("want not available long after the exam ended, got available")
	}
}

func TestExamMonitorWindow_ScheduledEndAt_ExtendsLatestStart(t *testing.T) {
	scheduledAt := time.Now().Add(-3 * time.Hour)
	scheduledEndAt := time.Now().Add(-1 * time.Hour)
	c := model.ExamMonitorCandidate{
		ScheduledAt:     &scheduledAt,
		ScheduledEndAt:  &scheduledEndAt,
		DurationMinutes: intptr(90),
	}
	// A student could have started as late as scheduledEndAt, so the exam is
	// still live even though scheduledAt+duration has long passed.
	state, ok := examMonitorWindow(time.Now(), c)
	if !ok || state != "live" {
		t.Errorf("want live via scheduled_end_at extension, got state=%q ok=%v", state, ok)
	}
}
