package model

type Message struct {
	Role    string
	Content string
}

type State struct {
	Messages []Message
	Context  map[string]any
	Meta     map[string]any
	Counts   map[string]int // 每个节点执行次数
}
