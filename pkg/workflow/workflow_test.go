package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

// TestWorkflowCreation 测试工作流创建
func TestWorkflowCreation(t *testing.T) {
	// 创建两个步骤
	step1 := &Step{
		ID:          "step1",
		Name:        "Step 1",
		Description: "First step",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			return map[string]interface{}{"step1_result": "done"}, nil
		},
		Next: []string{"step2"},
	}

	step2 := &Step{
		ID:          "step2",
		Name:        "Step 2",
		Description: "Second step",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			// 使用前一步骤的结果
			step1Result := input["step1_result"].(string)
			return map[string]interface{}{
				"step1_result": step1Result,
				"step2_result": "completed",
			}, nil
		},
	}

	// 创建工作流
	workflow, err := NewWorkflow(&WorkflowOptions{
		Name:        "Test Workflow",
		Description: "A simple test workflow",
		Steps:       []*Step{step1, step2},
		StartStep:   "step1",
	})

	// 验证工作流创建成功
	assert.NoError(t, err)
	assert.NotNil(t, workflow)
	assert.Equal(t, "Test Workflow", workflow.Name)
	assert.Equal(t, "A simple test workflow", workflow.Description)
	assert.Equal(t, StatusPending, workflow.Status)
	assert.Equal(t, 2, len(workflow.Steps))
	assert.Equal(t, "step1", workflow.StartStep)
}

// TestWorkflowExecution 测试工作流执行
func TestWorkflowExecution(t *testing.T) {
	// 创建两个步骤
	step1 := &Step{
		ID:   "step1",
		Name: "Step 1",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			// 模拟处理
			time.Sleep(50 * time.Millisecond)
			return map[string]interface{}{"step1_result": "done"}, nil
		},
		Next: []string{"step2"},
	}

	step2 := &Step{
		ID:   "step2",
		Name: "Step 2",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			// 模拟处理
			time.Sleep(50 * time.Millisecond)

			// 使用前一步骤的结果
			step1Result := input["step1_result"].(string)
			return map[string]interface{}{
				"step1_result": step1Result,
				"step2_result": "completed",
				"final":        true,
			}, nil
		},
	}

	// 创建工作流
	workflow, err := NewWorkflow(&WorkflowOptions{
		Name:        "Test Workflow",
		Description: "A simple test workflow",
		Steps:       []*Step{step1, step2},
		StartStep:   "step1",
	})
	assert.NoError(t, err)

	// 执行工作流
	result, err := workflow.Run(context.Background(), map[string]interface{}{
		"initial": "value",
	})

	// 验证执行结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusCompleted, workflow.Status)
	assert.Equal(t, "done", result["step1_result"])
	assert.Equal(t, "completed", result["step2_result"])
	assert.Equal(t, true, result["final"])

	// 验证步骤结果
	step1Result := workflow.GetStepResult("step1")
	assert.NotNil(t, step1Result)
	assert.Equal(t, StatusCompleted, step1Result.Status)
	assert.Equal(t, "done", step1Result.Output["step1_result"])

	step2Result := workflow.GetStepResult("step2")
	assert.NotNil(t, step2Result)
	assert.Equal(t, StatusCompleted, step2Result.Status)
	assert.Equal(t, "completed", step2Result.Output["step2_result"])
}

