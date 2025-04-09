package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// TestActorHandler tests the ActorHandler's ServeHTTP method
type TestResponseActor struct {
	response *ActorResponse
}

func (a *TestResponseActor) Receive(ctx actor.Context) {
	switch ctx.Message().(type) {
	case *ActorMessage:
		// Just respond with the predefined response
		ctx.Respond(a.response)
	}
}

func TestActorHandler_ServeHTTP(t *testing.T) {
	// Create an actor system
	system := actor.NewActorSystem()

	// Test cases
	tests := []struct {
		name           string
		method         string
		url            string
		body           map[string]interface{}
		actorResponse  *ActorResponse
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:   "Success response",
			method: "GET",
			url:    "/api/test?param=value",
			actorResponse: &ActorResponse{
				StatusCode: http.StatusOK,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body: map[string]interface{}{
					"result": "success",
					"data":   "test data",
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"result": "success",
				"data":   "test data",
			},
		},
		{
			name:   "Error response",
			method: "POST",
			url:    "/api/test",
			body: map[string]interface{}{
				"test": "data",
			},
			actorResponse: &ActorResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body: map[string]interface{}{
					"error": "Invalid request",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error": "Invalid request",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test actor with predefined response
			props := actor.PropsFromProducer(func() actor.Actor {
				return &TestResponseActor{
					response: tt.actorResponse,
				}
			})
			pid := system.Root.Spawn(props)
			defer system.Root.Stop(pid)

			// Create the handler
			handler := &ActorHandler{
				actorSystem: system,
				actorPID:    pid,
				timeout:     time.Second,
			}

			// Create a request
			var requestBody []byte
			if tt.body != nil {
				requestBody, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.url, bytes.NewBuffer(requestBody))
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			// Create a response recorder
			w := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Check response body
			if tt.expectedBody != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, response)
			}

			// Check Content-Type header
			if contentType, ok := tt.actorResponse.Headers["Content-Type"]; ok {
				assert.Equal(t, contentType, w.Header().Get("Content-Type"))
			}
		})
	}
}

// TestNewActorMessage tests the NewActorMessage function
func TestNewActorMessage(t *testing.T) {
	// Test cases
	tests := []struct {
		name         string
		method       string
		url          string
		headers      map[string]string
		body         map[string]interface{}
		expectedPath string
		expectedQP   map[string][]string
	}{
		{
			name:   "GET request with query parameters",
			method: "GET",
			url:    "/api/test?param1=value1&param2=value2",
			headers: map[string]string{
				"X-Test-Header": "test",
			},
			expectedPath: "/api/test",
			expectedQP: map[string][]string{
				"param1": {"value1"},
				"param2": {"value2"},
			},
		},
		{
			name:   "POST request with body",
			method: "POST",
			url:    "/api/create",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body: map[string]interface{}{
				"name":  "Test",
				"value": 123,
			},
			expectedPath: "/api/create",
			expectedQP:   map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			var requestBody []byte
			if tt.body != nil {
				requestBody, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.url, bytes.NewBuffer(requestBody))

			// Add headers
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Create the ActorMessage
			message, err := NewActorMessage(req)
			assert.NoError(t, err)

			// Check the message properties
			assert.Equal(t, tt.method, message.Method)
			assert.Equal(t, tt.expectedPath, message.Path)

			// Check query parameters
			for k, v := range tt.expectedQP {
				assert.Equal(t, v, message.QueryParams[k])
			}

			// Check headers
			for k, v := range tt.headers {
				assert.Equal(t, v, message.Headers[k])
			}

			// Check body if provided
			if tt.body != nil {
				assert.Equal(t, tt.body, message.Body)
			}
		})
	}
}

// TestAPIHandlerActor tests the APIHandlerActor
func TestAPIHandlerActor(t *testing.T) {
	// Create a test handler function
	handlerFunc := func(msg *ActorMessage) (*ActorResponse, error) {
		// Echo back the message body with OK status
		return &ActorResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       msg.Body,
		}, nil
	}

	// Create the actor system
	system := actor.NewActorSystem()

	// Create the actor with the handler function
	props := NewAPIHandlerActor(handlerFunc)
	pid := system.Root.Spawn(props)
	defer system.Root.Stop(pid)

	// Create a test message
	testMessage := &ActorMessage{
		Method: "POST",
		Path:   "/api/test",
		Body: map[string]interface{}{
			"test": "value",
		},
		Headers: map[string][]string{
			"Content-Type": []string{"application/json"},
		},
		QueryParams: make(map[string][]string),
	}

	// Send the message to the actor and wait for response
	future := system.Root.RequestFuture(pid, testMessage, 1*time.Second)
	result, err := future.Result()

	// Assert no error occurred
	assert.NoError(t, err)

	// Assert we got a response of the expected type
	response, ok := result.(*ActorResponse)
	assert.True(t, ok)

	// Verify the response matches what we expect
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "application/json", response.Headers["Content-Type"])

	// Verify the body was echoed back
	bodyMap, ok := response.Body.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", bodyMap["test"])
}

