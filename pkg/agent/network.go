package agent

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
	pkgerrors "github.com/louloulin/gostra/pkg/errors"
	"github.com/louloulin/gostra/pkg/models"
)

// NetworkMessage represents a message in the agent network
type NetworkMessage struct {
	From    string      `json:"from"`
	To      string      `json:"to"`
	Content string      `json:"content"`
	Data    interface{} `json:"data,omitempty"`
}

// AgentNetwork represents a network of agents that can communicate with each other
type AgentNetwork struct {
	ID          string
	Name        string
	Description string
	AgentIDs    []string
	Supervisor  *actor.PID
	topology    map[string][]string // 记录agent之间的连接关系
	agentActors map[string]*actor.PID
	actorSystem *actor.ActorSystem
	rootContext *actor.RootContext
	mu          sync.RWMutex

	// Actor system reference
	system *actor.ActorSystem

	// Map of agent names to their PIDs
	agents map[string]*actor.PID

	// Router agent for dynamic routing
	routerPID *actor.PID

	// Supervisor for error handling
	supervisor *pkgerrors.Supervisor

	// Model provider for LLM operations
	model models.ModelProvider

	// New fields for context handling
	contextHandler *ContextHandler
}

// AgentNetworkOptions 创建网络的选项
type AgentNetworkOptions struct {
	ID            string
	Name          string
	Description   string
	AgentIDs      []string
	ActorSystem   *actor.ActorSystem
	RouterOptions *RouterOptions // Options for configuring the router agent
}

// NewAgentNetwork 创建一个新的Agent网络
func NewAgentNetwork(opts *AgentNetworkOptions, model models.ModelProvider) (*AgentNetwork, error) {
	if opts == nil {
		return nil, stderrors.New("options cannot be nil")
	}

	id := opts.ID
	if id == "" {
		id = uuid.New().String()
	}

	name := opts.Name
	if name == "" {
		name = "Network-" + id[:8]
	}

	actorSystem := opts.ActorSystem
	if actorSystem == nil {
		return nil, stderrors.New("actor system is required")
	}

	network := &AgentNetwork{
		ID:          id,
		Name:        name,
		Description: opts.Description,
		AgentIDs:    make([]string, 0),
		topology:    make(map[string][]string),
		agentActors: make(map[string]*actor.PID),
		actorSystem: actorSystem,
		rootContext: actor.NewRootContext(actorSystem, nil),
		agents:      make(map[string]*actor.PID),
		model:       model,
		supervisor:  pkgerrors.NewSupervisor(actorSystem, nil),
	}

	// 添加初始Agent
	for _, agentID := range opts.AgentIDs {
		network.AgentIDs = append(network.AgentIDs, agentID)
	}

	// Create router agent with the provided options
	routerProps := actor.PropsFromProducer(func() actor.Actor {
		return NewRouterAgent(network, opts.RouterOptions)
	})

	// Use the network ID to create a unique router name
	routerName := "router-" + id
	routerPID, err := actorSystem.Root.SpawnNamed(routerProps, routerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create router agent: %w", err)
	}

	// Store the PID rather than trying to get the actual RouterAgent
	network.routerPID = routerPID

	// Create and set up the context handler
	network.contextHandler = NewContextHandler(network)

	return network, nil
}

// Start 启动网络
func (n *AgentNetwork) Start() error {
	log.Printf("Starting agent network: %s", n.Name)

	// 创建网络监督者Actor
	supervisorProps := actor.PropsFromProducer(func() actor.Actor {
		return NewNetworkSupervisor(n)
	})

	pid, err := n.rootContext.SpawnNamed(supervisorProps, "network-supervisor-"+n.ID)
	if err != nil {
		return fmt.Errorf("failed to create network supervisor: %w", err)
	}

	n.Supervisor = pid

	// 查找并注册网络中的所有Agent
	for _, agentID := range n.AgentIDs {
		agentPID := actor.NewPID(n.actorSystem.Address(), "agent-"+agentID)

		// 验证Agent是否存在
		_, err := n.rootContext.RequestFuture(agentPID, &StatusMessage{}, 5*time.Second).Result()
		if err != nil {
			log.Printf("Warning: agent %s not found, will be added when registered", agentID)
			continue
		}

		n.mu.Lock()
		n.agentActors[agentID] = agentPID
		n.mu.Unlock()
	}

	return nil
}

