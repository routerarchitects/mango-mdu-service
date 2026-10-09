package owprov_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owprov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

func TestOWProvClient_GetPolicy_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/managementPolicy/pol-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or invalid auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("User-Agent") != "mango-mdu-service/1.0" {
			t.Errorf("missing user-agent: %s", r.Header.Get("User-Agent"))
		}

		resp := models.ManagementPolicy{
			ID:   "pol-1",
			Name: "Operator",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	policy, err := client.GetPolicy(context.Background(), "pol-1", "test-token", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.Name != "Operator" {
		t.Errorf("expected 'Operator', got %q", policy.Name)
	}
}

func TestOWProvClient_GetPolicy_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	_, err := client.GetPolicy(context.Background(), "not-found", "test-token", "req-1", "corr-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(models.ApiError)
	if !ok || apiErr.ErrorCode != http.StatusNotFound {
		t.Errorf("expected 404 ApiError, got %+v", err)
	}
}

func TestOWProvClient_GetRolesByPolicy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/managementRole" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("policyId") != "pol-1" {
			t.Errorf("missing policyId query: %s", r.URL.Query().Get("policyId"))
		}

		resp := models.ManagementRoleListResponse{
			Roles: []models.ManagementRole{
				{ID: "role-1", ManagementPolicy: "pol-1"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	roles, err := client.GetRolesByPolicy(context.Background(), "pol-1", "test-token", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 1 || roles[0].ID != "role-1" {
		t.Errorf("unexpected roles: %+v", roles)
	}
}

func TestOWProvClient_GetEntities(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Query().Get("limit") != "500" {
			t.Errorf("expected limit=500, got %s", r.URL.Query().Get("limit"))
		}
		offset := r.URL.Query().Get("offset")
		if offset == "0" {
			resp := models.EntityListResponse{
				Entities: []models.Entity{
					{ID: "ent-1", Name: "Sunrise"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			t.Errorf("unexpected offset: %s", offset)
		}
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	entities, err := client.GetEntities(context.Background(), "test-token", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 || entities[0].Name != "Sunrise" {
		t.Errorf("unexpected entities: %+v", entities)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestOWProvClient_GetEntities_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		offset := r.URL.Query().Get("offset")
		if offset == "0" {
			entities := make([]models.Entity, 500)
			for i := range entities {
				entities[i] = models.Entity{ID: "ent-" + string(rune('a'+i%26)), Name: "Prop"}
			}
			json.NewEncoder(w).Encode(models.EntityListResponse{Entities: entities})
		} else if offset == "500" {
			json.NewEncoder(w).Encode(models.EntityListResponse{
				Entities: []models.Entity{{ID: "ent-last", Name: "Last Prop"}},
			})
		} else {
			t.Errorf("unexpected offset: %s", offset)
		}
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	entities, err := client.GetEntities(context.Background(), "test-token", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 501 {
		t.Errorf("expected 501 entities, got %d", len(entities))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestOWProvClient_GetVenues(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Query().Get("limit") != "500" {
			t.Errorf("expected limit=500, got %s", r.URL.Query().Get("limit"))
		}
		offset := r.URL.Query().Get("offset")
		if offset == "0" {
			resp := models.VenueListResponse{
				Venues: []models.Venue{
					{ID: "ven-1", Name: "Tower A"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			t.Errorf("unexpected offset: %s", offset)
		}
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	venues, err := client.GetVenues(context.Background(), "test-token", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(venues) != 1 || venues[0].Name != "Tower A" {
		t.Errorf("unexpected venues: %+v", venues)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestOWProvClient_GetVenues_Pagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		offset := r.URL.Query().Get("offset")
		if offset == "0" {
			venues := make([]models.Venue, 500)
			for i := range venues {
				venues[i] = models.Venue{ID: "ven-" + string(rune('a'+i%26)), Name: "Venue"}
			}
			json.NewEncoder(w).Encode(models.VenueListResponse{Venues: venues})
		} else if offset == "500" {
			json.NewEncoder(w).Encode(models.VenueListResponse{
				Venues: []models.Venue{{ID: "ven-last", Name: "Last Venue"}},
			})
		} else {
			t.Errorf("unexpected offset: %s", offset)
		}
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	venues, err := client.GetVenues(context.Background(), "test-token", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(venues) != 501 {
		t.Errorf("expected 501 venues, got %d", len(venues))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestOWProvClient_DownstreamError_Sanitized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"internal_error":"FATAL: connection to db-internal-01.local:5432 failed"}`))
	}))
	defer ts.Close()

	client := owprov.NewClient(owprov.Config{URLResolver: func() string { return ts.URL }})
	_, err := client.GetPolicy(context.Background(), "pol-1", "token", "req-1", "corr-1")
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
	if strings.Contains(apiErr.ErrorDetails, "db-internal-01.local") {
		t.Errorf("sensitive body leaked in ErrorDetails: %s", apiErr.ErrorDetails)
	}
	if apiErr.ErrorDetails != "downstream OWPROV returned status 500" {
		t.Errorf("expected sanitized message, got %q", apiErr.ErrorDetails)
	}
}
