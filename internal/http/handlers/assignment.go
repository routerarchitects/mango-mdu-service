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

	if req.ScopeType != "entity" && req.ScopeType != "venue" {
		return apperror.New(apperror.CodeInvalidInput, "scopeType must be 'entity' or 'venue'")
	}

	resp, isAlreadyAssigned, err := h.service.CreateAssignment(reqCtx, userID, &req)
	if err != nil {
		return err
	}

	if isAlreadyAssigned {
		return c.Status(fiber.StatusOK).JSON(resp)
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
	roleTemplate := c.Query("roleTemplate")
	if roleTemplate == "" {
		roleTemplate = c.Query("role")
	}

	if scope != "entity" && scope != "venue" {
		return apperror.New(apperror.CodeInvalidInput, "scope must be 'entity' or 'venue'")
	}
	if entityID == "" {
		return apperror.New(apperror.CodeInvalidInput, "entityId is required")
	}
	if scope == "entity" {
		if venueID != "" {
			return apperror.New(apperror.CodeInvalidInput, "venueId must not be provided for entity scope")
		}
	} else if scope == "venue" {
		if venueID == "" {
			return apperror.New(apperror.CodeInvalidInput, "venueId is required for venue scope")
		}
	}

	resp, err := h.service.GetAccessPolicy(reqCtx, userID, scope, entityID, venueID, roleTemplate)
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

	if req.Scope != "entity" && req.Scope != "venue" {
		return apperror.New(apperror.CodeInvalidInput, "scope must be 'entity' or 'venue'")
	}
	if req.EntityID == "" {
		return apperror.New(apperror.CodeInvalidInput, "entityId is required")
	}
	if req.Scope == "entity" {
		if req.VenueID != "" {
			return apperror.New(apperror.CodeInvalidInput, "venueId must not be provided for entity scope")
		}
	} else if req.Scope == "venue" {
		if req.VenueID == "" {
			return apperror.New(apperror.CodeInvalidInput, "venueId is required for venue scope")
		}
	}

	resp, err := h.service.UpdateAccessPolicy(reqCtx, userID, &req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
