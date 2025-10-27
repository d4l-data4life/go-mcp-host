package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/weese/go-mcp-host/pkg/mcp/protocol"
	"github.com/weese/go-mcp-host/pkg/mcp/transport"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
	"github.com/pkg/errors"
)

// Client represents an MCP client that communicates with an MCP server
type Client struct {
	transport       transport.Transport
	serverInfo      *protocol.Implementation
	serverCapabilities *protocol.ServerCapabilities
	mu              sync.RWMutex
	initialized     bool
	requestIDCounter uint64

	// Notification callbacks
	onToolsListChanged     func()
	onResourcesListChanged func()
	onResourcesUpdated     func(uri string)
	onPromptsListChanged   func()
}

// ClientConfig holds configuration for creating a client
type ClientConfig struct {
	ClientName    string
	ClientVersion string
	Capabilities  protocol.ClientCapabilities
}

// NewClient creates a new MCP client with the given transport
func NewClient(trans transport.Transport, config ClientConfig) *Client {
	client := &Client{
		transport: trans,
	}

	// Add notification handler to transport
	if stdioTrans, ok := trans.(*transport.StdioTransport); ok {
		stdioTrans.AddNotificationHandler(client.handleNotification)
	} else if httpTrans, ok := trans.(*transport.HTTPTransport); ok {
		httpTrans.AddNotificationHandler(client.handleNotification)
	}

	return client
}

// Initialize performs the MCP initialization handshake
func (c *Client) Initialize(ctx context.Context, config ClientConfig) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return errors.New("client already initialized")
	}
	c.mu.Unlock()

	// Create initialize request
	initRequest := protocol.InitializeRequest{
		ProtocolVersion: protocol.MCPProtocolVersion,
		Capabilities:    config.Capabilities,
		ClientInfo: protocol.Implementation{
			Name:    config.ClientName,
			Version: config.ClientVersion,
		},
	}

	paramsData, err := json.Marshal(initRequest)
	if err != nil {
		return errors.Wrap(err, "failed to marshal initialize request")
	}

	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodInitialize,
		Params:  paramsData,
	}

	// Send initialize request
	response, err := c.transport.Send(ctx, request)
	if err != nil {
		return errors.Wrap(err, "initialize request failed")
	}

	// Parse initialize result
	var result protocol.InitializeResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return errors.Wrap(err, "failed to parse initialize result")
	}

	c.mu.Lock()
	c.serverInfo = &result.ServerInfo
	c.serverCapabilities = &result.Capabilities
	c.initialized = true
	c.mu.Unlock()

	logging.LogDebugf("MCP client initialized: server=%s version=%s", result.ServerInfo.Name, result.ServerInfo.Version)

	// Send initialized notification
	notification := &protocol.JSONRPCNotification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  protocol.NotificationInitialized,
	}

	if err := c.transport.SendNotification(ctx, notification); err != nil {
		logging.LogWarningf(err, "Failed to send initialized notification")
	}

	return nil
}

// GetCapabilities returns the server capabilities after initialization
func (c *Client) GetCapabilities() *protocol.ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverCapabilities
}

// ListTools lists all available tools from the server
func (c *Client) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	if !c.isInitialized() {
		return nil, errors.New("client not initialized")
	}

	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodListTools,
	}

	response, err := c.transport.Send(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "list tools request failed")
	}

	var result protocol.ListToolsResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse list tools result")
	}

	return result.Tools, nil
}

// CallTool executes a tool with the given arguments
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*protocol.CallToolResult, error) {
	if !c.isInitialized() {
		return nil, errors.New("client not initialized")
	}

	callRequest := protocol.CallToolRequest{
		Name:      name,
		Arguments: arguments,
	}

	paramsData, err := json.Marshal(callRequest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal call tool request")
	}

	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodCallTool,
		Params:  paramsData,
	}

	response, err := c.transport.Send(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "call tool request failed")
	}

	var result protocol.CallToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse call tool result")
	}

	return &result, nil
}

// ListResources lists all available resources from the server
func (c *Client) ListResources(ctx context.Context) ([]protocol.Resource, error) {
	if !c.isInitialized() {
		return nil, errors.New("client not initialized")
	}

	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodListResources,
	}

	response, err := c.transport.Send(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "list resources request failed")
	}

	var result protocol.ListResourcesResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse list resources result")
	}

	return result.Resources, nil
}

