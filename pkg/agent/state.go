package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State 表示Agent的状态
type State struct {
	// 基本信息
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 最近活动
	LastRunAt         *time.Time `json:"last_run_at,omitempty"`
	CurrentThreadID   string     `json:"current_thread_id,omitempty"`
	CurrentTaskID     string     `json:"current_task_id,omitempty"`
	LastCompletedTask string     `json:"last_completed_task,omitempty"`
	RunCount          int        `json:"run_count"`
	ErrorCount        int        `json:"error_count"`

	// 自定义状态数据
	Data map[string]interface{} `json:"data,omitempty"`
}

// Status 表示Agent的状态
type Status string

const (
	// StatusIdle 表示Agent处于空闲状态
	StatusIdle Status = "idle"
	// StatusRunning 表示Agent正在运行
	StatusRunning Status = "running"
	// StatusPaused 表示Agent已暂停
	StatusPaused Status = "paused"
	// StatusError 表示Agent处于错误状态
	StatusError Status = "error"
	// StatusStopped 表示Agent已停止
	StatusStopped Status = "stopped"
)

// StateManager 管理Agent的状态
type StateManager struct {
	mu    sync.RWMutex
	state *State
}

// NewStateManager 创建一个新的状态管理器
func NewStateManager(agentID string) *StateManager {
	now := time.Now()
	return &StateManager{
		state: &State{
			ID:        agentID,
			Status:    StatusIdle,
			CreatedAt: now,
			UpdatedAt: now,
			Data:      make(map[string]interface{}),
		},
	}
}

// GetState 获取当前状态的副本
func (m *StateManager) GetState() *State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 创建状态副本
	stateCopy := *m.state

	// 深拷贝数据
	if m.state.Data != nil {
		stateCopy.Data = make(map[string]interface{})
		for k, v := range m.state.Data {
			stateCopy.Data[k] = v
		}
	}

	return &stateCopy
}

// UpdateStatus 更新状态
func (m *StateManager) UpdateStatus(status Status) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Status = status
	m.state.UpdatedAt = time.Now()
}

// SetRunning 设置为运行状态
func (m *StateManager) SetRunning(threadID, taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.state.Status = StatusRunning
	m.state.LastRunAt = &now
	m.state.CurrentThreadID = threadID
	m.state.CurrentTaskID = taskID
	m.state.UpdatedAt = now
	m.state.RunCount++
}

// SetIdle 设置为空闲状态
func (m *StateManager) SetIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Status = StatusIdle
	m.state.UpdatedAt = time.Now()
	m.state.CurrentThreadID = ""
	m.state.CurrentTaskID = ""
}

// SetError 设置为错误状态
func (m *StateManager) SetError(errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Status = StatusError
	m.state.UpdatedAt = time.Now()
	m.state.ErrorCount++

	// 将错误信息保存在数据中
	m.state.Data["last_error"] = errMsg
	m.state.Data["last_error_time"] = time.Now()
}

// SetStopped 设置为停止状态
func (m *StateManager) SetStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Status = StatusStopped
	m.state.UpdatedAt = time.Now()
	m.state.CurrentThreadID = ""
	m.state.CurrentTaskID = ""
}

// SetPaused 设置为暂停状态
func (m *StateManager) SetPaused() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Status = StatusPaused
	m.state.UpdatedAt = time.Now()
}

// CompleteTask 标记任务完成
func (m *StateManager) CompleteTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.LastCompletedTask = taskID
	m.state.UpdatedAt = time.Now()

	// 如果是当前任务，清除当前任务ID
	if m.state.CurrentTaskID == taskID {
		m.state.CurrentTaskID = ""
	}
}

// SetData 设置自定义数据
func (m *StateManager) SetData(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state.Data == nil {
		m.state.Data = make(map[string]interface{})
	}

	m.state.Data[key] = value
	m.state.UpdatedAt = time.Now()
}

// GetData 获取自定义数据
func (m *StateManager) GetData(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state.Data == nil {
		return nil, false
	}

	value, exists := m.state.Data[key]
	return value, exists
}

// DeleteData 删除自定义数据
func (m *StateManager) DeleteData(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state.Data == nil {
		return
	}

	delete(m.state.Data, key)
	m.state.UpdatedAt = time.Now()
}

// SaveState 保存状态到JSON字符串
func (m *StateManager) SaveState() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.Marshal(m.state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	return string(data), nil
}

// LoadState 从JSON字符串加载状态
func (m *StateManager) LoadState(stateJSON string) error {
	if stateJSON == "" {
		return errors.New("state JSON cannot be empty")
	}

	var state State
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 保留原始ID，只加载其他字段
	agentID := m.state.ID
	*m.state = state
	m.state.ID = agentID
	m.state.UpdatedAt = time.Now()

	return nil
}

// IsRunning 判断Agent是否在运行
func (m *StateManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state.Status == StatusRunning
}

// IsStopped 判断Agent是否已停止
func (m *StateManager) IsStopped() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state.Status == StatusStopped
}

// IsIdle 判断Agent是否空闲
func (m *StateManager) IsIdle() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state.Status == StatusIdle
}

// HasError 判断Agent是否处于错误状态
func (m *StateManager) HasError() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state.Status == StatusError
}

// IsPaused 判断Agent是否已暂停
func (m *StateManager) IsPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state.Status == StatusPaused
}
