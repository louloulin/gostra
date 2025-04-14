package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
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

// --- Actor-Managed Lifecycle Test ---

// Actor message to start a workflow
type StartWorkflowRequest struct {
	InputData map[string]interface{}
}

// Actor response for starting a workflow
type StartWorkflowResponse struct {
	InstanceID string
	Error      error
}

// Actor message to get workflow state
type GetWorkflowStateRequest struct {
	InstanceID string
}

// Actor response for getting workflow state
type GetWorkflowStateResponse struct {
	State *WorkflowState
	Error error
}

// WorkflowManagerActor manages workflow instances
type WorkflowManagerActor struct {
	workflow *EventWorkflow
	t        *testing.T // To allow logging within the actor for debugging
}

// Creates props for the WorkflowManagerActor
func createManagerActorProps(t *testing.T, workflow *EventWorkflow) *actor.Props {
	return actor.PropsFromProducer(func() actor.Actor {
		return &WorkflowManagerActor{workflow: workflow, t: t}
	})
}

// Receive handles messages for the WorkflowManagerActor
func (a *WorkflowManagerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *StartWorkflowRequest:
		a.t.Logf("ManagerActor received StartWorkflowRequest: %+v", msg)
		instanceID, err := a.workflow.StartWorkflow(context.Background(), msg.InputData)
		ctx.Respond(&StartWorkflowResponse{InstanceID: instanceID, Error: err})

	case *GetWorkflowStateRequest:
		a.t.Logf("ManagerActor received GetWorkflowStateRequest: %+v", msg)
		state, err := a.workflow.GetState(msg.InstanceID)
		ctx.Respond(&GetWorkflowStateResponse{State: state, Error: err})

	// Add handling for WorkflowEventMessage if needed, or use separate actors
	// case *WorkflowEventMessage:
	//  ... handle events ...

	default:
		a.t.Logf("ManagerActor received unknown message type: %T", msg)
	}
}

// TestWorkflowLifecycleViaActor tests starting and querying workflow via actor messages
func TestWorkflowLifecycleViaActor(t *testing.T) {
	// Setup workflow (can reuse steps from previous test)
	stateStore := NewInMemoryStateStore()
	workflow, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          "actor-lifecycle-workflow",
		Name:        "Actor Lifecycle Test Workflow",
		StartStepID: "start",
		StateStore:  stateStore,
	})
	assert.NoError(t, err, "Failed to create workflow")
	createTestSteps(t, workflow) // Use the same steps for simplicity

	// Setup actor system
	system := actor.NewActorSystem()

	// Spawn manager actor
	managerProps := createManagerActorProps(t, workflow)
	managerPID := system.Root.Spawn(managerProps)
	defer system.Root.Stop(managerPID)

	// 1. Send StartWorkflowRequest
	startReq := &StartWorkflowRequest{InputData: map[string]interface{}{"input": "lifecycle test"}}
	startFuture := system.Root.RequestFuture(managerPID, startReq, 2*time.Second)
	startResult, err := startFuture.Result()
	assert.NoError(t, err, "Error requesting workflow start")

	startResp, ok := startResult.(*StartWorkflowResponse)
	assert.True(t, ok, "Expected StartWorkflowResponse, got %T", startResult)
	assert.NoError(t, startResp.Error, "Actor returned error on start")
	assert.NotEmpty(t, startResp.InstanceID, "Expected instance ID from start response")
	instanceID := startResp.InstanceID
	t.Logf("Workflow started via actor, InstanceID: %s", instanceID)

	// Wait briefly for workflow to potentially suspend (based on createTestSteps logic)
	time.Sleep(200 * time.Millisecond)

	// 2. Send GetWorkflowStateRequest
	getStateReq := &GetWorkflowStateRequest{InstanceID: instanceID}
	getStateFuture := system.Root.RequestFuture(managerPID, getStateReq, 2*time.Second)
	getStateResult, err := getStateFuture.Result()
	assert.NoError(t, err, "Error requesting workflow state")

	getStateResp, ok := getStateResult.(*GetWorkflowStateResponse)
	assert.True(t, ok, "Expected GetWorkflowStateResponse, got %T", getStateResult)
	assert.NoError(t, getStateResp.Error, "Actor returned error getting state")
	assert.NotNil(t, getStateResp.State, "Expected state in get response")

	// Verify the state retrieved via actor
	state := getStateResp.State
	assert.Equal(t, instanceID, state.WorkflowID)
	assert.Equal(t, EventStatusSuspended, state.Status, "Expected workflow to be suspended")
	assert.Equal(t, "wait-for-input", state.CurrentStepID, "Expected current step to be wait-for-input")
	assert.Contains(t, state.Results, "startStep", "Expected startStep result in state")
	t.Logf("Workflow state retrieved via actor: Status=%s, CurrentStep=%s", state.Status, state.CurrentStepID)

	// Note: This test doesn't resume the workflow via actor,
	// TestActorEventWorkflow already covers resuming via HandleEvent.
}

