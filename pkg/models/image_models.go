package models

import (
	"context"
)

// ImageFormat represents supported image formats
type ImageFormat string

const (
	// ImageFormatURL for URL responses
	ImageFormatURL ImageFormat = "url"
	// ImageFormatB64JSON for base64-encoded JSON responses
	ImageFormatB64JSON ImageFormat = "b64_json"
)

// ImageSize represents supported image sizes
type ImageSize string

const (
	// ImageSize256 for 256x256 images
	ImageSize256 ImageSize = "256x256"
	// ImageSize512 for 512x512 images
	ImageSize512 ImageSize = "512x512"
	// ImageSize1024 for 1024x1024 images
	ImageSize1024 ImageSize = "1024x1024"
	// ImageSize1792 for 1792x1024 images
	ImageSize1792 ImageSize = "1792x1024"
	// ImageSize1024x1792 for 1024x1792 images
	ImageSize1024x1792 ImageSize = "1024x1792"
)

// ImageQuality represents the quality of the generated images
type ImageQuality string

const (
	// ImageQualityStandard for standard quality
	ImageQualityStandard ImageQuality = "standard"
	// ImageQualityHD for high quality
	ImageQualityHD ImageQuality = "hd"
)

// ImageGenerationOptions contains options for image generation
type ImageGenerationOptions struct {
	// Prompt is the text prompt for image generation
	Prompt string `json:"prompt"`
	// Model specifies the model to use (optional, provider-specific)
	Model string `json:"model,omitempty"`
	// N is the number of images to generate
	N int `json:"n,omitempty"`
	// Size is the size of the generated images
	Size ImageSize `json:"size,omitempty"`
	// Quality is the quality of the generated images
	Quality ImageQuality `json:"quality,omitempty"`
	// ResponseFormat is the format of the response
	ResponseFormat ImageFormat `json:"response_format,omitempty"`
	// User is a unique identifier representing your end-user
	User string `json:"user,omitempty"`
}

// ImageEditOptions contains options for image editing
type ImageEditOptions struct {
	// Image is the base64-encoded image to edit (required)
	Image string `json:"image"`
	// Prompt is the text prompt for image editing
	Prompt string `json:"prompt"`
	// Mask is the base64-encoded mask for image editing (optional)
	Mask string `json:"mask,omitempty"`
	// Model specifies the model to use (optional, provider-specific)
	Model string `json:"model,omitempty"`
	// N is the number of images to generate
	N int `json:"n,omitempty"`
	// Size is the size of the generated images
	Size ImageSize `json:"size,omitempty"`
	// ResponseFormat is the format of the response
	ResponseFormat ImageFormat `json:"response_format,omitempty"`
	// User is a unique identifier representing your end-user
	User string `json:"user,omitempty"`
}

// ImageVariationOptions contains options for creating image variations
type ImageVariationOptions struct {
	// Image is the base64-encoded image to create variations of (required)
	Image string `json:"image"`
	// Model specifies the model to use (optional, provider-specific)
	Model string `json:"model,omitempty"`
	// N is the number of images to generate
	N int `json:"n,omitempty"`
	// Size is the size of the generated images
	Size ImageSize `json:"size,omitempty"`
	// ResponseFormat is the format of the response
	ResponseFormat ImageFormat `json:"response_format,omitempty"`
	// User is a unique identifier representing your end-user
	User string `json:"user,omitempty"`
}

// ImageResponse represents the response from image generation, editing, or variation
type ImageResponse struct {
	// URLs are the URLs of the generated images
	URLs []string `json:"urls,omitempty"`
	// B64Data contains the base64-encoded image data if ResponseFormat is b64_json
	B64Data []string `json:"b64_data,omitempty"`
	// RevisedPrompt contains the revised prompt if the original prompt was modified
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageProvider defines the interface for image generation services
type ImageProvider interface {
	// GetID returns the provider's ID
	GetID() string

	// GetProvider returns the provider name (e.g., "openai", "stability", etc.)
	GetProvider() string

	// GenerateImage generates images from a text prompt
	GenerateImage(ctx context.Context, options *ImageGenerationOptions) (*ImageResponse, error)

	// EditImage edits an image based on a prompt
	EditImage(ctx context.Context, options *ImageEditOptions) (*ImageResponse, error)

	// CreateImageVariation creates variations of an image
	CreateImageVariation(ctx context.Context, options *ImageVariationOptions) (*ImageResponse, error)
}

// DefaultImageGenerationOptions returns default options for image generation
func DefaultImageGenerationOptions() *ImageGenerationOptions {
	return &ImageGenerationOptions{
		N:              1,
		Size:           ImageSize1024,
		Quality:        ImageQualityStandard,
		ResponseFormat: ImageFormatURL,
	}
}

// DefaultImageEditOptions returns default options for image editing
func DefaultImageEditOptions() *ImageEditOptions {
	return &ImageEditOptions{
		N:              1,
		Size:           ImageSize1024,
		ResponseFormat: ImageFormatURL,
	}
}

// DefaultImageVariationOptions returns default options for image variation
func DefaultImageVariationOptions() *ImageVariationOptions {
	return &ImageVariationOptions{
		N:              1,
		Size:           ImageSize1024,
		ResponseFormat: ImageFormatURL,
	}
}
