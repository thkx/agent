package tool

import (
	"context"
)

type Property struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
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
}

type Catalog interface {
	Definitions() []Definition
}
