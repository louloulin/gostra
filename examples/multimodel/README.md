# Multi-Model Support Example

This example demonstrates how to use Gostra's multi-model support to register and use different types of AI models:

1. Text models for generating text responses
2. Image models for generating and manipulating images
3. Voice models for text-to-speech and speech-to-text operations

## Overview

Gostra now supports multiple model types through a unified registry system. Each model type implements its own interface:

- `models.ModelProvider` for text models
- `models.ImageProvider` for image models
- `models.VoiceProvider` for voice models

The main `Gostra` class provides methods for registering and retrieving different model types:

- `RegisterTextModel` / `GetTextModel` for text models
- `RegisterImageModel` / `GetImageModel` for image models
- `RegisterVoiceModel` / `GetVoiceModel` for voice models

Each model type supports its own set of operations:

- Text models: Generate text, stream text, function calling, etc.
- Image models: Generate images, edit images, create image variations
- Voice models: Convert text to speech, convert speech to text

## Running the Example

To run this example, you need an OpenAI API key:

```bash
export OPENAI_API_KEY=your_api_key_here
go run main.go
```

## How It Works

1. The example initializes three different OpenAI model providers:
   - A text model using GPT-3.5 Turbo
   - An image model using DALL-E 3
   - A voice model using TTS-1 (for text-to-speech) and Whisper-1 (for speech-to-text)

2. It registers these models with the Gostra system using their respective registration methods.

3. It then demonstrates how to retrieve and use the text model to generate responses.

4. The example also includes commented-out code for using the image and voice models, which can be uncommented if you want to test those capabilities.

## Multi-Model Architecture

The multi-model support is built on a registry system that:

1. Provides type safety for different model types
2. Ensures thread safety with mutex locks
3. Supports listing all registered models of each type
4. Maintains backward compatibility with the original API

This architecture allows agents to use different model types for different tasks, similar to how humans use different senses and abilities to interact with the world. 