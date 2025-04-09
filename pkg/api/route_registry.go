package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

// RouteMethod represents HTTP methods
type RouteMethod string

// HTTP methods
const (
	MethodGet     RouteMethod = "GET"
	MethodPost    RouteMethod = "POST"
	MethodPut     RouteMethod = "PUT"
	MethodDelete  RouteMethod = "DELETE"
	MethodPatch   RouteMethod = "PATCH"
	MethodOptions RouteMethod = "OPTIONS"
	MethodHead    RouteMethod = "HEAD"
)

// RouteContext represents the context for a route handler
type RouteContext struct {
	Request  *http.Request
	Response http.ResponseWriter
	Params   map[string]string
	Context  context.Context
}

// HandlerFunc defines the function signature for route handlers
type HandlerFunc func(ctx *RouteContext) (interface{}, error)

// RouteConfig defines a custom API route
type RouteConfig struct {
	Path        string
	Method      RouteMethod
	Handler     HandlerFunc
	Middlewares []func(http.Handler) http.Handler
	Description string
}

// RouteRegistry manages custom API routes
type RouteRegistry struct {
	routes     []RouteConfig
	systemPath string
	mu         sync.RWMutex
}

// NewRouteRegistry creates a new route registry
func NewRouteRegistry(systemPath string) *RouteRegistry {
	return &RouteRegistry{
		routes:     []RouteConfig{},
		systemPath: systemPath,
	}
}

// Register adds a new route to the registry
func (rr *RouteRegistry) Register(route RouteConfig) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.routes = append(rr.routes, route)
}

// GetRoutes returns all registered routes
func (rr *RouteRegistry) GetRoutes() []RouteConfig {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	// Return a copy to avoid race conditions
	routes := make([]RouteConfig, len(rr.routes))
	copy(routes, rr.routes)
	return routes
}

// IsSystemPath checks if a path is within the system path prefix
func (rr *RouteRegistry) IsSystemPath(path string) bool {
	// All routes starting with the systemPath (typically "/api/") are system routes
	return len(path) >= len(rr.systemPath) && path[:len(rr.systemPath)] == rr.systemPath
}

// ApplyRoutes registers all routes with a mux Router
func (rr *RouteRegistry) ApplyRoutes(router *mux.Router) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	for _, route := range rr.routes {
		handler := createHTTPHandlerFromRouteConfig(route)

		// Apply middlewares in reverse order (last added is first executed)
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// Register the route with the appropriate method
		r := router.HandleFunc(route.Path, handler.ServeHTTP)

		// Add method restriction
		switch route.Method {
		case MethodGet:
			r.Methods("GET")
		case MethodPost:
			r.Methods("POST")
		case MethodPut:
			r.Methods("PUT")
		case MethodDelete:
			r.Methods("DELETE")
		case MethodPatch:
			r.Methods("PATCH")
		case MethodOptions:
			r.Methods("OPTIONS")
		case MethodHead:
			r.Methods("HEAD")
		}
	}
}

// createHTTPHandlerFromRouteConfig converts a RouteConfig to an http.Handler
func createHTTPHandlerFromRouteConfig(route RouteConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create the route context
		routeCtx := &RouteContext{
			Request:  r,
			Response: w,
			Params:   extractURLParams(r),
			Context:  r.Context(),
		}

		// Call the handler function
		result, err := route.Handler(routeCtx)
		if err != nil {
			// Handle error
			httpErr, ok := err.(*HTTPError)
			if ok {
				http.Error(w, httpErr.Message, httpErr.StatusCode)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// If the result is already written to the response, return
		if w.Header().Get("Content-Type") != "" {
			return
		}

		// Otherwise, write the result as JSON
		w.Header().Set("Content-Type", "application/json")
		WriteJSON(w, result)
	})
}

// extractURLParams extracts path parameters from the URL
func extractURLParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	vars := mux.Vars(r)
	for k, v := range vars {
		params[k] = v
	}
	return params
}

// Helper functions

// WriteJSON writes the data as JSON to the response writer
func WriteJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// Use encoder to write JSON directly to the response writer
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Convenience methods for creating responses

// JSON creates a JSON response
func (ctx *RouteContext) JSON(statusCode int, data interface{}) (interface{}, error) {
	ctx.Response.WriteHeader(statusCode)
	return data, nil
}

// Text creates a plain text response
func (ctx *RouteContext) Text(statusCode int, text string) (interface{}, error) {
	ctx.Response.Header().Set("Content-Type", "text/plain")
	ctx.Response.WriteHeader(statusCode)
	ctx.Response.Write([]byte(text))
	return nil, nil
}

// Error creates an error response
func (ctx *RouteContext) Error(statusCode int, message string) (interface{}, error) {
	return nil, &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// HTTPError represents an HTTP error
type HTTPError struct {
	StatusCode int
	Message    string
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	return e.Message
}

// GetRequestBody decodes the request body into the provided interface
func (ctx *RouteContext) GetRequestBody(v interface{}) error {
	return json.NewDecoder(ctx.Request.Body).Decode(v)
}
