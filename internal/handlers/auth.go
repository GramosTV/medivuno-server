package handlers

import (
	"healthcare-app-server/internal/config"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/utils"
	"time" // Imported time

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{DB: db, Cfg: cfg}
}

// RegisterRequest represents the request body for user registration.
type RegisterRequest struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	Role      string `json:"role" binding:"required,oneof=PATIENT DOCTOR ADMIN"` // Validate role
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if !utils.BindAndValidate(c, &req) {
		return // Error response handled by BindAndValidate
	}

	// Check if user already exists
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
		Role:      models.Role(req.Role), // Convert string to models.Role
	}

	if err := user.SetPassword(req.Password); err != nil {
		utils.InternalServerError(c, "Failed to hash password: "+err.Error())
		return
	}

	if err := h.DB.Create(&user).Error; err != nil {
		utils.InternalServerError(c, "Failed to create user: "+err.Error())
		return
	}

	// Omit password from response
	userResponse := user.Sanitize()
	utils.Created(c, "User registered successfully", userResponse)
}

// LoginRequest represents the request body for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the response body for successful login.
type LoginResponse struct {
	AccessToken  string               `json:"accessToken"`
	RefreshToken string               `json:"refreshToken"`
	User         models.UserSanitized `json:"user"`
}

// Login handles user login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if !utils.BindAndValidate(c, &req) {
		return
	}

	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Unauthorized(c, "Invalid email or password")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	if !user.CheckPassword(req.Password) {
		utils.Unauthorized(c, "Invalid email or password")
		return
	}

	accessToken, refreshTokenString, err := utils.GenerateTokens(&user, h.Cfg)
	if err != nil {
		utils.InternalServerError(c, "Failed to generate tokens: "+err.Error())
		return
	}
	// Store refresh token in DB
	refreshToken := models.RefreshToken{
		UserID:    user.ID, // Ensure user.ID is the correct UUID string
		Token:     refreshTokenString,
		ExpiresAt: time.Now().Add(time.Duration(h.Cfg.JWTRefreshExpirationHours) * time.Hour),
		IsRevoked: false,
	}
	if err := h.DB.Create(&refreshToken).Error; err != nil {
		utils.InternalServerError(c, "Failed to store refresh token: "+err.Error())
		return
	}

	// Set refresh token as HTTP-only cookie
	c.SetCookie(
		"refresh_token",                       // Name
		refreshTokenString,                    // Value
		h.Cfg.JWTRefreshExpirationHours*60*60, // Max age in seconds
		"/",                                // Path
		"",                                 // Domain (empty means current domain)
		h.Cfg.Environment != "development", // Secure (true in prod, false in dev)
		true,                               // HTTP only
	)

	utils.Success(c, "Login successful", LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString, // Still include in response for backward compatibility
		User:         user.Sanitize(),
	})
}

