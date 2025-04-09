package workflow

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// EventWorkflowAPI 封装事件驱动工作流的REST API处理
type EventWorkflowAPI struct {
	stateStore WorkflowStateStore
	workflows  map[string]*EventWorkflow
}

// NewEventWorkflowAPI 创建新的事件工作流API处理器
func NewEventWorkflowAPI(stateStore WorkflowStateStore) *EventWorkflowAPI {
	return &EventWorkflowAPI{
		stateStore: stateStore,
		workflows:  make(map[string]*EventWorkflow),
	}
}

// RegisterWorkflow 注册工作流到API处理器
func (api *EventWorkflowAPI) RegisterWorkflow(workflow *EventWorkflow) {
	api.workflows[workflow.ID()] = workflow
}

// jsonResponse 发送JSON响应
func jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// errorResponse 发送错误响应
func errorResponse(w http.ResponseWriter, message string, statusCode int) {
	jsonResponse(w, map[string]string{"error": message}, statusCode)
}

// 工作流列表响应
type workflowListResponse struct {
	Workflows []workflowSummary `json:"workflows"`
}

// 工作流摘要信息
type workflowSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"startTime"`
	LastUpdated time.Time `json:"lastUpdated"`
	CurrentStep string    `json:"currentStep,omitempty"`
	IsSuspended bool      `json:"isSuspended"`
}

// HandleListWorkflows 处理获取工作流列表的请求
func (api *EventWorkflowAPI) HandleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取存储中的所有工作流状态
	states, err := api.stateStore.ListWorkflowStates()
	if err != nil {
		errorResponse(w, fmt.Sprintf("Failed to list workflows: %v", err), http.StatusInternalServerError)
		return
	}

	// 构建响应
	response := workflowListResponse{
		Workflows: make([]workflowSummary, 0, len(states)),
	}

	for _, state := range states {
		isSuspended := state.Status == EventStatusSuspended

		summary := workflowSummary{
			ID:          state.WorkflowID,
			Name:        state.WorkflowName,
			Status:      string(state.Status),
			StartTime:   state.StartTime,
			LastUpdated: state.LastUpdated,
			IsSuspended: isSuspended,
		}

		if state.CurrentStepID != "" {
			summary.CurrentStep = state.CurrentStepID
		}

		response.Workflows = append(response.Workflows, summary)
	}

	jsonResponse(w, response, http.StatusOK)
}

// 工作流详情响应
type workflowDetailResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	StartTime   time.Time              `json:"startTime"`
	LastUpdated time.Time              `json:"lastUpdated"`
	CurrentStep string                 `json:"currentStep,omitempty"`
	IsSuspended bool                   `json:"isSuspended"`
	Results     map[string]interface{} `json:"results,omitempty"`
	SuspendedAt string                 `json:"suspendedAt,omitempty"`
	ResumeData  interface{}            `json:"resumeData,omitempty"`
}

// HandleGetWorkflow 处理获取单个工作流状态的请求
func (api *EventWorkflowAPI) HandleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取工作流ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errorResponse(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	workflowID := parts[len(parts)-1]

	// 获取工作流状态
	state, err := api.stateStore.LoadWorkflowState(workflowID)
	if err != nil {
		errorResponse(w, fmt.Sprintf("Failed to get workflow: %v", err), http.StatusNotFound)
		return
	}

	// 构建响应
	response := workflowDetailResponse{
		ID:          state.WorkflowID,
		Name:        state.WorkflowName,
		Status:      string(state.Status),
		StartTime:   state.StartTime,
		LastUpdated: state.LastUpdated,
		CurrentStep: state.CurrentStepID,
		IsSuspended: state.Status == EventStatusSuspended,
		Results:     state.Results,
	}

	if state.Status == EventStatusSuspended {
		response.SuspendedAt = state.SuspendedAt.Format(time.RFC3339)
		response.ResumeData = state.ResumeData
	}

	jsonResponse(w, response, http.StatusOK)
}

// 创建工作流请求
type createWorkflowRequest struct {
	WorkflowType string                 `json:"workflowType"`
	Inputs       map[string]interface{} `json:"inputs"`
}

// HandleCreateWorkflow 处理创建新工作流的请求
func (api *EventWorkflowAPI) HandleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	var req createWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// 根据类型获取工作流
	workflow, exists := api.workflows[req.WorkflowType]
	if !exists {
		errorResponse(w, fmt.Sprintf("Workflow type '%s' not found", req.WorkflowType), http.StatusBadRequest)
		return
	}

	// 创建新的工作流实例
	newWorkflowID := workflow.CreateInstance(req.Inputs)

	// 返回新创建的工作流ID
	jsonResponse(w, map[string]string{"workflowId": newWorkflowID}, http.StatusCreated)
}

// 发送事件请求
type sendEventRequest struct {
	EventType string      `json:"eventType"`
	EventData interface{} `json:"eventData"`
}

// HandleSendEvent 处理向工作流发送事件的请求
func (api *EventWorkflowAPI) HandleSendEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取工作流ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[len(parts)-2] != "events" {
		errorResponse(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	workflowID := parts[len(parts)-3]

	// 解析请求
	var req sendEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// 获取工作流状态
	state, err := api.stateStore.LoadWorkflowState(workflowID)
	if err != nil {
		errorResponse(w, fmt.Sprintf("Workflow not found: %v", err), http.StatusNotFound)
		return
	}

	// 检查工作流是否已暂停
	if state.Status != EventStatusSuspended {
		errorResponse(w, "Cannot send event to a workflow that is not suspended", http.StatusBadRequest)
		return
	}

	// 查找对应的工作流定义
	workflowDef, exists := api.workflows[state.WorkflowName]
	if !exists {
		errorResponse(w, "Workflow definition not found", http.StatusInternalServerError)
		return
	}

	// 处理事件
	err = workflowDef.HandleEvent(r.Context(), workflowID, EventType(req.EventType), req.EventData)
	if err != nil {
		errorResponse(w, fmt.Sprintf("Failed to handle event: %v", err), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "event processed"}, http.StatusOK)
}

// HandleCancelWorkflow 处理取消工作流的请求
func (api *EventWorkflowAPI) HandleCancelWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取工作流ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errorResponse(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	workflowID := parts[len(parts)-1]

	// 获取工作流状态
	state, err := api.stateStore.LoadWorkflowState(workflowID)
	if err != nil {
		errorResponse(w, fmt.Sprintf("Workflow not found: %v", err), http.StatusNotFound)
		return
	}

	// 查找对应的工作流定义
	workflowDef, exists := api.workflows[state.WorkflowName]
	if !exists {
		errorResponse(w, "Workflow definition not found", http.StatusInternalServerError)
		return
	}

	// 取消工作流
	err = workflowDef.Cancel(workflowID)
	if err != nil {
		errorResponse(w, fmt.Sprintf("Failed to cancel workflow: %v", err), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, nil, http.StatusNoContent)
}

// SetupRoutes 设置API路由
func (api *EventWorkflowAPI) SetupRoutes(mux *http.ServeMux, basePath string) {
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	// 列出所有工作流
	mux.HandleFunc(basePath+"workflows", api.HandleListWorkflows)

	// 创建新工作流
	mux.HandleFunc(basePath+"workflows/create", api.HandleCreateWorkflow)

	// 获取单个工作流状态
	mux.HandleFunc(basePath+"workflows/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, basePath+"workflows/")

		// 如果是发送事件
		if strings.Contains(path, "/events") {
			api.HandleSendEvent(w, r)
			return
		}

		// 如果是取消工作流
		if r.Method == http.MethodDelete {
			api.HandleCancelWorkflow(w, r)
			return
		}

		// 否则是获取工作流状态
		api.HandleGetWorkflow(w, r)
	})
}
