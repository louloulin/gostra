package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
)

// Status 定义工作流的状态
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Step 表示工作流中的一个步骤
type Step struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Execute     StepExecuteFunc        `json:"-"`
	Next        []string               `json:"next,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// StepExecuteFunc 定义步骤执行函数
type StepExecuteFunc func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error)

// StepResult 表示步骤执行的结果
type StepResult struct {
	StepID    string                 `json:"step_id"`
	Status    Status                 `json:"status"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time,omitempty"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// WorkflowOptions 定义创建工作流的选项
type WorkflowOptions struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Steps       []*Step                `json:"steps"`
	StartStep   string                 `json:"start_step"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Workflow 表示一个工作流
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Steps       map[string]*Step       `json:"steps"`
	StartStep   string                 `json:"start_step"`
	Status      Status                 `json:"status"`
	Results     map[string]*StepResult `json:"results"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	mu          sync.RWMutex
}

// NewWorkflow 创建一个新的工作流
func NewWorkflow(opts *WorkflowOptions) (*Workflow, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	if len(opts.Steps) == 0 {
		return nil, errors.New("workflow must have at least one step")
	}

	if opts.StartStep == "" {
		return nil, errors.New("start step must be specified")
	}

	// 检查开始步骤是否存在
	var startStepExists bool
	for _, step := range opts.Steps {
		if step.ID == opts.StartStep {
			startStepExists = true
			break
		}
	}

	if !startStepExists {
		return nil, fmt.Errorf("start step '%s' not found in steps", opts.StartStep)
	}

	id := opts.ID
	if id == "" {
		id = uuid.New().String()
	}

	name := opts.Name
	if name == "" {
		name = "Workflow-" + id[:8]
	}

	// 构建步骤映射
	steps := make(map[string]*Step)
	for _, step := range opts.Steps {
		if step.ID == "" {
			return nil, errors.New("step ID cannot be empty")
		}
		if step.Execute == nil {
			return nil, fmt.Errorf("step '%s' has no execute function", step.ID)
		}
		steps[step.ID] = step
	}

	// 验证步骤的下一步是否存在
	for _, step := range steps {
		for _, nextID := range step.Next {
			if _, exists := steps[nextID]; !exists {
				return nil, fmt.Errorf("next step '%s' for step '%s' not found", nextID, step.ID)
			}
		}
	}

	workflow := &Workflow{
		ID:          id,
		Name:        name,
		Description: opts.Description,
		Steps:       steps,
		StartStep:   opts.StartStep,
		Status:      StatusPending,
		Results:     make(map[string]*StepResult),
		Context:     make(map[string]interface{}),
		Metadata:    opts.Metadata,
	}

	return workflow, nil
}

// Run 运行工作流
func (w *Workflow) Run(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	w.mu.Lock()
	if w.Status == StatusRunning {
		w.mu.Unlock()
		return nil, errors.New("workflow is already running")
	}

	w.Status = StatusRunning
	w.Results = make(map[string]*StepResult)

	// 将输入添加到上下文
	if input != nil {
		for k, v := range input {
			w.Context[k] = v
		}
	}
	w.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			w.mu.Lock()
			w.Status = StatusFailed
			w.mu.Unlock()
			log.Printf("Workflow '%s' panic: %v", w.ID, r)
		}
	}()

	// 执行从开始步骤开始的所有步骤
	result, err := w.executeStep(ctx, w.StartStep, w.Context)
	w.mu.Lock()
	defer w.mu.Unlock()

	if err != nil {
		w.Status = StatusFailed
		return nil, err
	}

	w.Status = StatusCompleted
	return result, nil
}

