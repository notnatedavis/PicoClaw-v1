// registry.go (excerpt)
package agent

import (
    "sync"
)

type Registry struct {
    mu      sync.RWMutex
    agents  map[string]*Agent
}

func NewRegistry() *Registry {
    return &Registry{
        agents: make(map[string]*Agent),
    }
}

func (r *Registry) Register(a *Agent) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.agents[a.ID] = a
}

// Get returns an agent by ID, or nil if not found.
func (r *Registry) Get(id string) *Agent {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.agents[id]
}