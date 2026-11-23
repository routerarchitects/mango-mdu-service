package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
)

// DeviceHandler handles HTTP requests for fleet devices.
type DeviceHandler struct {
	svc    services.DeviceService
	logger *slog.Logger
}

// NewDeviceHandler constructs a new DeviceHandler.
func NewDeviceHandler(svc services.DeviceService, logger ...*slog.Logger) *DeviceHandler {
	var l *slog.Logger
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	} else {
		l = slog.Default()
	}
	return &DeviceHandler{svc: svc, logger: l}
}

// GetDevices handles GET /api/v1/devices.
func (h *DeviceHandler) GetDevices(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Authorization header is required",
		))
	}

	pageIndex := 0
	if p := c.Query("pageIndex"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val >= 0 {
			pageIndex = val
		}
	} else if p := c.Query("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val >= 0 {
			pageIndex = val
		}
	}

	pageSize := 10
	if ps := c.Query("pageSize"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil && val > 0 && val <= 500 {
			pageSize = val
		}
	} else if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 500 {
			pageSize = val
		}
	}

	unassignedOnly := false
	if u := c.Query("unassignedOnly"); u != "" {
		unassignedOnly = u == "true" || u == "1"
	} else if u := c.Query("unassigned"); u != "" {
		unassignedOnly = u == "true" || u == "1"
	}

	resp, err := h.svc.GetDevices(c.Context(), token, reqID, corrID, pageIndex, pageSize, unassignedOnly)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}

		if h.logger != nil {
			h.logger.Error("failed to get devices",
				"error", err,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}

		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve devices",
		))
	}

	return c.Status(http.StatusOK).JSON(resp)
}

func extractQueryParams(c fiber.Ctx) url.Values {
	values := url.Values{}
	c.Request().URI().QueryArgs().VisitAll(func(key, val []byte) {
		values.Add(string(key), string(val))
	})
	return values
}

// GetDevice handles GET /api/v1/devices/:serial.
func (h *DeviceHandler) GetDevice(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	data, err := h.svc.GetDevice(c.Context(), serial, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// UpdateDevice handles notes-only PUT /api/v1/devices/:serial requests.
func (h *DeviceHandler) UpdateDevice(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	body := c.Body()
	data, err := h.svc.UpdateDevice(c.Context(), serial, body, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// DeleteDevice handles DELETE /api/v1/devices/:serial.
func (h *DeviceHandler) DeleteDevice(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	if err := h.svc.DeleteDevice(c.Context(), serial, token, reqID, corrID); err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"success": true})
}

// GetDeviceCommands handles GET /api/v1/devices/:serial/commands.
func (h *DeviceHandler) GetDeviceCommands(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	queryParams := extractQueryParams(c)
	data, err := h.svc.GetDeviceCommands(c.Context(), serial, token, reqID, corrID, queryParams)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// ApplyProvisionedDeviceConfiguration handles
// POST /api/v1/devices/:serial/configuration/apply-provisioned.
func (h *DeviceHandler) ApplyProvisionedDeviceConfiguration(c fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	data, err := h.svc.ApplyProvisionedDeviceConfiguration(
		c.Context(), c.Params("serial"), token, c.Get("X-Request-Id"), c.Get("X-Correlation-Id"),
	)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// PushDirectDeviceConfiguration handles
// POST /api/v1/devices/:serial/configuration/push-direct.
func (h *DeviceHandler) PushDirectDeviceConfiguration(c fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	var request models.DirectDeviceConfigurationRequest
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "configuration and optional when fields are required",
		))
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "Request body must contain a single JSON object",
		))
	}

	data, err := h.svc.PushDirectDeviceConfiguration(
		c.Context(), c.Params("serial"), request, token, c.Get("X-Request-Id"), c.Get("X-Correlation-Id"),
	)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// ExecuteDeviceCommand handles POST /api/v1/devices/:serial/:command.
