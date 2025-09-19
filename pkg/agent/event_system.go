package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// EVENT-DRIVEN COMMUNICATION SYSTEM
// =============================================================================

// EventSystem manages event-driven communication between agents
type EventSystem struct {
	eventBus        *EventBus
	eventStore      *EventStore
	eventHandlers   map[string][]*EventHandler
	eventFilters    map[string]*EventFilter
	eventMetrics    *EventMetrics
	logger          *logging.Logger
	mu              sync.RWMutex
	started         bool
	ctx             context.Context
	cancel          context.CancelFunc
}

// EventBus manages event publishing and subscription
type EventBus struct {
	subscribers     map[string][]*EventSubscriber
	eventQueue      chan *Event
	deadLetterQueue chan *Event
	logger          *logging.Logger
	mu              sync.RWMutex
}

// EventStore persists events for replay and analysis
type EventStore struct {
	events    []*Event
	indexes   map[string][]*Event
	logger    *logging.Logger
	mu        sync.RWMutex
}

// EventHandler handles specific types of events
type EventHandler struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	EventType   string                 `json:"event_type"`
	Handler     func(ctx context.Context, event *Event) error
	Priority    int                    `json:"priority"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	LastActivity time.Time             `json:"last_activity"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// EventFilter filters events based on criteria
type EventFilter struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	EventType   string                 `json:"event_type"`
	Conditions  []*EventCondition      `json:"conditions"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// EventCondition represents a condition for event filtering
type EventCondition struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
	Type     ValueType   `json:"type"`
}

// EventSubscriber subscribes to events
type EventSubscriber struct {
	ID           string                 `json:"id"`
	AgentID      string                 `json:"agent_id"`
	EventTypes   []string               `json:"event_types"`
	Handler      func(ctx context.Context, event *Event) error
	Priority     int                    `json:"priority"`
	Active       bool                   `json:"active"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActivity time.Time              `json:"last_activity"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Event represents an event in the system
type Event struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Target      string                 `json:"target,omitempty"`
	Data        interface{}            `json:"data"`
	Metadata    map[string]interface{} `json:"metadata"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     int                    `json:"version"`
	CorrelationID string               `json:"correlation_id,omitempty"`
	CausationID  string                `json:"causation_id,omitempty"`
	Status      EventStatus            `json:"status"`
}

// EventMetrics tracks event system performance
type EventMetrics struct {
	TotalEvents       int64         `json:"total_events"`
	ProcessedEvents   int64         `json:"processed_events"`
	FailedEvents      int64         `json:"failed_events"`
	DeadLetterEvents  int64         `json:"dead_letter_events"`
	AverageLatency    time.Duration `json:"average_latency"`
	Throughput        float64       `json:"throughput"`
	ActiveSubscribers int           `json:"active_subscribers"`
	ActiveHandlers    int           `json:"active_handlers"`
	ActiveFilters     int           `json:"active_filters"`
	LastUpdated       time.Time     `json:"last_updated"`
	mu                sync.RWMutex
}

// EventStatus represents the status of an event
type EventStatus string

const (
	EventStatusPending    EventStatus = "pending"
	EventStatusProcessing EventStatus = "processing"
	EventStatusProcessed  EventStatus = "processed"
	EventStatusFailed     EventStatus = "failed"
	EventStatusDeadLetter EventStatus = "dead_letter"
)

// NewEventSystem creates a new event system
func NewEventSystem(logger *logging.Logger) *EventSystem {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &EventSystem{
		eventBus: &EventBus{
			subscribers:     make(map[string][]*EventSubscriber),
			eventQueue:      make(chan *Event, 1000),
			deadLetterQueue: make(chan *Event, 100),
			logger:          logger,
		},
		eventStore: &EventStore{
			events:  make([]*Event, 0),
			indexes: make(map[string][]*Event),
			logger:  logger,
		},
		eventHandlers: make(map[string][]*EventHandler),
		eventFilters:  make(map[string]*EventFilter),
		eventMetrics:  &EventMetrics{},
		logger:        logger,
		started:       false,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// =============================================================================
// EVENT SYSTEM MANAGEMENT
// =============================================================================

// Start starts the event system
func (es *EventSystem) Start() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.started {
		return fmt.Errorf("event system is already started")
	}

	es.started = true
	es.logger.Info("Starting event system")

	// Start event processing goroutine
	go es.processEvents()

	// Start dead letter queue processing goroutine
	go es.processDeadLetterQueue()

	// Start metrics collection goroutine
	go es.collectMetrics()

	es.logger.Info("Event system started successfully")
	return nil
}

// Stop stops the event system
func (es *EventSystem) Stop() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.started {
		return fmt.Errorf("event system is not started")
	}

	es.started = false
	es.cancel()

	// Close channels
	close(es.eventBus.eventQueue)
	close(es.eventBus.deadLetterQueue)

	es.logger.Info("Event system stopped")
	return nil
}

