package tool

import (
	"context"
	"time"
)

type Property struct {
	Type        string              `json:"type,omitempty"`
	Description string              `json:"description,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Enum        []any               `json:"enum,omitempty"`
}

type Definition struct {
	Name        string
	Description string
	Schema      Schema
}

type Tool interface {
	Name() string
	Description() string
	Schema() Schema
	Invoke(ctx context.Context, input any) (any, error)
	// 新增：超时和权限
	Timeout() time.Duration
	Permissions() []string
}

type Catalog interface {
	Definitions() []Definition
}
