package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// AGENT REGISTRY IMPLEMENTATION
// =============================================================================

// AgentRegistryImpl implements the AgentRegistryInterface
type AgentRegistryImpl struct {
	agents              map[string]SpecializedAgentInterface
	agentsByType        map[AgentType][]SpecializedAgentInterface
	agentsByCapability  map[string][]SpecializedAgentInterface
	healthStatus        map[string]AgentStatus
	lastSeen            map[string]time.Time
	mutex               sync.RWMutex
	logger              *logging.Logger
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry(logger *logging.Logger) AgentRegistryInterface {
	registry := &AgentRegistryImpl{
		agents:              make(map[string]SpecializedAgentInterface),
		agentsByType:        make(map[AgentType][]SpecializedAgentInterface),
		agentsByCapability:  make(map[string][]SpecializedAgentInterface),
		healthStatus:        make(map[string]AgentStatus),
		lastSeen:            make(map[string]time.Time),
		logger:              logger,
		healthCheckInterval: 30 * time.Second,
		healthCheckTimeout:  5 * time.Second,
	}

	// Start health monitoring
	go registry.startHealthMonitoring()

	return registry
}

// RegisterAgent registers a new agent in the registry
func (r *AgentRegistryImpl) RegisterAgent(agent SpecializedAgentInterface) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	agentInfo := agent.GetInfo()
	agentID := agentInfo.ID

	// Check if agent already exists
	if _, exists := r.agents[agentID]; exists {
		return fmt.Errorf("agent %s already registered", agentID)
	}

	// Register the agent
	r.agents[agentID] = agent
	r.healthStatus[agentID] = AgentStatusReady
	r.lastSeen[agentID] = time.Now()

	// Update type index
	agentType := agentInfo.Type
	r.agentsByType[agentType] = append(r.agentsByType[agentType], agent)

	// Update capability index
	for _, capability := range agentInfo.Capabilities {
		r.agentsByCapability[capability.Name] = append(r.agentsByCapability[capability.Name], agent)
	}

	r.logger.WithFields(map[string]interface{}{
		"agent_id":     agentID,
		"agent_type":   agentType,
		"agent_name":   agentInfo.Name,
		"capabilities": len(agentInfo.Capabilities),
		"tools":        len(agentInfo.Tools),
	}).Info("Agent registered successfully")

	return nil
}

// UnregisterAgent removes an agent from the registry
func (r *AgentRegistryImpl) UnregisterAgent(agentID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agentInfo := agent.GetInfo()
	agentType := agentInfo.Type

	// Remove from main registry
	delete(r.agents, agentID)
	delete(r.healthStatus, agentID)
	delete(r.lastSeen, agentID)

	// Remove from type index
	if agents, exists := r.agentsByType[agentType]; exists {
		for i, a := range agents {
			if a.GetInfo().ID == agentID {
				r.agentsByType[agentType] = append(agents[:i], agents[i+1:]...)
				break
			}
		}
		if len(r.agentsByType[agentType]) == 0 {
			delete(r.agentsByType, agentType)
		}
	}

	// Remove from capability index
	for _, capability := range agentInfo.Capabilities {
		if agents, exists := r.agentsByCapability[capability.Name]; exists {
			for i, a := range agents {
				if a.GetInfo().ID == agentID {
					r.agentsByCapability[capability.Name] = append(agents[:i], agents[i+1:]...)
					break
				}
			}
			if len(r.agentsByCapability[capability.Name]) == 0 {
				delete(r.agentsByCapability, capability.Name)
			}
		}
	}

	r.logger.WithFields(map[string]interface{}{
		"agent_id":   agentID,
		"agent_type": agentType,
		"agent_name": agentInfo.Name,
	}).Info("Agent unregistered successfully")

	return nil
}

// GetAgent retrieves an agent by ID
func (r *AgentRegistryImpl) GetAgent(agentID string) (SpecializedAgentInterface, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agent, exists := r.agents[agentID]
	return agent, exists
}

// GetAllAgents returns all registered agents
func (r *AgentRegistryImpl) GetAllAgents() map[string]SpecializedAgentInterface {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Return a copy to prevent external modification
	agents := make(map[string]SpecializedAgentInterface)
	for id, agent := range r.agents {
		agents[id] = agent
	}
	return agents
}

// FindAgentsByType finds all agents of a specific type
func (r *AgentRegistryImpl) FindAgentsByType(agentType AgentType) []SpecializedAgentInterface {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agents, exists := r.agentsByType[agentType]
	if !exists {
		return []SpecializedAgentInterface{}
	}

	// Return a copy to prevent external modification
	result := make([]SpecializedAgentInterface, len(agents))
	copy(result, agents)
	return result
}

