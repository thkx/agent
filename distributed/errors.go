package distributed

import "errors"

var (
	ErrTransportClosed = errors.New("transport is closed")
	ErrChannelFull     = errors.New("channel is full")
	ErrNodeNotFound    = errors.New("node not found")
	ErrNodeAlreadyExists = errors.New("node already exists")
)