func (h *DeviceHandler) ExecuteDeviceCommand(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")
	command := c.Params("command")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	body := c.Body()
	data, err := h.svc.ExecuteDeviceCommand(c.Context(), serial, command, body, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// RebootDevice handles POST /api/v1/devices/:serial/reboot.
func (h *DeviceHandler) RebootDevice(c fiber.Ctx) error {
	token := strings.TrimSpace(c.Get("Authorization"))
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}
	parts := strings.Fields(token)
	if len(parts) < 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header must be a Bearer token",
		))
	}

	normSerial := models.NormalizeSerial(serial)
	if normSerial == "" {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "serial number is required",
		))
	}

	body := c.Body()
	if len(bytes.TrimSpace(body)) == 0 {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "Request body is required",
		))
	}

	var req models.DeviceRebootRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "Invalid JSON request body",
		))
	}

	normBodySerial := models.NormalizeSerial(req.SerialNumber)
	if normBodySerial == "" || normBodySerial != normSerial {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "serialNumber in body does not match route parameter",
		))
	}

	if req.When != nil && *req.When < 0 {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "invalid 'when' value; must be non-negative",
		))
	}

	data, err := h.svc.ExecuteDeviceCommand(c.Context(), serial, "reboot", body, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// AuthorizeDeviceCommand handles POST /api/v1/authorization/deviceCommand.
func (h *DeviceHandler) AuthorizeDeviceCommand(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	var req models.DeviceCommandAuthRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "Invalid JSON request body",
		))
	}

	if req.SerialNumber == "" || req.Action == "" {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "serialNumber and action are required",
		))
	}

	result, err := h.svc.AuthorizeDeviceCommand(c.Context(), req.SerialNumber, req.Action, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	return c.Status(http.StatusOK).JSON(result)
}

// GetDeviceHealthChecks handles GET /api/v1/devices/:serial/healthchecks.
func (h *DeviceHandler) GetDeviceHealthChecks(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	queryParams := extractQueryParams(c)
	data, err := h.svc.GetDeviceHealthChecks(c.Context(), serial, token, reqID, corrID, queryParams)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// GetDeviceLogs handles GET /api/v1/devices/:serial/logs.
func (h *DeviceHandler) GetDeviceLogs(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	queryParams := extractQueryParams(c)
	data, err := h.svc.GetDeviceLogs(c.Context(), serial, token, reqID, corrID, queryParams)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// GetDeviceStatistics handles GET /api/v1/devices/:serial/statistics.
func (h *DeviceHandler) GetDeviceStatistics(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	queryParams := extractQueryParams(c)
	data, err := h.svc.GetDeviceStatistics(c.Context(), serial, token, reqID, corrID, queryParams)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// GetDeviceStatus handles GET /api/v1/devices/:serial/status.
func (h *DeviceHandler) GetDeviceStatus(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	data, err := h.svc.GetDeviceStatus(c.Context(), serial, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// GetDeviceCapabilities handles GET /api/v1/devices/:serial/capabilities.
func (h *DeviceHandler) GetDeviceCapabilities(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	data, err := h.svc.GetDeviceCapabilities(c.Context(), serial, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// GetDeviceRtty handles GET /api/v1/devices/:serial/rtty.
func (h *DeviceHandler) GetDeviceRtty(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	data, err := h.svc.GetDeviceRtty(c.Context(), serial, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// GetBlacklist handles GET /api/v1/devices/blacklist.
func (h *DeviceHandler) GetBlacklist(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	devices, err := h.svc.GetBlacklist(c.Context(), token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	return c.Status(http.StatusOK).JSON(models.BlacklistListResponse{Devices: devices})
}

// AddBlacklistDevice handles POST /api/v1/devices/blacklist and POST /api/v1/devices/:serial/blacklist.
func (h *DeviceHandler) AddBlacklistDevice(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	var req models.AddBlacklistRequest
	if len(c.Body()) > 0 {
		_ = json.Unmarshal(c.Body(), &req)
	}

	serial := c.Params("serial")
	if serial == "" {
		serial = req.SerialNumber
	}
	if serial == "" {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "serial number is required",
		))
	}

	reason := req.Reason
	if reason == "" {
		reason = "Blocked by security operator"
	}

	data, err := h.svc.AddBlacklistDevice(c.Context(), serial, reason, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(data)
}

// DeleteBlacklistDevice handles DELETE /api/v1/devices/blacklist/:serial and DELETE /api/v1/devices/:serial/blacklist.
func (h *DeviceHandler) DeleteBlacklistDevice(c fiber.Ctx) error {
	token := c.Get("Authorization")
	reqID := c.Get("X-Request-Id")
	corrID := c.Get("X-Correlation-Id")
	serial := c.Params("serial")

	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(models.NewApiError(
			http.StatusUnauthorized, "Unauthorized", "Authorization header is required",
		))
	}

	if serial == "" {
		return c.Status(http.StatusBadRequest).JSON(models.NewApiError(
			http.StatusBadRequest, "Bad Request", "serial number is required",
		))
	}

	err := h.svc.DeleteBlacklistDevice(c.Context(), serial, token, reqID, corrID)
	if err != nil {
		var apiErr models.ApiError
		if errors.As(err, &apiErr) {
			return c.Status(apiErr.ErrorCode).JSON(apiErr)
		}
		return c.Status(http.StatusInternalServerError).JSON(models.NewApiError(
			http.StatusInternalServerError, "Internal Server Error", err.Error(),
		))
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"success": true})
}
