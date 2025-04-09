package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/api"
)

// WeatherActor handles weather forecast requests
type WeatherActor struct{}

// Receive processes incoming messages
func (a *WeatherActor) Receive(ctx actor.Context) {
	// Check if the message is an ActorMessage
	switch msg := ctx.Message().(type) {
	case *api.ActorMessage:
		// Extract the location from query parameters
		location := ""
		if locations, ok := msg.QueryParams["location"]; ok && len(locations) > 0 {
			location = locations[0]
		}

		if location == "" {
			// Return an error if location is not provided
			ctx.Respond(&api.ActorResponse{
				StatusCode: http.StatusBadRequest,
				Body:       map[string]string{"error": "Location parameter is required"},
			})
			return
		}

		// Generate a weather forecast for the location
		forecast := generateWeatherForecast(location)

		// Send the response back
		ctx.Respond(&api.ActorResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       forecast,
		})
	}
}

// AlertsActor handles alert creation requests
type AlertsActor struct{}

// Receive processes incoming messages
func (a *AlertsActor) Receive(ctx actor.Context) {
	// Check if the message is an ActorMessage
	switch msg := ctx.Message().(type) {
	case *api.ActorMessage:
		// Extract the alert request from the body
		alertReq, ok := msg.Body.(map[string]interface{})
		if !ok {
			ctx.Respond(&api.ActorResponse{
				StatusCode: http.StatusBadRequest,
				Body:       map[string]string{"error": "Invalid request body"},
			})
			return
		}

		// Check if the title is provided
		title, ok := alertReq["title"].(string)
		if !ok || title == "" {
			ctx.Respond(&api.ActorResponse{
				StatusCode: http.StatusBadRequest,
				Body:       map[string]string{"error": "Title is required"},
			})
			return
		}

		// Create the alert
		alert := createAlert(alertReq)

		// Send the response back
		ctx.Respond(&api.ActorResponse{
			StatusCode: http.StatusCreated,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       alert,
		})
	}
}

func main() {
	// Create Server v2 with our enhanced API route functionality
	config := api.DefaultServerConfigV2()
	config.Port = 8080

	// Create a server instance
	server := api.NewServerV2(nil, config)

	// Create the actor system
	system := actor.NewActorSystem()

	// Create and spawn the weather actor
	weatherProps := actor.PropsFromProducer(func() actor.Actor {
		return &WeatherActor{}
	})
	weatherPID := system.Root.Spawn(weatherProps)

	// Create and spawn the alerts actor
	alertsProps := actor.PropsFromProducer(func() actor.Actor {
		return &AlertsActor{}
	})
	alertsPID := system.Root.Spawn(alertsProps)

	// Register actor-based routes
	server.RegisterActorRoute("/api/v1/weather", api.MethodGet, weatherPID, 5*time.Second)
	server.RegisterActorRoute("/api/v1/alerts", api.MethodPost, alertsPID, 5*time.Second)

	// Add CORS middleware
	server.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Request: %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

	// Additionally, register a regular API route using the route registry
	server.RegisterAPIRoute("/api/v1/health", api.MethodGet, func(ctx *api.RouteContext) (interface{}, error) {
		return map[string]string{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		}, nil
	}, "Health check endpoint")

	// Create a functional-based actor handler
	echoHandler := func(msg *api.ActorMessage) (*api.ActorResponse, error) {
		return &api.ActorResponse{
			StatusCode: http.StatusOK,
			Body:       msg, // Echo back the received message
		}, nil
	}

	// Register a route using a functional actor handler
	server.RegisterActorRouteWithProps("/api/v1/echo", api.MethodPost, func() *actor.Props {
		return api.NewAPIHandlerActor(echoHandler)
	}, 5*time.Second)

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on port %d", config.Port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Wait for interrupt signal
	<-stop

	// Create a deadline for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shut down the server
	log.Println("Shutting down server...")
	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Error during shutdown: %v", err)
	}

	// Stop the actors explicitly
	system.Root.Stop(weatherPID)
	system.Root.Stop(alertsPID)

	log.Println("Server and actors stopped")
}

// Helper function to generate a weather forecast
func generateWeatherForecast(location string) map[string]interface{} {
	// In a real implementation, this would call a weather service
	// For the example, we'll return mock data
	return map[string]interface{}{
		"location":    location,
		"temperature": 22.5,
		"conditions":  "Sunny",
		"forecast":    "Clear skies expected for the next 24 hours",
		"timestamp":   time.Now().Format(time.RFC3339),
	}
}

// Helper function to create an alert
func createAlert(req map[string]interface{}) map[string]interface{} {
	// In a real implementation, this would store the alert in a database
	// For the example, we'll return a mock response
	return map[string]interface{}{
		"id":          "alert-123",
		"title":       req["title"],
		"description": req["description"],
		"severity":    req["severity"],
		"tags":        req["tags"],
		"created_at":  time.Now().Format(time.RFC3339),
		"status":      "active",
	}
}
