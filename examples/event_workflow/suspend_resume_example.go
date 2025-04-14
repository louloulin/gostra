package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/louloulin/gostra/pkg/workflow"
)

// runImplementation 运行带有数据恢复的事件驱动工作流示例
func runImplementation() {
	// 创建临时目录存储工作流状态
	tempDir, err := os.MkdirTemp("", "workflow-resume-data")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建文件状态存储
	stateStore := workflow.NewInMemoryStateStore()

	// 创建内容生成工作流
	contentWorkflow, err := workflow.NewEventWorkflow(workflow.EventWorkflowOptions{
		ID:          "content-generation-workflow",
		Name:        "内容生成工作流",
		Description: "展示暂停-恢复数据传递的内容生成工作流",
		StartStepID: "get-user-input",
		StateStore:  stateStore,
	})

	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}

	// 步骤1: 获取用户输入
	getUserInputStep := contentWorkflow.CreateStep(
		"get-user-input",
		"获取用户输入",
		"获取初始用户输入",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			inputData, ok := data.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid input data format")
			}

			topic, _ := inputData["topic"].(string)
			fmt.Printf("获取用户输入: 话题 '%s'\n", topic)

			return map[string]interface{}{
				"userInput": topic,
			}, nil
		},
		map[string]string{
			"": "generate-content", // 默认下一步
		},
	)

	// 步骤2: 生成内容 (可能暂停等待指导)
	generateContentStep := contentWorkflow.CreateStep(
		"generate-content",
		"生成内容",
		"生成初始内容草稿",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			userInput := stepData["userInput"].(string)
			fmt.Printf("基于用户输入 '%s' 生成内容\n", userInput)

			// 模拟AI生成内容
			initialDraft := generateInitialDraft(userInput)
			confidenceScore := calculateConfidenceScore(initialDraft)

			fmt.Printf("生成的内容：%s\n", initialDraft)
			fmt.Printf("置信度评分：%.2f\n", confidenceScore)

			// 如果置信度高，直接返回内容
			if confidenceScore > 0.7 {
				fmt.Println("置信度高，继续执行下一步")
				return map[string]interface{}{
					"content":         initialDraft,
					"confidenceScore": confidenceScore,
				}, nil
			}

			fmt.Println("置信度低，暂停等待人工指导")

			// 暂停工作流，等待人工指导数据
			// 保存暂停时的数据，供恢复时使用
			return map[string]interface{}{
				"content":         initialDraft,
				"confidenceScore": confidenceScore,
				"needsGuidance":   true,
				"suspendReason":   "低置信度，需要人工指导",
				"resumeData": map[string]interface{}{
					"draftContent":    initialDraft,
					"confidenceScore": confidenceScore,
				},
			}, nil
		},
		map[string]string{
			"human-guidance": "improve-content", // 人工指导事件后转到改进步骤
		},
	)

	// 步骤3: 根据人工指导改进内容
	improveContentStep := contentWorkflow.CreateStep(
		"improve-content",
		"改进内容",
		"根据人工指导改进内容",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			// 获取原始草稿和人工指导
			resumeData, exists := stepData["resumeData"].(map[string]interface{})
			if !exists {
				fmt.Println("警告: 没有找到恢复数据")
				resumeData = make(map[string]interface{})
			}

			// 获取事件数据 (人工指导)
			eventData, ok := data.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid event data format")
			}

			event, ok := eventData["event"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid event format")
			}

			guidanceData, ok := event["data"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid guidance data format")
			}

			guidance, _ := guidanceData["guidance"].(string)
			draftContent, _ := resumeData["draftContent"].(string)

			fmt.Printf("原始草稿: %s\n", draftContent)
			fmt.Printf("收到人工指导: %s\n", guidance)

			// 根据指导改进内容
			improvedContent := improveWithGuidance(draftContent, guidance)
			fmt.Printf("改进后的内容: %s\n", improvedContent)

			return map[string]interface{}{
				"improvedContent": improvedContent,
				"guidance":        guidance,
				"originalDraft":   draftContent,
			}, nil
		},
		map[string]string{
			"": "evaluate-content", // 默认下一步
		},
	)

	// 步骤4: 评估内容质量
	evaluateContentStep := contentWorkflow.CreateStep(
		"evaluate-content",
		"评估内容",
		"评估内容质量",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			content := stepData["improvedContent"].(string)
			fmt.Printf("评估改进后的内容质量: %s\n", content)

			// 模拟评估
			toneScore := calculateToneScore(content)
			completenessScore := calculateCompletenessScore(content)

			fmt.Printf("语调评分: %.2f\n", toneScore)
			fmt.Printf("完整性评分: %.2f\n", completenessScore)

			// 评估结果
			return map[string]interface{}{
				"toneScore":         toneScore,
				"completenessScore": completenessScore,
				"overallScore":      (toneScore + completenessScore) / 2,
			}, nil
		},
		map[string]string{
			"": "finalize-content", // 默认下一步
		},
	)

	// 步骤5: 最终确认内容
	finalizeContentStep := contentWorkflow.CreateStep(
		"finalize-content",
		"最终确认",
		"最终确认并提交内容",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			content := stepData["improvedContent"].(string)
			overallScore := stepData["overallScore"].(float64)

			fmt.Printf("最终确认内容: %s\n", content)
			fmt.Printf("总体评分: %.2f\n", overallScore)

			// 确认成功
			return map[string]interface{}{
				"finalContent":     content,
				"finalScore":       overallScore,
				"completionStatus": "success",
				"completedAt":      time.Now(),
			}, nil
		},
		nil, // 工作流结束
	)

	// 添加步骤到工作流
	contentWorkflow.AddStep(getUserInputStep)
	contentWorkflow.AddStep(generateContentStep)
	contentWorkflow.AddStep(improveContentStep)
	contentWorkflow.AddStep(evaluateContentStep)
	contentWorkflow.AddStep(finalizeContentStep)

	// 启动工作流
	instanceID, err := contentWorkflow.StartWorkflow(context.Background(), map[string]interface{}{
		"topic": "人工智能在教育中的应用",
	})

	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("工作流实例已创建: %s\n", instanceID)

	// 等待工作流暂停
	time.Sleep(500 * time.Millisecond)

	// 检查工作流状态
	state, err := contentWorkflow.GetState(instanceID)
	if err != nil {
		log.Fatalf("Failed to get workflow state: %v", err)
	}

	fmt.Printf("\n工作流状态: %s, 当前步骤: %s\n", state.Status, state.CurrentStepID)

	// 显示恢复数据
	if state.ResumeData != nil {
		resumeDataJSON, _ := json.MarshalIndent(state.ResumeData, "", "  ")
		fmt.Printf("恢复数据:\n%s\n", string(resumeDataJSON))
	}

	// 模拟人工指导，发送事件继续工作流
	fmt.Println("\n发送人工指导事件...")
	err = contentWorkflow.HandleEvent(context.Background(), instanceID, "human-guidance", map[string]interface{}{
		"guidance": "增加更多关于AI个性化学习的例子，并改进第二段的语言表达",
	})

	if err != nil {
		log.Fatalf("Failed to handle guidance event: %v", err)
	}

	// 等待工作流完成
	time.Sleep(500 * time.Millisecond)

	// 最终检查工作流状态
	state, err = contentWorkflow.GetState(instanceID)
	if err != nil {
		log.Fatalf("Failed to get workflow state: %v", err)
	}

	fmt.Printf("\n最终工作流状态: %s\n", state.Status)
	fmt.Println("内容生成结果:")

	// 输出重要结果
	finalContent, ok := state.Results["finalContent"].(string)
	if ok {
		fmt.Printf("最终内容: %s\n", finalContent)
	}

	finalScore, ok := state.Results["finalScore"].(float64)
	if ok {
		fmt.Printf("最终评分: %.2f\n", finalScore)
	}

	fmt.Println("Workflow completed successfully!")
}

// 模拟函数
func generateInitialDraft(topic string) string {
	return fmt.Sprintf("这是关于'%s'的初始内容草稿。这个草稿需要更多细节和改进。", topic)
}

func calculateConfidenceScore(content string) float64 {
	// 模拟低置信度，以便示例能够触发暂停
	return 0.5
}

func improveWithGuidance(content string, guidance string) string {
	return fmt.Sprintf("%s\n\n根据指导改进: %s", content, guidance)
}

func calculateToneScore(content string) float64 {
	return 0.85
}

func calculateCompletenessScore(content string) float64 {
	return 0.90
}

func RunSuspendResumeExample() {
	fmt.Println("=== 带有数据恢复的事件驱动工作流示例 ===")
	fmt.Println("这个示例展示了如何暂停工作流并保存状态数据")
	fmt.Println("然后在恢复时使用这些数据继续执行")
	fmt.Println("===========================")

	runImplementation()
}
