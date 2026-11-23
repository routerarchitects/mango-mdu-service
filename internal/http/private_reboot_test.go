package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/config"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owgw"
	"github.com/routerarchitects/mango-mdu-service/internal/gateway/owprov"
	apphttp "github.com/routerarchitects/mango-mdu-service/internal/http"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/http/middleware"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	subsystemroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

// mockProvClient implements owprov.Client for testing
type mockProvClient struct {
	owprov.Client
	authorizeFn func(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error)
}

func (m *mockProvClient) AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
	if m.authorizeFn != nil {
		return m.authorizeFn(ctx, serial, action, token, reqID, corrID)
	}
	return &models.DeviceCommandAuthResult{
		Allowed:      true,
		Resource:     "deviceCommand",
		Action:       action,
		SerialNumber: serial,
	}, nil
}

// mockGWClient implements owgw.Client for testing
type mockGWClient struct {
	owgw.Client
	executeCmdFn func(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error)
}

func (m *mockGWClient) ExecuteDeviceCommand(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
	if m.executeCmdFn != nil {
		return m.executeCmdFn(ctx, serial, command, body, token, reqID, corrID)
	}
	return json.RawMessage(fmt.Sprintf(`{"UUID":"cmd-123","serialNumber":"%s","command":"%s","status":"completed","errorCode":0}`, serial, command)), nil
}

type testHarness struct {
	publicApp    *fiber.App
	privateApp   *fiber.App
	publicURL    string
	secret       string
	derivedKey   string
	mockProv     *mockProvClient
	mockGW       *mockGWClient
	deviceSvc    services.DeviceService
	logBuf       *bytes.Buffer
}

func newTestHarness(t *testing.T, secret, publicURL string) *testHarness {
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockProv := &mockProvClient{}
	mockGW := &mockGWClient{}
	deviceSvc := services.NewDeviceService(mockProv, mockGW, logger)
	deviceHdlr := handlers.NewDeviceHandler(deviceSvc, logger)

	derivedKey := middleware.DeriveKey(publicURL, secret)

	derivedAuthConfig := middleware.DerivedInternalAPIKeyConfig{
		PublicEndpoint:   publicURL,
		ConfiguredSecret: secret,
		DiscoveryKey:     derivedKey,
	}

	module, err := apphttp.NewModule(apphttp.Dependencies{
		ServerLogger: logger,
		ServerConfig: config.ServerConfig{
			HTTPPort:    16010,
			PrivatePort: 17010,
		},
		SubsystemConfig: subsystemroutes.Config{},
		PublicAuthConfig: auth.PublicAuthConfig{
			Validator: nil, // unauthenticated for public test app pass-through unless enabled
		},
		PrivateAuthConfig: auth.InternalAPIKeyConfig{
			ExpectedAPIKey: derivedKey,
		},
		DerivedAuthConfig: derivedAuthConfig,
		AuthEnabled:       false,
		DeviceHandler:     deviceHdlr,
	})
	if err != nil {
		t.Fatalf("failed to create apphttp module: %v", err)
	}

	return &testHarness{
		publicApp:  module.PublicApp(),
		privateApp: module.PrivateApp(),
		publicURL:  publicURL,
		secret:     secret,
		derivedKey: derivedKey,
		mockProv:   mockProv,
		mockGW:     mockGW,
		deviceSvc:  deviceSvc,
		logBuf:     logBuf,
	}
}

// 1. Successful private reboot request with valid credentials and authorized user.
func TestPrivateReboot_1_Success(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	gwCalled := false
	h.mockGW.executeCmdFn = func(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
		gwCalled = true
		if serial != "dc62796520cd" || command != "reboot" {
			t.Errorf("unexpected serial=%s or command=%s", serial, command)
		}
		if token != "Bearer test-user-token" {
			t.Errorf("unexpected token: %s", token)
		}
		return json.RawMessage(`{"UUID":"cmd-reboot-1","serialNumber":"dc62796520cd","command":"reboot","status":"completed","errorCode":0}`), nil
	}

	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("X-API-KEY", "changeme")
	req.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req.Header.Set("Authorization", "Bearer test-user-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.privateApp.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d, body: %s", resp.StatusCode, string(respBody))
	}
	if !gwCalled {
		t.Error("expected downstream OWGW ExecuteDeviceCommand to be called")
	}
}

