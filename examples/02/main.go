package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// --- 1. 基础协议定义 ---

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
}

// State 核心状态对象
type State struct {
	ExecutionID string    `json:"execution_id"`
	Messages    []Message `json:"messages"`
	NextNode    string    `json:"next_node"`
	Version     int       `json:"version"`
}

// --- 2. 抽象接口：LLM、Tool 与 持久化 ---

type LLM interface {
	Generate(ctx context.Context, messages []Message) (Message, error)
}

type Tool interface {
	Name() string
	Execute(ctx context.Context, input string) (string, error)
}

// Checkpointer 实现可恢复执行的关键
type Checkpointer interface {
	Save(ctx context.Context, state *State) error
	LoadLatest(ctx context.Context, executionID string) (*State, error)
}

// --- 3. 节点定义 ---

type Node interface {
	Name() string
	Execute(ctx context.Context, state *State) (*State, error)
}

// PlannerNode: 决策大脑
type PlannerNode struct {
	llm LLM
}

func (n *PlannerNode) Name() string { return "planner" }
func (n *PlannerNode) Execute(ctx context.Context, s *State) (*State, error) {
	fmt.Printf("🤖 [%s] 正在思考...\n", n.Name())
	resp, err := n.llm.Generate(ctx, s.Messages)
	if err != nil {
		return nil, err
	}
	s.Messages = append(s.Messages, resp)

	// 协议约定：CALL:tool_name:input
	if strings.HasPrefix(resp.Content, "CALL:") {
		s.NextNode = "executor"
	} else {
		s.NextNode = "END"
	}
	return s, nil
}

// ExecutorNode: 执行手脚
type ExecutorNode struct {
	tools map[string]Tool
}

func (n *ExecutorNode) Name() string { return "executor" }
func (n *ExecutorNode) Execute(ctx context.Context, s *State) (*State, error) {
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

// --- 4. 核心引擎与持久化实现 ---

type AgentOS struct {
	nodes      map[string]Node
	checkpoint Checkpointer
}

func (os *AgentOS) Run(ctx context.Context, state *State) (*State, error) {
	for state.NextNode != "END" {
		// 1. 执行前持久化：确保崩溃后可从此节点重试
		state.Version++
		if err := os.checkpoint.Save(ctx, state); err != nil {
			return nil, err
		}

		node, ok := os.nodes[state.NextNode]
		if !ok {
			return nil, fmt.Errorf("未找到节点: %s", state.NextNode)
		}

		// 2. 执行节点逻辑
		var err error
		state, err = node.Execute(ctx, state)
		if err != nil {
			return nil, err
		}
	}
	// 最终状态持久化
	os.checkpoint.Save(ctx, state)
	return state, nil
}

// --- 5. 模拟组件实现 ---

type MockLLM struct{}

func (m *MockLLM) Generate(ctx context.Context, msgs []Message) (Message, error) {
	query := msgs[len(msgs)-1].Content
	if strings.Contains(query, "天气") && !strings.Contains(query, "25°C") {
		return Message{Role: RoleAssistant, Content: "CALL:weather:上海"}, nil
	}
	return Message{Role: RoleAssistant, Content: "上海今天天气晴朗，气温 25°C。"}, nil
}

type WeatherTool struct{}

func (t *WeatherTool) Name() string { return "weather" }
func (t *WeatherTool) Execute(ctx context.Context, input string) (string, error) {
	return "25°C, 晴", nil
}

type MemoryCheckpointer struct {
	mu    sync.Mutex
	store map[string][]byte
}

func (m *MemoryCheckpointer) Save(ctx context.Context, s *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, _ := json.Marshal(s)
	m.store[s.ExecutionID] = data
	fmt.Printf("💾 [Checkpoint] 已保存版本 v%d\n", s.Version)
	return nil
}

func (m *MemoryCheckpointer) LoadLatest(ctx context.Context, id string) (*State, error) {
	if data, ok := m.store[id]; ok {
		var s State
		json.Unmarshal(data, &s)
		return &s, nil
	}
	return nil, fmt.Errorf("未找到记录")
}

// --- 6. 入口函数 ---

func main() {
	// 初始化抽象组件
	llm := &MockLLM{}
	tools := map[string]Tool{"weather": &WeatherTool{}}
	cp := &MemoryCheckpointer{store: make(map[string][]byte)}

	// 注册节点
	engine := &AgentOS{
		nodes: map[string]Node{
			"planner":  &PlannerNode{llm: llm},
			"executor": &ExecutorNode{tools: tools},
		},
		checkpoint: cp,
	}

	// 初始状态
	state := &State{
		ExecutionID: "task_101",
		Messages:    []Message{{Role: RoleUser, Content: "上海天气怎么样？"}},
		NextNode:    "planner",
	}

	// 运行 Agent
	finalState, err := engine.Run(context.Background(), state)
	if err != nil {
		fmt.Printf("运行失败: %v\n", err)
		return
	}

	fmt.Println("\n--- 最终对话历史 ---")
	for _, m := range finalState.Messages {
		fmt.Printf("[%s]: %s\n", m.Role, m.Content)
	}
}
