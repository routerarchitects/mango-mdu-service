package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owgw"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owprov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

// DeviceService defines business operations for managing fleet devices.
type DeviceService interface {
	GetDevices(ctx context.Context, token, reqID, corrID string, pageIndex, pageSize int, unassignedOnly bool) (*models.PaginatedDevicesResponse, error)
	GetDevice(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	UpdateDevice(ctx context.Context, serial string, body []byte, token, reqID, corrID string) (json.RawMessage, error)
	GetDeviceCommands(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	GetDeviceHealthChecks(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	GetDeviceLogs(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	GetDeviceStatistics(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	GetDeviceStatus(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	GetDeviceCapabilities(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	ApplyProvisionedDeviceConfiguration(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	PushDirectDeviceConfiguration(ctx context.Context, serial string, req models.DirectDeviceConfigurationRequest, token, reqID, corrID string) (json.RawMessage, error)
	ExecuteDeviceCommand(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error)
	GetDeviceRtty(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error)
	DeleteDevice(ctx context.Context, serial, token, reqID, corrID string) error
	GetBlacklist(ctx context.Context, token, reqID, corrID string) ([]models.BlacklistDevice, error)
	AddBlacklistDevice(ctx context.Context, serial, reason string, token, reqID, corrID string) (json.RawMessage, error)
	DeleteBlacklistDevice(ctx context.Context, serial, token, reqID, corrID string) error
}

type deviceService struct {
	provClient owprov.Client
	gwClient   owgw.Client
	logger     *slog.Logger
}

// NewDeviceService constructs a new DeviceService instance.
func NewDeviceService(provClient owprov.Client, gwClient owgw.Client, logger ...*slog.Logger) DeviceService {
	var l *slog.Logger
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	} else {
		l = slog.Default()
	}
	return &deviceService{
		provClient: provClient,
		gwClient:   gwClient,
		logger:     l,
	}
}

func normalizeSerial(s string) string {
	return models.NormalizeSerial(s)
}

func lookupFold(values map[string]any, name string) (any, bool) {
	for key, value := range values {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return nil, false
}

func validateOWGWCommandResponse(raw json.RawMessage, expectedSerial, command string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var response map[string]any
	if err := decoder.Decode(&response); err != nil || response == nil {
		return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWGW returned an invalid command response")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWGW returned an invalid command response")
	}

	// OWGW's telemetry handler returns a telemetry status object rather than the
	// CommandDetails object used by commands executed through WaitForCommand.
	if canonicalDeviceCommandAction(command) == "TELEMETRY" {
		action, actionOK := lookupFold(response, "action")
		status, statusOK := lookupFold(response, "status")
		actionText, actionIsString := action.(string)
		_, statusIsObject := status.(map[string]any)
		if !actionOK || !actionIsString || strings.TrimSpace(actionText) == "" ||
			!statusOK || !statusIsObject {
			return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWGW returned an invalid telemetry response")
		}
		return nil
	}

	uuid, uuidOK := lookupFold(response, "UUID")
	serial, serialOK := lookupFold(response, "serialNumber")
	returnedCommand, commandOK := lookupFold(response, "command")
	statusValue, statusOK := lookupFold(response, "status")
	errorCodeValue, errorCodeOK := lookupFold(response, "errorCode")
	uuidText, uuidIsString := uuid.(string)
	serialText, serialIsString := serial.(string)
	commandText, commandIsString := returnedCommand.(string)
	status, statusIsString := statusValue.(string)
	if !uuidOK || !uuidIsString || strings.TrimSpace(uuidText) == "" ||
		!serialOK || !serialIsString || normalizeSerial(serialText) != normalizeSerial(expectedSerial) ||
		!commandOK || !commandIsString || strings.TrimSpace(commandText) == "" ||
		!statusOK || !statusIsString || strings.TrimSpace(status) == "" || !errorCodeOK {
		return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWGW returned an incomplete command response")
	}

	var errorCode int64
	switch code := errorCodeValue.(type) {
	case json.Number:
		parsed, err := code.Int64()
		if err != nil {
			return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWGW returned an invalid command error code")
		}
		errorCode = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(code), 10, 64)
		if err != nil {
			return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWGW returned an invalid command error code")
		}
		errorCode = parsed
	default:
		return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWGW returned an invalid command error code")
	}

	errorText := ""
	for key, value := range response {
		switch {
		case strings.EqualFold(key, "errorText") || strings.EqualFold(key, "errorDescription"):
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				errorText = text
			}
		}
	}

	if errorCode != 0 || strings.EqualFold(strings.TrimSpace(status), "failed") {
		if strings.TrimSpace(errorText) == "" {
			errorText = "OWGW rejected or failed the device command"
		}
		return models.NewApiError(http.StatusBadRequest, "Command Failed", errorText)
	}
	return nil
}

func parseLastContact(v any) *int64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		n := int64(val)
		return &n
	case int64:
		return &val
	case int:
		n := int64(val)
		return &n
	case string:
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			return &parsed
		}
	}
	return nil
}

// GetDevices orchestrates fetching user-scoped inventory from OWPROV and live telemetry from OWGW.
func (s *deviceService) GetDevices(ctx context.Context, token, reqID, corrID string, pageIndex, pageSize int, unassignedOnly bool) (*models.PaginatedDevicesResponse, error) {
	if pageIndex < 0 {
		pageIndex = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := pageIndex * pageSize

	var (
		tags       []models.InventoryTag
		totalCount int
		tagsErr    error
		countErr   error
		wg         sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		tags, tagsErr = s.provClient.GetInventory(ctx, token, reqID, corrID, pageSize, offset, unassignedOnly)
	}()

	go func() {
		defer wg.Done()
		totalCount, countErr = s.provClient.GetInventoryCount(ctx, token, reqID, corrID, unassignedOnly)
	}()

	wg.Wait()

	if tagsErr != nil {
		return nil, tagsErr
	}

	if countErr != nil {
		if s.logger != nil {
			s.logger.Warn("failed to fetch total inventory count, defaulting to active page count", "error", countErr)
		}
		totalCount = offset + len(tags)
	}

	totalPages := 1
	if pageSize > 0 && totalCount > 0 {
		totalPages = int(math.Ceil(float64(totalCount) / float64(pageSize)))
	}
	if totalPages < 1 {
		totalPages = 1
	}

	if len(tags) == 0 {
		return &models.PaginatedDevicesResponse{
			Devices:    []models.DeviceFleetItem{},
			TotalCount: totalCount,
			PageIndex:  pageIndex,
			PageSize:   pageSize,
			TotalPages: totalPages,
		}, nil
	}

	// Collect unique normalized serial numbers
	serials := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.TrimSpace(tag.SerialNumber) != "" {
			serials = append(serials, tag.SerialNumber)
		}
	}

	// Concurrently query OWGW for live status of only these scoped serials
	var gwStatuses []models.GatewayDeviceStatus
	if len(serials) > 0 && s.gwClient != nil {
		var gwErr error
		gwStatuses, gwErr = s.gwClient.GetDevicesStatus(ctx, serials, token, reqID, corrID)
		if gwErr != nil && s.logger != nil {
			s.logger.Warn("failed to fetch live gateway statuses, continuing with inventory data", "error", gwErr)
		}
	}

	gwMap := make(map[string]models.GatewayDeviceStatus, len(gwStatuses))
	for _, st := range gwStatuses {
		gwMap[normalizeSerial(st.SerialNumber)] = st
	}

	now := time.Now().Unix()
	devices := make([]models.DeviceFleetItem, 0, len(tags))

	for _, tag := range tags {
		norm := normalizeSerial(tag.SerialNumber)
		liveGw, hasGw := gwMap[norm]

		// Resolve hardware model
		var model string
		if hasGw && liveGw.Compatible != "" {
			model = liveGw.Compatible
		} else if tag.DeviceType != "" {
			model = tag.DeviceType
		} else {
			model = "Universal AP"
		}

		// Resolve platform group/type (ap, switch, olg) matching OWGW
		platformType := ""
		if hasGw && liveGw.DeviceType != "" {
			platformType = strings.ToLower(strings.TrimSpace(liveGw.DeviceType))
		} else if tag.DevClass != "" && (tag.DevClass == "ap" || tag.DevClass == "switch" || tag.DevClass == "olg") {
			platformType = strings.ToLower(strings.TrimSpace(tag.DevClass))
		} else {
			lowerType := strings.ToLower(tag.DeviceType)
			if strings.Contains(lowerType, "olg") || strings.Contains(lowerType, "gateway") {
				platformType = "olg"
			} else if strings.Contains(lowerType, "switch") || strings.Contains(lowerType, "ecs") {
				platformType = "switch"
			} else {
				platformType = "ap"
			}
		}

		// Resolve authoritative hardware SKU (Finding F7)
		// DeviceType represents the specific hardware SKU (e.g. edgecore_ecs4100_12ph),
		// while DeviceGroup represents the normalized platform category (e.g. switch, ap, olg).
		hardwareSku := strings.TrimSpace(tag.DeviceType)
		if hardwareSku == "" && hasGw && liveGw.Compatible != "" {
			hardwareSku = strings.TrimSpace(liveGw.Compatible)
		}
		if hardwareSku == "" {
			hardwareSku = platformType
		}

		item := models.DeviceFleetItem{
			SerialNumber: tag.SerialNumber,
			Name:         tag.Name,
			Description:  tag.Description,
			Model:        model,
			DeviceGroup:  platformType,
			DeviceType:   hardwareSku,
			IPAddress:    "-",
		}

		if item.Name == "" {
			if hasGw && liveGw.Compatible != "" {
				item.Name = liveGw.Compatible
			} else {
				item.Name = tag.SerialNumber
			}
		}

		// Extended info mapping
		if tag.ExtendedInfo != nil {
			if tag.ExtendedInfo.Entity != nil {
				item.EntityID = tag.ExtendedInfo.Entity.ID
				item.EntityName = tag.ExtendedInfo.Entity.Name
			}
			if tag.ExtendedInfo.Venue != nil {
				item.VenueID = tag.ExtendedInfo.Venue.ID
				item.VenueName = tag.ExtendedInfo.Venue.Name
			}
			if tag.ExtendedInfo.Subscriber != nil {
				item.SubscriberID = tag.ExtendedInfo.Subscriber.ID
				item.SubscriberEmail = tag.ExtendedInfo.Subscriber.Email
			}
			if tag.ExtendedInfo.DeviceConfiguration != nil {
				item.ConfigID = tag.ExtendedInfo.DeviceConfiguration.ID
				item.ConfigName = tag.ExtendedInfo.DeviceConfiguration.Name
			}
		} else {
			item.EntityID = tag.Entity
			item.VenueID = tag.Venue
			item.SubscriberID = tag.Subscriber
			item.ConfigID = tag.DeviceConfiguration
		}

		if tag.RealMacAddress != "" {
			item.MacAddress = tag.RealMacAddress
		}

		// Gateway live metrics merge
		if hasGw {
			item.Connected = liveGw.Connected
			item.Sanity = liveGw.Sanity

			if liveGw.IPAddress != "" {
				item.IPAddress = liveGw.IPAddress
			}
			if liveGw.MacAddress != "" {
				item.MacAddress = liveGw.MacAddress
			}
			if liveGw.Firmware != "" {
				item.Firmware = liveGw.Firmware
			}

			item.Locale = liveGw.Locale
			item.MemoryUsedPct = liveGw.MemoryUsed
			item.Load = liveGw.Load
			item.Temperature = liveGw.Temperature
			item.Started = liveGw.Started
			if liveGw.Started != nil && *liveGw.Started > 0 && *liveGw.Started <= now {
				uptime := now - *liveGw.Started
				item.UptimeSeconds = &uptime
			}

			item.LastContact = liveGw.LastContact
			item.LastContactTimestamp = parseLastContact(liveGw.LastContact)
			item.LastRecordedContact = liveGw.LastRecordedContact
			item.LastFWUpdate = liveGw.LastFWUpdate
			item.ConnectReason = liveGw.ConnectReason
			item.CertificateExpiryDate = liveGw.CertificateExpiryDate
			item.HasGPS = liveGw.HasGPS
			item.Blacklisted = liveGw.BlackListed
			item.Simulated = liveGw.Simulated
			item.Restricted = liveGw.RestrictedDevice

			if b, ok := liveGw.HasRADIUSSessions.(bool); ok {
				item.HasRADIUSSessions = b
			} else if n, ok := liveGw.HasRADIUSSessions.(float64); ok {
				item.HasRADIUSSessions = n > 0
			}

			item.Associations2G = liveGw.Associations2G
			item.Associations5G = liveGw.Associations5G
			item.Associations6G = liveGw.Associations6G
			item.AssociationsTotal = liveGw.Associations2G + liveGw.Associations5G + liveGw.Associations6G
			item.RxBytes = liveGw.RxBytes
			item.TxBytes = liveGw.TxBytes
		}

		devices = append(devices, item)
	}

	return &models.PaginatedDevicesResponse{
		Devices:    devices,
		TotalCount: totalCount,
		PageIndex:  pageIndex,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// verifyDeviceAccess enforces strict multi-tenant RBAC by verifying via OWPROV that
// the device is assigned to an entity, venue, or subscriber accessible to the authenticated user.
func (s *deviceService) verifyDeviceAccess(ctx context.Context, serial, token, reqID, corrID string) error {
	norm := normalizeSerial(serial)
	if norm == "" {
		return models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}

	if s.provClient == nil {
		return models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov client unavailable")
	}

	_, err := s.provClient.GetInventoryItem(ctx, norm, token, reqID, corrID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("RBAC access check rejected: device not found or not assigned to caller's scope in OWPROV",
				"serial", norm,
				"error", err,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return models.NewApiError(http.StatusForbidden, "Forbidden", "Device is not assigned to user or access denied")
	}

	return nil
}

// GetDeviceCommands proxies commands history from OWGW after verifying user authorization in OWPROV.
func (s *deviceService) GetDeviceCommands(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	norm := normalizeSerial(serial)
	if queryParams == nil {
		queryParams = url.Values{}
	}
	queryParams.Set("serialNumber", norm)
	return s.gwClient.GetCommands(ctx, queryParams, token, reqID, corrID)
}

// GetDeviceHealthChecks proxies healthcheck history from OWGW after verifying user authorization in OWPROV.
func (s *deviceService) GetDeviceHealthChecks(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	norm := normalizeSerial(serial)
	if queryParams == nil {
		queryParams = url.Values{}
	}
	return s.gwClient.GetHealthChecks(ctx, norm, queryParams, token, reqID, corrID)
}

// GetDeviceLogs proxies device logs from OWGW after verifying user authorization in OWPROV.
func (s *deviceService) GetDeviceLogs(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	norm := normalizeSerial(serial)
	if queryParams == nil {
		queryParams = url.Values{}
	}
	return s.gwClient.GetLogs(ctx, norm, queryParams, token, reqID, corrID)
}

// GetDeviceStatistics proxies device statistics from OWGW after verifying user authorization in OWPROV.
func (s *deviceService) GetDeviceStatistics(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	norm := normalizeSerial(serial)
	if queryParams == nil {
		queryParams = url.Values{}
	}
	return s.gwClient.GetStatistics(ctx, norm, queryParams, token, reqID, corrID)
}

// GetDeviceStatus proxies live device status from OWGW after verifying user authorization in OWPROV.
func (s *deviceService) GetDeviceStatus(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	norm := normalizeSerial(serial)
	return s.gwClient.GetDeviceStatus(ctx, norm, token, reqID, corrID)
}

// GetDevice proxies device metadata from OWGW after verifying user authorization in OWPROV.
func (s *deviceService) GetDevice(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	norm := normalizeSerial(serial)
	return s.gwClient.GetDevice(ctx, norm, token, reqID, corrID)
}

// GetDeviceCapabilities proxies device hardware/firmware capabilities from OWGW after verifying user authorization in OWPROV.
func (s *deviceService) GetDeviceCapabilities(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	norm := normalizeSerial(serial)
	return s.gwClient.GetCapabilities(ctx, norm, token, reqID, corrID)
}

// UpdateDevice appends operational notes using the existing inventory READ authorization.
// Inventory assignments and configuration updates must use their dedicated APIs.
func (s *deviceService) UpdateDevice(ctx context.Context, serial string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
	norm := normalizeSerial(serial)
	if norm == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}
	var update models.DeviceNotesUpdateRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "Only serialNumber and notes containing note and optional created fields are supported")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "Request body must contain a single JSON object")
	}
	if update.SerialNumber != "" && normalizeSerial(update.SerialNumber) != norm {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "Body serialNumber must match the requested device")
	}
	if len(update.Notes) == 0 {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "At least one note is required")
	}
	for i := range update.Notes {
		update.Notes[i].Note = strings.TrimSpace(update.Notes[i].Note)
		if update.Notes[i].Note == "" {
			return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "Note text must not be blank")
		}
		update.Notes[i].Created = 0
	}
	update.SerialNumber = norm
	if err := s.verifyDeviceAccess(ctx, norm, token, reqID, corrID); err != nil {
		return nil, err
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}
	notesBody, err := json.Marshal(update)
	if err != nil {
		return nil, models.NewApiError(http.StatusInternalServerError, "Internal Server Error", "Failed to encode device notes")
	}
	return s.gwClient.UpdateDevice(ctx, norm, notesBody, token, reqID, corrID)
}

