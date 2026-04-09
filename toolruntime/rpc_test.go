package toolruntime

import (
	"context"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"testing"
)

type rpcToolService struct{}

func (s *rpcToolService) Execute(call ToolCall, result *ToolResult) error {
	input, _ := call.Input.(map[string]any)
	*result = ToolResult{
		Output: map[string]any{
			"name":   call.Name,
			"symbol": input["symbol"],
		},
	}
	return nil
}

func TestRPCToolRuntimeExecute(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	server := rpc.NewServer()
	if err := server.RegisterName("ToolRuntime", &rpcToolService{}); err != nil {
		t.Fatalf("register rpc service: %v", err)
	}

	go server.ServeCodec(jsonrpc.NewServerCodec(serverConn))

	rt := NewRPCWithDialer(func(ctx context.Context) (RPCClient, error) {
		return &rpcClient{
			Client: jsonrpc.NewClient(clientConn),
			conn:   clientConn,
		}, nil
	})

	result, err := rt.Execute(context.Background(), ToolCall{
		Name: "get_price",
		Input: map[string]any{
			"symbol": "BTC",
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	output, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %#v", result.Output)
	}
	if output["name"] != "get_price" || output["symbol"] != "BTC" {
		t.Fatalf("unexpected rpc output: %#v", output)
	}
}
