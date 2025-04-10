package pkg

import (
	"context"
	"io"
	"testing"

	"github.com/louloulin/gostra/pkg/models"
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

func (m *MockTextModel) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	return "Generated text", nil
}

func (m *MockTextModel) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	return &models.ResponseWithFunctionCalls{Text: "Generated text with function calls"}, nil
}

func (m *MockTextModel) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "Streamed text"
	close(ch)
	return ch, nil
}

func (m *MockTextModel) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	ch := make(chan *models.ResponseChunk, 1)
	ch <- &models.ResponseChunk{Text: "Streamed text with function calls"}
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

func (m *MockImageModel) GenerateImage(ctx context.Context, options *models.ImageGenerationOptions) (*models.ImageResponse, error) {
	return &models.ImageResponse{URLs: []string{"https://example.com/image.jpg"}}, nil
}

func (m *MockImageModel) EditImage(ctx context.Context, options *models.ImageEditOptions) (*models.ImageResponse, error) {
	return &models.ImageResponse{URLs: []string{"https://example.com/edited-image.jpg"}}, nil
}

func (m *MockImageModel) CreateImageVariation(ctx context.Context, options *models.ImageVariationOptions) (*models.ImageResponse, error) {
	return &models.ImageResponse{URLs: []string{"https://example.com/variation.jpg"}}, nil
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

func (m *MockVoiceModel) TextToSpeech(ctx context.Context, options *models.TextToSpeechOptions) (*models.TextToSpeechResponse, error) {
	return &models.TextToSpeechResponse{AudioData: []byte{1, 2, 3, 4, 5}}, nil
}

func (m *MockVoiceModel) TextToSpeechStream(ctx context.Context, options *models.TextToSpeechOptions) (io.ReadCloser, error) {
	return io.NopCloser(io.LimitReader(io.NewSectionReader(nil, 0, 0), 0)), nil
}

func (m *MockVoiceModel) SpeechToText(ctx context.Context, options *models.SpeechToTextOptions) (*models.SpeechToTextResponse, error) {
	return &models.SpeechToTextResponse{Text: "Transcribed text"}, nil
}

func TestMultiModelSupport(t *testing.T) {
	g := New(nil)

	// 测试文本模型注册和获取
	textModel := &MockTextModel{
		id:       "text-model",
		provider: "mock",
	}
	if err := g.RegisterTextModel("text1", textModel); err != nil {
		t.Errorf("Failed to register text model: %v", err)
	}

	// 测试图像模型注册和获取
	imageModel := &MockImageModel{
		id:       "image-model",
		provider: "mock",
	}
	if err := g.RegisterImageModel("image1", imageModel); err != nil {
		t.Errorf("Failed to register image model: %v", err)
	}

	// 测试语音模型注册和获取
	voiceModel := &MockVoiceModel{
		id:       "voice-model",
		provider: "mock",
	}
	if err := g.RegisterVoiceModel("voice1", voiceModel); err != nil {
		t.Errorf("Failed to register voice model: %v", err)
	}

	// 测试获取文本模型
	retrievedTextModel, err := g.GetTextModel("text1")
	if err != nil {
		t.Errorf("Failed to get text model: %v", err)
	}
	if retrievedTextModel.GetID() != "text-model" {
		t.Errorf("Expected text model ID 'text-model', got '%s'", retrievedTextModel.GetID())
	}

	// 测试获取图像模型
	retrievedImageModel, err := g.GetImageModel("image1")
	if err != nil {
		t.Errorf("Failed to get image model: %v", err)
	}
	if retrievedImageModel.GetID() != "image-model" {
		t.Errorf("Expected image model ID 'image-model', got '%s'", retrievedImageModel.GetID())
	}

	// 测试获取语音模型
	retrievedVoiceModel, err := g.GetVoiceModel("voice1")
	if err != nil {
		t.Errorf("Failed to get voice model: %v", err)
	}
	if retrievedVoiceModel.GetID() != "voice-model" {
		t.Errorf("Expected voice model ID 'voice-model', got '%s'", retrievedVoiceModel.GetID())
	}

	// 测试兼容旧的API
	oldApiModel, err := g.GetModel("text1")
	if err != nil {
		t.Errorf("Failed to get model with old API: %v", err)
	}
	if oldApiModel.GetID() != "text-model" {
		t.Errorf("Expected model ID 'text-model' with old API, got '%s'", oldApiModel.GetID())
	}

	// 测试列出模型
	textModels := g.ListTextModels()
	if len(textModels) != 1 || textModels[0] != "text1" {
		t.Errorf("Expected text models ['text1'], got %v", textModels)
	}

	imageModels := g.ListImageModels()
	if len(imageModels) != 1 || imageModels[0] != "image1" {
		t.Errorf("Expected image models ['image1'], got %v", imageModels)
	}

	voiceModels := g.ListVoiceModels()
	if len(voiceModels) != 1 || voiceModels[0] != "voice1" {
		t.Errorf("Expected voice models ['voice1'], got %v", voiceModels)
	}
}

// 测试错误处理
func TestMultiModelErrorHandling(t *testing.T) {
	g := New(nil)

	// 测试注册nil模型
	if err := g.RegisterTextModel("text1", nil); err == nil {
		t.Errorf("Expected error when registering nil text model, got nil")
	}

	if err := g.RegisterImageModel("image1", nil); err == nil {
		t.Errorf("Expected error when registering nil image model, got nil")
	}

	if err := g.RegisterVoiceModel("voice1", nil); err == nil {
		t.Errorf("Expected error when registering nil voice model, got nil")
	}

	// 测试获取不存在的模型
	if _, err := g.GetTextModel("nonexistent"); err == nil {
		t.Errorf("Expected error when getting nonexistent text model, got nil")
	}

	if _, err := g.GetImageModel("nonexistent"); err == nil {
		t.Errorf("Expected error when getting nonexistent image model, got nil")
	}

	if _, err := g.GetVoiceModel("nonexistent"); err == nil {
		t.Errorf("Expected error when getting nonexistent voice model, got nil")
	}

	// 测试重复注册
	textModel := &MockTextModel{id: "text-model", provider: "mock"}
	imageModel := &MockImageModel{id: "image-model", provider: "mock"}
	voiceModel := &MockVoiceModel{id: "voice-model", provider: "mock"}

	// 先注册一次
	if err := g.RegisterTextModel("text1", textModel); err != nil {
		t.Errorf("Failed to register text model: %v", err)
	}
	if err := g.RegisterImageModel("image1", imageModel); err != nil {
		t.Errorf("Failed to register image model: %v", err)
	}
	if err := g.RegisterVoiceModel("voice1", voiceModel); err != nil {
		t.Errorf("Failed to register voice model: %v", err)
	}

	// 测试重复注册
	if err := g.RegisterTextModel("text1", textModel); err == nil {
		t.Errorf("Expected error when registering duplicate text model, got nil")
	}
	if err := g.RegisterImageModel("image1", imageModel); err == nil {
		t.Errorf("Expected error when registering duplicate image model, got nil")
	}
	if err := g.RegisterVoiceModel("voice1", voiceModel); err == nil {
		t.Errorf("Expected error when registering duplicate voice model, got nil")
	}
}
