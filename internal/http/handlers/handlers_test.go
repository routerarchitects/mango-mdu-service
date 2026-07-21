package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	provclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	secclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

func TestHandlers(t *testing.T) {
	// Mock tree response
	mockTree := provclient.ProvTreeNode{
		UUID: "00000000-0000-0000-0000-000000000000",
		Name: "Root Entity",
		Type: "entity",
		Children: []provclient.ProvTreeNode{
			{
				UUID: "ent-1",
				Name: "Sub Entity",
				Type: "entity",
				Venues: []provclient.ProvTreeNode{
					{
						UUID: "ven-1",
						Name: "My Venue",
						Type: "venue",
					},
				},
			},
		},
	}

	// Mock server for owprov
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/entity" && r.URL.Query().Get("getTree") == "true" {
			json.NewEncoder(w).Encode(mockTree)
			return
		}

		if r.URL.Path == "/api/v1/entity" {
			list := []provclient.ProvEntity{
				{
					Info: provclient.ProvObjectInfo{
						ID:   "ent-1",
						Name: "Sub Entity",
					},
					Type: "normal",
				},
			}
			json.NewEncoder(w).Encode(provclient.ProvEntityList{Entities: list})
			return
		}

		if r.URL.Path == "/api/v1/venue" {
			list := []provclient.ProvVenue{
				{
					Info: provclient.ProvObjectInfo{
						ID:   "ven-1",
						Name: "My Venue",
					},
				},
			}
			json.NewEncoder(w).Encode(provclient.ProvVenueList{Venues: list})
			return
		}

		if r.URL.Path == "/api/v1/managementPolicy" {
			list := []provclient.ProvManagementPolicy{
				{
					Info: provclient.ProvObjectInfo{
						ID:   "pol-1",
						Name: "admin",
					},
					Entity: "ent-1",
				},
			}
			json.NewEncoder(w).Encode(provclient.ProvManagementPolicyList{Policies: list})
			return
		}

		if r.URL.Path == "/api/v1/managementRole" {
			list := []provclient.ProvManagementRole{
				{
					Info: provclient.ProvObjectInfo{
						ID:   "rol-1",
						Name: "admin",
					},
					ManagementPolicy: "pol-1",
					Entity:           "ent-1",
					Users:            []string{"00000000-0000-0000-0000-000000000123"},
				},
			}
			json.NewEncoder(w).Encode(provclient.ProvManagementRoleList{Roles: list})
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/entity/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/entity/")
			ent := provclient.ProvEntity{
				Info: provclient.ProvObjectInfo{
					ID:          id,
					Name:        "Entity " + id,
					Description: "Desc of " + id,
				},
				Type: "normal",
			}
			json.NewEncoder(w).Encode(ent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

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
	secClient.AuthEnabled = false

	// Initialize Services & Handlers
	hierarchyService := services.NewHierarchyService(provClient)
	hierarchyHandler := handlers.NewHierarchyHandler(hierarchyService)

	entityService := services.NewEntityService(provClient)
	entityHandler := handlers.NewEntityHandler(entityService)

	venueService := services.NewVenueService(provClient)
	venueHandler := handlers.NewVenueHandler(venueService)

	assignmentService := services.NewAssignmentService(provClient, secClient)
	assignmentHandler := handlers.NewAssignmentHandler(assignmentService)

	// Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
	})

	apiGroup := app.Group("/api/v1")
	hierarchyHandler.Register(apiGroup)
	entityHandler.Register(apiGroup)
	venueHandler.Register(apiGroup)
	assignmentHandler.Register(apiGroup)

	// Test GET /api/v1/hierarchy
	t.Run("GET Hierarchy Tree", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/hierarchy", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var treeResp models.HierarchyTreeResponse
		json.NewDecoder(resp.Body).Decode(&treeResp)

		if len(treeResp.Roots) != 1 || treeResp.Roots[0].ID != "ent-1" || treeResp.Roots[0].Name != "Sub Entity" {
			t.Errorf("unexpected root of filtered tree: %+v", treeResp)
		}

		if len(treeResp.Roots[0].Children) != 1 || treeResp.Roots[0].Children[0].ID != "ven-1" {
			t.Errorf("unexpected venues/children: %+v", treeResp.Roots[0].Children)
		}
	})

	t.Run("GET Hierarchy Tree Scoped", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/hierarchy?scopeEntityId=ven-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var treeResp models.HierarchyTreeResponse
		json.NewDecoder(resp.Body).Decode(&treeResp)

		if len(treeResp.Roots) != 1 || treeResp.Roots[0].ID != "ven-1" || treeResp.Roots[0].Name != "My Venue" {
			t.Errorf("expected root to be scoped to ven-1, got: %+v", treeResp)
		}
	})

	// Test GET /api/v1/entities
	t.Run("GET Entities", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var listResp models.EntityListResponse
		json.NewDecoder(resp.Body).Decode(&listResp)

		if len(listResp.Items) != 1 || listResp.Items[0].ID != "ent-1" {
			t.Errorf("expected ent-1, got: %+v", listResp)
		}
	})

	// Test GET /api/v1/entities/:entityId
	t.Run("GET Entity Detail", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities/ent-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var detailResp models.EntityDetail
		json.NewDecoder(resp.Body).Decode(&detailResp)

		if detailResp.ID != "ent-1" || detailResp.Name != "Entity ent-1" {
			t.Errorf("unexpected entity detail: %+v", detailResp)
		}
	})
}

func TestProvOperatorSerialization(t *testing.T) {
	op := provclient.ProvOperator{
		ID:             "op-123",
		Name:           "Test Operator",
		Description:    "Test Desc",
		RegistrationID: "reg-123",
		EntityID:       "ent-123",
	}

	bytes, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("failed to marshal ProvOperator: %v", err)
	}

	jsonStr := string(bytes)
	if strings.Contains(jsonStr, "entityId") {
		t.Errorf("expected serialized ProvOperator JSON to NOT contain 'entityId', but got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"id":"op-123"`) || !strings.Contains(jsonStr, `"name":"Test Operator"`) {
		t.Errorf("expected serialized ProvOperator JSON to contain 'id' and 'name', got: %s", jsonStr)
	}
}
