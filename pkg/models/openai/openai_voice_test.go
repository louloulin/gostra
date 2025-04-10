package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/louloulin/gostra/pkg/models"
)

func TestNewOpenAIVoiceProvider(t *testing.T) {
	// Test with nil options
	_, err := NewOpenAIVoiceProvider(nil)
	if err == nil {
		t.Error("Expected error with nil options, got nil")
	}

	// Test with missing API key
	_, err = NewOpenAIVoiceProvider(&VoiceProviderOptions{
		TTSModel: "tts-1",
		STTModel: "whisper-1",
	})
	if err == nil {
		t.Error("Expected error with missing API key, got nil")
	}

	// Test with valid options
	provider, err := NewOpenAIVoiceProvider(&VoiceProviderOptions{
		APIKey:   "test-key",
		TTSModel: "tts-1",
		STTModel: "whisper-1",
	})
	if err != nil {
		t.Errorf("Unexpected error with valid options: %v", err)
	}
	if provider.apiKey != "test-key" {
		t.Errorf("Expected apiKey to be 'test-key', got '%s'", provider.apiKey)
	}
	if provider.ttsModel != "tts-1" {
		t.Errorf("Expected ttsModel to be 'tts-1', got '%s'", provider.ttsModel)
	}
	if provider.sttModel != "whisper-1" {
		t.Errorf("Expected sttModel to be 'whisper-1', got '%s'", provider.sttModel)
	}
	if provider.baseURL != defaultBaseURL {
		t.Errorf("Expected baseURL to be '%s', got '%s'", defaultBaseURL, provider.baseURL)
	}

	// Test with custom base URL
	provider, err = NewOpenAIVoiceProvider(&VoiceProviderOptions{
		APIKey:   "test-key",
		TTSModel: "tts-1",
		STTModel: "whisper-1",
		BaseURL:  "https://custom-api.openai.com/v1",
	})
	if err != nil {
		t.Errorf("Unexpected error with custom base URL: %v", err)
	}
	if provider.baseURL != "https://custom-api.openai.com/v1" {
		t.Errorf("Expected baseURL to be 'https://custom-api.openai.com/v1', got '%s'", provider.baseURL)
	}
}

func TestOpenAIVoiceProviderTextToSpeech(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/audio/speech" {
			t.Errorf("Expected path /audio/speech, got %s", r.URL.Path)
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
		if requestData["input"] != "Hello, world!" {
			t.Errorf("Expected input 'Hello, world!', got '%v'", requestData["input"])
		}
		if requestData["model"] != "tts-1" {
			t.Errorf("Expected model 'tts-1', got '%v'", requestData["model"])
		}
		if requestData["voice"] != "alloy" {
			t.Errorf("Expected voice 'alloy', got '%v'", requestData["voice"])
		}

		// Return a mock response
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		// Minimal MP3 header (not a real MP3, just for testing)
		w.Write([]byte{0xFF, 0xFB, 0x90, 0x44, 0x00})
	}))
	defer server.Close()

	// Create a provider that uses the mock server
	provider, err := NewOpenAIVoiceProvider(&VoiceProviderOptions{
		APIKey:   "test-key",
		TTSModel: "tts-1",
		STTModel: "whisper-1",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Error creating provider: %v", err)
	}

	// Test with valid options
	options := &models.TextToSpeechOptions{
		Text:  "Hello, world!",
		Voice: "alloy",
		VoiceOptions: models.VoiceOptions{
			Model:  "tts-1",
			Format: models.VoiceFormatMP3,
		},
		Speed: 1.0,
	}
	response, err := provider.TextToSpeech(context.Background(), options)
	if err != nil {
		t.Fatalf("Error converting text to speech: %v", err)
	}

	// Check response fields
	if len(response.AudioData) != 5 {
		t.Errorf("Expected 5 bytes of audio data, got %d", len(response.AudioData))
	}

	// Test with nil options
	_, err = provider.TextToSpeech(context.Background(), nil)
	if err == nil {
		t.Error("Expected error with nil options, got nil")
	}

	// Test with empty text
	_, err = provider.TextToSpeech(context.Background(), &models.TextToSpeechOptions{
		Voice: "alloy",
		VoiceOptions: models.VoiceOptions{
			Format: models.VoiceFormatMP3,
		},
	})
	if err == nil {
		t.Error("Expected error with empty text, got nil")
	}
}

func TestOpenAIVoiceProviderSpeechToText(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("Expected path /audio/transcriptions, got %s", r.URL.Path)
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
			"text": "Hello, world!",
			"language": "en",
			"segments": [
				{
					"id": 0,
					"text": "Hello, world!",
					"start": 0.0,
					"end": 1.5,
					"confidence": 0.95
				}
			]
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create a provider that uses the mock server
	provider, err := NewOpenAIVoiceProvider(&VoiceProviderOptions{
		APIKey:   "test-key",
		TTSModel: "tts-1",
		STTModel: "whisper-1",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Error creating provider: %v", err)
	}

	// Test with valid options
	options := &models.SpeechToTextOptions{
		Audio:       "Zm9vYmFy", // base64-encoded "foobar"
		Language:    "en",
		Prompt:      "Transcribe the following audio",
		Temperature: 0.0,
		VoiceOptions: models.VoiceOptions{
			Model: "whisper-1",
		},
	}
	response, err := provider.SpeechToText(context.Background(), options)
	if err != nil {
		t.Fatalf("Error converting speech to text: %v", err)
	}

	// Check response fields
	if response.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got '%s'", response.Text)
	}
	if response.Language != "en" {
		t.Errorf("Expected language 'en', got '%s'", response.Language)
	}
	if len(response.Segments) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(response.Segments))
	}
	if response.Segments[0].Text != "Hello, world!" {
		t.Errorf("Expected segment text 'Hello, world!', got '%s'", response.Segments[0].Text)
	}
	if response.Segments[0].Start != 0.0 {
		t.Errorf("Expected segment start time 0.0, got %f", response.Segments[0].Start)
	}
	if response.Segments[0].End != 1.5 {
		t.Errorf("Expected segment end time 1.5, got %f", response.Segments[0].End)
	}
	if response.Segments[0].Confidence != 0.95 {
		t.Errorf("Expected segment confidence 0.95, got %f", response.Segments[0].Confidence)
	}

	// Test with nil options
	_, err = provider.SpeechToText(context.Background(), nil)
	if err == nil {
		t.Error("Expected error with nil options, got nil")
	}

	// Test with empty audio
	_, err = provider.SpeechToText(context.Background(), &models.SpeechToTextOptions{
		Language: "en",
	})
	if err == nil {
		t.Error("Expected error with empty audio, got nil")
	}
}
