package router

import "github.com/gofiber/fiber/v3"

func RegisterPublic(app *fiber.App) {
	registerLivenessRoute(app)

	api := app.Group("/api/v1/mdu")

	// Customer APIs
	api.Post("/customers", notImplemented("CreateCustomer"))
	api.Get("/customers", notImplemented("ListCustomers"))
	api.Get("/customers/:customerId", notImplemented("GetCustomer"))
	api.Patch("/customers/:customerId", notImplemented("UpdateCustomer"))
	api.Delete("/customers/:customerId", notImplemented("DeleteCustomer"))

	// Customer Entity API
	api.Get("/customers/:customerId/entity", notImplemented("GetCustomerEntity"))

	// Customer Venue APIs
	api.Post("/customers/:customerId/venues/import", notImplemented("ImportCustomerVenues"))
	api.Get("/customers/:customerId/venues", notImplemented("ListCustomerVenues"))
	api.Get("/customers/:customerId/venues/:parentId/children", notImplemented("ListVenueChildren"))
	api.Get("/customers/:customerId/venues/:venueId", notImplemented("GetVenueByID"))
	api.Patch("/customers/:customerId/venues/:venueId", notImplemented("UpdateVenueByID"))
	api.Delete("/customers/:customerId/venues/:venueId", notImplemented("DeleteVenueByID"))

	// Operation / Saga APIs
	api.Get("/operations/:operationId", notImplemented("GetOperation"))
	api.Post("/operations/:operationId/retry", notImplemented("RetryOperation"))
	api.Post("/operations/:operationId/compensate", notImplemented("CompensateOperation"))

	// Device Inventory APIs
	api.Get("/devices", notImplemented("ListDevices"))
	api.Get("/devices/:serialNumber", notImplemented("GetDevice"))
	api.Post("/devices", notImplemented("CreateDevice"))
	api.Put("/devices/:serialNumber", notImplemented("UpdateDevice"))
	api.Delete("/devices/:serialNumber", notImplemented("DeleteDevice"))
	api.Post("/devices/import", notImplemented("ImportDevices"))
	api.Delete("/devices/import", notImplemented("DeleteImportedDevices"))

	// Venue Device APIs
	api.Get("/venues/:venueId/devices", notImplemented("ListVenueDevices"))
	api.Post("/venues/:venueId/devices", notImplemented("AddDevicesToVenue"))
	api.Put("/venues/:venueId/devices/:serialNumber", notImplemented("UpdateVenueDevice"))
	api.Delete("/venues/:venueId/devices/:serialNumber", notImplemented("RemoveDeviceFromVenue"))
	api.Delete("/venues/:venueId/devices", notImplemented("RemoveDevicesFromVenue"))
	api.Post("/venues/:venueId/devices/import", notImplemented("ImportDevicesToVenue"))
	api.Delete("/venues/:venueId/devices/import", notImplemented("DeleteImportedVenueDevices"))

	// Configuration APIs
	api.Get("/configurations", notImplemented("ListConfigurations"))
	api.Get("/configurations/:configurationId", notImplemented("GetConfiguration"))
	api.Post("/configurations", notImplemented("CreateConfiguration"))
	api.Put("/configurations/:configurationId", notImplemented("UpdateConfiguration"))
	api.Delete("/configurations/:configurationId", notImplemented("DeleteConfiguration"))

	// Venue Configuration APIs
	api.Get("/venues/:venueId/configurations", notImplemented("ListVenueConfigurations"))
	api.Put("/venues/:venueId/configuration/:configurationId", notImplemented("AssignConfigurationToVenue"))
	api.Delete("/venues/:venueId/configuration/:configurationId", notImplemented("RemoveConfigurationFromVenue"))

	// Access Point Configuration APIs
	api.Get("/access-points/:serialNumber/configuration", notImplemented("GetAccessPointConfiguration"))
	api.Put("/access-points/:serialNumber/configuration/:configurationId", notImplemented("AssignConfigurationToAccessPoint"))
	api.Delete("/access-points/:serialNumber/configuration", notImplemented("RemoveAccessPointConfiguration"))
	api.Post("/access-points/:serialNumber/configuration/apply", notImplemented("ApplyAccessPointConfiguration"))
}

func RegisterPrivate(app *fiber.App) {
	registerLivenessRoute(app)

	api := app.Group("/api/v1/mdu/internal")
	api.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"scope":  "private",
		})
	})
}

func registerLivenessRoute(app *fiber.App) {
	app.Get("/livez", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
}

func notImplemented(operation string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "NOT_IMPLEMENTED",
				"message": operation + " is not implemented yet.",
			},
		})
	}
}
