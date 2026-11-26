package model

import "time"

const (
	ProductGroupingStatusDraft     = "draft"
	ProductGroupingStatusPublished = "published"
)

type EventCategory struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

type EventFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// EventNarrative is one locale's landing-page copy for an event.
type EventNarrative struct {
	FullName    string          `json:"full_name"`
	Tagline     string          `json:"tagline"`
	LevelsLabel string          `json:"levels_label"`
	Story       []string        `json:"story"`
	Rounds      []string        `json:"rounds"`
	Categories  []EventCategory `json:"categories"`
	Facts       []EventFact     `json:"facts"`
}

// ProductGroupingScheduleItem dates are "YYYY-MM-DD" strings (Postgres DATE, no zone).
type ProductGroupingScheduleItem struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	LabelEN  *string `json:"label_en"`
	StartsOn string  `json:"starts_on"`
	EndsOn   *string `json:"ends_on"`
}

// ProductGrouping is an event: an admin-curated collection of products. A
// product belongs to at most one grouping at a time, via product.grouping_id.
// Content is keyed by locale ("id", "en").
type ProductGrouping struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	Status            string                    `json:"status"`
	DisplayOrder      int                       `json:"display_order"`
	Slug              string                    `json:"slug"`
	ShortName         *string                   `json:"short_name"`
	Scope             *string                   `json:"scope"`
	Subjects          []string                  `json:"subjects"`
	Tone              string                    `json:"tone"`
	Illustration      string                    `json:"illustration"`
	LogoKey           *string                   `json:"logo_key"`
	OfficialURL       *string                   `json:"official_url"`
	Content           map[string]EventNarrative `json:"content"`
	IsVisible         bool                      `json:"is_visible"`
	ParticipantPrefix *string                   `json:"participant_prefix"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}