// TestParallelWorkflow 测试并行工作流
func TestParallelWorkflow(t *testing.T) {
	// 创建初始步骤
	startStep := &Step{
		ID:   "start",
		Name: "Start Step",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			return map[string]interface{}{"start_data": "initial"}, nil
		},
		Next: []string{"parallel1", "parallel2"},
	}

	// 创建两个并行步骤
	parallel1 := &Step{
		ID:   "parallel1",
		Name: "Parallel 1",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return map[string]interface{}{"parallel1_result": "done1"}, nil
		},
		Next: []string{"final"},
	}

	parallel2 := &Step{
		ID:   "parallel2",
		Name: "Parallel 2",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return map[string]interface{}{"parallel2_result": "done2"}, nil
		},
		Next: []string{"final"},
	}

	// 创建最终步骤
	finalStep := &Step{
		ID:   "final",
		Name: "Final Step",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			// 合并所有前面步骤的结果
			result := map[string]interface{}{
				"final_result": "completed",
			}

			// 复制输入中的所有键
			for k, v := range input {
				result[k] = v
			}

			return result, nil
		},
	}

	// 创建工作流
	workflow, err := NewWorkflow(&WorkflowOptions{
		Name:      "Parallel Workflow",
		Steps:     []*Step{startStep, parallel1, parallel2, finalStep},
		StartStep: "start",
	})
	assert.NoError(t, err)

	// 执行工作流
	result, err := workflow.Run(context.Background(), nil)

	// 验证执行结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusCompleted, workflow.Status)
	assert.Equal(t, "initial", result["start_data"])
	assert.Equal(t, "done1", result["parallel1_result"])
	assert.Equal(t, "done2", result["parallel2_result"])
	assert.Equal(t, "completed", result["final_result"])
}

// TestWorkflowActorAndManager 测试工作流Actor和管理器
func TestWorkflowActorAndManager(t *testing.T) {
	// 创建一个简单的工作流
	step := &Step{
		ID:   "simple",
		Name: "Simple Step",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		},
	}

	workflow, err := NewWorkflow(&WorkflowOptions{
		Name:      "Actor Test Workflow",
		Steps:     []*Step{step},
		StartStep: "simple",
	})
	assert.NoError(t, err)

	// 创建Actor系统
	system := actor.NewActorSystem()

	// 创建工作流管理器
	manager := NewWorkflowManager(system)

	// 注册工作流
	pid, err := manager.RegisterWorkflow(workflow)
	assert.NoError(t, err)
	assert.NotNil(t, pid)

	// 验证注册
	workflows := manager.GetAllWorkflows()
	assert.Equal(t, 1, len(workflows))
	assert.Contains(t, workflows, workflow.ID)

	// 运行工作流
	result, err := manager.RunWorkflow(workflow.ID, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "success", result["result"])

	// 获取状态
	status, err := manager.GetWorkflowStatus(workflow.ID)
	assert.NoError(t, err)
	assert.Equal(t, workflow.ID, status.WorkflowID)
	assert.Equal(t, string(StatusCompleted), status.Status)

	// 取消注册
	err = manager.UnregisterWorkflow(workflow.ID)
	assert.NoError(t, err)

	// 验证取消注册
	workflows = manager.GetAllWorkflows()
	assert.Equal(t, 0, len(workflows))
}

// TestWorkflowCancellation 测试工作流取消
func TestWorkflowCancellation(t *testing.T) {
	// 创建一个长时间运行的步骤
	longStep := &Step{
		ID:   "long",
		Name: "Long Step",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second): // 长时间运行
				return map[string]interface{}{"result": "completed"}, nil
			}
		},
	}

	workflow, err := NewWorkflow(&WorkflowOptions{
		Name:      "Cancellation Test",
		Steps:     []*Step{longStep},
		StartStep: "long",
	})
	assert.NoError(t, err)

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 在另一个goroutine中运行工作流
	resChan := make(chan map[string]interface{})
	errChan := make(chan error)

	go func() {
		result, err := workflow.Run(ctx, nil)
		if err != nil {
			errChan <- err
			return
		}
		resChan <- result
	}()

	// 等待一会儿后取消上下文
	time.Sleep(100 * time.Millisecond)
	cancel()

	// 等待结果或错误
	select {
	case <-resChan:
		t.Fail() // 不应该有成功的结果
	case err := <-errChan:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	case <-time.After(200 * time.Millisecond):
		// 也可能是工作流内部处理了取消，没有返回错误
		assert.NotEqual(t, StatusCompleted, workflow.GetStatus())
	}
}
