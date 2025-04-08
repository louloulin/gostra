package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourusername/gostra/pkg/models"
)

func TestNewOpenAIImageProvider(t *testing.T) {
	// Test with nil options
	_, err := NewOpenAIImageProvider(nil)
	if err == nil {
		t.Error("Expected error with nil options, got nil")
	}

	// Test with missing API key
	_, err = NewOpenAIImageProvider(&ImageProviderOptions{
		Model: "dall-e-3",
	})
	if err == nil {
		t.Error("Expected error with missing API key, got nil")
	}

	// Test with valid options
	provider, err := NewOpenAIImageProvider(&ImageProviderOptions{
		APIKey: "test-key",
		Model:  "dall-e-3",
	})
	if err != nil {
		t.Errorf("Unexpected error with valid options: %v", err)
	}
	if provider.apiKey != "test-key" {
		t.Errorf("Expected apiKey to be 'test-key', got '%s'", provider.apiKey)
	}
	if provider.modelID != "dall-e-3" {
		t.Errorf("Expected modelID to be 'dall-e-3', got '%s'", provider.modelID)
	}
	if provider.baseURL != defaultBaseURL {
		t.Errorf("Expected baseURL to be '%s', got '%s'", defaultBaseURL, provider.baseURL)
	}

	// Test with custom base URL
	provider, err = NewOpenAIImageProvider(&ImageProviderOptions{
		APIKey:  "test-key",
		Model:   "dall-e-3",
		BaseURL: "https://custom-api.openai.com/v1",
	})
	if err != nil {
		t.Errorf("Unexpected error with custom base URL: %v", err)
	}
	if provider.baseURL != "https://custom-api.openai.com/v1" {
		t.Errorf("Expected baseURL to be 'https://custom-api.openai.com/v1', got '%s'", provider.baseURL)
	}
}

func TestOpenAIImageProviderGenerateImage(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/images/generations" {
			t.Errorf("Expected path /images/generations, got %s", r.URL.Path)
		}

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-key" {
			t.Errorf("Expected Authorization header 'Bearer test-key', got '%s'", authHeader)
		}

		// Read and validate request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading request body: %v", err)
		}

		var requestData map[string]interface{}
		if err := json.Unmarshal(body, &requestData); err != nil {
			t.Errorf("Error unmarshaling request body: %v", err)
		}

		// Check required fields
		if requestData["prompt"] != "A test image" {
			t.Errorf("Expected prompt 'A test image', got '%v'", requestData["prompt"])
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
			"data": [
				{
					"url": "https://example.com/image.jpg",
					"revised_prompt": "A beautiful test image"
				}
			]
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create a provider that uses the mock server
	provider, err := NewOpenAIImageProvider(&ImageProviderOptions{
		APIKey:  "test-key",
		Model:   "dall-e-3",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Error creating provider: %v", err)
	}

	// Test with valid options
	options := &models.ImageGenerationOptions{
		Prompt: "A test image",
		Size:   models.ImageSize1024,
		N:      1,
	}
	response, err := provider.GenerateImage(context.Background(), options)
	if err != nil {
		t.Fatalf("Error generating image: %v", err)
	}

	// Check response fields
	if len(response.URLs) != 1 {
		t.Errorf("Expected 1 URL, got %d", len(response.URLs))
	}
	if response.URLs[0] != "https://example.com/image.jpg" {
		t.Errorf("Expected URL 'https://example.com/image.jpg', got '%s'", response.URLs[0])
	}
	if response.RevisedPrompt != "A beautiful test image" {
		t.Errorf("Expected revised prompt 'A beautiful test image', got '%s'", response.RevisedPrompt)
	}

	// Test with nil options
	_, err = provider.GenerateImage(context.Background(), nil)
	if err == nil {
		t.Error("Expected error with nil options, got nil")
	}

	// Test with empty prompt
	_, err = provider.GenerateImage(context.Background(), &models.ImageGenerationOptions{})
	if err == nil {
		t.Error("Expected error with empty prompt, got nil")
	}
}

func TestOpenAIImageProviderEditImage(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/images/edits" {
			t.Errorf("Expected path /images/edits, got %s", r.URL.Path)
		}

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-key" {
			t.Errorf("Expected Authorization header 'Bearer test-key', got '%s'", authHeader)
		}

		// Check content type header
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("Expected Content-Type to contain 'multipart/form-data', got '%s'", contentType)
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
			"data": [
				{
					"url": "https://example.com/edited-image.jpg"
				}
			]
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create a provider that uses the mock server
	provider, err := NewOpenAIImageProvider(&ImageProviderOptions{
		APIKey:  "test-key",
		Model:   "dall-e-3",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Error creating provider: %v", err)
	}

	// Test with valid options
	options := &models.ImageEditOptions{
		Image:  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", // 1x1 transparent PNG
		Prompt: "Add a red dot",
	}
	response, err := provider.EditImage(context.Background(), options)
	if err != nil {
		t.Fatalf("Error editing image: %v", err)
	}

	// Check response fields
	if len(response.URLs) != 1 {
		t.Errorf("Expected 1 URL, got %d", len(response.URLs))
	}
	if response.URLs[0] != "https://example.com/edited-image.jpg" {
		t.Errorf("Expected URL 'https://example.com/edited-image.jpg', got '%s'", response.URLs[0])
	}

	// Test with nil options
	_, err = provider.EditImage(context.Background(), nil)
	if err == nil {
		t.Error("Expected error with nil options, got nil")
	}

	// Test with empty image
	_, err = provider.EditImage(context.Background(), &models.ImageEditOptions{
		Prompt: "Add a red dot",
	})
	if err == nil {
		t.Error("Expected error with empty image, got nil")
	}

	// Test with empty prompt
	_, err = provider.EditImage(context.Background(), &models.ImageEditOptions{
		Image: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
	})
	if err == nil {
		t.Error("Expected error with empty prompt, got nil")
	}
}

