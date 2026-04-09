package tools

import (
	"context"
	"fmt"
	"strings"

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

	symbol, _ := args["symbol"].(string)
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		symbol = "BTC"
	}

	return fmt.Sprintf("%s price is 65000 (mock)", strings.ToUpper(symbol)), nil
}
