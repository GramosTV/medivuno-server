package models

import (
	"time"
)

// AppointmentStatus represents the status of an appointment
type AppointmentStatus string

const (
	StatusPending     AppointmentStatus = "pending"
	StatusConfirmed   AppointmentStatus = "confirmed"
	StatusCancelled   AppointmentStatus = "cancelled"
	StatusCompleted   AppointmentStatus = "completed"
	StatusRescheduled AppointmentStatus = "rescheduled"
)

// Appointment represents a scheduled medical appointment
type Appointment struct {
	BaseModel
	PatientID  string            `gorm:"size:36;index" json:"patientId"`
	DoctorID   string            `gorm:"size:36;index" json:"doctorId"`
	StartTime  time.Time         `json:"startTime"`
	EndTime    time.Time         `json:"endTime"`
	Status     AppointmentStatus `gorm:"size:20;default:'pending'" json:"status"`
	Reason     string            `gorm:"size:255" json:"reason"`
	Notes      string            `gorm:"type:text" json:"notes"`
	IsFollowUp bool              `gorm:"default:false" json:"isFollowUp"`

	// Relations
	Patient User `gorm:"foreignKey:PatientID" json:"-"`
	Doctor  User `gorm:"foreignKey:DoctorID" json:"-"`
}
