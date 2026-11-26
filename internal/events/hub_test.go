package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/events"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type mockProvClientForHub struct {
	getInventorySerialsFn func(ctx context.Context, token, reqID, corrID string) ([]string, error)
}

func (m *mockProvClientForHub) GetPolicy(ctx context.Context, id, token, reqID, corrID string) (*models.ManagementPolicy, error) {
	return nil, nil
}
func (m *mockProvClientForHub) GetRolesByPolicy(ctx context.Context, policyID, token, reqID, corrID string) ([]models.ManagementRole, error) {
	return nil, nil
}
func (m *mockProvClientForHub) GetEntities(ctx context.Context, token, reqID, corrID string) ([]models.Entity, error) {
	return nil, nil
}
func (m *mockProvClientForHub) GetVenues(ctx context.Context, token, reqID, corrID string) ([]models.Venue, error) {
	return nil, nil
}
func (m *mockProvClientForHub) GetVenue(ctx context.Context, id, token, reqID, corrID string) (*models.VenueDetails, error) {
	return nil, nil
}
func (m *mockProvClientForHub) UpdateVenue(ctx context.Context, id string, body []byte, token, reqID, corrID string) error {
	return nil
}
func (m *mockProvClientForHub) GetInventory(ctx context.Context, token, reqID, corrID string, limit, offset int, unassigned bool) ([]models.InventoryTag, error) {
	return nil, nil
}
func (m *mockProvClientForHub) GetInventoryCount(ctx context.Context, token, reqID, corrID string, unassigned bool) (int, error) {
	return 0, nil
}
func (m *mockProvClientForHub) GetInventorySerials(ctx context.Context, token, reqID, corrID string) ([]string, error) {
	if m.getInventorySerialsFn != nil {
		return m.getInventorySerialsFn(ctx, token, reqID, corrID)
	}
	return nil, nil
}
func (m *mockProvClientForHub) GetInventoryItem(ctx context.Context, serial, token, reqID, corrID string) (*models.InventoryTag, error) {
	return nil, nil
}
func (m *mockProvClientForHub) DeleteInventoryItem(ctx context.Context, serial, token, reqID, corrID string) error {
	return nil
}
func (m *mockProvClientForHub) ApplyDeviceConfiguration(ctx context.Context, serial, token, reqID, corrID string) (json.RawMessage, error) {
	return json.RawMessage(`{"errorCode":0}`), nil
}
func (m *mockProvClientForHub) AuthorizeDeviceCommand(ctx context.Context, serial, action, token, reqID, corrID string) (*models.DeviceCommandAuthResult, error) {
	return &models.DeviceCommandAuthResult{Allowed: true}, nil
}

func TestHub_AuthorizeUser(t *testing.T) {
	mockProv := &mockProvClientForHub{
		getInventorySerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			if token != "valid-jwt" {
				return nil, models.NewApiError(401, "Unauthorized", "invalid token")
			}
			return []string{"24:F5:A2:01:B1:A0", "AA-BB-CC-DD-EE-FF"}, nil
		},
	}

	hub := events.NewHub(mockProv, nil)
	serials, err := hub.AuthorizeUser(context.Background(), "valid-jwt", "req-1", "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(serials) != 2 {
		t.Fatalf("expected 2 serials, got %d", len(serials))
	}
	if serials[0] != "24f5a201b1a0" || serials[1] != "aabbccddeeff" {
		t.Errorf("unexpected normalized serials: %v", serials)
	}
}