// Stop 停止网络
func (n *AgentNetwork) Stop() error {
	log.Printf("Stopping agent network: %s", n.Name)

	if n.Supervisor != nil {
		n.rootContext.Stop(n.Supervisor)
	}

	// Stop the context handler
	if n.contextHandler != nil {
		n.contextHandler.Stop()
	}

	return nil
}

// AddAgent 向网络中添加Agent
func (n *AgentNetwork) AddAgent(agentID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 检查Agent是否已在网络中
	for _, id := range n.AgentIDs {
		if id == agentID {
			return fmt.Errorf("agent %s already in network", agentID)
		}
	}

	// 获取Agent的PID
	agentPID := actor.NewPID(n.actorSystem.Address(), "agent-"+agentID)

	// 验证Agent是否存在
	_, err := n.rootContext.RequestFuture(agentPID, &StatusMessage{}, 5*time.Second).Result()
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	// 添加到网络
	n.AgentIDs = append(n.AgentIDs, agentID)
	n.agentActors[agentID] = agentPID

	// 通知网络监督者
	if n.Supervisor != nil {
		n.rootContext.Send(n.Supervisor, &AgentAddedMessage{
			AgentID: agentID,
			Network: n.ID,
		})
	}

	return nil
}

// RemoveAgent 从网络中移除Agent
func (n *AgentNetwork) RemoveAgent(agentID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Check if the agent exists in either map
	_, existsInAgents := n.agents[agentID]
	_, existsInActors := n.agentActors[agentID]

	if !existsInAgents && !existsInActors {
		return fmt.Errorf("agent %s not found in network", agentID)
	}

	// 查找Agent in AgentIDs
	var found bool
	var index int
	for i, id := range n.AgentIDs {
		if id == agentID {
			found = true
			index = i
			break
		}
	}

	if found {
		// 从网络中移除
		n.AgentIDs = append(n.AgentIDs[:index], n.AgentIDs[index+1:]...)
	}

	// 从两个maps中移除
	delete(n.agents, agentID)
	delete(n.agentActors, agentID)
	delete(n.topology, agentID)

	// 从其他Agent的连接中移除
	for _, connections := range n.topology {
		for i, connID := range connections {
			if connID == agentID {
				connections = append(connections[:i], connections[i+1:]...)
				break
			}
		}
	}

	// 通知网络监督者
	if n.Supervisor != nil {
		n.rootContext.Send(n.Supervisor, &AgentRemovedMessage{
			AgentID: agentID,
			Network: n.ID,
		})
	}

	return nil
}

// ConnectAgents 在两个Agent之间建立连接
func (n *AgentNetwork) ConnectAgents(sourceID, targetID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 检查两个Agent是否都在网络中
	var sourceFound, targetFound bool
	for _, id := range n.AgentIDs {
		if id == sourceID {
			sourceFound = true
		}
		if id == targetID {
			targetFound = true
		}
	}

	if !sourceFound {
		return fmt.Errorf("source agent %s not in network", sourceID)
	}
	if !targetFound {
		return fmt.Errorf("target agent %s not in network", targetID)
	}

	// 检查连接是否已存在
	connections, exists := n.topology[sourceID]
	if exists {
		for _, connID := range connections {
			if connID == targetID {
				return fmt.Errorf("connection from %s to %s already exists", sourceID, targetID)
			}
		}
		// 添加新连接
		n.topology[sourceID] = append(connections, targetID)
	} else {
		// 创建新的连接列表
		n.topology[sourceID] = []string{targetID}
	}

	// 通知网络监督者
	if n.Supervisor != nil {
		n.rootContext.Send(n.Supervisor, &AgentsConnectedMessage{
			SourceID: sourceID,
			TargetID: targetID,
			Network:  n.ID,
		})
	}

	return nil
}

// DisconnectAgents 移除两个Agent之间的连接
func (n *AgentNetwork) DisconnectAgents(sourceID, targetID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 检查连接是否存在
	connections, exists := n.topology[sourceID]
	if !exists {
		return fmt.Errorf("no connections from agent %s", sourceID)
	}

	var found bool
	var index int
	for i, connID := range connections {
		if connID == targetID {
			found = true
			index = i
			break
		}
	}

	if !found {
		return fmt.Errorf("no connection from %s to %s", sourceID, targetID)
	}

	// 移除连接
	n.topology[sourceID] = append(connections[:index], connections[index+1:]...)

	// 通知网络监督者
	if n.Supervisor != nil {
		n.rootContext.Send(n.Supervisor, &AgentsDisconnectedMessage{
			SourceID: sourceID,
			TargetID: targetID,
			Network:  n.ID,
		})
	}

	return nil
}