// ReadResource reads the contents of a resource
func (c *Client) ReadResource(ctx context.Context, uri string) (*protocol.ReadResourceResult, error) {
	if !c.isInitialized() {
		return nil, errors.New("client not initialized")
	}

	readRequest := protocol.ReadResourceRequest{
		URI: uri,
	}

	paramsData, err := json.Marshal(readRequest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal read resource request")
	}

	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodReadResource,
		Params:  paramsData,
	}

	response, err := c.transport.Send(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "read resource request failed")
	}

	var result protocol.ReadResourceResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse read resource result")
	}

	return &result, nil
}

// ListPrompts lists all available prompts from the server
func (c *Client) ListPrompts(ctx context.Context) ([]protocol.Prompt, error) {
	if !c.isInitialized() {
		return nil, errors.New("client not initialized")
	}

	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodListPrompts,
	}

	response, err := c.transport.Send(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "list prompts request failed")
	}

	var result protocol.ListPromptsResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse list prompts result")
	}

	return result.Prompts, nil
}

// GetPrompt retrieves a prompt with the given arguments
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*protocol.GetPromptResult, error) {
	if !c.isInitialized() {
		return nil, errors.New("client not initialized")
	}

	getRequest := protocol.GetPromptRequest{
		Name:      name,
		Arguments: arguments,
	}

	paramsData, err := json.Marshal(getRequest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal get prompt request")
	}

	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodGetPrompt,
		Params:  paramsData,
	}

	response, err := c.transport.Send(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "get prompt request failed")
	}

	var result protocol.GetPromptResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse get prompt result")
	}

	return &result, nil
}

// Ping sends a ping to check if the server is alive
func (c *Client) Ping(ctx context.Context) error {
	request := &protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      c.nextRequestID(),
		Method:  protocol.MethodPing,
	}

	_, err := c.transport.Send(ctx, request)
	return err
}

// Close closes the client and its transport
func (c *Client) Close() error {
	return c.transport.Close()
}

// GetServerInfo returns information about the connected server
func (c *Client) GetServerInfo() *protocol.Implementation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// GetServerCapabilities returns the capabilities of the connected server
func (c *Client) GetServerCapabilities() *protocol.ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverCapabilities
}

// SetOnToolsListChanged sets a callback for when the tools list changes
func (c *Client) SetOnToolsListChanged(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onToolsListChanged = callback
}

// SetOnResourcesListChanged sets a callback for when the resources list changes
func (c *Client) SetOnResourcesListChanged(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onResourcesListChanged = callback
}

// SetOnResourcesUpdated sets a callback for when a resource is updated
func (c *Client) SetOnResourcesUpdated(callback func(uri string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onResourcesUpdated = callback
}

// SetOnPromptsListChanged sets a callback for when the prompts list changes
func (c *Client) SetOnPromptsListChanged(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onPromptsListChanged = callback
}

// handleNotification handles incoming notifications from the server
func (c *Client) handleNotification(message interface{}) error {
	notification, ok := message.(*protocol.JSONRPCNotification)
	if !ok {
		return errors.New("invalid notification type")
	}

	logging.LogDebugf("Received notification: %s", notification.Method)

	c.mu.RLock()
	defer c.mu.RUnlock()

	switch notification.Method {
	case protocol.NotificationToolsListChanged:
		if c.onToolsListChanged != nil {
			go c.onToolsListChanged()
		}
	case protocol.NotificationResourcesListChanged:
		if c.onResourcesListChanged != nil {
			go c.onResourcesListChanged()
		}
	case protocol.NotificationResourcesUpdated:
		var params protocol.ResourceUpdatedNotification
		if err := json.Unmarshal(notification.Params, &params); err == nil {
			if c.onResourcesUpdated != nil {
				go c.onResourcesUpdated(params.URI)
			}
		}
	case protocol.NotificationPromptsListChanged:
		if c.onPromptsListChanged != nil {
			go c.onPromptsListChanged()
		}
	case protocol.NotificationProgress:
		// Log progress notifications
		var params protocol.ProgressNotification
		if err := json.Unmarshal(notification.Params, &params); err == nil {
			logging.LogDebugf("Progress: %.2f/%.2f", params.Progress, params.Total)
		}
	case protocol.NotificationMessage:
		// Log message notifications
		var params protocol.MessageNotification
		if err := json.Unmarshal(notification.Params, &params); err == nil {
			logging.LogDebugf("Server message [%s]: %s", params.Level, params.Data)
		}
	default:
		logging.LogDebugf("Unknown notification method: %s", notification.Method)
	}

	return nil
}

// isInitialized checks if the client is initialized (thread-safe)
func (c *Client) isInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

// nextRequestID generates a unique request ID
func (c *Client) nextRequestID() interface{} {
	return fmt.Sprintf("req_%d", atomic.AddUint64(&c.requestIDCounter, 1))
}

