package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/api"
)

// Define a custom agent actor for handling weather information
type WeatherAgent struct{}

func (a *WeatherAgent) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *api.ActorMessage:
		// Get the city from query parameters
		location := "Unknown"
		if cities, ok := msg.QueryParams["city"]; ok && len(cities) > 0 {
			location = cities[0]
		}

		// Generate a response with weather information
		response := &api.ActorResponse{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"location":    location,
				"temperature": 22.5,
				"conditions":  "Sunny",
				"timestamp":   time.Now().Format(time.RFC3339),
			},
		}

		ctx.Respond(response)
	}
}

// Define a customer service agent for handling support requests
type CustomerServiceAgent struct{}

func (a *CustomerServiceAgent) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *api.ActorMessage:
		// Extract body information if available
		var customerName string
		var issue string

		if msg.Body != nil {
			if body, ok := msg.Body.(map[string]interface{}); ok {
				if name, ok := body["customer_name"].(string); ok {
					customerName = name
				}
				if issueDesc, ok := body["issue"].(string); ok {
					issue = issueDesc
				}
			}
		}

		// If no customer name was provided, use a default
		if customerName == "" {
			customerName = "Valued Customer"
		}

		// If no issue was provided, use a default
		if issue == "" {
			issue = "unspecified issue"
		}

		// Generate a support ticket ID
		ticketID := fmt.Sprintf("TICKET-%d", time.Now().UnixNano()%10000)

		// Create a response
		response := &api.ActorResponse{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"ticket_id":      ticketID,
				"customer":       customerName,
				"issue":          issue,
				"created_at":     time.Now().Format(time.RFC3339),
				"status":         "open",
				"message":        fmt.Sprintf("Thank you %s, your support request for '%s' has been received.", customerName, issue),
				"estimated_wait": "24 hours",
			},
		}

		ctx.Respond(response)
	}
}

// Simple notification actor for handling webhook events
type NotificationActor struct {
	notifications []map[string]interface{}
}

func (a *NotificationActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *api.ActorMessage:
		// Add the notification to our collection
		notification := map[string]interface{}{
			"received_at": time.Now().Format(time.RFC3339),
			"data":        msg.Body,
			"headers":     msg.Headers,
		}
		a.notifications = append(a.notifications, notification)

		// Respond with success
		response := &api.ActorResponse{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"success": true,
				"message": "Notification received",
			},
		}

		ctx.Respond(response)
	}
}

// MockGostra is a mock implementation of the Gostra framework for this example
type MockGostra struct {
	agents map[string]*actor.PID
}

func NewMockGostra() *MockGostra {
	return &MockGostra{
		agents: make(map[string]*actor.PID),
	}
}

func (g *MockGostra) GetAgent(name string) (interface{}, error) {
	agent, ok := g.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}
	return agent, nil
}

func (g *MockGostra) RegisterAgent(name string, pid *actor.PID) {
	g.agents[name] = pid
}

func main() {
	// Create our actor system
	system := actor.NewActorSystem()

	// Create a mock Gostra instance for our agents
	gostra := NewMockGostra()

	// Create the API server
	config := api.DefaultServerConfigV2()
	config.Port = 8080
	server := api.NewServerV2(gostra, config)

	// Spawn the weather agent
	weatherProps := actor.PropsFromProducer(func() actor.Actor {
		return &WeatherAgent{}
	})
	weatherPID := system.Root.Spawn(weatherProps)
	gostra.RegisterAgent("weather", weatherPID)

	// Spawn the customer service agent
	customerServiceProps := actor.PropsFromProducer(func() actor.Actor {
		return &CustomerServiceAgent{}
	})
	customerServicePID := system.Root.Spawn(customerServiceProps)
	gostra.RegisterAgent("customer-service", customerServicePID)

	// Spawn the notification actor for webhooks
	notificationProps := actor.PropsFromProducer(func() actor.Actor {
		return &NotificationActor{
			notifications: make([]map[string]interface{}, 0),
		}
	})
	notificationPID := system.Root.Spawn(notificationProps)

	// Register the weather agent route - via agent name
	err := server.RegisterAgentRoute("/api/weather", api.MethodGet, "weather", 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to register weather agent route: %v", err)
	}

	// Register the customer service agent route - via agent name
	err = server.RegisterAgentRoute("/api/support", api.MethodPost, "customer-service", 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to register customer service agent route: %v", err)
	}

	// Create a webhook for notifications
	server.CreateWebhookRoute("/api/webhook/notifications", notificationPID)

	// Register a custom API route using a function handler
	server.RegisterCustomAPIRoute("/api/echo", api.MethodPost, func(msg *api.ActorMessage) (*api.ActorResponse, error) {
		// Simple echo handler that returns the body of the request
		return &api.ActorResponse{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"echo":      msg.Body,
				"timestamp": time.Now().Format(time.RFC3339),
			},
		}, nil
	}, "Echo service that returns the request body")

	// Add a standard health check route
	server.RegisterAPIRoute("/health", api.MethodGet, func(ctx *api.RouteContext) (interface{}, error) {
		return map[string]interface{}{
			"status":    "healthy",
			"uptime":    time.Now().Unix(),
			"endpoints": []string{"/api/weather", "/api/support", "/api/webhook/notifications", "/api/echo"},
		}, nil
	}, "Health check endpoint")

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on port %d", config.Port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Print usage information
	log.Println("Server running. Available endpoints:")
	log.Println("- GET  /health")
	log.Println("- GET  /api/weather?city=YourCity")
	log.Println("- POST /api/support (with JSON body: {\"customer_name\": \"Your Name\", \"issue\": \"Your Issue\"})")
	log.Println("- POST /api/webhook/notifications")
	log.Println("- POST /api/echo (echoes back the request body)")

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Wait for CTRL+C
	<-stop

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server
	log.Println("Shutting down server...")
	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	// Clean up actors
	system.Root.Stop(weatherPID)
	system.Root.Stop(customerServicePID)
	system.Root.Stop(notificationPID)

	log.Println("Server gracefully stopped")
}
