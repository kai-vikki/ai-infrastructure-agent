package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// =============================================================================
// AGENT FACTORY IMPLEMENTATION
// =============================================================================

// AgentFactoryImpl implements the AgentFactoryInterface
type AgentFactoryImpl struct {
	// Agent creators registry
	creators map[AgentType]AgentCreator

	// Configuration
	config *config.Config
	logger *logging.Logger

	// Thread safety
	mutex sync.RWMutex
}

// NewAgentFactory creates a new agent factory
func NewAgentFactory(config *config.Config, logger *logging.Logger) AgentFactoryInterface {
	factory := &AgentFactoryImpl{
		creators: make(map[AgentType]AgentCreator),
		config:   config,
		logger:   logger,
	}

	// Register default agent types
	factory.registerDefaultAgentTypes()

	return factory
}

// CreateAgent creates a new specialized agent
func (f *AgentFactoryImpl) CreateAgent(agentType AgentType, agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	f.mutex.RLock()
	creator, exists := f.creators[agentType]
	f.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported agent type: %s", agentType)
	}

	// Validate dependencies
	if err := f.validateDependencies(dependencies); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	// Create the agent
	agent, err := creator(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent %s: %w", agentType, err)
	}

	f.logger.WithFields(map[string]interface{}{
		"agent_type": agentType,
		"agent_id":   agent.GetInfo().ID,
	}).Info("Agent created successfully")

	return agent, nil
}

// CreateCoordinatorAgent creates a new coordinator agent
func (f *AgentFactoryImpl) CreateCoordinatorAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (CoordinatorAgentInterface, error) {
	// Create coordinator agent using the factory
	agent, err := f.CreateAgent(AgentTypeCoordinator, agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator agent: %w", err)
	}

	// Type assert to coordinator interface
	coordinator, ok := agent.(CoordinatorAgentInterface)
	if !ok {
		return nil, fmt.Errorf("created agent is not a coordinator agent")
	}

	return coordinator, nil
}

// RegisterAgentType registers a new agent type with its creator function
func (f *AgentFactoryImpl) RegisterAgentType(agentType AgentType, creator AgentCreator) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.creators[agentType] = creator
	f.logger.WithField("agent_type", agentType).Info("Agent type registered")
}

// GetSupportedAgentTypes returns all supported agent types
func (f *AgentFactoryImpl) GetSupportedAgentTypes() []AgentType {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	types := make([]AgentType, 0, len(f.creators))
	for agentType := range f.creators {
		types = append(types, agentType)
	}

	return types
}

// =============================================================================
// DEFAULT AGENT CREATORS
// =============================================================================

// registerDefaultAgentTypes registers the default agent types
func (f *AgentFactoryImpl) registerDefaultAgentTypes() {
	// Register coordinator agent
	f.RegisterAgentType(AgentTypeCoordinator, f.createCoordinatorAgent)

	// Register specialized agents
	f.RegisterAgentType(AgentTypeNetwork, f.createNetworkAgent)
	f.RegisterAgentType(AgentTypeCompute, f.createComputeAgent)
	f.RegisterAgentType(AgentTypeStorage, f.createStorageAgent)
	f.RegisterAgentType(AgentTypeSecurity, f.createSecurityAgent)
	f.RegisterAgentType(AgentTypeMonitoring, f.createMonitoringAgent)
	f.RegisterAgentType(AgentTypeBackup, f.createBackupAgent)
}