// TestHub_MultiTenantIsolation verifies that:
// 1. Client A (Tenant A) receives updates ONLY for Tenant A devices.
// 2. Client B (Tenant B) receives updates ONLY for Tenant B devices.
// 3. Client B CANNOT subscribe to Tenant A devices (privilege escalation prevention).
// 4. Events for unknown devices are received by NO clients.
func TestHub_MultiTenantIsolation(t *testing.T) {
	hub := events.NewHub(nil, nil)

	// User A has access to alpha1, alpha2
	sessionA := hub.CreateSession("token-A", []string{"alpha1", "alpha2"}, nil)
	hub.RegisterSession(sessionA)
	defer hub.UnregisterSession(sessionA.ID)

	// User B has access to beta1, beta2
	sessionB := hub.CreateSession("token-B", []string{"beta1", "beta2"}, nil)
	hub.RegisterSession(sessionB)
	defer hub.UnregisterSession(sessionB.ID)

	if hub.GetActiveSessionCount() != 2 {
		t.Fatalf("expected 2 active sessions, got %d", hub.GetActiveSessionCount())
	}

	// Verify initial index
	if hub.GetSubscribersForSerial("alpha1") != 1 {
		t.Errorf("expected 1 subscriber for alpha1, got %d", hub.GetSubscribersForSerial("alpha1"))
	}
	if hub.GetSubscribersForSerial("beta1") != 1 {
		t.Errorf("expected 1 subscriber for beta1, got %d", hub.GetSubscribersForSerial("beta1"))
	}

	// 1. Broadcast event for alpha1 (Tenant A device)
	connTrue := true
	hub.Broadcast(&events.DeviceEvent{
		Type:         "connection",
		SerialNumber: "alpha1",
		Connected:    &connTrue,
		Timestamp:    100,
	})

	// User A must receive alpha1
	select {
	case msg := <-sessionA.Send:
		var ev events.DeviceEvent
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if ev.SerialNumber != "alpha1" {
			t.Errorf("expected serial alpha1, got %s", ev.SerialNumber)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("sessionA timed out waiting for alpha1 event")
	}

	// User B must receive NOTHING
	select {
	case msg := <-sessionB.Send:
		t.Fatalf("SECURITY VIOLATION: User B received unauthorized event for alpha1: %s", string(msg))
	default:
		// expected
	}

	// 2. Broadcast event for beta1 (Tenant B device)
	hub.Broadcast(&events.DeviceEvent{
		Type:         "healthcheck",
		SerialNumber: "beta1",
		Timestamp:    200,
	})

	// User B must receive beta1
	select {
	case msg := <-sessionB.Send:
		var ev events.DeviceEvent
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if ev.SerialNumber != "beta1" {
			t.Errorf("expected serial beta1, got %s", ev.SerialNumber)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("sessionB timed out waiting for beta1 event")
	}

	// User A must receive NOTHING
	select {
	case msg := <-sessionA.Send:
		t.Fatalf("SECURITY VIOLATION: User A received unauthorized event for beta1: %s", string(msg))
	default:
		// expected
	}

	// 3. User B attempts PRIVILEGE ESCALATION: explicitly requests subscription to alpha1!
	granted, err := hub.UpdateSubscription(sessionB.ID, "token-B", []string{"alpha1", "beta1"})
	if err != nil {
		t.Fatalf("unexpected error updating subscription: %v", err)
	}
	// alpha1 must have been rejected because it's not in User B's allowedSerials!
	if granted != 1 {
		t.Errorf("expected only 1 granted subscription (beta1), got %d", granted)
	}

	// Broadcast alpha1 again
	hub.Broadcast(&events.DeviceEvent{
		Type:         "state",
		SerialNumber: "alpha1",
		Timestamp:    300,
	})

	// Drain User A
	<-sessionA.Send

	// Verify User B STILL received NOTHING
	select {
	case msg := <-sessionB.Send:
		t.Fatalf("SECURITY VIOLATION: User B successfully escalated privileges and received alpha1 event: %s", string(msg))
	default:
		// expected
	}

	// 4. Broadcast for unowned device gamma1
	hub.Broadcast(&events.DeviceEvent{
		Type:         "connection",
		SerialNumber: "gamma1",
		Timestamp:    400,
	})

	select {
	case msg := <-sessionA.Send:
		t.Fatalf("unexpected event on session A: %s", string(msg))
	default:
	}

	select {
	case msg := <-sessionB.Send:
		t.Fatalf("unexpected event on session B: %s", string(msg))
	default:
	}
}

func TestHub_UnregisterSession(t *testing.T) {
	hub := events.NewHub(nil, nil)

	session := hub.CreateSession("token-1", []string{"dev1"}, nil)
	hub.RegisterSession(session)

	if hub.GetActiveSessionCount() != 1 {
		t.Errorf("expected 1 session, got %d", hub.GetActiveSessionCount())
	}
	if hub.GetSubscribersForSerial("dev1") != 1 {
		t.Errorf("expected 1 subscriber for dev1, got %d", hub.GetSubscribersForSerial("dev1"))
	}

	hub.UnregisterSession(session.ID)

	if hub.GetActiveSessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", hub.GetActiveSessionCount())
	}
	if hub.GetSubscribersForSerial("dev1") != 0 {
		t.Errorf("expected 0 subscribers for dev1, got %d", hub.GetSubscribersForSerial("dev1"))
	}
	if !session.IsClosed() {
		t.Errorf("expected session to be closed")
	}
}

func TestHub_RefreshSessionAuthorization_PruneRevokedDevice(t *testing.T) {
	currentSerials := []string{"alpha1", "beta1"}
	mockProv := &mockProvClientForHub{
		getInventorySerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			return currentSerials, nil
		},
	}

	hub := events.NewHub(mockProv, nil)

	// User initially authorized and subscribed to alpha1 and beta1
	session := hub.CreateSession("token-A", currentSerials, []string{"alpha1", "beta1"})
	hub.RegisterSession(session)
	defer hub.UnregisterSession(session.ID)

	if hub.GetSubscribersForSerial("alpha1") != 1 || hub.GetSubscribersForSerial("beta1") != 1 {
		t.Fatalf("expected 1 subscriber each for alpha1 and beta1")
	}

	// Deliver alpha1 event: must receive
	connTrue := true
	hub.Broadcast(&events.DeviceEvent{
		Type:         "connection",
		SerialNumber: "alpha1",
		Connected:    &connTrue,
		Timestamp:    100,
	})
	select {
	case <-session.Send:
		// expected
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for alpha1 event")
	}

	// Now REVOKE access to alpha1 in OWPROV (e.g. venue access revoked)
	currentSerials = []string{"beta1"}

	// Refresh session authorization
	err := hub.RefreshSessionAuthorization(context.Background(), session.ID, "req-refresh", "corr-refresh")
	if err != nil {
		t.Fatalf("unexpected error refreshing authorization: %v", err)
	}

	// alpha1 must be pruned from inverted index
	if hub.GetSubscribersForSerial("alpha1") != 0 {
		t.Errorf("expected 0 subscribers for revoked alpha1, got %d", hub.GetSubscribersForSerial("alpha1"))
	}
	// beta1 must remain subscribed
	if hub.GetSubscribersForSerial("beta1") != 1 {
		t.Errorf("expected 1 subscriber for remaining beta1, got %d", hub.GetSubscribersForSerial("beta1"))
	}

	// Broadcast alpha1 again: must receive NOTHING
	hub.Broadcast(&events.DeviceEvent{
		Type:         "connection",
		SerialNumber: "alpha1",
		Connected:    &connTrue,
		Timestamp:    200,
	})
	select {
	case msg := <-session.Send:
		t.Fatalf("SECURITY VIOLATION: received event for revoked device alpha1: %s", string(msg))
	default:
		// expected: nothing received
	}

	// Broadcast beta1: must still receive
	hub.Broadcast(&events.DeviceEvent{
		Type:         "healthcheck",
		SerialNumber: "beta1",
		Timestamp:    300,
	})
	select {
	case <-session.Send:
		// expected
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for beta1 event")
	}
}

