package tool

import "context"

type Tool interface {
	Name() string
	Execute(ctx context.Context, args map[string]any) (any, error)
}
