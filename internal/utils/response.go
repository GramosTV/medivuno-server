package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ResponseData represents the structure of a standard API response.
type ResponseData struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Success sends a standard success response.
func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, ResponseData{
		Status:  http.StatusOK,
		Message: message,
		Data:    data,
	})
}

// Created sends a standard resource created response.
func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, ResponseData{
		Status:  http.StatusCreated,
		Message: message,
		Data:    data,
	})
}

// Error sends a standard error response.
func Error(c *gin.Context, statusCode int, errorMessage string) {
	c.JSON(statusCode, ResponseData{
		Status:  statusCode,
		Message: "An error occurred",
		Error:   errorMessage,
	})
}

// BadRequest sends a 400 Bad Request error response.
func BadRequest(c *gin.Context, errorMessage string) {
	Error(c, http.StatusBadRequest, errorMessage)
}

// Unauthorized sends a 401 Unauthorized error response.
func Unauthorized(c *gin.Context, errorMessage string) {
	Error(c, http.StatusUnauthorized, errorMessage)
}

// Forbidden sends a 403 Forbidden error response.
func Forbidden(c *gin.Context, errorMessage string) {
	Error(c, http.StatusForbidden, errorMessage)
}

// NotFound sends a 404 Not Found error response.
func NotFound(c *gin.Context, errorMessage string) {
	Error(c, http.StatusNotFound, errorMessage)
}

// InternalServerError sends a 500 Internal Server Error response.
func InternalServerError(c *gin.Context, errorMessage string) {
	Error(c, http.StatusInternalServerError, errorMessage)
}
