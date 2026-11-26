package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/routerarchitects/ra-common-mods/kafka"
)

// ConsumerInterface abstracts Kafka consumer operations for testing and flexibility.
type ConsumerInterface interface {
	SubscribeMultiple(ctx context.Context, topics []string, handler kafka.Handler, opts *kafka.SubscribeOptions) error
	Close() error
}

// DeviceEventCallback is invoked whenever a normalized device event is processed from Kafka.
type DeviceEventCallback func(event *DeviceEvent)

// KafkaEventConsumer consumes device events from Kafka and broadcasts them via Hub.
type KafkaEventConsumer struct {
	cfg       kafka.Config
	hub       *Hub
	consumer  ConsumerInterface
	topics    []string
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	started   bool
	callbacks []DeviceEventCallback
}

// NewKafkaConsumer creates a new KafkaEventConsumer instance.
func NewKafkaConsumer(cfg kafka.Config, hub *Hub, logger *slog.Logger) *KafkaEventConsumer {
	if logger == nil {
		logger = slog.Default()
	}

	topics := []string{TopicConnection, TopicState, TopicHealthcheck}

	return &KafkaEventConsumer{
		cfg:    cfg,
		hub:    hub,
		topics: topics,
		logger: logger,
	}
}

// SetConsumer sets a custom consumer (useful for unit testing).
func (kc *KafkaEventConsumer) SetConsumer(c ConsumerInterface) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	kc.consumer = c
}

// AddCallback registers a listener to be called whenever a device event is processed.
func (kc *KafkaEventConsumer) AddCallback(cb DeviceEventCallback) {
	if cb == nil {
		return
	}
	kc.mu.Lock()
	defer kc.mu.Unlock()
	kc.callbacks = append(kc.callbacks, cb)
}

// Start launches the Kafka consumer subscription loop in the background.
func (kc *KafkaEventConsumer) Start(parent context.Context) error {
	kc.mu.Lock()
	defer kc.mu.Unlock()

	if kc.started {
		return errors.New("kafka event consumer is already running")
	}

	if len(kc.cfg.Brokers) == 0 && kc.consumer == nil {
		kc.logger.Info("kafka consumer disabled: no brokers specified in configuration")
		return nil
	}

	if parent == nil {
		parent = context.Background()
	}
	kc.ctx, kc.cancel = context.WithCancel(parent)

	// Initialize Kafka consumer if not already injected
	if kc.consumer == nil {
		if kc.cfg.Consumer.GroupID == "" {
			kc.cfg.Consumer.GroupID = "mdu-device-event-consumer"
		}
		if kc.cfg.Consumer.CommitInterval <= 0 {
			kc.cfg.Consumer.CommitInterval = 1000000000 // 1s
		}

		c, err := kafka.NewConsumer(kc.cfg, kc.logger)
		if err != nil {
			return fmt.Errorf("failed to create kafka consumer: %w", err)
		}
		kc.consumer = c
	}

	kc.started = true

	// Capture consumer and ctx locally so the goroutine does not read
	// kc.consumer after Stop() may have set it to nil.
	consumer := kc.consumer
	ctx := kc.ctx

	kc.wg.Add(1)
	go func() {
		defer kc.wg.Done()
		kc.logger.Info("starting kafka device event consumer loop", "topics", kc.topics)

		opts := kafka.DefaultSubscribeOptions()
		err := consumer.SubscribeMultiple(ctx, kc.topics, kc.handleKafkaMessage, opts)
		if err != nil && !errors.Is(err, context.Canceled) {
			kc.logger.Error("kafka consumer subscription terminated with error", "error", err)
		}
	}()

	return nil
}

// Stop shuts down the consumer and waits for background goroutines to finish.
func (kc *KafkaEventConsumer) Stop() error {
	kc.mu.Lock()
	if !kc.started {
		kc.mu.Unlock()
		return nil
	}
	kc.started = false
	cancel := kc.cancel
	consumer := kc.consumer
	kc.consumer = nil
	kc.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var closeErr error
	if consumer != nil {
		closeErr = consumer.Close()
	}

	kc.wg.Wait()
	return closeErr
}

// ProcessMessage parses a raw Kafka message and broadcasts the resulting event to the hub.
func (kc *KafkaEventConsumer) ProcessMessage(topic string, key, value []byte) error {
	event, err := ParseKafkaMessage(topic, key, value)
	if err != nil {
		kc.logger.Debug("skipping unparseable kafka message", "topic", topic, "error", err)
		return err
	}

	delivered := kc.hub.Broadcast(event)
	kc.mu.Lock()
	cbs := append([]DeviceEventCallback(nil), kc.callbacks...)
	kc.mu.Unlock()
	for _, cb := range cbs {
		cb(event)
	}
	kc.logger.Debug("processed kafka device event",
		"topic", topic,
		"serial", event.SerialNumber,
		"type", event.Type,
		"deliveredToClients", delivered,
	)

	return nil
}

func (kc *KafkaEventConsumer) handleKafkaMessage(ctx context.Context, msg *kafka.Message) error {
	if msg == nil {
		return nil
	}
	_ = kc.ProcessMessage(msg.Topic, msg.Key, msg.Value)
	return nil
}