// canonicalDeviceCommandAction mirrors OWPROV's supported commands and aliases.
func canonicalDeviceCommandAction(action string) string {
	switch strings.ToUpper(action) {
	case "REBOOT", "CONFIGURE", "TELEMETRY", "UPGRADE", "TRACE", "RTTY", "SCRIPT", "EVENTQUEUE":
		return strings.ToUpper(action)
	case "BLINK", "LEDS":
		return "BLINK"
	case "WIFI_SCAN", "WIFISCAN":
		return "WIFI_SCAN"
	case "FACTORY_RESET", "FACTORY", "FACTORYRESET":
		return "FACTORY_RESET"
	case "RE_ENROLL", "REENROLL":
		return "RE_ENROLL"
	default:
		return ""
	}
}

// ApplyProvisionedDeviceConfiguration delegates compilation and delivery to
// OWPROV, matching the existing OWPROV UI applyConfiguration flow.
func (s *deviceService) ApplyProvisionedDeviceConfiguration(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	norm := normalizeSerial(serial)
	if norm == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}
	if s.provClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov client unavailable")
	}
	return s.provClient.ApplyDeviceConfiguration(ctx, norm, token, reqID, corrID)
}

// PushDirectDeviceConfiguration sends a concrete operator-supplied
// configuration to OWGW after explicit CONFIGURE authorization in OWPROV.
func (s *deviceService) PushDirectDeviceConfiguration(ctx context.Context, serial string, req models.DirectDeviceConfigurationRequest, token, reqID, corrID string) (json.RawMessage, error) {
	norm := normalizeSerial(serial)
	if norm == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}
	if len(req.Configuration) == 0 {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "configuration must be a non-empty JSON object")
	}

	hasSettings := false
	for key := range req.Configuration {
		if !strings.EqualFold(strings.TrimSpace(key), "uuid") {
			hasSettings = true
			break
		}
	}
	if !hasSettings {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "configuration must contain device settings in addition to uuid")
	}

	authRes, err := s.AuthorizeDeviceCommand(ctx, norm, "CONFIGURE", token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	if !authRes.Allowed {
		details := authRes.Details
		if details == "" {
			details = authRes.Reason
		}
		return nil, models.NewApiError(http.StatusForbidden, "Forbidden", details)
	}
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}

	commandUUID := uint64(time.Now().Unix())
	configuration := make(map[string]any, len(req.Configuration)+1)
	for key, value := range req.Configuration {
		if strings.EqualFold(strings.TrimSpace(key), "uuid") {
			continue
		}
		configuration[key] = value
	}
	configuration["uuid"] = commandUUID

	payload := map[string]any{
		"serialNumber":  norm,
		"UUID":          commandUUID,
		"when":          req.When,
		"configuration": configuration,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, models.NewApiError(http.StatusInternalServerError, "Internal Server Error", "failed to encode direct configuration request")
	}
	result, err := s.gwClient.ExecuteDeviceCommand(ctx, norm, "configure", body, token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	if err := validateOWGWCommandResponse(result, norm, "configure"); err != nil {
		return nil, err
	}
	return result, nil
}

