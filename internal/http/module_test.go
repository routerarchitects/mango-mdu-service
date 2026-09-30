package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/config"
	provclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	secclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
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
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/entity" {
			json.NewEncoder(w).Encode(provclient.ProvEntityList{Entities: []provclient.ProvEntity{}})
			return
		}
		if r.URL.Path == "/api/v1/venue" {
			json.NewEncoder(w).Encode(provclient.ProvVenueList{Venues: []provclient.ProvVenue{}})
			return
		}
		if r.URL.Path == "/api/v1/inventory" {
			json.NewEncoder(w).Encode(provclient.ProvInventoryList{Taglist: []provclient.ProvInventoryTag{}})
			return
		}
		if r.URL.Path == "/api/v1/operator" {
			json.NewEncoder(w).Encode(provclient.ProvOperatorList{Operators: []provclient.ProvOperator{}})
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

	deps := Dependencies{
		ServerLogger:    slog.New(slog.NewJSONHandler(ioDiscard{}, nil)),
		ServerConfig:    config.ServerConfig{},
		SubsystemConfig: subsystemroutes.Config{},
		AuthEnabled:     true,
		TokenValidator:  &mockTokenValidator{validToken: "my-valid-bearer-token"},
		PrivateAuthConfig: auth.InternalAPIKeyConfig{
			ExpectedAPIKey: "my-secret-internal-key",
		},
	}

	module, err := NewModule(deps)
	if err != nil {
		t.Fatalf("failed to create HTTP module: %v", err)
	}

	// 1. Test 401 Unauthorized (No token provided)
	t.Run("GET System - Missing Token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/system?command=info", nil)
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
	t.Run("GET System - Invalid Token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/system?command=info", nil)
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

	// 3. Test 200 OK (Valid token)
	t.Run("GET System - Success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/system?command=info", nil)
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
	})
}

func TestNewModuleValidation(t *testing.T) {
	deps := Dependencies{
		ServerLogger:    slog.New(slog.NewJSONHandler(ioDiscard{}, nil)),
		ServerConfig:    config.ServerConfig{},
		SubsystemConfig: subsystemroutes.Config{},
		AuthEnabled:     true,
		TokenValidator:  &mockTokenValidator{validToken: "my-valid-bearer-token"},
		PrivateAuthConfig: auth.InternalAPIKeyConfig{
			ExpectedAPIKey: "test-secret",
		},
	}

	module, err := NewModule(deps)
	if err != nil {
		t.Fatalf("expected NewModule to succeed, got error: %v", err)
	}
	if module == nil {
		t.Errorf("expected module to be initialized, got nil")
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
