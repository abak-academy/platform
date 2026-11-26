package model

import "time"

// TestimonialContent is one locale's copy of a testimonial.
type TestimonialContent struct {
	Quote        string   `json:"quote"`
	Achievements []string `json:"achievements"`
}

// Testimonial content is keyed by locale ("id", "en"). ConsentByName is
// joined from users by the read queries and never written.
type Testimonial struct {
	ID            string                        `json:"id"`
	Name          string                        `json:"name"`
	RoleLabel     string                        `json:"role_label"`
	PhotoKey      *string                       `json:"photo_key"`
	Content       map[string]TestimonialContent `json:"content"`
	HasConsent    bool                          `json:"has_consent"`
	ConsentNote   *string                       `json:"consent_note"`
	ConsentBy     *string                       `json:"consent_by"`
	ConsentByName *string                       `json:"consent_by_name"`
	ConsentAt     *time.Time                    `json:"consent_at"`
	IsVisible     bool                          `json:"is_visible"`
	SortOrder     int                           `json:"sort_order"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}
