package handlers

import (
	"healthcare-app-server/internal/middleware"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/utils"
	"time"

	// "mime/multipart"
	// "net/http"
	// "os"
	// "path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MedicalRecordHandler handles medical record related requests.
type MedicalRecordHandler struct {
	DB *gorm.DB
}

// NewMedicalRecordHandler creates a new MedicalRecordHandler.
func NewMedicalRecordHandler(db *gorm.DB) *MedicalRecordHandler {
	return &MedicalRecordHandler{DB: db}
}

// CreateMedicalRecordRequest represents the request body for creating a medical record.
type CreateMedicalRecordRequest struct {
	PatientID  string                   `json:"patientId" binding:"required,uuid"`
	RecordType models.MedicalRecordType `json:"recordType" binding:"required"`
	RecordDate string                   `json:"date" binding:"required"`
	Title      string                   `json:"title" binding:"required"`
	Department string                   `json:"department"`
	Summary    string                   `json:"summary" binding:"required"`
	Details    string                   `json:"details"`
	// Attachments will be handled separately or via multipart form
}

// CreateMedicalRecord handles creating a new medical record.
// Only accessible by doctors.
func (h *MedicalRecordHandler) CreateMedicalRecord(c *gin.Context) {
	var req CreateMedicalRecordRequest
	if !utils.BindAndValidate(c, &req) {
		return
	}

	doctorIDStr, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "Doctor ID not found in token")
		return
	}
	doctorID, err := uuid.Parse(doctorIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Doctor ID format in token")
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		utils.BadRequest(c, "Invalid Patient ID format")
		return
	}

	// Verify patient exists
	var patient models.User
	if err := h.DB.Where("id = ? AND role = ?", patientID, models.RolePatient).First(&patient).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Patient not found")
		} else {
			utils.InternalServerError(c, "Database error verifying patient: "+err.Error())
		}
		return
	}
	// Parse the date if needed
	var recordDate time.Time
	if req.RecordDate != "" {
		var err error
		recordDate, err = time.Parse(time.RFC3339, req.RecordDate)
		if err != nil {
			utils.BadRequest(c, "Invalid date format. Please use ISO 8601 format (YYYY-MM-DDTHH:MM:SSZ)")
			return
		}
	} else {
		recordDate = time.Now()
	}
	record := models.MedicalRecord{
		PatientID:  patientID.String(), // Convert UUID to string
		DoctorID:   doctorID.String(),  // Convert UUID to string
		RecordType: req.RecordType,
		RecordDate: recordDate,
		Title:      req.Title,
		Department: req.Department,
		Summary:    req.Summary,
		Details:    req.Details,
	}

	if err := h.DB.Create(&record).Error; err != nil {
		utils.InternalServerError(c, "Failed to create medical record: "+err.Error())
		return
	}

	utils.Created(c, "Medical record created successfully", record)
}

// GetMedicalRecordsForPatient handles fetching medical records for a specific patient.
// Accessible by the patient themselves or doctors.
func (h *MedicalRecordHandler) GetMedicalRecordsForPatient(c *gin.Context) {
	patientIDStr := c.Param("patientId")
	patientID, err := uuid.Parse(patientIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Patient ID format")
		return
	}

	requestingUserIDStr, _ := middleware.GetUserIDFromContext(c)
	requestingUserRole, _ := middleware.GetUserRoleFromContext(c)

	// Authorization: Patient can see their own records, Doctors can see any patient's records (for now)
	// More granular access control might be needed (e.g., doctor assigned to patient)
	if requestingUserRole != models.RoleDoctor && requestingUserIDStr != patientIDStr {
		utils.Forbidden(c, "You are not authorized to view these medical records")
		return
	}

	var records []models.MedicalRecord
	if err := h.DB.Preload("Attachments").Where("patient_id = ?", patientID).Order("created_at desc").Find(&records).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch medical records: "+err.Error())
		return
	}

	utils.Success(c, "Medical records fetched successfully", records)
}

