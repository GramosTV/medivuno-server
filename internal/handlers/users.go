package handlers

import (
	"healthcare-app-server/internal/middleware"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserHandler handles user-related requests (typically admin operations).
type UserHandler struct {
	DB *gorm.DB
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{DB: db}
}

// CreateUserRequest represents the request body for creating a user by an admin.
type CreateUserRequest struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	Role      string `json:"role" binding:"required,oneof=PATIENT DOCTOR ADMIN"`
}

// CreateUser handles creating a new user (admin).
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if !utils.BindAndValidate(c, &req) {
		return
	}

	var existingUser models.User
	if err := h.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		utils.BadRequest(c, "User with this email already exists")
		return
	} else if err != gorm.ErrRecordNotFound {
		utils.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	user := models.User{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Role:      models.Role(req.Role),
	}
	if err := user.SetPassword(req.Password); err != nil {
		utils.InternalServerError(c, "Failed to hash password: "+err.Error())
		return
	}

	if err := h.DB.Create(&user).Error; err != nil {
		utils.InternalServerError(c, "Failed to create user: "+err.Error())
		return
	}

	utils.Created(c, "User created successfully", user.Sanitize())
}

// GetUsers handles fetching all users (admin).
func (h *UserHandler) GetUsers(c *gin.Context) {
	var users []models.User
	if err := h.DB.Find(&users).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch users: "+err.Error())
		return
	}

	sanitizedUsers := make([]models.UserSanitized, len(users))
	for i, u := range users {
		sanitizedUsers[i] = u.Sanitize()
	}

	utils.Success(c, "Users fetched successfully", sanitizedUsers)
}

// GetUserByID handles fetching a single user by ID (admin).
func (h *UserHandler) GetUserByID(c *gin.Context) {
	userID := c.Param("id")

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "User not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}
	utils.Success(c, "User fetched successfully", user.Sanitize())
}

// UpdateUserRequest represents the request body for updating a user by an admin.
type UpdateUserRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email,omitempty"` // Allow email update, ensure uniqueness
	Role      string `json:"role,omitempty,oneof=PATIENT DOCTOR ADMIN"`
	// Password should be updated via a separate "change password" endpoint for security
}

// UpdateUser handles updating a user by ID (admin).
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil { // Use ShouldBindJSON for partial updates
		utils.BadRequest(c, "Invalid request payload: "+err.Error())
		return
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		utils.NotFound(c, "User not found")
		return
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Email != "" && req.Email != user.Email {
		// Check if new email is already taken
		var existingUser models.User
		if err := h.DB.Where("email = ? AND id != ?", req.Email, user.ID).First(&existingUser).Error; err == nil {
			utils.BadRequest(c, "New email is already in use")
			return
		} else if err != gorm.ErrRecordNotFound {
			utils.InternalServerError(c, "Database error checking email: "+err.Error())
			return
		}
		user.Email = req.Email
	}
	if req.Role != "" {
		user.Role = models.Role(req.Role)
	}

	if err := h.DB.Save(&user).Error; err != nil {
		utils.InternalServerError(c, "Failed to update user: "+err.Error())
		return
	}

	utils.Success(c, "User updated successfully", user.Sanitize())
}

// DeleteUser handles deleting a user by ID (admin).
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	// Optional: Check if user exists before attempting delete
	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "User not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	// Consider soft delete or handling related records (e.g., appointments)
	if err := h.DB.Delete(&models.User{}, "id = ?", userID).Error; err != nil {
		utils.InternalServerError(c, "Failed to delete user: "+err.Error())
		return
	}

	utils.Success(c, "User deleted successfully", nil)
}

// GetDoctors handles fetching all users with the doctor role.
// This endpoint will be accessible to patients for booking appointments.
func (h *UserHandler) GetDoctors(c *gin.Context) {
	var doctors []models.User
	if err := h.DB.Where("role = ?", models.RoleDoctor).Find(&doctors).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch doctors: "+err.Error())
		return
	}

	sanitizedDoctors := make([]models.UserSanitized, len(doctors))
	for i, doctor := range doctors {
		sanitizedDoctors[i] = doctor.Sanitize()
	}

	utils.Success(c, "Doctors fetched successfully", sanitizedDoctors)
}

// GetDoctorPatients handles fetching all patients.
// This endpoint is accessible to doctors and admins.
func (h *UserHandler) GetDoctorPatients(c *gin.Context) {
	_, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	userRole, _ := middleware.GetUserRoleFromContext(c)
	userRoleLower := strings.ToLower(string(userRole))

	// Only doctors and admins can access this endpoint
	if userRoleLower != "doctor" && userRoleLower != "admin" {
		utils.Forbidden(c, "Only doctors and admins can view patient lists")
		return
	}

	var patients []models.User
	var err error

	// Doctors and Admins should see all patients
	err = h.DB.Where("role = ?", models.RolePatient).Find(&patients).Error

	if err != nil {
		utils.InternalServerError(c, "Failed to fetch patients: "+err.Error())
		return
	}

	// Sanitize patient data before sending
	sanitizedPatients := make([]models.UserSanitized, len(patients))
	for i, patient := range patients {
		sanitizedPatients[i] = patient.Sanitize()
	}

	utils.Success(c, "Patients fetched successfully", sanitizedPatients)
}