// GetConnections 获取Agent的所有连接
func (n *AgentNetwork) GetConnections(agentID string) ([]string, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	connections, exists := n.topology[agentID]
	if !exists {
		return []string{}, nil // 返回空列表，表示没有连接
	}

	// 返回连接的副本
	result := make([]string, len(connections))
	copy(result, connections)

	return result, nil
}

// SendMessage 从一个Agent向另一个Agent发送消息
func (n *AgentNetwork) SendMessage(sourceID, targetID string, message interface{}) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// 检查两个Agent是否都在网络中
	_, sourceExists := n.agentActors[sourceID]
	if !sourceExists {
		return fmt.Errorf("source agent %s not found in network", sourceID)
	}

	targetPID, targetExists := n.agentActors[targetID]
	if !targetExists {
		return fmt.Errorf("target agent %s not found in network", targetID)
	}

	// 检查是否有从源到目标的连接
	connections, exists := n.topology[sourceID]
	if !exists {
		return fmt.Errorf("no connections from agent %s", sourceID)
	}

	var found bool
	for _, connID := range connections {
		if connID == targetID {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("no connection from %s to %s", sourceID, targetID)
	}

	// 将消息包装成AgentNetworkMessage
	networkMsg := &AgentNetworkMessage{
		SourceID: sourceID,
		TargetID: targetID,
		Message:  message,
	}

	// 发送消息
	n.rootContext.Send(targetPID, networkMsg)

	return nil
}

// BroadcastMessage 向网络中的所有连接的Agent广播消息
func (n *AgentNetwork) BroadcastMessage(sourceID string, message interface{}) error {
	n.mu.RLock()
	connections, exists := n.topology[sourceID]
	if !exists || len(connections) == 0 {
		n.mu.RUnlock()
		return fmt.Errorf("agent %s has no connections", sourceID)
	}

	// 获取源Agent的PID
	_, sourceExists := n.agentActors[sourceID]
	if !sourceExists {
		n.mu.RUnlock()
		return fmt.Errorf("source agent %s not found in network", sourceID)
	}

	// 创建目标Agent的PID和ID映射
	targets := make(map[string]*actor.PID)
	for _, targetID := range connections {
		if targetPID, ok := n.agentActors[targetID]; ok {
			targets[targetID] = targetPID
		}
	}
	n.mu.RUnlock()

	// 发送消息给所有连接的Agent
	for targetID, targetPID := range targets {
		networkMsg := &AgentNetworkMessage{
			SourceID: sourceID,
			TargetID: targetID,
			Message:  message,
		}
		n.rootContext.Send(targetPID, networkMsg)
	}

	return nil
}

// GetAgent 获取网络中Agent的PID
func (n *AgentNetwork) GetAgent(agentID string) (*actor.PID, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// First check the agents map
	pid, exists := n.agents[agentID]
	if exists {
		return pid, nil
	}

	// If not found, check the agentActors map
	pid, exists = n.agentActors[agentID]
	if exists {
		// For consistency, add it to the agents map too
		n.mu.RUnlock()
		n.mu.Lock()
		n.agents[agentID] = pid
		n.mu.Unlock()
		n.mu.RLock()
		return pid, nil
	}

	return nil, fmt.Errorf("agent %s not found in network", agentID)
}

// GetAllAgents 获取网络中所有Agent的ID
func (n *AgentNetwork) GetAllAgents() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	result := make([]string, len(n.AgentIDs))
	copy(result, n.AgentIDs)

	return result
}

// GetTopology 获取网络拓扑
func (n *AgentNetwork) GetTopology() map[string][]string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// 创建拓扑的副本
	result := make(map[string][]string)
	for agentID, connections := range n.topology {
		connCopy := make([]string, len(connections))
		copy(connCopy, connections)
		result[agentID] = connCopy
	}

	return result
}

// NetworkSupervisor 网络监督Actor
type NetworkSupervisor struct {
	network *AgentNetwork
}

// NewNetworkSupervisor 创建网络监督Actor
func NewNetworkSupervisor(network *AgentNetwork) *NetworkSupervisor {
	return &NetworkSupervisor{
		network: network,
	}
}