// executeStep 执行单个步骤，并递归执行后续步骤
func (w *Workflow) executeStep(ctx context.Context, stepID string, input map[string]interface{}) (map[string]interface{}, error) {
	w.mu.RLock()
	step, exists := w.Steps[stepID]
	w.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("step '%s' not found", stepID)
	}

	// 创建步骤结果
	result := &StepResult{
		StepID:    stepID,
		Status:    StatusRunning,
		StartTime: time.Now(),
		Output:    make(map[string]interface{}),
	}

	w.mu.Lock()
	w.Results[stepID] = result
	w.mu.Unlock()

	// 执行步骤
	log.Printf("Executing step '%s' of workflow '%s'", stepID, w.ID)
	output, err := step.Execute(ctx, input, w)

	w.mu.Lock()
	result.EndTime = time.Now()

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		w.mu.Unlock()
		return nil, fmt.Errorf("step '%s' failed: %w", stepID, err)
	}

	result.Status = StatusCompleted
	result.Output = output
	w.mu.Unlock()

	// 如果没有下一步，返回当前步骤的输出
	if len(step.Next) == 0 {
		return output, nil
	}

	// 否则，执行下一步
	finalOutput := make(map[string]interface{})

	// 合并输入和当前步骤的输出
	mergedInput := make(map[string]interface{})
	for k, v := range input {
		mergedInput[k] = v
	}
	for k, v := range output {
		mergedInput[k] = v
	}

	// 如果只有一个下一步，直接执行
	if len(step.Next) == 1 {
		return w.executeStep(ctx, step.Next[0], mergedInput)
	}

	// 如果有多个下一步，并行执行
	var wg sync.WaitGroup
	var mu sync.Mutex
	var finalErr error

	for _, nextID := range step.Next {
		wg.Add(1)
		go func(nextStepID string) {
			defer wg.Done()
			nextOutput, nextErr := w.executeStep(ctx, nextStepID, mergedInput)

			mu.Lock()
			defer mu.Unlock()

			if nextErr != nil {
				if finalErr == nil {
					finalErr = nextErr
				}
				return
			}

			// 合并输出
			for k, v := range nextOutput {
				finalOutput[k] = v
			}
		}(nextID)
	}

	wg.Wait()

	if finalErr != nil {
		return nil, finalErr
	}

	return finalOutput, nil
}

// GetStepResult 获取步骤的结果
func (w *Workflow) GetStepResult(stepID string) *StepResult {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.Results[stepID]
}

// GetStatus 获取工作流状态
func (w *Workflow) GetStatus() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.Status
}

// Cancel 取消工作流
func (w *Workflow) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.Status == StatusRunning {
		w.Status = StatusCancelled
	}
}

// WorkflowActor 是工作流的Actor实现
type WorkflowActor struct {
	workflow *Workflow
	pid      *actor.PID
}

// NewWorkflowActor 创建工作流Actor
func NewWorkflowActor(workflow *Workflow) actor.Actor {
	return &WorkflowActor{
		workflow: workflow,
	}
}

// Receive 处理接收到的消息
func (a *WorkflowActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		a.pid = context.Self()
		log.Printf("WorkflowActor '%s' started", a.workflow.ID)

	case *actor.Stopping:
		log.Printf("WorkflowActor '%s' stopping", a.workflow.ID)

	case *actor.Stopped:
		log.Printf("WorkflowActor '%s' stopped", a.workflow.ID)

	case *RunWorkflowMessage:
		a.handleRunWorkflow(context, msg)

	case *GetWorkflowStatusMessage:
		a.handleGetWorkflowStatus(context)

	case *CancelWorkflowMessage:
		a.handleCancelWorkflow(context)

	default:
		log.Printf("WorkflowActor '%s' received unknown message: %T", a.workflow.ID, msg)
	}
}

