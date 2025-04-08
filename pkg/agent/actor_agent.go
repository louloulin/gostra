package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	stdctx "context" // 正确导入标准context包重命名为stdctx，避免冲突

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models"
	"github.com/yourusername/gostra/pkg/tools"
)

// ActorAgent 表示基于Actor模型的Agent实现
type ActorAgent struct {
	agent       *Agent                // 内部包含常规Agent实例
	behavior    actor.Behavior        // Actor行为状态机
	supervisor  *actor.PID            // 监督Actor的PID
	toolActors  map[string]*actor.PID // 工具Actor的PID映射
	context     actor.Context         // Actor上下文
	rootCtx     *actor.RootContext    // 根上下文
	mailbox     chan interface{}      // 当Agent暂停时缓存消息的邮箱
	isPaused    bool                  // 是否暂停状态
	children    map[string]*actor.PID // 子Actor列表
	actorSystem *actor.ActorSystem    // Actor系统
}

// ActorAgentOptions 创建ActorAgent的选项
type ActorAgentOptions struct {
	ID             string
	Name           string
	SystemPrompt   string
	ModelProvider  models.ModelProvider
	MemoryProvider memory.MemoryProvider
	Tools          []tools.Tool
	MaxTokens      int
	RootContext    *actor.RootContext
	Supervisor     *actor.PID
	ActorSystem    *actor.ActorSystem
}

// 创建一个新的ActorAgent
func NewActorAgent(opts *ActorAgentOptions) (*actor.Props, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	// 创建标准Agent
	agentOpts := &Options{
		ID:             opts.ID,
		Name:           opts.Name,
		SystemPrompt:   opts.SystemPrompt,
		ModelProvider:  opts.ModelProvider,
		MemoryProvider: opts.MemoryProvider,
		Tools:          opts.Tools,
		MaxTokens:      opts.MaxTokens,
	}

	agent, err := NewAgent(agentOpts)
	if err != nil {
		return nil, err
	}

	// 确定根上下文
	rootCtx := opts.RootContext
	if rootCtx == nil {
		// 获取ActorSystem
		actorSystem := opts.ActorSystem
		if actorSystem == nil {
			actorSystem = actor.NewActorSystem()
		}
		// 使用默认的没有中间件的根上下文
		rootCtx = actor.NewRootContext(actorSystem, nil)
	}

	// 创建ActorAgent
	actorAgent := &ActorAgent{
		agent:       agent,
		behavior:    actor.NewBehavior(),
		supervisor:  opts.Supervisor,
		toolActors:  make(map[string]*actor.PID),
		rootCtx:     rootCtx,
		mailbox:     make(chan interface{}, 100),
		isPaused:    false,
		children:    make(map[string]*actor.PID),
		actorSystem: rootCtx.ActorSystem(),
	}

	// 注册默认行为
	actorAgent.behavior.Become(actorAgent.idleBehavior)

	// 创建actor props
	props := actor.PropsFromProducer(func() actor.Actor {
		return actorAgent
	})

	return props, nil
}

// Receive 处理接收到的消息
func (a *ActorAgent) Receive(context actor.Context) {
	a.context = context
	a.behavior.Receive(context)
}

// idleBehavior 处理Agent空闲状态下的消息
func (a *ActorAgent) idleBehavior(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		a.onStarted(context)

	case *actor.Stopping:
		a.onStopping(context)

	case *actor.Stopped:
		a.onStopped(context)

	case *actor.Restarting:
		log.Printf("Agent %s is restarting...", a.agent.ID)

	case *AgentGenerateMessage:
		a.handleGenerateMessage(context, msg)

	case *AgentStreamMessage:
		a.handleStreamMessage(context, msg)

	case *AgentRunMessage:
		a.handleRunMessage(context, msg)

	case *PauseMessage:
		a.onPauseMessage(context)

	case *ResumeMessage:
		// 空闲状态下收到恢复消息，忽略
		context.Respond(&StatusResponse{
			Status: string(a.agent.StateManager.GetState().Status),
			Info:   "Agent is already active",
		})

	case *StatusMessage:
		a.handleStatusMessage(context)

	case *StopMessage:
		a.handleStopMessage(context)

	case *ToolResponseMessage:
		// 空闲状态下收到工具响应消息，忽略
		log.Printf("Agent %s received tool response while idle", a.agent.ID)

	case *AgentNetworkMessage:
		a.handleNetworkMessage(context, msg)

	default:
		log.Printf("Agent %s received unknown message: %T", a.agent.ID, msg)
	}
}

