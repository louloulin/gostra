package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

// TestEventWorkflowIncorrectEvent tests sending an incorrect event to a suspended workflow
func TestEventWorkflowIncorrectEvent(t *testing.T) {
	// Create memory state store
	stateStore := NewInMemoryStateStore()

	// Create workflow
	workflow, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          "test-incorrect-event",
		Name:        "Test Incorrect Event Workflow",
		StartStepID: "step1",
		StateStore:  stateStore,
	})
	assert.NoError(t, err)

	// Step 1: Simple start step
	step1 := workflow.CreateStep("step1", "Step 1", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"step1": "done"}, nil
		},
		map[string]string{"": "wait_step"},
	)
	workflow.AddStep(step1)

	// Step 2: Waits for 'correct_event'
	waitStep := workflow.CreateStep("wait_step", "Wait Step", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			// This step will suspend
			return map[string]interface{}{"waited": true}, nil
		},
		map[string]string{"correct_event": "final_step"},
	)
	workflow.AddStep(waitStep)

	// Step 3: Final step
	finalStep := workflow.CreateStep("final_step", "Final Step", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"final": "reached"}, nil
		},
		nil,
	)
	workflow.AddStep(finalStep)

	// Start workflow
	instanceID, err := workflow.StartWorkflow(context.Background(), nil)
	assert.NoError(t, err)

	// Wait for workflow to suspend
	time.Sleep(100 * time.Millisecond)

	// Check state is suspended at wait_step
	state, err := workflow.GetState(instanceID)
	assert.NoError(t, err)
	assert.Equal(t, EventStatusSuspended, state.Status)
	assert.Equal(t, "wait_step", state.CurrentStepID)

	// Handle incorrect event
	err = workflow.HandleEvent(context.Background(), instanceID, "incorrect_event", map[string]string{"data": "wrong"})
	// Expect an error indicating the event is not expected or workflow remains suspended
	// Depending on implementation, HandleEvent might return an error or just do nothing.
	// For now, let's assume it doesn't error out but doesn't advance the workflow.
	// assert.Error(t, err) // Uncomment if HandleEvent should return an error for unexpected events
	assert.NoError(t, err) // Assume no error returned, just no state change

	// Wait a bit to ensure no state change occurred
	time.Sleep(50 * time.Millisecond)

	// Check state again - should still be suspended at the same step
	stateAfterIncorrect, err := workflow.GetState(instanceID)
	assert.NoError(t, err)
	assert.Equal(t, EventStatusSuspended, stateAfterIncorrect.Status, "Status should remain Suspended")
	assert.Equal(t, "wait_step", stateAfterIncorrect.CurrentStepID, "CurrentStepID should remain wait_step")

	// Verify final step was not reached
	assert.Nil(t, stateAfterIncorrect.Results["final"], "Final step should not have been reached")
}

// TestEventWorkflowMultipleEventPaths tests a step waiting for one of several events
func TestEventWorkflowMultipleEventPaths(t *testing.T) {
	// Common setup function for this test
	setup := func(t *testing.T) (*EventWorkflow, WorkflowStateStore) {
		stateStore := NewInMemoryStateStore()
		workflow, err := NewEventWorkflow(EventWorkflowOptions{
			ID:          "test-multi-event",
			Name:        "Test Multi Event Workflow",
			StartStepID: "start",
			StateStore:  stateStore,
		})
		assert.NoError(t, err)

		// Step 1: Start
		workflow.AddStep(workflow.CreateStep("start", "Start", "",
			func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{"start": "done"}, nil
			},
			map[string]string{"": "wait_multi"},
		))

		// Step 2: Wait for Event A or Event B
		workflow.AddStep(workflow.CreateStep("wait_multi", "Wait Multi", "",
			func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
				// Suspend step
				return map[string]interface{}{"waited": true}, nil
			},
			map[string]string{
				"event_A": "path_A",
				"event_B": "path_B",
			},
		))

		// Step 3a: Path A
		workflow.AddStep(workflow.CreateStep("path_A", "Path A", "",
			func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{"path": "A", "event_data": data}, nil
			},
			nil,
		))

		// Step 3b: Path B
		workflow.AddStep(workflow.CreateStep("path_B", "Path B", "",
			func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{"path": "B", "event_data": data}, nil
			},
			nil,
		))

		return workflow, stateStore
	}

	// Test Case 1: Trigger Event A
	t.Run("TriggerEventA", func(t *testing.T) {
		workflow, _ := setup(t)
		instanceID, err := workflow.StartWorkflow(context.Background(), nil)
		assert.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Wait for suspension

		eventDataA := map[string]string{"payloadA": "data for A"}
		err = workflow.HandleEvent(context.Background(), instanceID, "event_A", eventDataA)
		assert.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Wait for completion

		state, err := workflow.GetState(instanceID)
		assert.NoError(t, err)
		assert.Equal(t, EventStatusCompleted, state.Status)
		assert.Equal(t, "path_A", state.CurrentStepID)
		assert.Equal(t, "A", state.Results["path"], "Should have taken Path A")
		assert.Equal(t, eventDataA, state.Results["event_data"], "Event data for A should be present")
		assert.Nil(t, state.Results["path_B"], "Path B step should not exist in results")
	})

	// Test Case 2: Trigger Event B
	t.Run("TriggerEventB", func(t *testing.T) {
		workflow, _ := setup(t)
		instanceID, err := workflow.StartWorkflow(context.Background(), nil)
		assert.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Wait for suspension

		eventDataB := map[string]int{"payloadB": 123}
		err = workflow.HandleEvent(context.Background(), instanceID, "event_B", eventDataB)
		assert.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Wait for completion

		state, err := workflow.GetState(instanceID)
		assert.NoError(t, err)
		assert.Equal(t, EventStatusCompleted, state.Status)
		assert.Equal(t, "path_B", state.CurrentStepID)
		assert.Equal(t, "B", state.Results["path"], "Should have taken Path B")
		assert.Equal(t, eventDataB, state.Results["event_data"], "Event data for B should be present")
		assert.Nil(t, state.Results["path_A"], "Path A step should not exist in results")
	})
}

