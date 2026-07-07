package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

type HierarchyHandler struct {
	service *services.HierarchyService
}

func NewHierarchyHandler(service *services.HierarchyService) *HierarchyHandler {
	return &HierarchyHandler{
		service: service,
	}
}

func (h *HierarchyHandler) Register(router fiber.Router) {
	router.Get("/hierarchy", h.GetHierarchyTree)
}

func (h *HierarchyHandler) GetHierarchyTree(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	scopeEntityID := c.Query("scopeEntityId")
	tree, err := h.service.GetHierarchyTree(reqCtx, scopeEntityID)
	if err != nil {
		return err
	}
	return c.JSON(tree)
}
