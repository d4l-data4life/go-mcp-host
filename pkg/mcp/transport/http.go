package transport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/protocol"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// HTTPTransport implements the Transport interface for HTTP-based communication
type HTTPTransport struct {
	baseURL     string
	headers     map[string]string
	client      *http.Client
	mu          sync.Mutex
	connected   bool
	ctx         context.Context
	cancel      context.CancelFunc
	sseReader   *bufio.Reader
	sseResponse *http.Response

	// Session management for HTTP-based MCP servers
	sessionID string

	// Notification handlers
	notificationHandlers []MessageHandler
	notificationMu       sync.RWMutex
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(baseURL string, headers map[string]string, tlsSkipVerify bool) (*HTTPTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tlsSkipVerify,
		},
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	t := &HTTPTransport{
		baseURL:              baseURL,
		headers:              headers,
		client:               client,
		ctx:                  ctx,
		cancel:               cancel,
		connected:            true,
		notificationHandlers: make([]MessageHandler, 0),
		sessionID:            "",
	}

	logging.LogDebugf("Created HTTP transport: %s", baseURL)

	return t, nil
}

// Send sends a JSON-RPC request and waits for a response
func (t *HTTPTransport) Send(ctx context.Context, request *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	if !t.connected {
		return nil, errors.New("transport not connected")
	}

	// Marshal request
	data, err := json.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	logging.LogDebugf("Sending HTTP MCP request: %s", string(data))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewReader(data))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		httpReq.Header.Set(key, value)
		// Debug log for Authorization header forwarding
		if key == "Authorization" && len(value) > 5 {
			logging.LogDebugf("Forwarding Authorization header (first 5 chars): %s...", value[:5])
		}
	}
	// Per MCP HTTP spec, client must Accept both JSON responses and SSE
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	// Attach session header if present
	if t.sessionID != "" {
		httpReq.Header.Set("mcp-session-id", t.sessionID)
	}

	// Send request
	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send HTTP request")
	}
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(body))
	}

	contentType := httpResp.Header.Get("Content-Type")
	// If server responds using SSE, extract JSON payload from data: lines
	if strings.Contains(contentType, "text/event-stream") {
		raw, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read SSE response body")
		}
		logging.LogDebugf("Received HTTP MCP response: %s", string(raw))
		// Capture session id if provided on initialize
		if request.Method == protocol.MethodInitialize {
			if sid := httpResp.Header.Get("mcp-session-id"); sid != "" {
				logging.LogDebugf("Captured MCP session id from HTTP initialize (SSE): %s", sid)
				t.sessionID = sid
			}
		}

		resp, err := parseSSEJSONRPCResponse(string(raw), request.ID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse SSE JSON-RPC response")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	}

	// Read response as plain JSON
	respData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	logging.LogDebugf("Received HTTP MCP response: %s", string(respData))

	// Capture session id if provided on initialize
	if request.Method == protocol.MethodInitialize {
		if sid := httpResp.Header.Get("mcp-session-id"); sid != "" {
			logging.LogDebugf("Captured MCP session id from HTTP initialize: %s", sid)
			t.sessionID = sid
		}
	}

	// Parse response
	var response protocol.JSONRPCResponse
	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}

	if response.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	return &response, nil
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (t *HTTPTransport) SendNotification(ctx context.Context, notification *protocol.JSONRPCNotification) error {
	if !t.connected {
		return errors.New("transport not connected")
	}

	// Marshal notification
	data, err := json.Marshal(notification)
	if err != nil {
		return errors.Wrap(err, "failed to marshal notification")
	}

	logging.LogDebugf("Sending HTTP MCP notification: %s", string(data))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewReader(data))
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request")
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		httpReq.Header.Set(key, value)
		// Debug log for Authorization header forwarding
		if key == "Authorization" && len(value) > 5 {
			logging.LogDebugf("Forwarding Authorization header (first 5 chars): %s...", value[:5])
		}
	}
	// Ensure server can send either JSON or SSE compatible responses
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	// Attach session header if present
	if t.sessionID != "" {
		httpReq.Header.Set("mcp-session-id", t.sessionID)
	}

	// Send request (ignore response for notifications)
	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return errors.Wrap(err, "failed to send HTTP notification")
	}
	defer httpResp.Body.Close()

	// Drain response body
	_, _ = io.Copy(io.Discard, httpResp.Body)

	return nil
}