// AuthorizeDeviceCommand verifies the decision applies to the requested device and command.
func (s *deviceService) AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
	norm := normalizeSerial(serial)
	if norm == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}
	if strings.TrimSpace(action) == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "action is required")
	}

	if s.provClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov client unavailable")
	}

	result, err := s.provClient.AuthorizeDeviceCommand(ctx, norm, action, token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWPROV returned an empty authorization decision")
	}
	if result.Allowed {
		expectedAction := canonicalDeviceCommandAction(action)
		if result.Resource != "deviceCommand" || normalizeSerial(result.SerialNumber) != norm ||
			expectedAction == "" || canonicalDeviceCommandAction(result.Action) != expectedAction {
			return nil, models.NewApiError(http.StatusBadGateway, "Bad Gateway", "OWPROV authorization decision does not match the requested device command")
		}
	}
	return result, nil
}

// ExecuteDeviceCommand forwards a device command to OWGW after pre-flight RBAC verification against OWPROV.
func (s *deviceService) ExecuteDeviceCommand(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
	norm := normalizeSerial(serial)
	if norm == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}
	if canonicalDeviceCommandAction(command) == "CONFIGURE" {
		return nil, models.NewApiError(
			http.StatusBadRequest,
			"Bad Request",
			"configure is not supported by the generic command endpoint; use /api/v1/devices/{serial}/configuration/push-direct",
		)
	}

	if s.provClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov client unavailable")
	}

	// 1. Authoritative pre-flight RBAC verification against OWPROV
	authRes, err := s.AuthorizeDeviceCommand(ctx, norm, command, token, reqID, corrID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to authorize device command with OWPROV",
				"serial", norm,
				"command", command,
				"error", err,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		return nil, err
	}

	if !authRes.Allowed {
		if s.logger != nil {
			s.logger.Warn("device command execution denied by OWPROV authorization policy",
				"serial", norm,
				"command", command,
				"reason", authRes.Reason,
				"details", authRes.Details,
				"policy_id", authRes.PolicyID,
				"policy_name", authRes.PolicyName,
				"request_id", reqID,
				"correlation_id", corrID,
			)
		}
		details := authRes.Details
		if details == "" {
			details = authRes.Reason
		}
		return nil, models.NewApiError(http.StatusForbidden, "Forbidden", details)
	}

	// 2. Authorization succeeded -> call OWGW to implement command on device
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}

	result, err := s.gwClient.ExecuteDeviceCommand(ctx, norm, command, body, token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	if err := validateOWGWCommandResponse(result, norm, command); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDeviceRtty initiates or fetches RTTY connection details from OWGW after verifying authorization.
func (s *deviceService) GetDeviceRtty(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	norm := normalizeSerial(serial)
	if norm == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}

	if s.provClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov client unavailable")
	}

	authRes, err := s.AuthorizeDeviceCommand(ctx, norm, "rtty", token, reqID, corrID)
	if err != nil {
		return nil, err
	}
	if !authRes.Allowed {
		details := authRes.Details
		if details == "" {
			details = authRes.Reason
		}
		return nil, models.NewApiError(http.StatusForbidden, "Forbidden", details)
	}

	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}

	return s.gwClient.GetDeviceRtty(ctx, norm, token, reqID, corrID)
}