// TestEventWorkflowStatePersistence tests resuming a workflow from persisted state
func TestEventWorkflowStatePersistence(t *testing.T) {
	stateStore := NewInMemoryStateStore()
	workflowID := "test-persistence-workflow"

	// --- Phase 1: Create, run, and suspend the workflow ---
	workflow1, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          workflowID,
		Name:        "Persistence Test Workflow",
		StartStepID: "start",
		StateStore:  stateStore,
	})
	assert.NoError(t, err)

	// Add steps
	workflow1.AddStep(workflow1.CreateStep("start", "Start", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"initial_data": sd["run_input"]}, nil // Store initial run input
		},
		map[string]string{"": "wait"},
	))
	workflow1.AddStep(workflow1.CreateStep("wait", "Wait", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"waited": true}, nil // Suspend here
		},
		map[string]string{"resume_event": "final"},
	))
	workflow1.AddStep(workflow1.CreateStep("final", "Final", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"final_data": data, "finished": true}, nil
		},
		nil,
	))

	// Start workflow1
	initialInput := map[string]interface{}{"run_input": "hello persistence"}
	instanceID, err := workflow1.StartWorkflow(context.Background(), initialInput)
	assert.NoError(t, err)
	assert.NotEmpty(t, instanceID)

	// Wait for suspension
	time.Sleep(100 * time.Millisecond)

	// Verify suspended state with workflow1
	state1, err := workflow1.GetState(instanceID)
	assert.NoError(t, err)
	assert.Equal(t, EventStatusSuspended, state1.Status)
	assert.Equal(t, "wait", state1.CurrentStepID)
	assert.Equal(t, initialInput["run_input"], state1.Results["initial_data"])

	// --- Phase 2: Create a new workflow instance and resume from state ---
	workflow2, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          workflowID, // Same ID
		Name:        "Persistence Test Workflow",
		StartStepID: "start", // Need to redefine steps for the new instance
		StateStore:  stateStore,
	})
	assert.NoError(t, err)

	// Re-add steps to workflow2 (as if the process restarted and redefined the workflow)
	workflow2.AddStep(workflow2.CreateStep("start", "Start", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"initial_data": sd["run_input"]}, nil
		},
		map[string]string{"": "wait"},
	))
	workflow2.AddStep(workflow2.CreateStep("wait", "Wait", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"waited": true}, nil
		},
		map[string]string{"resume_event": "final"},
	))
	workflow2.AddStep(workflow2.CreateStep("final", "Final", "",
		func(ctx context.Context, data interface{}, sd map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"final_data": data, "finished": true}, nil
		},
		nil,
	))

	// Send resume event using workflow2
	eventData := map[string]bool{"resumed": true}
	err = workflow2.HandleEvent(context.Background(), instanceID, "resume_event", eventData)
	assert.NoError(t, err)

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Verify final state using workflow2
	state2, err := workflow2.GetState(instanceID)
	assert.NoError(t, err)
	assert.Equal(t, EventStatusCompleted, state2.Status)
	assert.Equal(t, "final", state2.CurrentStepID)
	assert.Equal(t, initialInput["run_input"], state2.Results["initial_data"], "Initial data should persist")
	assert.Equal(t, true, state2.Results["waited"], "Wait step result should persist")
	assert.Equal(t, eventData, state2.Results["final_data"], "Event data should be in final step result")
	assert.Equal(t, true, state2.Results["finished"], "Final step marker should be present")
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
