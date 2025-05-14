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
	MedicalRecordID string `gorm:"size:36;index" json:"medicalRecordId"`
	FileName        string `gorm:"size:255" json:"fileName"`
	FilePath        string `gorm:"size:255" json:"filePath"`
	FileSize        int64  `json:"fileSize"`
	MimeType        string `gorm:"size:100" json:"mimeType"`

	// Relation back to the medical record
	MedicalRecord MedicalRecord `gorm:"foreignKey:MedicalRecordID" json:"-"`
}
