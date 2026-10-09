package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type mockPolicyService struct {
	getPolicyOverviewFn func(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error)
}

func (m *mockPolicyService) GetPolicyOverview(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error) {
	if m.getPolicyOverviewFn != nil {
		return m.getPolicyOverviewFn(ctx, policyID, token, reqID, corrID)
	}
	return nil, nil
}

func setupTestApp(svc *mockPolicyService) *fiber.App {
	app := fiber.New()
	handler := handlers.NewPolicyHandler(svc)
	app.Get("/api/v1/policy/:id/overview", handler.GetOverview)
	return app
}

func TestPolicyHandler_GetOverview_Success(t *testing.T) {
	validID := "523e4567-e89b-12d3-a456-426614174000"
	svc := &mockPolicyService{
		getPolicyOverviewFn: func(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error) {
			return &models.PolicyOverviewResponse{
				Policy: models.PolicyMetadata{
					ID:   validID,
					Name: "Test Policy",
				},
				TotalUsers: 1,
			}, nil
		},
	}

	app := setupTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/policy/"+validID+"/overview", nil)
	req.Header.Set("Authorization", "Bearer token-123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var data models.PolicyOverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data.Policy.Name != "Test Policy" {
		t.Errorf("expected policy name 'Test Policy', got %q", data.Policy.Name)
	}
}

func TestPolicyHandler_GetOverview_ApiError(t *testing.T) {
	svc := &mockPolicyService{
		getPolicyOverviewFn: func(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error) {
			return nil, models.NewApiError(http.StatusNotFound, "Not Found", "Policy not found")
		},
	}

	app := setupTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/policy/00000000-0000-0000-0000-000000000000/overview", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	var apiErr models.ApiError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("failed to decode ApiError: %v", err)
	}
	if apiErr.ErrorCode != 404 || apiErr.ErrorDescription != "Not Found" {
		t.Errorf("unexpected ApiError: %+v", apiErr)
	}
}

func TestPolicyHandler_GetOverview_InternalError(t *testing.T) {
	svc := &mockPolicyService{
		getPolicyOverviewFn: func(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error) {
			return nil, errors.New("unexpected database glitch")
		},
	}

	app := setupTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/policy/523e4567-e89b-12d3-a456-426614174000/overview", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var apiErr models.ApiError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		t.Fatalf("failed to decode ApiError: %v", err)
	}
	if apiErr.ErrorCode != 500 {
		t.Errorf("expected 500, got %d", apiErr.ErrorCode)
	}
	if apiErr.ErrorDetails != "Internal server error" {
		t.Errorf("expected 'Internal server error', got %q", apiErr.ErrorDetails)
	}
}
