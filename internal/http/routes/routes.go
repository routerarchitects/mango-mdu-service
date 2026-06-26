package routes

import (
	"github.com/gofiber/fiber/v3"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

type PublicDeps struct {
	AuthHandler fiber.Handler
	Subsystem   subsysteroutes.Config
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
	subsysteroutes.RegisterRoutes(deps.Subsystem, group)
}

// RegisterPrivate configures the private/internal HTTP router paths.
func RegisterPrivate(app *fiber.App, deps PrivateDeps) {
	registerLivenessRoute(app)

	// Create authenticated route group
	group := app.Group("", deps.AuthHandler)

	// Register system diagnostics routes
	subsysteroutes.RegisterRoutes(deps.Subsystem, group)
}

func registerLivenessRoute(app *fiber.App) {
	app.Get("/livez", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
}
