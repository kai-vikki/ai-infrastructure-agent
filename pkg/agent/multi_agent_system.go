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
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// =============================================================================
// MULTI-AGENT SYSTEM MANAGER
// =============================================================================

// MultiAgentSystem manages the entire multi-agent infrastructure
type MultiAgentSystem struct {
	// Core components
	config       *config.Config
	awsClient    *aws.Client
	logger       *logging.Logger
	stateManager interfaces.StateManager

	// Multi-agent components
	agentFactory  AgentFactoryInterface
	agentRegistry AgentRegistryInterface
	messageBus    MessageBusInterface
	coordinator   CoordinatorAgentInterface

	// System state
	running       bool
	agents        map[string]SpecializedAgentInterface
	activeTasks   map[string]*Task
	systemMetrics *SystemMetrics

	// Thread safety
	mutex sync.RWMutex
}

// SystemMetrics contains metrics for the entire multi-agent system
type SystemMetrics struct {
	TotalAgents       int                    `json:"totalAgents"`
	ActiveAgents      int                    `json:"activeAgents"`
	TotalTasks        int                    `json:"totalTasks"`
	CompletedTasks    int                    `json:"completedTasks"`
	FailedTasks       int                    `json:"failedTasks"`
	MessagesProcessed int64                  `json:"messagesProcessed"`
	SystemUptime      time.Duration          `json:"systemUptime"`
	AgentMetrics      map[string]interface{} `json:"agentMetrics"`
	LastActivity      time.Time              `json:"lastActivity"`
	mutex             sync.RWMutex
}

// NewMultiAgentSystem creates a new multi-agent system
func NewMultiAgentSystem(config *config.Config, awsClient *aws.Client, logger *logging.Logger) (*MultiAgentSystem, error) {
	// Initialize state manager
    stateManager := masMakeStateManager(config, logger)

	// Initialize multi-agent components
	agentFactory := NewAgentFactory(config, logger)
	agentRegistry := NewAgentRegistry(logger)
	messageBus := NewMessageBus(logger)

	// Create system
	system := &MultiAgentSystem{
		config:        config,
		awsClient:     awsClient,
		logger:        logger,
		stateManager:  stateManager,
		agentFactory:  agentFactory,
		agentRegistry: agentRegistry,
		messageBus:    messageBus,
		agents:        make(map[string]SpecializedAgentInterface),
		activeTasks:   make(map[string]*Task),
		systemMetrics: &SystemMetrics{
			AgentMetrics: make(map[string]interface{}),
		},
	}

	return system, nil
}

// masMakeStateManager provides a minimal state manager placeholder to satisfy interfaces.StateManager
func masMakeStateManager(cfg *config.Config, logger *logging.Logger) interfaces.StateManager {
    // Use an in-memory state manager to satisfy dependencies by default
    return NewInMemoryStateManager()
}

// MessageHandlerFunc is an adapter to allow the use of
// ordinary functions as MessageHandler.
type MessageHandlerFunc func(ctx context.Context, message *AgentMessage) error

// HandleMessage calls f(ctx, message).
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, message *AgentMessage) error {
	return f(ctx, message)
}

// makeAgentMessageHandler wraps a SpecializedAgentInterface to handle messages
func (mas *MultiAgentSystem) makeAgentMessageHandler(agent SpecializedAgentInterface) MessageHandler {
	return MessageHandlerFunc(func(ctx context.Context, msg *AgentMessage) error {
		// No-op for now
		return nil
	})
}

// =============================================================================
// SYSTEM LIFECYCLE
// =============================================================================

