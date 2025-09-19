package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// ADVANCED MESSAGE BUS
// =============================================================================

// Message represents a bus-level message (separate from AgentMessage)
type Message struct {
	ID        string                 `json:"id"`
	Topic     string                 `json:"topic"`
	Type      string                 `json:"type"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Content   interface{}            `json:"content"`
	Priority  MessagePriority        `json:"priority"`
	Status    MessageStatus          `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	RouteID   string                 `json:"route_id"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// MessagePriority represents message priority
type MessagePriority int

const (
	MessagePriorityLow MessagePriority = iota
	MessagePriorityNormal
	MessagePriorityHigh
)

// MessageStatus represents processing status of a message
type MessageStatus string

const (
	MessageStatusPending    MessageStatus = "pending"
	MessageStatusProcessed  MessageStatus = "processed"
	MessageStatusFailed     MessageStatus = "failed"
	MessageStatusDeadLetter MessageStatus = "dead_letter"
)

// AdvancedMessageBus provides advanced message routing and filtering capabilities
type AdvancedMessageBus struct {
	subscribers     map[string][]*Subscriber
	routes          map[string]*Route
	filters         map[string]*Filter
	messageQueue    chan *Message
	deadLetterQueue chan *Message
	metrics         *MessageMetrics
	logger          *logging.Logger
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	started         bool
}

// Subscriber represents a message subscriber
type Subscriber struct {
	ID           string                 `json:"id"`
	AgentID      string                 `json:"agent_id"`
	AgentType    AgentType              `json:"agent_type"`
	Topics       []string               `json:"topics"`
	Filters      []*Filter              `json:"filters"`
	Handler      BusMessageHandler      `json:"-"`
	Priority     int                    `json:"priority"`
	Active       bool                   `json:"active"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActivity time.Time              `json:"last_activity"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Route represents a message routing rule
type Route struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Pattern     string                 `json:"pattern"`
	Destination string                 `json:"destination"`
	Priority    int                    `json:"priority"`
	Active      bool                   `json:"active"`
	Conditions  []*Condition           `json:"conditions"`
	Transform   *Transform             `json:"transform,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Filter represents a message filter
type Filter struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       FilterType             `json:"type"`
	Conditions []*Condition           `json:"conditions"`
	Active     bool                   `json:"active"`
	CreatedAt  time.Time              `json:"created_at"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// Condition represents a filter condition
type Condition struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
	Type     ValueType   `json:"type"`
}

// Transform represents a message transformation
type Transform struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Operations []*TransformOperation  `json:"operations"`
	Active     bool                   `json:"active"`
	CreatedAt  time.Time              `json:"created_at"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// TransformOperation represents a transformation operation
type TransformOperation struct {
	Type     TransformType          `json:"type"`
	Field    string                 `json:"field"`
	Value    interface{}            `json:"value"`
	Metadata map[string]interface{} `json:"metadata"`
}

// MessageMetrics tracks message bus performance
type MessageMetrics struct {
	TotalMessages      int64         `json:"total_messages"`
	ProcessedMessages  int64         `json:"processed_messages"`
	FailedMessages     int64         `json:"failed_messages"`
	DeadLetterMessages int64         `json:"dead_letter_messages"`
	AverageLatency     time.Duration `json:"average_latency"`
	Throughput         float64       `json:"throughput"`
	ActiveSubscribers  int           `json:"active_subscribers"`
	ActiveRoutes       int           `json:"active_routes"`
	ActiveFilters      int           `json:"active_filters"`
	LastUpdated        time.Time     `json:"last_updated"`
	mu                 sync.RWMutex
}

// BusMessageHandler is a function type for handling messages on the bus
type BusMessageHandler func(ctx context.Context, message *Message) error

// FilterType represents the type of filter
type FilterType string

const (
	FilterTypeInclude   FilterType = "include"
	FilterTypeExclude   FilterType = "exclude"
	FilterTypeTransform FilterType = "transform"
)

// Operator represents a comparison operator
type Operator string

const (
	OperatorEquals      Operator = "equals"
	OperatorNotEquals   Operator = "not_equals"
	OperatorContains    Operator = "contains"
	OperatorNotContains Operator = "not_contains"
	OperatorGreater     Operator = "greater"
	OperatorLess        Operator = "less"
	OperatorRegex       Operator = "regex"
	OperatorIn          Operator = "in"
	OperatorNotIn       Operator = "not_in"
)

// ValueType represents the type of value
type ValueType string

const (
	ValueTypeString  ValueType = "string"
	ValueTypeNumber  ValueType = "number"
	ValueTypeBoolean ValueType = "boolean"
	ValueTypeArray   ValueType = "array"
	ValueTypeObject  ValueType = "object"
)

// TransformType represents the type of transformation
type TransformType string

const (
	TransformTypeAdd    TransformType = "add"
	TransformTypeRemove TransformType = "remove"
	TransformTypeUpdate TransformType = "update"
	TransformTypeRename TransformType = "rename"
	TransformTypeCopy   TransformType = "copy"
	TransformTypeMove   TransformType = "move"
)

// NewAdvancedMessageBus creates a new advanced message bus
func NewAdvancedMessageBus(logger *logging.Logger) *AdvancedMessageBus {
	ctx, cancel := context.WithCancel(context.Background())

	return &AdvancedMessageBus{
		subscribers:     make(map[string][]*Subscriber),
		routes:          make(map[string]*Route),
		filters:         make(map[string]*Filter),
		messageQueue:    make(chan *Message, 1000),
		deadLetterQueue: make(chan *Message, 100),
		metrics:         &MessageMetrics{},
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		started:         false,
	}
}

// =============================================================================
// MESSAGE BUS INTERFACE IMPLEMENTATION
// =============================================================================

// Start starts the message bus
func (amb *AdvancedMessageBus) Start() error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	if amb.started {
		return fmt.Errorf("message bus is already started")
	}

	amb.started = true
	amb.logger.Info("Starting advanced message bus")

	// Start message processing goroutine
	go amb.processMessages()

	// Start metrics collection goroutine
	go amb.collectMetrics()

	// Start dead letter queue processing goroutine
	go amb.processDeadLetterQueue()

	amb.logger.Info("Advanced message bus started successfully")
	return nil
}