// 2. Missing or incorrect X-API-KEY.
func TestPrivateReboot_2_MissingOrIncorrectAPIKey(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	// Missing X-API-KEY
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req1.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req1.Header.Set("Authorization", "Bearer token")
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := h.privateApp.Test(req1)
	if err != nil || resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing X-API-KEY, got status=%d err=%v", resp1.StatusCode, err)
	}

	// Incorrect X-API-KEY
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req2.Header.Set("X-API-KEY", "wrong-secret")
	req2.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req2.Header.Set("Authorization", "Bearer token")
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := h.privateApp.Test(req2)
	if err != nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for incorrect X-API-KEY, got status=%d err=%v", resp2.StatusCode, err)
	}
}

// 3. Missing or invalid X-INTERNAL-NAME.
func TestPrivateReboot_3_MissingOrInvalidInternalName(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	// Missing X-INTERNAL-NAME
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req1.Header.Set("X-API-KEY", "changeme")
	req1.Header.Set("Authorization", "Bearer token")
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := h.privateApp.Test(req1)
	if err != nil || resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing X-INTERNAL-NAME, got status=%d err=%v", resp1.StatusCode, err)
	}

	// Empty / whitespace X-INTERNAL-NAME
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req2.Header.Set("X-API-KEY", "changeme")
	req2.Header.Set("X-INTERNAL-NAME", "   ")
	req2.Header.Set("Authorization", "Bearer token")
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := h.privateApp.Test(req2)
	if err != nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty X-INTERNAL-NAME, got status=%d err=%v", resp2.StatusCode, err)
	}
}

// 4. Missing or invalid bearer token.
func TestPrivateReboot_4_MissingOrInvalidBearerToken(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	// Missing Authorization header
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req1.Header.Set("X-API-KEY", "changeme")
	req1.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := h.privateApp.Test(req1)
	if err != nil || resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing Authorization, got status=%d err=%v", resp1.StatusCode, err)
	}

	// Invalid token format (e.g. Basic instead of Bearer)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req2.Header.Set("X-API-KEY", "changeme")
	req2.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req2.Header.Set("Authorization", "Basic 123456")
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := h.privateApp.Test(req2)
	if err != nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid Bearer format, got status=%d err=%v", resp2.StatusCode, err)
	}
}

// 5. Valid internal key but insufficient device permissions (RBAC denied).
func TestPrivateReboot_5_InsufficientPermissions(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	// Configure mock OWPROV to deny permission
	h.mockProv.authorizeFn = func(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
		return &models.DeviceCommandAuthResult{
			Allowed:      false,
			Resource:     "deviceCommand",
			Action:       "REBOOT",
			SerialNumber: serial,
			Reason:       "DENIED",
			Details:      "User is not authorized to execute command 'reboot' on device 'dc62796520cd'",
		}, nil
	}

	gwCalled := false
	h.mockGW.executeCmdFn = func(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
		gwCalled = true
		return nil, nil
	}

	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("X-API-KEY", "changeme")
	req.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req.Header.Set("Authorization", "Bearer limited-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.privateApp.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden, got %d", resp.StatusCode)
	}
	if gwCalled {
		t.Error("OWGW reboot MUST NOT be called when OWPROV RBAC denies permission")
	}
}

// 6. Body serial number differing from the URL serial number.
func TestPrivateReboot_6_BodySerialMismatch(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	body := strings.NewReader(`{"serialNumber":"different-serial","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("X-API-KEY", "changeme")
	req.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.privateApp.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for serial mismatch, got %d", resp.StatusCode)
	}
}

// 7. Invalid reboot payload or scheduling values.
func TestPrivateReboot_7_InvalidPayloadOrScheduling(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	// Negative 'when' value
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":-10}`))
	req1.Header.Set("X-API-KEY", "changeme")
	req1.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req1.Header.Set("Authorization", "Bearer valid-token")
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := h.privateApp.Test(req1)
	if err != nil || resp1.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for negative 'when', got status=%d err=%v", resp1.StatusCode, err)
	}

	// Malformed JSON body
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{malformed json`))
	req2.Header.Set("X-API-KEY", "changeme")
	req2.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req2.Header.Set("Authorization", "Bearer valid-token")
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := h.privateApp.Test(req2)
	if err != nil || resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed json, got status=%d err=%v", resp2.StatusCode, err)
	}
}

// 8. SHA-256 derived-key consistency between discovery and authentication.
func TestPrivateReboot_8_DiscoveryKeyConsistency(t *testing.T) {
	publicURL := "https://openwifi.wlan.local:16010"
	secret := "changeme"
	expectedDerivedKey := middleware.DeriveKey(publicURL, secret)

	h := newTestHarness(t, secret, publicURL)

	// Call using the derived key directly as X-API-KEY
	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("X-API-KEY", expectedDerivedKey)
	req.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.privateApp.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 when authenticating with derived discovery key, got %d", resp.StatusCode)
	}
}

// 9. Rejection when the secret or public endpoint produces a different derived key.
func TestPrivateReboot_9_DifferentDerivedKeyRejection(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	// Key derived with a different secret
	keyOtherSecret := middleware.DeriveKey("https://openwifi.wlan.local:16010", "other-secret")
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req1.Header.Set("X-API-KEY", keyOtherSecret)
	req1.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req1.Header.Set("Authorization", "Bearer valid-token")
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := h.privateApp.Test(req1)
	if err != nil || resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 when key was derived from different secret, got status=%d err=%v", resp1.StatusCode, err)
	}

	// Key derived with a different public endpoint
	keyOtherEndpoint := middleware.DeriveKey("https://different-host.wlan.local:16010", "changeme")
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`))
	req2.Header.Set("X-API-KEY", keyOtherEndpoint)
	req2.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req2.Header.Set("Authorization", "Bearer valid-token")
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := h.privateApp.Test(req2)
	if err != nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 when key was derived from different endpoint, got status=%d err=%v", resp2.StatusCode, err)
	}
}

