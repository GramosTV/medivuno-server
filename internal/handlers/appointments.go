package handlers

import (
	"healthcare-app-server/internal/middleware"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/utils"
	"strings"
	"time"

	// "net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AppointmentHandler handles appointment related requests.
type AppointmentHandler struct {
	DB *gorm.DB
}

// NewAppointmentHandler creates a new AppointmentHandler.
func NewAppointmentHandler(db *gorm.DB) *AppointmentHandler {
	return &AppointmentHandler{DB: db}
}

// CreateAppointmentRequest represents the request body for creating an appointment.
type CreateAppointmentRequest struct {
	DoctorID  string    `json:"doctorId" binding:"required,uuid"`
	PatientID string    `json:"patientId" binding:"required,uuid"` // Should be set from authenticated user (patient)
	StartTime time.Time `json:"startTime" binding:"required"`
	Reason    string    `json:"reason" binding:"required"`
	Notes     string    `json:"notes"`
}

// CreateAppointment handles creating a new appointment.
// Typically initiated by a patient.
func (h *AppointmentHandler) CreateAppointment(c *gin.Context) {
	var req CreateAppointmentRequest
	if !utils.BindAndValidate(c, &req) {
		return
	}

	patientIDStr, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "Patient ID not found in token")
		return
	}
	// Ensure the patient ID from token matches the one in request, or that requestor is an admin/doctor booking for patient
	requestingUserRole, _ := middleware.GetUserRoleFromContext(c)
	if requestingUserRole == models.RolePatient && patientIDStr != req.PatientID {
		utils.Forbidden(c, "Patients can only book appointments for themselves.")
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		utils.BadRequest(c, "Invalid Patient ID format")
		return
	}
	doctorID, err := uuid.Parse(req.DoctorID)
	if err != nil {
		utils.BadRequest(c, "Invalid Doctor ID format")
		return
	}

	// Verify doctor exists and is a doctor
	var doctor models.User
	if err := h.DB.Where("id = ? AND role = ?", doctorID, models.RoleDoctor).First(&doctor).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Doctor not found or user is not a doctor")
		} else {
			utils.InternalServerError(c, "Database error verifying doctor: "+err.Error())
		}
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

	// Basic validation for appointment time (e.g., not in the past)
	if req.StartTime.Before(time.Now()) {
		utils.BadRequest(c, "Appointment date must be in the future.")
		return
	}

	// TODO: Add more complex validation (e.g., doctor availability, no overlapping appointments)

	appointment := models.Appointment{
		PatientID: req.PatientID, // Directly assign as string
		DoctorID:  req.DoctorID,  // Directly assign as string
		StartTime: req.StartTime,
		Reason:    req.Reason,
		Notes:     req.Notes,
		Status:    models.StatusPending, // Default status
	}

	if err := h.DB.Create(&appointment).Error; err != nil {
		utils.InternalServerError(c, "Failed to create appointment: "+err.Error())
		return
	}

	utils.Created(c, "Appointment created successfully", appointment)
}

// GetAppointmentsForUser handles fetching appointments for the logged-in user (patient or doctor).
func (h *AppointmentHandler) GetAppointmentsForUser(c *gin.Context) {
	userIDStr, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	userRole, _ := middleware.GetUserRoleFromContext(c)

	// Debug: Log the role that was extracted from the context
	c.Writer.Header().Set("X-Debug-User-Role", string(userRole))

	// Normalize user role to handle case sensitivity issues
	userRoleLower := strings.ToLower(string(userRole))

	var appointments []models.Appointment
	var err error

	query := h.DB.Preload("Patient").Preload("Doctor").Order("start_time asc")

	if userRoleLower == string(models.RolePatient) || userRoleLower == "user" || userRoleLower == "patient" {
		err = query.Where("patient_id = ?", userIDStr).Find(&appointments).Error
	} else if userRoleLower == string(models.RoleDoctor) || userRoleLower == "doctor" {
		err = query.Where("doctor_id = ?", userIDStr).Find(&appointments).Error
	} else if userRoleLower == string(models.RoleAdmin) || userRoleLower == "admin" { // Admins can see all appointments
		err = query.Find(&appointments).Error
	} else {
		utils.Forbidden(c, "User role not permitted to view appointments this way. Role: "+string(userRole))
		return
	}

	if err != nil {
		utils.InternalServerError(c, "Failed to fetch appointments: "+err.Error())
		return
	}

	utils.Success(c, "Appointments fetched successfully", appointments)
}

