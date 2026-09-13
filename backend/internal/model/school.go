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

type PusdatinCityReference struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProvinceID   string `json:"province_id"`
	ProvinceName string `json:"province_name"`
}

type PusdatinGeographicAlias struct {
	SourceLabel        string `json:"source_label"`
	TargetCityID       string `json:"target_city_id"`
	TargetCityName     string `json:"target_city_name"`
	TargetProvinceID   string `json:"target_province_id"`
	TargetProvinceName string `json:"target_province_name"`
	ReferenceEvidence  string `json:"reference_evidence"`
}

type PusdatinDuplicateResolution struct {
	SourceSHA256 string `json:"source_sha256"`
	NPSN         string `json:"npsn"`
	RecordNumber int    `json:"record_number"`
	RowHash      string `json:"row_hash"`
}

type PusdatinTransformOptions struct {
	ExpectedSourceSHA256 string
	Cities               []PusdatinCityReference
	Aliases              []PusdatinGeographicAlias
	Resolutions          []PusdatinDuplicateResolution
}

type PusdatinTransformStats struct {
	RawRecords           int `json:"raw_records"`
	UniqueNormalizedNPSN int `json:"unique_normalized_npsn"`
	InvalidRows          int `json:"invalid_rows"`
	BlankOrDashAddresses int `json:"blank_or_dash_addresses"`
}

type PusdatinTransformedSchool struct {
	SourceRecordNumber int      `json:"source_record_number"`
	SourceRowHash      string   `json:"source_row_hash"`
	NPSN               string   `json:"npsn"`
	Name               string   `json:"name"`
	Category           string   `json:"category"`
	SchoolTypes        []string `json:"school_types"`
	Alamat             *string  `json:"alamat"`
	CityID             string   `json:"city_id"`
	CityName           string   `json:"city_name"`
	ProvinceID         string   `json:"province_id"`
	ProvinceName       string   `json:"province_name"`
}

type PusdatinDuplicateStatus string

const (
	PusdatinDuplicateIdentical PusdatinDuplicateStatus = "identical"
	PusdatinDuplicateConflict  PusdatinDuplicateStatus = "conflict"
)

type PusdatinDuplicateRow struct {
	RecordNumber int    `json:"record_number"`
	RowHash      string `json:"row_hash"`
}

type PusdatinDuplicateGroup struct {
	NPSN      string                  `json:"npsn"`
	Status    PusdatinDuplicateStatus `json:"status"`
	Identical bool                    `json:"identical"`
	Rows      []PusdatinDuplicateRow  `json:"rows"`
}

type PusdatinTransformBlocker struct {
	Code         string `json:"code"`
	NPSN         string `json:"npsn,omitempty"`
	RecordNumber int    `json:"record_number,omitempty"`
	Message      string `json:"message"`
}

type PusdatinTransformReport struct {
	SourceSHA256            string                      `json:"source_sha256"`
	Stats                   PusdatinTransformStats      `json:"stats"`
	RecordCount             int                         `json:"record_count"`
	UniqueNormalizedNPSN    int                         `json:"unique_normalized_npsn"`
	InvalidRows             int                         `json:"invalid_rows"`
	BlankOrDashAddressCount int                         `json:"blank_or_dash_address_count"`
	DuplicateGroups         []PusdatinDuplicateGroup    `json:"duplicate_groups"`
	Transformed             []PusdatinTransformedSchool `json:"transformed"`
	Schools                 []PusdatinTransformedSchool `json:"schools"`
	TransformedChecksum     string                      `json:"transformed_checksum"`
	Blockers                []PusdatinTransformBlocker  `json:"blockers"`
}