// DeleteDevice deletes a device from OWPROV inventory and OWGW after verifying authorization.
func (s *deviceService) DeleteDevice(ctx context.Context, serial, token, reqID, corrID string) error {
	if err := s.verifyDeviceAccess(ctx, serial, token, reqID, corrID); err != nil {
		return err
	}
	norm := normalizeSerial(serial)

	// 1. Authoritative deletion from OWPROV inventory FIRST.
	// OWPROV is the authoritative source of truth for device existence and deletion authorization.
	// If OWPROV rejects the deletion (e.g. 403 Forbidden, 404 Not Found, 500 Server Error),
	// we must abort immediately without touching OWGW to avoid deleting devices from the gateway
	// when the caller lacks inventory deletion authorization.
	if s.provClient == nil {
		return models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov client unavailable")
	}
	if err := s.provClient.DeleteInventoryItem(ctx, norm, token, reqID, corrID); err != nil {
		if s.logger != nil {
			s.logger.Error("failed to delete device from OWPROV inventory", "serial", norm, "error", err)
		}
		return err
	}

	// 2. Best-effort cleanup in OWGW after authoritative inventory deletion has succeeded.
	// Ignore 404 Not Found since offline/unassigned devices may not be present in the gateway.
	if s.gwClient != nil {
		if err := s.gwClient.DeleteDevice(ctx, norm, token, reqID, corrID); err != nil {
			var apiErr models.ApiError
			if errors.As(err, &apiErr) && apiErr.ErrorCode == http.StatusNotFound {
				if s.logger != nil {
					s.logger.Debug("device not present in OWGW during deletion cleanup", "serial", norm)
				}
			} else {
				if s.logger != nil {
					s.logger.Warn("failed to delete device from OWGW during cleanup", "serial", norm, "error", err)
				}
				return err
			}
		}
	}

	return nil
}

