package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/ra-common-mods/apperror"
	"github.com/routerarchitects/ra-common-mods/logger"
)

type ApiErrorResponse struct {
	ErrorCode        int    `json:"errorCode"`
	ErrorDetails     string `json:"errorDetails"`
	ErrorDescription string `json:"errorDescription"`
}

func ErrorHandler(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// Default values
	statusCode := http.StatusInternalServerError
	errDesc := "Internal Server Error"
	errDetails := err.Error()

	// Check if fiber.Error (e.g. 404 route not found, 400 bad JSON)
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		statusCode = fiberErr.Code
		errDetails = fiberErr.Message
		errDesc = mapHttpStatusToDesc(statusCode)
	} else {
		// Extract apperror properties
		code := apperror.CodeOf(err)
		statusCode = mapCodeToHttpStatus(code)
		errDesc = mapCodeToDesc(code)
		errDetails = apperror.MessageOf(err)
	}

	// Extract request and correlation IDs
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")

	// Log the original raw error internally with request and correlation IDs
	logFields := []any{
		slog.String("error", err.Error()),
		slog.Int("status_code", statusCode),
	}
	if reqID != "" {
		logFields = append(logFields, slog.String("request_id", reqID))
	}
	if corrID != "" {
		logFields = append(logFields, slog.String("correlation_id", corrID))
	}

	if statusCode >= 500 {
		logger.Subsystem("server").Error("Server error", logFields...)
	} else {
		logger.Subsystem("server").Warn("Client error", logFields...)
	}

	// Sanitize error details for public response to avoid leaking internal/downstream details
	if statusCode >= 500 {
		code := apperror.CodeOf(err)
		errStr := err.Error()

		if code == apperror.Code("DOWNSTREAM_UNAVAILABLE") {
			if strings.Contains(errStr, "owsec") || strings.Contains(errStr, "token validation") || strings.Contains(errStr, "API key validation") {
				errDetails = "Authentication service is temporarily unavailable"
			} else if strings.Contains(errStr, "owprov") {
				errDetails = "Provisioning service is temporarily unavailable"
			} else {
				errDetails = "Downstream service is temporarily unavailable"
			}
		} else if code == apperror.Code("DOWNSTREAM_TIMEOUT") || code == apperror.Code("TIMEOUT") {
			if strings.Contains(errStr, "owsec") {
				errDetails = "Authentication service timed out"
			} else if strings.Contains(errStr, "owprov") {
				errDetails = "Provisioning service timed out"
			} else {
				errDetails = "Downstream service timed out"
			}
		} else {
			errDetails = "An internal server error occurred"
		}
	}

	return c.Status(statusCode).JSON(ApiErrorResponse{
		ErrorCode:        statusCode,
		ErrorDetails:     errDetails,
		ErrorDescription: errDesc,
	})
}

func mapCodeToHttpStatus(code apperror.Code) int {
	switch code {
	case apperror.CodeNotFound:
		return http.StatusNotFound
	case apperror.CodeInvalidInput:
		return http.StatusBadRequest
	case apperror.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperror.CodeForbidden:
		return http.StatusForbidden
	case apperror.CodeConflict:
		return http.StatusConflict
	case apperror.Code("TIMEOUT"), apperror.Code("DOWNSTREAM_TIMEOUT"):
		return http.StatusGatewayTimeout
	case apperror.Code("DOWNSTREAM_UNAVAILABLE"):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func mapCodeToDesc(code apperror.Code) string {
	switch code {
	case apperror.CodeNotFound:
		return "Not Found"
	case apperror.CodeInvalidInput:
		return "Bad Request"
	case apperror.CodeUnauthorized:
		return "Unauthorized"
	case apperror.CodeForbidden:
		return "Forbidden"
	case apperror.CodeConflict:
		return "Conflict"
	case apperror.Code("TIMEOUT"), apperror.Code("DOWNSTREAM_TIMEOUT"):
		return "Gateway Timeout"
	case apperror.Code("DOWNSTREAM_UNAVAILABLE"):
		return "Service Unavailable"
	default:
		return "Internal Server Error"
	}
}

func mapHttpStatusToDesc(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusConflict:
		return "Conflict"
	case http.StatusGatewayTimeout:
		return "Gateway Timeout"
	case http.StatusServiceUnavailable:
		return "Service Unavailable"
	default:
		return "Internal Server Error"
	}
}
