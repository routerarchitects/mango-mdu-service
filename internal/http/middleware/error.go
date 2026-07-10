package middleware

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/ra-common-mods/apperror"
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
