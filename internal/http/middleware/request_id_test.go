package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
)

func TestCorrelationAndRequestID(t *testing.T) {
	tests := []struct {
		name                  string
		inputRequestID        string
		inputCorrelationID    string
		expectGeneratedReqID  bool
		expectReqID           string
		expectCorrID          string
		expectFallbackToReqID bool
	}{
		{
			name:                  "Generate Request ID if empty",
			inputRequestID:        "",
			inputCorrelationID:    "",
			expectGeneratedReqID:  true,
			expectFallbackToReqID: true,
		},
		{
			name:                  "Keep valid Request ID and Correlation ID",
			inputRequestID:        "req-abc-123",
			inputCorrelationID:    "corr:xyz_456",
			expectGeneratedReqID:  false,
			expectReqID:           "req-abc-123",
			expectCorrID:          "corr:xyz_456",
			expectFallbackToReqID: false,
		},
		{
			name:                  "Discard malformed Correlation ID",
			inputRequestID:        "req-abc-123",
			inputCorrelationID:    "bad-corr-id-with-spaces and <xml>",
			expectGeneratedReqID:  false,
			expectReqID:           "req-abc-123",
			expectCorrID:          "",
			expectFallbackToReqID: true,
		},
		{
			name:                  "Regenerate excessively long Request ID",
			inputRequestID:        strings.Repeat("a", 101),
			inputCorrelationID:    "valid-corr",
			expectGeneratedReqID:  true,
			expectCorrID:          "valid-corr",
			expectFallbackToReqID: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(middleware.CorrelationAndRequestID())

			var capturedReqID, capturedCorrID string

			app.Get("/test", func(c fiber.Ctx) error {
				capturedReqID = c.Get("X-Request-Id")
				capturedCorrID = c.Get("X-Correlation-Id")
				return c.SendStatus(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.inputRequestID != "" {
				req.Header.Set("X-Request-ID", tc.inputRequestID)
			}
			if tc.inputCorrelationID != "" {
				req.Header.Set("X-Correlation-ID", tc.inputCorrelationID)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			// Check captured values in context
			if tc.expectGeneratedReqID {
				if len(capturedReqID) != 24 { // random hex length (12 bytes)
					t.Errorf("expected 24 character hex request ID, got %q", capturedReqID)
				}
			} else {
				if capturedReqID != tc.expectReqID {
					t.Errorf("expected request ID %q, got %q", tc.expectReqID, capturedReqID)
				}
			}

			expectedCorr := tc.expectCorrID
			if tc.expectFallbackToReqID {
				expectedCorr = capturedReqID
			}

			if capturedCorrID != expectedCorr {
				t.Errorf("expected correlation ID %q, got %q", expectedCorr, capturedCorrID)
			}

			// Check response headers are set
			respReqID := resp.Header.Get("X-Request-ID")
			if respReqID != capturedReqID {
				t.Errorf("expected X-Request-ID response header %q, got %q", capturedReqID, respReqID)
			}
			respReqIdLower := resp.Header.Get("X-Request-Id")
			if respReqIdLower != capturedReqID {
				t.Errorf("expected X-Request-Id response header %q, got %q", capturedReqID, respReqIdLower)
			}

			respCorrID := resp.Header.Get("X-Correlation-ID")
			if respCorrID != capturedCorrID {
				t.Errorf("expected X-Correlation-ID response header %q, got %q", capturedCorrID, respCorrID)
			}
		})
	}
}
