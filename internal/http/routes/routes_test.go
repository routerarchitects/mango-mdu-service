package routes_test

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/routes"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

func TestRouteVisibility(t *testing.T) {
	// 1. Setup Public App
	publicApp := fiber.New()
	publicDeps := routes.PublicDeps{
		AuthHandler: func(c fiber.Ctx) error { return c.Next() },
		Subsystem:   subsysteroutes.Config{},
	}
	routes.RegisterPublic(publicApp, publicDeps)

	// 2. Setup Private App
	privateApp := fiber.New()
	privateDeps := routes.PrivateDeps{
		AuthHandler: func(c fiber.Ctx) error { return c.Next() },
		Subsystem:   subsysteroutes.Config{},
	}
	routes.RegisterPrivate(privateApp, privateDeps)

	// Check if public app has system route registered
	hasPublicSystemRoute := false
	for _, route := range publicApp.GetRoutes() {
		if route.Path == "/api/v1/system" {
			hasPublicSystemRoute = true
			break
		}
	}
	if !hasPublicSystemRoute {
		t.Errorf("expected public app to register /api/v1/system route, but it did not")
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
