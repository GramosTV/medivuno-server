package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role enum
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleDoctor  Role = "doctor"
	RolePatient Role = "patient"
	RoleUser    Role = "user"
)

// User represents a user in the system
type User struct {
	BaseModel
	Email             string     `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Password          string     `gorm:"size:255;not null" json:"-"` // Never send password in JSON
	FirstName         string     `gorm:"size:100" json:"firstName"`
	LastName          string     `gorm:"size:100" json:"lastName"`
	Role              Role       `gorm:"size:20;default:'user'" json:"role"`
	DateOfBirth       *time.Time `json:"dateOfBirth,omitempty"`
	PhoneNumber       string     `json:"phoneNumber,omitempty"`
	Address           string     `json:"address,omitempty"`
	ProfileImage      string     `json:"profileImage,omitempty"`
	VerificationToken string     `gorm:"size:255" json:"-"`
	IsVerified        bool       `gorm:"default:false" json:"isVerified"`
	ResetToken        string     `gorm:"size:255" json:"-"`
	ResetTokenExpiry  *time.Time `json:"-"`
	GoogleID          string     `gorm:"size:255" json:"-"`

	// Relations (not always preloaded)
	RefreshTokens       []RefreshToken  `gorm:"foreignKey:UserID" json:"-"`
	DoctorAppointments  []Appointment   `gorm:"foreignKey:DoctorID" json:"-"`
	PatientAppointments []Appointment   `gorm:"foreignKey:PatientID" json:"-"`
	MedicalRecords      []MedicalRecord `gorm:"foreignKey:PatientID" json:"-"`
	SentMessages        []Message       `gorm:"foreignKey:SenderID" json:"-"`
	ReceivedMessages    []Message       `gorm:"foreignKey:ReceiverID" json:"-"`
}

// UserSanitized represents the user data that is safe to send in API responses.
type UserSanitized struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	FirstName    string     `json:"firstName"`
	LastName     string     `json:"lastName"`
	Role         Role       `json:"role"`
	DateOfBirth  *time.Time `json:"dateOfBirth,omitempty"`
	PhoneNumber  string     `json:"phoneNumber,omitempty"`
	Address      string     `json:"address,omitempty"`
	ProfileImage string     `json:"profileImage,omitempty"`
	IsVerified   bool       `json:"isVerified"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// SetPassword hashes a password and sets it on the user
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword compares a password with the user's hashed password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// Sanitize creates a UserSanitized struct from a User model, excluding sensitive data.
func (u *User) Sanitize() UserSanitized {
	return UserSanitized{
		ID:           u.ID,
		Email:        u.Email,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Role:         u.Role,
		DateOfBirth:  u.DateOfBirth,
		PhoneNumber:  u.PhoneNumber,
		Address:      u.Address,
		ProfileImage: u.ProfileImage,
		IsVerified:   u.IsVerified,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}