func TestOpenAIImageProviderCreateImageVariation(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/images/variations" {
			t.Errorf("Expected path /images/variations, got %s", r.URL.Path)
		}

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-key" {
			t.Errorf("Expected Authorization header 'Bearer test-key', got '%s'", authHeader)
		}

		// Check content type header
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("Expected Content-Type to contain 'multipart/form-data', got '%s'", contentType)
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
			"data": [
				{
					"url": "https://example.com/variation-1.jpg"
				},
				{
					"url": "https://example.com/variation-2.jpg"
				}
			]
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create a provider that uses the mock server
	provider, err := NewOpenAIImageProvider(&ImageProviderOptions{
		APIKey:  "test-key",
		Model:   "dall-e-3",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Error creating provider: %v", err)
	}

	// Test with valid options
	options := &models.ImageVariationOptions{
		Image: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", // 1x1 transparent PNG
		N:     2,
	}
	response, err := provider.CreateImageVariation(context.Background(), options)
	if err != nil {
		t.Fatalf("Error creating image variation: %v", err)
	}

	// Check response fields
	if len(response.URLs) != 2 {
		t.Errorf("Expected 2 URLs, got %d", len(response.URLs))
	}
	if response.URLs[0] != "https://example.com/variation-1.jpg" {
		t.Errorf("Expected first URL 'https://example.com/variation-1.jpg', got '%s'", response.URLs[0])
	}
	if response.URLs[1] != "https://example.com/variation-2.jpg" {
		t.Errorf("Expected second URL 'https://example.com/variation-2.jpg', got '%s'", response.URLs[1])
	}

	// Test with nil options
	_, err = provider.CreateImageVariation(context.Background(), nil)
	if err == nil {
		t.Error("Expected error with nil options, got nil")
	}

	// Test with empty image
	_, err = provider.CreateImageVariation(context.Background(), &models.ImageVariationOptions{})
	if err == nil {
		t.Error("Expected error with empty image, got nil")
	}
}
