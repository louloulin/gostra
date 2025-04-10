package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType 表示事件类型
type EventType string

// EventWorkflowStatus 表示事件工作流状态
type EventWorkflowStatus string

// 工作流状态常量
const (
	EventStatusPending   EventWorkflowStatus = "PENDING"
	EventStatusRunning   EventWorkflowStatus = "RUNNING"
	EventStatusCompleted EventWorkflowStatus = "COMPLETED"
	EventStatusFailed    EventWorkflowStatus = "FAILED"
	EventStatusCanceled  EventWorkflowStatus = "CANCELED"
	EventStatusSuspended EventWorkflowStatus = "SUSPENDED"
)

// Event 表示一个事件
type Event struct {
	Type EventType
	Data interface{}
}

// WorkflowStateStore 定义工作流状态存储接口
type WorkflowStateStore interface {
	SaveWorkflowState(state *WorkflowState) error
	LoadWorkflowState(workflowID string) (*WorkflowState, error)
	ListWorkflowStates() ([]*WorkflowState, error)
	DeleteWorkflowState(workflowID string) error
}

// WorkflowState 表示工作流的状态
type WorkflowState struct {
	WorkflowID    string                 `json:"workflowId"`
	WorkflowName  string                 `json:"workflowName"`
	Status        EventWorkflowStatus    `json:"status"`
	StartTime     time.Time              `json:"startTime"`
	LastUpdated   time.Time              `json:"lastUpdated"`
	CurrentStepID string                 `json:"currentStepId,omitempty"`
	Results       map[string]interface{} `json:"results,omitempty"`
	SuspendedAt   time.Time              `json:"suspendedAt,omitempty"`
	ResumeData    interface{}            `json:"resumeData,omitempty"`
}

// EventHandler 表示事件处理器
type EventHandler func(ctx context.Context, eventData interface{}, stepData map[string]interface{}) (map[string]interface{}, error)

// EventStep 表示事件工作流的一个步骤
type EventStep struct {
	ID          string
	Name        string
	Description string
	Handler     EventHandler
	NextSteps   map[string]string // 事件类型到下一步ID的映射，"*"表示任何事件
}

// EventWorkflow 表示事件驱动的工作流
type EventWorkflow struct {
	id          string
	name        string
	description string
	steps       map[string]*EventStep
	instances   map[string]*workflowInstance
	startStepID string
	stateStore  WorkflowStateStore
	mutex       sync.RWMutex
}

// workflowInstance 表示一个工作流实例
type workflowInstance struct {
	id          string
	workflowID  string
	data        map[string]interface{}
	currentStep string
	status      EventWorkflowStatus
	createdAt   time.Time
}

// EventWorkflowOptions 表示创建事件工作流的选项
type EventWorkflowOptions struct {
	ID          string
	Name        string
	Description string
	StartStepID string
	StateStore  WorkflowStateStore
}

// NewEventWorkflow 创建一个新的事件工作流
func NewEventWorkflow(opts EventWorkflowOptions) (*EventWorkflow, error) {
	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}

	if opts.Name == "" {
		return nil, errors.New("workflow name is required")
	}

	if opts.StartStepID == "" {
		return nil, errors.New("start step ID is required")
	}

	// 如果未提供状态存储，使用内存存储
	if opts.StateStore == nil {
		opts.StateStore = NewInMemoryStateStore()
	}

	return &EventWorkflow{
		id:          opts.ID,
		name:        opts.Name,
		description: opts.Description,
		steps:       make(map[string]*EventStep),
		instances:   make(map[string]*workflowInstance),
		startStepID: opts.StartStepID,
		stateStore:  opts.StateStore,
		mutex:       sync.RWMutex{},
	}, nil
}

// ID 返回工作流ID
func (w *EventWorkflow) ID() string {
	return w.id
}

// Name 返回工作流名称
func (w *EventWorkflow) Name() string {
	return w.name
}

// Description 返回工作流描述
func (w *EventWorkflow) Description() string {
	return w.description
}

// AddStep 添加一个步骤到工作流
func (w *EventWorkflow) AddStep(step *EventStep) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if step.ID == "" {
		return errors.New("step ID is required")
	}

	if step.Handler == nil {
		return errors.New("step handler is required")
	}

	// 检查步骤ID是否已存在
	if _, exists := w.steps[step.ID]; exists {
		return fmt.Errorf("step with ID %s already exists", step.ID)
	}

	w.steps[step.ID] = step

	return nil
}