// =============================================================================
// EVENT PUBLISHING AND SUBSCRIPTION
// =============================================================================

// PublishEvent publishes an event to the event bus
func (es *EventSystem) PublishEvent(ctx context.Context, event *Event) error {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if !es.started {
		return fmt.Errorf("event system is not started")
	}

	// Set event metadata
	event.ID = uuid.New().String()
	event.Timestamp = time.Now()
	event.Status = EventStatusPending
	event.Version = 1

	// Store event
	if err := es.eventStore.StoreEvent(event); err != nil {
		es.logger.WithError(err).WithField("event_id", event.ID).Error("Failed to store event")
		return fmt.Errorf("failed to store event: %w", err)
	}

	// Apply filters
	filteredEvent, err := es.applyEventFilters(event)
	if err != nil {
		es.logger.WithError(err).WithField("event_id", event.ID).Error("Failed to apply event filters")
		return fmt.Errorf("failed to apply event filters: %w", err)
	}

	if filteredEvent == nil {
		es.logger.WithField("event_id", event.ID).Debug("Event filtered out")
		return nil
	}

	// Queue event for processing
	select {
	case es.eventBus.eventQueue <- filteredEvent:
		es.updateMetrics("published")
		es.logger.WithFields(map[string]interface{}{
			"event_id": event.ID,
			"type":     event.Type,
			"source":   event.Source,
		}).Debug("Event published successfully")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("event queue is full")
	}
}

// SubscribeToEvents subscribes to events
func (es *EventSystem) SubscribeToEvents(ctx context.Context, subscriber *EventSubscriber) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.started {
		return fmt.Errorf("event system is not started")
	}

	// Set subscriber metadata
	subscriber.ID = uuid.New().String()
	subscriber.CreatedAt = time.Now()
	subscriber.LastActivity = time.Now()
	subscriber.Active = true

	// Add subscriber to event types
	for _, eventType := range subscriber.EventTypes {
		es.eventBus.subscribers[eventType] = append(es.eventBus.subscribers[eventType], subscriber)
	}

	es.updateMetrics("subscribed")
	es.logger.WithFields(map[string]interface{}{
		"subscriber_id": subscriber.ID,
		"agent_id":      subscriber.AgentID,
		"event_types":   subscriber.EventTypes,
	}).Info("Event subscriber registered successfully")

	return nil
}

// UnsubscribeFromEvents unsubscribes from events
func (es *EventSystem) UnsubscribeFromEvents(ctx context.Context, subscriberID string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.started {
		return fmt.Errorf("event system is not started")
	}

	// Remove subscriber from all event types
	for eventType, subscribers := range es.eventBus.subscribers {
		for i, subscriber := range subscribers {
			if subscriber.ID == subscriberID {
				es.eventBus.subscribers[eventType] = append(subscribers[:i], subscribers[i+1:]...)
				break
			}
		}
	}

	es.updateMetrics("unsubscribed")
	es.logger.WithField("subscriber_id", subscriberID).Info("Event subscriber unregistered successfully")

	return nil
}

// =============================================================================
// EVENT HANDLERS
// =============================================================================

// RegisterEventHandler registers an event handler
func (es *EventSystem) RegisterEventHandler(handler *EventHandler) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	handler.ID = uuid.New().String()
	handler.CreatedAt = time.Now()
	handler.LastActivity = time.Now()
	handler.Active = true

	es.eventHandlers[handler.EventType] = append(es.eventHandlers[handler.EventType], handler)

	es.logger.WithFields(map[string]interface{}{
		"handler_id":  handler.ID,
		"agent_id":    handler.AgentID,
		"event_type":  handler.EventType,
	}).Info("Event handler registered successfully")

	return nil
}

