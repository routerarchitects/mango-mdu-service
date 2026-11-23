package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type mockDeviceService struct {
	getDevicesFn             func(ctx context.Context, token, reqID, corrID string, pageIndex, pageSize int, unassignedOnly bool) (*models.PaginatedDevicesResponse, error)
	getDeviceCommandsFn      func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	getDeviceHealthChecksFn  func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	getDeviceLogsFn          func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	getDeviceStatisticsFn    func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error)
	getDeviceStatusFn        func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	getDeviceFn              func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	updateDeviceFn           func(ctx context.Context, serial string, body []byte, token, reqID, corrID string) (json.RawMessage, error)
	getDeviceCapabilitiesFn  func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	applyProvisionedConfigFn func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error)
	pushDirectConfigFn       func(ctx context.Context, serial string, req models.DirectDeviceConfigurationRequest, token, reqID, corrID string) (json.RawMessage, error)
	executeDeviceCommandFn   func(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error)
	deleteDeviceFn           func(ctx context.Context, serial, token, reqID, corrID string) error
	getBlacklistFn           func(ctx context.Context, token, reqID, corrID string) ([]models.BlacklistDevice, error)
	addBlacklistFn           func(ctx context.Context, serial, reason string, token, reqID, corrID string) (json.RawMessage, error)
	deleteBlacklistFn        func(ctx context.Context, serial, token, reqID, corrID string) error
	authorizeDeviceCommandFn func(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error)
}

func (m *mockDeviceService) GetDevices(ctx context.Context, token, reqID, corrID string, pageIndex, pageSize int, unassignedOnly bool) (*models.PaginatedDevicesResponse, error) {
	if m.getDevicesFn != nil {
		return m.getDevicesFn(ctx, token, reqID, corrID, pageIndex, pageSize, unassignedOnly)
	}
	return nil, nil
}

func (m *mockDeviceService) GetDeviceCommands(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if m.getDeviceCommandsFn != nil {
		return m.getDeviceCommandsFn(ctx, serial, token, reqID, corrID, queryParams)
	}
	return json.RawMessage(`{"commands":[]}`), nil
}

func (m *mockDeviceService) GetDeviceHealthChecks(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if m.getDeviceHealthChecksFn != nil {
		return m.getDeviceHealthChecksFn(ctx, serial, token, reqID, corrID, queryParams)
	}
	return json.RawMessage(`{"values":[]}`), nil
}

func (m *mockDeviceService) GetDeviceLogs(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if m.getDeviceLogsFn != nil {
		return m.getDeviceLogsFn(ctx, serial, token, reqID, corrID, queryParams)
	}
	return json.RawMessage(`{"values":[]}`), nil
}

func (m *mockDeviceService) GetDeviceStatistics(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
	if m.getDeviceStatisticsFn != nil {
		return m.getDeviceStatisticsFn(ctx, serial, token, reqID, corrID, queryParams)
	}
	return json.RawMessage(`{"data":[]}`), nil
}

func (m *mockDeviceService) GetDeviceStatus(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	if m.getDeviceStatusFn != nil {
		return m.getDeviceStatusFn(ctx, serial, token, reqID, corrID)
	}
	return json.RawMessage(`{"status":{}}`), nil
}

func (m *mockDeviceService) GetDevice(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, serial, token, reqID, corrID)
	}
	return json.RawMessage(`{"serialNumber":"60cf84f22290"}`), nil
}

func (m *mockDeviceService) UpdateDevice(ctx context.Context, serial string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
	if m.updateDeviceFn != nil {
		return m.updateDeviceFn(ctx, serial, body, token, reqID, corrID)
	}
	return json.RawMessage(`{"serialNumber":"` + serial + `"}`), nil
}

func (m *mockDeviceService) GetDeviceCapabilities(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	if m.getDeviceCapabilitiesFn != nil {
		return m.getDeviceCapabilitiesFn(ctx, serial, token, reqID, corrID)
	}
	return json.RawMessage(`{"capabilities":{"model":"ASUS RT-AX53U"}}`), nil
}

func (m *mockDeviceService) ApplyProvisionedDeviceConfiguration(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	if m.applyProvisionedConfigFn != nil {
		return m.applyProvisionedConfigFn(ctx, serial, token, reqID, corrID)
	}
	return json.RawMessage(`{"errorCode":0}`), nil
}

func (m *mockDeviceService) PushDirectDeviceConfiguration(ctx context.Context, serial string, req models.DirectDeviceConfigurationRequest, token, reqID, corrID string) (json.RawMessage, error) {
	if m.pushDirectConfigFn != nil {
		return m.pushDirectConfigFn(ctx, serial, req, token, reqID, corrID)
	}
	return json.RawMessage(`{"status":"pending"}`), nil
}

