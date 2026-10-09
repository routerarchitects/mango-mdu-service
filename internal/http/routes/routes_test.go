package routes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
	"github.com/routerarchitects/mango-mdu-service/internal/http/routes"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

type dummyPolicyService struct{}

func (d *dummyPolicyService) GetPolicyOverview(ctx context.Context, policyID, token, reqID, corrID string) (*models.PolicyOverviewResponse, error) {
	return &models.PolicyOverviewResponse{}, nil
}

func TestRouteVisibility(t *testing.T) {
	policyHdlr := handlers.NewPolicyHandler(&dummyPolicyService{})

	// 1. Setup Public App
	publicApp := fiber.New()
	publicDeps := routes.PublicDeps{
		AuthHandler:   func(c fiber.Ctx) error { return c.Next() },
		Subsystem:     subsysteroutes.Config{},
		PolicyHandler: policyHdlr,
	}
	routes.RegisterPublic(publicApp, publicDeps)

	// 2. Setup Private App
	privateApp := fiber.New()
	privateDeps := routes.PrivateDeps{
		AuthHandler: func(c fiber.Ctx) error { return c.Next() },
		Subsystem:   subsysteroutes.Config{},
	}
	routes.RegisterPrivate(privateApp, privateDeps)

	// Check if public app has system route and policy overview route registered
	hasPublicSystemRoute := false
	hasPolicyOverviewRoute := false
	for _, route := range publicApp.GetRoutes() {
		if route.Path == "/api/v1/system" {
			hasPublicSystemRoute = true
		}
		if route.Path == "/api/v1/policy/:id/overview" {
			hasPolicyOverviewRoute = true
		}
	}
	if !hasPublicSystemRoute {
		t.Errorf("expected public app to register /api/v1/system route, but it did not")
	}
	if !hasPolicyOverviewRoute {
		t.Errorf("expected public app to register /api/v1/policy/:id/overview route, but it did not")
	}

	// Check if private app has system route registered
	hasPrivateSystemRoute := false
	for _, route := range privateApp.GetRoutes() {
		if route.Path == "/api/v1/system" {
			hasPrivateSystemRoute = true
			break
		}
	}
	if !hasPrivateSystemRoute {
		t.Errorf("expected private app to register /api/v1/system route, but it did not")
	}
}

func TestCORSPreflight_TC_POL_008(t *testing.T) {
	app := fiber.New()
	middleware.RegisterPublicCORS(app)

	policyHdlr := handlers.NewPolicyHandler(&dummyPolicyService{})
	routes.RegisterPublic(app, routes.PublicDeps{
		AuthHandler:   func(c fiber.Ctx) error { return c.Next() },
		Subsystem:     subsysteroutes.Config{},
		PolicyHandler: policyHdlr,
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/policy/523e4567-e89b-12d3-a456-426614174000/overview", nil)
	req.Header.Set("Origin", "https://operator.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Request-Id, X-Correlation-Id")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Errorf("expected 204 No Content or 200 OK for CORS preflight, got %d", resp.StatusCode)
	}

	allowHeaders := resp.Header.Get("Access-Control-Allow-Headers")
	if allowHeaders == "" {
		t.Errorf("expected Access-Control-Allow-Headers to be set, but got empty")
	}
}
