package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/config"
	provclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	secclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	subsystemroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

type mockTokenValidator struct {
	validToken string
}

func (m *mockTokenValidator) ValidateToken(ctx context.Context, token string) error {
	if token == m.validToken {
		return nil
	}
	return fmt.Errorf("invalid token")
}

func (m *mockTokenValidator) ValidateAPIKey(ctx context.Context, apiKey string) error {
	return nil
}

func TestModuleAuthenticationAndHeaderPropagation(t *testing.T) {
	// Setup a mock PROV server to assert header propagation
	var lastReceivedRequestID string
	var lastReceivedCorrelationID string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		lastReceivedRequestID = r.Header.Get("X-Request-Id")
		lastReceivedCorrelationID = r.Header.Get("X-Correlation-Id")

		if r.URL.Path == "/api/v1/entity" {
			if r.URL.Query().Get("getTree") == "true" {
				tree := provclient.ProvTreeNode{
					UUID: "00000000-0000-0000-0000-000000000000",
					Name: "Root",
					Type: "entity",
				}
				json.NewEncoder(w).Encode(tree)
				return
			}
			json.NewEncoder(w).Encode(provclient.ProvEntityList{Entities: []provclient.ProvEntity{}})
			return
		}
		if r.URL.Path == "/api/v1/venue" {
			json.NewEncoder(w).Encode(provclient.ProvVenueList{Venues: []provclient.ProvVenue{}})
			return
		}
		if r.URL.Path == "/api/v1/managementRole" {
			json.NewEncoder(w).Encode(provclient.ProvManagementRoleList{Roles: []provclient.ProvManagementRole{}})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Initialize real clients targeting mockServer
	provClient, err := provclient.NewClient(nil, "", "mdu-test")
	if err != nil {
		t.Fatalf("failed to create prov client: %v", err)
	}
	provClient.BaseURL = mockServer.URL

	secClient, err := secclient.NewClient(nil, "", "mdu-test")
	if err != nil {
		t.Fatalf("failed to create sec client: %v", err)
	}
	secClient.BaseURL = mockServer.URL

	// Services & Handlers
	sessionService := services.NewSessionService(secClient, provClient)
	sessionHandler := handlers.NewSessionHandler(sessionService, true)

	hierarchyService := services.NewHierarchyService(provClient)
	hierarchyHandler := handlers.NewHierarchyHandler(hierarchyService)

	entityService := services.NewEntityService(provClient)
	entityHandler := handlers.NewEntityHandler(entityService)

	venueService := services.NewVenueService(provClient)
	venueHandler := handlers.NewVenueHandler(venueService)

	assignmentService := services.NewAssignmentService(provClient)
	assignmentHandler := handlers.NewAssignmentHandler(assignmentService)

	deps := Dependencies{
		ServerLogger:    slog.New(slog.NewJSONHandler(ioDiscard{}, nil)),
		ServerConfig:    config.ServerConfig{},
		SubsystemConfig: subsystemroutes.Config{},
		AuthEnabled:     true,
		TokenValidator:  &mockTokenValidator{validToken: "my-valid-bearer-token"},
		PrivateAuthConfig: auth.InternalAPIKeyConfig{
			ExpectedAPIKey: "my-secret-internal-key",
		},
		SessionHandler:    sessionHandler,
		HierarchyHandler:  hierarchyHandler,
		EntityHandler:     entityHandler,
		VenueHandler:      venueHandler,
		AssignmentHandler: assignmentHandler,
	}

	module, err := NewModule(deps)
	if err != nil {
		t.Fatalf("failed to create HTTP module: %v", err)
	}

	// 1. Test 401 Unauthorized (No token provided)
	t.Run("GET Hierarchy - Missing Token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/hierarchy", nil)
		resp, err := module.publicApp.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	// 2. Test 401 Unauthorized (Invalid token provided)
	t.Run("GET Hierarchy - Invalid Token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/hierarchy", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		resp, err := module.publicApp.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	// 3. Test 200 OK (Valid token) & Tracing Header Propagation
	t.Run("GET Hierarchy - Success and Header Propagation", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/hierarchy", nil)
		req.Header.Set("Authorization", "Bearer my-valid-bearer-token")
		req.Header.Set("X-Request-Id", "req-id-12345")
		req.Header.Set("X-Correlation-Id", "corr-id-67890")

		resp, err := module.publicApp.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, string(bodyBytes))
		}

		// Assert downstream client header propagation
		if lastReceivedRequestID != "req-id-12345" {
			t.Errorf("expected request ID 'req-id-12345' to propagate to PROV, got '%s'", lastReceivedRequestID)
		}
		if lastReceivedCorrelationID != "corr-id-67890" {
			t.Errorf("expected correlation ID 'corr-id-67890' to propagate to PROV, got '%s'", lastReceivedCorrelationID)
		}
	})
}

func TestNewModuleValidation(t *testing.T) {
	// Test that NewModule returns an error if any required business handlers are nil
	deps := Dependencies{
		ServerLogger:    slog.New(slog.NewJSONHandler(ioDiscard{}, nil)),
		ServerConfig:    config.ServerConfig{},
		SubsystemConfig: subsystemroutes.Config{},
		AuthEnabled:     true,
		TokenValidator:  &mockTokenValidator{validToken: "my-valid-bearer-token"},
		SessionHandler:  nil, // missing
	}

	_, err := NewModule(deps)
	if err == nil {
		t.Errorf("expected NewModule to return error when session handler is nil, but got nil")
	} else if !strings.Contains(err.Error(), "missing required business handlers") {
		t.Errorf("expected error message to contain 'missing required business handlers', got: %v", err)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