// handleRunWorkflow 处理运行工作流的消息
func (a *WorkflowActor) handleRunWorkflow(actorContext actor.Context, msg *RunWorkflowMessage) {
	go func() {
		// 创建一个可取消的上下文
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 启动一个goroutine监听取消消息
		go func() {
			for {
				if a.workflow.GetStatus() == StatusCancelled {
					cancel()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()

		// 运行工作流
		result, err := a.workflow.Run(ctx, msg.Input)

		// 确定响应目标
		var target *actor.PID
		if msg.ResponsePID != nil {
			// 优先使用消息中指定的响应PID
			target = msg.ResponsePID
		} else if actorContext.Sender() != nil {
			// 如果消息中没有指定响应PID，则使用发送者
			target = actorContext.Sender()
		} else {
			// 如果都没有有效的目标，则记录警告并返回
			log.Printf("Warning: No valid target to send workflow result for workflow '%s'", a.workflow.ID)
			return
		}

		if err != nil {
			actorContext.Send(target, &WorkflowErrorResponse{
				WorkflowID: a.workflow.ID,
				Error:      err.Error(),
			})
			return
		}

		actorContext.Send(target, &WorkflowCompleteResponse{
			WorkflowID: a.workflow.ID,
			Result:     result,
		})
	}()
}

// handleGetWorkflowStatus 处理获取工作流状态的消息
func (a *WorkflowActor) handleGetWorkflowStatus(context actor.Context) {
	// 获取工作流的状态
	status := a.workflow.GetStatus()

	// 获取工作流的结果
	results := a.workflow.Results

	// 检查发送者是否为nil
	sender := context.Sender()
	if sender == nil {
		log.Printf("Warning: Sender is nil, cannot send workflow status for workflow '%s'", a.workflow.ID)
		return
	}

	// 发送响应
	context.Respond(&WorkflowStatusResponse{
		WorkflowID: a.workflow.ID,
		Status:     string(status),
		Results:    results,
	})
}

// handleCancelWorkflow 处理取消工作流的消息
func (a *WorkflowActor) handleCancelWorkflow(context actor.Context) {
	// 取消工作流
	a.workflow.Cancel()

	// 检查发送者是否为nil
	sender := context.Sender()
	if sender == nil {
		log.Printf("Warning: Sender is nil, cannot send workflow cancel response for workflow '%s'", a.workflow.ID)
		return
	}

	// 发送响应
	context.Respond(&WorkflowCancelResponse{
		WorkflowID: a.workflow.ID,
		Status:     string(a.workflow.GetStatus()),
	})
}

// WorkflowManager 管理多个工作流
type WorkflowManager struct {
	actorSystem    *actor.ActorSystem
	rootContext    *actor.RootContext
	workflowActors map[string]*actor.PID
	mu             sync.RWMutex
}

// NewWorkflowManager 创建工作流管理器
func NewWorkflowManager(actorSystem *actor.ActorSystem) *WorkflowManager {
	return &WorkflowManager{
		actorSystem:    actorSystem,
		rootContext:    actor.NewRootContext(actorSystem, nil),
		workflowActors: make(map[string]*actor.PID),
	}
}

// RegisterWorkflow 注册工作流
func (m *WorkflowManager) RegisterWorkflow(workflow *Workflow) (*actor.PID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.workflowActors[workflow.ID]; exists {
		return nil, fmt.Errorf("workflow '%s' already registered", workflow.ID)
	}

	// 创建工作流Actor
	props := actor.PropsFromProducer(func() actor.Actor {
		return NewWorkflowActor(workflow)
	})

	// 启动Actor
	pid, err := m.rootContext.SpawnNamed(props, "workflow-"+workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn workflow actor: %w", err)
	}

	m.workflowActors[workflow.ID] = pid

	return pid, nil
}

// RunWorkflow 运行工作流
func (m *WorkflowManager) RunWorkflow(workflowID string, input map[string]interface{}) (map[string]interface{}, error) {
	m.mu.RLock()
	pid, exists := m.workflowActors[workflowID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow '%s' not found", workflowID)
	}

	// 创建一个通道来接收工作流完成响应
	responseChan := make(chan interface{})

	// 创建一个临时的响应处理actor
	responseProps := actor.PropsFromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *WorkflowCompleteResponse:
			responseChan <- msg
		case *WorkflowErrorResponse:
			responseChan <- msg
		}
	})

	// 启动响应处理actor
	responsePID := m.rootContext.Spawn(responseProps)
	defer m.rootContext.Stop(responsePID) // 确保在函数返回时停止actor

	// 发送运行消息，包含响应actor的PID
	m.rootContext.Send(pid, &RunWorkflowMessage{
		Input:       input,
		ResponsePID: responsePID,
	})

	// 使用超时等待响应
	select {
	case resp := <-responseChan:
		switch response := resp.(type) {
		case *WorkflowCompleteResponse:
			return response.Result, nil
		case *WorkflowErrorResponse:
			return nil, errors.New(response.Error)
		default:
			return nil, fmt.Errorf("unexpected response type: %T", resp)
		}
	case <-time.After(2 * time.Minute):
		return nil, errors.New("workflow execution timed out")
	}
}