func TestHub_RefreshSessionAuthorization_AllRevoked_TerminatesSession(t *testing.T) {
	currentSerials := []string{"alpha1"}
	mockProv := &mockProvClientForHub{
		getInventorySerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			return currentSerials, nil
		},
	}

	hub := events.NewHub(mockProv, nil)
	session := hub.CreateSession("token-A", currentSerials, []string{"alpha1"})
	hub.RegisterSession(session)

	if hub.GetActiveSessionCount() != 1 {
		t.Fatalf("expected 1 active session")
	}

	// All device access revoked in OWPROV
	currentSerials = []string{}

	err := hub.RefreshSessionAuthorization(context.Background(), session.ID, "req-1", "corr-1")
	if err == nil {
		t.Fatalf("expected error when all access is revoked")
	}

	// Session must be closed and unregistered
	if !session.IsClosed() {
		t.Errorf("expected session to be closed")
	}
	if hub.GetActiveSessionCount() != 0 {
		t.Errorf("expected 0 active sessions after revocation, got %d", hub.GetActiveSessionCount())
	}
	if hub.GetSubscribersForSerial("alpha1") != 0 {
		t.Errorf("expected 0 subscribers for alpha1, got %d", hub.GetSubscribersForSerial("alpha1"))
	}
}

func TestHub_RefreshSessionAuthorization_AuthError_UnregistersSession(t *testing.T) {
	mockProv := &mockProvClientForHub{
		getInventorySerialsFn: func(ctx context.Context, token, reqID, corrID string) ([]string, error) {
			return nil, models.NewApiError(401, "Unauthorized", "token expired")
		},
	}

	hub := events.NewHub(mockProv, nil)
	session := hub.CreateSession("expired-token", []string{"alpha1"}, []string{"alpha1"})
	hub.RegisterSession(session)

	err := hub.RefreshSessionAuthorization(context.Background(), session.ID, "req-1", "corr-1")
	if err == nil {
		t.Fatalf("expected error on downstream 401")
	}

	if !session.IsClosed() {
		t.Errorf("expected session to be closed on auth failure")
	}
	if hub.GetActiveSessionCount() != 0 {
		t.Errorf("expected 0 active sessions, got %d", hub.GetActiveSessionCount())
	}
}

