package workflow

import (
	"context"
	"testing"
	"time"
)

func TestEventWorkflow(t *testing.T) {
	// 创建内存状态存储
	stateStore := NewInMemoryStateStore()

	// 创建工作流
	workflow, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          "test-workflow",
		Name:        "Test Workflow",
		Description: "A test workflow for events",
		StartStepID: "start",
		StateStore:  stateStore,
	})

	if err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// 测试结果记录
	results := make(map[string]interface{})

	// 创建步骤
	startStep := workflow.CreateStep(
		"start",
		"Start Step",
		"The first step in the workflow",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"startStep": "completed",
			}, nil
		},
		map[string]string{
			"": "wait-for-input", // 默认下一步，无需事件
		},
	)

	waitStep := workflow.CreateStep(
		"wait-for-input",
		"Wait for Input",
		"Waits for user input",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			// 这步会暂停工作流，等待外部事件
			results["waitStep"] = "waiting"
			return map[string]interface{}{
				"waitStep": "waiting",
			}, nil
		},
		map[string]string{
			"user-input": "final", // 只有user-input事件才能继续
		},
	)

	finalStep := workflow.CreateStep(
		"final",
		"Final Step",
		"The final step",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			// 获取事件数据
			results["finalStep"] = "completed"
			results["eventData"] = data
			return map[string]interface{}{
				"finalStep": "completed",
			}, nil
		},
		nil, // 没有下一步，工作流结束
	)

	// 添加步骤到工作流
	if err := workflow.AddStep(startStep); err != nil {
		t.Fatalf("Failed to add start step: %v", err)
	}
	if err := workflow.AddStep(waitStep); err != nil {
		t.Fatalf("Failed to add wait step: %v", err)
	}
	if err := workflow.AddStep(finalStep); err != nil {
		t.Fatalf("Failed to add final step: %v", err)
	}

	// 启动工作流
	instanceID, err := workflow.StartWorkflow(context.Background(), map[string]interface{}{
		"input": "initial data",
	})

	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// 等待工作流执行到等待步骤
	time.Sleep(200 * time.Millisecond)

	// 检查工作流状态
	state, err := workflow.GetState(instanceID)
	if err != nil {
		t.Fatalf("Failed to get workflow state: %v", err)
	}

	// 验证工作流已暂停在等待步骤
	if state.Status != EventStatusSuspended {
		t.Errorf("Expected workflow to be suspended, got %s", state.Status)
	}

	if state.CurrentStepID != "wait-for-input" {
		t.Errorf("Expected current step to be wait-for-input, got %s", state.CurrentStepID)
	}

	// 发送事件继续工作流
	err = workflow.HandleEvent(context.Background(), instanceID, "user-input", map[string]string{
		"message": "Hello from test",
	})

	if err != nil {
		t.Fatalf("Failed to handle event: %v", err)
	}

	// 等待工作流完成
	time.Sleep(200 * time.Millisecond)

	// 再次检查工作流状态
	state, err = workflow.GetState(instanceID)
	if err != nil {
		t.Fatalf("Failed to get workflow state: %v", err)
	}

	// 验证工作流已完成
	if state.Status != EventStatusCompleted {
		t.Errorf("Expected workflow to be completed, got %s", state.Status)
	}

	// 验证最终结果包含了我们期望的数据
	if state.Results["startStep"] != "completed" {
		t.Errorf("Missing expected startStep result")
	}

	if state.Results["waitStep"] != "waiting" {
		t.Errorf("Missing expected waitStep result")
	}

	if state.Results["finalStep"] != "completed" {
		t.Errorf("Missing expected finalStep result")
	}
}

func TestEventWorkflowAPI(t *testing.T) {
	// 创建内存状态存储
	stateStore := NewInMemoryStateStore()

	// 创建工作流
	workflow, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          "api-test-workflow",
		Name:        "API Test Workflow",
		Description: "A test workflow for the API",
		StartStepID: "start",
		StateStore:  stateStore,
	})

	if err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// 创建步骤
	startStep := workflow.CreateStep(
		"start",
		"Start Step",
		"The first step in the workflow",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"startStep": "completed",
			}, nil
		},
		map[string]string{
			"": "wait-step", // 默认下一步，无需事件
		},
	)

	waitStep := workflow.CreateStep(
		"wait-step",
		"Wait Step",
		"Waits for user input",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"waitStep": "waiting",
			}, nil
		},
		map[string]string{
			"user-input": "final", // 只有user-input事件才能继续
		},
	)

	finalStep := workflow.CreateStep(
		"final",
		"Final Step",
		"The final step",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"finalStep": "completed",
			}, nil
		},
		nil, // 没有下一步，工作流结束
	)

	// 添加步骤到工作流
	workflow.AddStep(startStep)
	workflow.AddStep(waitStep)
	workflow.AddStep(finalStep)

	// 创建API处理器
	api := NewEventWorkflowAPI(stateStore)
	api.RegisterWorkflow(workflow)

	// 在这里我们可以测试API逻辑，但不会启动HTTP服务器
	// 这里只验证工作流注册逻辑

	// 验证工作流已正确注册
	if len(api.workflows) != 1 {
		t.Errorf("Expected 1 workflow to be registered, got %d", len(api.workflows))
	}

	if _, exists := api.workflows["api-test-workflow"]; !exists {
		t.Errorf("Expected workflow 'api-test-workflow' to be registered")
	}
}
