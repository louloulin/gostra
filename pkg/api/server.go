package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// ServerOptions holds configuration options for the API server
type ServerOptions struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	EnableCORS      bool
	AllowedOrigins  []string
	AllowedMethods  []string
	AllowedHeaders  []string
	ExposedHeaders  []string
	SystemAPIPrefix string
}

// DefaultServerOptions returns a default server configuration
func DefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		Host:            "localhost",
		Port:            8000,
		ReadTimeout:     time.Second * 15,
		WriteTimeout:    time.Second * 15,
		IdleTimeout:     time.Second * 60,
		EnableCORS:      true,
		AllowedOrigins:  []string{"*"},
		AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:  []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:  []string{"Link"},
		SystemAPIPrefix: "/api/",
	}
}

// Server represents an HTTP server for the Gostra API
type Server struct {
	options          *ServerOptions
	router           *mux.Router
	gostra           interface{}
	middleware       []func(http.Handler) http.Handler
	httpServer       *http.Server
	routeRegistry    *RouteRegistry
	pluginRegistry   *PluginRegistry
	shutdownHandlers []func() error
}

// NewServer creates a new API server
func NewServer(gostra interface{}, options *ServerOptions) *Server {
	if options == nil {
		options = DefaultServerOptions()
	}

	return &Server{
		options:          options,
		router:           mux.NewRouter(),
		gostra:           gostra,
		middleware:       []func(http.Handler) http.Handler{},
		routeRegistry:    NewRouteRegistry(options.SystemAPIPrefix),
		pluginRegistry:   NewPluginRegistry(),
		shutdownHandlers: []func() error{},
	}
}

// AddMiddleware adds a middleware function to the server
func (s *Server) AddMiddleware(middleware func(http.Handler) http.Handler) {
	s.middleware = append(s.middleware, middleware)
}

// RegisterAPIRoute registers a custom API route
func (s *Server) RegisterAPIRoute(path string, method RouteMethod, handler HandlerFunc, description string) {
	if s.routeRegistry.IsSystemPath(path) {
		panic(fmt.Sprintf("Cannot register custom route at system path: %s", path))
	}

	s.routeRegistry.Register(RouteConfig{
		Path:        path,
		Method:      method,
		Handler:     handler,
		Description: description,
	})
}

// RegisterPlugin registers a plugin with the server
func (s *Server) RegisterPlugin(plugin *Plugin) error {
	return s.pluginRegistry.Register(plugin)
}

// AddShutdownHandler adds a function to be called during server shutdown
func (s *Server) AddShutdownHandler(handler func() error) {
	s.shutdownHandlers = append(s.shutdownHandlers, handler)
}

// Start starts the server
func (s *Server) Start() error {
	r := mux.NewRouter()

	// Register API routes
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.StrictSlash(true)

	// Health check endpoint
	apiRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	// v1 API endpoints
	v1Router := apiRouter.PathPrefix("/v1").Subrouter()

	// Agents endpoints
	agentsRouter := v1Router.PathPrefix("/agents").Subrouter()
	agentsRouter.HandleFunc("", s.handleListAgents).Methods("GET")
	agentsRouter.HandleFunc("", s.handleCreateAgent).Methods("POST")
	agentsRouter.HandleFunc("/{agentID}", s.handleGetAgent).Methods("GET")
	agentsRouter.HandleFunc("/{agentID}", s.handleDeleteAgent).Methods("DELETE")
	agentsRouter.HandleFunc("/{agentID}/generate", s.handleGenerateResponse).Methods("POST")
	agentsRouter.HandleFunc("/{agentID}/stream", s.handleStreamResponse).Methods("POST")

	// Register custom routes from route registry
	s.routeRegistry.ApplyRoutes(r)

	// Register routes from plugins
	for _, plugin := range s.pluginRegistry.List() {
		if plugin.RegisterRoutes != nil {
			plugin.RegisterRoutes(r)
		}
	}

	// Apply custom middleware
	var handler http.Handler = r
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}

	// Apply plugin middleware
	for _, plugin := range s.pluginRegistry.List() {
		if plugin.Middleware != nil {
			handler = plugin.Middleware(handler)
		}
	}

	// Setup CORS if enabled
	if s.options.EnableCORS {
		handler = s.applyCORSMiddleware(handler)
	}

	addr := fmt.Sprintf("%s:%d", s.options.Host, s.options.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  s.options.ReadTimeout,
		WriteTimeout: s.options.WriteTimeout,
		IdleTimeout:  s.options.IdleTimeout,
	}

	fmt.Printf("Server starting on %s\n", addr)
	return s.httpServer.ListenAndServe()
}

// applyCORSMiddleware adds CORS support to the handler
func (s *Server) applyCORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", s.corsOriginHeader())
		w.Header().Set("Access-Control-Allow-Methods", s.corsMethodsHeader())
		w.Header().Set("Access-Control-Allow-Headers", s.corsHeadersHeader())
		w.Header().Set("Access-Control-Expose-Headers", s.corsExposeHeader())
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "300")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// CORS header helper methods
func (s *Server) corsOriginHeader() string {
	return joinStrings(s.options.AllowedOrigins, ", ")
}

func (s *Server) corsMethodsHeader() string {
	return joinStrings(s.options.AllowedMethods, ", ")
}

func (s *Server) corsHeadersHeader() string {
	return joinStrings(s.options.AllowedHeaders, ", ")
}

func (s *Server) corsExposeHeader() string {
	return joinStrings(s.options.ExposedHeaders, ", ")
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	// Call shutdown handlers
	for _, handler := range s.shutdownHandlers {
		if err := handler(); err != nil {
			fmt.Printf("Error in shutdown handler: %v\n", err)
		}
	}

	// Shutdown plugins
	s.pluginRegistry.Shutdown()

	// Shutdown server
	if s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}

// Handler implementations
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	// Implementation depends on the Gostra interface
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]map[string]string{{"id": "sample-agent", "name": "Sample Agent"}})
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement agent creation
	w.WriteHeader(http.StatusNotImplemented)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"error": "Not implemented"})
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentID"]

	// Implementation depends on the Gostra interface
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": agentID, "name": "Sample Agent"})
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentID"]

	// Implementation depends on the Gostra interface
	fmt.Printf("Deleting agent: %s\n", agentID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGenerateResponse(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentID"]

	var req struct {
		Messages []map[string]string    `json:"messages"`
		Options  map[string]interface{} `json:"options,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Implementation depends on the Gostra interface
	fmt.Printf("Generating response for agent: %s\n", agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"content": "This is a sample response from the agent.",
	})
}

func (s *Server) handleStreamResponse(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["agentID"]

	var req struct {
		Messages []map[string]string    `json:"messages"`
		Options  map[string]interface{} `json:"options,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Implementation depends on the Gostra interface
	fmt.Printf("Streaming response for agent: %s\n", agentID)

	// Set headers for streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Example streaming
	events := []string{
		`{"type":"content","content":"This "}`,
		`{"type":"content","content":"is "}`,
		`{"type":"content","content":"a "}`,
		`{"type":"content","content":"streaming "}`,
		`{"type":"content","content":"response."}`,
		`{"type":"done"}`,
	}

	for _, event := range events {
		fmt.Fprintf(w, "data: %s\n\n", event)
		w.(http.Flusher).Flush()
		time.Sleep(200 * time.Millisecond)
	}
}

// Helper function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