func (m *mockDeviceService) ExecuteDeviceCommand(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
	if m.executeDeviceCommandFn != nil {
		return m.executeDeviceCommandFn(ctx, serial, command, body, token, reqID, corrID)
	}
	return json.RawMessage(`{"result":"ok"}`), nil
}

func (m *mockDeviceService) GetDeviceRtty(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	return json.RawMessage(`{"server":"tty.example.com","viewport":5912,"connectionId":"conn-123"}`), nil
}

func (m *mockDeviceService) DeleteDevice(ctx context.Context, serial, token, reqID, corrID string) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, serial, token, reqID, corrID)
	}
	return nil
}

func (m *mockDeviceService) GetBlacklist(ctx context.Context, token, reqID, corrID string) ([]models.BlacklistDevice, error) {
	if m.getBlacklistFn != nil {
		return m.getBlacklistFn(ctx, token, reqID, corrID)
	}
	return []models.BlacklistDevice{}, nil
}

func (m *mockDeviceService) AddBlacklistDevice(ctx context.Context, serial, reason string, token, reqID, corrID string) (json.RawMessage, error) {
	if m.addBlacklistFn != nil {
		return m.addBlacklistFn(ctx, serial, reason, token, reqID, corrID)
	}
	return json.RawMessage(`{"serialNumber":"` + serial + `","reason":"` + reason + `"}`), nil
}

func (m *mockDeviceService) DeleteBlacklistDevice(ctx context.Context, serial, token, reqID, corrID string) error {
	if m.deleteBlacklistFn != nil {
		return m.deleteBlacklistFn(ctx, serial, token, reqID, corrID)
	}
	return nil
}

func (m *mockDeviceService) AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
	if m.authorizeDeviceCommandFn != nil {
		return m.authorizeDeviceCommandFn(ctx, serial, action, token, reqID, corrID)
	}
	return &models.DeviceCommandAuthResult{
		Allowed:      true,
		Resource:     "deviceCommand",
		Action:       action,
		SerialNumber: serial,
		Reason:       "ALLOWED",
	}, nil
}

func setupDeviceTestApp(svc *mockDeviceService) *fiber.App {
	app := fiber.New()
	handler := handlers.NewDeviceHandler(svc)
	app.Get("/api/v1/devices/blacklist", handler.GetBlacklist)
	app.Post("/api/v1/devices/blacklist", handler.AddBlacklistDevice)
	app.Delete("/api/v1/devices/blacklist/:serial", handler.DeleteBlacklistDevice)
	app.Post("/api/v1/devices/:serial/blacklist", handler.AddBlacklistDevice)
	app.Delete("/api/v1/devices/:serial/blacklist", handler.DeleteBlacklistDevice)
	app.Get("/api/v1/devices", handler.GetDevices)
	app.Get("/api/v1/devices/:serial/commands", handler.GetDeviceCommands)
	app.Get("/api/v1/devices/:serial/healthchecks", handler.GetDeviceHealthChecks)
	app.Get("/api/v1/devices/:serial/logs", handler.GetDeviceLogs)
	app.Get("/api/v1/devices/:serial/statistics", handler.GetDeviceStatistics)
	app.Get("/api/v1/devices/:serial/status", handler.GetDeviceStatus)
	app.Get("/api/v1/devices/:serial/capabilities", handler.GetDeviceCapabilities)
	app.Post("/api/v1/devices/:serial/configuration/apply-provisioned", handler.ApplyProvisionedDeviceConfiguration)
	app.Post("/api/v1/devices/:serial/configuration/push-direct", handler.PushDirectDeviceConfiguration)
	app.Get("/api/v1/devices/:serial/rtty", handler.GetDeviceRtty)
	app.Get("/api/v1/devices/:serial", handler.GetDevice)
	app.Put("/api/v1/devices/:serial", handler.UpdateDevice)
	app.Delete("/api/v1/devices/:serial", handler.DeleteDevice)
	app.Post("/api/v1/devices/:serial/reboot", handler.RebootDevice)
	app.Post("/api/v1/devices/:serial/:command", handler.ExecuteDeviceCommand)
	app.Post("/api/v1/authorization/deviceCommand", handler.AuthorizeDeviceCommand)
	return app
}