// Receive 处理接收到的消息
func (s *NetworkSupervisor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Printf("Network Supervisor for %s started", s.network.Name)

	case *actor.Stopping:
		log.Printf("Network Supervisor for %s is stopping...", s.network.Name)

	case *actor.Stopped:
		log.Printf("Network Supervisor for %s stopped", s.network.Name)

	case *AgentAddedMessage:
		log.Printf("Agent %s added to network %s", msg.AgentID, msg.Network)

	case *AgentRemovedMessage:
		log.Printf("Agent %s removed from network %s", msg.AgentID, msg.Network)

	case *AgentsConnectedMessage:
		log.Printf("Connection established from %s to %s in network %s",
			msg.SourceID, msg.TargetID, msg.Network)

	case *AgentsDisconnectedMessage:
		log.Printf("Connection removed from %s to %s in network %s",
			msg.SourceID, msg.TargetID, msg.Network)

	case *NetworkStatusRequest:
		// 返回网络状态
		context.Respond(&NetworkStatusResponse{
			NetworkID:   s.network.ID,
			NetworkName: s.network.Name,
			AgentCount:  len(s.network.AgentIDs),
			Agents:      s.network.GetAllAgents(),
			Topology:    s.network.GetTopology(),
		})

	default:
		log.Printf("Network Supervisor received unknown message: %T", msg)
	}
}

// 网络消息定义

// AgentNetworkMessage 是Agent之间通信的消息
type AgentNetworkMessage struct {
	SourceID string      `json:"source_id"`
	TargetID string      `json:"target_id"`
	Message  interface{} `json:"message"`
}

// AgentAddedMessage 通知Agent被添加到网络
type AgentAddedMessage struct {
	AgentID string `json:"agent_id"`
	Network string `json:"network"`
}

// AgentRemovedMessage 通知Agent被从网络移除
type AgentRemovedMessage struct {
	AgentID string `json:"agent_id"`
	Network string `json:"network"`
}

// AgentsConnectedMessage 通知两个Agent已连接
type AgentsConnectedMessage struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Network  string `json:"network"`
}

// AgentsDisconnectedMessage 通知两个Agent已断开连接
type AgentsDisconnectedMessage struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Network  string `json:"network"`
}

// NetworkStatusRequest 请求网络状态
type NetworkStatusRequest struct{}

// NetworkStatusResponse 网络状态响应
type NetworkStatusResponse struct {
	NetworkID   string              `json:"network_id"`
	NetworkName string              `json:"network_name"`
	AgentCount  int                 `json:"agent_count"`
	Agents      []string            `json:"agents"`
	Topology    map[string][]string `json:"topology"`
}

// Transmit sends a message through the network
func (n *AgentNetwork) Transmit(ctx context.Context, msg *NetworkMessage) error {
	// Check if the message has a specific target
	if msg.To == "" {
		// Send to router for dynamic routing
		if n.routerPID != nil {
			timeout := 30 * time.Second // Default timeout

			// Use actorSystem instead of system which might be nil
			if n.actorSystem == nil {
				return fmt.Errorf("actor system is nil")
			}

			future := n.actorSystem.Root.RequestFuture(n.routerPID, msg, timeout)
			_, err := future.Result()
			if err != nil {
				return fmt.Errorf("router error: %w", err)
			}
			return nil
		}
		return fmt.Errorf("no target specified and no router available")
	}

	// Direct message to a specific agent
	n.mu.RLock()
	targetPID, exists := n.agents[msg.To]
	n.mu.RUnlock()

	if !exists {
		return fmt.Errorf("target agent '%s' not found", msg.To)
	}

	// Use default timeout for direct communication
	timeout := 30 * time.Second

	// Use actorSystem instead of system which might be nil
	if n.actorSystem == nil {
		return fmt.Errorf("actor system is nil")
	}

	future := n.actorSystem.Root.RequestFuture(targetPID, msg, timeout)
	_, err := future.Result()
	return err
}

// AddAgentPID adds an agent PID directly to the network
// This is a simplified version for testing and examples
func (n *AgentNetwork) AddAgentPID(name string, pid *actor.PID) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Add to both maps to ensure consistency
	n.agents[name] = pid
	n.agentActors[name] = pid

	// Only add to AgentIDs if it's not already there
	for _, id := range n.AgentIDs {
		if id == name {
			return
		}
	}
	n.AgentIDs = append(n.AgentIDs, name)
}