// TestRegisterActorRoute tests the RegisterActorRoute method
func TestRegisterActorRoute(t *testing.T) {
	// Create a server
	config := DefaultServerConfigV2()
	server := NewServerV2(nil, config)

	// Create an actor system
	system := actor.NewActorSystem()

	// Create test actor props
	props := actor.PropsFromProducer(func() actor.Actor {
		return &TestResponseActor{
			response: &ActorResponse{
				StatusCode: http.StatusOK,
				Body:       map[string]string{"result": "success"},
			},
		}
	})

	// Spawn the actor
	pid := system.Root.Spawn(props)
	defer system.Root.Stop(pid)

	// Register the actor route
	path := "/api/actor-test"
	method := MethodGet
	server.RegisterActorRoute(path, method, pid, 1*time.Second)

	// Verify the route was registered
	routes := server.routeRegistry.GetRoutes()
	assert.True(t, len(routes) > 0)

	// Make a request to test the route
	req := httptest.NewRequest(string(method), path, nil)
	w := httptest.NewRecorder()

	// Create a test router and register the handler
	router := mux.NewRouter()
	server.routeRegistry.ApplyRoutes(router)

	// Serve the request
	router.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["result"])
}

// MockAgentProvider is a mock implementation of the AgentProvider interface for testing
type MockAgentProvider struct {
	agents map[string]*actor.PID
}

func NewMockAgentProvider() *MockAgentProvider {
	return &MockAgentProvider{
		agents: make(map[string]*actor.PID),
	}
}

func (m *MockAgentProvider) GetAgent(name string) (interface{}, error) {
	agent, ok := m.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}
	return agent, nil
}

func (m *MockAgentProvider) RegisterAgent(name string, pid *actor.PID) {
	m.agents[name] = pid
}

// TestRegisterCustomAPIRoute tests the RegisterCustomAPIRoute method
func TestRegisterCustomAPIRoute(t *testing.T) {
	// Create a server
	config := DefaultServerConfigV2()
	server := NewServerV2(nil, config)

	// Define a custom handler function
	handlerFunc := func(msg *ActorMessage) (*ActorResponse, error) {
		return &ActorResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"message": "Custom route handled successfully"},
		}, nil
	}

	// Register a custom API route
	path := "/api/custom"
	server.RegisterCustomAPIRoute(path, MethodPost, handlerFunc, "Test custom route")

	// Get routes from registry
	routes := server.routeRegistry.GetRoutes()
	assert.True(t, len(routes) > 0, "No routes registered")

	// Create a test request
	req := httptest.NewRequest(string(MethodPost), path, nil)
	w := httptest.NewRecorder()

	// Create a router and apply routes
	router := mux.NewRouter()
	server.routeRegistry.ApplyRoutes(router)

	// Serve the request
	router.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Custom route handled successfully", response["message"])
}

// TestRegisterAgentRoute tests the RegisterAgentRoute method
func TestRegisterAgentRoute(t *testing.T) {
	// Create an actor system
	system := actor.NewActorSystem()

	// Create a mock agent
	props := actor.PropsFromProducer(func() actor.Actor {
		return &TestResponseActor{
			response: &ActorResponse{
				StatusCode: http.StatusOK,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"agent": "response"},
			},
		}
	})
	agentPID := system.Root.Spawn(props)
	defer system.Root.Stop(agentPID)

	// Create a mock agent provider
	mockProvider := NewMockAgentProvider()
	mockProvider.RegisterAgent("test-agent", agentPID)

	// Create a server with the mock provider
	config := DefaultServerConfigV2()
	server := NewServerV2(mockProvider, config)

	// Register an agent route
	path := "/api/agent-route"
	err := server.RegisterAgentRoute(path, MethodGet, "test-agent", 1*time.Second)
	assert.NoError(t, err)

	// Get routes from registry
	routes := server.routeRegistry.GetRoutes()
	assert.True(t, len(routes) > 0, "No routes registered")

	// Create a test request
	req := httptest.NewRequest(string(MethodGet), path, nil)
	w := httptest.NewRecorder()

	// Create a router and apply routes
	router := mux.NewRouter()
	server.routeRegistry.ApplyRoutes(router)

	// Serve the request
	router.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "response", response["agent"])
}

// TestCreateWebhookRoute tests the CreateWebhookRoute method
func TestCreateWebhookRoute(t *testing.T) {
	// Create a server
	config := DefaultServerConfigV2()
	server := NewServerV2(nil, config)

	// Create an actor system
	system := actor.NewActorSystem()

	// Create a webhook receiver actor
	props := actor.PropsFromProducer(func() actor.Actor {
		return &TestResponseActor{
			response: &ActorResponse{
				StatusCode: http.StatusOK,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"webhook": "received"},
			},
		}
	})
	webhookPID := system.Root.Spawn(props)
	defer system.Root.Stop(webhookPID)

	// Register a webhook route
	webhookPath := "/api/webhook"
	server.CreateWebhookRoute(webhookPath, webhookPID)

	// Test POST request to webhook
	req := httptest.NewRequest(string(MethodPost), webhookPath, nil)
	w := httptest.NewRecorder()

	// Create a router and apply routes
	router := mux.NewRouter()
	server.routeRegistry.ApplyRoutes(router)

	// Serve the request
	router.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "received", response["webhook"])

	// Test OPTIONS request for CORS preflight
	reqOptions := httptest.NewRequest(string(MethodOptions), webhookPath, nil)
	wOptions := httptest.NewRecorder()

	// Serve the OPTIONS request
	router.ServeHTTP(wOptions, reqOptions)

	// Verify the OPTIONS response
	assert.Equal(t, http.StatusOK, wOptions.Code)
	assert.Equal(t, "*", wOptions.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "POST, OPTIONS", wOptions.Header().Get("Access-Control-Allow-Methods"))
}
