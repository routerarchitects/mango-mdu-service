package middleware

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type ApiErrorResponse struct {
	ErrorCode        int    `json:"ErrorCode"`
	ErrorDetails     string `json:"ErrorDetails"`
	ErrorDescription string `json:"ErrorDescription"`
}

func ErrorHandler(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// Default values
	statusCode := http.StatusInternalServerError
	errDesc := "internal_error"
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
	default:
		return http.StatusInternalServerError
	}
}

func mapCodeToDesc(code apperror.Code) string {
	switch code {
	case apperror.CodeNotFound:
		return "not_found"
	case apperror.CodeInvalidInput:
		return "validation_error"
	case apperror.CodeUnauthorized:
		return "unauthorized"
	case apperror.CodeForbidden:
		return "forbidden"
	case apperror.CodeConflict:
		return "conflict"
	case apperror.Code("TIMEOUT"), apperror.Code("DOWNSTREAM_TIMEOUT"):
		return "downstream_timeout"
	default:
		return "internal_error"
	}
}

func mapHttpStatusToDesc(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusGatewayTimeout:
		return "downstream_timeout"
	default:
		return "internal_error"
	}
}
