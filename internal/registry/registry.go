// Package registry provides thread-safe local registration and discovery
// of sub-agents for monolithic composition.
package registry

import (
	"fmt"
	"sync"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
)

// Factory is a function that instantiates a sub-agent for a given LLM model.
type Factory func(m model.LLM) (agent.Agent, error)

type AgentSpec struct {
	Name        string
	Description string
	Factory     Factory
}

var (
	mu       sync.RWMutex
	registry = make(map[string]AgentSpec)
)

// Register registers a sub-agent factory in the local registry.
func Register(name, description string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = AgentSpec{
		Name:        name,
		Description: description,
		Factory:     factory,
	}
}

// All instantiates and returns all registered sub-agents for the given LLM model.
func All(m model.LLM) ([]agent.Agent, error) {
	mu.RLock()
	defer mu.RUnlock()

	var agents []agent.Agent
	for name, spec := range registry {
		ag, err := spec.Factory(m)
		if err != nil {
			return nil, fmt.Errorf("instantiating sub-agent %q: %w", name, err)
		}
		agents = append(agents, ag)
	}
	return agents, nil
}

// Specs returns a copy of all registered agent specifications.
func Specs() []AgentSpec {
	mu.RLock()
	defer mu.RUnlock()

	var specs []AgentSpec
	for _, spec := range registry {
		specs = append(specs, spec)
	}
	return specs
}
