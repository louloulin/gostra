package workflow

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryStateStore 是一个简单的内存中工作流状态存储实现
type MemoryStateStore struct {
	states map[string]*WorkflowState
	mu     sync.RWMutex
}

// NewMemoryStateStore 创建一个新的内存状态存储
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		states: make(map[string]*WorkflowState),
	}
}

// SaveWorkflowState 保存工作流状态到内存
func (s *MemoryStateStore) SaveWorkflowState(state *WorkflowState) error {
	if state == nil {
		return fmt.Errorf("cannot save nil state")
	}
	if state.WorkflowID == "" {
		return fmt.Errorf("workflow ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建状态副本以避免外部修改
	stateCopy := *state
	s.states[state.WorkflowID] = &stateCopy

	return nil
}

// LoadWorkflowState 从内存加载工作流状态
func (s *MemoryStateStore) LoadWorkflowState(workflowID string) (*WorkflowState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.states[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow state not found for ID: %s", workflowID)
	}

	// 返回状态副本以避免外部修改
	stateCopy := *state
	return &stateCopy, nil
}

// ListWorkflowStates 列出所有工作流状态
func (s *MemoryStateStore) ListWorkflowStates() ([]*WorkflowState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	states := make([]*WorkflowState, 0, len(s.states))
	for _, state := range s.states {
		stateCopy := *state
		states = append(states, &stateCopy)
	}

	return states, nil
}

// DeleteWorkflowState 从内存中删除工作流状态
func (s *MemoryStateStore) DeleteWorkflowState(workflowID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.states[workflowID]; !exists {
		return fmt.Errorf("workflow state not found for ID: %s", workflowID)
	}

	delete(s.states, workflowID)
	return nil
}

// FileStateStore 是一个基于文件的工作流状态存储实现
type FileStateStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStateStore 创建一个新的基于文件的状态存储
func NewFileStateStore(baseDir string) (*FileStateStore, error) {
	// 确保目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", baseDir, err)
	}

	return &FileStateStore{
		baseDir: baseDir,
	}, nil
}

// 获取状态文件路径
func (s *FileStateStore) getFilePath(workflowID string) string {
	return filepath.Join(s.baseDir, fmt.Sprintf("%s.json", workflowID))
}

// SaveWorkflowState 保存工作流状态到文件
func (s *FileStateStore) SaveWorkflowState(state *WorkflowState) error {
	if state == nil {
		return fmt.Errorf("cannot save nil state")
	}
	if state.WorkflowID == "" {
		return fmt.Errorf("workflow ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新时间戳
	state.LastUpdated = time.Now()

	// 序列化状态为JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workflow state: %w", err)
	}

	// 写入文件
	filePath := s.getFilePath(state.WorkflowID)
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workflow state to file: %w", err)
	}

	return nil
}

// LoadWorkflowState 从文件加载工作流状态
func (s *FileStateStore) LoadWorkflowState(workflowID string) (*WorkflowState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := s.getFilePath(workflowID)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("workflow state file not found for ID: %s", workflowID)
	}

	// 读取文件
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow state file: %w", err)
	}

	// 反序列化JSON
	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow state: %w", err)
	}

	return &state, nil
}

// ListWorkflowStates 列出所有工作流状态
func (s *FileStateStore) ListWorkflowStates() ([]*WorkflowState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 获取所有JSON文件
	files, err := filepath.Glob(filepath.Join(s.baseDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow state files: %w", err)
	}

	states := make([]*WorkflowState, 0, len(files))
	for _, file := range files {
		// 读取文件
		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue // 跳过不能读取的文件
		}

		// 反序列化JSON
		var state WorkflowState
		if err := json.Unmarshal(data, &state); err != nil {
			continue // 跳过格式不正确的文件
		}

		states = append(states, &state)
	}

	return states, nil
}

// DeleteWorkflowState 删除工作流状态文件
func (s *FileStateStore) DeleteWorkflowState(workflowID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.getFilePath(workflowID)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("workflow state file not found for ID: %s", workflowID)
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete workflow state file: %w", err)
	}

	return nil
}
