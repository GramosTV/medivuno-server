package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// BaseModel contains common columns for all tables
type BaseModel struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BeforeCreate will set a UUID rather than numeric ID
func (base *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if base.ID == "" {
		base.ID = uuid.New().String()
	}
	return nil
}

// Database connection instance
var DB *gorm.DB

// InitDB initializes database connection
func InitDB(config DatabaseConfig) (*gorm.DB, error) {
	var err error

	// Connect to MySQL database
	DB, err = gorm.Open(mysql.Open(config.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto migrate the database models
	err = DB.AutoMigrate(
		&User{},
		&RefreshToken{},
		&MedicalRecord{},
		&MedicalRecordAttachment{},
		&Appointment{},
		&Message{},
	)
	if err != nil {
		return nil, err
	}

	return DB, nil
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	DSN string
}
