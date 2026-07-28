package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

type PublicDeps struct {
	AuthHandler       fiber.Handler
	Subsystem         subsysteroutes.Config
	SessionHandler    *handlers.SessionHandler
	HierarchyHandler  *handlers.HierarchyHandler
	EntityHandler     *handlers.EntityHandler
	VenueHandler      *handlers.VenueHandler
	AssignmentHandler *handlers.AssignmentHandler
	DashboardHandler  *handlers.DashboardHandler
}

type PrivateDeps struct {
	AuthHandler fiber.Handler
	Subsystem   subsysteroutes.Config
}

// RegisterPublic configures the public HTTP router paths.
func RegisterPublic(app *fiber.App, deps PublicDeps) {
	registerLivenessRoute(app)

	// Create authenticated route group
	group := app.Group("", deps.AuthHandler)

	// Register system diagnostics routes
	// NOTE: The subsysteroutes.RegisterRoutes helper internally hardcodes "/api/v1/system" paths.
	// We pass the root group ("") here to ensure the routes are mounted exactly at "/api/v1/system"
	// (passing the apiV1 group would result in "/api/v1/api/v1/system").
	subsysteroutes.RegisterRoutes(deps.Subsystem, group)

	// Base API V1 route group shared by all public endpoints
	apiV1 := group.Group("/api/v1")

	// Register handlers
	if deps.DashboardHandler != nil {
		deps.DashboardHandler.Register(apiV1)
	}
	if deps.SessionHandler != nil {
		deps.SessionHandler.Register(apiV1)
	}
	if deps.HierarchyHandler != nil {
		deps.HierarchyHandler.Register(apiV1)
	}
	if deps.EntityHandler != nil {
		deps.EntityHandler.Register(apiV1)
	}
	if deps.VenueHandler != nil {
		deps.VenueHandler.Register(apiV1)
	}
	if deps.AssignmentHandler != nil {
		deps.AssignmentHandler.Register(apiV1)
	}
}

// RegisterPrivate configures the private/internal HTTP router paths.
func RegisterPrivate(app *fiber.App, deps PrivateDeps) {
	registerLivenessRoute(app)

	// Create authenticated route group
	group := app.Group("", deps.AuthHandler)

	// Register system diagnostics routes
	// NOTE: The subsysteroutes.RegisterRoutes helper internally hardcodes "/api/v1/system" paths.
	// We pass the root group ("") here to ensure the routes are mounted exactly at "/api/v1/system".
	subsysteroutes.RegisterRoutes(deps.Subsystem, group)
}

func registerLivenessRoute(app *fiber.App) {
	app.Get("/livez", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
}
