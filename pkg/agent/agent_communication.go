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
// AGENT COMMUNICATION PROTOCOLS
// =============================================================================

// AgentCommunicationManager manages communication between agents
type AgentCommunicationManager struct {
	messageBus       *AdvancedMessageBus
	agentRegistry    AgentRegistry
	communicationLog *CommunicationLog
	logger           *logging.Logger
	mu               sync.RWMutex
	activeSessions   map[string]*CommunicationSession
	protocols        map[CommunicationProtocol]*ProtocolHandler
}

// CommunicationSession represents an active communication session
type CommunicationSession struct {
	ID           string                 `json:"id"`
	Participants []string               `json:"participants"`
	Protocol     CommunicationProtocol  `json:"protocol"`
	Status       SessionStatus          `json:"status"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActivity time.Time              `json:"last_activity"`
	Messages     []*Message             `json:"messages"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ProtocolHandler handles a specific communication protocol
type ProtocolHandler struct {
	Protocol     CommunicationProtocol `json:"protocol"`
	Handler      func(ctx context.Context, session *CommunicationSession, message *Message) error
	Description  string                 `json:"description"`
	Capabilities []string               `json:"capabilities"`
}

// CommunicationLog logs all communication activities
type CommunicationLog struct {
	Entries []*CommunicationLogEntry `json:"entries"`
	mu      sync.RWMutex
}

// CommunicationLogEntry represents a log entry
type CommunicationLogEntry struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	AgentID   string                 `json:"agent_id"`
	Action    string                 `json:"action"`
	Message   *Message               `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// CommunicationProtocol represents a communication protocol
type CommunicationProtocol string

const (
	ProtocolRequestResponse CommunicationProtocol = "request_response"
	ProtocolPublishSubscribe CommunicationProtocol = "publish_subscribe"
	ProtocolEventDriven     CommunicationProtocol = "event_driven"
	ProtocolStreaming       CommunicationProtocol = "streaming"
	ProtocolBatch           CommunicationProtocol = "batch"
	ProtocolBroadcast       CommunicationProtocol = "broadcast"
)

// SessionStatus represents the status of a communication session
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// NewAgentCommunicationManager creates a new agent communication manager
func NewAgentCommunicationManager(messageBus *AdvancedMessageBus, agentRegistry AgentRegistry, logger *logging.Logger) *AgentCommunicationManager {
	manager := &AgentCommunicationManager{
		messageBus:       messageBus,
		agentRegistry:    agentRegistry,
		communicationLog: &CommunicationLog{},
		logger:           logger,
		activeSessions:   make(map[string]*CommunicationSession),
		protocols:        make(map[CommunicationProtocol]*ProtocolHandler),
	}

	// Register default protocols
	manager.registerDefaultProtocols()

	return manager
}

// =============================================================================
// COMMUNICATION SESSION MANAGEMENT
// =============================================================================

// StartSession starts a new communication session
func (acm *AgentCommunicationManager) StartSession(ctx context.Context, participants []string, protocol CommunicationProtocol) (*CommunicationSession, error) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	// Validate participants
	for _, participantID := range participants {
		agent, err := acm.agentRegistry.GetAgent(participantID)
		if err != nil {
			return nil, fmt.Errorf("participant %s not found: %w", participantID, err)
		}
		if agent == nil {
			return nil, fmt.Errorf("participant %s is nil", participantID)
		}
	}

	// Validate protocol
	protocolHandler, exists := acm.protocols[protocol]
	if !exists {
		return nil, fmt.Errorf("protocol %s not supported", protocol)
	}

	// Create session
	session := &CommunicationSession{
		ID:           uuid.New().String(),
		Participants: participants,
		Protocol:     protocol,
		Status:       SessionStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Messages:     make([]*Message, 0),
		Metadata:     make(map[string]interface{}),
	}

	// Store session
	acm.activeSessions[session.ID] = session

	// Log session start
	acm.logCommunication(session.ID, participants[0], "session_started", nil, map[string]interface{}{
		"protocol":     protocol,
		"participants": participants,
	})

	acm.logger.WithFields(map[string]interface{}{
		"session_id":   session.ID,
		"protocol":     protocol,
		"participants": participants,
	}).Info("Communication session started")

	return session, nil
}

// EndSession ends a communication session
func (acm *AgentCommunicationManager) EndSession(ctx context.Context, sessionID string, reason string) error {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	session, exists := acm.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Update session status
	session.Status = SessionStatusCompleted
	session.LastActivity = time.Now()
	session.Metadata["end_reason"] = reason

	// Log session end
	acm.logCommunication(sessionID, session.Participants[0], "session_ended", nil, map[string]interface{}{
		"reason": reason,
	})

	// Remove from active sessions
	delete(acm.activeSessions, sessionID)

	acm.logger.WithFields(map[string]interface{}{
		"session_id": sessionID,
		"reason":     reason,
	}).Info("Communication session ended")

	return nil
}

// GetSession gets a communication session
func (acm *AgentCommunicationManager) GetSession(sessionID string) (*CommunicationSession, error) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	session, exists := acm.activeSessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// ListSessions lists all active communication sessions
func (acm *AgentCommunicationManager) ListSessions() []*CommunicationSession {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	sessions := make([]*CommunicationSession, 0, len(acm.activeSessions))
	for _, session := range acm.activeSessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// =============================================================================
// MESSAGE SENDING AND RECEIVING
// =============================================================================

// SendMessage sends a message in a communication session
func (acm *AgentCommunicationManager) SendMessage(ctx context.Context, sessionID string, fromAgentID string, toAgentID string, content interface{}, messageType string) (*Message, error) {
	acm.mu.RLock()
	session, exists := acm.activeSessions[sessionID]
	acm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Validate participants
	if !acm.isParticipant(session, fromAgentID) || !acm.isParticipant(session, toAgentID) {
		return nil, fmt.Errorf("agent not a participant in session %s", sessionID)
	}

	// Create message
	message := &Message{
		ID:        uuid.New().String(),
		Topic:     fmt.Sprintf("session_%s", sessionID),
		Type:      messageType,
		From:      fromAgentID,
		To:        toAgentID,
		Content:   content,
		Priority:  MessagePriorityNormal,
		Status:    MessageStatusPending,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"session_id": sessionID,
			"protocol":   session.Protocol,
		},
	}

	// Add message to session
	acm.mu.Lock()
	session.Messages = append(session.Messages, message)
	session.LastActivity = time.Now()
	acm.mu.Unlock()

	// Log message
	acm.logCommunication(sessionID, fromAgentID, "message_sent", message, nil)

	// Send message via message bus
	if err := acm.messageBus.Publish(ctx, message); err != nil {
		acm.logger.WithError(err).WithFields(map[string]interface{}{
			"session_id": sessionID,
			"message_id": message.ID,
		}).Error("Failed to publish message")
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	acm.logger.WithFields(map[string]interface{}{
		"session_id": sessionID,
		"message_id": message.ID,
		"from":       fromAgentID,
		"to":         toAgentID,
		"type":       messageType,
	}).Debug("Message sent successfully")

	return message, nil
}

// SendBroadcastMessage sends a broadcast message to all participants
func (acm *AgentCommunicationManager) SendBroadcastMessage(ctx context.Context, sessionID string, fromAgentID string, content interface{}, messageType string) ([]*Message, error) {
	acm.mu.RLock()
	session, exists := acm.activeSessions[sessionID]
	acm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Validate participant
	if !acm.isParticipant(session, fromAgentID) {
		return nil, fmt.Errorf("agent not a participant in session %s", sessionID)
	}

	var messages []*Message

	// Send message to all other participants
	for _, participantID := range session.Participants {
		if participantID != fromAgentID {
			message, err := acm.SendMessage(ctx, sessionID, fromAgentID, participantID, content, messageType)
			if err != nil {
				acm.logger.WithError(err).WithFields(map[string]interface{}{
					"session_id": sessionID,
					"from":       fromAgentID,
					"to":         participantID,
				}).Error("Failed to send broadcast message")
				continue
			}
			messages = append(messages, message)
		}
	}

	acm.logger.WithFields(map[string]interface{}{
		"session_id": sessionID,
		"from":       fromAgentID,
		"message_count": len(messages),
	}).Info("Broadcast message sent successfully")

	return messages, nil
}

// SendRequest sends a request and waits for a response
func (acm *AgentCommunicationManager) SendRequest(ctx context.Context, sessionID string, fromAgentID string, toAgentID string, request interface{}, timeout time.Duration) (*Message, error) {
	// Create request message
	requestMessage, err := acm.SendMessage(ctx, sessionID, fromAgentID, toAgentID, request, "request")
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	responseChannel := make(chan *Message, 1)
	errorChannel := make(chan error, 1)

	// Set up response handler
	responseHandler := func(ctx context.Context, message *Message) error {
		if message.Metadata["request_id"] == requestMessage.ID {
			select {
			case responseChannel <- message:
			default:
			}
		}
		return nil
	}

	// Subscribe to responses
	subscriber := &Subscriber{
		ID:        uuid.New().String(),
		AgentID:   fromAgentID,
		Topics:    []string{fmt.Sprintf("session_%s", sessionID)},
		Handler:   responseHandler,
		Priority:  1,
		Active:    true,
		CreatedAt: time.Now(),
	}

	if err := acm.messageBus.Subscribe(ctx, subscriber); err != nil {
		return nil, fmt.Errorf("failed to subscribe to responses: %w", err)
	}

	// Set up timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for response or timeout
	select {
	case response := <-responseChannel:
		// Unsubscribe
		acm.messageBus.Unsubscribe(ctx, subscriber.ID)
		return response, nil
	case <-timeoutCtx.Done():
		// Unsubscribe
		acm.messageBus.Unsubscribe(ctx, subscriber.ID)
		return nil, fmt.Errorf("request timeout after %v", timeout)
	case err := <-errorChannel:
		// Unsubscribe
		acm.messageBus.Unsubscribe(ctx, subscriber.ID)
		return nil, err
	}
}

// =============================================================================
// PROTOCOL HANDLERS
// =============================================================================

// registerDefaultProtocols registers default communication protocols
func (acm *AgentCommunicationManager) registerDefaultProtocols() {
	// Request-Response Protocol
	acm.protocols[ProtocolRequestResponse] = &ProtocolHandler{
		Protocol:    ProtocolRequestResponse,
		Description: "Request-response communication pattern",
		Capabilities: []string{"request", "response", "timeout", "retry"},
		Handler:     acm.handleRequestResponse,
	}

	// Publish-Subscribe Protocol
	acm.protocols[ProtocolPublishSubscribe] = &ProtocolHandler{
		Protocol:    ProtocolPublishSubscribe,
		Description: "Publish-subscribe communication pattern",
		Capabilities: []string{"publish", "subscribe", "unsubscribe", "filter"},
		Handler:     acm.handlePublishSubscribe,
	}

	// Event-Driven Protocol
	acm.protocols[ProtocolEventDriven] = &ProtocolHandler{
		Protocol:    ProtocolEventDriven,
		Description: "Event-driven communication pattern",
		Capabilities: []string{"event", "listener", "trigger", "propagation"},
		Handler:     acm.handleEventDriven,
	}

	// Streaming Protocol
	acm.protocols[ProtocolStreaming] = &ProtocolHandler{
		Protocol:    ProtocolStreaming,
		Description: "Streaming communication pattern",
		Capabilities: []string{"stream", "chunk", "flow_control", "backpressure"},
		Handler:     acm.handleStreaming,
	}

	// Batch Protocol
	acm.protocols[ProtocolBatch] = &ProtocolHandler{
		Protocol:    ProtocolBatch,
		Description: "Batch communication pattern",
		Capabilities: []string{"batch", "aggregate", "flush", "size_limit"},
		Handler:     acm.handleBatch,
	}

	// Broadcast Protocol
	acm.protocols[ProtocolBroadcast] = &ProtocolHandler{
		Protocol:    ProtocolBroadcast,
		Description: "Broadcast communication pattern",
		Capabilities: []string{"broadcast", "multicast", "fanout", "replication"},
		Handler:     acm.handleBroadcast,
	}
}

// handleRequestResponse handles request-response protocol
func (acm *AgentCommunicationManager) handleRequestResponse(ctx context.Context, session *CommunicationSession, message *Message) error {
	acm.logger.WithFields(map[string]interface{}{
		"session_id": session.ID,
		"message_id": message.ID,
		"type":       message.Type,
	}).Debug("Handling request-response message")

	// Handle request
	if message.Type == "request" {
		// Find target agent
		targetAgent, err := acm.agentRegistry.GetAgent(message.To)
		if err != nil {
			return fmt.Errorf("target agent not found: %w", err)
		}

		// Process request
		response, err := acm.processRequest(ctx, targetAgent, message)
		if err != nil {
			return fmt.Errorf("failed to process request: %w", err)
		}

		// Send response
		responseMessage, err := acm.SendMessage(ctx, session.ID, message.To, message.From, response, "response")
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}

		acm.logger.WithFields(map[string]interface{}{
			"session_id": session.ID,
			"request_id": message.ID,
			"response_id": responseMessage.ID,
		}).Debug("Request-response completed")
	}

	return nil
}

// handlePublishSubscribe handles publish-subscribe protocol
func (acm *AgentCommunicationManager) handlePublishSubscribe(ctx context.Context, session *CommunicationSession, message *Message) error {
	acm.logger.WithFields(map[string]interface{}{
		"session_id": session.ID,
		"message_id": message.ID,
		"type":       message.Type,
	}).Debug("Handling publish-subscribe message")

	// Handle publish
	if message.Type == "publish" {
		// Broadcast to all subscribers
		_, err := acm.SendBroadcastMessage(ctx, session.ID, message.From, message.Content, "notification")
		if err != nil {
			return fmt.Errorf("failed to broadcast message: %w", err)
		}
	}

	return nil
}

// handleEventDriven handles event-driven protocol
func (acm *AgentCommunicationManager) handleEventDriven(ctx context.Context, session *CommunicationSession, message *Message) error {
	acm.logger.WithFields(map[string]interface{}{
		"session_id": session.ID,
		"message_id": message.ID,
		"type":       message.Type,
	}).Debug("Handling event-driven message")

	// Handle event
	if message.Type == "event" {
		// Trigger event handlers for all participants
		for _, participantID := range session.Participants {
			if participantID != message.From {
				// Send event notification
				_, err := acm.SendMessage(ctx, session.ID, message.From, participantID, message.Content, "event_notification")
				if err != nil {
					acm.logger.WithError(err).WithFields(map[string]interface{}{
						"session_id": session.ID,
						"participant": participantID,
					}).Error("Failed to send event notification")
				}
			}
		}
	}

	return nil
}

// handleStreaming handles streaming protocol
func (acm *AgentCommunicationManager) handleStreaming(ctx context.Context, session *CommunicationSession, message *Message) error {
	acm.logger.WithFields(map[string]interface{}{
		"session_id": session.ID,
		"message_id": message.ID,
		"type":       message.Type,
	}).Debug("Handling streaming message")

	// Handle stream chunk
	if message.Type == "stream_chunk" {
		// Forward chunk to target
		_, err := acm.SendMessage(ctx, session.ID, message.From, message.To, message.Content, "stream_chunk")
		if err != nil {
			return fmt.Errorf("failed to forward stream chunk: %w", err)
		}
	}

	return nil
}

// handleBatch handles batch protocol
func (acm *AgentCommunicationManager) handleBatch(ctx context.Context, session *CommunicationSession, message *Message) error {
	acm.logger.WithFields(map[string]interface{}{
		"session_id": session.ID,
		"message_id": message.ID,
		"type":       message.Type,
	}).Debug("Handling batch message")

	// Handle batch
	if message.Type == "batch" {
		// Process batch
		batchData, ok := message.Content.([]interface{})
		if !ok {
			return fmt.Errorf("invalid batch data format")
		}

		// Process each item in batch
		for _, item := range batchData {
			_, err := acm.SendMessage(ctx, session.ID, message.From, message.To, item, "batch_item")
			if err != nil {
				acm.logger.WithError(err).WithFields(map[string]interface{}{
					"session_id": session.ID,
					"item":       item,
				}).Error("Failed to process batch item")
			}
		}
	}

	return nil
}

// handleBroadcast handles broadcast protocol
func (acm *AgentCommunicationManager) handleBroadcast(ctx context.Context, session *CommunicationSession, message *Message) error {
	acm.logger.WithFields(map[string]interface{}{
		"session_id": session.ID,
		"message_id": message.ID,
		"type":       message.Type,
	}).Debug("Handling broadcast message")

	// Handle broadcast
	if message.Type == "broadcast" {
		// Broadcast to all participants
		_, err := acm.SendBroadcastMessage(ctx, session.ID, message.From, message.Content, "broadcast")
		if err != nil {
			return fmt.Errorf("failed to broadcast message: %w", err)
		}
	}

	return nil
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// isParticipant checks if an agent is a participant in a session
func (acm *AgentCommunicationManager) isParticipant(session *CommunicationSession, agentID string) bool {
	for _, participantID := range session.Participants {
		if participantID == agentID {
			return true
		}
	}
	return false
}

// processRequest processes a request from an agent
func (acm *AgentCommunicationManager) processRequest(ctx context.Context, agent Agent, request *Message) (interface{}, error) {
	// This is a simplified implementation
	// In a real system, this would call the agent's request processing method
	
	// For now, return a simple acknowledgment
	return map[string]interface{}{
		"status":    "processed",
		"agent_id":  agent.ID(),
		"timestamp": time.Now().Format(time.RFC3339),
		"request_id": request.ID,
	}, nil
}

// logCommunication logs communication activities
func (acm *AgentCommunicationManager) logCommunication(sessionID string, agentID string, action string, message *Message, metadata map[string]interface{}) {
	entry := &CommunicationLogEntry{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		AgentID:   agentID,
		Action:    action,
		Message:   message,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	acm.communicationLog.mu.Lock()
	acm.communicationLog.Entries = append(acm.communicationLog.Entries, entry)
	acm.communicationLog.mu.Unlock()
}

// GetCommunicationLog returns the communication log
func (acm *AgentCommunicationManager) GetCommunicationLog() []*CommunicationLogEntry {
	acm.communicationLog.mu.RLock()
	defer acm.communicationLog.mu.RUnlock()

	// Return a copy of the log entries
	entries := make([]*CommunicationLogEntry, len(acm.communicationLog.Entries))
	copy(entries, acm.communicationLog.Entries)

	return entries
}

// GetSupportedProtocols returns the list of supported communication protocols
func (acm *AgentCommunicationManager) GetSupportedProtocols() []*ProtocolHandler {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	protocols := make([]*ProtocolHandler, 0, len(acm.protocols))
	for _, protocol := range acm.protocols {
		protocols = append(protocols, protocol)
	}

	return protocols
}

// GetStatus returns the current status of the communication manager
func (acm *AgentCommunicationManager) GetStatus() map[string]interface{} {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	return map[string]interface{}{
		"active_sessions":    len(acm.activeSessions),
		"supported_protocols": len(acm.protocols),
		"log_entries":       len(acm.communicationLog.Entries),
		"timestamp":         time.Now().Format(time.RFC3339),
	}
}
