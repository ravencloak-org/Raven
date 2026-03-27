package apierror

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError represents a structured API error response.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Detail != "" {
		return e.Message + ": " + e.Detail
	}
	return e.Message
}

// NewBadRequest creates a 400 Bad Request error.
func NewBadRequest(detail string) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: "Bad Request",
		Detail:  detail,
	}
}

// NewNotFound creates a 404 Not Found error.
func NewNotFound(detail string) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: "Not Found",
		Detail:  detail,
	}
}

// NewUnauthorized creates a 401 Unauthorized error.
func NewUnauthorized(detail string) *AppError {
	return &AppError{
		Code:    http.StatusUnauthorized,
		Message: "Unauthorized",
		Detail:  detail,
	}
}

// NewInternal creates a 500 Internal Server Error.
func NewInternal(detail string) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: "Internal Server Error",
		Detail:  detail,
	}
}

// ErrorHandler is a Gin middleware that catches errors set via c.Error()
// and returns a JSON error response.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if appErr, ok := err.(*AppError); ok {
				c.JSON(appErr.Code, appErr)
			} else {
				c.JSON(http.StatusInternalServerError, &AppError{
					Code:    http.StatusInternalServerError,
					Message: "Internal Server Error",
					Detail:  err.Error(),
				})
			}
		}
	}
}
