package apierror

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/resilience"
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

// NewConflict creates a 409 Conflict error.
func NewConflict(detail string) *AppError {
	return &AppError{
		Code:    http.StatusConflict,
		Message: "Conflict",
		Detail:  detail,
	}
}

// NewTooManyRequests creates a 429 Too Many Requests error.
func NewTooManyRequests(detail string) *AppError {
	return &AppError{
		Code:    http.StatusTooManyRequests,
		Message: "Too Many Requests",
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

// QuotaError extends AppError with billing-specific fields for 402 responses.
type QuotaError struct {
	AppError
	UpgradeRequired bool `json:"upgrade_required"`
	Limit           int  `json:"limit"`
}

// NewPaymentRequired creates a 402 Payment Required error with upgrade context.
func NewPaymentRequired(detail string, limit int) *QuotaError {
	return &QuotaError{
		AppError: AppError{
			Code:    http.StatusPaymentRequired,
			Message: "Payment Required",
			Detail:  detail,
		},
		UpgradeRequired: true,
		Limit:           limit,
	}
}

// ErrorHandler is a Gin middleware that catches errors set via c.Error()
// and returns a JSON error response.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if errors.Is(err, resilience.ErrCircuitOpen) {
				c.Header("Retry-After", "30")
				c.JSON(http.StatusServiceUnavailable, &AppError{
					Code:    http.StatusServiceUnavailable,
					Message: "Service Unavailable",
					Detail:  "circuit breaker open; please retry after 30 seconds",
				})
			} else if quotaErr, ok := err.(*QuotaError); ok {
				c.JSON(quotaErr.Code, quotaErr)
			} else if appErr, ok := err.(*AppError); ok {
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