// --- Test Step Calling Actor ---

// Message for the responder actor
type PingMessage struct {
	Data string
}

// Response from the responder actor
type PongMessage struct {
	Response string
}

// ResponderActor simply replies to PingMessage
type ResponderActor struct{}

func (a *ResponderActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *PingMessage:
		ctx.Respond(&PongMessage{Response: "Pong: " + msg.Data})
	}
}

// TestWorkflowStepCallsActor tests a step that calls another actor
func TestWorkflowStepCallsActor(t *testing.T) {
	// Setup Actor System
	system := actor.NewActorSystem()
	defer system.Shutdown()

	// Setup Workflow
	stateStore := NewInMemoryStateStore()
	workflow, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          "step-calls-actor-workflow",
		Name:        "Step Calls Actor Test Workflow",
		StartStepID: "start-call",
		StateStore:  stateStore,
	})
	assert.NoError(t, err, "Failed to create workflow")

	// Define steps
	// Start step that triggers the actor call step
	startCallStep := workflow.CreateStep(
		"start-call",
		"Start Call Step",
		"Initiates the actor call",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"trigger": "go"}, nil
		},
		map[string]string{"": "call-actor-step"},
	)

	// Step that calls the actor
	callActorStep := workflow.CreateStep(
		"call-actor-step",
		"Call Actor Step",
		"This step spawns and calls a responder actor",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			t.Log("Executing call-actor-step")
			// Spawn the responder actor
			responderProps := actor.PropsFromProducer(func() actor.Actor { return &ResponderActor{} })
			responderPID := system.Root.Spawn(responderProps)
			defer system.Root.Stop(responderPID)

			// Send message and await response
			ping := &PingMessage{Data: "hello from step"}
			future := system.Root.RequestFuture(responderPID, ping, 2*time.Second)
			result, err := future.Result()
			if err != nil {
				t.Errorf("Error requesting from responder actor: %v", err)
				return nil, err
			}

			pong, ok := result.(*PongMessage)
			if !ok {
				t.Errorf("Unexpected response type from responder actor: %T", result)
				return nil, errors.New("unexpected actor response type")
			}
			t.Logf("Received response from actor: %s", pong.Response)

			// Return actor response in step result
			return map[string]interface{}{"actor_response": pong.Response}, nil
		},
		map[string]string{"": "finish-call"}, // Next step
	)

	// Final step
	finishCallStep := workflow.CreateStep(
		"finish-call",
		"Finish Call Step",
		"Completes the workflow after actor call",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"final_status": "done"}, nil
		},
		nil, // End workflow
	)

	// Add steps to workflow
	workflow.AddStep(startCallStep)
	workflow.AddStep(callActorStep)
	workflow.AddStep(finishCallStep)

	// Run the workflow directly (no manager actor needed for this test focus)
	instanceID, err := workflow.StartWorkflow(context.Background(), map[string]interface{}{"input": "step-actor-test"})
	assert.NoError(t, err, "Failed to start workflow")
	assert.NotEmpty(t, instanceID)

	// Wait for completion (adjust time if needed)
	time.Sleep(300 * time.Millisecond)

	// Verify final state and results
	finalState, err := workflow.GetState(instanceID)
	assert.NoError(t, err, "Failed to get final state")
	assert.Equal(t, EventStatusCompleted, finalState.Status, "Workflow did not complete")

	// Check that the actor's response is in the results
	assert.Contains(t, finalState.Results, "actor_response", "Actor response missing from final results")
	actorResp, ok := finalState.Results["actor_response"].(string)
	assert.True(t, ok, "Actor response is not a string")
	assert.Equal(t, "Pong: hello from step", actorResp, "Incorrect actor response in final results")
	assert.Contains(t, finalState.Results, "final_status", "Final step result missing")
	t.Logf("Final workflow results: %+v", finalState.Results)
}

// --- Test Step Actor Call Error ---

// FailingActor demonstrates error handling
type FailingActor struct{}

