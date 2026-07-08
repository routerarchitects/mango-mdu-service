package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type EntityHandler struct {
	service *services.EntityService
}

func NewEntityHandler(service *services.EntityService) *EntityHandler {
	return &EntityHandler{
		service: service,
	}
}

func (h *EntityHandler) Register(router fiber.Router) {
	group := router.Group("/entities")
	group.Get("", h.ListEntities)
	group.Get("/:entityId", h.GetEntity)
	group.Post("", h.CreateEntity)
	group.Put("/:entityId", h.UpdateEntity)
	group.Delete("/:entityId", h.DeleteEntity)
}

func (h *EntityHandler) ListEntities(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)

	limit := 100
	if lStr := c.Query("limit"); lStr != "" {
		val, err := strconv.Atoi(lStr)
		if err != nil || val < 0 {
			return apperror.New(apperror.CodeInvalidInput, "invalid limit parameter")
		}
		limit = val
	}

	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		val, err := strconv.Atoi(oStr)
		if err != nil || val < 0 {
			return apperror.New(apperror.CodeInvalidInput, "invalid offset parameter")
		}
		offset = val
	}

	resp, err := h.service.ListEntities(reqCtx, limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *EntityHandler) GetEntity(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	entityID := c.Params("entityId")
	resp, err := h.service.GetEntity(reqCtx, entityID)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *EntityHandler) CreateEntity(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)

	var req models.CreateEntityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid payload request")
	}

	if req.Name == "" {
		return apperror.New(apperror.CodeInvalidInput, "name is required")
	}

	resp, err := h.service.CreateEntity(reqCtx, &req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *EntityHandler) UpdateEntity(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	entityID := c.Params("entityId")

	var req models.UpdateEntityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid payload request")
	}

	resp, err := h.service.UpdateEntity(reqCtx, entityID, &req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *EntityHandler) DeleteEntity(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	entityID := c.Params("entityId")

	err := h.service.DeleteEntity(reqCtx, entityID)
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
