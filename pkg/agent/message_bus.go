package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// MESSAGE BUS IMPLEMENTATION
// =============================================================================

// MessageBusImpl implements the MessageBusInterface
type MessageBusImpl struct {
	// Message routing
	subscribers    map[string]MessageHandler
	messageQueues  map[string][]*AgentMessage
	messageHistory map[string][]*AgentMessage

	// Configuration
	maxQueueSize    int
	maxHistorySize  int
	messageTTL      time.Duration
	cleanupInterval time.Duration

	// State management
	mutex    sync.RWMutex
	running  bool
	stopChan chan struct{}
	logger   *logging.Logger

	// Statistics
	stats      *MessageBusStats
	statsMutex sync.RWMutex
}

// MessageBusStats contains statistics about the message bus
type MessageBusStats struct {
	MessagesSent      int64          `json:"messagesSent"`
	MessagesReceived  int64          `json:"messagesReceived"`
	MessagesDropped   int64          `json:"messagesDropped"`
	ActiveSubscribers int            `json:"activeSubscribers"`
	QueueSizes        map[string]int `json:"queueSizes"`
	LastActivity      time.Time      `json:"lastActivity"`
}

// NewMessageBus creates a new message bus instance
func NewMessageBus(logger *logging.Logger) MessageBusInterface {
	bus := &MessageBusImpl{
		subscribers:     make(map[string]MessageHandler),
		messageQueues:   make(map[string][]*AgentMessage),
		messageHistory:  make(map[string][]*AgentMessage),
		maxQueueSize:    1000,
		maxHistorySize:  100,
		messageTTL:      5 * time.Minute,
		cleanupInterval: 1 * time.Minute,
		stopChan:        make(chan struct{}),
		logger:          logger,
		stats: &MessageBusStats{
			QueueSizes: make(map[string]int),
		},
	}

	return bus
}

// Publish publishes a message to the message bus
func (mb *MessageBusImpl) Publish(message *AgentMessage) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	// Validate message
	if err := mb.validateMessage(message); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Set message ID if not set
	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	// Set timestamp if not set
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Route the message
	if err := mb.routeMessageInternal(message); err != nil {
		mb.statsMutex.Lock()
		mb.stats.MessagesDropped++
		mb.statsMutex.Unlock()
		return fmt.Errorf("failed to route message: %w", err)
	}

	// Update statistics
	mb.statsMutex.Lock()
	mb.stats.MessagesSent++
	mb.stats.LastActivity = time.Now()
	mb.statsMutex.Unlock()

	mb.logger.WithFields(map[string]interface{}{
		"message_id":   message.ID,
		"from":         message.From,
		"to":           message.To,
		"message_type": message.MessageType,
	}).Debug("Message published")

	return nil
}

// Subscribe subscribes an agent to receive messages
func (mb *MessageBusImpl) Subscribe(agentID string, handler MessageHandler) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if agentID == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}

	if handler == nil {
		return fmt.Errorf("message handler cannot be nil")
	}

	// Check if already subscribed
	if _, exists := mb.subscribers[agentID]; exists {
		return fmt.Errorf("agent %s already subscribed", agentID)
	}

	// Subscribe the agent
	mb.subscribers[agentID] = handler
	mb.messageQueues[agentID] = make([]*AgentMessage, 0)
	mb.messageHistory[agentID] = make([]*AgentMessage, 0)

	// Update statistics
	mb.statsMutex.Lock()
	mb.stats.ActiveSubscribers = len(mb.subscribers)
	mb.statsMutex.Unlock()

	mb.logger.WithField("agent_id", agentID).Info("Agent subscribed to message bus")

	return nil
}

// Unsubscribe unsubscribes an agent from the message bus
func (mb *MessageBusImpl) Unsubscribe(agentID string) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if _, exists := mb.subscribers[agentID]; !exists {
		return fmt.Errorf("agent %s not subscribed", agentID)
	}

	// Unsubscribe the agent
	delete(mb.subscribers, agentID)
	delete(mb.messageQueues, agentID)
	delete(mb.messageHistory, agentID)

	// Update statistics
	mb.statsMutex.Lock()
	mb.stats.ActiveSubscribers = len(mb.subscribers)
	delete(mb.stats.QueueSizes, agentID)
	mb.statsMutex.Unlock()

	mb.logger.WithField("agent_id", agentID).Info("Agent unsubscribed from message bus")

	return nil
}

// RouteMessage routes a message to the appropriate agent
func (mb *MessageBusImpl) RouteMessage(message *AgentMessage) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	return mb.routeMessageInternal(message)
}

