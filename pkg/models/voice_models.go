package models

import (
	"context"
	"io"
)

// VoiceFormat represents the audio format for speech operations
type VoiceFormat string

const (
	// VoiceFormatMP3 for MP3 format
	VoiceFormatMP3 VoiceFormat = "mp3"
	// VoiceFormatWAV for WAV format
	VoiceFormatWAV VoiceFormat = "wav"
	// VoiceFormatOGG for OGG format
	VoiceFormatOGG VoiceFormat = "ogg"
	// VoiceFormatFLAC for FLAC format
	VoiceFormatFLAC VoiceFormat = "flac"
)

// VoiceOptions contains common options for voice operations
type VoiceOptions struct {
	// Model specifies the model to use (provider-specific)
	Model string `json:"model,omitempty"`
	// Format specifies the audio format to use
	Format VoiceFormat `json:"format,omitempty"`
	// User is a unique identifier representing the end-user
	User string `json:"user,omitempty"`
}

// TextToSpeechOptions contains options for text-to-speech operations
type TextToSpeechOptions struct {
	// Text is the text to convert to speech
	Text string `json:"text"`
	// Voice is the voice ID to use (provider-specific)
	Voice string `json:"voice,omitempty"`
	// Speed is the speaking rate/speed (0.25 to 4.0, default 1.0)
	Speed float64 `json:"speed,omitempty"`
	// Common options
	VoiceOptions
}

// SpeechToTextOptions contains options for speech-to-text operations
type SpeechToTextOptions struct {
	// Audio is the base64-encoded audio or an URL to audio file
	Audio string `json:"audio"`
	// Language specifies the language of the audio (e.g., "en", "zh", etc.)
	Language string `json:"language,omitempty"`
	// Temperature controls randomness in results
	Temperature float64 `json:"temperature,omitempty"`
	// Prompt provides additional context to guide transcription
	Prompt string `json:"prompt,omitempty"`
	// Common options
	VoiceOptions
}

// TextToSpeechResponse contains the response from text-to-speech operations
type TextToSpeechResponse struct {
	// AudioData contains the audio data if requested directly
	AudioData []byte `json:"-"`
	// AudioURL contains a URL to the generated audio if available
	AudioURL string `json:"audio_url,omitempty"`
}

// SpeechToTextResponse contains the response from speech-to-text operations
type SpeechToTextResponse struct {
	// Text contains the transcribed text
	Text string `json:"text"`
	// Segments contains detailed information about each spoken segment
	Segments []SpeechSegment `json:"segments,omitempty"`
	// Language contains the detected language
	Language string `json:"language,omitempty"`
}

// SpeechSegment represents a segment of transcribed speech
type SpeechSegment struct {
	// ID is the unique identifier for the segment
	ID int `json:"id"`
	// Text contains the transcribed text for this segment
	Text string `json:"text"`
	// Start is the start time of the segment in seconds
	Start float64 `json:"start"`
	// End is the end time of the segment in seconds
	End float64 `json:"end"`
	// Confidence indicates the confidence level (0-1) of the transcription
	Confidence float64 `json:"confidence"`
	// Speaker is the identified speaker (if applicable)
	Speaker string `json:"speaker,omitempty"`
}

// VoiceProvider defines the interface for voice service providers
type VoiceProvider interface {
	// GetID returns the provider's ID
	GetID() string

	// GetProvider returns the provider name (e.g., "openai", "google", etc.)
	GetProvider() string

	// TextToSpeech converts text to speech
	TextToSpeech(ctx context.Context, options *TextToSpeechOptions) (*TextToSpeechResponse, error)

	// TextToSpeechStream converts text to speech and returns a stream
	TextToSpeechStream(ctx context.Context, options *TextToSpeechOptions) (io.ReadCloser, error)

	// SpeechToText converts speech to text
	SpeechToText(ctx context.Context, options *SpeechToTextOptions) (*SpeechToTextResponse, error)
}

// DefaultTextToSpeechOptions returns default options for text-to-speech operations
func DefaultTextToSpeechOptions() *TextToSpeechOptions {
	return &TextToSpeechOptions{
		Speed: 1.0,
		VoiceOptions: VoiceOptions{
			Format: VoiceFormatMP3,
		},
	}
}

// DefaultSpeechToTextOptions returns default options for speech-to-text operations
func DefaultSpeechToTextOptions() *SpeechToTextOptions {
	return &SpeechToTextOptions{
		Temperature:  0.0,
		VoiceOptions: VoiceOptions{},
	}
}