func TestDeviceHandler_GetDevices_MissingAuth(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_GetDevices_Success(t *testing.T) {
	svc := &mockDeviceService{
		getDevicesFn: func(ctx context.Context, token, reqID, corrID string, pageIndex, pageSize int, unassignedOnly bool) (*models.PaginatedDevicesResponse, error) {
			if token != "Bearer my-jwt" {
				t.Errorf("expected token 'Bearer my-jwt', got %q", token)
			}
			if pageIndex != 1 || pageSize != 20 || !unassignedOnly {
				t.Errorf("unexpected params: pageIndex=%d pageSize=%d unassigned=%v", pageIndex, pageSize, unassignedOnly)
			}
			return &models.PaginatedDevicesResponse{
				Devices: []models.DeviceFleetItem{
					{
						SerialNumber: "60cf84f22290",
						Name:         "AP-Lobby",
						Connected:    true,
					},
				},
				TotalCount: 1,
				PageIndex:  1,
				PageSize:   20,
				TotalPages: 1,
			}, nil
		},
	}

	app := setupDeviceTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices?pageIndex=1&pageSize=20&unassignedOnly=true", nil)
	req.Header.Set("Authorization", "Bearer my-jwt")
	req.Header.Set("X-Request-Id", "req-123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var data models.PaginatedDevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(data.Devices) != 1 || data.Devices[0].SerialNumber != "60cf84f22290" {
		t.Errorf("unexpected devices payload: %+v", data)
	}
}

func TestDeviceHandler_GetDevices_ApiError(t *testing.T) {
	svc := &mockDeviceService{
		getDevicesFn: func(ctx context.Context, token, reqID, corrID string, pageIndex, pageSize int, unassignedOnly bool) (*models.PaginatedDevicesResponse, error) {
			return nil, models.NewApiError(http.StatusForbidden, "Forbidden", "Scope permission denied")
		},
	}

	app := setupDeviceTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer token-123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_GetDevices_GenericError(t *testing.T) {
	svc := &mockDeviceService{
		getDevicesFn: func(ctx context.Context, token, reqID, corrID string, pageIndex, pageSize int, unassignedOnly bool) (*models.PaginatedDevicesResponse, error) {
			return nil, errors.New("unhandled internal crash")
		},
	}

	app := setupDeviceTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer token-123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_ProxyEndpoints_MissingAuth(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/devices/60cf84f22290/commands"},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/healthchecks"},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/logs"},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/statistics"},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/status"},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/capabilities"},
		{http.MethodGet, "/api/v1/devices/60cf84f22290"},
		{http.MethodPut, "/api/v1/devices/60cf84f22290"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed test request %s %s: %v", ep.method, ep.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 for %s %s, got %d", ep.method, ep.path, resp.StatusCode)
		}
	}
}

func TestDeviceHandler_ProxyEndpoints_Success(t *testing.T) {
	svc := &mockDeviceService{
		getDeviceCommandsFn: func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
			if queryParams.Get("limit") != "25" || queryParams.Get("newest") != "true" {
				t.Errorf("unexpected query params: %v", queryParams)
			}
			return json.RawMessage(`{"commands":[{"command":"configure"}]}`), nil
		},
		getDeviceHealthChecksFn: func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
			return json.RawMessage(`{"values":[{"sanity":100}]}`), nil
		},
		getDeviceLogsFn: func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
			return json.RawMessage(`{"values":[{"log":"booting"}]}`), nil
		},
		getDeviceStatisticsFn: func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
			return json.RawMessage(`{"data":[{"tx":1000}]}`), nil
		},
		getDeviceStatusFn: func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
			return json.RawMessage(`{"serialNumber":"60cf84f22290","connected":true}`), nil
		},
		getDeviceFn: func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
			return json.RawMessage(`{"serialNumber":"60cf84f22290","compatible":"asus_rt-ax53u"}`), nil
		},
		getDeviceCapabilitiesFn: func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
			return json.RawMessage(`{"serialNumber":"60cf84f22290","capabilities":{"model":"ASUS RT-AX53U"}}`), nil
		},
	}

	app := setupDeviceTestApp(svc)

	tests := []struct {
		method         string
		path           string
		expectedStatus int
	}{
		{http.MethodGet, "/api/v1/devices/60cf84f22290/commands?limit=25&newest=true", http.StatusOK},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/healthchecks?limit=10", http.StatusOK},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/logs?limit=50&logType=crash", http.StatusOK},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/statistics?lastOnly=true", http.StatusOK},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/status", http.StatusOK},
		{http.MethodGet, "/api/v1/devices/60cf84f22290/capabilities", http.StatusOK},
		{http.MethodGet, "/api/v1/devices/60cf84f22290", http.StatusOK},
		{http.MethodPut, "/api/v1/devices/60cf84f22290", http.StatusOK},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		req.Header.Set("Authorization", "Bearer valid-jwt")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %s %s failed: %v", tt.method, tt.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != tt.expectedStatus {
			t.Errorf("expected %d for %s %s, got %d", tt.expectedStatus, tt.method, tt.path, resp.StatusCode)
		}
	}
}

