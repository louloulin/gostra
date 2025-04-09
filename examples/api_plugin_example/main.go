package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/yourusername/gostra/pkg"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/api"
)

// CustomResponseData is our custom data structure
type CustomResponseData struct {
	Timestamp string   `json:"timestamp"`
	Features  []string `json:"features"`
	Count     int      `json:"count"`
}

func main() {
	// Create new Gostra instance
	gostra := pkg.NewGostra()

	// Create API server with default options
	server := api.NewServer(gostra, nil)

	// Create a custom plugin
	statusPlugin := &api.Plugin{
		Name:        "status",
		Description: "Provides extended status information about the system",
		Version:     "1.0.0",

		// Initialize is called when the plugin is registered
		Initialize: func() error {
			log.Println("Initializing status plugin...")
			return nil
		},

		// RegisterRoutes adds custom routes to the provided router
		RegisterRoutes: func(r *mux.Router) {
			r.HandleFunc("/system", handleSystemStatus).Methods("GET")
			r.HandleFunc("/metrics", handleMetrics).Methods("GET")

			// Grouped routes with parameters
			reports := r.PathPrefix("/reports").Subrouter()
			reports.HandleFunc("/{type}", handleReports).Methods("GET")
		},

		// Middleware that will be applied to all routes in this plugin
		Middleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Log the request to this plugin
				log.Printf("[Status Plugin] %s %s", r.Method, r.URL.Path)

				// Set custom header for all plugin responses
				w.Header().Set("X-Plugin", "status-1.0.0")

				// Call the next handler
				next.ServeHTTP(w, r)
			})
		},

		// Shutdown is called when the API server is shutting down
		Shutdown: func() error {
			log.Println("Shutting down status plugin...")
			return nil
		},
	}

	// Register the plugin
	if err := server.RegisterPlugin(statusPlugin); err != nil {
		log.Fatalf("Failed to register plugin: %v", err)
	}

	// Create another plugin for custom agent operations
	agentPlugin := &api.Plugin{
		Name:        "agent-tools",
		Description: "Provides additional agent operations",
		Version:     "0.5.0",

		Initialize: func() error {
			log.Println("Initializing agent-tools plugin...")
			return nil
		},

		RegisterRoutes: func(r *mux.Router) {
			r.HandleFunc("/batch-run", handleBatchRun).Methods("POST")
			r.HandleFunc("/performance", handleAgentPerformance).Methods("GET")
		},

		Shutdown: func() error {
			log.Println("Shutting down agent-tools plugin...")
			return nil
		},
	}

	// Register the second plugin
	if err := server.RegisterPlugin(agentPlugin); err != nil {
		log.Fatalf("Failed to register plugin: %v", err)
	}

	// Start the server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline
	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// Handler functions for the status plugin
func handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	data := CustomResponseData{
		Timestamp: time.Now().Format(time.RFC3339),
		Features:  []string{"API", "Agent", "RAG", "Tools", "Workflow"},
		Count:     5,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"uptime":           "3h 24m 12s",
		"requests":         1245,
		"average_latency":  "120ms",
		"memory_usage":     "256MB",
		"active_threads":   8,
		"active_agents":    3,
		"total_operations": 5230,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func handleReports(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportType := vars["type"]

	var data interface{}

	switch reportType {
	case "usage":
		data = map[string]interface{}{
			"daily_active_users": 120,
			"api_calls":          5432,
			"peak_time":          "14:00-15:00",
		}
	case "performance":
		data = map[string]interface{}{
			"p50_latency": "80ms",
			"p95_latency": "150ms",
			"p99_latency": "300ms",
		}
	case "errors":
		data = map[string]interface{}{
			"count":      23,
			"top_error":  "connection_timeout",
			"error_rate": "0.42%",
		}
	default:
		http.Error(w, fmt.Sprintf("Unknown report type: %s", reportType), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// Handler functions for the agent-tools plugin
func handleBatchRun(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var requests []struct {
		AgentName string            `json:"agent_name"`
		Input     string            `json:"input"`
		Options   *agent.RunOptions `json:"options,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Simulate batch processing
	results := make([]map[string]interface{}, 0, len(requests))

	for _, req := range requests {
		// In a real implementation, this would call the actual agent run logic
		result := map[string]interface{}{
			"agent_name": req.AgentName,
			"status":     "success",
			"output":     fmt.Sprintf("Processed input: %s", req.Input),
			"timestamp":  time.Now().Format(time.RFC3339),
		}

		results = append(results, result)

		// Simulate processing time
		time.Sleep(100 * time.Millisecond)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"batch_id":     "batch_" + time.Now().Format("20060102150405"),
		"total":        len(requests),
		"completed":    len(results),
		"results":      results,
		"elapsed_time": fmt.Sprintf("%dms", 100*len(requests)),
	})
}

func handleAgentPerformance(w http.ResponseWriter, r *http.Request) {
	// Get agent name from query parameters
	agentName := r.URL.Query().Get("name")

	// Get timeframe from query parameters (default to "day")
	timeframe := r.URL.Query().Get("timeframe")
	if timeframe == "" {
		timeframe = "day"
	}

	// Prepare performance data (simulated)
	performance := map[string]interface{}{
		"agent_name": agentName,
		"timeframe":  timeframe,
		"metrics": map[string]interface{}{
			"total_runs":       126,
			"average_duration": "1.2s",
			"success_rate":     "98.4%",
			"error_rate":       "1.6%",
			"tool_usage": map[string]int{
				"search":    45,
				"calculate": 32,
				"fetch":     28,
				"other":     21,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(performance)
}
