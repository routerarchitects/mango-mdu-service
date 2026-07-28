package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

type DashboardHandler struct {
	dashboardService *services.DashboardService
}

func NewDashboardHandler(dashboardService *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
	}
}

func (h *DashboardHandler) Register(router fiber.Router) {
	router.Get("/dashboard", h.GetDashboard)
	router.Get("/mdu/dashboard", h.GetDashboard)
}

func (h *DashboardHandler) GetDashboard(c fiber.Ctx) error {
	ctx := c.Context()
	scopeId := c.Query("scopeId")

	bearerToken := c.Get("Authorization")

	reqCtx := prov.RequestContext{
		Context:     ctx,
		BearerToken: bearerToken,
	}

	resp, err := h.dashboardService.GetDashboard(ctx, reqCtx, scopeId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(resp)
}