func TestDeviceHandler_ProxyEndpoints_RBACForbidden(t *testing.T) {
	svc := &mockDeviceService{
		getDeviceCommandsFn: func(ctx context.Context, serial, token, reqID, corrID string, queryParams url.Values) (json.RawMessage, error) {
			return nil, models.NewApiError(http.StatusForbidden, "Forbidden", "Device is not assigned to user or access denied")
		},
	}

	app := setupDeviceTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/unassigned-device/commands", nil)
	req.Header.Set("Authorization", "Bearer rogue-jwt")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_DeleteDevice_Success(t *testing.T) {
	svc := &mockDeviceService{
		deleteDeviceFn: func(ctx context.Context, serial, token, reqID, corrID string) error {
			if serial != "60cf84f22290" {
				t.Errorf("expected serial 60cf84f22290, got %s", serial)
			}
			return nil
		},
	}

	app := setupDeviceTestApp(svc)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/60cf84f22290", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Errorf("expected success: true, got %v", result)
	}
}

func TestDeviceHandler_DeleteDevice_DownstreamError(t *testing.T) {
	svc := &mockDeviceService{
		deleteDeviceFn: func(ctx context.Context, serial, token, reqID, corrID string) error {
			return models.NewApiError(http.StatusBadGateway, "Bad Gateway", "failed to reach downstream owprov service")
		},
	}

	app := setupDeviceTestApp(svc)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/60cf84f22290", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", resp.StatusCode)
	}

	var apiErr models.ApiError
	json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.ErrorCode != http.StatusBadGateway || apiErr.ErrorDescription != "Bad Gateway" {
		t.Errorf("unexpected ApiError: %+v", apiErr)
	}
}

func TestDeviceHandler_DeleteDevice_MissingAuth(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/60cf84f22290", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_GetBlacklist_Success(t *testing.T) {
	svc := &mockDeviceService{
		getBlacklistFn: func(ctx context.Context, token, reqID, corrID string) ([]models.BlacklistDevice, error) {
			return []models.BlacklistDevice{
				{SerialNumber: "60cf84f22290", Reason: "Security rogue"},
			}, nil
		},
	}
	app := setupDeviceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/blacklist", nil)
	req.Header.Set("Authorization", "Bearer jwt")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var res models.BlacklistListResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if len(res.Devices) != 1 || res.Devices[0].SerialNumber != "60cf84f22290" {
		t.Errorf("unexpected blacklist response: %+v", res)
	}
}