// GetWorkflowStatus 获取工作流状态
func (m *WorkflowManager) GetWorkflowStatus(workflowID string) (*WorkflowStatusResponse, error) {
	m.mu.RLock()
	pid, exists := m.workflowActors[workflowID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow '%s' not found", workflowID)
	}

	// 发送状态查询消息
	future := m.rootContext.RequestFuture(pid, &GetWorkflowStatusMessage{}, 5*time.Second)

	// 等待响应
	result, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow status: %w", err)
	}

	// 处理响应
	response, ok := result.(*WorkflowStatusResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return response, nil
}

// CancelWorkflow 取消工作流
func (m *WorkflowManager) CancelWorkflow(workflowID string) error {
	m.mu.RLock()
	pid, exists := m.workflowActors[workflowID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workflow '%s' not found", workflowID)
	}

	// 发送取消消息
	future := m.rootContext.RequestFuture(pid, &CancelWorkflowMessage{}, 5*time.Second)

	// 等待响应
	result, err := future.Result()
	if err != nil {
		return fmt.Errorf("failed to cancel workflow: %w", err)
	}

	// 处理响应
	_, ok := result.(*WorkflowCancelResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", result)
	}

	return nil
}

// UnregisterWorkflow 取消注册工作流
func (m *WorkflowManager) UnregisterWorkflow(workflowID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pid, exists := m.workflowActors[workflowID]
	if !exists {
		return fmt.Errorf("workflow '%s' not found", workflowID)
	}

	// 停止Actor
	m.rootContext.Stop(pid)

	// 从映射中删除
	delete(m.workflowActors, workflowID)

	return nil
}

// GetAllWorkflows 获取所有已注册的工作流ID
func (m *WorkflowManager) GetAllWorkflows() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.workflowActors))
	for id := range m.workflowActors {
		ids = append(ids, id)
	}

	return ids
}

// SerializeWorkflow 将工作流序列化为JSON
func SerializeWorkflow(w *Workflow) (string, error) {
	// 创建一个可序列化的工作流
	type SerializableWorkflow struct {
		ID          string                 `json:"id"`
		Name        string                 `json:"name"`
		Description string                 `json:"description,omitempty"`
		Steps       map[string]*Step       `json:"steps"`
		StartStep   string                 `json:"start_step"`
		Status      Status                 `json:"status"`
		Results     map[string]*StepResult `json:"results"`
		Context     map[string]interface{} `json:"context,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}

	serializableWorkflow := &SerializableWorkflow{
		ID:          w.ID,
		Name:        w.Name,
		Description: w.Description,
		Steps:       w.Steps,
		StartStep:   w.StartStep,
		Status:      w.Status,
		Results:     w.Results,
		Context:     w.Context,
		Metadata:    w.Metadata,
	}

	data, err := json.Marshal(serializableWorkflow)
	if err != nil {
		return "", fmt.Errorf("failed to serialize workflow: %w", err)
	}

	return string(data), nil
}

// 消息定义

// RunWorkflowMessage 运行工作流的消息
type RunWorkflowMessage struct {
	Input       map[string]interface{} `json:"input,omitempty"`
	ResponsePID *actor.PID             `json:"response_pid,omitempty"`
}

// GetWorkflowStatusMessage 获取工作流状态的消息
type GetWorkflowStatusMessage struct{}

// CancelWorkflowMessage 取消工作流的消息
type CancelWorkflowMessage struct{}

// WorkflowCompleteResponse 工作流完成的响应
type WorkflowCompleteResponse struct {
	WorkflowID string                 `json:"workflow_id"`
	Result     map[string]interface{} `json:"result"`
}

// WorkflowErrorResponse 工作流错误的响应
type WorkflowErrorResponse struct {
	WorkflowID string `json:"workflow_id"`
	Error      string `json:"error"`
}

// WorkflowStatusResponse 工作流状态的响应
type WorkflowStatusResponse struct {
	WorkflowID string                 `json:"workflow_id"`
	Status     string                 `json:"status"`
	Results    map[string]*StepResult `json:"results,omitempty"`
}

// WorkflowCancelResponse 取消工作流的响应
type WorkflowCancelResponse struct {
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
}