// RefreshTokenRequest represents the request body for token refresh.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// RefreshTokenResponse represents the response body for successful token refresh.
type RefreshTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// RefreshToken handles refreshing an access token using a refresh token.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// First try to get the refresh token from HTTP-only cookie
	refreshTokenFromCookie, err := c.Cookie("refresh_token")

	// If no cookie, fall back to request body (for backward compatibility)
	if err != nil || refreshTokenFromCookie == "" {
		var req RefreshTokenRequest
		if !utils.BindAndValidate(c, &req) {
			return
		}
		refreshTokenFromCookie = req.RefreshToken
	}

	// Validate the token regardless of source
	claims, err := utils.ValidateToken(refreshTokenFromCookie, h.Cfg.JWTRefreshSecret)
	if err != nil {
		utils.Unauthorized(c, "Invalid refresh token structure or signature: "+err.Error())
		return
	}
	// Check if refresh token is revoked or still valid in DB
	var storedToken models.RefreshToken
	if err := h.DB.Where("token = ? AND user_id = ? AND is_revoked = ? AND expires_at > ?", refreshTokenFromCookie, claims.UserID, false, time.Now()).First(&storedToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Unauthorized(c, "Refresh token not found, expired, or revoked")
		} else {
			utils.InternalServerError(c, "Database error checking refresh token: "+err.Error())
		}
		return
	}

	var user models.User
	// Use claims.UserID which should be the string representation of the UUID
	if err := h.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		utils.InternalServerError(c, "Failed to find user associated with token: "+err.Error())
		return
	}
	// Implement refresh token rotation for security:
	// 1. Revoke the old refresh token
	storedToken.IsRevoked = true
	h.DB.Save(&storedToken)

	// 2. Generate new tokens
	newAccessToken, newRefreshTokenString, err := utils.GenerateTokens(&user, h.Cfg)
	if err != nil {
		utils.InternalServerError(c, "Failed to generate new tokens: "+err.Error())
		return
	}

	// 3. Store the new refresh token in DB
	newRefreshToken := models.RefreshToken{
		UserID:    user.ID,
		Token:     newRefreshTokenString,
		ExpiresAt: time.Now().Add(time.Duration(h.Cfg.JWTRefreshExpirationHours) * time.Hour),
		IsRevoked: false,
	}
	if err := h.DB.Create(&newRefreshToken).Error; err != nil {
		utils.InternalServerError(c, "Failed to store new refresh token: "+err.Error())
		return
	}

	// 4. Set the new refresh token as HTTP-only cookie
	c.SetCookie(
		"refresh_token",                       // Name
		newRefreshTokenString,                 // Value
		h.Cfg.JWTRefreshExpirationHours*60*60, // Max age in seconds
		"/",                                // Path
		"",                                 // Domain (empty means current domain)
		h.Cfg.Environment != "development", // Secure (true in prod, false in dev)
		true,                               // HTTP only
	)

	utils.Success(c, "Access token refreshed successfully", RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshTokenString, // Include for backward compatibility
	})
}

// LogoutRequest represents the request body for user logout.
type LogoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// Logout handles user logout (can involve invalidating tokens if using a denylist).
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request payload: "+err.Error())
		return
	}

	if req.RefreshToken == "" {
		utils.BadRequest(c, "Refresh token is required")
		return
	}

	// Attempt to find the refresh token in the DB
	var storedToken models.RefreshToken
	// We only care if it exists and is not already revoked, UserID isn't strictly necessary for logout
	// as the token itself is unique.
	if err := h.DB.Where("token = ? AND is_revoked = ?", req.RefreshToken, false).First(&storedToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Token not found or already revoked, which is acceptable for logout.
			utils.Success(c, "Logout successful (token not found or already invalid).", nil)
		} else {
			utils.InternalServerError(c, "Database error during logout: "+err.Error())
		}
		return
	}

	// Mark the token as revoked and effectively expire it
	storedToken.IsRevoked = true
	storedToken.ExpiresAt = time.Now() // Optional: force expiry
	if err := h.DB.Save(&storedToken).Error; err != nil {
		utils.InternalServerError(c, "Failed to revoke refresh token: "+err.Error())
		return
	}

	// Clear the refresh token cookie
	c.SetCookie(
		"refresh_token",                    // Name
		"",                                 // Value (empty to delete)
		-1,                                 // MaxAge (negative to expire immediately)
		"/",                                // Path
		"",                                 // Domain
		h.Cfg.Environment != "development", // Secure
		true,                               // HttpOnly
	)

	utils.Success(c, "Logout successful. Refresh token has been invalidated.", nil)
}

// GetProfile handles fetching the currently authenticated user's profile.
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "User profile not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}

	utils.Success(c, "Profile fetched successfully", user.Sanitize())
}

// UpdateProfileRequest represents the request body for updating user profile.
type UpdateProfileRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	// Email cannot be changed via this endpoint for simplicity, handle separately if needed
}

// UpdateProfile handles updating the currently authenticated user's profile.
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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
	// Add other updatable fields here

	if err := h.DB.Save(&user).Error; err != nil {
		utils.InternalServerError(c, "Failed to update profile: "+err.Error())
		return
	}

	utils.Success(c, "Profile updated successfully", user.Sanitize())
}
