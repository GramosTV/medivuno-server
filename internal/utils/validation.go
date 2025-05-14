package utils

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Validate performs validation on a struct.
func Validate(s interface{}) error {
	validate := validator.New()
	return validate.Struct(s)
}

// FormatValidationError formats validation errors into a readable string.
func FormatValidationError(err error) string {
	if errs, ok := err.(validator.ValidationErrors); ok {
		var errorMessages []string
		for _, e := range errs {
			errorMessages = append(errorMessages, e.Translate(nil)) // Requires a translator for user-friendly messages
		}
		return strings.Join(errorMessages, ", ")
	}
	return err.Error()
}

// BindAndValidate binds the request body to a struct and validates it.
// If validation fails, it sends a BadRequest response and returns false.
func BindAndValidate(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		BadRequest(c, "Invalid request payload: "+err.Error())
		return false
	}
	if err := Validate(obj); err != nil {
		BadRequest(c, "Validation failed: "+FormatValidationError(err))
		return false
	}
	return true
}
