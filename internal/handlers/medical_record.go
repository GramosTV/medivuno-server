package handlers

import (
	"fmt" // Added for logging
	"healthcare-app-server/internal/middleware"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/utils"
	"io/ioutil" // Added for ioutil.ReadAll
	"net/http"  // Added for http.StatusOK and http.StatusNotImplemented
	"strings"   // Import for strings.EqualFold
	"time"

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
	RecordDate string                   `json:"recordDate" binding:"required"` // Changed from json:"date"
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
	_, err := uuid.Parse(patientIDStr) // Changed patientID to _ as it's not used before re-check
	if err != nil {
		fmt.Println("[DEBUG] Invalid Patient ID format from URL param:", patientIDStr) // Log the problematic param
		utils.BadRequest(c, "Invalid Patient ID format from URL param: "+patientIDStr)
		return
	}

	requestingUserIDStr, userIDExists := middleware.GetUserIDFromContext(c)
	requestingUserRole, userRoleExists := middleware.GetUserRoleFromContext(c)

	// Log the received and parsed values for debugging
	fmt.Println("[DEBUG] GetMedicalRecordsForPatient: Attempting to access records.")
	fmt.Printf("[DEBUG] GetMedicalRecordsForPatient: PatientID from URL: %s\n", patientIDStr)
	fmt.Printf("[DEBUG] GetMedicalRecordsForPatient: Requesting User ID: %s (Exists: %t)\n", requestingUserIDStr, userIDExists)
	fmt.Printf("[DEBUG] GetMedicalRecordsForPatient: Requesting User Role: %s (Exists: %t)\n", string(requestingUserRole), userRoleExists)

	// Authorization: Patient can see their own records, Doctors can see any patient\'s records
	// Use strings.EqualFold for case-insensitive role comparison.
	isDoctor := userRoleExists && strings.EqualFold(string(requestingUserRole), string(models.RoleDoctor))
	isSelf := userIDExists && requestingUserIDStr == patientIDStr

	if isDoctor || isSelf {
		// Authorized
		fmt.Printf("[DEBUG] GetMedicalRecordsForPatient: Authorization successful. Role: %s (IsDoctor: %t), RequestingID: %s, TargetPatientID: %s (IsSelf: %t)\n", string(requestingUserRole), isDoctor, requestingUserIDStr, patientIDStr, isSelf)
	} else {
		// Not authorized
		fmt.Printf("[DEBUG] GetMedicalRecordsForPatient: Authorization failed. Role: %s (IsDoctor: %t), RequestingID: %s, TargetPatientID: %s (IsSelf: %t). models.RoleDoctor is: %s\n", string(requestingUserRole), isDoctor, requestingUserIDStr, patientIDStr, isSelf, string(models.RoleDoctor))
		utils.Forbidden(c, "You are not authorized to view these medical records")
		return
	}

	fmt.Printf("[DEBUG] GetMedicalRecordsForPatient: Proceeding to fetch records for patient %s\n", patientIDStr)

	// Re-parse patientID here as it's needed for the DB query
	parsedPatientID, err := uuid.Parse(patientIDStr)
	if err != nil {
		// This should ideally not happen if the first parse succeeded, but as a safeguard:
		utils.InternalServerError(c, "Failed to parse patient ID for database query")
		return
	}

	var records []models.MedicalRecord
	if err := h.DB.Preload("Attachments").Where("patient_id = ?", parsedPatientID).Order("created_at desc").Find(&records).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch medical records: "+err.Error())
		return
	}

	utils.Success(c, "Medical records fetched successfully", records)
}

// UploadMedicalRecordAttachment handles uploading attachment files for a specific medical record.
// Stores the file as binary data in the database.
// Only accessible by doctors.
func (h *MedicalRecordHandler) UploadMedicalRecordAttachment(c *gin.Context) {
	medicalRecordIDStr := c.Param("id")
	medicalRecordID, err := uuid.Parse(medicalRecordIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Medical Record ID format from URL param: "+medicalRecordIDStr)
		return
	}

	// Verify the medical record exists
	var record models.MedicalRecord
	if err := h.DB.First(&record, "id = ?", medicalRecordID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Medical record not found")
		} else {
			utils.InternalServerError(c, "Database error verifying medical record: "+err.Error())
		}
		return
	}

	file, header, err := c.Request.FormFile("file") // "file" is the name of the form field
	if err != nil {
		utils.BadRequest(c, "Error retrieving file from form: "+err.Error())
		return
	}
	defer file.Close()

	fileData, err := ioutil.ReadAll(file)
	if err != nil {
		utils.InternalServerError(c, "Error reading file content: "+err.Error())
		return
	}

	// Create MedicalRecordAttachment entry
	attachment := models.MedicalRecordAttachment{
		MedicalRecordID: medicalRecordID.String(),
		FileName:        header.Filename,
		FileType:        header.Header.Get("Content-Type"),
		FileData:        fileData,
	}

	if err := h.DB.Create(&attachment).Error; err != nil {
		utils.InternalServerError(c, "Failed to create medical record attachment entry: "+err.Error())
		return
	}

	// Return a slimmed down version of the attachment, without the FileData
	responseAttachment := struct {
		ID              string    `json:"id"`
		MedicalRecordID string    `json:"medicalRecordId"`
		FileName        string    `json:"fileName"`
		FileType        string    `json:"fileType"`
		CreatedAt       time.Time `json:"createdAt"`
	}{
		ID:              attachment.ID,
		MedicalRecordID: attachment.MedicalRecordID,
		FileName:        attachment.FileName,
		FileType:        attachment.FileType,
		CreatedAt:       attachment.CreatedAt,
	}

	utils.Success(c, "File uploaded and linked to medical record successfully", responseAttachment)
}

