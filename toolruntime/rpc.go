package toolruntime

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

type RPCClient interface {
	Call(serviceMethod string, args any, reply any) error
	Close() error
}

type rpcClient struct {
	*rpc.Client
	conn net.Conn
}

func (c *rpcClient) Close() error {
	_ = c.Client.Close()
	return c.conn.Close()
}

type RPCDialer func(context.Context) (RPCClient, error)

type RPCToolRuntime struct {
	dialer        RPCDialer
	serviceMethod string
}

func NewRPC(network, address string) *RPCToolRuntime {
	return &RPCToolRuntime{
		dialer: func(ctx context.Context) (RPCClient, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &rpcClient{
				Client: jsonrpc.NewClient(conn),
				conn:   conn,
			}, nil
		},
		serviceMethod: "ToolRuntime.Execute",
	}
}

func NewRPCWithDialer(dialer RPCDialer) *RPCToolRuntime {
	return &RPCToolRuntime{
		dialer:        dialer,
		serviceMethod: "ToolRuntime.Execute",
	}
}

func (r *RPCToolRuntime) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	if r.dialer == nil {
		return ToolResult{}, fmt.Errorf("rpc dialer not configured")
	}

	client, err := r.dialer(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	defer client.Close()

	type result struct {
		reply ToolResult
		err   error
	}

	done := make(chan result, 1)
	go func() {
		var reply ToolResult
		err := client.Call(r.serviceMethod, call, &reply)
		done <- result{reply: reply, err: err}
	}()

	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	case res := <-done:
		return res.reply, res.err
	}
}