// RegisterAgent registers an actor as an agent in the network
func (n *AgentNetwork) RegisterAgent(name string, agent actor.Actor) error {
	if n.actorSystem == nil {
		return fmt.Errorf("actor system is nil")
	}

	props := actor.PropsFromProducer(func() actor.Actor {
		return agent
	})

	pid, err := n.actorSystem.Root.SpawnNamed(props, "agent-"+name)
	if err != nil {
		return fmt.Errorf("failed to spawn agent actor: %w", err)
	}

	n.AddAgentPID(name, pid)
	return nil
}

// BroadcastMessage sends a message to multiple specified agents
func (n *AgentNetwork) BroadcastToTargets(ctx context.Context, msg *NetworkMessage, targets []string) error {
	var lastErr error

	// Send the message to each target
	for _, target := range targets {
		// Create a copy of the message with the specific target
		targetMsg := &NetworkMessage{
			From:    msg.From,
			To:      target,
			Content: msg.Content,
			Data:    msg.Data,
		}

		// Send the message to this target
		err := n.Transmit(ctx, targetMsg)
		if err != nil {
			lastErr = err
			// Continue trying other targets even if one fails
		}
	}

	return lastErr
}

// SendRequest sends a request to the agent network through the router
func (n *AgentNetwork) SendRequest(ctx context.Context, req *TransmitRequest) (*TransmitResponse, error) {
	if n.routerPID == nil {
		return nil, fmt.Errorf("router not initialized")
	}

	// Use the context timeout if available, otherwise use default timeout
	timeout := n.getTimeout(ctx)

	// Apply context enrichment if needed
	if n.contextHandler != nil && req.Context != nil {
		// If there's a conversation ID in the context, use it for state tracking
		conversationID, hasConvID := req.Context["conversation_id"].(string)
		if !hasConvID {
			// Generate a new conversation ID if not present
			conversationID = generateConversationID()
			req.Context["conversation_id"] = conversationID
		}

		// Enrich the request with stored context data for this conversation
		storedContext := n.contextHandler.GetAllContext(conversationID)
		for k, v := range storedContext {
			// Don't overwrite existing context values from the request
			if _, exists := req.Context[k]; !exists {
				req.Context[k] = v
			}
		}

		// Add trace information
		trace, hasTrace := req.Context["conversation_trace"].([]string)
		if !hasTrace {
			trace = []string{}
		}
		traceEntry := fmt.Sprintf("[%s] Request: %s", time.Now().Format(time.RFC3339), req.Message)
		req.Context["conversation_trace"] = append(trace, traceEntry)
	}

	// Send the request to the router
	future := n.actorSystem.Root.RequestFuture(n.routerPID, req, timeout)
	result, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("router error: %w", err)
	}

	// Check if the result is the expected type
	if resp, ok := result.(*TransmitResponse); ok {
		// Process response context if we have a context handler
		if n.contextHandler != nil && resp.Context != nil {
			// Extract conversation ID from the context
			conversationID, ok := resp.Context["conversation_id"].(string)
			if !ok {
				// If no conversation ID, generate one
				conversationID = generateConversationID()
				resp.Context["conversation_id"] = conversationID
			}

			// Store the context data
			n.contextHandler.MergeContext(conversationID, resp.Context, "router")

			// Update trace information
			trace, hasTrace := resp.Context["conversation_trace"].([]string)
			if !hasTrace {
				trace = []string{}
			}
			traceEntry := fmt.Sprintf("[%s] Response: %d agent results",
				time.Now().Format(time.RFC3339),
				len(resp.Results))
			resp.Context["conversation_trace"] = append(trace, traceEntry)

			// Track which agents contributed to this context
			agentContributions, hasContributions := resp.Context["agent_contributions"].(map[string]interface{})
			if !hasContributions {
				agentContributions = make(map[string]interface{})
			}

			for _, result := range resp.Results {
				agentContributions[result.Agent] = time.Now().Format(time.RFC3339)
			}
			resp.Context["agent_contributions"] = agentContributions
		}
		return resp, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", result)
}

// getTimeout extracts timeout from context or returns default timeout
func (n *AgentNetwork) getTimeout(ctx context.Context) time.Duration {
	// Try to get timeout from context
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout > 0 {
			return timeout
		}
	}

	// Use default timeout if not found in context
	return 30 * time.Second
}

// generateConversationID generates a unique conversation ID
func generateConversationID() string {
	return fmt.Sprintf("conv-%d", time.Now().UnixNano())
}

// GetContextHandler returns the context handler for the network
func (n *AgentNetwork) GetContextHandler() *ContextHandler {
	return n.contextHandler
}