// runningBehavior 处理Agent运行状态下的消息
func (a *ActorAgent) runningBehavior(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started, *actor.Stopping, *actor.Stopped, *actor.Restarting:
		// 这些生命周期消息应该由默认行为处理
		a.idleBehavior(context)

	case *AgentGenerateMessage, *AgentStreamMessage, *AgentRunMessage:
		// 运行状态下不能再次运行
		context.Respond(&ErrorResponse{
			Error: "Agent is already running",
		})

	case *PauseMessage:
		a.onPauseMessage(context)

	case *ResumeMessage:
		// 运行状态下收到恢复消息，忽略
		context.Respond(&StatusResponse{
			Status: string(a.agent.StateManager.GetState().Status),
			Info:   "Agent is already running",
		})

	case *StatusMessage:
		a.handleStatusMessage(context)

	case *StopMessage:
		a.handleStopMessage(context)

	case *ToolResponseMessage:
		a.handleToolResponseMessage(context, msg)

	case *AgentNetworkMessage:
		a.handleNetworkMessage(context, msg)

	default:
		log.Printf("Agent %s received unknown message: %T", a.agent.ID, msg)
	}
}

// pausedBehavior 处理Agent暂停状态下的消息
func (a *ActorAgent) pausedBehavior(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started, *actor.Stopping, *actor.Stopped, *actor.Restarting:
		// 这些生命周期消息应该由默认行为处理
		a.idleBehavior(context)

	case *AgentGenerateMessage, *AgentStreamMessage, *AgentRunMessage:
		// 暂停状态下，缓存这些消息
		select {
		case a.mailbox <- msg:
			context.Respond(&StatusResponse{
				Status: string(a.agent.StateManager.GetState().Status),
				Info:   "Message queued, agent is paused",
			})
		default:
			context.Respond(&ErrorResponse{
				Error: "Agent mailbox is full",
			})
		}

	case *PauseMessage:
		// 已经暂停状态，忽略
		context.Respond(&StatusResponse{
			Status: string(a.agent.StateManager.GetState().Status),
			Info:   "Agent is already paused",
		})

	case *ResumeMessage:
		a.onResumeMessage(context)

	case *StatusMessage:
		a.handleStatusMessage(context)

	case *StopMessage:
		a.handleStopMessage(context)

	case *ToolResponseMessage:
		// 暂停状态下缓存工具响应
		select {
		case a.mailbox <- msg:
			log.Printf("Queued tool response, agent is paused")
		default:
			log.Printf("Failed to queue tool response, mailbox full")
		}

	case *AgentNetworkMessage:
		// 暂停状态下缓存网络消息
		select {
		case a.mailbox <- msg:
			log.Printf("Queued network message, agent is paused")
		default:
			log.Printf("Failed to queue network message, mailbox full")
		}

	default:
		log.Printf("Agent %s received unknown message: %T", a.agent.ID, msg)
	}
}

// onStarted 处理Actor启动事件
func (a *ActorAgent) onStarted(context actor.Context) {
	log.Printf("Agent %s started", a.agent.ID)

	// 初始化工具Actor
	for _, tool := range a.agent.Tools {
		toolID := tool.GetID()
		toolProps := actor.PropsFromProducer(func() actor.Actor {
			return NewToolActor(tool, context.Self())
		})
		pid := a.rootCtx.Spawn(toolProps)
		a.toolActors[toolID] = pid
		a.children[toolID] = pid
	}

	// 通知监督Actor已启动
	if a.supervisor != nil {
		a.rootCtx.Send(a.supervisor, &AgentStartedMessage{
			AgentID: a.agent.ID,
			Name:    a.agent.Name,
		})
	}
}

// onStopping 处理Actor即将停止事件
func (a *ActorAgent) onStopping(context actor.Context) {
	log.Printf("Agent %s is stopping...", a.agent.ID)

	// 停止所有子Actor
	for _, pid := range a.children {
		a.rootCtx.Stop(pid)
	}

	// 更新Agent状态
	a.agent.StateManager.SetStopped()
}

// onStopped 处理Actor已停止事件
func (a *ActorAgent) onStopped(context actor.Context) {
	log.Printf("Agent %s stopped", a.agent.ID)

	// 通知监督Actor已停止
	if a.supervisor != nil {
		a.rootCtx.Send(a.supervisor, &AgentStoppedMessage{
			AgentID: a.agent.ID,
		})
	}
}

// onPauseMessage 处理暂停消息
func (a *ActorAgent) onPauseMessage(context actor.Context) {
	a.isPaused = true
	a.agent.StateManager.SetPaused()
	a.behavior.Become(a.pausedBehavior)

	context.Respond(&StatusResponse{
		Status: string(a.agent.StateManager.GetState().Status),
		Info:   "Agent paused",
	})
}