// CreateInstance 创建一个新的工作流实例
func (w *EventWorkflow) CreateInstance(inputs map[string]interface{}) string {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	instanceID := uuid.New().String()

	instance := &workflowInstance{
		id:          instanceID,
		workflowID:  w.id,
		data:        inputs,
		currentStep: w.startStepID,
		status:      EventStatusPending,
		createdAt:   time.Now(),
	}

	w.instances[instanceID] = instance

	// 创建初始状态
	state := &WorkflowState{
		WorkflowID:    instanceID,
		WorkflowName:  w.name,
		Status:        EventStatusPending,
		StartTime:     time.Now(),
		LastUpdated:   time.Now(),
		CurrentStepID: w.startStepID,
		Results:       make(map[string]interface{}),
	}

	err := w.stateStore.SaveWorkflowState(state)
	if err != nil {
		// 记录错误但继续
		fmt.Printf("Error saving workflow state: %v\n", err)
	}

	// 异步启动工作流
	go w.startWorkflow(context.Background(), instanceID)

	return instanceID
}

// StartWorkflow 创建工作流实例并返回实例ID
func (w *EventWorkflow) StartWorkflow(ctx context.Context, data map[string]interface{}) (string, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 验证起始步骤是否存在
	if _, exists := w.steps[w.startStepID]; !exists {
		return "", fmt.Errorf("start step '%s' not found", w.startStepID)
	}

	// 创建唯一ID
	instanceID := uuid.New().String()

	// 如果没有提供数据，初始化一个空map
	if data == nil {
		data = make(map[string]interface{})
	}

	// 添加工作流基本信息到数据
	data["workflow"] = map[string]interface{}{
		"id":   w.id,
		"name": w.name,
	}

	// 创建新的实例
	instance := &workflowInstance{
		id:         instanceID,
		workflowID: w.id,
		data:       data,
		createdAt:  time.Now(),
	}

	// 保存实例到内存
	w.instances[instanceID] = instance

	// 创建初始状态
	state := &WorkflowState{
		WorkflowID:    instanceID,
		WorkflowName:  w.name,
		Status:        EventStatusPending,
		StartTime:     time.Now(),
		LastUpdated:   time.Now(),
		CurrentStepID: w.startStepID,
		Results:       make(map[string]interface{}),
		ResumeData:    nil, // 确保恢复数据字段已初始化
	}

	// 保存初始状态
	err := w.stateStore.SaveWorkflowState(state)
	if err != nil {
		delete(w.instances, instanceID)
		return "", fmt.Errorf("failed to save initial workflow state: %w", err)
	}

	// 异步启动工作流执行
	go w.startWorkflow(ctx, instanceID)

	return instanceID, nil
}

// startWorkflow 开始执行工作流
func (w *EventWorkflow) startWorkflow(ctx context.Context, instanceID string) {
	// 加载工作流状态
	state, err := w.stateStore.LoadWorkflowState(instanceID)
	if err != nil {
		fmt.Printf("Error loading workflow state: %v\n", err)
		return
	}

	state.Status = EventStatusRunning
	state.LastUpdated = time.Now()

	err = w.stateStore.SaveWorkflowState(state)
	if err != nil {
		fmt.Printf("Error saving workflow state: %v\n", err)
		return
	}

	// 加载实例数据
	w.mutex.Lock()
	instance, exists := w.instances[instanceID]
	w.mutex.Unlock()

	if !exists {
		fmt.Printf("Workflow instance not found: %s\n", instanceID)
		return
	}

	// 确保实例数据中有恢复数据字段
	if instance.data == nil {
		instance.data = make(map[string]interface{})
	}

	// 复制实例数据，以便不修改原始对象
	dataCopy := make(map[string]interface{})
	for k, v := range instance.data {
		dataCopy[k] = v
	}

	// 执行第一个步骤
	err = w.executeStep(ctx, instanceID, dataCopy, w.startStepID)
	if err != nil {
		w.handleStepError(instanceID, w.startStepID, err)
	}
}

