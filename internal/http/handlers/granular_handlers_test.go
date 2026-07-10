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
	"github.com/routerarchitects/mango-mdu-service/internal/http/routes"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

func TestGranularHandlers(t *testing.T) {
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

	// Mock server for downstream owprov REST API calls
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Header.Get("X-Request-Id") == "downstream-error" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"mock prov server error"}`))
			return
		}

		// 1. Entity and Venue lists
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
					Entity: "ent-1",
				},
			}
			json.NewEncoder(w).Encode(provclient.ProvVenueList{Venues: list})
			return
		}

		// 2. Roles list/creation
		if r.URL.Path == "/api/v1/managementRole" {
			authHeader := r.Header.Get("Authorization")
			if strings.Contains(authHeader, "multi-policy-user") {
				list := []provclient.ProvManagementRole{
					{
						Info: provclient.ProvObjectInfo{
							ID:   "rol-read-only",
							Name: "admin",
						},
						ManagementPolicy: "pol-read-only",
						Entity:           "ent-1",
						Users:            []string{"multi-policy-user"},
					},
					{
						Info: provclient.ProvObjectInfo{
							ID:   "rol-interactive",
							Name: "admin",
						},
						ManagementPolicy: "pol-interactive",
						Entity:           "ent-2",
						Users:            []string{"multi-policy-user"},
					},
				}
				json.NewEncoder(w).Encode(provclient.ProvManagementRoleList{Roles: list})
				return
			}

			list := []provclient.ProvManagementRole{
				{
					Info: provclient.ProvObjectInfo{
						ID:   "rol-1",
						Name: "admin",
					},
					ManagementPolicy: "pol-1",
					Entity:           "ent-1",
					Users:            []string{"user-123"},
				},
			}
			json.NewEncoder(w).Encode(provclient.ProvManagementRoleList{Roles: list})
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/managementRole/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/managementRole/")
			if id == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method == http.MethodGet {
				rol := provclient.ProvManagementRole{
					Info: provclient.ProvObjectInfo{
						ID:   id,
						Name: "admin",
					},
					ManagementPolicy: "pol-1",
					Entity:           "ent-1",
					Users:            []string{"user-123"},
				}
				json.NewEncoder(w).Encode(rol)
				return
			}
			// For updates/creations
			rol := provclient.ProvManagementRole{
				Info: provclient.ProvObjectInfo{
					ID:   id,
					Name: "admin",
				},
				ManagementPolicy: "pol-1",
				Entity:           "ent-1",
				Users:            []string{"user-123"},
			}
			json.NewEncoder(w).Encode(rol)
			return
		}

		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/api/v1/managementRole/") {
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			if id == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			rol := provclient.ProvManagementRole{
				Info: provclient.ProvObjectInfo{
					ID:   id,
					Name: "admin",
				},
				ManagementPolicy: "pol-1",
				Entity:           "ent-1",
				Users:            []string{"user-123"},
			}
			json.NewEncoder(w).Encode(rol)
			return
		}

		// 3. Policies list/creation
		if r.URL.Path == "/api/v1/managementPolicy" {
			list := []provclient.ProvManagementPolicy{
				{
					Info: provclient.ProvObjectInfo{
						ID:   "pol-1",
						Name: "adminPolicy-user-123",
					},
					Entity: "ent-1",
					Entries: []provclient.ProvManagementPolicyEntry{
						{
							Users:     []string{"user-123"},
							Resources: []string{"configuration", "inventory"},
							Access:    []string{"READ", "MODIFY"},
							Policy:    `{"type":"entity","entityId":"ent-1","includeVenues":true,"includeChildEntities":true}`,
						},
					},
				},
			}
			json.NewEncoder(w).Encode(provclient.ProvManagementPolicyList{Policies: list})
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/managementPolicy/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/managementPolicy/")
			if id == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
				return
			}
			// For Get, Post or Put
			var entries []provclient.ProvManagementPolicyEntry
			if id == "pol-read-only" {
				entries = []provclient.ProvManagementPolicyEntry{
					{
						Users:     []string{"multi-policy-user"},
						Resources: []string{"configuration"},
						Access:    []string{"READ"},
						Policy:    `{"type":"entity","entityId":"ent-1","includeVenues":true,"includeChildEntities":true}`,
					},
				}
			} else if id == "pol-interactive" {
				entries = []provclient.ProvManagementPolicyEntry{
					{
						Users:     []string{"multi-policy-user"},
						Resources: []string{"inventory"},
						Access:    []string{"READ", "MODIFY"},
						Policy:    `{"type":"entity","entityId":"ent-2","includeVenues":true,"includeChildEntities":true}`,
					},
				}
			} else {
				entries = []provclient.ProvManagementPolicyEntry{
					{
						Users:     []string{"user-123"},
						Resources: []string{"configuration", "inventory"},
						Access:    []string{"READ", "MODIFY"},
						Policy:    `{"type":"entity","entityId":"ent-1","includeVenues":true,"includeChildEntities":true}`,
					},
				}
			}

			pol := provclient.ProvManagementPolicy{
				Info: provclient.ProvObjectInfo{
					ID:   id,
					Name: "Policy " + id,
				},
				Entity:  "ent-1",
				Entries: entries,
			}
			json.NewEncoder(w).Encode(pol)
			return
		}

		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/api/v1/managementPolicy/") {
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			if id == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			pol := provclient.ProvManagementPolicy{
				Info: provclient.ProvObjectInfo{
					ID:   id,
					Name: "Policy " + id,
				},
				Entity: "ent-1",
			}
			json.NewEncoder(w).Encode(pol)
			return
		}

		// 4. Venues Detail/CRUD
		if strings.HasPrefix(r.URL.Path, "/api/v1/venue/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/venue/")
			if id == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
				return
			}
			var bodyVen provclient.ProvVenue
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				json.NewDecoder(r.Body).Decode(&bodyVen)
			}
			name := bodyVen.Info.Name
			if name == "" {
				name = "Venue " + id
			}
			desc := bodyVen.Info.Description
			if desc == "" {
				desc = "Desc of " + id
			}
			ven := provclient.ProvVenue{
				Info: provclient.ProvObjectInfo{
					ID:          id,
					Name:        name,
					Description: desc,
				},
				Entity: "ent-1",
			}
			json.NewEncoder(w).Encode(ven)
			return
		}

		// 5. Entities Detail/CRUD
		if strings.HasPrefix(r.URL.Path, "/api/v1/entity/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/entity/")
			if id == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
				return
			}
			var bodyEnt provclient.ProvEntity
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				json.NewDecoder(r.Body).Decode(&bodyEnt)
			}
			name := bodyEnt.Info.Name
			if name == "" {
				name = "Entity " + id
			}
			desc := bodyEnt.Info.Description
			if desc == "" {
				desc = "Desc of " + id
			}
			ent := provclient.ProvEntity{
				Info: provclient.ProvObjectInfo{
					ID:          id,
					Name:        name,
					Description: desc,
				},
				Type: "normal",
			}
			json.NewEncoder(w).Encode(ent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Mock server for downstream owsec REST API calls
	mockSecServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/v1/validateToken") {
			token := r.URL.Query().Get("token")
			if token == "invalid-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if token == "downstream-error-token" {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"mock sec server error"}`))
				return
			}
			userID := "user-123"
			if token == "multi-policy-user" {
				userID = "multi-policy-user"
			}
			userInfo := secclient.UserInfo{
				ID:       userID,
				Email:    userID + "@example.com",
				Name:     "Test User",
				Owner:    "owner-123",
				UserRole: "admin",
			}
			json.NewEncoder(w).Encode(secclient.TokenValidationResponse{UserInfo: userInfo})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockSecServer.Close()

	provClient, err := provclient.NewClient(nil, "", "mdu-test")
	if err != nil {
		t.Fatalf("failed to create prov client: %v", err)
	}
	provClient.BaseURL = mockServer.URL

	secClient, err := secclient.NewClient(nil, "", "mdu-test")
	if err != nil {
		t.Fatalf("failed to create sec client: %v", err)
	}
	secClient.BaseURL = mockSecServer.URL

	// Initialize Services & Handlers
	venueService := services.NewVenueService(provClient)
	venueHandler := handlers.NewVenueHandler(venueService)

	assignmentService := services.NewAssignmentService(provClient)
	assignmentHandler := handlers.NewAssignmentHandler(assignmentService)

	entityService := services.NewEntityService(provClient)
	entityHandler := handlers.NewEntityHandler(entityService)

	sessionService := services.NewSessionService(secClient, provClient)
	sessionHandler := handlers.NewSessionHandler(sessionService, true)

	hierarchyService := services.NewHierarchyService(provClient)
	hierarchyHandler := handlers.NewHierarchyHandler(hierarchyService)

	// Fiber app setup
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler,
	})

	app.Use(func(c fiber.Ctx) error {
		if c.Get("X-Test-No-Auth-Inject") == "true" || c.Get("X-Test-Bypass-Auth") == "true" {
			return c.Next()
		}
		if c.Get("Authorization") == "" {
			c.Request().Header.Set("Authorization", "Bearer valid-token")
		}
		return c.Next()
	})

	tokenValidator := secclient.NewClientAdapter(secClient)
	authMiddleware, err := middleware.NewServiceAuth(
		true, // authEnabled
		auth.PublicAuthConfig{},
		auth.InternalAPIKeyConfig{
			ExpectedAPIKey: "test-secret",
		},
		tokenValidator,
	)
	if err != nil {
		t.Fatalf("failed to create auth middleware: %v", err)
	}
	publicAuthHandler := authMiddleware.GetPublicAuthHandler()

	testPublicAuthHandler := func(c fiber.Ctx) error {
		if c.Get("X-Test-Bypass-Auth") == "true" {
			return c.Next()
		}
		return publicAuthHandler(c)
	}

	routes.RegisterPublic(app, routes.PublicDeps{
		AuthHandler:       testPublicAuthHandler,
		Subsystem:         subsysteroutes.Config{},
		SessionHandler:    sessionHandler,
		HierarchyHandler:  hierarchyHandler,
		EntityHandler:     entityHandler,
		VenueHandler:      venueHandler,
		AssignmentHandler: assignmentHandler,
	})

	// ==========================================
	// TEST CASES
	// ==========================================

	// 1. GET /api/v1/session - Positive Case
	t.Run("GET Session - Positive", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var sessResp models.SessionContext
		json.NewDecoder(resp.Body).Decode(&sessResp)

		if sessResp.User.Name != "Test User" || sessResp.User.Role != "admin" {
			t.Errorf("unexpected session context user: %+v", sessResp.User)
		}
	})

	t.Run("GET Session - Positive (Multiple Policies)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("Authorization", "Bearer multi-policy-user")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var sessResp models.SessionContext
		json.NewDecoder(resp.Body).Decode(&sessResp)

		if sessResp.Permissions.Configurations.Mode != "read_only" {
			t.Errorf("expected configurations mode to be read_only, got %q", sessResp.Permissions.Configurations.Mode)
		}
		if sessResp.Permissions.Devices.Mode != "interactive" {
			t.Errorf("expected devices mode to be interactive, got %q", sessResp.Permissions.Devices.Mode)
		}
		if sessResp.Permissions.Hierarchy.Mode != "hidden" {
			t.Errorf("expected hierarchy mode to be hidden, got %q", sessResp.Permissions.Hierarchy.Mode)
		}
	})

	// 2. GET /api/v1/session - Negative Case
	t.Run("GET Session - Negative (Missing Header)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("X-Test-Bypass-Auth", "true")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("GET Session - Auth Disabled (AUTH_ENABLED=false)", func(t *testing.T) {
		sessionHandler.AuthEnabled = false
		secClient.AuthEnabled = false
		defer func() {
			sessionHandler.AuthEnabled = true
			secClient.AuthEnabled = true
		}()

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("X-Test-Bypass-Auth", "true")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var sessResp models.SessionContext
		json.NewDecoder(resp.Body).Decode(&sessResp)

		if sessResp.User.ID != "00000000-0000-0000-0000-000000000000" || sessResp.User.Role != "admin" {
			t.Errorf("unexpected session context user: %+v", sessResp.User)
		}
	})

	// 2b. Verify Public Route Protection (Missing Authorization on another route)
	t.Run("Verify Public Route Protection (Missing Authorization)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities", nil)
		req.Header.Set("X-Test-No-Auth-Inject", "true")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized for unprotected access to /api/v1/entities, got %d", resp.StatusCode)
		}
	})

	// 3. GET /api/v1/session - Negative Case (Invalid Token)
	t.Run("GET Session - Negative (Invalid Token)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("X-Test-Bypass-Auth", "true")
		req.Header.Set("Authorization", "Bearer invalid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	// 4. GET /api/v1/entities - Positive Case
	t.Run("GET Entities list", func(t *testing.T) {
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
			t.Errorf("unexpected entity items list: %+v", listResp)
		}
	})

	// 5. POST /api/v1/entities - Positive Case
	t.Run("POST Create Entity", func(t *testing.T) {
		payload := `{"name":"New Root Entity", "description":"Root Level Entity"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/entities", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d", resp.StatusCode)
		}

		var detailResp models.EntityDetail
		json.NewDecoder(resp.Body).Decode(&detailResp)

		if detailResp.Name != "New Root Entity" {
			t.Errorf("expected entity name New Root Entity, got: %s", detailResp.Name)
		}
	})

	// 6. POST /api/v1/entities - Negative Case (Missing Name)
	t.Run("POST Create Entity - Missing Name", func(t *testing.T) {
		payload := `{"description":"Entity without name"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/entities", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	// 7. GET /api/v1/entities/:entityId - Positive Case
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

		if detailResp.ID != "ent-1" {
			t.Errorf("expected entity ID ent-1, got: %s", detailResp.ID)
		}
	})

	// 8. GET /api/v1/entities/:entityId - Negative Case
	t.Run("GET Entity Detail - Non-Existent", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities/non-existent", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// 9. PUT /api/v1/entities/:entityId - Positive Case
	t.Run("PUT Update Entity", func(t *testing.T) {
		payload := `{"name":"Updated Entity Title"}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/entities/ent-1", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
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

		if detailResp.Name != "Updated Entity Title" {
			t.Errorf("expected entity name Updated Entity Title, got: %s", detailResp.Name)
		}
	})

	// 10. PUT /api/v1/entities/:entityId - Negative Case
	t.Run("PUT Update Entity - Non-Existent", func(t *testing.T) {
		payload := `{"name":"Fail Me"}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/entities/non-existent", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// 11. DELETE /api/v1/entities/:entityId - Positive Case
	t.Run("DELETE Entity", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/entities/ent-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 or 204, got %d", resp.StatusCode)
		}
	})

	// 12. GET /api/v1/venues - Positive Case
	t.Run("GET Venues list", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/venues", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var listResp models.VenueListResponse
		json.NewDecoder(resp.Body).Decode(&listResp)

		if len(listResp.Items) != 1 || listResp.Items[0].ID != "ven-1" {
			t.Errorf("unexpected venues items list: %+v", listResp)
		}
	})

	// 13. POST /api/v1/venues - Positive Case
	t.Run("POST Create Venue", func(t *testing.T) {
		payload := `{"name":"New Root Venue", "description":"Root Level Venue"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/venues", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d", resp.StatusCode)
		}

		var detailResp models.VenueDetail
		json.NewDecoder(resp.Body).Decode(&detailResp)

		if detailResp.Name != "New Root Venue" {
			t.Errorf("expected venue name New Root Venue, got: %s", detailResp.Name)
		}
	})

	// 14. POST /api/v1/venues - Negative Case (Missing Name)
	t.Run("POST Create Venue - Missing Name", func(t *testing.T) {
		payload := `{"description":"Venue without name"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/venues", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	// 15. GET /api/v1/venues/:venueId - Positive Case
	t.Run("GET Venue Detail", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/venues/ven-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var detailResp models.VenueDetail
		json.NewDecoder(resp.Body).Decode(&detailResp)

		if detailResp.ID != "ven-1" {
			t.Errorf("expected venue ID ven-1, got: %s", detailResp.ID)
		}
	})

	// 16. GET /api/v1/venues/:venueId - Negative Case
	t.Run("GET Venue Detail - Non-Existent", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/venues/non-existent", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// 17. PUT /api/v1/venues/:venueId - Positive Case
	t.Run("PUT Update Venue", func(t *testing.T) {
		payload := `{"name":"Updated Venue Name", "description":"Updated Description"}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/venues/ven-1", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var detailResp models.VenueDetail
		json.NewDecoder(resp.Body).Decode(&detailResp)

		if detailResp.Name != "Updated Venue Name" {
			t.Errorf("expected venue name Updated Venue Name, got: %s", detailResp.Name)
		}
	})

	// 18. PUT /api/v1/venues/:venueId - Negative Case
	t.Run("PUT Update Venue - Non-Existent", func(t *testing.T) {
		payload := `{"name":"Updated Venue Name"}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/venues/non-existent", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// 19. DELETE /api/v1/venues/:venueId - Positive Case
	t.Run("DELETE Venue", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/venues/ven-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 or 204, got %d", resp.StatusCode)
		}
	})

	// 20. GET /api/v1/entities/:entityId/venues - Positive Case
	t.Run("GET Entity Venues list", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities/ent-1/venues", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var listResp models.VenueListResponse
		json.NewDecoder(resp.Body).Decode(&listResp)

		if len(listResp.Items) != 1 || listResp.Items[0].ID != "ven-1" {
			t.Errorf("unexpected venue items list: %+v", listResp)
		}
	})

	// 21. POST /api/v1/entities/:entityId/venues - Positive Case
	t.Run("POST Create Entity Venue", func(t *testing.T) {
		payload := `{"name":"New Venue Under Entity", "description":"Entity Child Venue"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/entities/ent-1/venues", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d", resp.StatusCode)
		}

		var detailResp models.VenueDetail
		json.NewDecoder(resp.Body).Decode(&detailResp)

		if detailResp.Name != "New Venue Under Entity" {
			t.Errorf("expected venue name New Venue Under Entity, got: %s", detailResp.Name)
		}
	})

	// 22. POST /api/v1/entities/:entityId/venues - Negative Case (Missing Name)
	t.Run("POST Create Entity Venue - Missing Name", func(t *testing.T) {
		payload := `{"description":"Entity Child Venue"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/entities/ent-1/venues", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	// 23. GET /api/v1/users/:userId/assignments - Positive Case
	t.Run("GET User Scoped Assignments", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/user-123/assignments", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var assignmentsResp models.UserAssignmentsResponse
		json.NewDecoder(resp.Body).Decode(&assignmentsResp)

		if len(assignmentsResp.Items) != 1 || assignmentsResp.Items[0].ScopeID != "ent-1" {
			t.Errorf("expected assignment to ent-1, got: %+v", assignmentsResp)
		}
	})

	// 24. POST /api/v1/users/:userId/assignments - Positive Case
	t.Run("POST Create User Scoped Assignment", func(t *testing.T) {
		payload := `{"scopeType":"entity", "scopeId":"ent-1", "role":"admin"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/user-123/assignments", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d", resp.StatusCode)
		}

		var assignment models.UserAssignment
		json.NewDecoder(resp.Body).Decode(&assignment)

		if assignment.ScopeID != "ent-1" || assignment.Role != "admin" {
			t.Errorf("unexpected assignment response: %+v", assignment)
		}
	})

	// 25. POST /api/v1/users/:userId/assignments - Negative Case (Invalid payload / missing fields)
	t.Run("POST Create Assignment - Missing Fields", func(t *testing.T) {
		payload := `{"scopeType":"entity"}` // Missing role and scopeId
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/user-123/assignments", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	// 26. DELETE /api/v1/users/:userId/assignments/:assignmentId - Positive Case
	t.Run("DELETE User Scoped Assignment", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/user-123/assignments/rol-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 or 204, got %d", resp.StatusCode)
		}
	})

	// 27. DELETE /api/v1/users/:userId/assignments/:assignmentId - Negative Case (404 Not Found)
	t.Run("DELETE Assignment - Non-Existent", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/user-123/assignments/non-existent", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// 28. GET /api/v1/users/:userId/access-policy - Positive Case
	t.Run("GET User Scoped Access Policy", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/user-123/access-policy?scope=entity&entityId=ent-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var policyResp models.UserAccessPolicy
		json.NewDecoder(resp.Body).Decode(&policyResp)

		if policyResp.Scope != "entity" || policyResp.RoleTemplate != "admin" {
			t.Errorf("unexpected access policy details: %+v", policyResp)
		}
		if len(policyResp.ResourcePermissions) != 2 {
			t.Errorf("expected 2 resource permissions, got: %d", len(policyResp.ResourcePermissions))
		}
	})

	// 29. GET /api/v1/users/:userId/access-policy - Negative Case (User/Assignment Not Found)
	t.Run("GET Access Policy - Assignment Not Found", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/unknown-user/access-policy?scope=entity&entityId=ent-1", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// 30. PUT /api/v1/users/:userId/access-policy - Positive Case
	t.Run("PUT Update User Scoped Access Policy", func(t *testing.T) {
		payload := `{
			"scope": "entity",
			"entityId": "ent-1",
			"roleTemplate": "admin",
			"resourcePermissions": [
				{
					"resource": "configuration",
					"policies": ["READ", "MODIFY"]
				}
			]
		}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/user-123/access-policy", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var policyResp models.UserAccessPolicy
		json.NewDecoder(resp.Body).Decode(&policyResp)

		if len(policyResp.ResourcePermissions) != 1 || policyResp.ResourcePermissions[0].Resource != "configuration" {
			t.Errorf("unexpected updated access policy: %+v", policyResp)
		}
	})

	// 31. PUT /api/v1/users/:userId/access-policy - Negative Case (User/Assignment Not Found)
	t.Run("PUT Access Policy - Assignment Not Found", func(t *testing.T) {
		payload := `{
			"scope": "entity",
			"entityId": "ent-1",
			"roleTemplate": "admin",
			"resourcePermissions": [
				{
					"resource": "configuration",
					"policies": ["READ"]
				}
			]
		}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/unknown-user/access-policy", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// 32. GET /api/v1/hierarchy - Scoped and Unscoped Tree
	t.Run("GET Hierarchy Tree - Granular", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/hierarchy", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var treeResp models.HierarchyTreeResponse
		json.NewDecoder(resp.Body).Decode(&treeResp)
		if len(treeResp.Roots) == 0 {
			t.Errorf("expected hierarchy tree roots, got none")
		}
	})

	// 33. GET /api/v1/system - System Diagnostics (command=info)
	t.Run("GET System Diagnostics - Info", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/system?command=info", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	// 34. POST /api/v1/system - Subsystem Log Levels
	t.Run("POST System Diagnostics - Log Level Names", func(t *testing.T) {
		payload := `{"command":"getloglevelnames"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/system", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	// 35. GET /api/v1/entities - Pagination Defaults and Fallback
	t.Run("GET Entities list - Pagination Assertions", func(t *testing.T) {
		// Omitted limit/offset query parameters
		reqDefault, _ := http.NewRequest(http.MethodGet, "/api/v1/entities", nil)
		respDefault, err := app.Test(reqDefault)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer respDefault.Body.Close()

		if respDefault.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", respDefault.StatusCode)
		}

		var listDefault models.EntityListResponse
		json.NewDecoder(respDefault.Body).Decode(&listDefault)
		if listDefault.Metadata.Limit != 20 {
			t.Errorf("expected default limit to be 20, got %d", listDefault.Metadata.Limit)
		}
		if listDefault.Metadata.Offset != 0 {
			t.Errorf("expected default offset to be 0, got %d", listDefault.Metadata.Offset)
		}

		// Invalid pagination values (non-integers)
		reqInvalid, _ := http.NewRequest(http.MethodGet, "/api/v1/entities?limit=invalid&offset=invalid", nil)
		respInvalid, err := app.Test(reqInvalid)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer respInvalid.Body.Close()

		if respInvalid.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 for invalid query parameters, got %d", respInvalid.StatusCode)
		}
	})

	// 36. GET /api/v1/venues - Pagination Defaults and Fallback
	t.Run("GET Venues list - Pagination Assertions", func(t *testing.T) {
		// Omitted limit/offset query parameters
		reqDefault, _ := http.NewRequest(http.MethodGet, "/api/v1/venues", nil)
		respDefault, err := app.Test(reqDefault)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer respDefault.Body.Close()

		if respDefault.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", respDefault.StatusCode)
		}

		var listDefault models.VenueListResponse
		json.NewDecoder(respDefault.Body).Decode(&listDefault)
		if listDefault.Metadata.Limit != 20 {
			t.Errorf("expected default limit to be 20, got %d", listDefault.Metadata.Limit)
		}
		if listDefault.Metadata.Offset != 0 {
			t.Errorf("expected default offset to be 0, got %d", listDefault.Metadata.Offset)
		}

		// Invalid pagination values (non-integers)
		reqInvalid, _ := http.NewRequest(http.MethodGet, "/api/v1/venues?limit=invalid&offset=invalid", nil)
		respInvalid, err := app.Test(reqInvalid)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer respInvalid.Body.Close()

		if respInvalid.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 for invalid query parameters, got %d", respInvalid.StatusCode)
		}
	})

	// 37. GET /api/v1/users/:userId/access-policy - Validation error when entityId is missing for venue scope
	t.Run("GET Access Policy - Validation error when entityId is missing for venue scope", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/user-123/access-policy?scope=venue&venueId=ven-1", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 38. GET /api/v1/users/:userId/access-policy - Downstream connection failure mapping to 503
	t.Run("GET Access Policy - Downstream PROV error maps to 503", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/user-123/access-policy?scope=entity&entityId=ent-1", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("X-Request-Id", "downstream-error")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected status 503 Service Unavailable, got %d", resp.StatusCode)
		}
	})

	// 39. GET /api/v1/session - Downstream SEC error maps to 503
	t.Run("GET Session - Downstream SEC error maps to 503", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("X-Test-Bypass-Auth", "true")
		req.Header.Set("Authorization", "Bearer downstream-error-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected status 503 Service Unavailable, got %d", resp.StatusCode)
		}
	})

	// 40. POST /api/v1/users/:userId/assignments - Idempotent 200 OK when user is already assigned
	t.Run("POST Create Assignment - Idempotent 200 OK when user is already assigned", func(t *testing.T) {
		payload := `{"scopeType":"entity","scopeId":"ent-1","role":"admin"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/user-123/assignments", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
		}
	})

	// 41. POST /api/v1/users/:userId/assignments - New assignment created and 201 Created when user is not assigned
	t.Run("POST Create Assignment - New assignment created and 201 Created when user is not assigned", func(t *testing.T) {
		payload := `{"scopeType":"entity","scopeId":"ent-1","role":"admin"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/user-456/assignments", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201 Created, got %d", resp.StatusCode)
		}
	})

	// 42. POST /api/v1/users/:userId/assignments - Invalid scopeType returns 400 Bad Request
	t.Run("POST Create Assignment - Invalid scopeType returns 400 Bad Request", func(t *testing.T) {
		payload := `{"scopeType":"invalid-scope","scopeId":"ent-1","role":"admin"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/user-123/assignments", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 43. GET /api/v1/users/:userId/access-policy - Invalid combination (scope=entity with venueId present) returns 400
	t.Run("GET Access Policy - Invalid combination (scope=entity with venueId) returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/user-123/access-policy?scope=entity&entityId=ent-1&venueId=ven-1", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 44. PUT /api/v1/users/:userId/access-policy - Invalid combination (scope=entity with venueId present) returns 400
	t.Run("PUT Access Policy - Invalid combination (scope=entity with venueId) returns 400", func(t *testing.T) {
		payload := `{"scope":"entity","entityId":"ent-1","venueId":"ven-1","role":"admin","entries":[]}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/user-123/access-policy", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 45. GET /api/v1/users/:userId/access-policy - Venue scope missing venueId returns 400
	t.Run("GET Access Policy - Venue scope missing venueId returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/user-123/access-policy?scope=venue&entityId=ent-1", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 46. PUT /api/v1/users/:userId/access-policy - Venue scope missing venueId returns 400
	t.Run("PUT Access Policy - Venue scope missing venueId returns 400", func(t *testing.T) {
		payload := `{"scope":"venue","entityId":"ent-1","role":"admin","entries":[]}`
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/user-123/access-policy", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 47. GET /api/v1/session - Downstream PROV error maps to 503
	t.Run("GET Session - Downstream PROV error maps to 503", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/session", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("X-Request-Id", "downstream-error")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected status 503 Service Unavailable, got %d", resp.StatusCode)
		}
	})

	// 48. GET /api/v1/entities - Invalid pagination returns 400
	t.Run("GET Entities - Invalid pagination limit returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities?limit=-5", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	t.Run("GET Entities - Invalid pagination offset returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities?offset=abc", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 49. GET /api/v1/venues - Invalid pagination returns 400
	t.Run("GET Venues - Invalid pagination limit returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/venues?limit=-100", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	// 50. GET /api/v1/entities/:entityId/venues - Invalid pagination returns 400
	t.Run("GET Entity Venues - Invalid pagination limit returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/entities/ent-1/venues?limit=xyz", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", resp.StatusCode)
		}
	})
}
