package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/events"
	"github.com/routerarchitects/mango-mdu-service/internal/http/handlers"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
	"golang.org/x/net/websocket"
)

type mockProvForEventsHandler struct {
	getSerialsFn func(ctx context.Context, token, reqID, corrID string) ([]string, error)
}

func (m *mockProvForEventsHandler) GetPolicy(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
	return nil, nil
}
func (m *mockProvForEventsHandler) GetRolesByPolicy(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error) {
	return nil, nil
}
func (m *mockProvForEventsHandler) GetEntities(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
	return nil, nil
}
func (m *mockProvForEventsHandler) GetVenues(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
	return nil, nil
}
func (m *mockProvForEventsHandler) GetVenue(ctx context.Context, id, token, reqID, corrID string) (*models.VenueDetails, error) {
	return nil, nil
}
func (m *mockProvForEventsHandler) UpdateVenue(ctx context.Context, id string, body []byte, token, reqID, corrID string) error {
	return nil
}
func (m *mockProvForEventsHandler) GetInventory(ctx context.Context, token, reqID, corrID string, limit, offset int, unassigned bool) ([]models.InventoryTag, error) {
	return nil, nil
}
func (m *mockProvForEventsHandler) GetInventoryCount(ctx context.Context, token, reqID, corrID string, unassigned bool) (int, error) {
	return 0, nil
}
func (m *mockProvForEventsHandler) GetInventorySerials(ctx context.Context, token, reqID, corrID string) ([]string, error) {
	if m.getSerialsFn != nil {
		return m.getSerialsFn(ctx, token, reqID, corrID)
	}
	return nil, nil
}
func (m *mockProvForEventsHandler) GetInventoryItem(ctx context.Context, serial, token, reqID, corrID string) (*models.InventoryTag, error) {
	return nil, nil
}
func (m *mockProvForEventsHandler) DeleteInventoryItem(ctx context.Context, serial, token, reqID, corrID string) error {
	return nil
}
func (m *mockProvForEventsHandler) ApplyDeviceConfiguration(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	return json.RawMessage(`{"errorCode":0}`), nil
}
func (m *mockProvForEventsHandler) AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
	return &models.DeviceCommandAuthResult{Allowed: true}, nil
}

type mockTokenValidator struct {
	validateFn func(ctx context.Context, rawToken string) error
}

func (m *mockTokenValidator) ValidateToken(ctx context.Context, rawToken string) error {
	if m.validateFn != nil {
		return m.validateFn(ctx, rawToken)
	}
	return nil
}

func setupWSServer(t *testing.T, handler *handlers.DeviceEventsHandler) (*httptest.Server, string) {
	s := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		handler.ServeWebSocket(ws, "test-req-id", "test-corr-id")
	}))
	wsURL := "ws://" + s.Listener.Addr().String()
	return s, wsURL
}

func TestWebSocket_InvalidAuthFrameFormat(t *testing.T) {
	hub := events.NewHub(nil, nil)
	handler := handlers.NewDeviceEventsHandler(hub, nil)
	server, wsURL := setupWSServer(t, handler)
	defer server.Close()

	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer ws.Close()

	// Send frame without "token:" prefix
	if err := websocket.Message.Send(ws, "some-random-data"); err != nil {
		t.Fatalf("failed to send frame: %v", err)
	}

	var response string
	if err := websocket.Message.Receive(ws, &response); err != nil {
		t.Fatalf("failed to receive error frame: %v", err)
	}

	if response != `{"error":"Invalid token format. Closing connection."}` {
		t.Errorf("unexpected response: %s", response)
	}

	// Verify server closes connection
	var next string
	if err := websocket.Message.Receive(ws, &next); err == nil {
		t.Errorf("expected connection to be closed, but received: %s", next)
	}
}

func TestWebSocket_TokenValidationFailure(t *testing.T) {
	validator := &mockTokenValidator{
		validateFn: func(ctx context.Context, rawToken string) error {
			return errors.New("revoked token")
		},
	}
	hub := events.NewHub(nil, nil)
	handler := handlers.NewDeviceEventsHandler(hub, validator)
	server, wsURL := setupWSServer(t, handler)
	defer server.Close()

	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer ws.Close()

	if err := websocket.Message.Send(ws, "token:revoked-token"); err != nil {
		t.Fatalf("failed to send frame: %v", err)
	}

	var response string
	if err := websocket.Message.Receive(ws, &response); err != nil {
		t.Fatalf("failed to receive error frame: %v", err)
	}

	if response != `{"error":"Invalid token. Closing connection."}` {
		t.Errorf("unexpected response: %s", response)
	}
}

