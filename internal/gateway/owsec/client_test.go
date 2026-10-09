package owsec_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owsec"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

func TestOWSecClient_GetUsers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-sec-token" {
			t.Errorf("missing or invalid auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-INTERNAL-NAME") != "mango-mdu-service" {
			t.Errorf("missing or invalid internal name header: %s", r.Header.Get("X-INTERNAL-NAME"))
		}
		if r.Header.Get("X-API-KEY") != "test-sec-api-key" {
			t.Errorf("missing or invalid api key header: %s", r.Header.Get("X-API-KEY"))
		}
		if r.Header.Get("User-Agent") != "mango-mdu-service/1.0" {
			t.Errorf("missing user-agent: %s", r.Header.Get("User-Agent"))
		}

		resp := models.SecUserListResponse{
			Users: []models.SecUser{
				{ID: "usr-1", Name: "Anita Sharma", Email: "anita@example.com", UserRole: "noc"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := owsec.NewClient(owsec.Config{
		URLResolver: func() string { return ts.URL },
		InternalKey: "test-sec-api-key",
	})
	users, err := client.GetUsers(context.Background(), "test-sec-token", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].Name != "Anita Sharma" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestOWSecClient_Unreachable(t *testing.T) {
	client := owsec.NewClient(owsec.Config{
		URLResolver: func() string { return "http://127.0.0.1:1" }, // closed port
		InternalKey: "test-sec-api-key",
	})
	_, err := client.GetUsers(context.Background(), "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(models.ApiError)
	if !ok || apiErr.ErrorCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 ApiError, got %+v", err)
	}
}

func TestOWSecClient_DualAuthentication(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-INTERNAL-NAME") != "https://localhost:17005" {
			t.Errorf("missing or incorrect X-INTERNAL-NAME: %s", r.Header.Get("X-INTERNAL-NAME"))
		}
		if r.Header.Get("X-API-KEY") != "test-mdu-api-key" {
			t.Errorf("missing or incorrect X-API-KEY: %s", r.Header.Get("X-API-KEY"))
		}
		if r.Header.Get("Authorization") != "Bearer test-user-token" {
			t.Errorf("missing or incorrect Authorization: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Request-Id") != "req-dual-1" {
			t.Errorf("missing or incorrect X-Request-Id: %s", r.Header.Get("X-Request-Id"))
		}
		if r.Header.Get("X-Correlation-Id") != "corr-dual-1" {
			t.Errorf("missing or incorrect X-Correlation-Id: %s", r.Header.Get("X-Correlation-Id"))
		}

		resp := models.SecUserListResponse{
			Users: []models.SecUser{
				{ID: "usr-2", Name: "Bob Smith", Email: "bob@example.com", UserRole: "admin"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := owsec.NewClient(owsec.Config{
		URLResolver:  func() string { return ts.URL },
		InternalName: "https://localhost:17005",
		InternalKey:  "test-mdu-api-key",
	})
	users, err := client.GetUsers(context.Background(), "test-user-token", "req-dual-1", "corr-dual-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].Name != "Bob Smith" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestOWSecClient_KeyResolver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-INTERNAL-NAME") != "https://localhost:17005" {
			t.Errorf("missing or incorrect X-INTERNAL-NAME: %s", r.Header.Get("X-INTERNAL-NAME"))
		}
		if r.Header.Get("X-API-KEY") != "discovered-owsec-key" {
			t.Errorf("expected discovered-owsec-key, got: %s", r.Header.Get("X-API-KEY"))
		}

		resp := models.SecUserListResponse{
			Users: []models.SecUser{
				{ID: "usr-3", Name: "Carol Danvers", Email: "carol@example.com", UserRole: "csr"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := owsec.NewClient(owsec.Config{
		URLResolver:  func() string { return ts.URL },
		KeyResolver:  func() string { return "discovered-owsec-key" },
		InternalName: "https://localhost:17005",
	})
	users, err := client.GetUsers(context.Background(), "test-user-token", "req-key-1", "corr-key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].Name != "Carol Danvers" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestOWSecClient_DownstreamError_Sanitized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"internal_error":"FATAL: connection to sec-db-01.lan:5432 failed"}`))
	}))
	defer ts.Close()

	client := owsec.NewClient(owsec.Config{
		URLResolver: func() string { return ts.URL },
		InternalKey: "test-sec-api-key",
	})
	_, err := client.GetUsers(context.Background(), "token", "req-1", "corr-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(models.ApiError)
	if !ok {
		t.Fatalf("expected ApiError, got %T: %v", err, err)
	}
	if apiErr.ErrorCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", apiErr.ErrorCode)
	}
	if apiErr.ErrorDescription != "Downstream Error" {
		t.Errorf("expected 'Downstream Error', got %q", apiErr.ErrorDescription)
	}
	// Verify raw body is NOT leaked
	if strings.Contains(apiErr.ErrorDetails, "sec-db-01.lan") {
		t.Errorf("sensitive body leaked in ErrorDetails: %s", apiErr.ErrorDetails)
	}
	if apiErr.ErrorDetails != "downstream OWSEC returned status 500" {
		t.Errorf("expected sanitized message, got %q", apiErr.ErrorDetails)
	}
}

func TestOWSecClient_InstanceResolver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-INTERNAL-NAME") != "https://localhost:17005" {
			t.Errorf("missing or incorrect X-INTERNAL-NAME: %s", r.Header.Get("X-INTERNAL-NAME"))
		}
		if r.Header.Get("X-API-KEY") != "atomic-instance-key" {
			t.Errorf("expected atomic-instance-key, got: %s", r.Header.Get("X-API-KEY"))
		}

		resp := models.SecUserListResponse{
			Users: []models.SecUser{
				{ID: "usr-4", Name: "Dave Miller", Email: "dave@example.com", UserRole: "admin"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := owsec.NewClient(owsec.Config{
		InstanceResolver: func() (string, string, error) {
			return ts.URL, "atomic-instance-key", nil
		},
		InternalName: "https://localhost:17005",
	})
	users, err := client.GetUsers(context.Background(), "test-token", "req-inst-1", "corr-inst-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].Name != "Dave Miller" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestOWSecClient_MissingAPIKey(t *testing.T) {
	client := owsec.NewClient(owsec.Config{
		URLResolver: func() string { return "http://127.0.0.1:8080" },
	})
	_, err := client.GetUsers(context.Background(), "token", "req", "corr")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(models.ApiError)
	if !ok || apiErr.ErrorCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 ApiError, got %+v", err)
	}
}