// GetMedicalRecordByID handles fetching a single medical record by its ID.
// Accessible by the patient (if it's theirs) or doctors.
func (h *MedicalRecordHandler) GetMedicalRecordByID(c *gin.Context) {
	recordIDStr := c.Param("id")
	recordID, err := uuid.Parse(recordIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Medical Record ID format")
		return
	}

	var record models.MedicalRecord
	if err := h.DB.Preload("Attachments").First(&record, "id = ?", recordID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Medical record not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}
	requestingUserIDStr, _ := middleware.GetUserIDFromContext(c)
	requestingUserRole, _ := middleware.GetUserRoleFromContext(c)

	if requestingUserRole != models.RoleDoctor && requestingUserIDStr != record.PatientID {
		utils.Forbidden(c, "You are not authorized to view this medical record")
		return
	}

	utils.Success(c, "Medical record fetched successfully", record)
}

// UpdateMedicalRecordRequest represents the request body for updating a medical record.
type UpdateMedicalRecordRequest struct {
	RecordType models.MedicalRecordType `json:"recordType,omitempty"`
	Title      string                   `json:"title,omitempty"`
	Department string                   `json:"department,omitempty"`
	Summary    string                   `json:"summary,omitempty"`
	Details    string                   `json:"details,omitempty"`
}

// UpdateMedicalRecord handles updating an existing medical record.
// Only accessible by the doctor who created it or an admin.
func (h *MedicalRecordHandler) UpdateMedicalRecord(c *gin.Context) {
	recordIDStr := c.Param("id")
	recordID, err := uuid.Parse(recordIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Medical Record ID format")
		return
	}

	var req UpdateMedicalRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request payload: "+err.Error())
		return
	}

	var record models.MedicalRecord
	if err := h.DB.First(&record, "id = ?", recordID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Medical record not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	requestingUserIDStr, _ := middleware.GetUserIDFromContext(c)
	requestingUserRole, _ := middleware.GetUserRoleFromContext(c)
	// Authorization: Only the creating doctor or an Admin can update.
	if requestingUserRole != models.RoleAdmin && requestingUserIDStr != record.DoctorID {
		utils.Forbidden(c, "You are not authorized to update this medical record")
		return
	}

	if req.RecordType != "" {
		record.RecordType = req.RecordType
	}
	if req.Title != "" {
		record.Title = req.Title
	}
	if req.Department != "" {
		record.Department = req.Department
	}
	if req.Summary != "" {
		record.Summary = req.Summary
	}
	if req.Details != "" {
		record.Details = req.Details
	}

	if err := h.DB.Save(&record).Error; err != nil {
		utils.InternalServerError(c, "Failed to update medical record: "+err.Error())
		return
	}

	utils.Success(c, "Medical record updated successfully", record)
}

// DeleteMedicalRecord handles deleting a medical record.
// Typically restricted to admins or the creating doctor under certain conditions.
func (h *MedicalRecordHandler) DeleteMedicalRecord(c *gin.Context) {
	recordIDStr := c.Param("id")
	recordID, err := uuid.Parse(recordIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Medical Record ID format")
		return
	}

	var record models.MedicalRecord
	if err := h.DB.First(&record, "id = ?", recordID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Medical record not found to delete")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	requestingUserIDStr, _ := middleware.GetUserIDFromContext(c)
	requestingUserRole, _ := middleware.GetUserRoleFromContext(c)
	// Authorization: Only the creating doctor or an Admin can delete.
	// Consider soft delete.
	if requestingUserRole != models.RoleAdmin && requestingUserIDStr != record.DoctorID {
		utils.Forbidden(c, "You are not authorized to delete this medical record")
		return
	}

	// Also delete attachments if any (or handle via DB cascade)
	if err := h.DB.Where("medical_record_id = ?", recordID).Delete(&models.MedicalRecordAttachment{}).Error; err != nil {
		// Log this error but proceed with deleting the main record
		// utils.InternalServerError(c, "Failed to delete associated attachments: "+err.Error())
		// return
	}

	if err := h.DB.Delete(&record).Error; err != nil {
		utils.InternalServerError(c, "Failed to delete medical record: "+err.Error())
		return
	}

	utils.Success(c, "Medical record deleted successfully", nil)
}

// TODO: Add handlers for uploading and managing MedicalRecordAttachments
// e.g., UploadAttachment, GetAttachment, DeleteAttachment
