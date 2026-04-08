package tool

import (
	"context"
	"fmt"
)

type PriceTool struct{}

func (t *PriceTool) Name() string {
	return "get_price"
}

func (t *PriceTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	symbol, ok := args["symbol"]
	fmt.Println(symbol, ok)
	return "BTC price is 65000 (mock)", nil
}
