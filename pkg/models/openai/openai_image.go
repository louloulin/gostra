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
	"time"

	"github.com/yourusername/gostra/pkg/models"
)

// OpenAIImageProvider implements the ImageProvider interface for OpenAI's DALL-E models
type OpenAIImageProvider struct {
	apiKey     string
	modelID    string
	orgID      string
	baseURL    string
	httpClient *http.Client
}

// ImageProviderOptions defines the options for creating a new OpenAI image provider
type ImageProviderOptions struct {
	APIKey  string
	BaseURL string
	OrgID   string
	Model   string // e.g., "dall-e-3" or "dall-e-2"
	Timeout time.Duration
}

// NewOpenAIImageProvider creates a new OpenAI image provider
func NewOpenAIImageProvider(opts *ImageProviderOptions) (*OpenAIImageProvider, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	if opts.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	if opts.Model == "" {
		opts.Model = "dall-e-3" // Default to DALL-E 3
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &OpenAIImageProvider{
		apiKey:     opts.APIKey,
		modelID:    opts.Model,
		orgID:      opts.OrgID,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// GetID returns the model ID
func (p *OpenAIImageProvider) GetID() string {
	return p.modelID
}

// GetProvider returns the provider name
func (p *OpenAIImageProvider) GetProvider() string {
	return "openai"
}

// GenerateImage generates images from a text prompt
func (p *OpenAIImageProvider) GenerateImage(ctx context.Context, options *models.ImageGenerationOptions) (*models.ImageResponse, error) {
	if options == nil || options.Prompt == "" {
		return nil, errors.New("prompt is required for image generation")
	}

	// Prepare the request body
	requestBody := map[string]interface{}{
		"prompt": options.Prompt,
		"model":  p.modelID,
	}

	// Add optional parameters if provided
	if options.N > 0 {
		requestBody["n"] = options.N
	}
	if options.Size != "" {
		requestBody["size"] = options.Size
	}
	if options.Quality != "" {
		requestBody["quality"] = options.Quality
	}
	if options.ResponseFormat != "" {
		requestBody["response_format"] = options.ResponseFormat
	}
	if options.User != "" {
		requestBody["user"] = options.User
	}

	// Marshal the request body
	requestData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/images/generations", bytes.NewReader(requestData))
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
		Data []struct {
			URL           string `json:"url,omitempty"`
			B64JSON       string `json:"b64_json,omitempty"`
			RevisedPrompt string `json:"revised_prompt,omitempty"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	// Convert to our ImageResponse format
	result := &models.ImageResponse{
		URLs:          make([]string, 0, len(openaiResp.Data)),
		B64Data:       make([]string, 0, len(openaiResp.Data)),
		RevisedPrompt: "",
	}

	for i, data := range openaiResp.Data {
		if data.URL != "" {
			result.URLs = append(result.URLs, data.URL)
		}
		if data.B64JSON != "" {
			result.B64Data = append(result.B64Data, data.B64JSON)
		}
		// Save the revised prompt from the first item if available
		if i == 0 && data.RevisedPrompt != "" {
			result.RevisedPrompt = data.RevisedPrompt
		}
	}

	return result, nil
}

// EditImage edits an image based on a prompt
func (p *OpenAIImageProvider) EditImage(ctx context.Context, options *models.ImageEditOptions) (*models.ImageResponse, error) {
	if options == nil || options.Image == "" || options.Prompt == "" {
		return nil, errors.New("image and prompt are required for image editing")
	}

	// Create a buffer to write the multipart form data
	var requestBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBuffer)

	// Add the prompt field
	if err := multipartWriter.WriteField("prompt", options.Prompt); err != nil {
		return nil, fmt.Errorf("error writing prompt field: %w", err)
	}

	// Add the model field if provided
	if p.modelID != "" {
		if err := multipartWriter.WriteField("model", p.modelID); err != nil {
			return nil, fmt.Errorf("error writing model field: %w", err)
		}
	}

	// Add optional fields
	if options.N > 0 {
		if err := multipartWriter.WriteField("n", fmt.Sprintf("%d", options.N)); err != nil {
			return nil, fmt.Errorf("error writing n field: %w", err)
		}
	}
	if options.Size != "" {
		if err := multipartWriter.WriteField("size", string(options.Size)); err != nil {
			return nil, fmt.Errorf("error writing size field: %w", err)
		}
	}
	if options.ResponseFormat != "" {
		if err := multipartWriter.WriteField("response_format", string(options.ResponseFormat)); err != nil {
			return nil, fmt.Errorf("error writing response_format field: %w", err)
		}
	}
	if options.User != "" {
		if err := multipartWriter.WriteField("user", options.User); err != nil {
			return nil, fmt.Errorf("error writing user field: %w", err)
		}
	}

	// Decode base64 image
	imageData, err := base64.StdEncoding.DecodeString(options.Image)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64 image: %w", err)
	}

	// Add the image file
	imageField, err := multipartWriter.CreateFormFile("image", "image.png")
	if err != nil {
		return nil, fmt.Errorf("error creating image form field: %w", err)
	}
	if _, err := imageField.Write(imageData); err != nil {
		return nil, fmt.Errorf("error writing image data: %w", err)
	}

	// Add the mask if provided
	if options.Mask != "" {
		maskData, err := base64.StdEncoding.DecodeString(options.Mask)
		if err != nil {
			return nil, fmt.Errorf("error decoding base64 mask: %w", err)
		}

		maskField, err := multipartWriter.CreateFormFile("mask", "mask.png")
		if err != nil {
			return nil, fmt.Errorf("error creating mask form field: %w", err)
		}
		if _, err := maskField.Write(maskData); err != nil {
			return nil, fmt.Errorf("error writing mask data: %w", err)
		}
	}

	// Close the multipart writer to finalize the form
	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("error closing multipart writer: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/images/edits", &requestBuffer)
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
		Data []struct {
			URL     string `json:"url,omitempty"`
			B64JSON string `json:"b64_json,omitempty"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	// Convert to our ImageResponse format
	result := &models.ImageResponse{
		URLs:    make([]string, 0, len(openaiResp.Data)),
		B64Data: make([]string, 0, len(openaiResp.Data)),
	}

	for _, data := range openaiResp.Data {
		if data.URL != "" {
			result.URLs = append(result.URLs, data.URL)
		}
		if data.B64JSON != "" {
			result.B64Data = append(result.B64Data, data.B64JSON)
		}
	}

	return result, nil
}

// CreateImageVariation creates variations of an image
func (p *OpenAIImageProvider) CreateImageVariation(ctx context.Context, options *models.ImageVariationOptions) (*models.ImageResponse, error) {
	if options == nil || options.Image == "" {
		return nil, errors.New("image is required for image variation")
	}

	// Create a buffer to write the multipart form data
	var requestBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBuffer)

	// Add the model field if provided
	if p.modelID != "" {
		if err := multipartWriter.WriteField("model", p.modelID); err != nil {
			return nil, fmt.Errorf("error writing model field: %w", err)
		}
	}

	// Add optional fields
	if options.N > 0 {
		if err := multipartWriter.WriteField("n", fmt.Sprintf("%d", options.N)); err != nil {
			return nil, fmt.Errorf("error writing n field: %w", err)
		}
	}
	if options.Size != "" {
		if err := multipartWriter.WriteField("size", string(options.Size)); err != nil {
			return nil, fmt.Errorf("error writing size field: %w", err)
		}
	}
	if options.ResponseFormat != "" {
		if err := multipartWriter.WriteField("response_format", string(options.ResponseFormat)); err != nil {
			return nil, fmt.Errorf("error writing response_format field: %w", err)
		}
	}
	if options.User != "" {
		if err := multipartWriter.WriteField("user", options.User); err != nil {
			return nil, fmt.Errorf("error writing user field: %w", err)
		}
	}

	// Decode base64 image
	imageData, err := base64.StdEncoding.DecodeString(options.Image)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64 image: %w", err)
	}

	// Add the image file
	imageField, err := multipartWriter.CreateFormFile("image", "image.png")
	if err != nil {
		return nil, fmt.Errorf("error creating image form field: %w", err)
	}
	if _, err := imageField.Write(imageData); err != nil {
		return nil, fmt.Errorf("error writing image data: %w", err)
	}

	// Close the multipart writer to finalize the form
	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("error closing multipart writer: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/images/variations", &requestBuffer)
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
		Data []struct {
			URL     string `json:"url,omitempty"`
			B64JSON string `json:"b64_json,omitempty"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	// Convert to our ImageResponse format
	result := &models.ImageResponse{
		URLs:    make([]string, 0, len(openaiResp.Data)),
		B64Data: make([]string, 0, len(openaiResp.Data)),
	}

	for _, data := range openaiResp.Data {
		if data.URL != "" {
			result.URLs = append(result.URLs, data.URL)
		}
		if data.B64JSON != "" {
			result.B64Data = append(result.B64Data, data.B64JSON)
		}
	}

	return result, nil
}

// setHeaders sets the required headers for OpenAI API requests
func (p *OpenAIImageProvider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	if p.orgID != "" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}
}