func TestWebSocket_InventoryAuthorizationFailure(t *testing.T) {
	mockProv := &mockProvForEventsHandler{
		getSerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			return nil, errors.New("owprov down")
		},
	}
	hub := events.NewHub(mockProv, nil)
	handler := handlers.NewDeviceEventsHandler(hub, nil)
	server, wsURL := setupWSServer(t, handler)
	defer server.Close()

	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer ws.Close()

	if err := websocket.Message.Send(ws, "token:valid-token"); err != nil {
		t.Fatalf("failed to send frame: %v", err)
	}

	var response string
	if err := websocket.Message.Receive(ws, &response); err != nil {
		t.Fatalf("failed to receive error frame: %v", err)
	}

	if response != `{"error":"Failed to authorize inventory. Closing connection."}` {
		t.Errorf("unexpected response: %s", response)
	}
}

func TestWebSocket_AuthenticationAndWelcomeHandshake(t *testing.T) {
	mockProv := &mockProvForEventsHandler{
		getSerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			return []string{"24f5a201b1a0", "24f5a201b1a1"}, nil
		},
	}
	hub := events.NewHub(mockProv, nil)
	handler := handlers.NewDeviceEventsHandler(hub, nil)
	server, wsURL := setupWSServer(t, handler)
	defer server.Close()

	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer ws.Close()

	if err := websocket.Message.Send(ws, "token:my-owsec-token"); err != nil {
		t.Fatalf("failed to send token: %v", err)
	}

	var welcome string
	if err := websocket.Message.Receive(ws, &welcome); err != nil {
		t.Fatalf("failed to receive welcome frame: %v", err)
	}

	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(welcome), &msg); err != nil {
		t.Fatalf("failed to parse welcome JSON: %v", err)
	}

	if msg["success"] != "Welcome! Bienvenue! Bienvenidos!" {
		t.Errorf("unexpected welcome message: %v", msg["success"])
	}
	if msg["allowedCount"] != float64(2) {
		t.Errorf("expected allowedCount=2, got %v", msg["allowedCount"])
	}
	if msg["sessionId"] == "" {
		t.Errorf("expected valid non-empty sessionId")
	}
}

func TestWebSocket_InBandSubscriptionUpdate(t *testing.T) {
	mockProv := &mockProvForEventsHandler{
		getSerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			return []string{"24f5a201b1a0", "24f5a201b1a1"}, nil
		},
	}
	hub := events.NewHub(mockProv, nil)
	handler := handlers.NewDeviceEventsHandler(hub, nil)
	server, wsURL := setupWSServer(t, handler)
	defer server.Close()

	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer ws.Close()

	// Authenticate
	if err := websocket.Message.Send(ws, "token:my-owsec-token"); err != nil {
		t.Fatalf("failed to send token: %v", err)
	}

	var welcome string
	if err := websocket.Message.Receive(ws, &welcome); err != nil {
		t.Fatalf("failed to receive welcome frame: %v", err)
	}

	// Update subscription to 1 serial
	subReq, _ := json.Marshal(map[string]interface{}{
		"serials": []string{"24f5a201b1a0"},
	})
	if err := websocket.Message.Send(ws, string(subReq)); err != nil {
		t.Fatalf("failed to send subscription: %v", err)
	}

	var ackStr string
	if err := websocket.Message.Receive(ws, &ackStr); err != nil {
		t.Fatalf("failed to receive subscription ack: %v", err)
	}

	var ack map[string]interface{}
	if err := json.Unmarshal([]byte(ackStr), &ack); err != nil {
		t.Fatalf("failed to unmarshal ack JSON: %v", err)
	}

	if ack["type"] != "subscription_ack" {
		t.Errorf("expected type=subscription_ack, got %v", ack["type"])
	}
	if ack["subscribedCount"] != float64(1) {
		t.Errorf("expected subscribedCount=1, got %v", ack["subscribedCount"])
	}

	// Update subscription to empty list
	emptyReq, _ := json.Marshal(map[string]interface{}{
		"serials": []string{},
	})
	if err := websocket.Message.Send(ws, string(emptyReq)); err != nil {
		t.Fatalf("failed to send empty subscription: %v", err)
	}

	if err := websocket.Message.Receive(ws, &ackStr); err != nil {
		t.Fatalf("failed to receive empty subscription ack: %v", err)
	}

	if err := json.Unmarshal([]byte(ackStr), &ack); err != nil {
		t.Fatalf("failed to unmarshal empty ack JSON: %v", err)
	}
	if ack["subscribedCount"] != float64(0) {
		t.Errorf("expected subscribedCount=0, got %v", ack["subscribedCount"])
	}
}

