package models

import "fmt"

// ApiError represents the normalized OpenWiFi/MDU API error response payload.
type ApiError struct {
	ErrorCode        int    `json:"ErrorCode"`
	ErrorDescription string `json:"ErrorDescription"`
	ErrorDetails     string `json:"ErrorDetails,omitempty"`
}

// Error implements the standard Go error interface.
func (e ApiError) Error() string {
	if e.ErrorDetails != "" {
		return fmt.Sprintf("%d %s: %s", e.ErrorCode, e.ErrorDescription, e.ErrorDetails)
	}
	return fmt.Sprintf("%d %s", e.ErrorCode, e.ErrorDescription)
}

// NewApiError creates a new ApiError envelope.
func NewApiError(code int, description string, details ...string) ApiError {
	err := ApiError{
		ErrorCode:        code,
		ErrorDescription: description,
	}
	if len(details) > 0 && details[0] != "" {
		err.ErrorDetails = details[0]
	}
	return err
}