// onResumeMessage 处理恢复消息
func (a *ActorAgent) onResumeMessage(context actor.Context) {
	a.isPaused = false

	// 如果有运行中的任务，恢复为运行状态，否则为空闲状态
	if a.agent.StateManager.GetState().CurrentTaskID != "" {
		a.agent.StateManager.UpdateStatus(StatusRunning)
		a.behavior.Become(a.runningBehavior)
	} else {
		a.agent.StateManager.SetIdle()
		a.behavior.Become(a.idleBehavior)
	}

	context.Respond(&StatusResponse{
		Status: string(a.agent.StateManager.GetState().Status),
		Info:   "Agent resumed",
	})

	// 处理缓存的消息
	go a.processQueuedMessages()
}

// processQueuedMessages 处理暂停期间缓存的消息
func (a *ActorAgent) processQueuedMessages() {
	for !a.isPaused {
		select {
		case msg, ok := <-a.mailbox:
			if !ok {
				return // 邮箱已关闭
			}

			// 发送消息给自己
			a.rootCtx.Send(a.context.Self(), msg)

		default:
			// 所有消息已处理
			return
		}
	}
}

// handleGenerateMessage 处理生成消息请求
func (a *ActorAgent) handleGenerateMessage(actorCtx actor.Context, msg *AgentGenerateMessage) {
	// 更新状态
	taskID := "task_" + uuid.New().String()
	threadID := msg.Options.ThreadID
	if threadID == "" {
		threadID = uuid.New().String()
		// 如果未提供线程ID，创建一个新线程
		// 使用Go标准context，而不是actor.Context
		ctx := context.Background()
		_, err := a.agent.MemoryProvider.CreateThread(ctx, nil)
		if err != nil {
			actorCtx.Respond(&ErrorResponse{
				Error: fmt.Sprintf("Failed to create thread: %v", err),
			})
			return
		}
	}

	a.agent.StateManager.SetRunning(threadID, taskID)
	a.behavior.Become(a.runningBehavior)

	// 异步执行生成任务
	go func() {
		resp, err := a.agent.Generate(msg.Messages, msg.Options)
		if err != nil {
			a.agent.StateManager.SetError(fmt.Sprintf("Generation error: %v", err))
			a.behavior.Become(a.idleBehavior)

			a.rootCtx.Send(actorCtx.Sender(), &ErrorResponse{
				Error: err.Error(),
			})
			return
		}

		// 完成任务
		a.agent.StateManager.CompleteTask(taskID)
		a.agent.StateManager.SetIdle()
		a.behavior.Become(a.idleBehavior)

		// 返回响应
		a.rootCtx.Send(actorCtx.Sender(), &AgentGenerateResponse{
			Text:       resp.Text,
			Object:     resp.Object,
			Messages:   resp.Messages,
			Steps:      resp.Steps,
			ToolCalls:  resp.ToolCalls,
			FinishInfo: resp.FinishInfo,
		})
	}()
}

// handleStreamMessage 处理流式生成消息请求
func (a *ActorAgent) handleStreamMessage(actorCtx actor.Context, msg *AgentStreamMessage) {
	// 更新状态
	taskID := "task_" + uuid.New().String()
	threadID := msg.Options.ThreadID
	if threadID == "" {
		threadID = uuid.New().String()
		// 如果未提供线程ID，创建一个新线程
		ctx := context.Background()
		_, err := a.agent.MemoryProvider.CreateThread(ctx, nil)
		if err != nil {
			actorCtx.Respond(&ErrorResponse{
				Error: fmt.Sprintf("Failed to create thread: %v", err),
			})
			return
		}
	}

	a.agent.StateManager.SetRunning(threadID, taskID)
	a.behavior.Become(a.runningBehavior)

	// 异步执行流式生成任务
	go func() {
		resp, err := a.agent.Stream(msg.Messages, msg.Options)
		if err != nil {
			a.agent.StateManager.SetError(fmt.Sprintf("Stream error: %v", err))
			a.behavior.Become(a.idleBehavior)

			a.rootCtx.Send(actorCtx.Sender(), &ErrorResponse{
				Error: err.Error(),
			})
			return
		}

		// 返回流式响应
		a.rootCtx.Send(actorCtx.Sender(), resp)

		// 等待流完成
		for range resp.FinishChan {
			// 流结束
			break
		}

		// 完成任务
		a.agent.StateManager.CompleteTask(taskID)
		a.agent.StateManager.SetIdle()
		a.behavior.Become(a.idleBehavior)
	}()
}

