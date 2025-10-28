package transport

import (
	"context"
	"io"

	"github.com/weese/go-mcp-host/pkg/mcp/protocol"
)

// Transport defines the interface for MCP transport mechanisms
type Transport interface {
	// Send sends a JSON-RPC request and waits for a response
	Send(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error)

	// SendNotification sends a JSON-RPC notification (no response expected)
	SendNotification(ctx context.Context, notification *protocol.JSONRPCNotification) error

	// Receive receives incoming messages from the server (for notifications and bidirectional communication)
	Receive(ctx context.Context) (interface{}, error)

	// Close closes the transport connection
	Close() error

	// IsConnected returns whether the transport is currently connected
	IsConnected() bool
}

// Message represents a message that can be sent or received
type Message interface {
	GetMethod() string
	GetID() interface{}
}

// TransportType defines the type of transport
type TransportType string

const (
	TransportTypeStdio TransportType = "stdio"
	TransportTypeHTTP  TransportType = "http"
	TransportTypeSSE   TransportType = "sse"
)

// Config holds transport configuration
type Config struct {
	Type TransportType

	// Stdio configuration
	Command string
	Args    []string
	Env     []string

	// HTTP/SSE configuration
	URL         string
	Headers     map[string]string
	BearerToken string

	// TLS configuration
	TLSSkipVerify bool
}

// MessageHandler is a function that handles incoming messages
type MessageHandler func(message interface{}) error

// ReadWriter wraps io.Reader and io.Writer for message passing
type ReadWriter struct {
	Reader io.Reader
	Writer io.Writer
}

// NewReadWriter creates a new ReadWriter
func NewReadWriter(r io.Reader, w io.Writer) *ReadWriter {
	return &ReadWriter{
		Reader: r,
		Writer: w,
	}
}
