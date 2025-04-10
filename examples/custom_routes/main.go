package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/louloulin/gostra/pkg/api"
)

// WeatherResponse represents a weather forecast response
type WeatherResponse struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Conditions  string  `json:"conditions"`
	Forecast    string  `json:"forecast"`
}

func main() {
	// Create Server v2 with our enhanced API route functionality
	config := api.DefaultServerConfigV2()
	config.Port = 8080

	// Create a server instance
	server := api.NewServerV2(nil, config)

	// Register a custom route for getting weather
	server.RegisterAPIRoute("/weather/forecast", api.MethodGet, handleWeatherForecast, "Get weather forecast for a location")

	// Register a custom route for creating an alert
	server.RegisterAPIRoute("/alerts/create", api.MethodPost, handleCreateAlert, "Create a new alert")

	// Add CORS middleware
	server.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Request: %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

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

	log.Println("Server stopped")
}

// handleWeatherForecast handles requests for weather forecasts
func handleWeatherForecast(ctx *api.RouteContext) (interface{}, error) {
	// Get the location from query parameters
	location := ctx.Request.URL.Query().Get("location")
	if location == "" {
		return nil, &api.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Location parameter is required",
		}
	}

	// In a real implementation, we would call a weather service here
	// For the example, we'll return mock data
	response := WeatherResponse{
		Location:    location,
		Temperature: 22.5,
		Conditions:  "Sunny",
		Forecast:    "Clear skies expected for the next 24 hours",
	}

	return response, nil
}

// handleCreateAlert handles requests to create new alerts
func handleCreateAlert(ctx *api.RouteContext) (interface{}, error) {
	// Parse request body
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Severity    string   `json:"severity"`
		Tags        []string `json:"tags"`
	}

	if err := ctx.GetRequestBody(&req); err != nil {
		return nil, &api.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body",
		}
	}

	// Validate the request
	if req.Title == "" {
		return nil, &api.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "Title is required",
		}
	}

	// In a real implementation, we would store the alert in a database
	// For the example, we'll return a mock response
	response := map[string]interface{}{
		"id":          "alert-123",
		"title":       req.Title,
		"description": req.Description,
		"severity":    req.Severity,
		"tags":        req.Tags,
		"created_at":  time.Now().Format(time.RFC3339),
		"status":      "active",
	}

	return response, nil
}