// FindAgentsByCapability finds all agents that have a specific capability
func (r *AgentRegistryImpl) FindAgentsByCapability(capability string) []SpecializedAgentInterface {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agents, exists := r.agentsByCapability[capability]
	if !exists {
		return []SpecializedAgentInterface{}
	}

	// Return a copy to prevent external modification
	result := make([]SpecializedAgentInterface, len(agents))
	copy(result, agents)
	return result
}

// FindBestAgentForTask finds the best agent to handle a specific task
func (r *AgentRegistryImpl) FindBestAgentForTask(task *Task) (SpecializedAgentInterface, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// First, try to find agents by required type
	if task.RequiredAgent != "" {
		agents := r.agentsByType[task.RequiredAgent]
		if len(agents) > 0 {
			// Return the first healthy agent
			for _, agent := range agents {
				if r.isAgentHealthy(agent.GetInfo().ID) {
					return agent, nil
				}
			}
		}
	}

	// If no specific type required, find by task type
	agents := r.agentsByCapability[task.Type]
	if len(agents) > 0 {
		// Return the first healthy agent
		for _, agent := range agents {
			if r.isAgentHealthy(agent.GetInfo().ID) {
				return agent, nil
			}
		}
	}

	// If no specialized agents found, try to find any agent that can handle the task
	for _, agent := range r.agents {
		if r.isAgentHealthy(agent.GetInfo().ID) && agent.CanHandleTask(task) {
			return agent, nil
		}
	}

	return nil, fmt.Errorf("no suitable agent found for task %s", task.ID)
}

// GetAgentHealth returns the health status of a specific agent
func (r *AgentRegistryImpl) GetAgentHealth(agentID string) (AgentStatus, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	status, exists := r.healthStatus[agentID]
	if !exists {
		return AgentStatusStopped, fmt.Errorf("agent %s not found", agentID)
	}

	return status, nil
}

// GetAllAgentHealth returns the health status of all agents
func (r *AgentRegistryImpl) GetAllAgentHealth() map[string]AgentStatus {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Return a copy to prevent external modification
	health := make(map[string]AgentStatus)
	for id, status := range r.healthStatus {
		health[id] = status
	}
	return health
}

// RemoveUnhealthyAgents removes agents that are no longer healthy
func (r *AgentRegistryImpl) RemoveUnhealthyAgents() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var agentsToRemove []string

	for agentID, status := range r.healthStatus {
		if status == AgentStatusError || status == AgentStatusStopped {
			// Check if agent hasn't been seen for too long
			if lastSeen, exists := r.lastSeen[agentID]; exists {
				if time.Since(lastSeen) > 5*time.Minute {
					agentsToRemove = append(agentsToRemove, agentID)
				}
			}
		}
	}

	// Remove unhealthy agents
	for _, agentID := range agentsToRemove {
		if err := r.unregisterAgentInternal(agentID); err != nil {
			r.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to remove unhealthy agent")
		} else {
			r.logger.WithField("agent_id", agentID).Info("Removed unhealthy agent")
		}
	}

	return nil
}

// =============================================================================
// PRIVATE METHODS
// =============================================================================

// isAgentHealthy checks if an agent is healthy
func (r *AgentRegistryImpl) isAgentHealthy(agentID string) bool {
	status, exists := r.healthStatus[agentID]
	if !exists {
		return false
	}

	return status == AgentStatusReady || status == AgentStatusBusy
}

// unregisterAgentInternal removes an agent without acquiring the lock
func (r *AgentRegistryImpl) unregisterAgentInternal(agentID string) error {
	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agentInfo := agent.GetInfo()
	agentType := agentInfo.Type

	// Remove from main registry
	delete(r.agents, agentID)
	delete(r.healthStatus, agentID)
	delete(r.lastSeen, agentID)

	// Remove from type index
	if agents, exists := r.agentsByType[agentType]; exists {
		for i, a := range agents {
			if a.GetInfo().ID == agentID {
				r.agentsByType[agentType] = append(agents[:i], agents[i+1:]...)
				break
			}
		}
		if len(r.agentsByType[agentType]) == 0 {
			delete(r.agentsByType, agentType)
		}
	}

	// Remove from capability index
	for _, capability := range agentInfo.Capabilities {
		if agents, exists := r.agentsByCapability[capability.Name]; exists {
			for i, a := range agents {
				if a.GetInfo().ID == agentID {
					r.agentsByCapability[capability.Name] = append(agents[:i], agents[i+1:]...)
					break
				}
			}
			if len(r.agentsByCapability[capability.Name]) == 0 {
				delete(r.agentsByCapability, capability.Name)
			}
		}
	}

	return nil
}