// handleRunMessage 处理运行消息请求
func (a *ActorAgent) handleRunMessage(actorCtx actor.Context, msg *AgentRunMessage) {
	// 检查选项
	if msg.Options == nil {
		actorCtx.Respond(&ErrorResponse{
			Error: "Options cannot be nil",
		})
		return
	}

	// 检查线程ID
	if msg.Options.ThreadID == "" {
		actorCtx.Respond(&ErrorResponse{
			Error: "ThreadID is required",
		})
		return
	}

	// 更新状态
	taskID := "task_" + uuid.New().String()
	a.agent.StateManager.SetRunning(msg.Options.ThreadID, taskID)
	a.behavior.Become(a.runningBehavior)

	// 异步执行任务
	go func() {
		// 使用正确的context包
		ctx := stdctx.Background()
		result, err := a.agent.Run(ctx, msg.Options)
		if err != nil {
			a.agent.StateManager.SetError(fmt.Sprintf("Run error: %v", err))
			a.behavior.Become(a.idleBehavior)

			a.rootCtx.Send(actorCtx.Sender(), &ErrorResponse{
				Error: err.Error(),
			})
			return
		}

		// 完成任务
		a.agent.StateManager.CompleteTask(taskID)
		a.agent.StateManager.SetIdle()
		a.behavior.Become(a.idleBehavior)

		// 返回响应
		a.rootCtx.Send(actorCtx.Sender(), &RunResponse{
			Result: result,
		})
	}()
}

// handleToolResponseMessage 处理工具响应消息
func (a *ActorAgent) handleToolResponseMessage(context actor.Context, msg *ToolResponseMessage) {
	// 处理工具执行结果
	// 这里应该根据当前正在执行的步骤，将工具执行结果传递给Agent
	log.Printf("Agent %s received tool response for %s: %v",
		a.agent.ID, msg.ToolID, msg.Result)

	// 具体的处理逻辑取决于Agent正在执行的任务
	// 这里只是简单记录
}

// handleStatusMessage 处理状态查询消息
func (a *ActorAgent) handleStatusMessage(context actor.Context) {
	state := a.agent.StateManager.GetState()
	resp := &StatusResponse{
		Status:     string(state.Status),
		AgentID:    state.ID,
		RunCount:   state.RunCount,
		ErrorCount: state.ErrorCount,
		LastUpdate: state.UpdatedAt,
	}

	context.Respond(resp)
}

// handleStopMessage 处理停止消息
func (a *ActorAgent) handleStopMessage(context actor.Context) {
	// 更新状态
	a.agent.StateManager.SetStopped()

	// 响应
	context.Respond(&StatusResponse{
		Status: string(a.agent.StateManager.GetState().Status),
		Info:   "Agent stopped",
	})

	// 停止自己 - 使用正确的方法
	context.Stop(context.Self())
}

// handleNetworkMessage 处理从网络接收的消息
func (a *ActorAgent) handleNetworkMessage(context actor.Context, msg *AgentNetworkMessage) {
	log.Printf("Agent %s received network message from %s: %T",
		a.agent.ID, msg.SourceID, msg.Message)

	// 根据消息类型处理
	switch netMsg := msg.Message.(type) {
	case *GenerateMessage:
		// 将网络消息转换为本地消息处理
		a.handleGenerateMessage(context, &AgentGenerateMessage{
			Messages: netMsg.Messages,
			Options:  netMsg.Options,
		})

	case *StreamMessage:
		// 将网络消息转换为本地消息处理
		a.handleStreamMessage(context, &AgentStreamMessage{
			Messages: netMsg.Messages,
			Options:  netMsg.Options,
		})

	case *RunOptions:
		// 将网络消息转换为本地消息处理
		a.handleRunMessage(context, &AgentRunMessage{
			Options: (*RunOptions)(netMsg),
		})

	case string:
		// 简单文本消息处理
		// 可以将文本消息添加到Agent的内存中
		ctx := stdctx.Background()
		threadID := a.agent.StateManager.GetState().CurrentThreadID
		if threadID == "" {
			// 如果没有当前线程，创建一个新线程
			thread, err := a.agent.MemoryProvider.CreateThread(ctx, nil)
			if err != nil {
				log.Printf("Failed to create thread for network message: %v", err)
				return
			}
			threadID = thread.ID
		}

		// 添加消息到线程
		_, err := a.agent.MemoryProvider.AddMessage(ctx, threadID, "network",
			fmt.Sprintf("Message from agent %s: %s", msg.SourceID, netMsg),
			map[string]interface{}{
				"source_agent": msg.SourceID,
			})

		if err != nil {
			log.Printf("Failed to add network message to memory: %v", err)
		}

	default:
		log.Printf("Unhandled network message type: %T", netMsg)
	}
}

