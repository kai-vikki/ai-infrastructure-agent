package agent

import (
	"context"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// InMemoryStateManager is a minimal, thread-safe implementation of interfaces.StateManager
// suitable for enabling the multi-agent APIs without external persistence.
type InMemoryStateManager struct {
	mu      sync.RWMutex
	state   *types.InfrastructureState
	byID    map[string]*types.ResourceState
	deps    map[string]map[string]bool
	revDeps map[string]map[string]bool
}

func NewInMemoryStateManager() interfaces.StateManager {
	return &InMemoryStateManager{
		state: &types.InfrastructureState{
			Resources:    make(map[string]*types.ResourceState),
			Dependencies: make(map[string][]string),
			Metadata:     make(map[string]interface{}),
		},
		byID:    make(map[string]*types.ResourceState),
		deps:    make(map[string]map[string]bool),
		revDeps: make(map[string]map[string]bool),
	}
}

// State persistence (no-op for in-memory)
func (sm *InMemoryStateManager) LoadState(ctx context.Context) error { return nil }
func (sm *InMemoryStateManager) SaveState(ctx context.Context) error { return nil }

// Resource management
func (sm *InMemoryStateManager) AddResource(ctx context.Context, resource *types.ResourceState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.byID[resource.ID] = resource
	sm.state.Resources[resource.ID] = resource
	return nil
}

func (sm *InMemoryStateManager) UpdateResource(ctx context.Context, resourceID string, updates map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if res, ok := sm.byID[resourceID]; ok {
		if res.Properties == nil {
			res.Properties = map[string]interface{}{}
		}
		for k, v := range updates {
			res.Properties[k] = v
		}
	}
	return nil
}

func (sm *InMemoryStateManager) RemoveResource(ctx context.Context, resourceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.byID, resourceID)
	delete(sm.state.Resources, resourceID)
	delete(sm.deps, resourceID)
	delete(sm.revDeps, resourceID)
	return nil
}

func (sm *InMemoryStateManager) GetResource(resourceID string) (*types.ResourceState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	r, ok := sm.byID[resourceID]
	return r, ok
}

func (sm *InMemoryStateManager) ListResources(resourceType string) []*types.ResourceState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var out []*types.ResourceState
	for _, r := range sm.state.Resources {
		if resourceType == "" || r.Type == resourceType {
			out = append(out, r)
		}
	}
	return out
}

// Dependency management
func (sm *InMemoryStateManager) AddDependency(ctx context.Context, resourceID, dependsOn string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.deps[resourceID] == nil {
		sm.deps[resourceID] = make(map[string]bool)
	}
	if sm.revDeps[dependsOn] == nil {
		sm.revDeps[dependsOn] = make(map[string]bool)
	}
	sm.deps[resourceID][dependsOn] = true
	sm.revDeps[dependsOn][resourceID] = true
	// also reflect into state.Dependencies as a simple list view
	var list []string
	for d := range sm.deps[resourceID] {
		list = append(list, d)
	}
	sm.state.Dependencies[resourceID] = list
	return nil
}

func (sm *InMemoryStateManager) GetDependencies(resourceID string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var out []string
	for dep := range sm.deps[resourceID] {
		out = append(out, dep)
	}
	return out
}

func (sm *InMemoryStateManager) GetDependents(resourceID string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var out []string
	for dep := range sm.revDeps[resourceID] {
		out = append(out, dep)
	}
	return out
}

// State queries
func (sm *InMemoryStateManager) GetState() *types.InfrastructureState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

func (sm *InMemoryStateManager) DetectDrift(ctx context.Context, actualState map[string]interface{}, resourceID string) (*types.ChangeDetection, error) {
	// Minimal placeholder: report no drift
	return &types.ChangeDetection{Resource: resourceID, ChangeType: "none"}, nil
}


