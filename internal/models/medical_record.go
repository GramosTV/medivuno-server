package models

import (
	"time"
)

// MedicalRecordType represents the type of medical record
type MedicalRecordType string

const (
	RecordTypeConsultation     MedicalRecordType = "ConsultationNote"
	RecordTypeLabResult        MedicalRecordType = "LabResult"
	RecordTypePrescription     MedicalRecordType = "Prescription"
	RecordTypeImagingReport    MedicalRecordType = "ImagingReport"
	RecordTypeVaccination      MedicalRecordType = "VaccinationRecord"
	RecordTypeAllergy          MedicalRecordType = "AllergyRecord"
	RecordTypeDischargeSummary MedicalRecordType = "DischargeSummary"
)

// MedicalRecord represents a patient's medical record
type MedicalRecord struct {
	BaseModel
	PatientID  string            `gorm:"size:36;index" json:"patientId"`
	DoctorID   string            `gorm:"size:36;index" json:"doctorId"`
	RecordType MedicalRecordType `gorm:"size:50" json:"recordType"`
	RecordDate time.Time         `json:"date"`
	Title      string            `gorm:"size:255;not null" json:"title"`
	Department string            `gorm:"size:100" json:"department"`
	Summary    string            `gorm:"type:text" json:"summary"`
	Details    string            `gorm:"type:text" json:"details"`

	// Relations
	Patient     User                      `gorm:"foreignKey:PatientID" json:"-"`
	Doctor      User                      `gorm:"foreignKey:DoctorID" json:"-"`
	Attachments []MedicalRecordAttachment `gorm:"foreignKey:MedicalRecordID" json:"attachments,omitempty"`
}

// MedicalRecordAttachment represents a file attached to a medical record
type MedicalRecordAttachment struct {
	BaseModel
	MedicalRecordID string `json:"medicalRecordId" gorm:"not null;type:varchar(36)"` // Changed from uint to string to match MedicalRecord.ID
	FileName        string `json:"fileName" gorm:"not null"`                         // Original name of the file
	FileType        string `json:"fileType" gorm:"not null"`                         // MIME type of the file
	FileData        []byte `json:"-" gorm:"type:longblob;not null"`                  // File content as binary data (longblob for MySQL)
}