// startHealthMonitoring starts the health monitoring goroutine
func (r *AgentRegistryImpl) startHealthMonitoring() {
	ticker := time.NewTicker(r.healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.performHealthCheck()
	}
}

// performHealthCheck performs health checks on all registered agents
func (r *AgentRegistryImpl) performHealthCheck() {
	r.mutex.RLock()
	agents := make(map[string]SpecializedAgentInterface)
	for id, agent := range r.agents {
		agents[id] = agent
	}
	r.mutex.RUnlock()

	for agentID, agent := range agents {
		// Perform health check with timeout
		_, cancel := context.WithTimeout(context.Background(), r.healthCheckTimeout)

		// Update last seen time
		r.mutex.Lock()
		r.lastSeen[agentID] = time.Now()
		r.mutex.Unlock()

		// Check agent health
		if err := agent.HealthCheck(); err != nil {
			r.mutex.Lock()
			r.healthStatus[agentID] = AgentStatusError
			r.mutex.Unlock()

			r.logger.WithError(err).WithField("agent_id", agentID).Warn("Agent health check failed")
		} else {
			// Update status based on current agent status
			currentStatus := agent.GetStatus()
			r.mutex.Lock()
			r.healthStatus[agentID] = currentStatus
			r.mutex.Unlock()
		}

		cancel()
	}

	// Remove unhealthy agents
	if err := r.RemoveUnhealthyAgents(); err != nil {
		r.logger.WithError(err).Error("Failed to remove unhealthy agents")
	}
}

// =============================================================================
// AGENT DISCOVERY UTILITIES
// =============================================================================

// AgentDiscovery provides utility functions for agent discovery
type AgentDiscovery struct {
	registry AgentRegistryInterface
	logger   *logging.Logger
}

// NewAgentDiscovery creates a new agent discovery instance
func NewAgentDiscovery(registry AgentRegistryInterface, logger *logging.Logger) *AgentDiscovery {
	return &AgentDiscovery{
		registry: registry,
		logger:   logger,
	}
}

// DiscoverAgentsByCapability discovers agents that can handle specific capabilities
func (ad *AgentDiscovery) DiscoverAgentsByCapability(capabilities []string) map[string][]SpecializedAgentInterface {
	result := make(map[string][]SpecializedAgentInterface)

	for _, capability := range capabilities {
		agents := ad.registry.FindAgentsByCapability(capability)
		if len(agents) > 0 {
			result[capability] = agents
		}
	}

	return result
}

// DiscoverBestAgentForRequest finds the best agent to handle a specific request
func (ad *AgentDiscovery) DiscoverBestAgentForRequest(request *AgentRequest) (SpecializedAgentInterface, error) {
	// Try to find agents that can handle the request directly
	allAgents := ad.registry.GetAllAgents()

	for _, agent := range allAgents {
		if agent.CanHandleRequest(request) {
			return agent, nil
		}
	}

	// If no direct match, try to find by request type
	if request.Type != "" {
		agents := ad.registry.FindAgentsByCapability(request.Type)
		if len(agents) > 0 {
			return agents[0], nil
		}
	}

	return nil, fmt.Errorf("no suitable agent found for request %s", request.ID)
}

// GetAgentStatistics returns statistics about registered agents
func (ad *AgentDiscovery) GetAgentStatistics() map[string]interface{} {
	allAgents := ad.registry.GetAllAgents()
	healthStatus := ad.registry.GetAllAgentHealth()

	stats := map[string]interface{}{
		"total_agents":  len(allAgents),
		"by_type":       make(map[AgentType]int),
		"by_status":     make(map[AgentStatus]int),
		"by_capability": make(map[string]int),
	}

	// Count by type and status
	for _, agent := range allAgents {
		agentInfo := agent.GetInfo()

		// Count by type
		if count, exists := stats["by_type"].(map[AgentType]int); exists {
			count[agentInfo.Type]++
		}

		// Count by status
		if status, exists := healthStatus[agentInfo.ID]; exists {
			if count, exists := stats["by_status"].(map[AgentStatus]int); exists {
				count[status]++
			}
		}

		// Count by capability
		for _, capability := range agentInfo.Capabilities {
			if count, exists := stats["by_capability"].(map[string]int); exists {
				count[capability.Name]++
			}
		}
	}

	return stats
}