// createCoordinatorAgent creates a coordinator agent
func (f *AgentFactoryImpl) createCoordinatorAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	// Create base agent first
	baseAgent, err := f.createBaseAgent(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	// Create coordinator agent
	coordinator := &CoordinatorAgent{
		BaseAgent:         baseAgent,
		taskQueue:         make(chan *Task, 100),
		activeTasks:       make(map[string]*Task),
		completedTasks:    make(map[string]*Task),
		failedTasks:       make(map[string]*Task),
		taskDependencies:  make(map[string][]string),
		agentCapabilities: make(map[AgentType][]AgentCapability),
		logger:            dependencies.Logger,
		awsClient:         dependencies.AWSClient,
	}

	// Initialize LLM for coordinator (enables AI-based decomposition/decision making)
	if llm, err := initializeLLM(agentConfig, f.logger); err != nil {
		f.logger.WithError(err).Warn("Coordinator LLM initialization failed; falling back to rule-based decomposition")
	} else {
		coordinator.llm = llm
	}

	// Ensure orchestration engine is ready
	coordinator.initializeOrchestrationEngine()

	// Initialize the coordinator
	if err := coordinator.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize coordinator agent: %w", err)
	}

	return coordinator, nil
}

// createNetworkAgent creates a network agent
func (f *AgentFactoryImpl) createNetworkAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	// Create base agent first
	baseAgent, err := f.createBaseAgent(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	// Create network agent using constructor
	networkAgent := NewNetworkAgent(baseAgent, dependencies.AWSClient, dependencies.Logger)

	// Initialize the network agent
	if err := networkAgent.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize network agent: %w", err)
	}

	return networkAgent, nil
}

// createComputeAgent creates a compute agent
func (f *AgentFactoryImpl) createComputeAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	// Create base agent first
	baseAgent, err := f.createBaseAgent(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	// Create compute agent using constructor
	computeAgent := NewComputeAgent(baseAgent, dependencies.AWSClient, dependencies.Logger)

	// Initialize the compute agent
	if err := computeAgent.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize compute agent: %w", err)
	}

	return computeAgent, nil
}

// createStorageAgent creates a storage agent
func (f *AgentFactoryImpl) createStorageAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	// Create base agent first
	baseAgent, err := f.createBaseAgent(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	// Create storage agent using constructor
	storageAgent := NewStorageAgent(baseAgent, dependencies.AWSClient, dependencies.Logger)

	// Initialize the storage agent
	if err := storageAgent.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize storage agent: %w", err)
	}

	return storageAgent, nil
}

// createSecurityAgent creates a security agent
func (f *AgentFactoryImpl) createSecurityAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	// Create base agent first
	baseAgent, err := f.createBaseAgent(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	// Create security agent using constructor
	securityAgent := NewSecurityAgent(baseAgent, dependencies.AWSClient, dependencies.Logger)

	// Initialize the security agent
	if err := securityAgent.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize security agent: %w", err)
	}

	return securityAgent, nil
}

// createMonitoringAgent creates a monitoring agent
func (f *AgentFactoryImpl) createMonitoringAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	// Create base agent first
	baseAgent, err := f.createBaseAgent(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	// Create monitoring agent using constructor
	monitoringAgent := NewMonitoringAgent(baseAgent, dependencies.AWSClient, dependencies.Logger)

	// Initialize the monitoring agent
	if err := monitoringAgent.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize monitoring agent: %w", err)
	}

	return monitoringAgent, nil
}

// createBackupAgent creates a backup agent
func (f *AgentFactoryImpl) createBackupAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error) {
	// Create base agent first
	baseAgent, err := f.createBaseAgent(agentConfig, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	// Create backup agent using constructor
	backupAgent := NewBackupAgent(baseAgent, dependencies.AWSClient, dependencies.Logger)

	// Initialize the backup agent
	if err := backupAgent.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize backup agent: %w", err)
	}

	return backupAgent, nil
}

// =============================================================================
// BASE AGENT CREATION
// =============================================================================