// Stop stops the message bus
func (amb *AdvancedMessageBus) Stop() error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	if !amb.started {
		return fmt.Errorf("message bus is not started")
	}

	amb.started = false
	amb.cancel()

	// Close channels
	close(amb.messageQueue)
	close(amb.deadLetterQueue)

	amb.logger.Info("Advanced message bus stopped")
	return nil
}

// Publish publishes a message to the message bus
func (amb *AdvancedMessageBus) Publish(ctx context.Context, message *Message) error {
	amb.mu.RLock()
	defer amb.mu.RUnlock()

	if !amb.started {
		return fmt.Errorf("message bus is not started")
	}

	// Set message metadata
	message.ID = uuid.New().String()
	message.Timestamp = time.Now()
	message.Status = MessageStatusPending

	// Apply routing
	routedMessage, err := amb.applyRouting(message)
	if err != nil {
		amb.logger.WithError(err).WithField("message_id", message.ID).Error("Failed to apply routing")
		return fmt.Errorf("failed to apply routing: %w", err)
	}

	// Apply filters
	filteredMessage, err := amb.applyFilters(routedMessage)
	if err != nil {
		amb.logger.WithError(err).WithField("message_id", message.ID).Error("Failed to apply filters")
		return fmt.Errorf("failed to apply filters: %w", err)
	}

	// Queue message for processing
	select {
	case amb.messageQueue <- filteredMessage:
		amb.updateMetrics("published")
		amb.logger.WithFields(map[string]interface{}{
			"message_id": message.ID,
			"topic":      message.Topic,
			"from":       message.From,
			"to":         message.To,
		}).Debug("Message published successfully")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("message queue is full")
	}
}

// Subscribe subscribes to messages
func (amb *AdvancedMessageBus) Subscribe(ctx context.Context, subscriber *Subscriber) error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	if !amb.started {
		return fmt.Errorf("message bus is not started")
	}

	// Set subscriber metadata
	subscriber.ID = uuid.New().String()
	subscriber.CreatedAt = time.Now()
	subscriber.LastActivity = time.Now()
	subscriber.Active = true

	// Add subscriber to topics
	for _, topic := range subscriber.Topics {
		amb.subscribers[topic] = append(amb.subscribers[topic], subscriber)
	}

	amb.updateMetrics("subscribed")
	amb.logger.WithFields(map[string]interface{}{
		"subscriber_id": subscriber.ID,
		"agent_id":      subscriber.AgentID,
		"topics":        subscriber.Topics,
	}).Info("Subscriber registered successfully")

	return nil
}

