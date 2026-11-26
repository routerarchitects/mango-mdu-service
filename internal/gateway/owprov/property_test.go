package owprov_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owprov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

func TestGetInventoryForScopeResolvesVenueDeviceReferences(t *testing.T) {
	requests := make([]string, 0, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Path {
		case "/api/v1/venue/venue-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "venue-1", "devices": []string{"inventory-1", "inventory-2", "inventory-1"},
			})
		case "/api/v1/inventory":
			if got := r.URL.Query().Get("select"); got != "inventory-1,inventory-2" {
				t.Fatalf("unexpected inventory selection: %q", got)
			}
			_ = json.NewEncoder(w).Encode(models.InventoryTagListResponse{TagList: []models.InventoryTag{
				{ID: "inventory-1", SerialNumber: "serial-a"},
				{ID: "inventory-2", SerialNumber: "serial-b"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client := owprov.NewClient(testOWProvConfig(ts.URL))
	tags, err := client.GetInventoryForScope(context.Background(), "", "venue-1", "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetInventoryForScope returned error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 resolved inventory records, got %+v", tags)
	}
	if got := []string{tags[0].SerialNumber, tags[1].SerialNumber}; !reflect.DeepEqual(got, []string{"serial-a", "serial-b"}) {
		t.Fatalf("unexpected resolved inventory: %+v", tags)
	}
	if len(requests) != 2 {
		t.Fatalf("expected scope and inventory requests, got %v", requests)
	}
}

func TestGetInventoryForScopeWithNoDeviceReferencesSkipsInventoryLookup(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/api/v1/entity/property-1" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "property-1", "devices": []string{}})
	}))
	defer ts.Close()

	client := owprov.NewClient(testOWProvConfig(ts.URL))
	tags, err := client.GetInventoryForScope(context.Background(), "property-1", "", "token", "req", "corr")
	if err != nil {
		t.Fatalf("GetInventoryForScope returned error: %v", err)
	}
	if len(tags) != 0 || requestCount != 1 {
		t.Fatalf("expected an empty result after one scope request, tags=%+v requests=%d", tags, requestCount)
	}
}

func TestGetInventoryForScopeRejectsIncompleteSelectedInventory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/venue/venue-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"devices": []string{"inventory-1", "inventory-2"}})
		case "/api/v1/inventory":
			_ = json.NewEncoder(w).Encode(models.InventoryTagListResponse{TagList: []models.InventoryTag{
				{ID: "inventory-1", SerialNumber: "serial-a"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client := owprov.NewClient(testOWProvConfig(ts.URL))
	if _, err := client.GetInventoryForScope(context.Background(), "", "venue-1", "token", "req", "corr"); err == nil {
		t.Fatal("expected incomplete selected inventory to fail")
	}
}