// Receive receives the next message from the transport (for SSE)
func (t *HTTPTransport) Receive(ctx context.Context) (interface{}, error) {
	// Not implemented for basic HTTP transport
	// SSE support would require a separate connection
	return nil, errors.New("receive not supported for HTTP transport")
}

// StartSSE starts listening for Server-Sent Events
func (t *HTTPTransport) StartSSE(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.sseResponse != nil {
		return errors.New("SSE already started")
	}

	// Create SSE endpoint URL (typically /sse or /events)
	sseURL := strings.TrimSuffix(t.baseURL, "/") + "/sse"

	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create SSE request")
	}

	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to connect to SSE")
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d", resp.StatusCode)
	}

	t.sseResponse = resp
	t.sseReader = bufio.NewReader(resp.Body)

	logging.LogDebugf("Started SSE connection to %s", sseURL)

	// Start reading SSE events
	go t.sseReadLoop()

	return nil
}

// sseReadLoop reads Server-Sent Events
func (t *HTTPTransport) sseReadLoop() {
	defer func() {
		if t.sseResponse != nil {
			t.sseResponse.Body.Close()
			t.sseResponse = nil
		}
	}()

	var eventType string
	var eventData strings.Builder

	for t.connected {
		line, err := t.sseReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				logging.LogDebugf("SSE connection closed")
				return
			}
			logging.LogErrorf(err, "Error reading SSE")
			return
		}

		line = strings.TrimSpace(line)

		// Empty line signals end of event
		if line == "" {
			if eventData.Len() > 0 {
				t.handleSSEEvent(eventType, eventData.String())
				eventType = ""
				eventData.Reset()
			}
			continue
		}

		// Parse SSE field
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(data)
		}
		// Ignore comments (lines starting with :)
	}
}

// handleSSEEvent handles a received SSE event
func (t *HTTPTransport) handleSSEEvent(eventType, data string) {
	logging.LogDebugf("Received SSE event [%s]: %s", eventType, data)

	// Try to parse as JSON-RPC notification
	var notification protocol.JSONRPCNotification
	if err := json.Unmarshal([]byte(data), &notification); err == nil {
		t.notificationMu.RLock()
		handlers := t.notificationHandlers
		t.notificationMu.RUnlock()

		for _, handler := range handlers {
			if err := handler(&notification); err != nil {
				logging.LogErrorf(err, "Error handling SSE notification: %s", notification.Method)
			}
		}
	} else {
		logging.LogWarningf(err, "Failed to parse SSE event as notification: %s", data)
	}
}

// parseSSEJSONRPCResponse extracts the JSON-RPC response from an SSE-formatted payload.
// It scans for data: lines within SSE events and returns the first JSON object
// that parses into a JSONRPCResponse matching the given request ID (if provided).
func parseSSEJSONRPCResponse(payload string, requestID interface{}) (*protocol.JSONRPCResponse, error) {
	scanner := bufio.NewScanner(strings.NewReader(payload))
	var eventData strings.Builder

	flush := func() (*protocol.JSONRPCResponse, bool) {
		if eventData.Len() == 0 {
			return nil, false
		}
		candidate := eventData.String()
		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal([]byte(candidate), &resp); err == nil {
			if requestID == nil || fmt.Sprint(resp.ID) == fmt.Sprint(requestID) {
				return &resp, true
			}
		}
		eventData.Reset()
		return nil, false
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if resp, ok := flush(); ok {
				return resp, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(data)
		}
	}

	// Flush any remaining buffered data (in case stream ended without blank line)
	if resp, ok := flush(); ok {
		return resp, nil
	}

	return nil, errors.New("no JSON-RPC response found in SSE payload")
}

// Close closes the transport
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	t.connected = false
	t.cancel()

	if t.sseResponse != nil {
		t.sseResponse.Body.Close()
		t.sseResponse = nil
	}

	logging.LogDebugf("Closed HTTP transport")
	return nil
}

// IsConnected returns whether the transport is connected
func (t *HTTPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// AddNotificationHandler adds a handler for notifications
func (t *HTTPTransport) AddNotificationHandler(handler MessageHandler) {
	t.notificationMu.Lock()
	defer t.notificationMu.Unlock()
	t.notificationHandlers = append(t.notificationHandlers, handler)
}