// GetBlacklist returns blacklisted devices filtered by caller's inventory scope.
func (s *deviceService) GetBlacklist(ctx context.Context, token, reqID, corrID string) ([]models.BlacklistDevice, error) {
	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}

	// 1. Fetch blacklisted devices from OWGW
	gwDevices, err := s.gwClient.GetBlacklist(ctx, 500, 0, token, reqID, corrID)
	if err != nil {
		return nil, err
	}

	if len(gwDevices) == 0 {
		return []models.BlacklistDevice{}, nil
	}

	// 2. Fetch accessible inventory serial numbers for caller to enforce tenant scoping
	if s.provClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owprov client unavailable")
	}

	accessibleSerials, err := s.provClient.GetInventorySerials(ctx, token, reqID, corrID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to fetch user inventory serials for blacklist scoping", "error", err)
		}
		return nil, err
	}

	allowed := make(map[string]bool, len(accessibleSerials))
	for _, sn := range accessibleSerials {
		allowed[normalizeSerial(sn)] = true
	}

	// 3. Filter only devices that belong to the caller's inventory
	scoped := make([]models.BlacklistDevice, 0, len(gwDevices))
	for _, dev := range gwDevices {
		if allowed[normalizeSerial(dev.SerialNumber)] {
			scoped = append(scoped, dev)
		}
	}

	return scoped, nil
}

