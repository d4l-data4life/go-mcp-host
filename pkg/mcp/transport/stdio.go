package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/weese/go-mcp-host/pkg/mcp/protocol"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
	"github.com/pkg/errors"
)

// StdioTransport implements the Transport interface for stdio-based communication
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	reader    *bufio.Reader
	writer    *bufio.Writer
	mu        sync.Mutex
	connected bool
	ctx       context.Context
	cancel    context.CancelFunc

	// Message queue for handling responses
	pendingRequests map[interface{}]chan *protocol.JSONRPCResponse
	responseMu      sync.RWMutex

	// Notification handlers
	notificationHandlers []MessageHandler
	notificationMu       sync.RWMutex
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args []string, env []string) (*StdioTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, command, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "failed to create stdin pipe")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		stdin.Close()
		return nil, errors.Wrap(err, "failed to create stdout pipe")
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		stdin.Close()
		stdout.Close()
		return nil, errors.Wrap(err, "failed to create stderr pipe")
	}

	transport := &StdioTransport{
		cmd:                  cmd,
		stdin:                stdin,
		stdout:               stdout,
		stderr:               stderr,
		reader:               bufio.NewReader(stdout),
		writer:               bufio.NewWriter(stdin),
		ctx:                  ctx,
		cancel:               cancel,
		pendingRequests:      make(map[interface{}]chan *protocol.JSONRPCResponse),
		notificationHandlers: make([]MessageHandler, 0),
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, errors.Wrap(err, "failed to start command")
	}

	transport.connected = true

	// Start reading responses in background
	go transport.readLoop()

	// Monitor stderr
	go transport.stderrLoop()

	logging.LogDebugf("Started stdio transport: %s %v", command, args)

	return transport, nil
}

// Send sends a JSON-RPC request and waits for a response
func (t *StdioTransport) Send(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	if !t.connected {
		return nil, errors.New("transport not connected")
	}

	// Create response channel
	responseChan := make(chan *protocol.JSONRPCResponse, 1)
	t.responseMu.Lock()
	t.pendingRequests[request.ID] = responseChan
	t.responseMu.Unlock()

	// Clean up on exit
	defer func() {
		t.responseMu.Lock()
		delete(t.pendingRequests, request.ID)
		t.responseMu.Unlock()
		close(responseChan)
	}()

	// Send request
	if err := t.writeMessage(request); err != nil {
		return nil, errors.Wrap(err, "failed to write request")
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.ctx.Done():
		return nil, errors.New("transport closed")
	case response := <-responseChan:
		if response.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
		}
		return response, nil
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (t *StdioTransport) SendNotification(ctx context.Context, notification *protocol.JSONRPCNotification) error {
	if !t.connected {
		return errors.New("transport not connected")
	}

	return t.writeMessage(notification)
}

// Receive receives the next message from the transport
func (t *StdioTransport) Receive(ctx context.Context) (interface{}, error) {
	// This is handled by readLoop in background
	// For stdio, we don't expose raw Receive to consumers
	return nil, errors.New("not implemented for stdio transport")
}

// Close closes the transport
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	t.connected = false
	t.cancel()

	// Close pipes
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.stdout != nil {
		t.stdout.Close()
	}
	if t.stderr != nil {
		t.stderr.Close()
	}

	// Wait for process to exit
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}

	logging.LogDebugf("Closed stdio transport")
	return nil
}

// IsConnected returns whether the transport is connected
func (t *StdioTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// AddNotificationHandler adds a handler for notifications
func (t *StdioTransport) AddNotificationHandler(handler MessageHandler) {
	t.notificationMu.Lock()
	defer t.notificationMu.Unlock()
	t.notificationHandlers = append(t.notificationHandlers, handler)
}

// writeMessage writes a message to stdin
func (t *StdioTransport) writeMessage(message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	logging.LogDebugf("Sending MCP message: %s", string(data))

	// Write message followed by newline
	if _, err := t.writer.Write(data); err != nil {
		return errors.Wrap(err, "failed to write message")
	}
	if err := t.writer.WriteByte('\n'); err != nil {
		return errors.Wrap(err, "failed to write newline")
	}
	if err := t.writer.Flush(); err != nil {
		return errors.Wrap(err, "failed to flush")
	}

	return nil
}

// readLoop continuously reads messages from stdout
func (t *StdioTransport) readLoop() {
	for t.connected {
		line, err := t.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				logging.LogDebugf("MCP server closed connection")
				t.Close()
				return
			}
			logging.LogErrorf(err, "Error reading from stdout")
			continue
		}

		logging.LogDebugf("Received MCP message: %s", string(line))

		// Try to parse as response first
		var response protocol.JSONRPCResponse
		if err := json.Unmarshal(line, &response); err == nil && response.ID != nil {
			t.responseMu.RLock()
			if ch, ok := t.pendingRequests[response.ID]; ok {
				ch <- &response
			}
			t.responseMu.RUnlock()
			continue
		}

		// Try to parse as notification
		var notification protocol.JSONRPCNotification
		if err := json.Unmarshal(line, &notification); err == nil && notification.Method != "" {
			t.notificationMu.RLock()
			handlers := t.notificationHandlers
			t.notificationMu.RUnlock()

			for _, handler := range handlers {
				if err := handler(&notification); err != nil {
					logging.LogErrorf(err, "Error handling notification: %s", notification.Method)
				}
			}
			continue
		}

		logging.LogWarningf(nil, "Received unknown message type: %s", string(line))
	}
}

// stderrLoop monitors stderr for debug output
func (t *StdioTransport) stderrLoop() {
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		logging.LogDebugf("MCP stderr: %s", line)
	}
}