// executeStep 执行单个步骤
func (w *EventWorkflow) executeStep(ctx context.Context, instanceID string, data map[string]interface{}, stepID string) error {
	w.mutex.RLock()
	step, exists := w.steps[stepID]
	w.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("step not found: %s", stepID)
	}

	// 加载状态
	state, err := w.stateStore.LoadWorkflowState(instanceID)
	if err != nil {
		return err
	}

	// 更新当前步骤
	state.CurrentStepID = stepID
	state.LastUpdated = time.Now()

	// 确保数据包含恢复数据
	if state.ResumeData != nil && data != nil {
		data["resumeData"] = state.ResumeData
	}

	err = w.stateStore.SaveWorkflowState(state)
	if err != nil {
		return err
	}

	// 执行步骤处理器
	result, err := step.Handler(ctx, data, state.Results)
	if err != nil {
		return err
	}

	// 重新加载状态以确保一致性
	state, err = w.stateStore.LoadWorkflowState(instanceID)
	if err != nil {
		return err
	}

	// 合并结果
	if result != nil {
		// 特殊处理resumeData
		if resumeData, ok := result["resumeData"]; ok {
			state.ResumeData = resumeData
			delete(result, "resumeData") // 从普通结果中移除，避免重复
		}

		// 合并其他结果
		for k, v := range result {
			state.Results[k] = v
		}
	}

	// 如果没有下一步，完成工作流
	if len(step.NextSteps) == 0 {
		state.Status = EventStatusCompleted
		state.LastUpdated = time.Now()

		err = w.stateStore.SaveWorkflowState(state)
		if err != nil {
			return err
		}

		return nil
	}

	// 如果此步骤需要等待事件，暂停工作流
	if _, waitForEvent := step.NextSteps["*"]; waitForEvent {
		state.Status = EventStatusSuspended
		state.SuspendedAt = time.Now()
		state.LastUpdated = time.Now()

		err = w.stateStore.SaveWorkflowState(state)
		if err != nil {
			return err
		}

		return nil
	}

	// 如果有默认下一步，继续执行
	if nextStepID, hasDefault := step.NextSteps[""]; hasDefault {
		return w.executeStep(ctx, instanceID, data, nextStepID)
	}

	// 如果没有默认下一步但有其他步骤，等待事件
	state.Status = EventStatusSuspended
	state.SuspendedAt = time.Now()
	state.LastUpdated = time.Now()

	err = w.stateStore.SaveWorkflowState(state)
	if err != nil {
		return err
	}

	return nil
}

// handleStepError 处理步骤执行错误
func (w *EventWorkflow) handleStepError(instanceID, stepID string, execError error) {
	state, err := w.stateStore.LoadWorkflowState(instanceID)
	if err != nil {
		fmt.Printf("Error loading workflow state: %v\n", err)
		return
	}

	state.Status = EventStatusFailed
	state.LastUpdated = time.Now()
	state.Results["error"] = execError.Error()

	err = w.stateStore.SaveWorkflowState(state)
	if err != nil {
		fmt.Printf("Error saving workflow state: %v\n", err)
	}
}

// HandleEvent 处理发送到工作流实例的事件
func (w *EventWorkflow) HandleEvent(ctx context.Context, instanceID string, eventType EventType, eventData interface{}) error {
	// 加载状态
	state, err := w.stateStore.LoadWorkflowState(instanceID)
	if err != nil {
		return err
	}

	// 检查工作流是否已暂停
	if state.Status != EventStatusSuspended {
		return errors.New("workflow is not in suspended state, cannot handle event")
	}

	// 获取当前步骤
	w.mutex.RLock()
	step, exists := w.steps[state.CurrentStepID]
	w.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("current step not found: %s", state.CurrentStepID)
	}

	// 确定下一步
	var nextStepID string
	var found bool

	// 首先检查是否有匹配此事件类型的下一步
	if nextID, ok := step.NextSteps[string(eventType)]; ok {
		nextStepID = nextID
		found = true
	} else if nextID, ok := step.NextSteps["*"]; ok {
		// 然后检查是否有通配符
		nextStepID = nextID
		found = true
	}

	if !found {
		return fmt.Errorf("no matching next step found for event type: %s", eventType)
	}

	// 更新状态为运行中
	state.Status = EventStatusRunning
	state.LastUpdated = time.Now()

	err = w.stateStore.SaveWorkflowState(state)
	if err != nil {
		return err
	}

	// 将事件数据添加到工作流数据
	w.mutex.Lock()
	instance, exists := w.instances[instanceID]
	if !exists {
		w.mutex.Unlock()
		return fmt.Errorf("workflow instance not found: %s", instanceID)
	}

	// 添加事件数据
	instance.data["event"] = map[string]interface{}{
		"type": eventType,
		"data": eventData,
	}

	// 如果有恢复数据，也添加到实例数据
	if state.ResumeData != nil {
		instance.data["resumeData"] = state.ResumeData
	}

	// 创建数据副本用于下一步，确保包含resumeData
	dataCopy := make(map[string]interface{})
	for k, v := range instance.data {
		dataCopy[k] = v
	}

	w.mutex.Unlock()

	// 执行下一步，传递包含恢复数据的新数据
	err = w.executeStep(ctx, instanceID, dataCopy, nextStepID)
	if err != nil {
		w.handleStepError(instanceID, nextStepID, err)
		return err
	}

	return nil
}

