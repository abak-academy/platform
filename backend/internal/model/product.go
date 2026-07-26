package model

import "time"

// ProductSpec is one row of the product specification table shown on the
// storefront. Label travels with the value so rendering never needs to look up
// a field catalogue, and so operator-added custom rows render like any other.
type ProductSpec struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type Product struct {
	ID             string        `json:"id"`
	Type           string        `json:"type"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Price          int64         `json:"price"`
	Stock          int           `json:"stock"`
	Status         string        `json:"status"`
	WeightGrams    int           `json:"weight_grams"`
	ImageURL       string        `json:"image_url"`
	Specs          []ProductSpec `json:"specs"`
	WeightGramsSet bool          `json:"-"`
	ImageURLSet    bool          `json:"-"`
	CourseIDs      []string      `json:"course_ids"`
	ExamIDs        []string      `json:"exam_ids"`
	// AvailableFrom/AvailableUntil bound the marketplace listing window. NULL on
	// either side means unbounded on that side. The *Set flags mark whether an
	// update request touched the field, so an omitted field preserves the
	// existing value instead of clobbering it to NULL.
	AvailableFrom     *time.Time `json:"available_from"`
	AvailableUntil    *time.Time `json:"available_until"`
	AvailableFromSet  bool       `json:"-"`
	AvailableUntilSet bool       `json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
