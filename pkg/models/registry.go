package models

import (
	"errors"
	"fmt"
	"sync"
)

// ModelType 表示模型类型
type ModelType string

const (
	// ModelTypeText 表示文本模型
	ModelTypeText ModelType = "text"
	// ModelTypeImage 表示图像模型
	ModelTypeImage ModelType = "image"
	// ModelTypeVoice 表示语音模型
	ModelTypeVoice ModelType = "voice"
)

// ModelRegistry 是模型注册表，用于管理不同类型的模型提供者
type ModelRegistry struct {
	textModels  map[string]ModelProvider
	imageModels map[string]ImageProvider
	voiceModels map[string]VoiceProvider
	mutex       sync.RWMutex
}

// NewModelRegistry 创建一个新的模型注册表
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		textModels:  make(map[string]ModelProvider),
		imageModels: make(map[string]ImageProvider),
		voiceModels: make(map[string]VoiceProvider),
	}
}

// RegisterTextModel 注册一个文本模型提供者
func (r *ModelRegistry) RegisterTextModel(name string, provider ModelProvider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.textModels[name]; exists {
		return fmt.Errorf("text model already registered: %s", name)
	}

	r.textModels[name] = provider
	return nil
}

// RegisterImageModel 注册一个图像模型提供者
func (r *ModelRegistry) RegisterImageModel(name string, provider ImageProvider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.imageModels[name]; exists {
		return fmt.Errorf("image model already registered: %s", name)
	}

	r.imageModels[name] = provider
	return nil
}

// RegisterVoiceModel 注册一个语音模型提供者
func (r *ModelRegistry) RegisterVoiceModel(name string, provider VoiceProvider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.voiceModels[name]; exists {
		return fmt.Errorf("voice model already registered: %s", name)
	}

	r.voiceModels[name] = provider
	return nil
}

// GetTextModel 获取一个文本模型提供者
func (r *ModelRegistry) GetTextModel(name string) (ModelProvider, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if model, exists := r.textModels[name]; exists {
		return model, nil
	}
	return nil, fmt.Errorf("text model not found: %s", name)
}

// GetImageModel 获取一个图像模型提供者
func (r *ModelRegistry) GetImageModel(name string) (ImageProvider, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if model, exists := r.imageModels[name]; exists {
		return model, nil
	}
	return nil, fmt.Errorf("image model not found: %s", name)
}

// GetVoiceModel 获取一个语音模型提供者
func (r *ModelRegistry) GetVoiceModel(name string) (VoiceProvider, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if model, exists := r.voiceModels[name]; exists {
		return model, nil
	}
	return nil, fmt.Errorf("voice model not found: %s", name)
}

// ListTextModels 列出所有已注册的文本模型
func (r *ModelRegistry) ListTextModels() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	models := make([]string, 0, len(r.textModels))
	for name := range r.textModels {
		models = append(models, name)
	}
	return models
}

// ListImageModels 列出所有已注册的图像模型
func (r *ModelRegistry) ListImageModels() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	models := make([]string, 0, len(r.imageModels))
	for name := range r.imageModels {
		models = append(models, name)
	}
	return models
}

// ListVoiceModels 列出所有已注册的语音模型
func (r *ModelRegistry) ListVoiceModels() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	models := make([]string, 0, len(r.voiceModels))
	for name := range r.voiceModels {
		models = append(models, name)
	}
	return models
}
