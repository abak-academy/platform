package model

import (
	"time"

	"github.com/google/uuid"
)

type CompetitionResult struct {
	ID          uuid.UUID  `json:"id"`
	ProductID   uuid.UUID  `json:"product_id"`
	SourceKey   *string    `json:"source_key"`
	RowCount    int        `json:"row_count"`
	PublishedAt *time.Time `json:"published_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CompetitionResultInput is one validated CSV row resolved to a registration.
type CompetitionResultInput struct {
	RegistrationID uuid.UUID
	Score          string
	Rank           *int
	Award          *string
	CertificateURL *string
}

type CompetitionResultRow struct {
	RegistrationID uuid.UUID `json:"registration_id"`
	StudentName    string    `json:"student_name"`
	ParticipantNo  string    `json:"participant_no"`
	Score          float64   `json:"score"`
	Rank           *int      `json:"rank"`
	Award          *string   `json:"award"`
	CertificateURL *string   `json:"certificate_url"`

	ParticipantNumber *int       `json:"-"`
	ExamNumber        *int       `json:"-"`
	ExamPrefix        *string    `json:"-"`
	DisplayDate       time.Time  `json:"-"`
	ExamScheduledAt   *time.Time `json:"-"`
}

type StudentCompetitionResult struct {
	Score          float64   `json:"score"`
	Rank           *int      `json:"rank"`
	Award          *string   `json:"award"`
	CertificateURL *string   `json:"certificate_url"`
	PublishedAt    time.Time `json:"published_at"`

	RegistrationID uuid.UUID `json:"-"`
}

type CompetitionRowError struct {
	Line          int    `json:"line"`
	ParticipantNo string `json:"participant_no"`
	Reason        string `json:"reason"`
}

type CompetitionResultPreview struct {
	Matched         int                   `json:"matched"`
	Errors          []CompetitionRowError `json:"errors"`
	ResultExists    bool                  `json:"result_exists"`
	ResultPublished bool                  `json:"result_published"`
}