func TestClientSession_VerifyOwnership(t *testing.T) {
	const validToken = "278e32562c4cd14df17aabc9d6c41e1db9d47a181c443b13e4f15c5e89846284"
	const diffToken = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	tests := []struct {
		name         string
		sessionToken string
		callerToken  string
		expected     bool
	}{
		{
			name:         "matching tokens",
			sessionToken: validToken,
			callerToken:  validToken,
			expected:     true,
		},
		{
			name:         "matching tokens with Bearer prefix",
			sessionToken: "Bearer " + validToken,
			callerToken:  validToken,
			expected:     true,
		},
		{
			name:         "mismatched tokens",
			sessionToken: validToken,
			callerToken:  diffToken,
			expected:     false,
		},
		{
			name:         "empty caller token rejected",
			sessionToken: validToken,
			callerToken:  "",
			expected:     false,
		},
		{
			name:         "empty session token rejected",
			sessionToken: "",
			callerToken:  validToken,
			expected:     false,
		},
		{
			name:         "both tokens empty rejected",
			sessionToken: "",
			callerToken:  "",
			expected:     false,
		},
		{
			name:         "whitespace caller token rejected",
			sessionToken: validToken,
			callerToken:  "   ",
			expected:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &events.ClientSession{
				Token: tc.sessionToken,
			}
			actual := s.VerifyOwnership(tc.callerToken)
			if actual != tc.expected {
				t.Errorf("expected VerifyOwnership to return %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestHub_UpdateSubscription_OwnershipMismatch_Forbidden(t *testing.T) {
	hub := events.NewHub(nil, nil)
	session := hub.CreateSession("owner-token-1234", []string{"dev1", "dev2"}, nil)
	hub.RegisterSession(session)
	defer hub.UnregisterSession(session.ID)

	// Attacker tries to update owner's session subscriptions with mismatched token
	_, err := hub.UpdateSubscription(session.ID, "attacker-token-9999", []string{"dev1"})
	if err == nil {
		t.Fatalf("expected error when caller token does not match session owner")
	}
	if !errors.Is(err, events.ErrSessionOwnershipMismatch) {
		t.Errorf("expected ErrSessionOwnershipMismatch, got: %v", err)
	}

	// Owner with matching token succeeds
	granted, err := hub.UpdateSubscription(session.ID, "owner-token-1234", []string{"dev1"})
	if err != nil {
		t.Fatalf("unexpected error for legitimate owner: %v", err)
	}
	if granted != 1 {
		t.Errorf("expected 1 granted subscription, got %d", granted)
	}
}

func TestHub_UpdateSubscription_TokenImmutable(t *testing.T) {
	const initialToken = "initial-owsec-token-abc123"
	hub := events.NewHub(nil, nil)
	session := hub.CreateSession(initialToken, []string{"dev1", "dev2"}, nil)
	hub.RegisterSession(session)
	defer hub.UnregisterSession(session.ID)

	granted, err := hub.UpdateSubscription(session.ID, initialToken, []string{"dev1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if granted != 1 {
		t.Errorf("expected 1 granted, got %d", granted)
	}

	// Verify session token was not mutated or cleared
	if session.Token != initialToken {
		t.Errorf("expected session.Token to remain %q, got %q", initialToken, session.Token)
	}
}

func TestHub_UpdateSubscription_SessionNotFound(t *testing.T) {
	hub := events.NewHub(nil, nil)
	_, err := hub.UpdateSubscription("non-existent-id", "some-token", []string{"dev1"})
	if err == nil {
		t.Fatalf("expected error for non-existent session")
	}
	if !errors.Is(err, events.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestHub_UpdateSubscription_EmptySubscribesZero(t *testing.T) {
	hub := events.NewHub(nil, nil)
	session := hub.CreateSession("token-A", []string{"dev1", "dev2"}, nil)
	hub.RegisterSession(session)
	defer hub.UnregisterSession(session.ID)

	// Initially unrestricted: all allowed devices are indexed
	if !session.IsUnrestricted() {
		t.Errorf("expected session to be unrestricted initially")
	}
	if session.SubscribedCount() != 2 {
		t.Errorf("expected subscribedCount=2 initially, got %d", session.SubscribedCount())
	}
	if hub.GetSubscribersForSerial("dev1") != 1 || hub.GetSubscribersForSerial("dev2") != 1 {
		t.Errorf("expected 1 subscriber for each device initially")
	}

	// Explicitly empty subscription: must subscribe to exactly 0 devices
	granted, err := hub.UpdateSubscription(session.ID, "token-A", []string{})
	if err != nil {
		t.Fatalf("unexpected error updating subscription: %v", err)
	}
	if granted != 0 {
		t.Errorf("expected 0 granted subscriptions for empty list, got %d", granted)
	}
	if session.IsUnrestricted() {
		t.Errorf("expected session to be in filtered mode after UpdateSubscription")
	}
	if session.SubscribedCount() != 0 {
		t.Errorf("expected subscribedCount=0, got %d", session.SubscribedCount())
	}
	if hub.GetSubscribersForSerial("dev1") != 0 {
		t.Errorf("SECURITY/LEAK: expected 0 subscribers for dev1 after empty update, got %d", hub.GetSubscribersForSerial("dev1"))
	}
	if hub.GetSubscribersForSerial("dev2") != 0 {
		t.Errorf("SECURITY/LEAK: expected 0 subscribers for dev2 after empty update, got %d", hub.GetSubscribersForSerial("dev2"))
	}

	// Broadcast an event for dev1: session must receive NOTHING
	hub.Broadcast(&events.DeviceEvent{
		Type:         "state",
		SerialNumber: "dev1",
		Timestamp:    100,
	})

	select {
	case msg := <-session.Send:
		t.Fatalf("UNEXPECTED EVENT: session received event after empty subscription: %s", string(msg))
	case <-time.After(50 * time.Millisecond):
		// Expected: nothing received
	}
}

func TestHub_UpdateSubscription_WildcardRevertsToUnrestricted(t *testing.T) {
	hub := events.NewHub(nil, nil)
	session := hub.CreateSession("token-A", []string{"dev1", "dev2"}, []string{"dev1"})
	hub.RegisterSession(session)
	defer hub.UnregisterSession(session.ID)

	// Initially filtered to dev1
	if session.IsUnrestricted() {
		t.Errorf("expected session to be filtered initially")
	}
	if session.SubscribedCount() != 1 {
		t.Errorf("expected subscribedCount=1, got %d", session.SubscribedCount())
	}

	// Client sends wildcard "*" to re-subscribe to all allowed devices
	granted, err := hub.UpdateSubscription(session.ID, "token-A", []string{"*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if granted != 2 {
		t.Errorf("expected 2 granted for wildcard, got %d", granted)
	}
	if !session.IsUnrestricted() {
		t.Errorf("expected session to revert to unrestricted mode")
	}
	if session.SubscribedCount() != 2 {
		t.Errorf("expected subscribedCount=2, got %d", session.SubscribedCount())
	}
	if hub.GetSubscribersForSerial("dev1") != 1 || hub.GetSubscribersForSerial("dev2") != 1 {
		t.Errorf("expected both dev1 and dev2 to have 1 subscriber")
	}

	// Broadcast an event for dev2: session must receive it
	hub.Broadcast(&events.DeviceEvent{
		Type:         "state",
		SerialNumber: "dev2",
		Timestamp:    200,
	})

	select {
	case <-session.Send:
		// Expected
	case <-time.After(50 * time.Millisecond):
		t.Errorf("session timed out waiting for dev2 event in unrestricted mode")
	}
}

func TestHub_UpdateSubscription_NoGhostIndexes(t *testing.T) {
	hub := events.NewHub(nil, nil)
	session := hub.CreateSession("token-A", []string{"dev1", "dev2"}, nil)
	hub.RegisterSession(session)
	defer hub.UnregisterSession(session.ID)

	// 1. Initial unrestricted: dev1=1, dev2=1
	if hub.GetSubscribersForSerial("dev1") != 1 || hub.GetSubscribersForSerial("dev2") != 1 {
		t.Fatalf("step 1 failed")
	}

	// 2. Filter to dev1: dev1=1, dev2=0 (no ghost index on dev2)
	hub.UpdateSubscription(session.ID, "token-A", []string{"dev1"})
	if hub.GetSubscribersForSerial("dev1") != 1 || hub.GetSubscribersForSerial("dev2") != 0 {
		t.Fatalf("step 2 failed: dev1=%d, dev2=%d", hub.GetSubscribersForSerial("dev1"), hub.GetSubscribersForSerial("dev2"))
	}

	// 3. Filter to empty: dev1=0, dev2=0 (no ghost index on dev1)
	hub.UpdateSubscription(session.ID, "token-A", []string{})
	if hub.GetSubscribersForSerial("dev1") != 0 || hub.GetSubscribersForSerial("dev2") != 0 {
		t.Fatalf("step 3 failed: dev1=%d, dev2=%d", hub.GetSubscribersForSerial("dev1"), hub.GetSubscribersForSerial("dev2"))
	}

	// 4. Filter to dev2: dev1=0, dev2=1 (no ghost index on dev1)
	hub.UpdateSubscription(session.ID, "token-A", []string{"dev2"})
	if hub.GetSubscribersForSerial("dev1") != 0 || hub.GetSubscribersForSerial("dev2") != 1 {
		t.Fatalf("step 4 failed: dev1=%d, dev2=%d", hub.GetSubscribersForSerial("dev1"), hub.GetSubscribersForSerial("dev2"))
	}

	// 5. Unregister: dev1=0, dev2=0
	hub.UnregisterSession(session.ID)
	if hub.GetSubscribersForSerial("dev1") != 0 || hub.GetSubscribersForSerial("dev2") != 0 {
		t.Fatalf("step 5 failed: dev1=%d, dev2=%d", hub.GetSubscribersForSerial("dev1"), hub.GetSubscribersForSerial("dev2"))
	}
}

func TestHub_ConcurrentBroadcastAndClose_NoPanic(t *testing.T) {
	hub := events.NewHub(nil, nil)
	serial := "test-device-race"

	const numSessions = 30
	const numEvents = 300

	sessions := make([]*events.ClientSession, numSessions)
	for i := 0; i < numSessions; i++ {
		sess := hub.CreateSession(fmt.Sprintf("token-%d", i), []string{serial}, nil)
		hub.RegisterSession(sess)
		sessions[i] = sess
	}

	var wg sync.WaitGroup

	// Goroutines continuously broadcasting device events to the subscribed serial
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numEvents; j++ {
				hub.Broadcast(&events.DeviceEvent{
					Type:         "state",
					SerialNumber: serial,
					Timestamp:    time.Now().Unix(),
				})
				time.Sleep(50 * time.Microsecond)
			}
		}(i)
	}

	// Goroutines closing sessions and unregistering them concurrently with broadcast
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			time.Sleep(time.Duration((idx%10)+1) * time.Millisecond)
			// Trigger direct session Close() (simulating token expiry / client disconnect)
			sessions[idx].Close()
			// Then unregister from hub
			hub.UnregisterSession(sessions[idx].ID)
		}(i)
	}

	wg.Wait()
}

func TestClientSession_TrySend(t *testing.T) {
	hub := events.NewHub(nil, nil)
	session := hub.CreateSession("token-test", []string{"dev1"}, nil)

	// 1. Successful delivery to empty buffer
	delivered, closed := session.TrySend([]byte(`{"test":1}`))
	if !delivered || closed {
		t.Fatalf("expected delivered=true, closed=false; got delivered=%v, closed=%v", delivered, closed)
	}

	// Read message out
	select {
	case msg := <-session.Send:
		if string(msg) != `{"test":1}` {
			t.Errorf("unexpected message: %s", string(msg))
		}
	default:
		t.Fatalf("expected message in session.Send")
	}

	// 2. Buffer full condition: fill up buffer (capacity is 256)
	for i := 0; i < cap(session.Send); i++ {
		del, cls := session.TrySend([]byte("item"))
		if !del || cls {
			t.Fatalf("failed filling buffer at index %d: del=%v, cls=%v", i, del, cls)
		}
	}
	// Next TrySend should hit default (buffer full)
	del, cls := session.TrySend([]byte("overflow"))
	if del || cls {
		t.Fatalf("expected delivered=false, closed=false on full buffer; got del=%v, cls=%v", del, cls)
	}

	// 3. Closed session condition
	session.Close()
	if !session.IsClosed() {
		t.Fatalf("expected session.IsClosed() == true")
	}

	del, cls = session.TrySend([]byte("after-close"))
	if del || !cls {
		t.Fatalf("expected delivered=false, closed=true on closed session; got del=%v, cls=%v", del, cls)
	}
}
