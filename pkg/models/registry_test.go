package models

import (
	"context"
	"io"
	"testing"
)

// MockTextModel 模拟文本模型提供者
type MockTextModel struct {
	id       string
	provider string
}

func (m *MockTextModel) GetID() string {
	return m.id
}

func (m *MockTextModel) GetProvider() string {
	return m.provider
}

func (m *MockTextModel) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (string, error) {
	return "Generated text", nil
}

func (m *MockTextModel) GenerateWithFunctionCalls(ctx context.Context, messages []Message, options *GenerateOptions) (*ResponseWithFunctionCalls, error) {
	return &ResponseWithFunctionCalls{Text: "Generated text with function calls"}, nil
}

func (m *MockTextModel) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "Streamed text"
	close(ch)
	return ch, nil
}

func (m *MockTextModel) StreamWithFunctionCalls(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan *ResponseChunk, error) {
	ch := make(chan *ResponseChunk, 1)
	ch <- &ResponseChunk{Text: "Streamed text with function calls"}
	close(ch)
	return ch, nil
}

// MockImageModel 模拟图像模型提供者
type MockImageModel struct {
	id       string
	provider string
}

func (m *MockImageModel) GetID() string {
	return m.id
}

func (m *MockImageModel) GetProvider() string {
	return m.provider
}

func (m *MockImageModel) GenerateImage(ctx context.Context, options *ImageGenerationOptions) (*ImageResponse, error) {
	return &ImageResponse{URLs: []string{"https://example.com/image.jpg"}}, nil
}

func (m *MockImageModel) EditImage(ctx context.Context, options *ImageEditOptions) (*ImageResponse, error) {
	return &ImageResponse{URLs: []string{"https://example.com/edited-image.jpg"}}, nil
}

func (m *MockImageModel) CreateImageVariation(ctx context.Context, options *ImageVariationOptions) (*ImageResponse, error) {
	return &ImageResponse{URLs: []string{"https://example.com/variation.jpg"}}, nil
}

// MockVoiceModel 模拟语音模型提供者
type MockVoiceModel struct {
	id       string
	provider string
}

func (m *MockVoiceModel) GetID() string {
	return m.id
}

func (m *MockVoiceModel) GetProvider() string {
	return m.provider
}

func (m *MockVoiceModel) TextToSpeech(ctx context.Context, options *TextToSpeechOptions) (*TextToSpeechResponse, error) {
	return &TextToSpeechResponse{AudioData: []byte{1, 2, 3, 4, 5}}, nil
}

func (m *MockVoiceModel) TextToSpeechStream(ctx context.Context, options *TextToSpeechOptions) (io.ReadCloser, error) {
	return io.NopCloser(io.LimitReader(io.NewSectionReader(nil, 0, 0), 0)), nil
}

func (m *MockVoiceModel) SpeechToText(ctx context.Context, options *SpeechToTextOptions) (*SpeechToTextResponse, error) {
	return &SpeechToTextResponse{Text: "Transcribed text"}, nil
}

func TestModelRegistry(t *testing.T) {
	registry := NewModelRegistry()

	// 测试注册模型
	textModel := &MockTextModel{id: "text-model", provider: "mock"}
	imageModel := &MockImageModel{id: "image-model", provider: "mock"}
	voiceModel := &MockVoiceModel{id: "voice-model", provider: "mock"}

	if err := registry.RegisterTextModel("text1", textModel); err != nil {
		t.Errorf("Failed to register text model: %v", err)
	}

	if err := registry.RegisterImageModel("image1", imageModel); err != nil {
		t.Errorf("Failed to register image model: %v", err)
	}

	if err := registry.RegisterVoiceModel("voice1", voiceModel); err != nil {
		t.Errorf("Failed to register voice model: %v", err)
	}

	// 测试重复注册
	if err := registry.RegisterTextModel("text1", textModel); err == nil {
		t.Errorf("Expected error when registering duplicate text model, got nil")
	}

	if err := registry.RegisterImageModel("image1", imageModel); err == nil {
		t.Errorf("Expected error when registering duplicate image model, got nil")
	}

	if err := registry.RegisterVoiceModel("voice1", voiceModel); err == nil {
		t.Errorf("Expected error when registering duplicate voice model, got nil")
	}

	// 测试获取模型
	if model, err := registry.GetTextModel("text1"); err != nil {
		t.Errorf("Failed to get text model: %v", err)
	} else if model.GetID() != "text-model" {
		t.Errorf("Expected text model ID 'text-model', got '%s'", model.GetID())
	}

	if model, err := registry.GetImageModel("image1"); err != nil {
		t.Errorf("Failed to get image model: %v", err)
	} else if model.GetID() != "image-model" {
		t.Errorf("Expected image model ID 'image-model', got '%s'", model.GetID())
	}

	if model, err := registry.GetVoiceModel("voice1"); err != nil {
		t.Errorf("Failed to get voice model: %v", err)
	} else if model.GetID() != "voice-model" {
		t.Errorf("Expected voice model ID 'voice-model', got '%s'", model.GetID())
	}

	// 测试获取不存在的模型
	if _, err := registry.GetTextModel("nonexistent"); err == nil {
		t.Errorf("Expected error when getting nonexistent text model, got nil")
	}

	if _, err := registry.GetImageModel("nonexistent"); err == nil {
		t.Errorf("Expected error when getting nonexistent image model, got nil")
	}

	if _, err := registry.GetVoiceModel("nonexistent"); err == nil {
		t.Errorf("Expected error when getting nonexistent voice model, got nil")
	}

	// 测试列出模型
	textModels := registry.ListTextModels()
	if len(textModels) != 1 || textModels[0] != "text1" {
		t.Errorf("Expected text models ['text1'], got %v", textModels)
	}

	imageModels := registry.ListImageModels()
	if len(imageModels) != 1 || imageModels[0] != "image1" {
		t.Errorf("Expected image models ['image1'], got %v", imageModels)
	}

	voiceModels := registry.ListVoiceModels()
	if len(voiceModels) != 1 || voiceModels[0] != "voice1" {
		t.Errorf("Expected voice models ['voice1'], got %v", voiceModels)
	}
}
