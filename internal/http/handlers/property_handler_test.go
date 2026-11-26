package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

type mockPropertyService struct {
	services.PropertyService
	capturedToken  string
	capturedReqID  string
	capturedCorrID string
}

func (m *mockPropertyService) GetPortfolio(ctx context.Context, token, reqID, corrID string) (*models.PropertyPortfolioResponse, error) {
	m.capturedToken = token
	m.capturedReqID = reqID
	m.capturedCorrID = corrID
	return &models.PropertyPortfolioResponse{
		PropertyCount: 1,
	}, nil
}

func TestPropertyHandler_MetadataCloning(t *testing.T) {
	mockSvc := &mockPropertyService{}
	handler := handlers.NewPropertyHandler(mockSvc)

	app := fiber.New()
	app.Get("/api/v1/properties/portfolio", handler.GetPortfolio)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/properties/portfolio", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	req.Header.Set("X-Request-Id", "req-12345")
	req.Header.Set("X-Correlation-Id", "corr-67890")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	if mockSvc.capturedToken != "Bearer test-admin-token" {
		t.Fatalf("expected token 'Bearer test-admin-token', got %q", mockSvc.capturedToken)
	}
	if mockSvc.capturedReqID != "req-12345" {
		t.Fatalf("expected reqID 'req-12345', got %q", mockSvc.capturedReqID)
	}
	if mockSvc.capturedCorrID != "corr-67890" {
		t.Fatalf("expected corrID 'corr-67890', got %q", mockSvc.capturedCorrID)
	}

	var portfolioResp models.PropertyPortfolioResponse
	if err := json.NewDecoder(resp.Body).Decode(&portfolioResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if portfolioResp.PropertyCount != 1 {
		t.Fatalf("expected PropertyCount 1, got %d", portfolioResp.PropertyCount)
	}
}