// Initialize initializes the multi-agent system
func (mas *MultiAgentSystem) Initialize(ctx context.Context) error {
	mas.mutex.Lock()
	defer mas.mutex.Unlock()

	mas.logger.Info("Initializing multi-agent system")

	// Start message bus
	if err := mas.messageBus.Start(ctx); err != nil {
		return fmt.Errorf("failed to start message bus: %w", err)
	}

	// Create coordinator agent
	coordinator, err := mas.createCoordinatorAgent()
	if err != nil {
		return fmt.Errorf("failed to create coordinator agent: %w", err)
	}
	mas.coordinator = coordinator

	// Register coordinator with registry
	if err := mas.agentRegistry.RegisterAgent(coordinator); err != nil {
		return fmt.Errorf("failed to register coordinator agent: %w", err)
	}

	// Create specialized agents
	if err := mas.createSpecializedAgents(); err != nil {
		return fmt.Errorf("failed to create specialized agents: %w", err)
	}

	// Register all agents with coordinator
	if err := mas.registerAgentsWithCoordinator(); err != nil {
		return fmt.Errorf("failed to register agents with coordinator: %w", err)
	}

	mas.running = true
	mas.systemMetrics.LastActivity = time.Now()

	mas.logger.WithField("agent_count", len(mas.agents)).Info("Multi-agent system initialized successfully")
	return nil
}

// Start starts the multi-agent system
func (mas *MultiAgentSystem) Start(ctx context.Context) error {
	mas.mutex.Lock()
	defer mas.mutex.Unlock()

	if mas.running {
		return fmt.Errorf("multi-agent system already running")
	}

	mas.logger.Info("Starting multi-agent system")

	// Start all agents
	for _, agent := range mas.agents {
		if err := agent.Start(ctx); err != nil {
			mas.logger.WithError(err).WithField("agent_id", agent.GetInfo().ID).Error("Failed to start agent")
			return fmt.Errorf("failed to start agent %s: %w", agent.GetInfo().ID, err)
		}
	}

	// Start coordinator
	if err := mas.coordinator.Start(ctx); err != nil {
		return fmt.Errorf("failed to start coordinator: %w", err)
	}

	mas.running = true
	mas.systemMetrics.LastActivity = time.Now()

	mas.logger.Info("Multi-agent system started successfully")
	return nil
}

// Stop stops the multi-agent system
func (mas *MultiAgentSystem) Stop(ctx context.Context) error {
	mas.mutex.Lock()
	defer mas.mutex.Unlock()

	if !mas.running {
		return fmt.Errorf("multi-agent system not running")
	}

	mas.logger.Info("Stopping multi-agent system")

	// Stop all agents
	for _, agent := range mas.agents {
		if err := agent.Stop(ctx); err != nil {
			mas.logger.WithError(err).WithField("agent_id", agent.GetInfo().ID).Error("Failed to stop agent")
		}
	}

	// Stop coordinator
	if err := mas.coordinator.Stop(ctx); err != nil {
		mas.logger.WithError(err).Error("Failed to stop coordinator")
	}

	// Stop message bus
	if err := mas.messageBus.Stop(ctx); err != nil {
		mas.logger.WithError(err).Error("Failed to stop message bus")
	}

	mas.running = false
	mas.systemMetrics.LastActivity = time.Now()

	mas.logger.Info("Multi-agent system stopped")
	return nil
}

// =============================================================================
// AGENT MANAGEMENT
// =============================================================================

// createCoordinatorAgent creates the coordinator agent
func (mas *MultiAgentSystem) createCoordinatorAgent() (CoordinatorAgentInterface, error) {
	// Create dependencies
	dependencies := &AgentDependencies{
		Config:        mas.config,
		AgentConfig:   &mas.config.Agent,
		AWSConfig:     &mas.config.AWS,
		AWSClient:     mas.awsClient,
		Logger:        mas.logger,
		StateManager:  mas.stateManager,
		MessageBus:    mas.messageBus,
		AgentRegistry: mas.agentRegistry,
	}

	// Create coordinator agent
	coordinator, err := mas.agentFactory.CreateCoordinatorAgent(&mas.config.Agent, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator agent: %w", err)
	}

	return coordinator, nil
}

