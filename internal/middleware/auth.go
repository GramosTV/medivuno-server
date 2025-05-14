package middleware

import (
	"healthcare-app-server/internal/config"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware creates a middleware for JWT authentication.
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			utils.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := utils.ValidateToken(tokenString, cfg.JWTSecret)
		if err != nil {
			utils.Unauthorized(c, "Invalid token: "+err.Error())
			c.Abort()
			return
		}

		// Set user information in context for downstream handlers
		c.Set("userID", claims.UserID)
		c.Set("userRole", claims.Role)

		c.Next()
	}
}

// RoleAuthMiddleware creates a middleware for role-based authorization.
// It should be used *after* AuthMiddleware.
func RoleAuthMiddleware(allowedRoles ...models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			utils.InternalServerError(c, "User role not found in context. AuthMiddleware might be missing.")
			c.Abort()
			return
		}

		role, ok := userRole.(models.Role)
		if !ok {
			utils.InternalServerError(c, "User role in context is not of expected type.")
			c.Abort()
			return
		}

		isAllowed := false
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			utils.Forbidden(c, "You do not have permission to access this resource.")
			c.Abort()
			return
		}

		c.Next()
	}
}

// Helper function to get user ID from context
func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return "", false
	}
	idStr, ok := userID.(string)
	return idStr, ok
}

// Helper function to get user role from context
func GetUserRoleFromContext(c *gin.Context) (models.Role, bool) {
	userRole, exists := c.Get("userRole")
	if !exists {
		return "", false
	}
	role, ok := userRole.(models.Role)
	return role, ok
}
