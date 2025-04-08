package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
	"github.com/yourusername/gostra/pkg/errors"
	"github.com/yourusername/gostra/pkg/models"
)

// NetworkMessage represents a message in the agent network
type NetworkMessage struct {
	From    string      `json:"from"`
	To      string      `json:"to"`
	Content string      `json:"content"`
	Data    interface{} `json:"data,omitempty"`
}

// AgentNetwork manages a network of collaborative agents
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
	router *RouterAgent

	// Supervisor for error handling
	supervisor *errors.Supervisor

	// Model provider for LLM operations
	model models.ModelProvider
}

// AgentNetworkOptions 创建网络的选项
type AgentNetworkOptions struct {
	ID          string
	Name        string
	Description string
	AgentIDs    []string
	ActorSystem *actor.ActorSystem
}

// NewAgentNetwork 创建一个新的Agent网络
func NewAgentNetwork(opts *AgentNetworkOptions, model models.ModelProvider) (*AgentNetwork, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
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
		return nil, errors.New("actor system is required")
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
		supervisor:  errors.NewSupervisor(actorSystem, nil),
	}

	// 添加初始Agent
	for _, agentID := range opts.AgentIDs {
		network.AgentIDs = append(network.AgentIDs, agentID)
	}

	// Create router agent
	routerProps := actor.PropsFromProducer(func() actor.Actor {
		return NewRouterAgent(network)
	})
	routerPID, err := actorSystem.Root.SpawnNamed(routerProps, "router")
	if err != nil {
		return nil, fmt.Errorf("failed to create router agent: %w", err)
	}

	network.router = routerPID.Interface().(*RouterAgent)

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

	// 查找Agent
	var found bool
	var index int
	for i, id := range n.AgentIDs {
		if id == agentID {
			found = true
			index = i
			break
		}
	}

	if !found {
		return fmt.Errorf("agent %s not in network", agentID)
	}

	// 从网络中移除
	n.AgentIDs = append(n.AgentIDs[:index], n.AgentIDs[index+1:]...)
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

	pid, exists := n.agentActors[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %s not found in network", agentID)
	}

	return pid, nil
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

// RegisterAgent adds an agent to the network
func (n *AgentNetwork) RegisterAgent(name string, agent Actor) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	props := actor.PropsFromProducer(func() actor.Actor {
		return NewActorAgent(agent, n.model)
	})

	pid, err := n.system.Root.SpawnNamed(props, name)
	if err != nil {
		return err
	}

	n.agents[name] = pid
	return nil
}

// Transmit sends a message through the network
func (n *AgentNetwork) Transmit(ctx context.Context, msg *NetworkMessage) error {
	n.mu.RLock()
	targetPID, exists := n.agents[msg.To]
	n.mu.RUnlock()

	if !exists {
		return errors.NewAgentError(
			errors.ActorError,
			errors.Error,
			"Target agent not found",
			nil,
			true,
		)
	}

	future := n.system.Root.RequestFuture(targetPID, msg, timeout)
	result, err := future.Result()
	if err != nil {
		return n.supervisor.HandleFailure(ctx, targetPID, msg.To, err)
	}

	return result.(error)
}

// BroadcastMessage sends a message to multiple agents in parallel
func (n *AgentNetwork) BroadcastMessage(ctx context.Context, msg *NetworkMessage, targets []string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(targets))

	for _, target := range targets {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			msg.To = target
			if err := n.Transmit(ctx, msg); err != nil {
				errChan <- err
			}
		}(target)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.NewAgentError(
			errors.SystemError,
			errors.Error,
			"Broadcast failed for some targets",
			errs[0],
			true,
		)
	}

	return nil
}

// GetAgent retrieves an agent from the network
func (n *AgentNetwork) GetAgent(name string) (Actor, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	pid, exists := n.agents[name]
	if !exists {
		return nil, false
	}

	return pid.Interface().(Actor), true
}

// RemoveAgent removes an agent from the network
func (n *AgentNetwork) RemoveAgent(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if pid, exists := n.agents[name]; exists {
		n.supervisor.RemoveActor(pid)
		pid.Stop()
		delete(n.agents, name)
	}
}
