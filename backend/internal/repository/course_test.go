package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

// Compile-time signature checks, hoisted out of the assertion-free tests that used to
// wrap them. The compiler enforces these; a test function around them did not.
var (
	_ func(context.Context, uuid.UUID) ([]model.Course, error)        = (*Repository)(nil).GetCoursesByProductID
	_ func(context.Context, uuid.UUID) (int, error)                   = (*Repository)(nil).CountLessonsByCourse
	_ func(context.Context, uuid.UUID) ([]model.Section, error)       = (*Repository)(nil).ListSections
	_ func(context.Context, uuid.UUID, []uuid.UUID) error             = (*Repository)(nil).ReorderSections
	_ func(context.Context, uuid.UUID, string) (model.Section, error) = (*Repository)(nil).UpdateSection
	_ func(context.Context, uuid.UUID) error                          = (*Repository)(nil).DeleteSection

	_ func(context.Context, pgx.Tx, model.CourseSession) error     = (*Repository)(nil).CreateCourseSession
	_ func(context.Context, pgx.Tx, uuid.UUID) error               = (*Repository)(nil).RevokeEnrollmentsByOrder
	_ func(context.Context, uuid.UUID, uuid.UUID, time.Time) error = (*Repository)(nil).MarkLessonComplete
)
