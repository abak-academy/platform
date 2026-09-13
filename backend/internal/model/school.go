package model

import "time"

type School struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	NPSN        *string   `json:"npsn"`
	SchoolTypes []string  `json:"school_types"`
	Alamat      *string   `json:"alamat"`
	Category    *string   `json:"category"`
	CityID      *string   `json:"city_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SchoolOption struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Code         string   `json:"code"`
	NPSN         *string  `json:"npsn"`
	SchoolTypes  []string `json:"school_types"`
	Alamat       *string  `json:"alamat"`
	Status       string   `json:"status"`
	Category     *string  `json:"category"`
	CityID       *string  `json:"city_id"`
	CityName     *string  `json:"city_name"`
	ProvinceID   *string  `json:"province_id"`
	ProvinceName *string  `json:"province_name"`
}
