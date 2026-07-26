package service

import "errors"

var (
	// ErrStorageNotConfigured replaces eight separate inline allocations of the
	// same message across certificate/exam/job/student, so callers can match it
	// with errors.Is instead of comparing strings.
	ErrStorageNotConfigured = errors.New("storage not configured")

	ErrAlreadyAttempted     = errors.New("already attempted")
	ErrExamNotStarted       = errors.New("exam not started")
	ErrDeviceMismatch       = errors.New("device mismatch")
	ErrCheckinWindowClosed  = errors.New("check-in window closed")
	ErrExamWindowClosed     = errors.New("exam availability window closed")
	ErrNotCheckedIn         = errors.New("not checked in")
	ErrAlreadySubmitted     = errors.New("already submitted")
	ErrSessionNotFound      = errors.New("session not found")
	ErrInvalidViolationType = errors.New("invalid violation type")

	// Result gating (FR-S5-21) and essay grading (FR-S5-13) sentinels.
	// The gating non-result states (hidden/grading/locked) are returned as data on
	// SessionResult, not as errors — these are for the grading write path and any
	// hard-fail result cases.
	ErrResultHidden      = errors.New("result hidden")
	ErrResultNotReleased = errors.New("result not released")
	ErrSessionNotGraded  = errors.New("session not fully graded")
	ErrGradeOutOfRange   = errors.New("grade out of range")
	ErrNotEssayQuestion  = errors.New("question is not an essay")

	ErrLeaderboardNotAvailable = errors.New("leaderboard not available")

	// Sectioned-exam errors (FR-11/FR-14).
	ErrSectionLocked    = errors.New("section locked")
	ErrSectionNotActive = errors.New("section not active")
)