func (a *FailingActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *PingMessage:
		_ = msg // Explicitly ignore msg to fix linter warning
		// Simulate failure
		ctx.Respond(errors.New("actor failed intentionally for test")) // Respond with an error
		// Alternatively, cause a panic or timeout:
		// panic("Simulated actor panic")
		// time.Sleep(5 * time.Second) // Simulate timeout
	default:
		// ignore
	}
}

// TestWorkflowStepActorCallError tests when an actor called by a step fails
func TestWorkflowStepActorCallError(t *testing.T) {
	// Setup Actor System
	system := actor.NewActorSystem()
	defer system.Shutdown()

	// Setup Workflow
	stateStore := NewInMemoryStateStore()
	workflow, err := NewEventWorkflow(EventWorkflowOptions{
		ID:          "step-actor-error-workflow",
		Name:        "Step Actor Error Test Workflow",
		StartStepID: "start-error-call",
		StateStore:  stateStore,
	})
	assert.NoError(t, err, "Failed to create workflow")

	// Define steps
	// Start step
	startErrorCallStep := workflow.CreateStep(
		"start-error-call", "Start Error Call", "Initiates the failing actor call",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"trigger": "fail"}, nil
		},
		map[string]string{"": "call-failing-actor"},
	)

	// Step that calls the failing actor
	callFailingActorStep := workflow.CreateStep(
		"call-failing-actor", "Call Failing Actor", "Calls an actor designed to fail",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			t.Log("Executing call-failing-actor step")
			// Spawn the failing actor
			failingProps := actor.PropsFromProducer(func() actor.Actor { return &FailingActor{} })
			failingPID := system.Root.Spawn(failingProps)
			// defer system.Root.Stop(failingPID) // Stop might interfere with checking error state

			// Send message and expect error
			ping := &PingMessage{Data: "trigger failure"}
			// Use a shorter timeout to also catch potential actor non-responsiveness
			future := system.Root.RequestFuture(failingPID, ping, 500*time.Millisecond)
			_, err := future.Result() // We expect an error here

			// IMPORTANT: The step function MUST return the error for the workflow to fail
			if err != nil {
				t.Logf("Actor call failed as expected: %v", err)
				// Return the error to fail the step and workflow
				return map[string]interface{}{"failure_reason": err.Error()}, err
			} else {
				t.Error("Actor call succeeded unexpectedly")
				return nil, errors.New("actor call succeeded unexpectedly")
			}
		},
		map[string]string{"": "should-not-run"}, // This step should not be reached
	)

	// Step that should not run
	shouldNotRunStep := workflow.CreateStep(
		"should-not-run", "Should Not Run", "This step indicates failure if executed",
		func(ctx context.Context, data interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
			t.Error("Error: 'should-not-run' step was executed after failure")
			return map[string]interface{}{"unexpected_execution": true}, nil
		},
		nil,
	)

	// Add steps
	workflow.AddStep(startErrorCallStep)
	workflow.AddStep(callFailingActorStep)
	workflow.AddStep(shouldNotRunStep)

	// Run workflow
	instanceID, err := workflow.StartWorkflow(context.Background(), map[string]interface{}{"input": "error-test"})
	assert.NoError(t, err, "Failed to start workflow")
	assert.NotEmpty(t, instanceID)

	// Wait for workflow to process (likely fail quickly)
	time.Sleep(600 * time.Millisecond)

	// Verify final state is Failed
	finalState, err := workflow.GetState(instanceID)
	assert.NoError(t, err, "Failed to get final state")
	assert.Equal(t, EventStatusFailed, finalState.Status, "Workflow did not enter Failed state")

	// Verify the failed step and error message
	assert.Equal(t, "call-failing-actor", finalState.CurrentStepID, "Expected failure on call-failing-actor step")
	failedStepResult := finalState.Results["call-failing-actor"]
	assert.NotNil(t, failedStepResult, "Result for failed step missing")

	resultMap, ok := failedStepResult.(map[string]interface{})
	assert.True(t, ok, "Failed step result is not a map")
	assert.Contains(t, resultMap, "failure_reason", "Failure reason missing from step result")
	assert.Contains(t, resultMap["failure_reason"], "actor failed intentionally", "Incorrect error message captured")
	t.Logf("Workflow failed as expected. Failed step result: %+v", resultMap)

	// Ensure the subsequent step did not run
	assert.NotContains(t, finalState.Results, "unexpected_execution", "'should-not-run' step was executed")
}