func TestWebSocket_OutboundEventDelivery(t *testing.T) {
	mockProv := &mockProvForEventsHandler{
		getSerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			return []string{"24f5a201b1a0"}, nil
		},
	}
	hub := events.NewHub(mockProv, nil)
	handler := handlers.NewDeviceEventsHandler(hub, nil)
	server, wsURL := setupWSServer(t, handler)
	defer server.Close()

	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer ws.Close()

	// Authenticate
	if err := websocket.Message.Send(ws, "token:my-owsec-token"); err != nil {
		t.Fatalf("failed to send token: %v", err)
	}

	var welcome string
	if err := websocket.Message.Receive(ws, &welcome); err != nil {
		t.Fatalf("failed to receive welcome frame: %v", err)
	}

	// Subscribe to the serial
	subReq, _ := json.Marshal(map[string]interface{}{
		"serials": []string{"24f5a201b1a0"},
	})
	if err := websocket.Message.Send(ws, string(subReq)); err != nil {
		t.Fatalf("failed to send subscription: %v", err)
	}

	var ackStr string
	if err := websocket.Message.Receive(ws, &ackStr); err != nil {
		t.Fatalf("failed to receive subscription ack: %v", err)
	}

	// Broadcast event from Hub
	connTrue := true
	hub.Broadcast(&events.DeviceEvent{
		SerialNumber: "24f5a201b1a0",
		Connected:    &connTrue,
		ConnectionIP: "192.168.1.50",
		Firmware:     "OpenWrt-21.02",
		Timestamp:    1729850000,
	})

	var eventStr string
	if err := websocket.Message.Receive(ws, &eventStr); err != nil {
		t.Fatalf("failed to receive broadcast event: %v", err)
	}

	var ev map[string]interface{}
	if err := json.Unmarshal([]byte(eventStr), &ev); err != nil {
		t.Fatalf("failed to unmarshal event JSON: %v", err)
	}

	if ev["serialNumber"] != "24f5a201b1a0" {
		t.Errorf("expected serialNumber 24f5a201b1a0, got %v", ev["serialNumber"])
	}
	if ev["connected"] != true {
		t.Errorf("expected connected=true, got %v", ev["connected"])
	}
	if ev["connectionIp"] != "192.168.1.50" {
		t.Errorf("expected connectionIp 192.168.1.50, got %v", ev["connectionIp"])
	}
}

func TestWebSocketHandler_Registered(t *testing.T) {
	app := fiber.New()
	hub := events.NewHub(nil, nil)
	handler := handlers.NewDeviceEventsHandler(hub, nil)
	app.Get("/api/v1/ws", handler.HandleWebSocket)

	routes := app.GetRoutes()
	hasWs := false
	for _, r := range routes {
		if r.Path == "/api/v1/ws" && r.Method == "GET" {
			hasWs = true
			break
		}
	}
	if !hasWs {
		t.Errorf("expected /api/v1/ws GET route to be registered on app")
	}
}

func TestDeviceEventsHandler_WriteTimeoutConfiguration(t *testing.T) {
	handler := handlers.NewDeviceEventsHandler(nil, nil)
	if handler.WriteTimeout() != 10*time.Second {
		t.Errorf("expected default write timeout 10s, got %v", handler.WriteTimeout())
	}

	handler.SetWriteTimeout(5 * time.Second)
	if handler.WriteTimeout() != 5*time.Second {
		t.Errorf("expected write timeout 5s, got %v", handler.WriteTimeout())
	}

	// Non-positive timeout should be ignored
	handler.SetWriteTimeout(0)
	if handler.WriteTimeout() != 5*time.Second {
		t.Errorf("expected write timeout to remain 5s, got %v", handler.WriteTimeout())
	}
}