// Unsubscribe unsubscribes from messages
func (amb *AdvancedMessageBus) Unsubscribe(ctx context.Context, subscriberID string) error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	if !amb.started {
		return fmt.Errorf("message bus is not started")
	}

	// Remove subscriber from all topics
	for topic, subscribers := range amb.subscribers {
		for i, subscriber := range subscribers {
			if subscriber.ID == subscriberID {
				amb.subscribers[topic] = append(subscribers[:i], subscribers[i+1:]...)
				break
			}
		}
	}

	amb.updateMetrics("unsubscribed")
	amb.logger.WithField("subscriber_id", subscriberID).Info("Subscriber unregistered successfully")

	return nil
}

// =============================================================================
// ROUTING AND FILTERING
// =============================================================================

// AddRoute adds a routing rule
func (amb *AdvancedMessageBus) AddRoute(route *Route) error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	route.ID = uuid.New().String()
	route.CreatedAt = time.Now()
	route.Active = true

	amb.routes[route.ID] = route

	amb.logger.WithFields(map[string]interface{}{
		"route_id":   route.ID,
		"route_name": route.Name,
		"pattern":    route.Pattern,
	}).Info("Route added successfully")

	return nil
}

// RemoveRoute removes a routing rule
func (amb *AdvancedMessageBus) RemoveRoute(routeID string) error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	delete(amb.routes, routeID)

	amb.logger.WithField("route_id", routeID).Info("Route removed successfully")
	return nil
}

// AddFilter adds a filter
func (amb *AdvancedMessageBus) AddFilter(filter *Filter) error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	filter.ID = uuid.New().String()
	filter.CreatedAt = time.Now()
	filter.Active = true

	amb.filters[filter.ID] = filter

	amb.logger.WithFields(map[string]interface{}{
		"filter_id":   filter.ID,
		"filter_name": filter.Name,
		"filter_type": filter.Type,
	}).Info("Filter added successfully")

	return nil
}

// RemoveFilter removes a filter
func (amb *AdvancedMessageBus) RemoveFilter(filterID string) error {
	amb.mu.Lock()
	defer amb.mu.Unlock()

	delete(amb.filters, filterID)

	amb.logger.WithField("filter_id", filterID).Info("Filter removed successfully")
	return nil
}

// applyRouting applies routing rules to a message
func (amb *AdvancedMessageBus) applyRouting(message *Message) (*Message, error) {
	amb.mu.RLock()
	defer amb.mu.RUnlock()

	// Apply routing rules in priority order
	for _, route := range amb.routes {
		if !route.Active {
			continue
		}

		if amb.matchesRoute(message, route) {
			// Apply route conditions
			if amb.matchesConditions(message, route.Conditions) {
				// Apply transformation if specified
				if route.Transform != nil {
					transformedMessage, err := amb.applyTransform(message, route.Transform)
					if err != nil {
						return nil, fmt.Errorf("failed to apply transformation: %w", err)
					}
					message = transformedMessage
				}

				// Update message destination
				message.To = route.Destination
				message.RouteID = route.ID

				amb.logger.WithFields(map[string]interface{}{
					"message_id":  message.ID,
					"route_id":    route.ID,
					"destination": route.Destination,
				}).Debug("Message routed successfully")
			}
		}
	}

	return message, nil
}

// applyFilters applies filters to a message
func (amb *AdvancedMessageBus) applyFilters(message *Message) (*Message, error) {
	amb.mu.RLock()
	defer amb.mu.RUnlock()

	// Apply filters in priority order
	for _, filter := range amb.filters {
		if !filter.Active {
			continue
		}

		if amb.matchesConditions(message, filter.Conditions) {
			switch filter.Type {
			case FilterTypeInclude:
				// Message passes through
				continue
			case FilterTypeExclude:
				// Message is filtered out
				return nil, fmt.Errorf("message filtered out by filter %s", filter.ID)
			case FilterTypeTransform:
				// Apply transformation
				transformedMessage, err := amb.applyTransform(message, &Transform{
					ID:         filter.ID,
					Name:       filter.Name,
					Operations: []*TransformOperation{}, // This would be populated from filter
					Active:     true,
					CreatedAt:  time.Now(),
				})
				if err != nil {
					return nil, fmt.Errorf("failed to apply filter transformation: %w", err)
				}
				message = transformedMessage
			}
		}
	}

	return message, nil
}