// GetAppointmentByID handles fetching a single appointment by its ID.
// Accessible by involved patient, doctor, or an admin.
func (h *AppointmentHandler) GetAppointmentByID(c *gin.Context) {
	appointmentIDStr := c.Param("id")
	appointmentID, err := uuid.Parse(appointmentIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Appointment ID format")
		return
	}

	var appointment models.Appointment
	if err := h.DB.Preload("Patient").Preload("Doctor").First(&appointment, "id = ?", appointmentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Appointment not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	userIDStr, _ := middleware.GetUserIDFromContext(c)
	userRole, _ := middleware.GetUserRoleFromContext(c)

	isPatientInvolved := userIDStr == appointment.PatientID
	isDoctorInvolved := userIDStr == appointment.DoctorID

	if userRole != models.RoleAdmin && !isPatientInvolved && !isDoctorInvolved {
		utils.Forbidden(c, "You are not authorized to view this appointment")
		return
	}

	utils.Success(c, "Appointment fetched successfully", appointment)
}

// UpdateAppointmentStatusRequest represents the request body for updating an appointment's status.
type UpdateAppointmentStatusRequest struct {
	Status models.AppointmentStatus `json:"status" binding:"required,oneof=PENDING CONFIRMED CANCELLED COMPLETED"`
	Notes  string                   `json:"notes"` // Optional notes for status change (e.g., cancellation reason)
}

// UpdateAppointmentStatus handles updating the status of an appointment.
// Typically by a doctor or admin, or patient (for cancellation).
func (h *AppointmentHandler) UpdateAppointmentStatus(c *gin.Context) {
	appointmentIDStr := c.Param("id")
	appointmentID, err := uuid.Parse(appointmentIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Appointment ID format")
		return
	}

	var req UpdateAppointmentStatusRequest
	if !utils.BindAndValidate(c, &req) {
		return
	}

	var appointment models.Appointment
	if err := h.DB.First(&appointment, "id = ?", appointmentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Appointment not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	userIDStr, _ := middleware.GetUserIDFromContext(c)
	userRole, _ := middleware.GetUserRoleFromContext(c)

	// Authorization logic:
	// - Patient can cancel their own appointments (if status allows)
	// - Doctor can update status for their appointments
	// - Admin can update any appointment
	canUpdate := false
	if userRole == models.RoleAdmin {
		canUpdate = true
	} else if userRole == models.RoleDoctor && userIDStr == appointment.DoctorID {
		canUpdate = true
	} else if userRole == models.RolePatient && userIDStr == appointment.PatientID {
		// Patients can only cancel, and only if it's currently scheduled or confirmed
		if req.Status == models.StatusCancelled &&
			(appointment.Status == models.StatusPending || appointment.Status == models.StatusConfirmed) {
			canUpdate = true
		} else if req.Status != models.StatusCancelled {
			utils.Forbidden(c, "Patients can only cancel appointments.")
			return
		}
	}

	if !canUpdate {
		utils.Forbidden(c, "You are not authorized to update this appointment's status or perform this status transition.")
		return
	}

	appointment.Status = req.Status
	if req.Notes != "" {
		// Uncomment the preferred behavior:
		// Overwrite notes:
		appointment.Notes = req.Notes
		// Or append to existing notes:
		// appointment.Notes += "\nStatus Update: " + req.Notes
	}

	if err := h.DB.Save(&appointment).Error; err != nil {
		utils.InternalServerError(c, "Failed to update appointment status: "+err.Error())
		return
	}

	utils.Success(c, "Appointment status updated successfully", appointment)
}

// RescheduleAppointmentRequest represents the request body for rescheduling an appointment.
type RescheduleAppointmentRequest struct {
	NewAppointmentAt time.Time `json:"newAppointmentAt" binding:"required"`
	Notes            string    `json:"notes"` // Optional notes for rescheduling
}

// RescheduleAppointment handles rescheduling an appointment.
// Typically by a doctor or admin, or patient if allowed.
func (h *AppointmentHandler) RescheduleAppointment(c *gin.Context) {
	appointmentIDStr := c.Param("id")
	appointmentID, err := uuid.Parse(appointmentIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Appointment ID format")
		return
	}

	var req RescheduleAppointmentRequest
	if !utils.BindAndValidate(c, &req) {
		return
	}

	if req.NewAppointmentAt.Before(time.Now()) {
		utils.BadRequest(c, "New appointment date must be in the future.")
		return
	}

	var appointment models.Appointment
	if err := h.DB.First(&appointment, "id = ?", appointmentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Appointment not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	userIDStr, _ := middleware.GetUserIDFromContext(c)
	userRole, _ := middleware.GetUserRoleFromContext(c)

	// Authorization: Similar to UpdateAppointmentStatus, define who can reschedule.
	// For simplicity, let's say only Doctor involved or Admin can reschedule.
	// Patient might need to cancel and re-book.
	canReschedule := false
	if userRole == models.RoleAdmin {
		canReschedule = true
	} else if userRole == models.RoleDoctor && userIDStr == appointment.DoctorID {
		canReschedule = true
	} else if userRole == models.RolePatient && userIDStr == appointment.PatientID {
		// Allow patient to reschedule if appointment is not too soon or already passed
		if appointment.Status == models.StatusPending || appointment.Status == models.StatusConfirmed {
			// Add more conditions, e.g., cannot reschedule within 24 hours of appointment time
			canReschedule = true
		}
	}

	if !canReschedule {
		utils.Forbidden(c, "You are not authorized to reschedule this appointment.")
		return
	}
	// Update the existing appointment object instead of creating a new one
	appointment.StartTime = req.NewAppointmentAt  // Assuming NewAppointmentAt maps to StartTime
	appointment.Status = models.StatusRescheduled // Reset status to rescheduled after reschedule

	if req.Notes != "" {
		appointment.Notes = req.Notes // Or append
	}

	if err := h.DB.Save(&appointment).Error; err != nil {
		utils.InternalServerError(c, "Failed to reschedule appointment: "+err.Error())
		return
	}

	utils.Success(c, "Appointment rescheduled successfully", appointment)
}