func TestDeviceHandler_AddBlacklistDevice_Success(t *testing.T) {
	svc := &mockDeviceService{
		addBlacklistFn: func(ctx context.Context, serial, reason string, token, reqID, corrID string) (json.RawMessage, error) {
			if serial != "60cf84f22290" {
				t.Errorf("expected serial 60cf84f22290, got %s", serial)
			}
			return json.RawMessage(`{"serialNumber":"60cf84f22290","reason":"` + reason + `"}`), nil
		},
	}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"60cf84f22290","reason":"Blocked reason"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/blacklist", body)
	req.Header.Set("Authorization", "Bearer jwt")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_DeleteBlacklistDevice_Success(t *testing.T) {
	svc := &mockDeviceService{
		deleteBlacklistFn: func(ctx context.Context, serial, token, reqID, corrID string) error {
			if serial != "60cf84f22290" {
				t.Errorf("expected serial 60cf84f22290, got %s", serial)
			}
			return nil
		},
	}
	app := setupDeviceTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/blacklist/60cf84f22290", nil)
	req.Header.Set("Authorization", "Bearer jwt")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_AuthorizeDeviceCommand_Success(t *testing.T) {
	svc := &mockDeviceService{
		authorizeDeviceCommandFn: func(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
			return &models.DeviceCommandAuthResult{
				Allowed:      true,
				Resource:     "deviceCommand",
				Action:       action,
				SerialNumber: serial,
				Reason:       "ALLOWED",
			}, nil
		},
	}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"60cf84f22290","action":"reboot"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/authorization/deviceCommand", body)
	req.Header.Set("Authorization", "Bearer jwt")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var authRes models.DeviceCommandAuthResult
	if err := json.NewDecoder(resp.Body).Decode(&authRes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !authRes.Allowed {
		t.Errorf("expected allowed=true, got false")
	}
}

func TestDeviceHandler_ApplyProvisionedDeviceConfiguration(t *testing.T) {
	called := 0
	svc := &mockDeviceService{applyProvisionedConfigFn: func(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
		called++
		if serial != "60:cf:84:f2:22:90" || token != "Bearer jwt" || reqID != "req" || corrID != "corr" {
			t.Fatalf("unexpected service arguments: serial=%q token=%q reqID=%q corrID=%q", serial, token, reqID, corrID)
		}
		return json.RawMessage(`{"errorCode":0}`), nil
	}}
	app := setupDeviceTestApp(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/60:cf:84:f2:22:90/configuration/apply-provisioned", nil)
	req.Header.Set("Authorization", "Bearer jwt")
	req.Header.Set("X-Request-Id", "req")
	req.Header.Set("X-Correlation-Id", "corr")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || called != 1 {
		t.Fatalf("expected status 200 and one service call, got status=%d calls=%d", resp.StatusCode, called)
	}
}

func TestDeviceHandler_PushDirectDeviceConfiguration_StrictBody(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		called := 0
		svc := &mockDeviceService{pushDirectConfigFn: func(ctx context.Context, serial string, request models.DirectDeviceConfigurationRequest, token, reqID, corrID string) (json.RawMessage, error) {
			called++
			if serial != "60cf84f22290" || request.When != 9 || request.Configuration["radios"] == nil {
				t.Fatalf("unexpected direct configuration request: serial=%q request=%+v", serial, request)
			}
			radios, ok := request.Configuration["radios"].(map[string]any)
			if !ok {
				t.Fatalf("expected radios object, got %T", request.Configuration["radios"])
			}
			counter, ok := radios["counter"].(json.Number)
			if !ok || counter.String() != "9007199254740993" {
				t.Fatalf("large integer lost precision: %#v (%T)", radios["counter"], radios["counter"])
			}
			return json.RawMessage(`{"status":"pending"}`), nil
		}}
		app := setupDeviceTestApp(svc)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/60cf84f22290/configuration/push-direct", strings.NewReader(`{"configuration":{"radios":{"band":"5G","counter":9007199254740993}},"when":9}`))
		req.Header.Set("Authorization", "Bearer jwt")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || called != 1 {
			t.Fatalf("expected status 200 and one service call, got status=%d calls=%d", resp.StatusCode, called)
		}
	})

	for _, body := range []string{
		`{"configuration":{"radios":{}},"unexpected":true}`,
		`{"configuration":{"radios":{}}}{"configuration":{"radios":{}}}`,
		`not-json`,
	} {
		t.Run("rejects malformed or unknown fields", func(t *testing.T) {
			called := 0
			svc := &mockDeviceService{pushDirectConfigFn: func(context.Context, string, models.DirectDeviceConfigurationRequest, string, string, string) (json.RawMessage, error) {
				called++
				return nil, nil
			}}
			app := setupDeviceTestApp(svc)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/60cf84f22290/configuration/push-direct", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer jwt")
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest || called != 0 {
				t.Fatalf("expected status 400 and no service call, got status=%d calls=%d", resp.StatusCode, called)
			}
		})
	}
}

func TestDeviceHandler_AuthorizeDeviceCommand_MissingAuth(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"60cf84f22290","action":"reboot"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/authorization/deviceCommand", body)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_RebootDevice_MissingAuth(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_RebootDevice_InvalidBearer(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("Authorization", "Basic 12345")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_RebootDevice_EmptyBody(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty body, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_RebootDevice_MismatchedSerial(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"other-serial","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for mismatched serial, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_RebootDevice_NegativeWhen(t *testing.T) {
	svc := &mockDeviceService{}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":-5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative when, got %d", resp.StatusCode)
	}
}

func TestDeviceHandler_RebootDevice_Success(t *testing.T) {
	called := false
	svc := &mockDeviceService{
		executeDeviceCommandFn: func(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
			called = true
			if serial != "dc62796520cd" || command != "reboot" {
				t.Errorf("unexpected serial=%s or command=%s", serial, command)
			}
			if token != "Bearer valid-token" {
				t.Errorf("unexpected token: %s", token)
			}
			return json.RawMessage(`{"UUID":"cmd-1","serialNumber":"dc62796520cd","status":"completed","errorCode":0}`), nil
		},
	}
	app := setupDeviceTestApp(svc)

	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Error("expected executeDeviceCommandFn to be called")
	}
}
