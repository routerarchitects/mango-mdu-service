package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	provclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

func TestOperatorAPI(t *testing.T) {
	// 1. Mock owprov downstream server
	mockOperator := provclient.ProvOperator{
		ID:             "op-123",
		Name:           "Original Operator",
		Description:    "Original Description",
		Created:        1719878400, // 2024-07-02T00:00:00Z
		Modified:       1719878400,
		RegistrationID: "reg-123",
		EntityID:       "entity-123",
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Assert headers propagate
		if r.Header.Get("X-API-KEY") != "mock-api-key" {
			http.Error(w, "missing x-api-key", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/api/v1/operator/") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		uuid := strings.TrimPrefix(r.URL.Path, "/api/v1/operator/")

		if uuid == "op-999" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(mockOperator)
		case http.MethodPut:
			var op provclient.ProvOperator
			json.NewDecoder(r.Body).Decode(&op)
			mockOperator = op
			mockOperator.Modified = time.Now().Unix()
			json.NewEncoder(w).Encode(mockOperator)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer mockServer.Close()

	// 2. Setup client, service, and handler
	provClient, err := provclient.NewClient(nil, "", "mdu-test")
	if err != nil {
		t.Fatalf("failed to create prov client: %v", err)
	}
	provClient.BaseURL = mockServer.URL

	service := services.NewOperatorService(provClient)
	handler := handlers.NewOperatorHandler(service)

	// 3. Setup Fiber App with custom error handler
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
	})

	handler.Register(app.Group("/api/v1"))

	// --- TEST GET OPERATOR ---
	t.Run("GET Success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/operators/op-123", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var detail models.OperatorDetail
		json.NewDecoder(resp.Body).Decode(&detail)

		if detail.ID != "op-123" || detail.Name != "Original Operator" {
			t.Errorf("unexpected operator details: %+v", detail)
		}

		expectedTime := time.Unix(1719878400, 0).UTC()
		if !detail.CreatedAt.Equal(expectedTime) {
			t.Errorf("expected CreatedAt %v, got %v", expectedTime, detail.CreatedAt)
		}
	})

	t.Run("GET Not Found", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/operators/op-999", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}

		var apiErr middleware.ApiErrorResponse
		json.NewDecoder(resp.Body).Decode(&apiErr)

		if apiErr.ErrorCode != http.StatusNotFound || apiErr.ErrorDescription != "not_found" {
			t.Errorf("unexpected error payload: %+v", apiErr)
		}
	})

	// --- TEST PUT OPERATOR ---
	t.Run("PUT Update", func(t *testing.T) {
		updateBody := `{"name":"New Name","description":"New Description"}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/operators/op-123", strings.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var detail models.OperatorDetail
		json.NewDecoder(resp.Body).Decode(&detail)

		if detail.Name != "New Name" || detail.Description != "New Description" {
			t.Errorf("expected name and description to update, got %+v", detail)
		}
		// Confirm registration ID was merged/preserved
		if detail.RegistrationID != "reg-123" {
			t.Errorf("expected registration ID to be preserved, got %s", detail.RegistrationID)
		}
	})

	t.Run("PUT Bad JSON", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/operators/op-123", strings.NewReader("invalid-json"))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}

		var apiErr middleware.ApiErrorResponse
		json.NewDecoder(resp.Body).Decode(&apiErr)

		if apiErr.ErrorCode != http.StatusBadRequest || apiErr.ErrorDescription != "validation_error" {
			t.Errorf("unexpected error payload: %+v", apiErr)
		}
	})

	// --- TEST DELETE OPERATOR ---
	t.Run("DELETE Success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/operators/op-123", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status 24, got %d", resp.StatusCode)
		}
	})
}
