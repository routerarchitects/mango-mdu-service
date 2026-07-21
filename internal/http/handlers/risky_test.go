package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	provclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	secclient "github.com/routerarchitects/mango-mdu-service/internal/gateway/sec"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

func TestRiskyBehaviors(t *testing.T) {
	// Setup in-memory mock storage for dynamic PROV test cases
	var mu sync.Mutex
	var activeRoles = map[string]*provclient.ProvManagementRole{
		"role-b-id": {
			Info: provclient.ProvObjectInfo{
				ID:   "role-b-id",
				Name: "admin",
			},
			ManagementPolicy: "policy-b-id",
			Entity:           "ent-1",
			Users:            []string{"00000000-0000-0000-0000-00000000000b"}, // belongs to User B
		},
		"role-a-id": {
			Info: provclient.ProvObjectInfo{
				ID:   "role-a-id",
				Name: "admin",
			},
			ManagementPolicy: "policy-a-id",
			Entity:           "ent-1",
			Users:            []string{"00000000-0000-0000-0000-00000000000a"}, // belongs to User A
		},
	}

	mockProvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()

		testCase := r.Header.Get("X-Correlation-Id")

		// 1. policy created, role creation fails
		if testCase == "role-creation-fails" {
			if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/managementPolicy") {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(provclient.ProvManagementPolicy{
					Info: provclient.ProvObjectInfo{ID: "policy-new-id", Name: "adminPolicy"},
				})
				return
			}
			if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/managementRole") {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"failed to create management role"}`))
				return
			}
		}

		// 2. role deletion succeeds, policy deletion fails
		if testCase == "policy-deletion-fails" {
			if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/managementRole/") {
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/managementPolicy/") {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"failed to delete management policy"}`))
				return
			}
		}

		// List roles
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/managementRole" {
			var list []provclient.ProvManagementRole
			for _, rl := range activeRoles {
				list = append(list, *rl)
			}
			json.NewEncoder(w).Encode(provclient.ProvManagementRoleList{Roles: list})
			return
		}

		// List policies
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/managementPolicy" {
			list := []provclient.ProvManagementPolicy{
				{Info: provclient.ProvObjectInfo{ID: "pol-admin", Name: "admin"}},
				{Info: provclient.ProvObjectInfo{ID: "pol-noc", Name: "noc"}},
				{Info: provclient.ProvObjectInfo{ID: "pol-csr", Name: "csr"}},
				{Info: provclient.ProvObjectInfo{ID: "pol-installer", Name: "installer"}},
			}
			json.NewEncoder(w).Encode(provclient.ProvManagementPolicyList{Policies: list})
			return
		}

		// Get role details
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/managementRole/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/managementRole/")
			if rl, ok := activeRoles[id]; ok {
				json.NewEncoder(w).Encode(rl)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"role not found"}`))
			return
		}

		// Delete role
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/managementRole/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/managementRole/")
			if _, ok := activeRoles[id]; ok {
				delete(activeRoles, id)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Get policy
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/managementPolicy/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/managementPolicy/")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(provclient.ProvManagementPolicy{
				Info: provclient.ProvObjectInfo{ID: id, Name: "adminPolicy"},
			})
			return
		}

		// Delete policy
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/managementPolicy/") {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Create Policy / Role standard handlers
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/managementPolicy") {
			var p provclient.ProvManagementPolicy
			json.NewDecoder(r.Body).Decode(&p)
			p.Info.ID = "policy-gen-id"
			json.NewEncoder(w).Encode(p)
			return
		}

		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/managementRole") {
			var rl provclient.ProvManagementRole
			json.NewDecoder(r.Body).Decode(&rl)
			rl.Info.ID = "role-gen-id"
			activeRoles[rl.Info.ID] = &rl
			json.NewEncoder(w).Encode(rl)
			return
		}

		// Entity Tree and other dependencies
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/entity" {
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

		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/venue" {
			json.NewEncoder(w).Encode(provclient.ProvVenueList{Venues: []provclient.ProvVenue{}})
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer mockProvServer.Close()

	// Mock server for downstream OWSEC HTTP status code assertions
	mockSecServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/validateToken" {
			token := r.URL.Query().Get("token")
			if token == "invalid-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			userID := "00000000-0000-0000-0000-000000000123"
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
		if strings.HasPrefix(r.URL.Path, "/api/v1/user/") {
			userID := strings.TrimPrefix(r.URL.Path, "/api/v1/user/")

			switch userID {
			case "00000000-0000-0000-0000-000000000401":
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized access"}`))
			case "00000000-0000-0000-0000-000000000403":
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"forbidden access"}`))
			case "00000000-0000-0000-0000-000000000404":
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":"user not found"}`))
			case "00000000-0000-0000-0000-000000000429":
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded"}`))
			case "00000000-0000-0000-0000-000000000500":
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal database failure"}`))
			case "00000000-0000-0000-0000-00000000000f": // malformed JSON
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{invalid-json`))
			default:
				userInfo := secclient.UserInfo{
					ID:       userID,
					Email:    userID + "@example.com",
					Name:     "Test User",
					Owner:    "owner-123",
					UserRole: "admin",
				}
				json.NewEncoder(w).Encode(userInfo)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockSecServer.Close()

	provClient, _ := provclient.NewClient(nil, "", "mdu-test")
	provClient.BaseURL = mockProvServer.URL

	secClient, _ := secclient.NewClient(nil, "", "mdu-test")
	secClient.BaseURL = mockSecServer.URL

	// 1. policy created, role creation fails
	t.Run("Policy created but role creation fails", func(t *testing.T) {
		svc := services.NewAssignmentService(provClient, secClient)
		reqCtx := provclient.RequestContext{
			Context:       context.Background(),
			CorrelationID: "role-creation-fails",
		}
		req := &models.CreateUserAssignmentRequest{
			ScopeType: "entity",
			ScopeID:   "ent-1",
			Role:      "admin",
		}

		// User C has no existing assignments
		_, _, err := svc.CreateAssignment(reqCtx, "00000000-0000-0000-0000-00000000000c", req)
		if err == nil {
			t.Fatal("expected CreateAssignment to fail when role creation fails")
		}
		if !strings.Contains(err.Error(), "failed to create management role") {
			t.Errorf("expected role creation failure message, got: %v", err)
		}
	})

	// 3. repeated assignment deletion
	t.Run("Repeated assignment deletion returns 404", func(t *testing.T) {
		svc := services.NewAssignmentService(provClient, secClient)
		reqCtx := provclient.RequestContext{
			Context: context.Background(),
		}

		// First deletion of role-a-id should succeed
		err := svc.DeleteAssignment(reqCtx, "00000000-0000-0000-0000-00000000000a", "role-a-id")
		if err != nil {
			t.Fatalf("first deletion failed: %v", err)
		}

		// Second deletion should fail with 404 because role-a-id was deleted from activeRoles map
		err = svc.DeleteAssignment(reqCtx, "00000000-0000-0000-0000-00000000000a", "role-a-id")
		if err == nil {
			t.Fatal("expected second deletion to fail")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected user assignment not found, got: %v", err)
		}
	})

	// 4. simultaneous duplicate assignment creation
	t.Run("Simultaneous duplicate assignment creation is idempotent", func(t *testing.T) {
		svc := services.NewAssignmentService(provClient, secClient)
		reqCtx := provclient.RequestContext{
			Context: context.Background(),
		}
		req := &models.CreateUserAssignmentRequest{
			ScopeType: "entity",
			ScopeID:   "ent-dup",
			Role:      "admin",
		}

		// Try creating concurrently
		var wg sync.WaitGroup
		var res1, res2 *models.UserAssignment
		var isAlready1, isAlready2 bool
		var err1, err2 error

		wg.Add(2)
		go func() {
			defer wg.Done()
			res1, isAlready1, err1 = svc.CreateAssignment(reqCtx, "00000000-0000-0000-0000-000000000999", req)
		}()
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			res2, isAlready2, err2 = svc.CreateAssignment(reqCtx, "00000000-0000-0000-0000-000000000999", req)
		}()
		wg.Wait()

		if err1 != nil {
			t.Fatalf("creation 1 failed: %v", err1)
		}
		if err2 != nil {
			t.Fatalf("creation 2 failed: %v", err2)
		}

		// One should have isAlreadyAssigned=false and the other should have isAlreadyAssigned=true
		if isAlready1 && isAlready2 {
			t.Error("expected at least one assignment creation to not be already assigned")
		}
		if !isAlready1 && !isAlready2 {
			t.Error("expected one creation to be flagged as already assigned (idempotent)")
		}

		if res1.AssignmentID != res2.AssignmentID {
			t.Errorf("expected same assignment ID, got %s and %s", res1.AssignmentID, res2.AssignmentID)
		}
	})

	// 5. authentication enabled with RPC disabled
	t.Run("Auth enabled with RPC disabled fails middleware setup", func(t *testing.T) {
		_, err := middleware.NewServiceAuth(
			true,
			auth.PublicAuthConfig{},
			auth.InternalAPIKeyConfig{},
			nil, // validator is nil (RPC disabled)
		)
		if err == nil {
			t.Error("expected error when auth is enabled but public token validator is nil")
		}
	})

	// 6. authentication enabled with discovery disabled
	t.Run("Auth enabled with discovery disabled fails middleware setup", func(t *testing.T) {
		_, err := middleware.NewServiceAuth(
			true,
			auth.PublicAuthConfig{},
			auth.InternalAPIKeyConfig{},
			nil, // discovery disabled results in nil validator
		)
		if err == nil {
			t.Error("expected error when auth is enabled but validator is nil due to disabled discovery")
		}
	})

	// 7. OWSEC 401, 403, 404, 429, 500 and malformed JSON
	t.Run("OWSEC error code mappings", func(t *testing.T) {
		tests := []struct {
			userID       string
			expectedCode string
		}{
			{"00000000-0000-0000-0000-000000000401", "UNAUTHORIZED"},
			{"00000000-0000-0000-0000-000000000403", "UNAUTHORIZED"},
			{"00000000-0000-0000-0000-000000000404", "NOT_FOUND"},
			{"00000000-0000-0000-0000-000000000429", "DOWNSTREAM_UNAVAILABLE"},
			{"00000000-0000-0000-0000-000000000500", "DOWNSTREAM_UNAVAILABLE"},
			{"00000000-0000-0000-0000-00000000000f", "INTERNAL"},
		}

		for _, tc := range tests {
			_, err := secClient.GetUser(context.Background(), tc.userID, "token")
			if err == nil {
				t.Fatalf("expected error for user %s", tc.userID)
			}
			code := apperror.CodeOf(err)
			if string(code) != tc.expectedCode {
				t.Errorf("for user %s expected code %s, got %s", tc.userID, tc.expectedCode, code)
			}
		}
	})

	// 8. sanitization of downstream error bodies
	t.Run("Sanitization of downstream error bodies in public responses", func(t *testing.T) {
		app := fiber.New(fiber.Config{
			ErrorHandler: middleware.ErrorHandler,
		})
		app.Get("/test-downstream-error", func(c fiber.Ctx) error {
			// Simulate a raw downstream error returning HTML/details
			return apperror.New(apperror.Code("DOWNSTREAM_UNAVAILABLE"), "owsec returned status 502: <html>Raw Internal Stack Trace</html>")
		})

		req := httptest.NewRequest(http.MethodGet, "/test-downstream-error", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		// Public response must be sanitized
		if strings.Contains(bodyStr, "Raw Internal Stack Trace") {
			t.Errorf("leak detected! response body contains raw downstream details: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "Authentication service is temporarily unavailable") &&
			!strings.Contains(bodyStr, "Provisioning service is temporarily unavailable") {
			t.Errorf("expected sanitized error message, got: %s", bodyStr)
		}
	})

	// 9. missing request/correlation IDs
	t.Run("Missing request ID is automatically generated", func(t *testing.T) {
		app := fiber.New()
		app.Use(middleware.CorrelationAndRequestID())
		app.Get("/test-ids", func(c fiber.Ctx) error {
			return c.SendStatus(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test-ids", nil)
		// Do not set any request or correlation ID headers
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Request ID must be generated and set in response headers
		reqID := resp.Header.Get("X-Request-ID")
		if len(reqID) != 24 {
			t.Errorf("expected 24 character hex request ID to be generated, got %q", reqID)
		}
	})

	// 11. assignment ID belongs to the requested user before deletion
	t.Run("Assignment ID belongs to requested user before deletion", func(t *testing.T) {
		svc := services.NewAssignmentService(provClient, secClient)
		reqCtx := provclient.RequestContext{
			Context: context.Background(),
		}

		// User A (00000000-0000-0000-0000-00000000000a) attempts to delete User B's assignment (role-b-id)
		err := svc.DeleteAssignment(reqCtx, "00000000-0000-0000-0000-00000000000a", "role-b-id")
		if err == nil {
			t.Fatal("expected DeleteAssignment to fail when role does not contain requested user")
		}
		if !strings.Contains(err.Error(), "user assignment not found") {
			t.Errorf("expected user assignment not found, got: %v", err)
		}
	})
}
