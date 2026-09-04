package model

import "time"

type User struct {
	ID           string  `json:"id"`
	Email        *string `json:"email"`
	Username     *string `json:"username"`
	Phone        *string `json:"phone"`
	PasswordHash string  `json:"-"`
	Role         string  `json:"role"`
	Name         string  `json:"name"`
	SchoolID     *string `json:"school_id"`
	PhotoURL     *string `json:"photo_url"`
	Status       string  `json:"status"`
	AuthProvider string  `json:"auth_provider"`
	OTPEnabled   bool    `json:"otp_enabled"`
	// MustChangePassword is set when an admin issued the current credential
	// (register, bulk, reissue) and cleared on the user's own password change.
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	// student-only
	Jenjang            *string    `json:"jenjang"`
	ProvinsiID         *string    `json:"provinsi_id"`
	KotaID             *string    `json:"kota_id"`
	KecamatanID        *string    `json:"kecamatan_id"`
	KodePos            *string    `json:"kode_pos"`
	UnlistedSchoolName *string    `json:"unlisted_school_name"`
	DOB                *time.Time `json:"dob"`
	Gender             *string    `json:"gender"`
	Grade              *int       `json:"grade"`
	AlamatDomisili     *string    `json:"alamat_domisili"`
	TargetExam         *string    `json:"target_exam"`
}
