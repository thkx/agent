package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// --- 基础类型定义 ---

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	ToolID  string `json:"tool_id,omitempty"`
}

// State 状态驱动核心
type State struct {
	ExecutionID string                 `json:"execution_id"`
	Messages    []Message              `json:"messages"`
	Context     map[string]interface{} `json:"context"`
	NextNode    string                 `json:"next_node"`
}

// --- 内存 Checkpointer 实现 (模拟持久化) ---

type MemoryCheckpointer struct {
	mu    sync.RWMutex
	Store map[string][]byte
}

func (m *MemoryCheckpointer) Save(state *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, _ := json.Marshal(state)
	m.Store[state.ExecutionID] = data
	fmt.Printf("[Checkpoint] Saved state for %s at node: %s\n", state.ExecutionID, state.NextNode)
	return nil
}

func (m *MemoryCheckpointer) Load(id string) (*State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if data, ok := m.Store[id]; ok {
		var s State
		json.Unmarshal(data, &s)
		return &s, nil
	}
	return nil, errors.New("not found")
}

// --- Tool 抽象接口 ---
type Tool interface {
	Name() string
	Execute(ctx context.Context, input string) (string, error)
}

// Executor: 执行手脚
type Executor struct {
	tools map[string]Tool
}

func (n *Executor) Name() string { return "executor" }
func (n *Executor) Execute(ctx context.Context, s *State) (*State, error) {
	lastMsg := s.Messages[len(s.Messages)-1].Content
	parts := strings.Split(lastMsg, ":")
	if len(parts) < 3 {
		return nil, fmt.Errorf("指令格式错误")
	}

	toolName, input := parts[1], parts[2]
	tool, ok := n.tools[toolName]
	if !ok {
		return nil, fmt.Errorf("工具 %s 未注册", toolName)
	}

	fmt.Printf("🛠️ [%s] 正在调用工具: %s(%s)\n", n.Name(), toolName, input)
	result, err := tool.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	s.Messages = append(s.Messages, Message{Role: RoleTool, Content: result})
	s.NextNode = "planner" // 形成 ReAct 循环
	return s, nil
}

// --- 核心引擎实现 ---

type AgentOS struct {
	nodes      map[string]Node
	checkpoint Checkpointer
}

func New(cp Checkpointer) *AgentOS {
	return &AgentOS{
		nodes:      make(map[string]Node),
		checkpoint: cp,
	}
}

func (os *AgentOS) RegisterNode(n Node) {
	os.nodes[n.Name()] = n
}

// Run 核心 Loop
func (os *AgentOS) Run(ctx context.Context, state *State) (*State, error) {
	for {
		// 1. 检查是否结束
		if state.NextNode == "END" || state.NextNode == "" {
			break
		}

		// 2. 状态持久化 (可恢复执行的关键)
		os.checkpoint.Save(state)

		// 3. 节点分发与执行
		node, ok := os.nodes[state.NextNode]
		if !ok {
			return nil, fmt.Errorf("node %s not found", state.NextNode)
		}

		fmt.Printf(">>> Executing Node: %s\n", node.Name())
		var err error
		state, err = node.Execute(ctx, state)
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}

// --- 接口抽象 ---

type Node interface {
	Name() string
	Execute(ctx context.Context, state *State) (*State, error)
}

type Checkpointer interface {
	Save(state *State) error
	Load(executionID string) (*State, error)
}

// --- 具体节点实现 (模拟 LLM & Tool) ---

type LLMNode struct{}

func (n *LLMNode) Name() string { return "llm_think" }
func (n *LLMNode) Execute(ctx context.Context, s *State) (*State, error) {
	// 模拟 LLM 决策过程
	s.Messages = append(s.Messages, Message{Role: RoleAssistant, Content: "I need to check the weather."})
	s.NextNode = "tool_weather" // 状态跳转
	return s, nil
}

type WeatherToolNode struct{}

func (n *WeatherToolNode) Name() string { return "tool_weather" }
func (n *WeatherToolNode) Execute(ctx context.Context, s *State) (*State, error) {
	// 模拟工具执行
	fmt.Println("   [Tool] Calling Weather API...")
	time.Sleep(500 * time.Millisecond)
	s.Messages = append(s.Messages, Message{Role: RoleTool, Content: "Sunny, 25°C", ToolID: "weather_01"})
	s.NextNode = "END" // 执行完成
	return s, nil
}

// --- 运行示例 ---

func main() {
	// 初始化组件
	cp := &MemoryCheckpointer{Store: make(map[string][]byte)}
	engine := New(cp)

	// 注册节点
	engine.RegisterNode(&LLMNode{})
	engine.RegisterNode(&WeatherToolNode{})

	// 初始化状态
	initialState := &State{
		ExecutionID: "exec_001",
		Messages: []Message{
			{Role: RoleUser, Content: "What's the weather?"},
		},
		NextNode: "llm_think",
		Context:  make(map[string]interface{}),
	}

	// 启动 Agent Loop
	finalState, err := engine.Run(context.Background(), initialState)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Println("\n--- Final Conversation History ---")
	for _, msg := range finalState.Messages {
		fmt.Printf("[%s]: %s\n", msg.Role, msg.Content)
	}
}
