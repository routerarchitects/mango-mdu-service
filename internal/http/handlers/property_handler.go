package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

type PropertyHandler struct {
	svc    services.PropertyService
	logger *slog.Logger
}

func NewPropertyHandler(svc services.PropertyService, logger ...*slog.Logger) *PropertyHandler {
	l := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &PropertyHandler{svc: svc, logger: l}
}

func propertyRequestMetadata(c fiber.Ctx) (string, string, string) {
	return strings.Clone(c.Get("Authorization")), strings.Clone(c.Get("X-Request-Id")), strings.Clone(c.Get("X-Correlation-Id"))
}

func (h *PropertyHandler) respond(c fiber.Ctx, data any, err error) error {
	if err == nil {
		return c.Status(http.StatusOK).JSON(data)
	}
	var apiErr models.ApiError
	if errors.As(err, &apiErr) {
		return c.Status(apiErr.ErrorCode).JSON(apiErr)
	}
	h.logger.Error("property request failed", "error", err, "request_id", c.Get("X-Request-Id"), "correlation_id", c.Get("X-Correlation-Id"))
	return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve property data"))
}

func (h *PropertyHandler) GetTree(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	data, err := h.svc.GetTree(c.Context(), token, reqID, corrID)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) GetMap(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	data, err := h.svc.GetMap(c.Context(), token, reqID, corrID)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) GetPortfolio(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	data, err := h.svc.GetPortfolio(c.Context(), token, reqID, corrID)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) getOverview(c fiber.Ctx, scopeType string) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	data, err := h.svc.GetOverview(c.Context(), scopeType, c.Params("id"), token, reqID, corrID)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) GetPropertyOverview(c fiber.Ctx) error { return h.getOverview(c, "property") }
func (h *PropertyHandler) GetVenueOverview(c fiber.Ctx) error    { return h.getOverview(c, "venue") }

func intQuery(c fiber.Ctx, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return value
}

func (h *PropertyHandler) getDevices(c fiber.Ctx, scopeType string) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	query := models.ScopedDevicesQuery{
		PageIndex: intQuery(c, "pageIndex", 0), PageSize: intQuery(c, "pageSize", 25),
		Search: c.Query("search"), VenueID: c.Query("venue"), Availability: c.Query("availability"),
		DeviceType: c.Query("type"), Attention: c.Query("attention"),
	}
	data, err := h.svc.GetDevices(c.Context(), scopeType, c.Params("id"), token, reqID, corrID, query)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) GetPropertyDevices(c fiber.Ctx) error { return h.getDevices(c, "property") }
func (h *PropertyHandler) GetVenueDevices(c fiber.Ctx) error    { return h.getDevices(c, "venue") }

func (h *PropertyHandler) GetVenueMonitoring(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	data, err := h.svc.GetVenueMonitoring(c.Context(), c.Params("id"), token, reqID, corrID)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) StartVenueMonitoring(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	var req models.StartVenueMonitoringRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(models.NewApiError(http.StatusBadRequest, "Bad Request", "Invalid JSON request body"))
		}
	}
	data, err := h.svc.StartVenueMonitoring(c.Context(), c.Params("id"), req, token, reqID, corrID)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) StopVenueMonitoring(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	data, err := h.svc.StopVenueMonitoring(c.Context(), c.Params("id"), token, reqID, corrID)
	return h.respond(c, data, err)
}

func (h *PropertyHandler) GetVenueMonitoringClients(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	data, err := h.svc.GetVenueMonitoringClients(c.Context(), c.Params("id"), token, reqID, corrID)
	if err != nil {
		return h.respond(c, nil, err)
	}
	return c.Status(http.StatusOK).JSON(map[string]any{"entries": data})
}

func (h *PropertyHandler) GetVenueClientLifecycle(c fiber.Ctx) error {
	token, reqID, corrID := propertyRequestMetadata(c)
	fromDate, _ := strconv.ParseInt(c.Query("fromDate"), 10, 64)
	endDate, _ := strconv.ParseInt(c.Query("endDate"), 10, 64)
	limit := intQuery(c, "limit", 100)
	offset := intQuery(c, "offset", 0)

	data, err := h.svc.GetVenueClientLifecycle(c.Context(), c.Params("id"), c.Params("client"), fromDate, endDate, limit, offset, token, reqID, corrID)
	if err != nil {
		return h.respond(c, nil, err)
	}
	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}
