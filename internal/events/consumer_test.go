package events_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/events"
	"github.com/routerarchitects/ra-common-mods/kafka"
)

type mockKafkaConsumer struct {
	mu         sync.Mutex
	subscribed bool
	closed     bool
	topics     []string
	handler    kafka.Handler
	stopChan   chan struct{}
}

func newMockKafkaConsumer() *mockKafkaConsumer {
	return &mockKafkaConsumer{
		stopChan: make(chan struct{}),
	}
}

func (m *mockKafkaConsumer) SubscribeMultiple(ctx context.Context, topics []string, handler kafka.Handler, opts *kafka.SubscribeOptions) error {
	m.mu.Lock()
	m.subscribed = true
	m.topics = topics
	m.handler = handler
	m.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.stopChan:
		return nil
	}
}

func (m *mockKafkaConsumer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.stopChan)
	}
	return nil
}

func TestKafkaEventConsumer_ProcessMessage_BroadcastsToHub(t *testing.T) {
	hub := events.NewHub(nil, nil)
	session := hub.CreateSession("token", []string{"test-dev-1"}, nil)
	hub.RegisterSession(session)
	defer hub.UnregisterSession(session.ID)

	kc := events.NewKafkaConsumer(kafka.Config{}, hub, nil)
	callbackEvents := make(chan *events.DeviceEvent, 1)
	kc.AddCallback(func(event *events.DeviceEvent) { callbackEvents <- event })

	payload := `{
		"system": {"id": 1, "host": "owgw"},
		"payload": {
			"serialNumber": "test-dev-1",
			"connectionIp": "10.0.0.1",
			"firmware": "v1.0"
		}
	}`

	err := kc.ProcessMessage("connection", []byte("test-dev-1"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error processing message: %v", err)
	}

	select {
	case msg := <-session.Send:
		var ev events.DeviceEvent
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if ev.SerialNumber != "testdev1" {
			t.Errorf("expected normalized serial testdev1, got %s", ev.SerialNumber)
		}
		if ev.ConnectionIP != "10.0.0.1" {
			t.Errorf("expected connection IP 10.0.0.1, got %s", ev.ConnectionIP)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timed out waiting for event on session channel")
	}

	select {
	case event := <-callbackEvents:
		if event.SerialNumber != "testdev1" || event.Type != "connection" {
			t.Fatalf("callback received unexpected event: %+v", event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for event callback")
	}
}

func TestKafkaEventConsumer_StartAndStop(t *testing.T) {
	hub := events.NewHub(nil, nil)
	kc := events.NewKafkaConsumer(kafka.Config{
		Brokers: []string{"localhost:9092"},
	}, hub, nil)

	mockConsumer := newMockKafkaConsumer()
	kc.SetConsumer(mockConsumer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := kc.Start(ctx); err != nil {
		t.Fatalf("failed to start kafka consumer: %v", err)
	}

	// Give goroutine a moment to start
	time.Sleep(20 * time.Millisecond)

	mockConsumer.mu.Lock()
	if !mockConsumer.subscribed {
		t.Errorf("expected consumer to be subscribed")
	}
	if len(mockConsumer.topics) != 3 {
		t.Errorf("expected 3 topics subscribed, got %d", len(mockConsumer.topics))
	}
	mockConsumer.mu.Unlock()

	if err := kc.Stop(); err != nil {
		t.Fatalf("failed to stop kafka consumer: %v", err)
	}
}