// GetMedicalRecordAttachment handles retrieving a specific attachment by its ID and serving its file data.
// Authorization should ensure the requesting user has rights to view the parent medical record.
func (h *MedicalRecordHandler) GetMedicalRecordAttachment(c *gin.Context) {
	attachmentIDStr := c.Param("attachmentId")
	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Attachment ID format: "+attachmentIDStr)
		return
	}

	var attachment models.MedicalRecordAttachment
	if err := h.DB.First(&attachment, "id = ?", attachmentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Attachment not found")
		} else {
			utils.InternalServerError(c, "Database error fetching attachment: "+err.Error())
		}
		return
	}

	// Authorization: Check if the user can access the parent medical record
	var medicalRecord models.MedicalRecord
	if err := h.DB.First(&medicalRecord, "id = ?", attachment.MedicalRecordID).Error; err != nil {
		utils.InternalServerError(c, "Could not fetch parent medical record for authorization check.")
		return
	}

	requestingUserIDStr, userIDExists := middleware.GetUserIDFromContext(c)
	requestingUserRole, userRoleExists := middleware.GetUserRoleFromContext(c)

	if !userIDExists || !userRoleExists {
		utils.Unauthorized(c, "User information not found in token for authorization.")
		return
	}

	isDoctor := strings.EqualFold(string(requestingUserRole), string(models.RoleDoctor))
	isPatientOwner := strings.EqualFold(string(requestingUserRole), string(models.RolePatient)) && requestingUserIDStr == medicalRecord.PatientID
	isRecordCreator := isDoctor && requestingUserIDStr == medicalRecord.DoctorID // Or any doctor if policy allows

	// Allow if: user is a doctor (general access to records they can see), or patient owning the record.
	// More granular: isDoctor && (requestingUserIDStr == medicalRecord.DoctorID || userHasAccessToPatient(requestingUserIDStr, medicalRecord.PatientID))
	_ = isRecordCreator // Explicitly ignore if not used, or remove if logic changes
	if !(isDoctor || isPatientOwner) {
		// If it's a doctor, they might not be the creator, but they might have access to the patient's records in general.
		// The GetMedicalRecordsForPatient and GetMedicalRecordByID already handle this logic for the record itself.
		// For simplicity here, if they are a doctor, we assume they have passed the previous checks to get to this point
		// or that general doctor access to any attachment (if record is accessible) is okay.
		// A stricter check would re-verify access to medicalRecord.ID similar to GetMedicalRecordByID.
		// For now, if not a doctor and not the patient owner, deny.
		utils.Forbidden(c, "You are not authorized to view this attachment.")
		return
	}

	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", attachment.FileName))
	c.Data(http.StatusOK, attachment.FileType, attachment.FileData)
}

// DeleteMedicalRecord handles deleting a medical record.
// Only accessible by the doctor who created it or an admin.
func (h *MedicalRecordHandler) DeleteMedicalRecord(c *gin.Context) {
	// Placeholder implementation - actual deletion logic needed
	c.JSON(http.StatusNotImplemented, gin.H{"message": "DeleteMedicalRecord not yet implemented."})
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

	// Use strings.EqualFold for case-insensitive role comparison.
	isDoctor := strings.EqualFold(string(requestingUserRole), string(models.RoleDoctor))
	isPatientOwner := strings.EqualFold(string(requestingUserRole), string(models.RolePatient)) && requestingUserIDStr == record.PatientID

	if !(isDoctor || isPatientOwner) {
		utils.Forbidden(c, "You are not authorized to view this medical record")
		return
	}

	utils.Success(c, "Medical record fetched successfully", record)
}

// UpdateMedicalRecordRequest represents the request body for updating a medical record.
type UpdateMedicalRecordRequest struct {
	RecordType models.MedicalRecordType `json:"recordType,omitempty"`
	RecordDate string                   `json:"recordDate,omitempty"` // Added to allow date updates
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
	isAdmin := strings.EqualFold(string(requestingUserRole), string(models.RoleAdmin))
	isCreatorDoctor := strings.EqualFold(string(requestingUserRole), string(models.RoleDoctor)) && requestingUserIDStr == record.DoctorID

	if !(isAdmin || isCreatorDoctor) {
		utils.Forbidden(c, "You are not authorized to update this medical record")
		return
	}

	// Apply updates
	if req.RecordType != "" {
		record.RecordType = req.RecordType
	}
	if req.RecordDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, req.RecordDate)
		if err != nil {
			utils.BadRequest(c, "Invalid date format for recordDate. Please use ISO 8601 format (YYYY-MM-DDTHH:MM:SSZ)")
			return
		}
		record.RecordDate = parsedDate
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