// createBaseAgent creates a base agent with common functionality
func (f *AgentFactoryImpl) createBaseAgent(agentConfig *config.AgentConfig, dependencies *AgentDependencies) (*BaseAgent, error) {
	// Generate unique agent ID
	agentID := uuid.New().String()

	// Create base agent
	baseAgent := &BaseAgent{
		id:           agentID,
		agentType:    AgentType(""), // Will be set by specialized agent
		name:         "",
		description:  "",
		status:       AgentStatusInitializing,
		capabilities: make([]AgentCapability, 0),
		tools:        make([]interfaces.MCPTool, 0),
		lastSeen:     time.Now(),
		metadata:     make(map[string]string),

		// Dependencies
		config:        dependencies.Config,
		agentConfig:   agentConfig,
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
			requestsProcessed: 0,
			tasksProcessed:    0,
			errors:            0,
			lastActivity:      time.Now(),
		},
	}

	return baseAgent, nil
}

// =============================================================================
// DEPENDENCY VALIDATION
// =============================================================================

// validateDependencies validates the dependencies required to create an agent
func (f *AgentFactoryImpl) validateDependencies(dependencies *AgentDependencies) error {
	if dependencies == nil {
		return fmt.Errorf("dependencies cannot be nil")
	}

	if dependencies.Config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if dependencies.AgentConfig == nil {
		return fmt.Errorf("agent config cannot be nil")
	}

	if dependencies.AWSConfig == nil {
		return fmt.Errorf("AWS config cannot be nil")
	}

	if dependencies.AWSClient == nil {
		return fmt.Errorf("AWS client cannot be nil")
	}

	if dependencies.Logger == nil {
		return fmt.Errorf("logger cannot be nil")
	}

	if dependencies.StateManager == nil {
		return fmt.Errorf("state manager cannot be nil")
	}

	if dependencies.MessageBus == nil {
		return fmt.Errorf("message bus cannot be nil")
	}

	if dependencies.AgentRegistry == nil {
		return fmt.Errorf("agent registry cannot be nil")
	}

	return nil
}

// =============================================================================
// AGENT CREATION UTILITIES
// =============================================================================

// AgentCreationOptions provides options for agent creation
type AgentCreationOptions struct {
	AgentID      string
	AgentName    string
	Description  string
	Capabilities []AgentCapability
	Tools        []string
	Metadata     map[string]string
}

// CreateAgentWithOptions creates an agent with specific options
func (f *AgentFactoryImpl) CreateAgentWithOptions(agentType AgentType, agentConfig *config.AgentConfig, dependencies *AgentDependencies, options *AgentCreationOptions) (SpecializedAgentInterface, error) {
	// Create the agent
	agent, err := f.CreateAgent(agentType, agentConfig, dependencies)
	if err != nil {
		return nil, err
	}

	// Apply options if provided
	if options != nil {
		// Update agent info
		info := agent.GetInfo()
		if options.AgentID != "" {
			info.ID = options.AgentID
		}
		if options.AgentName != "" {
			info.Name = options.AgentName
		}
		if options.Description != "" {
			info.Description = options.Description
		}
		if len(options.Capabilities) > 0 {
			info.Capabilities = options.Capabilities
		}
		if len(options.Tools) > 0 {
			info.Tools = options.Tools
		}
		if len(options.Metadata) > 0 {
			info.Metadata = options.Metadata
		}
	}

	return agent, nil
}

// CreateAgentCluster creates a cluster of agents for a specific purpose
func (f *AgentFactoryImpl) CreateAgentCluster(agentTypes []AgentType, agentConfig *config.AgentConfig, dependencies *AgentDependencies) (map[AgentType]SpecializedAgentInterface, error) {
	agents := make(map[AgentType]SpecializedAgentInterface)

	for _, agentType := range agentTypes {
		agent, err := f.CreateAgent(agentType, agentConfig, dependencies)
		if err != nil {
			// Clean up already created agents
			for _, createdAgent := range agents {
				if stopErr := createdAgent.Stop(context.Background()); stopErr != nil {
					f.logger.WithError(stopErr).Error("Failed to stop agent during cleanup")
				}
			}
			return nil, fmt.Errorf("failed to create agent cluster: %w", err)
		}

		agents[agentType] = agent
	}

	f.logger.WithField("agent_count", len(agents)).Info("Agent cluster created successfully")

	return agents, nil
}
