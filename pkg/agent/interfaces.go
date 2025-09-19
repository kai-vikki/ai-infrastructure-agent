package agent

import (
	"context"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// =============================================================================
// MULTI-AGENT SYSTEM INTERFACES
// =============================================================================

// AgentType represents the type of specialized agent
type AgentType string

const (
	AgentTypeCoordinator AgentType = "coordinator"
	AgentTypeNetwork     AgentType = "network"
	AgentTypeCompute     AgentType = "compute"
	AgentTypeStorage     AgentType = "storage"
	AgentTypeSecurity    AgentType = "security"
	AgentTypeMonitoring  AgentType = "monitoring"
	AgentTypeBackup      AgentType = "backup"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusInitializing AgentStatus = "initializing"
	AgentStatusReady        AgentStatus = "ready"
	AgentStatusBusy         AgentStatus = "busy"
	AgentStatusError        AgentStatus = "error"
	AgentStatusStopped      AgentStatus = "stopped"
)

// AgentCapability represents what an agent can do
type AgentCapability struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Tools       []string               `json:"tools"`
}

// AgentInfo contains metadata about an agent
type AgentInfo struct {
	ID           string            `json:"id"`
	Type         AgentType         `json:"type"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Status       AgentStatus       `json:"status"`
	Capabilities []AgentCapability `json:"capabilities"`
	Tools        []string          `json:"tools"`
	LastSeen     time.Time         `json:"lastSeen"`
	Metadata     map[string]string `json:"metadata"`
}

// AgentRequest represents a request to an agent
type AgentRequest struct {
	ID         string                 `json:"id"`
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Type       string                 `json:"type"`
	Content    string                 `json:"content"`
	Parameters map[string]interface{} `json:"parameters"`
	Priority   int                    `json:"priority"`
	Timeout    time.Duration          `json:"timeout"`
	Timestamp  time.Time              `json:"timestamp"`
	Context    context.Context        `json:"-"`
}

// AgentResponse represents a response from an agent
type AgentResponse struct {
	ID        string                 `json:"id"`
	RequestID string                 `json:"requestId"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Success   bool                   `json:"success"`
	Content   string                 `json:"content"`
	Data      map[string]interface{} `json:"data"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
}

// AgentMessage represents inter-agent communication
type AgentMessage struct {
	ID          string                 `json:"id"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	MessageType string                 `json:"messageType"`
	Content     map[string]interface{} `json:"content"`
	Priority    int                    `json:"priority"`
	Timestamp   time.Time              `json:"timestamp"`
	TTL         time.Duration          `json:"ttl"`
}

// Message types for inter-agent communication
const (
	MessageTypeRequestHelp    = "request_help"
	MessageTypeProvideResult  = "provide_result"
	MessageTypeCoordinate     = "coordinate"
	MessageTypeStatusUpdate   = "status_update"
	MessageTypeErrorReport    = "error_report"
	MessageTypeTaskDelegation = "task_delegation"
	MessageTypeTaskComplete   = "task_complete"
	MessageTypeResourceUpdate = "resource_update"
	MessageTypeHealthCheck    = "health_check"
)