// 定义Agent Actor消息

// AgentGenerateMessage 用于请求生成文本
type AgentGenerateMessage struct {
	Messages []Message        `json:"messages"`
	Options  *GenerateOptions `json:"options,omitempty"`
}

// AgentStreamMessage 用于请求流式生成文本
type AgentStreamMessage struct {
	Messages []Message      `json:"messages"`
	Options  *StreamOptions `json:"options,omitempty"`
}

// AgentRunMessage 用于请求Agent执行任务
type AgentRunMessage struct {
	Options *RunOptions `json:"options"`
}

// AgentRunOptions 执行选项
type AgentRunOptions struct {
	ThreadID            string       `json:"thread_id"`
	Input               string       `json:"input,omitempty"`
	AvailableTools      []tools.Tool `json:"-"`
	MaxConsecutiveCalls int          `json:"max_consecutive_calls,omitempty"`
}

// PauseMessage 用于暂停Agent
type PauseMessage struct{}

// ResumeMessage 用于恢复Agent
type ResumeMessage struct{}

// StatusMessage 用于查询Agent状态
type StatusMessage struct{}

// StopMessage 用于停止Agent
type StopMessage struct{}

// ToolResponseMessage 工具执行结果响应
type ToolResponseMessage struct {
	ToolID     string      `json:"tool_id"`
	ToolCallID string      `json:"tool_call_id"`
	Result     interface{} `json:"result"`
	Error      string      `json:"error,omitempty"`
}

// AgentStartedMessage Agent已启动消息
type AgentStartedMessage struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
}

// AgentStoppedMessage Agent已停止消息
type AgentStoppedMessage struct {
	AgentID string `json:"agent_id"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// StatusResponse 状态响应
type StatusResponse struct {
	Status     string    `json:"status"`
	AgentID    string    `json:"agent_id,omitempty"`
	RunCount   int       `json:"run_count,omitempty"`
	ErrorCount int       `json:"error_count,omitempty"`
	LastUpdate time.Time `json:"last_update,omitempty"`
	Info       string    `json:"info,omitempty"`
}

// AgentGenerateResponse 生成响应
type AgentGenerateResponse struct {
	Text       string      `json:"text,omitempty"`
	Object     interface{} `json:"object,omitempty"`
	Messages   []Message   `json:"messages,omitempty"`
	Steps      []Step      `json:"steps,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	FinishInfo FinishInfo  `json:"finish_info,omitempty"`
}

// RunResponse 运行响应
type RunResponse struct {
	Result string `json:"result"`
}

// ToolActor 表示工具的Actor实现
type ToolActor struct {
	tool      tools.Tool
	parentPID *actor.PID
}

// NewToolActor 创建一个工具Actor
func NewToolActor(tool tools.Tool, parentPID *actor.PID) *ToolActor {
	return &ToolActor{
		tool:      tool,
		parentPID: parentPID,
	}
}

// Receive 工具Actor接收消息
func (t *ToolActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Printf("Tool %s started", t.tool.GetID())

	case *actor.Stopping:
		log.Printf("Tool %s is stopping...", t.tool.GetID())

	case *actor.Stopped:
		log.Printf("Tool %s stopped", t.tool.GetID())

	case *ExecuteToolMessage:
		t.handleExecuteToolMessage(context, msg)

	default:
		log.Printf("Tool %s received unknown message: %T", t.tool.GetID(), msg)
	}
}

// handleExecuteToolMessage 处理工具执行请求
func (t *ToolActor) handleExecuteToolMessage(context actor.Context, msg *ExecuteToolMessage) {
	go func() {
		// 使用正确的context包
		stdCtx := stdctx.Background()
		// 执行工具
		opts := &tools.ExecuteOptions{
			ThreadID: msg.ThreadID,
			CallID:   msg.ToolCallID,
			Context:  stdCtx,
		}
		result, err := t.tool.Execute(msg.Parameters, opts)

		// 构造响应
		response := &ToolResponseMessage{
			ToolID:     t.tool.GetID(),
			ToolCallID: msg.ToolCallID,
			Result:     result,
		}

		if err != nil {
			response.Error = err.Error()
		}

		// 发送响应给父Actor
		context.Send(t.parentPID, response)
	}()
}

// ExecuteToolMessage 工具执行请求
type ExecuteToolMessage struct {
	ToolCallID string                 `json:"tool_call_id"`
	Parameters map[string]interface{} `json:"parameters"`
	ThreadID   string                 `json:"thread_id,omitempty"`
}