// matchesRoute checks if a message matches a route pattern
func (amb *AdvancedMessageBus) matchesRoute(message *Message, route *Route) bool {
	// Simple pattern matching - in a real system, this would be more sophisticated
	return message.Topic == route.Pattern || message.Type == route.Pattern
}

// matchesConditions checks if a message matches the given conditions
func (amb *AdvancedMessageBus) matchesConditions(message *Message, conditions []*Condition) bool {
	for _, condition := range conditions {
		if !amb.matchesCondition(message, condition) {
			return false
		}
	}
	return true
}

// matchesCondition checks if a message matches a single condition
func (amb *AdvancedMessageBus) matchesCondition(message *Message, condition *Condition) bool {
	// Get field value from message
	fieldValue := amb.getFieldValue(message, condition.Field)

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
		return amb.compareValues(fieldValue, condition.Value) > 0
	case OperatorLess:
		return amb.compareValues(fieldValue, condition.Value) < 0
	case OperatorRegex:
		// This would require regex implementation
		return false
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

// getFieldValue gets a field value from a message
func (amb *AdvancedMessageBus) getFieldValue(message *Message, field string) interface{} {
	switch field {
	case "topic":
		return message.Topic
	case "type":
		return message.Type
	case "from":
		return message.From
	case "to":
		return message.To
	case "priority":
		return message.Priority
	case "status":
		return message.Status
	default:
		// Check in metadata
		if value, exists := message.Metadata[field]; exists {
			return value
		}
		return nil
	}
}

// compareValues compares two values
func (amb *AdvancedMessageBus) compareValues(a, b interface{}) int {
	// Best-effort comparison for common types
	switch av := a.(type) {
	case int:
		if bv, ok := b.(int); ok {
			if av == bv {
				return 0
			}
			if av < bv {
				return -1
			}
			return 1
		}
	case int64:
		if bv, ok := b.(int64); ok {
			if av == bv {
				return 0
			}
			if av < bv {
				return -1
			}
			return 1
		}
	case float64:
		if bv, ok := b.(float64); ok {
			if av == bv {
				return 0
			}
			if av < bv {
				return -1
			}
			return 1
		}
	case string:
		if bv, ok := b.(string); ok {
			if av == bv {
				return 0
			}
			if av < bv {
				return -1
			}
			return 1
		}
	}
	// Fallback: not comparable
	if a == b {
		return 0
	}
	return 1
}

// applyTransform applies a transformation to a message
func (amb *AdvancedMessageBus) applyTransform(message *Message, transform *Transform) (*Message, error) {
	// Create a copy of the message
	transformedMessage := &Message{
		ID:        message.ID,
		Topic:     message.Topic,
		Type:      message.Type,
		From:      message.From,
		To:        message.To,
		Content:   message.Content,
		Priority:  message.Priority,
		Status:    message.Status,
		Timestamp: message.Timestamp,
		RouteID:   message.RouteID,
		Metadata:  make(map[string]interface{}),
	}

	// Copy metadata
	for k, v := range message.Metadata {
		transformedMessage.Metadata[k] = v
	}

	// Apply transformation operations
	for _, operation := range transform.Operations {
		switch operation.Type {
		case TransformTypeAdd:
			transformedMessage.Metadata[operation.Field] = operation.Value
		case TransformTypeRemove:
			delete(transformedMessage.Metadata, operation.Field)
		case TransformTypeUpdate:
			transformedMessage.Metadata[operation.Field] = operation.Value
		case TransformTypeRename:
			if oldValue, exists := transformedMessage.Metadata[operation.Field]; exists {
				if newField, ok := operation.Metadata["new_field"].(string); ok {
					transformedMessage.Metadata[newField] = oldValue
					delete(transformedMessage.Metadata, operation.Field)
				}
			}
		case TransformTypeCopy:
			if oldValue, exists := transformedMessage.Metadata[operation.Field]; exists {
				if newField, ok := operation.Metadata["new_field"].(string); ok {
					transformedMessage.Metadata[newField] = oldValue
				}
			}
		case TransformTypeMove:
			if oldValue, exists := transformedMessage.Metadata[operation.Field]; exists {
				if newField, ok := operation.Metadata["new_field"].(string); ok {
					transformedMessage.Metadata[newField] = oldValue
					delete(transformedMessage.Metadata, operation.Field)
				}
			}
		}
	}

	return transformedMessage, nil
}

// =============================================================================
// MESSAGE PROCESSING
// =============================================================================

// processMessages processes messages from the queue
func (amb *AdvancedMessageBus) processMessages() {
	amb.logger.Info("Starting message processing")

	for {
		select {
		case message := <-amb.messageQueue:
			amb.processMessage(message)
		case <-amb.ctx.Done():
			amb.logger.Info("Message processing stopped")
			return
		}
	}
}

// processMessage processes a single message
func (amb *AdvancedMessageBus) processMessage(message *Message) {
	startTime := time.Now()

	amb.logger.WithFields(map[string]interface{}{
		"message_id": message.ID,
		"topic":      message.Topic,
		"type":       message.Type,
	}).Debug("Processing message")

	// Find subscribers for the message topic
	subscribers := amb.getSubscribersForTopic(message.Topic)

	if len(subscribers) == 0 {
		amb.logger.WithFields(map[string]interface{}{
			"message_id": message.ID,
			"topic":      message.Topic,
		}).Warn("No subscribers found for topic")

		// Send to dead letter queue
		amb.sendToDeadLetterQueue(message, "No subscribers found")
		return
	}

	// Process message for each subscriber
	successCount := 0
	for _, subscriber := range subscribers {
		if !subscriber.Active {
			continue
		}

		// Check if message matches subscriber filters
		if amb.matchesSubscriberFilters(message, subscriber) {
			// Process message
			if err := amb.processMessageForSubscriber(message, subscriber); err != nil {
				amb.logger.WithError(err).WithFields(map[string]interface{}{
					"message_id":    message.ID,
					"subscriber_id": subscriber.ID,
				}).Error("Failed to process message for subscriber")
			} else {
				successCount++
			}
		}
	}

	// Update message status
	if successCount > 0 {
		message.Status = MessageStatusProcessed
		amb.updateMetrics("processed")
	} else {
		message.Status = MessageStatusFailed
		amb.updateMetrics("failed")
		amb.sendToDeadLetterQueue(message, "No successful processing")
	}

	// Update latency metrics
	latency := time.Since(startTime)
	amb.updateLatencyMetrics(latency)

	amb.logger.WithFields(map[string]interface{}{
		"message_id":    message.ID,
		"success_count": successCount,
		"latency":       latency,
	}).Debug("Message processing completed")
}

// processMessageForSubscriber processes a message for a specific subscriber
func (amb *AdvancedMessageBus) processMessageForSubscriber(message *Message, subscriber *Subscriber) error {
	// Update subscriber activity
	subscriber.LastActivity = time.Now()

	// Call subscriber handler
	if subscriber.Handler != nil {
		ctx, cancel := context.WithTimeout(amb.ctx, 30*time.Second)
		defer cancel()

		return subscriber.Handler(ctx, message)
	}

	return fmt.Errorf("no handler defined for subscriber %s", subscriber.ID)
}

// getSubscribersForTopic gets subscribers for a specific topic
func (amb *AdvancedMessageBus) getSubscribersForTopic(topic string) []*Subscriber {
	amb.mu.RLock()
	defer amb.mu.RUnlock()

	subscribers, exists := amb.subscribers[topic]
	if !exists {
		return []*Subscriber{}
	}

	// Filter active subscribers
	var activeSubscribers []*Subscriber
	for _, subscriber := range subscribers {
		if subscriber.Active {
			activeSubscribers = append(activeSubscribers, subscriber)
		}
	}

	return activeSubscribers
}

// matchesSubscriberFilters checks if a message matches subscriber filters
func (amb *AdvancedMessageBus) matchesSubscriberFilters(message *Message, subscriber *Subscriber) bool {
	for _, filter := range subscriber.Filters {
		if !amb.matchesConditions(message, filter.Conditions) {
			return false
		}
	}
	return true
}

// sendToDeadLetterQueue sends a message to the dead letter queue
func (amb *AdvancedMessageBus) sendToDeadLetterQueue(message *Message, reason string) {
	message.Status = MessageStatusDeadLetter
	message.Metadata["dead_letter_reason"] = reason
	message.Metadata["dead_letter_timestamp"] = time.Now()

	select {
	case amb.deadLetterQueue <- message:
		amb.updateMetrics("dead_letter")
		amb.logger.WithFields(map[string]interface{}{
			"message_id": message.ID,
			"reason":     reason,
		}).Warn("Message sent to dead letter queue")
	default:
		amb.logger.WithField("message_id", message.ID).Error("Dead letter queue is full")
	}
}

// processDeadLetterQueue processes messages in the dead letter queue
func (amb *AdvancedMessageBus) processDeadLetterQueue() {
	amb.logger.Info("Starting dead letter queue processing")

	for {
		select {
		case message := <-amb.deadLetterQueue:
			amb.processDeadLetterMessage(message)
		case <-amb.ctx.Done():
			amb.logger.Info("Dead letter queue processing stopped")
			return
		}
	}
}

// processDeadLetterMessage processes a dead letter message
func (amb *AdvancedMessageBus) processDeadLetterMessage(message *Message) {
	amb.logger.WithFields(map[string]interface{}{
		"message_id": message.ID,
		"topic":      message.Topic,
		"reason":     message.Metadata["dead_letter_reason"],
	}).Warn("Processing dead letter message")

	// In a real system, this would implement retry logic, alerting, etc.
	// For now, we just log the message
}

// =============================================================================
// METRICS AND MONITORING
// =============================================================================

// collectMetrics collects and updates metrics
func (amb *AdvancedMessageBus) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			amb.updateMetrics("collect")
		case <-amb.ctx.Done():
			return
		}
	}
}

