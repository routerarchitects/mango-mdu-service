package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

type PublicDeps struct {
	AuthHandler         fiber.Handler
	Subsystem           subsysteroutes.Config
	PolicyHandler       *handlers.PolicyHandler
	DeviceHandler       *handlers.DeviceHandler
	DeviceEventsHandler *handlers.DeviceEventsHandler
}

type PrivateDeps struct {
	AuthHandler   fiber.Handler
	Subsystem     subsysteroutes.Config
	DeviceHandler *handlers.DeviceHandler
}

// RegisterPublic configures the public HTTP router paths.
func RegisterPublic(app *fiber.App, deps PublicDeps) {
	registerLivenessRoute(app)

	// Register production OpenWiFi WebSocket routes (bypasses upfront HTTP Auth, enforces in-band token handshake)
	if deps.DeviceEventsHandler != nil {
		app.Get("/api/v1/ws", deps.DeviceEventsHandler.HandleWebSocket)
		app.Get("/ws", deps.DeviceEventsHandler.HandleWebSocket)
	}

	// Create authenticated route group
	group := app.Group("", deps.AuthHandler)

	// Register system diagnostics routes
	subsysteroutes.RegisterRoutes(deps.Subsystem, group)

	// Register policy overview route
	if deps.PolicyHandler != nil {
		group.Get("/api/v1/policy/:id/overview", deps.PolicyHandler.GetOverview)
	}

	// Register device routes
	if deps.DeviceHandler != nil {
		// Authorization routes
		group.Post("/api/v1/authorization/deviceCommand", deps.DeviceHandler.AuthorizeDeviceCommand)

		// Blacklist routes (register before /api/v1/devices/:serial wildcard)
		group.Get("/api/v1/devices/blacklist", deps.DeviceHandler.GetBlacklist)
		group.Post("/api/v1/devices/blacklist", deps.DeviceHandler.AddBlacklistDevice)
		group.Delete("/api/v1/devices/blacklist/:serial", deps.DeviceHandler.DeleteBlacklistDevice)
		group.Post("/api/v1/devices/:serial/blacklist", deps.DeviceHandler.AddBlacklistDevice)
		group.Delete("/api/v1/devices/:serial/blacklist", deps.DeviceHandler.DeleteBlacklistDevice)

		group.Get("/api/v1/devices", deps.DeviceHandler.GetDevices)
		group.Get("/api/v1/devices/:serial/commands", deps.DeviceHandler.GetDeviceCommands)
		group.Get("/api/v1/devices/:serial/healthchecks", deps.DeviceHandler.GetDeviceHealthChecks)
		group.Get("/api/v1/devices/:serial/logs", deps.DeviceHandler.GetDeviceLogs)
		group.Get("/api/v1/devices/:serial/statistics", deps.DeviceHandler.GetDeviceStatistics)
		group.Get("/api/v1/devices/:serial/status", deps.DeviceHandler.GetDeviceStatus)
		group.Get("/api/v1/devices/:serial/capabilities", deps.DeviceHandler.GetDeviceCapabilities)
		group.Post("/api/v1/devices/:serial/configuration/apply-provisioned", deps.DeviceHandler.ApplyProvisionedDeviceConfiguration)
		group.Post("/api/v1/devices/:serial/configuration/push-direct", deps.DeviceHandler.PushDirectDeviceConfiguration)
		group.Get("/api/v1/devices/:serial/rtty", deps.DeviceHandler.GetDeviceRtty)
		group.Get("/api/v1/devices/:serial", deps.DeviceHandler.GetDevice)
		group.Put("/api/v1/devices/:serial", deps.DeviceHandler.UpdateDevice)
		group.Delete("/api/v1/devices/:serial", deps.DeviceHandler.DeleteDevice)
		group.Post("/api/v1/devices/:serial/:command", deps.DeviceHandler.ExecuteDeviceCommand)
	}
}

// RegisterPrivate configures the private/internal HTTP router paths.
func RegisterPrivate(app *fiber.App, deps PrivateDeps) {
	registerLivenessRoute(app)

	// Create authenticated route group
	group := app.Group("", deps.AuthHandler)

	// Register system diagnostics routes
	subsysteroutes.RegisterRoutes(deps.Subsystem, group)

	// Register private device reboot route
	if deps.DeviceHandler != nil {
		group.Post("/api/v1/devices/:serial/reboot", deps.DeviceHandler.RebootDevice)
	}
}

func registerLivenessRoute(app *fiber.App) {
	app.Get("/livez", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
}
