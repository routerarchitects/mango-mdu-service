// Package handlers implements the HTTP request handlers for the service.
package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
)

func buildRequestContext(c fiber.Ctx) prov.RequestContext {
	return prov.RequestContext{
		Context:       c.Context(),
		BearerToken:   c.Get("Authorization"),
		RequestID:     c.Get("X-Request-Id"),
		CorrelationID: c.Get("X-Correlation-Id"),
	}
}