// 10. Existing public reboot API remains functional.
func TestPrivateReboot_10_PublicRebootFunctional(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	publicGWCalled := false
	h.mockGW.executeCmdFn = func(ctx context.Context, serial, command string, body []byte, token, reqID, corrID string) (json.RawMessage, error) {
		publicGWCalled = true
		return json.RawMessage(`{"UUID":"cmd-public-1","serialNumber":"dc62796520cd","command":"reboot","status":"completed","errorCode":0}`), nil
	}

	// Hit public app on POST /api/v1/devices/:serial/reboot
	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("Authorization", "Bearer user-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.publicApp.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 on public reboot endpoint, got %d", resp.StatusCode)
	}
	if !publicGWCalled {
		t.Error("expected downstream OWGW to be called via public endpoint")
	}
}

// 11. Other private routes remain unaffected.
func TestPrivateReboot_11_OtherPrivateRoutesUnaffected(t *testing.T) {
	h := newTestHarness(t, "changeme", "https://openwifi.wlan.local:16010")

	// /livez is unauthenticated
	reqLivez := httptest.NewRequest(http.MethodGet, "/livez", nil)
	respLivez, err := h.privateApp.Test(reqLivez)
	if err != nil || respLivez.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /livez, got status=%d err=%v", respLivez.StatusCode, err)
	}

	// /api/v1/system requires internal authentication (X-API-KEY + X-INTERNAL-NAME)
	reqSysUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/system?command=info", nil)
	respSysUnauth, err := h.privateApp.Test(reqSysUnauth)
	if err != nil || respSysUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated /api/v1/system, got status=%d err=%v", respSysUnauth.StatusCode, err)
	}

	reqSysAuth := httptest.NewRequest(http.MethodGet, "/api/v1/system?command=info", nil)
	reqSysAuth.Header.Set("X-API-KEY", "changeme")
	reqSysAuth.Header.Set("X-INTERNAL-NAME", "mcp-server")
	respSysAuth, err := h.privateApp.Test(reqSysAuth)
	if err != nil || respSysAuth.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for authenticated /api/v1/system, got status=%d err=%v", respSysAuth.StatusCode, err)
	}
}

// 12. No plaintext secrets exposed in logs or discovery messages.
func TestPrivateReboot_12_NoPlaintextSecretsInLogsOrDiscovery(t *testing.T) {
	secret := "super-sensitive-secret-changeme"
	publicURL := "https://openwifi.wlan.local:16010"

	h := newTestHarness(t, secret, publicURL)

	// Perform a private reboot call
	body := strings.NewReader(`{"serialNumber":"dc62796520cd","when":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dc62796520cd/reboot", body)
	req.Header.Set("X-API-KEY", secret)
	req.Header.Set("X-INTERNAL-NAME", "mcp-server")
	req.Header.Set("Authorization", "Bearer sensitive-bearer-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.privateApp.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	logs := h.logBuf.String()

	// Verify plaintext secret and bearer token never appear in log buffer
	if strings.Contains(logs, secret) {
		t.Errorf("SECURITY VULNERABILITY: Plaintext secret leaked in logs!")
	}
	if strings.Contains(logs, "sensitive-bearer-token") {
		t.Errorf("SECURITY VULNERABILITY: Bearer token leaked in logs!")
	}

	// Verify discovery key derivation is SHA-256 and does not match the secret
	derivedKey := middleware.DeriveKey(publicURL, secret)
	if derivedKey == secret {
		t.Error("derived discovery key must not equal the raw secret")
	}
	if len(derivedKey) != 64 {
		t.Errorf("expected SHA-256 hex string of 64 characters, got %d", len(derivedKey))
	}
}
