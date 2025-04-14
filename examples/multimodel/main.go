package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/louloulin/gostra/pkg"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/models/openai"
)

func main() {
	// 获取OpenAI API密钥
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("请设置 OPENAI_API_KEY 环境变量")
	}

	// 创建Gostra实例
	gostra := pkg.NewGostra(pkg.DefaultOptions())

	// 初始化文本模型
	textModel, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: apiKey,
		Model:  "gpt-3.5-turbo",
	})
	if err != nil {
		log.Fatalf("初始化文本模型失败: %v", err)
	}

	// 注册文本模型
	if err := gostra.RegisterTextModel("openai-text", textModel); err != nil {
		log.Fatalf("注册文本模型失败: %v", err)
	}

	// 初始化图像模型
	imageModel, err := openai.NewOpenAIImageProvider(&openai.ImageProviderOptions{
		APIKey: apiKey,
		Model:  "dall-e-3",
	})
	if err != nil {
		log.Fatalf("初始化图像模型失败: %v", err)
	}

	// 注册图像模型
	if err := gostra.RegisterImageModel("openai-image", imageModel); err != nil {
		log.Fatalf("注册图像模型失败: %v", err)
	}

	// 初始化语音模型
	voiceModel, err := openai.NewOpenAIVoiceProvider(&openai.VoiceProviderOptions{
		APIKey:   apiKey,
		TTSModel: "tts-1",
		STTModel: "whisper-1",
	})
	if err != nil {
		log.Fatalf("初始化语音模型失败: %v", err)
	}

	// 注册语音模型
	if err := gostra.RegisterVoiceModel("openai-voice", voiceModel); err != nil {
		log.Fatalf("注册语音模型失败: %v", err)
	}

	// 启动Gostra
	if err := gostra.Start(context.Background()); err != nil {
		log.Fatalf("启动Gostra失败: %v", err)
	}
	defer gostra.Stop()

	// 列出所有模型
	fmt.Println("已注册的文本模型:", gostra.ListTextModels())
	fmt.Println("已注册的图像模型:", gostra.ListImageModels())
	fmt.Println("已注册的语音模型:", gostra.ListVoiceModels())

	// 使用文本模型示例
	textModelExample(gostra)

	// 使用图像模型示例（注释掉以避免实际API调用）
	// imageModelExample(gostra)

	// 使用语音模型示例（注释掉以避免实际API调用）
	// voiceModelExample(gostra)
}

func textModelExample(gostra *pkg.Gostra) {
	fmt.Println("\n===== 文本模型示例 =====")

	// 获取文本模型
	textModel, err := gostra.GetTextModel("openai-text")
	if err != nil {
		log.Fatalf("获取文本模型失败: %v", err)
	}

	// 创建消息
	messages := []models.Message{
		{
			Role:    "system",
			Content: "你是一个有用的助手。",
		},
		{
			Role:    "user",
			Content: "Hello, world!",
		},
	}

	// 生成文本
	response, err := textModel.Generate(context.Background(), messages, models.DefaultGenerateOptions())
	if err != nil {
		log.Fatalf("生成文本失败: %v", err)
	}

	fmt.Printf("文本模型响应: %s\n", response)
}

func imageModelExample(gostra *pkg.Gostra) {
	fmt.Println("\n===== 图像模型示例 =====")

	// 获取图像模型
	imageModel, err := gostra.GetImageModel("openai-image")
	if err != nil {
		log.Fatalf("获取图像模型失败: %v", err)
	}

	// 设置生成选项
	options := &models.ImageGenerationOptions{
		Prompt:         "一只可爱的小猫咪，坐在窗台上看着窗外的雨",
		N:              1,
		Size:           models.ImageSize1024,
		ResponseFormat: models.ImageFormatURL,
	}

	// 生成图像
	response, err := imageModel.GenerateImage(context.Background(), options)
	if err != nil {
		log.Fatalf("生成图像失败: %v", err)
	}

	fmt.Printf("图像模型响应: %v\n", response.URLs)
}

func voiceModelExample(gostra *pkg.Gostra) {
	fmt.Println("\n===== 语音模型示例 =====")

	// 获取语音模型
	voiceModel, err := gostra.GetVoiceModel("openai-voice")
	if err != nil {
		log.Fatalf("获取语音模型失败: %v", err)
	}

	// 文本转语音
	ttsOptions := &models.TextToSpeechOptions{
		Text:  "你好，我是一个AI助手。很高兴为你服务！",
		Voice: "alloy",
		VoiceOptions: models.VoiceOptions{
			Format: models.VoiceFormatMP3,
		},
	}

	ttsResponse, err := voiceModel.TextToSpeech(context.Background(), ttsOptions)
	if err != nil {
		log.Fatalf("文本转语音失败: %v", err)
	}

	// 保存语音数据到文件
	if len(ttsResponse.AudioData) > 0 {
		if err := os.WriteFile("output.mp3", ttsResponse.AudioData, 0644); err != nil {
			log.Fatalf("保存语音文件失败: %v", err)
		}
		fmt.Println("语音文件已保存为 output.mp3")
	}
}