// Task represents a work unit that can be assigned to agents
type Task struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Description   string                 `json:"description"`
	AssignedAgent string                 `json:"assignedAgent"`
	RequiredAgent AgentType              `json:"requiredAgent"`
	Parameters    map[string]interface{} `json:"parameters"`
	Priority      int                    `json:"priority"`
	Status        string                 `json:"status"`
	CreatedAt     time.Time              `json:"createdAt"`
	StartedAt     *time.Time             `json:"startedAt,omitempty"`
	CompletedAt   *time.Time             `json:"completedAt,omitempty"`
	Result        map[string]interface{} `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Dependencies  []string               `json:"dependencies"`
	Dependents    []string               `json:"dependents"`
}

// Task status constants
const (
	TaskStatusPending    = "pending"
	TaskStatusAssigned   = "assigned"
	TaskStatusInProgress = "in_progress"
	TaskStatusCompleted  = "completed"
	TaskStatusFailed     = "failed"
	TaskStatusCancelled  = "cancelled"
)

// =============================================================================
// CORE AGENT INTERFACES
// =============================================================================

// BaseAgentInterface defines the core functionality that all agents must implement
type BaseAgentInterface interface {
	// Core lifecycle methods
	Initialize(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() AgentStatus
	GetInfo() AgentInfo

	// Request processing
	ProcessRequest(ctx context.Context, request *AgentRequest) (*AgentResponse, error)
	CanHandleRequest(request *AgentRequest) bool

	// Tool management
	GetTools() []interfaces.MCPTool
	GetToolNames() []string
	HasTool(toolName string) bool

	// Communication
	SendMessage(message *AgentMessage) error
	ReceiveMessage(message *AgentMessage) error
	SetMessageHandler(handler MessageHandler)

	// Health and monitoring
	HealthCheck() error
	GetMetrics() map[string]interface{}
}

// SpecializedAgentInterface extends BaseAgentInterface for specialized agents
type SpecializedAgentInterface interface {
	BaseAgentInterface

	// Specialized functionality
	GetAgentType() AgentType
	GetCapabilities() []AgentCapability
	GetSpecializedTools() []interfaces.MCPTool

	// Task processing
	ProcessTask(ctx context.Context, task *Task) (*Task, error)
	CanHandleTask(task *Task) bool
	GetTaskQueue() chan *Task

	// Coordination
	CoordinateWith(otherAgent SpecializedAgentInterface) error
	RequestHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error)
	ProvideHelp(ctx context.Context, request *AgentRequest) (*AgentResponse, error)
}

// CoordinatorAgentInterface extends SpecializedAgentInterface for coordination
type CoordinatorAgentInterface interface {
	SpecializedAgentInterface

	// Coordination specific methods
	RegisterAgent(agent SpecializedAgentInterface) error
	UnregisterAgent(agentID string) error
	GetRegisteredAgents() map[string]SpecializedAgentInterface
	GetAgentByType(agentType AgentType) []SpecializedAgentInterface

	// Task management
	DecomposeRequest(request string) ([]*Task, error)
	AssignTask(task *Task) error
	GetTaskStatus(taskID string) (*Task, error)
	GetAllTasks() []*Task

	// Orchestration
	OrchestrateExecution(tasks []*Task) (*types.AgentDecision, error)
	CoordinateExecution(tasks []*Task) error
	HandleTaskCompletion(task *Task) error
}

// MessageHandler defines the interface for handling incoming messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, message *AgentMessage) error
}

// AgentRegistryInterface defines the interface for agent registration and discovery
type AgentRegistryInterface interface {
	// Agent management
	RegisterAgent(agent SpecializedAgentInterface) error
	UnregisterAgent(agentID string) error
	GetAgent(agentID string) (SpecializedAgentInterface, bool)
	GetAllAgents() map[string]SpecializedAgentInterface

	// Discovery
	FindAgentsByType(agentType AgentType) []SpecializedAgentInterface
	FindAgentsByCapability(capability string) []SpecializedAgentInterface
	FindBestAgentForTask(task *Task) (SpecializedAgentInterface, error)

	// Health monitoring
	GetAgentHealth(agentID string) (AgentStatus, error)
	GetAllAgentHealth() map[string]AgentStatus
	RemoveUnhealthyAgents() error
}

// MessageBusInterface defines the interface for inter-agent communication
type MessageBusInterface interface {
	// Message handling
	Publish(message *AgentMessage) error
	Subscribe(agentID string, handler MessageHandler) error
	Unsubscribe(agentID string) error

	// Message routing
	RouteMessage(message *AgentMessage) error
	Broadcast(message *AgentMessage, agentTypes []AgentType) error

	// Queue management
	GetMessageQueue(agentID string) ([]*AgentMessage, error)
	ClearMessageQueue(agentID string) error

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() string
	GetStatistics() *MessageBusStats
}

// AgentFactoryInterface defines the interface for creating agents
type AgentFactoryInterface interface {
	// Agent creation
	CreateAgent(agentType AgentType, config *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error)
	CreateCoordinatorAgent(config *config.AgentConfig, dependencies *AgentDependencies) (CoordinatorAgentInterface, error)

	// Factory management
	RegisterAgentType(agentType AgentType, creator AgentCreator)
	GetSupportedAgentTypes() []AgentType
}

// AgentCreator defines the function signature for creating agents
type AgentCreator func(config *config.AgentConfig, dependencies *AgentDependencies) (SpecializedAgentInterface, error)

// AgentDependencies contains all dependencies needed to create an agent
type AgentDependencies struct {
	// Core dependencies
	Config      *config.Config
	AgentConfig *config.AgentConfig
	AWSConfig   *config.AWSConfig
	AWSClient   *aws.Client
	Logger      *logging.Logger

	// System components
	StateManager     interfaces.StateManager
	DiscoveryScanner interfaces.DiscoveryScanner
	GraphManager     interface{}
	GraphAnalyzer    interface{}
	ConflictResolver interfaces.ConflictResolver

	// Multi-agent system components
	AgentRegistry AgentRegistryInterface
	MessageBus    MessageBusInterface
	TaskQueue     chan *Task
}

// =============================================================================
// UTILITY INTERFACES
// =============================================================================

// AgentMetricsInterface defines metrics collection for agents
type AgentMetricsInterface interface {
	// Metrics collection
	RecordRequest(request *AgentRequest, response *AgentResponse, duration time.Duration)
	RecordTask(task *Task, duration time.Duration)
	RecordError(agentID string, error error)

	// Metrics retrieval
	GetRequestMetrics(agentID string) map[string]interface{}
	GetTaskMetrics(agentID string) map[string]interface{}
	GetErrorMetrics(agentID string) map[string]interface{}
	GetOverallMetrics() map[string]interface{}
}

// AgentConfigInterface defines configuration management for agents
type AgentConfigInterface interface {
	// Configuration management
	GetConfig() *config.AgentConfig
	UpdateConfig(newConfig *config.AgentConfig) error
	GetAgentSpecificConfig() map[string]interface{}
	SetAgentSpecificConfig(config map[string]interface{}) error

	// Dynamic configuration
	ReloadConfig() error
	ValidateConfig() error
}

// AgentSecurityInterface defines security features for agents
type AgentSecurityInterface interface {
	// Authentication and authorization
	AuthenticateAgent(agentID string, credentials map[string]string) error
	AuthorizeRequest(agentID string, request *AgentRequest) error
	ValidateMessage(message *AgentMessage) error

	// Security policies
	GetSecurityPolicy() map[string]interface{}
	UpdateSecurityPolicy(policy map[string]interface{}) error
	EnforceSecurityPolicy(request *AgentRequest) error
}
