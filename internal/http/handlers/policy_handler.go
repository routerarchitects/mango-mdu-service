package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

// PolicyHandler handles HTTP requests for policy overview.
type PolicyHandler struct {
	svc    services.PolicyService
	logger *slog.Logger
}

// NewPolicyHandler constructs a new PolicyHandler.
func NewPolicyHandler(svc services.PolicyService, logger ...*slog.Logger) *PolicyHandler {
	var l *slog.Logger
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	} else {
		l = slog.Default()
	}
	return &PolicyHandler{svc: svc, logger: l}
}

// GetOverview handles GET /api/v1/policy/:id/overview.
func (h *PolicyHandler) GetOverview(c fiber.Ctx) error {
	id := c.Params("id")

	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")

	overview, err := h.svc.GetPolicyOverview(c.Context(), id, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}

		if h.logger != nil {
			h.logger.Error("failed to get policy overview",
				"error", err,
				"policy_id", id,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}

		// Fallback generic 500 error - sanitize without leaking internal details
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Internal server error",
		))
	}

	return c.Status(http.StatusOK).JSON(overview)
}
