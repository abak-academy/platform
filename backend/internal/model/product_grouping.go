package model

import "time"

const (
	ProductGroupingStatusDraft     = "draft"
	ProductGroupingStatusPublished = "published"
)

// ProductGrouping is an admin-curated collection of products shown together in
// the catalog (e.g. a themed bundle). A product belongs to at most one
// grouping at a time, via product.grouping_id.
type ProductGrouping struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	DisplayOrder int       `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
