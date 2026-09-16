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
	ProvinsiID  *string   `json:"provinsi_id"`
	KotaID      *string   `json:"kota_id"`
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
	ProvinsiID   *string  `json:"provinsi_id"`
	KotaID       *string  `json:"kota_id"`
	KotaName     *string  `json:"kota_name"`
	ProvinsiName *string  `json:"provinsi_name"`
}
