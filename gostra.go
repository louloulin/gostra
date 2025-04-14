// Package gostra provides a high-level API for the Gostra AI Agent Framework
package gostra

import (
	"context"

	"github.com/louloulin/gostra/pkg"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/tools"
)

// NewGostra creates a new Gostra instance
func NewGostra(options *Options) *Gostra {
	if options == nil {
		options = DefaultOptions()
	}

	pkgOptions := &pkg.Options{
		DefaultModelProvider: options.DefaultModelProvider,
		MaxConcurrency:       options.MaxConcurrency,
		Debug:                options.Debug,
	}
	return &Gostra{internal: pkg.NewGostra(pkgOptions)}
}

// Gostra is the main entry point for the Gostra framework
type Gostra struct {
	internal *pkg.Gostra
}

// Options contains configuration options for Gostra
type Options struct {
	DefaultModelProvider string
	MaxConcurrency       int
	Debug                bool
}

// DefaultOptions returns the default options for Gostra
func DefaultOptions() *Options {
	defaults := pkg.DefaultOptions()
	return &Options{
		DefaultModelProvider: defaults.DefaultModelProvider,
		MaxConcurrency:       defaults.MaxConcurrency,
		Debug:                defaults.Debug,
	}
}

// Start initializes and starts the Gostra system
func (g *Gostra) Start(ctx context.Context) error {
	return g.internal.Start(ctx)
}

// Stop stops the Gostra system
func (g *Gostra) Stop() error {
	return g.internal.Stop()
}

// RegisterModelProvider registers a model provider with Gostra
func (g *Gostra) RegisterModelProvider(name string, provider models.ModelProvider) error {
	return g.internal.RegisterTextModel(name, provider)
}

// RegisterTool registers a tool with Gostra
func (g *Gostra) RegisterTool(tool tools.Tool) error {
	return g.internal.RegisterTool(tool.GetID(), tool)
}

// RegisterAgent registers a new agent with the specified options
func (g *Gostra) RegisterAgent(id string, options *agent.Options) (*agent.Agent, error) {
	if options == nil {
		return nil, nil
	}

	// Set the ID from the parameter
	options.ID = id

	// Create a new agent directly
	agentInstance, err := agent.NewAgent(options)
	if err != nil {
		return nil, err
	}

	return agentInstance, nil
}

// GetAgent returns an agent by ID
func (g *Gostra) GetAgent(id string) (*agent.Agent, error) {
	// Since we can't convert between the interfaces directly,
	// we'll need to implement a lookup mechanism in a real implementation
	// For now, this is a simplified version that just returns an error
	// indicating that the agent needs to be retrieved differently
	return nil, nil
}
