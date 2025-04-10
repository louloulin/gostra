package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// 模拟Actor消息
type WorkflowEventMessage struct {
	InstanceID string
	EventType  string
	EventData  interface{}
}

// 模拟Actor响应
type WorkflowEventResponse struct {
	Success bool
	Error   string
}

// 测试用的Actor Props
func createTestWorkflowActor(t *testing.T, workflow *EventWorkflow) *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *WorkflowEventMessage:
			t.Logf("Actor received event message: %v", msg)

			// 处理事件
			err := workflow.HandleEvent(context.Background(), msg.InstanceID, EventType(msg.EventType), msg.EventData)

			// 发送响应
			if err != nil {
				ctx.Respond(&WorkflowEventResponse{
					Success: false,
					Error:   err.Error(),
				})
			} else {
				ctx.Respond(&WorkflowEventResponse{
					Success: true,
				})
			}
		}
	})
}

// 测试事件工作流与Actor的集成
func TestActorEventWorkflow(t *testing.T) {
	// 创建内存状态存储
	stateStore := NewInMemoryStateStore()

	// 创建工作流
	workflow, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          "actor-test-workflow",
		Name:        "Actor Test Workflow",
		Description: "Testing actor integration with event workflow",
		StartStepID: "start",
		StateStore:  stateStore,
	})

	if err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// 创建步骤
	createTestSteps(t, workflow)

	// 创建Actor系统
	system := actor.NewActorSystem()

	// 创建工作流Actor
	props := createTestWorkflowActor(t, workflow)
	pid := system.Root.Spawn(props)

	// 启动工作流
	instanceID, err := workflow.StartWorkflow(context.Background(), map[string]interface{}{
		"input": "actor test data",
	})

	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// 等待工作流暂停
	time.Sleep(200 * time.Millisecond)

	// 验证工作流状态
	state, err := workflow.GetState(instanceID)
	if err != nil {
		t.Fatalf("Failed to get workflow state: %v", err)
	}

	if state.Status != EventStatusSuspended {
		t.Errorf("Expected workflow to be suspended, got %s", state.Status)
	}

	if state.CurrentStepID != "wait-for-input" {
		t.Errorf("Expected current step to be wait-for-input, got %s", state.CurrentStepID)
	}

	// 通过Actor发送事件
	eventMsg := &WorkflowEventMessage{
		InstanceID: instanceID,
		EventType:  "user-input",
		EventData: map[string]interface{}{
			"message": "Hello from actor",
		},
	}

	// 发送消息并等待响应
	future := system.Root.RequestFuture(pid, eventMsg, 2*time.Second)
	result, err := future.Result()
	if err != nil {
		t.Fatalf("Failed to get response from actor: %v", err)
	}

	// 检查响应
	response, ok := result.(*WorkflowEventResponse)
	if !ok {
		t.Fatalf("Expected WorkflowEventResponse, got %T", result)
	}

	if !response.Success {
		t.Errorf("Event handling failed: %s", response.Error)
	}

	// 等待工作流完成
	time.Sleep(200 * time.Millisecond)

	// 验证最终状态
	state, err = workflow.GetState(instanceID)
	if err != nil {
		t.Fatalf("Failed to get workflow state: %v", err)
	}

	if state.Status != EventStatusCompleted {
		t.Errorf("Expected workflow to be completed, got %s", state.Status)
	}

	// 验证结果
	if state.Results["finalStep"] != "completed" {
		t.Errorf("Expected finalStep=completed in results, got %v", state.Results["finalStep"])
	}

	// 清理
	system.Root.Stop(pid)
}

// 创建测试步骤
func createTestSteps(t *testing.T, workflow *EventWorkflow) {
	// 创建起始步骤
	startStep := workflow.CreateStep(
		"start",
		"Start Step",
		"The first step in the workflow",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"startStep": "completed",
				"resumeData": map[string]interface{}{
					"actorTest": true,
					"timestamp": time.Now().Unix(),
				},
			}, nil
		},
		map[string]string{
			"": "wait-for-input", // 默认下一步
		},
	)

	// 创建等待步骤
	waitStep := workflow.CreateStep(
		"wait-for-input",
		"Wait for Input",
		"Waits for user input",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"waitStep": "waiting",
			}, nil
		},
		map[string]string{
			"user-input": "final", // 用户输入事件
		},
	)

	// 创建最终步骤
	finalStep := workflow.CreateStep(
		"final",
		"Final Step",
		"The final step",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			dataMap, ok := data.(map[string]interface{})
			if ok {
				t.Logf("Final step data: %v", dataMap)
			}

			return map[string]interface{}{
				"finalStep": "completed",
				"actorTest": "passed",
			}, nil
		},
		nil, // 工作流结束
	)

	// 添加步骤到工作流
	workflow.AddStep(startStep)
	workflow.AddStep(waitStep)
	workflow.AddStep(finalStep)
}
