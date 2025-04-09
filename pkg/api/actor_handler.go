package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// ActorMessage is a message sent to an actor handling an API request
type ActorMessage struct {
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Headers     map[string][]string `json:"headers"`
	QueryParams map[string][]string `json:"query_params"`
	Body        interface{}         `json:"body"`
	Context     context.Context     `json:"context"`
}

// ActorResponse is a response from an actor handling an API request
type ActorResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       interface{}       `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// NewActorMessage creates a new ActorMessage from an HTTP request
func NewActorMessage(r *http.Request) (*ActorMessage, error) {
	// Parse query parameters
	queryParams := make(map[string][]string)
	for key, values := range r.URL.Query() {
		queryParams[key] = values
	}

	// Parse request body if content is not empty
	var body interface{}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
	}

	return &ActorMessage{
		Method:      r.Method,
		Path:        r.URL.Path,
		Headers:     r.Header,
		QueryParams: queryParams,
		Body:        body,
		Context:     r.Context(),
	}, nil
}

// ActorHandler handles API requests by sending them to an actor
type ActorHandler struct {
	actorSystem       *actor.ActorSystem
	actorPID          *actor.PID
	timeout           time.Duration
	defaultStatusCode int
}

// NewActorHandler creates a new ActorHandler
func NewActorHandler(system *actor.ActorSystem, pid *actor.PID, timeout time.Duration) *ActorHandler {
	return &ActorHandler{
		actorSystem:       system,
		actorPID:          pid,
		timeout:           timeout,
		defaultStatusCode: http.StatusOK,
	}
}

// ServeHTTP implements the http.Handler interface
func (h *ActorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create actor message from request
	message, err := NewActorMessage(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing request: %v", err), http.StatusBadRequest)
		return
	}

	// Create context with timeout for the request
	_, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	// Send message to actor and wait for response
	future := h.actorSystem.Root.RequestFuture(h.actorPID, message, h.timeout)
	result, err := future.Result()
	if err != nil {
		http.Error(w, fmt.Sprintf("Actor error: %v", err), http.StatusInternalServerError)
		return
	}

	// Process the response
	switch response := result.(type) {
	case *ActorResponse:
		// Set response headers
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}

		// Set content type if not already set
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Set status code
		statusCode := response.StatusCode
		if statusCode == 0 {
			statusCode = h.defaultStatusCode
		}
		w.WriteHeader(statusCode)

		// Write response body
		if response.Body != nil {
			if err := json.NewEncoder(w).Encode(response.Body); err != nil {
				http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
				return
			}
		}

	default:
		// Unknown response type
		http.Error(w, "Invalid actor response", http.StatusInternalServerError)
	}
}

// RegisterActorRoute registers a route that sends requests to an actor
func (s *ServerV2) RegisterActorRoute(path string, method RouteMethod, actorPID *actor.PID, timeout time.Duration) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create an actor handler
	handler := NewActorHandler(actor.NewActorSystem(), actorPID, timeout)

	// Register the route
	s.routeRegistry.Register(RouteConfig{
		Path:   path,
		Method: method,
		Handler: func(ctx *RouteContext) (interface{}, error) {
			// This is a wrapper that will delegate to the actor handler
			// ActorHandler will handle the HTTP response directly
			handler.ServeHTTP(ctx.Response, ctx.Request)
			// Return nil to prevent the route registry from writing a response
			return nil, nil
		},
		Description: fmt.Sprintf("Actor-based handler for %s %s", method, path),
	})
}

// ActorProps is a function that creates actor props
type ActorProps func() *actor.Props

// RegisterActorRouteWithProps registers a route that creates an actor from props to handle requests
func (s *ServerV2) RegisterActorRouteWithProps(path string, method RouteMethod, propsFunc ActorProps, timeout time.Duration) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create the actor system
	system := actor.NewActorSystem()

	// Create the actor from props
	props := propsFunc()
	pid := system.Root.Spawn(props)

	// Register the route with the actor PID
	s.RegisterActorRoute(path, method, pid, timeout)

	// Add shutdown handler to stop the actor
	s.AddShutdownHandler(func() error {
		future := system.Root.StopFuture(pid)
		_, err := future.Result()
		return err
	})
}

// APIHandlerActor is an actor that handles API requests
type APIHandlerActor struct {
	// Handler function that processes the request and returns a response
	handler func(*ActorMessage) (*ActorResponse, error)
}

// NewAPIHandlerActor creates a new APIHandlerActor
func NewAPIHandlerActor(handler func(*ActorMessage) (*ActorResponse, error)) *actor.Props {
	return actor.PropsFromProducer(func() actor.Actor {
		return &APIHandlerActor{handler: handler}
	})
}

// RegisterAgentRoute registers a route that sends requests to an agent implemented as an actor
func (s *ServerV2) RegisterAgentRoute(path string, method RouteMethod, agentName string, timeout time.Duration) error {
	if s.gostra == nil {
		return fmt.Errorf("gostra instance is required to register agent routes")
	}

	// Check if we can access agents through the gostra interface
	gostraInstance, ok := s.gostra.(AgentProvider)
	if !ok {
		return fmt.Errorf("gostra instance does not implement the AgentProvider interface")
	}

	// Get the agent PID
	agent, err := gostraInstance.GetAgent(agentName)
	if err != nil {
		return fmt.Errorf("failed to get agent '%s': %v", agentName, err)
	}

	// Get the agent's PID
	agentPID, ok := agent.(*actor.PID)
	if !ok {
		return fmt.Errorf("agent '%s' is not implemented as an actor", agentName)
	}

	// Register the route
	s.RegisterActorRoute(path, method, agentPID, timeout)
	return nil
}

// AgentProvider defines the interface for Gostra instances that can provide agents
type AgentProvider interface {
	GetAgent(name string) (interface{}, error)
}

// RegisterCustomAPIRoute registers a custom API route with a handler function that
// uses the actor system for processing
func (s *ServerV2) RegisterCustomAPIRoute(path string, method RouteMethod, handler func(*ActorMessage) (*ActorResponse, error), description string) {
	// Create an actor to handle the requests
	props := NewAPIHandlerActor(handler)

	// Register the route with actor props
	s.RegisterActorRouteWithProps(path, method, func() *actor.Props {
		return props
	}, 30*time.Second)
}

// CreateWebhookRoute creates a webhook route that forwards requests to an actor
func (s *ServerV2) CreateWebhookRoute(path string, receiverPID *actor.PID) {
	// Register a POST route for the webhook
	s.RegisterActorRoute(path, MethodPost, receiverPID, 30*time.Second)

	// Register an OPTIONS route for CORS preflight requests
	s.RegisterAPIRoute(path, MethodOptions, func(ctx *RouteContext) (interface{}, error) {
		// Set CORS headers for preflight requests
		ctx.Response.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		ctx.Response.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		ctx.Response.WriteHeader(http.StatusOK)
		return nil, nil
	}, "CORS preflight handler for webhook")
}

// Receive handles messages sent to the actor
func (a *APIHandlerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *ActorMessage:
		// Process the API request
		response, err := a.handler(msg)
		if err != nil {
			// Create error response
			ctx.Respond(&ActorResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       map[string]string{"error": err.Error()},
			})
			return
		}

		// Send response
		ctx.Respond(response)
	}
}
