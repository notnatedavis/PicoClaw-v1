//   pkg/agent/registry.go

//   Package agent provides a thread-safe registry for managing Agent instances

package agent

import (
	"sync"
)

// Registry holds all active agents and allows lookup by ID
// safe for concurrent use
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

// NewRegistry creates a new empty agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

// Register adds an agent to the registry. If an agent with the same ID already
// exists, it will be overwritten
func (r *Registry) Register(a *Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.ID] = a
}

// Get returns the agent with the given ID, or nil if not found
func (r *Registry) Get(id string) *Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[id]
}