// createSpecializedAgents creates all specialized agents
func (mas *MultiAgentSystem) createSpecializedAgents() error {
	agentTypes := []AgentType{
		AgentTypeNetwork,
		AgentTypeCompute,
		AgentTypeStorage,
		AgentTypeSecurity,
		AgentTypeMonitoring,
		AgentTypeBackup,
	}

	for _, agentType := range agentTypes {
		agent, err := mas.createSpecializedAgent(agentType)
		if err != nil {
			mas.logger.WithError(err).WithField("agent_type", agentType).Error("Failed to create specialized agent")
			continue
		}

		mas.agents[agent.GetInfo().ID] = agent
		mas.logger.WithFields(map[string]interface{}{
			"agent_id":   agent.GetInfo().ID,
			"agent_type": agentType,
		}).Info("Specialized agent created")
	}

	return nil
}

// createSpecializedAgent creates a specialized agent of the specified type
func (mas *MultiAgentSystem) createSpecializedAgent(agentType AgentType) (SpecializedAgentInterface, error) {
	// Create dependencies
	dependencies := &AgentDependencies{
		Config:        mas.config,
		AgentConfig:   &mas.config.Agent,
		AWSConfig:     &mas.config.AWS,
		AWSClient:     mas.awsClient,
		Logger:        mas.logger,
		StateManager:  mas.stateManager,
		MessageBus:    mas.messageBus,
		AgentRegistry: mas.agentRegistry,
	}

	// Create agent
	agent, err := mas.agentFactory.CreateAgent(agentType, &mas.config.Agent, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent %s: %w", agentType, err)
	}

	// Register with message bus
	if err := mas.messageBus.Subscribe(agent.GetInfo().ID, mas.makeAgentMessageHandler(agent)); err != nil {
		return nil, fmt.Errorf("failed to subscribe agent to message bus: %w", err)
	}

	// Register with registry
	if err := mas.agentRegistry.RegisterAgent(agent); err != nil {
		return nil, fmt.Errorf("failed to register agent: %w", err)
	}

	return agent, nil
}

// registerAgentsWithCoordinator registers all agents with the coordinator
func (mas *MultiAgentSystem) registerAgentsWithCoordinator() error {
	for _, agent := range mas.agents {
		if err := mas.coordinator.RegisterAgent(agent); err != nil {
			mas.logger.WithError(err).WithField("agent_id", agent.GetInfo().ID).Error("Failed to register agent with coordinator")
			return err
		}
	}
	return nil
}

// =============================================================================
// REQUEST PROCESSING
// =============================================================================

// ProcessRequest processes a natural language infrastructure request
func (mas *MultiAgentSystem) ProcessRequest(ctx context.Context, request string) (*types.AgentDecision, error) {
	mas.mutex.RLock()
	running := mas.running
	mas.mutex.RUnlock()

	if !running {
		return nil, fmt.Errorf("multi-agent system not running")
	}

	mas.logger.WithField("request", request).Info("Processing infrastructure request")

	// Decompose request into tasks
	tasks, err := mas.coordinator.DecomposeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to decompose request: %w", err)
	}

	// Orchestrate execution
	decision, err := mas.coordinator.OrchestrateExecution(tasks)
	if err != nil {
		return nil, fmt.Errorf("failed to orchestrate execution: %w", err)
	}

	// Update system metrics
	mas.updateSystemMetrics()

	mas.logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"task_count":  len(tasks),
		"action":      decision.Action,
	}).Info("Infrastructure request processed successfully")

	return decision, nil
}

// =============================================================================
// SYSTEM MONITORING
// =============================================================================

