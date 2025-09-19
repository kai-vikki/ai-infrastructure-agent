package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// =============================================================================
// BASE AGENT IMPLEMENTATION
// =============================================================================

// BaseAgent provides the foundation for all specialized agents
type BaseAgent struct {
	// Agent identity
	id           string
	agentType    AgentType
	name         string
	description  string
	status       AgentStatus
	capabilities []AgentCapability
	tools        []interfaces.MCPTool
	lastSeen     time.Time
	metadata     map[string]string

	// Dependencies
	config        *config.Config
	agentConfig   *config.AgentConfig
	awsConfig     *config.AWSConfig
	awsClient     *aws.Client
	logger        *logging.Logger
	stateManager  interfaces.StateManager
	messageBus    MessageBusInterface
	agentRegistry AgentRegistryInterface

	// Communication
	messageHandler MessageHandler
	messageQueue   chan *AgentMessage

	// Metrics
	metrics *AgentMetrics

	// Thread safety
	mutex sync.RWMutex
}

// AgentMetrics contains metrics for an agent
type AgentMetrics struct {
	requestsProcessed int64
	tasksProcessed    int64
	errors           int64
	lastActivity     time.Time
	mutex            sync.RWMutex
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(id string, agentType AgentType, name string, description string, dependencies *AgentDependencies) *BaseAgent {
	return &BaseAgent{
		id:           id,
		agentType:    agentType,
		name:         name,
		description:  description,
		status:       AgentStatusInitializing,
		capabilities: make([]AgentCapability, 0),
		tools:        make([]interfaces.MCPTool, 0),
		lastSeen:     time.Now(),
		metadata:     make(map[string]string),

		// Dependencies
		config:        dependencies.Config,
		agentConfig:   dependencies.AgentConfig,
		awsConfig:     dependencies.AWSConfig,
		awsClient:     dependencies.AWSClient,
		logger:        dependencies.Logger,
		stateManager:  dependencies.StateManager,
		messageBus:    dependencies.MessageBus,
		agentRegistry: dependencies.AgentRegistry,

		// Communication
		messageHandler: nil,
		messageQueue:   make(chan *AgentMessage, 100),

		// Metrics
		metrics: &AgentMetrics{
			lastActivity: time.Now(),
		},
	}
}

// =============================================================================
// CORE LIFECYCLE METHODS
// =============================================================================

// Initialize initializes the base agent
func (ba *BaseAgent) Initialize(ctx context.Context) error {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	ba.logger.WithFields(map[string]interface{}{
		"agent_id":   ba.id,
		"agent_type": ba.agentType,
		"agent_name": ba.name,
	}).Info("Initializing base agent")

	// Set status to ready
	ba.status = AgentStatusReady
	ba.lastSeen = time.Now()

	// Start message processing
	go ba.startMessageProcessing()

	ba.logger.WithField("agent_id", ba.id).Info("Base agent initialized successfully")
	return nil
}

// Start starts the base agent
func (ba *BaseAgent) Start(ctx context.Context) error {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	if ba.status != AgentStatusReady {
		return fmt.Errorf("agent %s is not ready to start", ba.id)
	}

	ba.status = AgentStatusReady
	ba.lastSeen = time.Now()

	ba.logger.WithField("agent_id", ba.id).Info("Base agent started")
	return nil
}

// Stop stops the base agent
func (ba *BaseAgent) Stop(ctx context.Context) error {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	ba.status = AgentStatusStopped
	ba.lastSeen = time.Now()

	// Close message queue
	close(ba.messageQueue)

	ba.logger.WithField("agent_id", ba.id).Info("Base agent stopped")
	return nil
}

// GetStatus returns the current status of the agent
func (ba *BaseAgent) GetStatus() AgentStatus {
	ba.mutex.RLock()
	defer ba.mutex.RUnlock()

	return ba.status
}

// GetInfo returns information about the agent
func (ba *BaseAgent) GetInfo() AgentInfo {
	ba.mutex.RLock()
	defer ba.mutex.RUnlock()

	toolNames := make([]string, len(ba.tools))
	for i, tool := range ba.tools {
		toolNames[i] = tool.Name()
	}

	return AgentInfo{
		ID:           ba.id,
		Type:         ba.agentType,
		Name:         ba.name,
		Description:  ba.description,
		Status:       ba.status,
		Capabilities: ba.capabilities,
		Tools:        toolNames,
		LastSeen:     ba.lastSeen,
		Metadata:     ba.metadata,
	}
}

// =============================================================================
// REQUEST PROCESSING
// =============================================================================

// ProcessRequest processes a request (to be implemented by specialized agents)
func (ba *BaseAgent) ProcessRequest(ctx context.Context, request *AgentRequest) (*AgentResponse, error) {
	ba.mutex.Lock()
	ba.status = AgentStatusBusy
	ba.lastSeen = time.Now()
	ba.mutex.Unlock()

	defer func() {
		ba.mutex.Lock()
		ba.status = AgentStatusReady
		ba.lastSeen = time.Now()
		ba.mutex.Unlock()
	}()

	startTime := time.Now()

	// Update metrics
	ba.metrics.mutex.Lock()
	ba.metrics.requestsProcessed++
	ba.metrics.lastActivity = time.Now()
	ba.metrics.mutex.Unlock()

	// Create response
	response := &AgentResponse{
		ID:        uuid.New().String(),
		RequestID: request.ID,
		From:      ba.id,
		To:        request.From,
		Success:   true,
		Timestamp: time.Now(),
		Duration:  time.Since(startTime),
	}

	ba.logger.WithFields(map[string]interface{}{
		"agent_id":   ba.id,
		"request_id": request.ID,
		"duration":   response.Duration,
	}).Debug("Request processed")

	return response, nil
}

// CanHandleRequest checks if the agent can handle a specific request
func (ba *BaseAgent) CanHandleRequest(request *AgentRequest) bool {
	// Base implementation - to be overridden by specialized agents
	return false
}

// =============================================================================
// TOOL MANAGEMENT
// =============================================================================

// GetTools returns all tools available to the agent
func (ba *BaseAgent) GetTools() []interfaces.MCPTool {
	ba.mutex.RLock()
	defer ba.mutex.RUnlock()

	// Return a copy to prevent external modification
	tools := make([]interfaces.MCPTool, len(ba.tools))
	copy(tools, ba.tools)
	return tools
}

// GetToolNames returns the names of all tools available to the agent
func (ba *BaseAgent) GetToolNames() []string {
	ba.mutex.RLock()
	defer ba.mutex.RUnlock()

	toolNames := make([]string, len(ba.tools))
	for i, tool := range ba.tools {
		toolNames[i] = tool.Name()
	}
	return toolNames
}

// HasTool checks if the agent has a specific tool
func (ba *BaseAgent) HasTool(toolName string) bool {
	ba.mutex.RLock()
	defer ba.mutex.RUnlock()

	for _, tool := range ba.tools {
		if tool.Name() == toolName {
			return true
		}
	}
	return false
}

// AddTool adds a tool to the agent
func (ba *BaseAgent) AddTool(tool interfaces.MCPTool) {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	ba.tools = append(ba.tools, tool)
	ba.logger.WithFields(map[string]interface{}{
		"agent_id":  ba.id,
		"tool_name": tool.Name(),
	}).Debug("Tool added to agent")
}

// RemoveTool removes a tool from the agent
func (ba *BaseAgent) RemoveTool(toolName string) bool {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	for i, tool := range ba.tools {
		if tool.Name() == toolName {
			ba.tools = append(ba.tools[:i], ba.tools[i+1:]...)
			ba.logger.WithFields(map[string]interface{}{
				"agent_id":  ba.id,
				"tool_name": toolName,
			}).Debug("Tool removed from agent")
			return true
		}
	}
	return false
}

// =============================================================================
// COMMUNICATION
// =============================================================================

// SendMessage sends a message to another agent
func (ba *BaseAgent) SendMessage(message *AgentMessage) error {
	if ba.messageBus == nil {
		return fmt.Errorf("message bus not available")
	}

	// Set sender if not set
	if message.From == "" {
		message.From = ba.id
	}

	return ba.messageBus.Publish(message)
}

// ReceiveMessage receives a message from another agent
func (ba *BaseAgent) ReceiveMessage(message *AgentMessage) error {
	// Add to message queue for processing
	select {
	case ba.messageQueue <- message:
		return nil
	default:
		return fmt.Errorf("message queue full")
	}
}

// SetMessageHandler sets the message handler for the agent
func (ba *BaseAgent) SetMessageHandler(handler MessageHandler) {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	ba.messageHandler = handler
}

// =============================================================================
// HEALTH AND MONITORING
// =============================================================================

// HealthCheck performs a health check on the agent
func (ba *BaseAgent) HealthCheck() error {
	ba.mutex.RLock()
	defer ba.mutex.RUnlock()

	// Basic health checks
	if ba.status == AgentStatusError {
		return fmt.Errorf("agent is in error state")
	}

	if time.Since(ba.lastSeen) > 5*time.Minute {
		return fmt.Errorf("agent has not been seen for too long")
	}

	// Check message queue health
	if len(ba.messageQueue) > 90 { // 90% of capacity
		return fmt.Errorf("message queue is nearly full")
	}

	return nil
}

// GetMetrics returns metrics for the agent
func (ba *BaseAgent) GetMetrics() map[string]interface{} {
	ba.metrics.mutex.RLock()
	defer ba.metrics.mutex.RUnlock()

	return map[string]interface{}{
		"requests_processed": ba.metrics.requestsProcessed,
		"tasks_processed":    ba.metrics.tasksProcessed,
		"errors":            ba.metrics.errors,
		"last_activity":     ba.metrics.lastActivity,
		"message_queue_size": len(ba.messageQueue),
		"status":            ba.GetStatus(),
		"tools_count":       len(ba.tools),
		"capabilities_count": len(ba.capabilities),
	}
}

// =============================================================================
// PRIVATE METHODS
// =============================================================================

// startMessageProcessing starts the message processing goroutine
func (ba *BaseAgent) startMessageProcessing() {
	for message := range ba.messageQueue {
		ba.processMessage(message)
	}
}

// processMessage processes an incoming message
func (ba *BaseAgent) processMessage(message *AgentMessage) {
	if ba.messageHandler == nil {
		ba.logger.WithField("agent_id", ba.id).Warn("No message handler set, ignoring message")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ba.messageHandler.HandleMessage(ctx, message); err != nil {
		ba.logger.WithError(err).WithFields(map[string]interface{}{
			"agent_id":     ba.id,
			"message_id":   message.ID,
			"message_type": message.MessageType,
		}).Error("Failed to process message")

		// Update error metrics
		ba.metrics.mutex.Lock()
		ba.metrics.errors++
		ba.metrics.mutex.Unlock()
	}
}

// =============================================================================
// UTILITY METHODS
// =============================================================================

// SetCapability adds a capability to the agent
func (ba *BaseAgent) SetCapability(capability AgentCapability) {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	// Check if capability already exists
	for i, existing := range ba.capabilities {
		if existing.Name == capability.Name {
			ba.capabilities[i] = capability
			return
		}
	}

	// Add new capability
	ba.capabilities = append(ba.capabilities, capability)
}

// RemoveCapability removes a capability from the agent
func (ba *BaseAgent) RemoveCapability(capabilityName string) bool {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	for i, capability := range ba.capabilities {
		if capability.Name == capabilityName {
			ba.capabilities = append(ba.capabilities[:i], ba.capabilities[i+1:]...)
			return true
		}
	}
	return false
}

// SetMetadata sets metadata for the agent
func (ba *BaseAgent) SetMetadata(key, value string) {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	ba.metadata[key] = value
}

// GetMetadata gets metadata for the agent
func (ba *BaseAgent) GetMetadata(key string) (string, bool) {
	ba.mutex.RLock()
	defer ba.mutex.RUnlock()

	value, exists := ba.metadata[key]
	return value, exists
}

// UpdateLastSeen updates the last seen timestamp
func (ba *BaseAgent) UpdateLastSeen() {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	ba.lastSeen = time.Now()
}
