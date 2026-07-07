package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type AssignmentHandler struct {
	service *services.AssignmentService
}

func NewAssignmentHandler(service *services.AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{
		service: service,
	}
}

func (h *AssignmentHandler) Register(router fiber.Router) {
	group := router.Group("/users/:userId")
	group.Get("/assignments", h.ListAssignments)
	group.Post("/assignments", h.CreateAssignment)
	group.Delete("/assignments/:assignmentId", h.DeleteAssignment)

	group.Get("/access-policy", h.GetAccessPolicy)
	group.Put("/access-policy", h.UpdateAccessPolicy)
}

func (h *AssignmentHandler) ListAssignments(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	userID := c.Params("userId")

	resp, err := h.service.ListAssignments(reqCtx, userID)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *AssignmentHandler) CreateAssignment(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	userID := c.Params("userId")

	var req models.CreateUserAssignmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid payload request")
	}

	if req.ScopeID == "" || req.ScopeType == "" || req.Role == "" {
		return apperror.New(apperror.CodeInvalidInput, "scopeId, scopeType, and role are required")
	}

	resp, err := h.service.CreateAssignment(reqCtx, userID, &req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *AssignmentHandler) DeleteAssignment(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	userID := c.Params("userId")
	assignmentID := c.Params("assignmentId")

	err := h.service.DeleteAssignment(reqCtx, userID, assignmentID)
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AssignmentHandler) GetAccessPolicy(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	userID := c.Params("userId")

	scope := c.Query("scope")
	entityID := c.Query("entityId")
	venueID := c.Query("venueId")

	if scope == "" || (scope == "entity" && entityID == "") || (scope == "venue" && venueID == "") {
		return apperror.New(apperror.CodeInvalidInput, "scope, entityId (if scope=entity), or venueId (if scope=venue) are required")
	}

	resp, err := h.service.GetAccessPolicy(reqCtx, userID, scope, entityID, venueID)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *AssignmentHandler) UpdateAccessPolicy(c fiber.Ctx) error {
	reqCtx := buildRequestContext(c)
	userID := c.Params("userId")

	var req models.UserAccessPolicy
	if err := c.Bind().JSON(&req); err != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid payload request")
	}

	if req.Scope == "" || (req.Scope == "entity" && req.EntityID == "") || (req.Scope == "venue" && req.VenueID == "") {
		return apperror.New(apperror.CodeInvalidInput, "scope, entityId (if scope=entity), or venueId (if scope=venue) are required")
	}

	resp, err := h.service.UpdateAccessPolicy(reqCtx, userID, &req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
