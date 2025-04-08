package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/yourusername/gostra/pkg/models"
)

// OpenAIVoiceProvider implements the VoiceProvider interface for OpenAI's voice services
type OpenAIVoiceProvider struct {
	apiKey     string
	orgID      string
	baseURL    string
	ttsModel   string
	sttModel   string
	httpClient *http.Client
}

// VoiceProviderOptions defines the options for creating a new OpenAI voice provider
type VoiceProviderOptions struct {
	APIKey   string
	BaseURL  string
	OrgID    string
	TTSModel string // Default: "tts-1"
	STTModel string // Default: "whisper-1"
	Timeout  time.Duration
}

// NewOpenAIVoiceProvider creates a new OpenAI voice provider
func NewOpenAIVoiceProvider(opts *VoiceProviderOptions) (*OpenAIVoiceProvider, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	if opts.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	ttsModel := opts.TTSModel
	if ttsModel == "" {
		ttsModel = "tts-1"
	}

	sttModel := opts.STTModel
	if sttModel == "" {
		sttModel = "whisper-1"
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &OpenAIVoiceProvider{
		apiKey:     opts.APIKey,
		orgID:      opts.OrgID,
		baseURL:    baseURL,
		ttsModel:   ttsModel,
		sttModel:   sttModel,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// GetID returns the model ID (using TTS model as the primary identifier)
func (p *OpenAIVoiceProvider) GetID() string {
	return p.ttsModel
}

// GetProvider returns the provider name
func (p *OpenAIVoiceProvider) GetProvider() string {
	return "openai"
}

// TextToSpeech converts text to speech
func (p *OpenAIVoiceProvider) TextToSpeech(ctx context.Context, options *models.TextToSpeechOptions) (*models.TextToSpeechResponse, error) {
	if options == nil || options.Text == "" {
		return nil, errors.New("text is required for text-to-speech")
	}

	// Prepare the request body
	requestBody := map[string]interface{}{
		"model": p.ttsModel,
		"input": options.Text,
	}

	// Add voice if provided
	if options.Voice != "" {
		requestBody["voice"] = options.Voice
	} else {
		// Default voice
		requestBody["voice"] = "alloy" // Use a default voice
	}

	// Add speed if provided
	if options.Speed > 0 {
		requestBody["speed"] = options.Speed
	}

	// Add response format if provided
	responseFormat := "mp3"
	if options.Format != "" {
		responseFormat = string(options.Format)
	}
	requestBody["response_format"] = responseFormat

	// Marshal the request body
	requestData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/speech", bytes.NewReader(requestData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	p.setHeaders(req)

	// Send the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	return &models.TextToSpeechResponse{
		AudioData: audioData,
	}, nil
}

// TextToSpeechStream converts text to speech and returns a stream
func (p *OpenAIVoiceProvider) TextToSpeechStream(ctx context.Context, options *models.TextToSpeechOptions) (io.ReadCloser, error) {
	if options == nil || options.Text == "" {
		return nil, errors.New("text is required for text-to-speech")
	}

	// Prepare the request body
	requestBody := map[string]interface{}{
		"model": p.ttsModel,
		"input": options.Text,
	}

	// Add voice if provided
	if options.Voice != "" {
		requestBody["voice"] = options.Voice
	} else {
		// Default voice
		requestBody["voice"] = "alloy" // Use a default voice
	}

	// Add speed if provided
	if options.Speed > 0 {
		requestBody["speed"] = options.Speed
	}

	// Add response format if provided
	responseFormat := "mp3"
	if options.Format != "" {
		responseFormat = string(options.Format)
	}
	requestBody["response_format"] = responseFormat

	// Marshal the request body
	requestData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/speech", bytes.NewReader(requestData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	p.setHeaders(req)

	// Send the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API returned error status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Return the response body as a stream
	return resp.Body, nil
}

// SpeechToText converts speech to text
func (p *OpenAIVoiceProvider) SpeechToText(ctx context.Context, options *models.SpeechToTextOptions) (*models.SpeechToTextResponse, error) {
	if options == nil || options.Audio == "" {
		return nil, errors.New("audio is required for speech-to-text")
	}

	var audioData []byte
	var err error

	// Check if the audio is a base64-encoded string
	if strings.HasPrefix(options.Audio, "data:") || !strings.Contains(options.Audio, "://") {
		// Extract the base64-encoded part
		base64Data := options.Audio
		if strings.Contains(options.Audio, ",") {
			parts := strings.SplitN(options.Audio, ",", 2)
			if len(parts) == 2 {
				base64Data = parts[1]
			}
		}

		// Remove any prefixes like "data:audio/mp3;base64,"
		if strings.Contains(base64Data, ";base64,") {
			parts := strings.SplitN(base64Data, ";base64,", 2)
			if len(parts) == 2 {
				base64Data = parts[1]
			}
		}

		// Decode base64
		audioData, err = base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return nil, fmt.Errorf("error decoding base64 audio: %w", err)
		}
	} else {
		// URL-based audio is not directly supported by OpenAI
		return nil, errors.New("URL-based audio is not supported by OpenAI, please download and convert to base64")
	}

	// Create a buffer to write the multipart form data
	var requestBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBuffer)

	// Add the model field
	if err := multipartWriter.WriteField("model", p.sttModel); err != nil {
		return nil, fmt.Errorf("error writing model field: %w", err)
	}

	// Add optional fields
	if options.Language != "" {
		if err := multipartWriter.WriteField("language", options.Language); err != nil {
			return nil, fmt.Errorf("error writing language field: %w", err)
		}
	}

	if options.Prompt != "" {
		if err := multipartWriter.WriteField("prompt", options.Prompt); err != nil {
			return nil, fmt.Errorf("error writing prompt field: %w", err)
		}
	}

	if options.Temperature != 0 {
		if err := multipartWriter.WriteField("temperature", fmt.Sprintf("%f", options.Temperature)); err != nil {
			return nil, fmt.Errorf("error writing temperature field: %w", err)
		}
	}

	// Add response format - default to verbose_json to get segment information
	if err := multipartWriter.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("error writing response_format field: %w", err)
	}

	// Add the audio file
	audioField, err := multipartWriter.CreateFormFile("file", "audio.mp3")
	if err != nil {
		return nil, fmt.Errorf("error creating audio form field: %w", err)
	}
	if _, err := audioField.Write(audioData); err != nil {
		return nil, fmt.Errorf("error writing audio data: %w", err)
	}

	// Close the multipart writer to finalize the form
	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("error closing multipart writer: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/transcriptions", &requestBuffer)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	p.setHeaders(req)
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	// Send the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var openaiResp struct {
		Text     string `json:"text"`
		Language string `json:"language,omitempty"`
		Segments []struct {
			ID         int     `json:"id"`
			Text       string  `json:"text"`
			Start      float64 `json:"start"`
			End        float64 `json:"end"`
			Confidence float64 `json:"confidence"`
		} `json:"segments,omitempty"`
	}

	if err := json.Unmarshal(body, &openaiResp); err != nil {
		// Try parsing as simple text response
		var simpleResp struct {
			Text string `json:"text"`
		}
		if jsonErr := json.Unmarshal(body, &simpleResp); jsonErr != nil {
			return nil, fmt.Errorf("error unmarshaling response: %w (original error: %v)", jsonErr, err)
		}
		return &models.SpeechToTextResponse{
			Text: simpleResp.Text,
		}, nil
	}

	// Convert to our SpeechToTextResponse format
	result := &models.SpeechToTextResponse{
		Text:     openaiResp.Text,
		Language: openaiResp.Language,
		Segments: make([]models.SpeechSegment, 0, len(openaiResp.Segments)),
	}

	for _, segment := range openaiResp.Segments {
		result.Segments = append(result.Segments, models.SpeechSegment{
			ID:         segment.ID,
			Text:       segment.Text,
			Start:      segment.Start,
			End:        segment.End,
			Confidence: segment.Confidence,
		})
	}

	return result, nil
}

// setHeaders sets the required headers for OpenAI API requests
func (p *OpenAIVoiceProvider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	if p.orgID != "" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}
}
