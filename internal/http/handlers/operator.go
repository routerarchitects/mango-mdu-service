package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type OperatorHandler struct {
	service *services.OperatorService
}

func NewOperatorHandler(service *services.OperatorService) *OperatorHandler {
	return &OperatorHandler{
		service: service,
	}
}

func (h *OperatorHandler) GetOperator(c fiber.Ctx) error {
	operatorID := c.Params("operatorId")
	if operatorID == "" {
		return apperror.New(apperror.CodeInvalidInput, "operatorId path parameter is required")
	}

	reqCtx := h.buildRequestContext(c)
	detail, err := h.service.GetOperator(reqCtx, operatorID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(detail)
}

func (h *OperatorHandler) UpdateOperator(c fiber.Ctx) error {
	operatorID := c.Params("operatorId")
	if operatorID == "" {
		return apperror.New(apperror.CodeInvalidInput, "operatorId path parameter is required")
	}

	var req models.UpdateOperatorRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.Wrap(apperror.CodeInvalidInput, "invalid request body", err)
	}

	reqCtx := h.buildRequestContext(c)
	detail, err := h.service.UpdateOperator(reqCtx, operatorID, &req)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(detail)
}

func (h *OperatorHandler) DeleteOperator(c fiber.Ctx) error {
	operatorID := c.Params("operatorId")
	if operatorID == "" {
		return apperror.New(apperror.CodeInvalidInput, "operatorId path parameter is required")
	}

	reqCtx := h.buildRequestContext(c)
	err := h.service.DeleteOperator(reqCtx, operatorID)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *OperatorHandler) buildRequestContext(c fiber.Ctx) prov.RequestContext {
	return prov.RequestContext{
		Context:       c.Context(),
		BearerToken:   c.Get("Authorization"),
		RequestID:     c.Get("X-Request-Id"),
		CorrelationID: c.Get("X-Correlation-Id"),
	}
}

func (h *OperatorHandler) Register(router fiber.Router) {
	group := router.Group("/operators")
	group.Get("/:operatorId", h.GetOperator)
	group.Put("/:operatorId", h.UpdateOperator)
	group.Delete("/:operatorId", h.DeleteOperator)
}