// UnregisterEventHandler unregisters an event handler
func (es *EventSystem) UnregisterEventHandler(handlerID string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Remove handler from all event types
	for eventType, handlers := range es.eventHandlers {
		for i, handler := range handlers {
			if handler.ID == handlerID {
				es.eventHandlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}

	es.logger.WithField("handler_id", handlerID).Info("Event handler unregistered successfully")
	return nil
}

// =============================================================================
// EVENT FILTERS
// =============================================================================

// AddEventFilter adds an event filter
func (es *EventSystem) AddEventFilter(filter *EventFilter) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	filter.ID = uuid.New().String()
	filter.CreatedAt = time.Now()
	filter.Active = true

	es.eventFilters[filter.ID] = filter

	es.logger.WithFields(map[string]interface{}{
		"filter_id":   filter.ID,
		"filter_name": filter.Name,
		"event_type":  filter.EventType,
	}).Info("Event filter added successfully")

	return nil
}

// RemoveEventFilter removes an event filter
func (es *EventSystem) RemoveEventFilter(filterID string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	delete(es.eventFilters, filterID)

	es.logger.WithField("filter_id", filterID).Info("Event filter removed successfully")
	return nil
}

// =============================================================================
// EVENT PROCESSING
// =============================================================================

// processEvents processes events from the queue
func (es *EventSystem) processEvents() {
	es.logger.Info("Starting event processing")

	for {
		select {
		case event := <-es.eventBus.eventQueue:
			es.processEvent(event)
		case <-es.ctx.Done():
			es.logger.Info("Event processing stopped")
			return
		}
	}
}

// processEvent processes a single event
func (es *EventSystem) processEvent(event *Event) {
	startTime := time.Now()
	
	es.logger.WithFields(map[string]interface{}{
		"event_id": event.ID,
		"type":     event.Type,
		"source":   event.Source,
	}).Debug("Processing event")

	// Update event status
	event.Status = EventStatusProcessing

	// Find subscribers for the event type
	subscribers := es.getSubscribersForEventType(event.Type)
	
	if len(subscribers) == 0 {
		es.logger.WithFields(map[string]interface{}{
			"event_id": event.ID,
			"type":     event.Type,
		}).Warn("No subscribers found for event type")
		
		// Send to dead letter queue
		es.sendToDeadLetterQueue(event, "No subscribers found")
		return
	}

	// Process event for each subscriber
	successCount := 0
	for _, subscriber := range subscribers {
		if !subscriber.Active {
			continue
		}

		// Process event
		if err := es.processEventForSubscriber(event, subscriber); err != nil {
			es.logger.WithError(err).WithFields(map[string]interface{}{
				"event_id":      event.ID,
				"subscriber_id": subscriber.ID,
			}).Error("Failed to process event for subscriber")
		} else {
			successCount++
		}
	}

	// Update event status
	if successCount > 0 {
		event.Status = EventStatusProcessed
		es.updateMetrics("processed")
	} else {
		event.Status = EventStatusFailed
		es.updateMetrics("failed")
		es.sendToDeadLetterQueue(event, "No successful processing")
	}

	// Update latency metrics
	latency := time.Since(startTime)
	es.updateLatencyMetrics(latency)

	es.logger.WithFields(map[string]interface{}{
		"event_id":      event.ID,
		"success_count": successCount,
		"latency":       latency,
	}).Debug("Event processing completed")
}

// processEventForSubscriber processes an event for a specific subscriber
func (es *EventSystem) processEventForSubscriber(event *Event, subscriber *EventSubscriber) error {
	// Update subscriber activity
	subscriber.LastActivity = time.Now()

	// Call subscriber handler
	if subscriber.Handler != nil {
		ctx, cancel := context.WithTimeout(es.ctx, 30*time.Second)
		defer cancel()

		return subscriber.Handler(ctx, event)
	}

	return fmt.Errorf("no handler defined for subscriber %s", subscriber.ID)
}

// getSubscribersForEventType gets subscribers for a specific event type
func (es *EventSystem) getSubscribersForEventType(eventType string) []*EventSubscriber {
	es.mu.RLock()
	defer es.mu.RUnlock()

	subscribers, exists := es.eventBus.subscribers[eventType]
	if !exists {
		return []*EventSubscriber{}
	}

	// Filter active subscribers
	var activeSubscribers []*EventSubscriber
	for _, subscriber := range subscribers {
		if subscriber.Active {
			activeSubscribers = append(activeSubscribers, subscriber)
		}
	}

	return activeSubscribers
}

// applyEventFilters applies filters to an event
func (es *EventSystem) applyEventFilters(event *Event) (*Event, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// Apply filters for the event type
	for _, filter := range es.eventFilters {
		if !filter.Active || filter.EventType != event.Type {
			continue
		}

		if es.matchesEventConditions(event, filter.Conditions) {
			// Event matches filter conditions
			// For now, we'll just pass the event through
			// In a real system, this would apply transformations, etc.
		}
	}

	return event, nil
}

// matchesEventConditions checks if an event matches the given conditions
func (es *EventSystem) matchesEventConditions(event *Event, conditions []*EventCondition) bool {
	for _, condition := range conditions {
		if !es.matchesEventCondition(event, condition) {
			return false
		}
	}
	return true
}

// matchesEventCondition checks if an event matches a single condition
func (es *EventSystem) matchesEventCondition(event *Event, condition *EventCondition) bool {
	// Get field value from event
	fieldValue := es.getEventFieldValue(event, condition.Field)
	
	// Apply operator
	switch condition.Operator {
	case OperatorEquals:
		return fieldValue == condition.Value
	case OperatorNotEquals:
		return fieldValue != condition.Value
	case OperatorContains:
		if fieldStr, ok := fieldValue.(string); ok {
			if valueStr, ok := condition.Value.(string); ok {
				return strings.Contains(fieldStr, valueStr)
			}
		}
		return false
	case OperatorNotContains:
		if fieldStr, ok := fieldValue.(string); ok {
			if valueStr, ok := condition.Value.(string); ok {
				return !strings.Contains(fieldStr, valueStr)
			}
		}
		return true
	case OperatorGreater:
		return es.compareValues(fieldValue, condition.Value) > 0
	case OperatorLess:
		return es.compareValues(fieldValue, condition.Value) < 0
	case OperatorIn:
		if valueArray, ok := condition.Value.([]interface{}); ok {
			for _, v := range valueArray {
				if fieldValue == v {
					return true
				}
			}
		}
		return false
	case OperatorNotIn:
		if valueArray, ok := condition.Value.([]interface{}); ok {
			for _, v := range valueArray {
				if fieldValue == v {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

// getEventFieldValue gets a field value from an event
func (es *EventSystem) getEventFieldValue(event *Event, field string) interface{} {
	switch field {
	case "type":
		return event.Type
	case "source":
		return event.Source
	case "target":
		return event.Target
	case "version":
		return event.Version
	case "status":
		return event.Status
	default:
		// Check in metadata
		if value, exists := event.Metadata[field]; exists {
			return value
		}
		return nil
	}
}

// compareValues compares two values
func (es *EventSystem) compareValues(a, b interface{}) int {
	// Simple comparison - in a real system, this would be more sophisticated
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

// sendToDeadLetterQueue sends an event to the dead letter queue
func (es *EventSystem) sendToDeadLetterQueue(event *Event, reason string) {
	event.Status = EventStatusDeadLetter
	event.Metadata["dead_letter_reason"] = reason
	event.Metadata["dead_letter_timestamp"] = time.Now()

	select {
	case es.eventBus.deadLetterQueue <- event:
		es.updateMetrics("dead_letter")
		es.logger.WithFields(map[string]interface{}{
			"event_id": event.ID,
			"reason":   reason,
		}).Warn("Event sent to dead letter queue")
	default:
		es.logger.WithField("event_id", event.ID).Error("Dead letter queue is full")
	}
}

// processDeadLetterQueue processes events in the dead letter queue
func (es *EventSystem) processDeadLetterQueue() {
	es.logger.Info("Starting dead letter queue processing")

	for {
		select {
		case event := <-es.eventBus.deadLetterQueue:
			es.processDeadLetterEvent(event)
		case <-es.ctx.Done():
			es.logger.Info("Dead letter queue processing stopped")
			return
		}
	}
}

// processDeadLetterEvent processes a dead letter event
func (es *EventSystem) processDeadLetterEvent(event *Event) {
	es.logger.WithFields(map[string]interface{}{
		"event_id": event.ID,
		"type":     event.Type,
		"reason":   event.Metadata["dead_letter_reason"],
	}).Warn("Processing dead letter event")

	// In a real system, this would implement retry logic, alerting, etc.
	// For now, we just log the event
}

// =============================================================================
// EVENT STORE
// =============================================================================

// StoreEvent stores an event in the event store
func (es *EventStore) StoreEvent(event *Event) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Add event to store
	es.events = append(es.events, event)

	// Update indexes
	es.indexes[event.Type] = append(es.indexes[event.Type], event)
	es.indexes[event.Source] = append(es.indexes[event.Source], event)

	es.logger.WithFields(map[string]interface{}{
		"event_id": event.ID,
		"type":     event.Type,
		"source":   event.Source,
	}).Debug("Event stored successfully")

	return nil
}

// GetEventsByType gets events by type
func (es *EventStore) GetEventsByType(eventType string) ([]*Event, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	events, exists := es.indexes[eventType]
	if !exists {
		return []*Event{}, nil
	}

	// Return a copy of the events
	eventsCopy := make([]*Event, len(events))
	copy(eventsCopy, events)

	return eventsCopy, nil
}

// GetEventsBySource gets events by source
func (es *EventStore) GetEventsBySource(source string) ([]*Event, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	events, exists := es.indexes[source]
	if !exists {
		return []*Event{}, nil
	}

	// Return a copy of the events
	eventsCopy := make([]*Event, len(events))
	copy(eventsCopy, events)

	return eventsCopy, nil
}

// GetEventByID gets an event by ID
func (es *EventStore) GetEventByID(eventID string) (*Event, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	for _, event := range es.events {
		if event.ID == eventID {
			return event, nil
		}
	}

	return nil, fmt.Errorf("event %s not found", eventID)
}

// =============================================================================
// METRICS AND MONITORING
// =============================================================================

// collectMetrics collects and updates metrics
func (es *EventSystem) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			es.updateMetrics("collect")
		case <-es.ctx.Done():
			return
		}
	}
}

// updateMetrics updates event system metrics
func (es *EventSystem) updateMetrics(event string) {
	es.eventMetrics.mu.Lock()
	defer es.eventMetrics.mu.Unlock()

	switch event {
	case "published":
		es.eventMetrics.TotalEvents++
	case "processed":
		es.eventMetrics.ProcessedEvents++
	case "failed":
		es.eventMetrics.FailedEvents++
	case "dead_letter":
		es.eventMetrics.DeadLetterEvents++
	case "subscribed":
		es.eventMetrics.ActiveSubscribers++
	case "unsubscribed":
		es.eventMetrics.ActiveSubscribers--
	case "collect":
		es.eventMetrics.ActiveHandlers = len(es.eventHandlers)
		es.eventMetrics.ActiveFilters = len(es.eventFilters)
		es.eventMetrics.LastUpdated = time.Now()
		
		// Calculate throughput
		if es.eventMetrics.TotalEvents > 0 {
			es.eventMetrics.Throughput = float64(es.eventMetrics.ProcessedEvents) / float64(es.eventMetrics.TotalEvents)
		}
	}
}

// updateLatencyMetrics updates latency metrics
func (es *EventSystem) updateLatencyMetrics(latency time.Duration) {
	es.eventMetrics.mu.Lock()
	defer es.eventMetrics.mu.Unlock()

	// Simple moving average
	if es.eventMetrics.AverageLatency == 0 {
		es.eventMetrics.AverageLatency = latency
	} else {
		es.eventMetrics.AverageLatency = (es.eventMetrics.AverageLatency + latency) / 2
	}
}

// GetMetrics returns current metrics
func (es *EventSystem) GetMetrics() *EventMetrics {
	es.eventMetrics.mu.RLock()
	defer es.eventMetrics.mu.RUnlock()

	// Return a copy of the metrics
	return &EventMetrics{
		TotalEvents:       es.eventMetrics.TotalEvents,
		ProcessedEvents:   es.eventMetrics.ProcessedEvents,
		FailedEvents:      es.eventMetrics.FailedEvents,
		DeadLetterEvents:  es.eventMetrics.DeadLetterEvents,
		AverageLatency:    es.eventMetrics.AverageLatency,
		Throughput:        es.eventMetrics.Throughput,
		ActiveSubscribers: es.eventMetrics.ActiveSubscribers,
		ActiveHandlers:    es.eventMetrics.ActiveHandlers,
		ActiveFilters:     es.eventMetrics.ActiveFilters,
		LastUpdated:       es.eventMetrics.LastUpdated,
	}
}

// GetStatus returns the current status of the event system
func (es *EventSystem) GetStatus() map[string]interface{} {
	es.mu.RLock()
	defer es.mu.RUnlock()

	return map[string]interface{}{
		"started":           es.started,
		"subscriber_count":  len(es.eventBus.subscribers),
		"handler_count":     len(es.eventHandlers),
		"filter_count":      len(es.eventFilters),
		"event_count":       len(es.eventStore.events),
		"queue_size":        len(es.eventBus.eventQueue),
		"dead_letter_size":  len(es.eventBus.deadLetterQueue),
		"metrics":           es.GetMetrics(),
		"timestamp":         time.Now().Format(time.RFC3339),
	}
}