// GetSystemStatus returns the current status of the multi-agent system
func (mas *MultiAgentSystem) GetSystemStatus() map[string]interface{} {
	mas.mutex.RLock()
	defer mas.mutex.RUnlock()

	status := map[string]interface{}{
		"running":        mas.running,
		"total_agents":   len(mas.agents),
		"active_tasks":   len(mas.activeTasks),
		"coordinator_id": mas.coordinator.GetInfo().ID,
		"last_activity":  mas.systemMetrics.LastActivity,
	}

	// Add agent statuses
	agentStatuses := make(map[string]interface{})
	for id, agent := range mas.agents {
		agentStatuses[id] = map[string]interface{}{
			"type":   agent.GetAgentType(),
			"status": agent.GetStatus(),
			"name":   agent.GetInfo().Name,
		}
	}
	status["agents"] = agentStatuses

	return status
}

// GetSystemMetrics returns detailed metrics for the multi-agent system
func (mas *MultiAgentSystem) GetSystemMetrics() *SystemMetrics {
	mas.systemMetrics.mutex.Lock()
	defer mas.systemMetrics.mutex.Unlock()

	// Update metrics
	mas.systemMetrics.TotalAgents = len(mas.agents)
	mas.systemMetrics.ActiveAgents = 0
	mas.systemMetrics.TotalTasks = len(mas.activeTasks)

	for _, agent := range mas.agents {
		if agent.GetStatus() == AgentStatusReady || agent.GetStatus() == AgentStatusBusy {
			mas.systemMetrics.ActiveAgents++
		}
		mas.systemMetrics.AgentMetrics[agent.GetInfo().ID] = agent.GetMetrics()
	}

	// Return a copy to prevent external modification
	metrics := &SystemMetrics{
		TotalAgents:       mas.systemMetrics.TotalAgents,
		ActiveAgents:      mas.systemMetrics.ActiveAgents,
		TotalTasks:        mas.systemMetrics.TotalTasks,
		CompletedTasks:    mas.systemMetrics.CompletedTasks,
		FailedTasks:       mas.systemMetrics.FailedTasks,
		MessagesProcessed: mas.systemMetrics.MessagesProcessed,
		SystemUptime:      mas.systemMetrics.SystemUptime,
		AgentMetrics:      make(map[string]interface{}),
		LastActivity:      mas.systemMetrics.LastActivity,
	}

	// Copy agent metrics
	for id, m := range mas.systemMetrics.AgentMetrics {
		metrics.AgentMetrics[id] = m
	}

	return metrics
}

// updateSystemMetrics updates the system metrics
func (mas *MultiAgentSystem) updateSystemMetrics() {
	mas.systemMetrics.mutex.Lock()
	defer mas.systemMetrics.mutex.Unlock()

	mas.systemMetrics.LastActivity = time.Now()
	mas.systemMetrics.TotalAgents = len(mas.agents)
	mas.systemMetrics.ActiveAgents = 0

	for _, agent := range mas.agents {
		if agent.GetStatus() == AgentStatusReady || agent.GetStatus() == AgentStatusBusy {
			mas.systemMetrics.ActiveAgents++
		}
	}
}

// =============================================================================
// AGENT DISCOVERY AND MANAGEMENT
// =============================================================================

// GetAgent returns a specific agent by ID
func (mas *MultiAgentSystem) GetAgent(agentID string) (SpecializedAgentInterface, bool) {
	mas.mutex.RLock()
	defer mas.mutex.RUnlock()

	agent, exists := mas.agents[agentID]
	return agent, exists
}

// GetAgentsByType returns all agents of a specific type
func (mas *MultiAgentSystem) GetAgentsByType(agentType AgentType) []SpecializedAgentInterface {
	mas.mutex.RLock()
	defer mas.mutex.RUnlock()

	var agents []SpecializedAgentInterface
	for _, agent := range mas.agents {
		if agent.GetAgentType() == agentType {
			agents = append(agents, agent)
		}
	}
	return agents
}