// Broadcast broadcasts a message to multiple agents by type
func (mb *MessageBusImpl) Broadcast(message *AgentMessage, agentTypes []AgentType) error {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	// Create a copy of the message for each recipient
	for agentID, handler := range mb.subscribers {
		// Check if agent type matches (this would need agent registry integration)
		// For now, we'll broadcast to all agents
		broadcastMessage := &AgentMessage{
			ID:          uuid.New().String(),
			From:        message.From,
			To:          agentID,
			MessageType: message.MessageType,
			Content:     message.Content,
			Priority:    message.Priority,
			Timestamp:   time.Now(),
			TTL:         message.TTL,
		}

		// Add to agent's message queue
		if queue, exists := mb.messageQueues[agentID]; exists {
			if len(queue) < mb.maxQueueSize {
				mb.messageQueues[agentID] = append(queue, broadcastMessage)

				// Update queue size statistics
				mb.statsMutex.Lock()
				mb.stats.QueueSizes[agentID] = len(mb.messageQueues[agentID])
				mb.statsMutex.Unlock()
			} else {
				mb.logger.WithField("agent_id", agentID).Warn("Message queue full, dropping message")
				mb.statsMutex.Lock()
				mb.stats.MessagesDropped++
				mb.statsMutex.Unlock()
			}
		}

		// Process message asynchronously
		go mb.processMessage(agentID, broadcastMessage, handler)
	}

	return nil
}

// GetMessageQueue returns the message queue for a specific agent
func (mb *MessageBusImpl) GetMessageQueue(agentID string) ([]*AgentMessage, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	queue, exists := mb.messageQueues[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	// Return a copy to prevent external modification
	result := make([]*AgentMessage, len(queue))
	copy(result, queue)
	return result, nil
}

// ClearMessageQueue clears the message queue for a specific agent
func (mb *MessageBusImpl) ClearMessageQueue(agentID string) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if _, exists := mb.messageQueues[agentID]; !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	mb.messageQueues[agentID] = make([]*AgentMessage, 0)

	// Update statistics
	mb.statsMutex.Lock()
	mb.stats.QueueSizes[agentID] = 0
	mb.statsMutex.Unlock()

	mb.logger.WithField("agent_id", agentID).Info("Message queue cleared")

	return nil
}

// Start starts the message bus
func (mb *MessageBusImpl) Start(ctx context.Context) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if mb.running {
		return fmt.Errorf("message bus already running")
	}

	mb.running = true
	mb.stopChan = make(chan struct{})

	// Start cleanup goroutine
	go mb.startCleanupRoutine()

	mb.logger.Info("Message bus started")
	return nil
}

// Stop stops the message bus
func (mb *MessageBusImpl) Stop(ctx context.Context) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if !mb.running {
		return fmt.Errorf("message bus not running")
	}

	mb.running = false
	close(mb.stopChan)

	mb.logger.Info("Message bus stopped")
	return nil
}

// GetStatus returns the current status of the message bus
func (mb *MessageBusImpl) GetStatus() string {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	if mb.running {
		return "running"
	}
	return "stopped"
}

// =============================================================================
// PRIVATE METHODS
// =============================================================================

// validateMessage validates a message before processing
func (mb *MessageBusImpl) validateMessage(message *AgentMessage) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	if message.From == "" {
		return fmt.Errorf("message sender cannot be empty")
	}

	if message.To == "" {
		return fmt.Errorf("message recipient cannot be empty")
	}

	if message.MessageType == "" {
		return fmt.Errorf("message type cannot be empty")
	}

	return nil
}

// routeMessageInternal routes a message to the appropriate agent (internal method)
func (mb *MessageBusImpl) routeMessageInternal(message *AgentMessage) error {
	// Handle broadcast messages (empty To means broadcast)
	if message.To == "" {
		return mb.Broadcast(message, nil)
	}

	// Check if recipient is subscribed
	handler, exists := mb.subscribers[message.To]
	if !exists {
		return fmt.Errorf("recipient %s not subscribed", message.To)
	}

	// Add to agent's message queue
	if queue, exists := mb.messageQueues[message.To]; exists {
		if len(queue) < mb.maxQueueSize {
			mb.messageQueues[message.To] = append(queue, message)

			// Update queue size statistics
			mb.statsMutex.Lock()
			mb.stats.QueueSizes[message.To] = len(mb.messageQueues[message.To])
			mb.statsMutex.Unlock()
		} else {
			mb.logger.WithField("agent_id", message.To).Warn("Message queue full, dropping message")
			mb.statsMutex.Lock()
			mb.stats.MessagesDropped++
			mb.statsMutex.Unlock()
			return fmt.Errorf("message queue full for agent %s", message.To)
		}
	}

	// Process message asynchronously
	go mb.processMessage(message.To, message, handler)

	return nil
}

