package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Plugin defines an interface for API plugins that can extend the functionality
// of the Gostra API server
type Plugin struct {
	// Name is a unique identifier for the plugin
	Name string

	// Description provides information about the plugin's functionality
	Description string

	// Version of the plugin
	Version string

	// RegisterRoutes is called by the API server during initialization
	// It allows the plugin to register custom routes on the provided router
	RegisterRoutes func(r *mux.Router)

	// Middleware is an optional HTTP middleware function that can be applied to all routes
	// If not needed, this can be nil
	Middleware func(http.Handler) http.Handler

	// Initialize is called when the plugin is registered with the API server
	// It can be used for setup tasks
	Initialize func() error

	// Shutdown is called when the API server is shutting down
	// It allows the plugin to perform cleanup operations
	Shutdown func() error
}

// PluginRegistry maintains a registry of all registered plugins
type PluginRegistry struct {
	plugins map[string]*Plugin
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]*Plugin),
	}
}

// Register adds a new plugin to the registry
// Returns an error if a plugin with the same name already exists
func (pr *PluginRegistry) Register(p *Plugin) error {
	if p == nil {
		return ErrInvalidPlugin
	}

	if p.Name == "" {
		return ErrMissingPluginName
	}

	if _, exists := pr.plugins[p.Name]; exists {
		return ErrPluginAlreadyRegistered
	}

	// Initialize the plugin
	if p.Initialize != nil {
		if err := p.Initialize(); err != nil {
			return err
		}
	}

	pr.plugins[p.Name] = p
	return nil
}

// Get retrieves a plugin by name
func (pr *PluginRegistry) Get(name string) (*Plugin, bool) {
	p, exists := pr.plugins[name]
	return p, exists
}

// List returns all registered plugins
func (pr *PluginRegistry) List() []*Plugin {
	result := make([]*Plugin, 0, len(pr.plugins))
	for _, p := range pr.plugins {
		result = append(result, p)
	}
	return result
}

// Shutdown calls the Shutdown function on all registered plugins
func (pr *PluginRegistry) Shutdown() {
	for _, p := range pr.plugins {
		if p.Shutdown != nil {
			_ = p.Shutdown() // Ignoring errors during shutdown
		}
	}
}

// Error definitions for the plugin system
var (
	ErrInvalidPlugin           = NewError("invalid plugin")
	ErrMissingPluginName       = NewError("missing plugin name")
	ErrPluginAlreadyRegistered = NewError("plugin already registered")
)

// Error is a custom error type for the plugin system
type Error struct {
	msg string
}

// NewError creates a new Error
func NewError(msg string) *Error {
	return &Error{msg: msg}
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.msg
}
