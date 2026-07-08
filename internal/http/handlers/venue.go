package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type VenueHandler struct {
	service *services.VenueService
}

func NewVenueHandler(service *services.VenueService) *VenueHandler {
	return &VenueHandler{
		service: service,
	}
}

func (h *VenueHandler) Register(router fiber.Router) {
	group := router.Group("/venues")
	group.Get("", h.ListVenues)
	group.Get("/:venueId", h.GetVenue)
	group.Post("", h.CreateVenue)
	group.Put("/:venueId", h.UpdateVenue)
	group.Delete("/:venueId", h.DeleteVenue)

	router.Get("/entities/:entityId/venues", h.ListEntityVenues)
	router.Post("/entities/:entityId/venues", h.CreateEntityVenue)
}

func (h *VenueHandler) ListVenues(c fiber.Ctx) error {
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

	resp, err := h.service.ListVenues(reqCtx, limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *VenueHandler) GetVenue(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	venueID := c.Params("venueId")
	resp, err := h.service.GetVenue(reqCtx, venueID)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *VenueHandler) CreateVenue(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)

	var req models.CreateVenueRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid payload request")
	}

	if req.Name == "" {
		return apperror.New(apperror.CodeInvalidInput, "name is required")
	}

	resp, err := h.service.CreateVenue(reqCtx, &req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *VenueHandler) UpdateVenue(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	venueID := c.Params("venueId")

	var req models.UpdateVenueRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid payload request")
	}

	resp, err := h.service.UpdateVenue(reqCtx, venueID, &req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *VenueHandler) DeleteVenue(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	venueID := c.Params("venueId")

	err := h.service.DeleteVenue(reqCtx, venueID)
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *VenueHandler) ListEntityVenues(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	entityID := c.Params("entityId")

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

	resp, err := h.service.ListEntityVenues(reqCtx, entityID, limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *VenueHandler) CreateEntityVenue(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	entityID := c.Params("entityId")

	var req models.CreateVenueRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid payload request")
	}

	if req.Name == "" {
		return apperror.New(apperror.CodeInvalidInput, "name is required")
	}

	resp, err := h.service.CreateEntityVenue(reqCtx, entityID, &req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}