// AddAgent adds a new agent to the system
func (mas *MultiAgentSystem) AddAgent(agent SpecializedAgentInterface) error {
	mas.mutex.Lock()
	defer mas.mutex.Unlock()

	agentID := agent.GetInfo().ID

	// Check if agent already exists
	if _, exists := mas.agents[agentID]; exists {
		return fmt.Errorf("agent %s already exists", agentID)
	}

	// Add to system
	mas.agents[agentID] = agent

	// Register with coordinator
	if err := mas.coordinator.RegisterAgent(agent); err != nil {
		delete(mas.agents, agentID)
		return fmt.Errorf("failed to register agent with coordinator: %w", err)
	}

	// Subscribe to message bus
	if err := mas.messageBus.Subscribe(agentID, mas.makeAgentMessageHandler(agent)); err != nil {
		delete(mas.agents, agentID)
		return fmt.Errorf("failed to subscribe agent to message bus: %w", err)
	}

	mas.logger.WithFields(map[string]interface{}{
		"agent_id":   agentID,
		"agent_type": agent.GetAgentType(),
	}).Info("Agent added to system")

	return nil
}

// RemoveAgent removes an agent from the system
func (mas *MultiAgentSystem) RemoveAgent(agentID string) error {
	mas.mutex.Lock()
	defer mas.mutex.Unlock()

	_, exists := mas.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	// Unregister from coordinator
	if err := mas.coordinator.UnregisterAgent(agentID); err != nil {
		mas.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to unregister agent from coordinator")
	}

	// Unsubscribe from message bus
	if err := mas.messageBus.Unsubscribe(agentID); err != nil {
		mas.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to unsubscribe agent from message bus")
	}

	// Remove from system
	delete(mas.agents, agentID)

	mas.logger.WithField("agent_id", agentID).Info("Agent removed from system")

	return nil
}

// =============================================================================
// TASK MANAGEMENT
// =============================================================================

// GetTaskStatus returns the status of a specific task
func (mas *MultiAgentSystem) GetTaskStatus(taskID string) (*Task, error) {
	return mas.coordinator.GetTaskStatus(taskID)
}

// GetAllTasks returns all tasks in the system
func (mas *MultiAgentSystem) GetAllTasks() []*Task {
	return mas.coordinator.GetAllTasks()
}

// =============================================================================
// SYSTEM HEALTH AND DIAGNOSTICS
// =============================================================================

// HealthCheck performs a health check on the entire system
func (mas *MultiAgentSystem) HealthCheck() error {
	mas.mutex.RLock()
	defer mas.mutex.RUnlock()

	if !mas.running {
		return fmt.Errorf("multi-agent system not running")
	}

	// Check coordinator health
	if err := mas.coordinator.HealthCheck(); err != nil {
		return fmt.Errorf("coordinator health check failed: %w", err)
	}

	// Check agent health
	for _, agent := range mas.agents {
		if err := agent.HealthCheck(); err != nil {
			mas.logger.WithError(err).WithField("agent_id", agent.GetInfo().ID).Warn("Agent health check failed")
		}
	}

	// Check message bus health
	if mas.messageBus.GetStatus() != "running" {
		return fmt.Errorf("message bus not running")
	}

	return nil
}

// GetSystemDiagnostics returns detailed diagnostics for the system
func (mas *MultiAgentSystem) GetSystemDiagnostics() map[string]interface{} {
	mas.mutex.RLock()
	defer mas.mutex.RUnlock()

	diagnostics := map[string]interface{}{
		"system_status":         mas.GetSystemStatus(),
		"system_metrics":        mas.GetSystemMetrics(),
		"message_bus_stats":     mas.messageBus.GetStatistics(),
		"agent_registry_health": mas.agentRegistry.GetAllAgentHealth(),
	}

	// Add individual agent diagnostics
	agentDiagnostics := make(map[string]interface{})
	for id, agent := range mas.agents {
		agentDiagnostics[id] = map[string]interface{}{
			"info":    agent.GetInfo(),
			"metrics": agent.GetMetrics(),
			"health":  agent.HealthCheck() == nil,
		}
	}
	diagnostics["agent_diagnostics"] = agentDiagnostics

	return diagnostics
}
