package model

import (
	"time"

	"github.com/google/uuid"
)

type ExternalRound struct {
	ID          uuid.UUID  `json:"id"`
	ExamID      uuid.UUID  `json:"exam_id"`
	Label       string     `json:"label"`
	LabelEN     *string    `json:"label_en"`
	SourceKey   *string    `json:"source_key"`
	RowCount    int        `json:"row_count"`
	PublishedAt *time.Time `json:"published_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ExternalResultInput is one validated CSV row resolved to a registration.
type ExternalResultInput struct {
	RegistrationID uuid.UUID
	Score          string
	Rank           *int
	Award          *string
	CertificateURL *string
}

type ExternalResultRow struct {
	RegistrationID uuid.UUID `json:"registration_id"`
	StudentName    string    `json:"student_name"`
	ParticipantNo  string    `json:"participant_no"`
	Score          float64   `json:"score"`
	Rank           *int      `json:"rank"`
	Award          *string   `json:"award"`
	CertificateURL *string   `json:"certificate_url"`

	ParticipantNumber *int       `json:"-"`
	ExamNumber        *int       `json:"-"`
	DisplayDate       time.Time  `json:"-"`
	ExamScheduledAt   *time.Time `json:"-"`
}

type StudentExternalResult struct {
	RoundID        uuid.UUID `json:"round_id"`
	RoundLabel     string    `json:"round_label"`
	Score          float64   `json:"score"`
	Rank           *int      `json:"rank"`
	Award          *string   `json:"award"`
	CertificateURL *string   `json:"certificate_url"`
	PublishedAt    time.Time `json:"published_at"`

	RegistrationID uuid.UUID `json:"-"`
	LabelEN        *string   `json:"-"`
}

type ExternalRowError struct {
	Line          int    `json:"line"`
	ParticipantNo string `json:"participant_no"`
	Reason        string `json:"reason"`
}

type ExternalRoundPreview struct {
	Matched        int                `json:"matched"`
	Errors         []ExternalRowError `json:"errors"`
	RoundExists    bool               `json:"round_exists"`
	RoundPublished bool               `json:"round_published"`
}