// Cancel 取消工作流
func (w *EventWorkflow) Cancel(instanceID string) error {
	state, err := w.stateStore.LoadWorkflowState(instanceID)
	if err != nil {
		return err
	}

	// 如果工作流已经完成或失败，无需取消
	if state.Status == EventStatusCompleted || state.Status == EventStatusFailed || state.Status == EventStatusCanceled {
		return nil
	}

	// 更新状态为已取消
	state.Status = EventStatusCanceled
	state.LastUpdated = time.Now()

	return w.stateStore.SaveWorkflowState(state)
}

// GetState 获取工作流实例状态
func (w *EventWorkflow) GetState(instanceID string) (*WorkflowState, error) {
	return w.stateStore.LoadWorkflowState(instanceID)
}

// InMemoryStateStore 提供内存中的工作流状态存储
type InMemoryStateStore struct {
	states map[string]*WorkflowState
	mutex  sync.RWMutex
}

// NewInMemoryStateStore 创建一个新的内存状态存储
func NewInMemoryStateStore() *InMemoryStateStore {
	return &InMemoryStateStore{
		states: make(map[string]*WorkflowState),
		mutex:  sync.RWMutex{},
	}
}

// SaveWorkflowState 保存工作流状态
func (s *InMemoryStateStore) SaveWorkflowState(state *WorkflowState) error {
	if state == nil {
		return errors.New("state cannot be nil")
	}

	if state.WorkflowID == "" {
		return errors.New("workflow ID is required")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 创建状态的副本进行存储
	stateCopy := *state
	s.states[state.WorkflowID] = &stateCopy

	return nil
}

// LoadWorkflowState 加载工作流状态
func (s *InMemoryStateStore) LoadWorkflowState(workflowID string) (*WorkflowState, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	state, exists := s.states[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow state not found for ID: %s", workflowID)
	}

	// 创建状态的副本返回
	stateCopy := *state
	return &stateCopy, nil
}

// ListWorkflowStates 列出所有工作流状态
func (s *InMemoryStateStore) ListWorkflowStates() ([]*WorkflowState, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	states := make([]*WorkflowState, 0, len(s.states))
	for _, state := range s.states {
		stateCopy := *state
		states = append(states, &stateCopy)
	}

	return states, nil
}

// DeleteWorkflowState 删除工作流状态
func (s *InMemoryStateStore) DeleteWorkflowState(workflowID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.states[workflowID]; !exists {
		return fmt.Errorf("workflow state not found for ID: %s", workflowID)
	}

	delete(s.states, workflowID)
	return nil
}

// ListInstances 列出所有工作流实例状态
func (w *EventWorkflow) ListInstances() ([]*WorkflowState, error) {
	return w.stateStore.ListWorkflowStates()
}

// GetInstanceState 获取指定实例的状态
func (w *EventWorkflow) GetInstanceState(instanceID string) (*WorkflowState, error) {
	return w.stateStore.LoadWorkflowState(instanceID)
}

// RemoveInstance 从工作流中移除指定实例
func (w *EventWorkflow) RemoveInstance(instanceID string) error {
	// 先取消工作流
	err := w.Cancel(instanceID)
	if err != nil {
		return err
	}

	// 从内存中移除实例
	w.mutex.Lock()
	delete(w.instances, instanceID)
	w.mutex.Unlock()

	// 从存储中删除状态
	return w.stateStore.DeleteWorkflowState(instanceID)
}

// CreateStep 创建新步骤的辅助方法
func (w *EventWorkflow) CreateStep(id, name, description string, handler EventHandler, nextSteps map[string]string) *EventStep {
	if nextSteps == nil {
		nextSteps = make(map[string]string)
	}

	return &EventStep{
		ID:          id,
		Name:        name,
		Description: description,
		Handler:     handler,
		NextSteps:   nextSteps,
	}
}
