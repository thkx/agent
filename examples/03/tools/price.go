package tools

import (
	"context"
	"fmt"

	"github.com/thkx/agent/tool"
)

type PriceTool struct{}

func (t *PriceTool) Name() string {
	return "get_price"
}

func (t *PriceTool) Description() string {
	return "Get the latest price for a crypto symbol."
}

func (t *PriceTool) Schema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Property{
			"symbol": {
				Type:        "string",
				Description: "Crypto symbol like BTC or ETH",
			},
		},
		Required: []string{"symbol"},
	}
}

func (t *PriceTool) Invoke(ctx context.Context, input any) (any, error) {
	args, _ := input.(map[string]any)
	symbol, ok := args["symbol"]
	fmt.Println(symbol, ok)
	return "BTC price is 65000 (mock)", nil
}
