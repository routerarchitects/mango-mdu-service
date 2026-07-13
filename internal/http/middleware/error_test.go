package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

func TestErrorHandler_Sanitization(t *testing.T) {
	// Setup custom logger to capture logs
	var logBuf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(testLogger)

	tests := []struct {
		name             string
		handlerErr       error
		expectedStatus   int
		expectedDetails  string
		expectedInLog    []string
		requestHeaders   map[string]string
	}{
		{
			name: "Sanitize OWSEC Downstream Unavailable",
			handlerErr: apperror.New(
				apperror.Code("DOWNSTREAM_UNAVAILABLE"),
				"owsec returned status 502: Bad Gateway: stacktrace details here",
			),
			expectedStatus:  http.StatusServiceUnavailable,
			expectedDetails: "Authentication service is temporarily unavailable",
			expectedInLog:   []string{"owsec returned status 502", "request_id", "correlation_id"},
			requestHeaders: map[string]string{
				"X-Request-Id":     "req-123",
				"X-Correlation-Id": "corr-456",
			},
		},
		{
			name: "Sanitize OWPROV Downstream Unavailable",
			handlerErr: apperror.New(
				apperror.Code("DOWNSTREAM_UNAVAILABLE"),
				"owprov returned status 500: Internal Server Error: some DB dump",
			),
			expectedStatus:  http.StatusServiceUnavailable,
			expectedDetails: "Provisioning service is temporarily unavailable",
			expectedInLog:   []string{"owprov returned status 500"},
		},
		{
			name: "Sanitize Generic Downstream Unavailable",
			handlerErr: apperror.New(
				apperror.Code("DOWNSTREAM_UNAVAILABLE"),
				"something else failed",
			),
			expectedStatus:  http.StatusServiceUnavailable,
			expectedDetails: "Downstream service is temporarily unavailable",
			expectedInLog:   []string{"something else failed"},
		},
		{
			name: "Sanitize Downstream Timeout",
			handlerErr: apperror.New(
				apperror.Code("DOWNSTREAM_TIMEOUT"),
				"owprov timeout occurred",
			),
			expectedStatus:  http.StatusGatewayTimeout,
			expectedDetails: "Provisioning service timed out",
			expectedInLog:   []string{"owprov timeout occurred"},
		},
		{
			name: "Sanitize Internal Error",
			handlerErr: apperror.New(
				apperror.CodeInternal,
				"sql: no rows in result set (highly sensitive table layout info)",
			),
			expectedStatus:  http.StatusInternalServerError,
			expectedDetails: "An internal server error occurred",
			expectedInLog:   []string{"sql: no rows in result set"},
		},
		{
			name:            "Do Not Sanitize Client Error (4xx)",
			handlerErr:      apperror.New(apperror.CodeNotFound, "user not found"),
			expectedStatus:  http.StatusNotFound,
			expectedDetails: "user not found",
			expectedInLog:   []string{"user not found"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logBuf.Reset()

			app := fiber.New(fiber.Config{
				ErrorHandler: middleware.ErrorHandler,
			})

			app.Get("/test", func(c fiber.Ctx) error {
				return tc.handlerErr
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for k, v := range tc.requestHeaders {
				req.Header.Set(k, v)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			var body middleware.ApiErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response JSON: %v", err)
			}

			if body.ErrorCode != tc.expectedStatus {
				t.Errorf("expected JSON errorCode %d, got %d", tc.expectedStatus, body.ErrorCode)
			}

			if body.ErrorDetails != tc.expectedDetails {
				t.Errorf("expected JSON errorDetails %q, got %q", tc.expectedDetails, body.ErrorDetails)
			}

			logOutput := logBuf.String()
			for _, expectedStr := range tc.expectedInLog {
				if !strings.Contains(logOutput, expectedStr) {
					t.Errorf("expected log to contain %q, but got:\n%s", expectedStr, logOutput)
				}
			}
		})
	}
}
