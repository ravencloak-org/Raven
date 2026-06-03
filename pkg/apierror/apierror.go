package apierror

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/resilience"
)

// AppError represents a structured API error response.
//
// ErrorCode is an optional machine-readable identifier (snake_case) clients
// can branch on without parsing the human-facing Detail string. It is only
// populated by helpers that want to expose a stable contract — e.g.
// NewKBFrozen / NewKBDMCAPending for KB lifecycle freezes (issue #725) —
// and stays absent from the JSON when empty so existing payloads are
// unaffected.
type AppError struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	ErrorCode string `json:"error,omitempty"`
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

// NewUnprocessableEntity creates a 422 Unprocessable Entity error. Use this
// for payloads whose shape parsed cleanly but whose semantic content was
// rejected by a domain rule — e.g. an SPDX license that is not in the
// Marketplace publish allow-list (ADR-0006). 400 is reserved for malformed
// requests; 422 says "I understood you, the answer is still no".
func NewUnprocessableEntity(detail string) *AppError {
	return &AppError{
		Code:    http.StatusUnprocessableEntity,
		Message: "Unprocessable Entity",
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

// NewKBFrozen creates a 409 Conflict for a write that targeted a KB which
// the lifecycle policy (model.KBStatusGate) currently has frozen against
// ingestion / mutation. The "kb_frozen" code is the stable identifier the
// SPA + widgets branch on (ADR-0004 §Consequences).
func NewKBFrozen(detail string) *AppError {
	return &AppError{
		Code:      http.StatusConflict,
		Message:   "Conflict",
		Detail:    detail,
		ErrorCode: "kb_frozen",
	}
}

// NewKBDMCALocked creates a 423 Locked for an interaction that targeted a
// KB sitting in the DMCA counter-notice window. Distinct status code from
// kb_frozen so clients can branch on legal-hold UX (ADR-0006 / ADR-0008).
func NewKBDMCALocked(detail string) *AppError {
	return &AppError{
		Code:      http.StatusLocked,
		Message:   "Locked",
		Detail:    detail,
		ErrorCode: "kb_dmca_locked",
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
			var cOpen *resilience.CircuitOpenError
			if errors.As(err, &cOpen) {
				retryAfter := int(cOpen.RetryAfter.Round(time.Second).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				c.Header("Retry-After", strconv.Itoa(retryAfter))
				c.JSON(http.StatusServiceUnavailable, &AppError{
					Code:    http.StatusServiceUnavailable,
					Message: "Service Unavailable",
					Detail:  fmt.Sprintf("upstream service temporarily unavailable; retry after %ds", retryAfter),
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