// updateMetrics updates message bus metrics
func (amb *AdvancedMessageBus) updateMetrics(event string) {
	amb.metrics.mu.Lock()
	defer amb.metrics.mu.Unlock()

	switch event {
	case "published":
		amb.metrics.TotalMessages++
	case "processed":
		amb.metrics.ProcessedMessages++
	case "failed":
		amb.metrics.FailedMessages++
	case "dead_letter":
		amb.metrics.DeadLetterMessages++
	case "subscribed":
		amb.metrics.ActiveSubscribers++
	case "unsubscribed":
		amb.metrics.ActiveSubscribers--
	case "collect":
		amb.metrics.ActiveRoutes = len(amb.routes)
		amb.metrics.ActiveFilters = len(amb.filters)
		amb.metrics.LastUpdated = time.Now()

		// Calculate throughput
		if amb.metrics.TotalMessages > 0 {
			amb.metrics.Throughput = float64(amb.metrics.ProcessedMessages) / float64(amb.metrics.TotalMessages)
		}
	}
}

// updateLatencyMetrics updates latency metrics
func (amb *AdvancedMessageBus) updateLatencyMetrics(latency time.Duration) {
	amb.metrics.mu.Lock()
	defer amb.metrics.mu.Unlock()

	// Simple moving average
	if amb.metrics.AverageLatency == 0 {
		amb.metrics.AverageLatency = latency
	} else {
		amb.metrics.AverageLatency = (amb.metrics.AverageLatency + latency) / 2
	}
}