// AddBlacklistDevice verifies device inventory access before adding it to OWGW blacklist.
func (s *deviceService) AddBlacklistDevice(ctx context.Context, serial, reason string, token, reqID, corrID string) (json.RawMessage, error) {
	norm := normalizeSerial(serial)
	if norm == "" {
		return nil, models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}

	// Authorize device against user's inventory in OWPROV
	if err := s.verifyDeviceAccess(ctx, norm, token, reqID, corrID); err != nil {
		return nil, err
	}

	if s.gwClient == nil {
		return nil, models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}

	payload := map[string]string{
		"serialNumber": norm,
		"reason":       reason,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, models.NewApiError(http.StatusInternalServerError, "Internal Server Error", "failed to serialize blacklist payload: "+err.Error())
	}

	return s.gwClient.AddBlacklistDevice(ctx, norm, body, token, reqID, corrID)
}

// DeleteBlacklistDevice verifies device inventory access before removing it from OWGW blacklist.
func (s *deviceService) DeleteBlacklistDevice(ctx context.Context, serial, token, reqID, corrID string) error {
	norm := normalizeSerial(serial)
	if norm == "" {
		return models.NewApiError(http.StatusBadRequest, "Bad Request", "serial number is required")
	}

	// Authorize device against user's inventory in OWPROV
	if err := s.verifyDeviceAccess(ctx, norm, token, reqID, corrID); err != nil {
		return err
	}

	if s.gwClient == nil {
		return models.NewApiError(http.StatusServiceUnavailable, "Service Unavailable", "owgw client unavailable")
	}

	return s.gwClient.DeleteBlacklistDevice(ctx, norm, token, reqID, corrID)
}
