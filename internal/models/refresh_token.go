package models

import (
	"time"
)

// RefreshToken represents a JWT refresh token in the database
type RefreshToken struct {
	BaseModel
	UserID    string    `gorm:"size:36;index" json:"userId"`
	Token     string    `gorm:"type:text;not null" json:"-"`
	ExpiresAt time.Time `json:"expiresAt"`
	IsRevoked bool      `gorm:"default:false" json:"isRevoked"`

	// Define the relationship to User
	User User `gorm:"foreignKey:UserID" json:"-"`
}