// GetMetrics returns current metrics
func (amb *AdvancedMessageBus) GetMetrics() *MessageMetrics {
	amb.metrics.mu.RLock()
	defer amb.metrics.mu.RUnlock()

	// Return a copy of the metrics
	return &MessageMetrics{
		TotalMessages:      amb.metrics.TotalMessages,
		ProcessedMessages:  amb.metrics.ProcessedMessages,
		FailedMessages:     amb.metrics.FailedMessages,
		DeadLetterMessages: amb.metrics.DeadLetterMessages,
		AverageLatency:     amb.metrics.AverageLatency,
		Throughput:         amb.metrics.Throughput,
		ActiveSubscribers:  amb.metrics.ActiveSubscribers,
		ActiveRoutes:       amb.metrics.ActiveRoutes,
		ActiveFilters:      amb.metrics.ActiveFilters,
		LastUpdated:        amb.metrics.LastUpdated,
	}
}

// GetStatus returns the current status of the message bus
func (amb *AdvancedMessageBus) GetStatus() map[string]interface{} {
	amb.mu.RLock()
	defer amb.mu.RUnlock()

	return map[string]interface{}{
		"started":          amb.started,
		"subscriber_count": len(amb.subscribers),
		"route_count":      len(amb.routes),
		"filter_count":     len(amb.filters),
		"queue_size":       len(amb.messageQueue),
		"dead_letter_size": len(amb.deadLetterQueue),
		"metrics":          amb.GetMetrics(),
		"timestamp":        time.Now().Format(time.RFC3339),
	}
}