// processMessage processes a message for a specific agent
func (mb *MessageBusImpl) processMessage(agentID string, message *AgentMessage, handler MessageHandler) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Process the message
	if err := handler.HandleMessage(ctx, message); err != nil {
		mb.logger.WithError(err).WithFields(map[string]interface{}{
			"agent_id":     agentID,
			"message_id":   message.ID,
			"message_type": message.MessageType,
		}).Error("Failed to process message")
	} else {
		mb.statsMutex.Lock()
		mb.stats.MessagesReceived++
		mb.statsMutex.Unlock()
	}

	// Remove message from queue
	mb.mutex.Lock()
	if queue, exists := mb.messageQueues[agentID]; exists {
		for i, msg := range queue {
			if msg.ID == message.ID {
				mb.messageQueues[agentID] = append(queue[:i], queue[i+1:]...)
				break
			}
		}

		// Update queue size statistics
		mb.statsMutex.Lock()
		mb.stats.QueueSizes[agentID] = len(mb.messageQueues[agentID])
		mb.statsMutex.Unlock()
	}
	mb.mutex.Unlock()

	// Add to message history
	mb.addToHistory(agentID, message)
}

// addToHistory adds a message to the agent's message history
func (mb *MessageBusImpl) addToHistory(agentID string, message *AgentMessage) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if history, exists := mb.messageHistory[agentID]; exists {
		// Add to history
		mb.messageHistory[agentID] = append(history, message)

		// Trim history if it exceeds max size
		if len(mb.messageHistory[agentID]) > mb.maxHistorySize {
			mb.messageHistory[agentID] = mb.messageHistory[agentID][len(mb.messageHistory[agentID])-mb.maxHistorySize:]
		}
	}
}

// startCleanupRoutine starts the cleanup routine for expired messages
func (mb *MessageBusImpl) startCleanupRoutine() {
	ticker := time.NewTicker(mb.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mb.cleanupExpiredMessages()
		case <-mb.stopChan:
			return
		}
	}
}

// cleanupExpiredMessages removes expired messages from queues and history
func (mb *MessageBusImpl) cleanupExpiredMessages() {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	now := time.Now()
	expiredCount := 0

	// Clean up message queues
	for agentID, queue := range mb.messageQueues {
		var validMessages []*AgentMessage
		for _, message := range queue {
			if message.TTL > 0 && now.Sub(message.Timestamp) > message.TTL {
				expiredCount++
			} else {
				validMessages = append(validMessages, message)
			}
		}
		mb.messageQueues[agentID] = validMessages

		// Update queue size statistics
		mb.statsMutex.Lock()
		mb.stats.QueueSizes[agentID] = len(validMessages)
		mb.statsMutex.Unlock()
	}

	// Clean up message history
	for agentID, history := range mb.messageHistory {
		var validMessages []*AgentMessage
		for _, message := range history {
			if message.TTL > 0 && now.Sub(message.Timestamp) > message.TTL {
				expiredCount++
			} else {
				validMessages = append(validMessages, message)
			}
		}
		mb.messageHistory[agentID] = validMessages
	}

	if expiredCount > 0 {
		mb.logger.WithField("expired_messages", expiredCount).Debug("Cleaned up expired messages")
	}
}

// =============================================================================
// MESSAGE BUS STATISTICS
// =============================================================================

// GetStatistics returns statistics about the message bus
func (mb *MessageBusImpl) GetStatistics() *MessageBusStats {
	mb.statsMutex.RLock()
	defer mb.statsMutex.RUnlock()

	// Return a copy to prevent external modification
	stats := &MessageBusStats{
		MessagesSent:      mb.stats.MessagesSent,
		MessagesReceived:  mb.stats.MessagesReceived,
		MessagesDropped:   mb.stats.MessagesDropped,
		ActiveSubscribers: mb.stats.ActiveSubscribers,
		QueueSizes:        make(map[string]int),
		LastActivity:      mb.stats.LastActivity,
	}

	// Copy queue sizes
	for agentID, size := range mb.stats.QueueSizes {
		stats.QueueSizes[agentID] = size
	}

	return stats
}

// ResetStatistics resets the message bus statistics
func (mb *MessageBusImpl) ResetStatistics() {
	mb.statsMutex.Lock()
	defer mb.statsMutex.Unlock()

	mb.stats = &MessageBusStats{
		QueueSizes: make(map[string]int),
	}
}
