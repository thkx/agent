package model

import (
	"testing"
)

func TestStateTrimMessages(t *testing.T) {
	t.Parallel()

	state := &State{
		Counts: make(map[string]int),
	}

	// 创建超过MaxMessages的消息列表
	for i := 0; i < MaxMessages+50; i++ {
		state.Messages = append(state.Messages, Message{
			Role:    "user",
			Content: "message" + string(rune('0'+i%10)),
		})
	}

	if len(state.Messages) != MaxMessages+50 {
		t.Fatalf("expected %d messages before trim, got %d", MaxMessages+50, len(state.Messages))
	}

	// 修剪消息
	state.TrimMessages()

	if len(state.Messages) != MaxMessages {
		t.Fatalf("expected %d messages after trim, got %d", MaxMessages, len(state.Messages))
	}
}

func TestStateTrimMessagesNoOp(t *testing.T) {
	t.Parallel()

	state := &State{
		Counts: make(map[string]int),
	}

	// 创建少于MaxMessages的消息列表
	for i := 0; i < 10; i++ {
		state.Messages = append(state.Messages, Message{
			Role:    "user",
			Content: "message",
		})
	}

	originalLen := len(state.Messages)
	state.TrimMessages()

	if len(state.Messages) != originalLen {
		t.Fatalf("expected %d messages after trim, got %d", originalLen, len(state.Messages))
	}
}

func TestMemoryBoundedGrowth(t *testing.T) {
	t.Parallel()

	state := &State{
		Counts: make(map[string]int),
	}

	// 模拟多次添加消息和修剪
	for round := 0; round < 10; round++ {
		for i := 0; i < 30; i++ {
			state.Messages = append(state.Messages, Message{
				Role:    "user",
				Content: "message",
			})
		}
		state.TrimMessages()

		if len(state.Messages) > MaxMessages {
			t.Fatalf("round %d: expected at most %d messages, got %d", round, MaxMessages, len(state.Messages))
		}
	}

	// 最终消息数应该接近(但不超过)MaxMessages
	if len(state.Messages) != MaxMessages {
		t.Fatalf("expected %d messages, got %d", MaxMessages, len(state.Messages))
	}
}
