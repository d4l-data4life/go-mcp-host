package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/weese/go-mcp-host/pkg/config"
	"github.com/weese/go-mcp-host/pkg/mcp/client"
	"github.com/weese/go-mcp-host/pkg/mcp/protocol"

	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// Manager manages MCP client sessions for conversations
type Manager struct {
	factory        *client.Factory
	serverConfigs  []config.MCPServerConfig
	sessions       map[string]*SessionInfo // key: conversationID:serverName
	mu             sync.RWMutex
	sessionTimeout time.Duration
}

// SessionInfo holds information about an active MCP session
type SessionInfo struct {
	Client         *client.Client
	ConversationID uuid.UUID
	ServerName     string
	ServerConfig   config.MCPServerConfig
	SessionID      uuid.UUID
	Tools          []protocol.Tool
	Resources      []protocol.Resource
	LastAccessed   time.Time
	mu             sync.RWMutex
}

// NewManager creates a new MCP manager
func NewManager(serverConfigs []config.MCPServerConfig) *Manager {
	factory := client.NewFactory("go-mcp-host", config.Version)

	m := &Manager{
		factory:        factory,
		serverConfigs:  serverConfigs,
		sessions:       make(map[string]*SessionInfo),
		sessionTimeout: 30 * time.Minute,
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m
}

// GetConfiguredServers returns the list of configured MCP servers
func (m *Manager) GetConfiguredServers() []config.MCPServerConfig {
	return m.serverConfigs
}

// ListAllTools lists all tools from all enabled servers for a conversation
func (m *Manager) ListAllTools(ctx context.Context, conversationID uuid.UUID, bearerToken string) ([]ToolWithServer, error) {
	return m.GetAllTools(ctx, conversationID)
}

// ListAllResources lists all resources from all enabled servers for a conversation
func (m *Manager) ListAllResources(ctx context.Context, conversationID uuid.UUID, bearerToken string) ([]ResourceWithServer, error) {
	return m.GetAllResources(ctx, conversationID)
}

// GetOrCreateSession gets or creates an MCP session for a conversation and server
func (m *Manager) GetOrCreateSession(ctx context.Context, conversationID uuid.UUID, serverConfig config.MCPServerConfig) (*SessionInfo, error) {
	sessionKey := m.getSessionKey(conversationID, serverConfig.Name)

	// Check if session already exists
	m.mu.RLock()
	if session, exists := m.sessions[sessionKey]; exists {
		session.mu.Lock()
		session.LastAccessed = time.Now()
		session.mu.Unlock()
		m.mu.RUnlock()
		return session, nil
	}
	m.mu.RUnlock()

	// Create new session
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if session, exists := m.sessions[sessionKey]; exists {
		session.mu.Lock()
		session.LastAccessed = time.Now()
		session.mu.Unlock()
		return session, nil
	}

	logging.LogDebugf("Creating new MCP session: conversation=%s server=%s", conversationID, serverConfig.Name)

	// Create MCP client
	mcpClient, err := m.factory.CreateClient(serverConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create MCP client for server %s", serverConfig.Name)
	}

	// Initialize client
	clientConfig := client.ClientConfig{
		ClientName:    "go-mcp-host",
		ClientVersion: config.Version,
		Capabilities: protocol.ClientCapabilities{
			Roots: &protocol.RootsCapability{
				ListChanged: true,
			},
			Sampling: map[string]interface{}{},
		},
	}

	if err := mcpClient.Initialize(ctx, clientConfig); err != nil {
		mcpClient.Close()
		return nil, errors.Wrapf(err, "failed to initialize MCP client for server %s", serverConfig.Name)
	}

	// Create session info (in-memory only)
	session := &SessionInfo{
		Client:         mcpClient,
		ConversationID: conversationID,
		ServerName:     serverConfig.Name,
		ServerConfig:   serverConfig,
		SessionID:      uuid.New(), // Generate UUID for internal tracking
		LastAccessed:   time.Now(),
	}

	// Set up notification handlers
	mcpClient.SetOnToolsListChanged(func() {
		m.refreshTools(ctx, session)
	})

	mcpClient.SetOnResourcesListChanged(func() {
		m.refreshResources(ctx, session)
	})

	// Load initial tools and resources
	m.refreshTools(ctx, session)
	m.refreshResources(ctx, session)

	// Store session
	m.sessions[sessionKey] = session

	logging.LogDebugf("Created MCP session: id=%s server=%s", session.SessionID, serverConfig.Name)

	return session, nil
}

// GetSession retrieves an existing session
func (m *Manager) GetSession(conversationID uuid.UUID, serverName string) (*SessionInfo, bool) {
	sessionKey := m.getSessionKey(conversationID, serverName)

	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionKey]
	if exists {
		session.mu.Lock()
		session.LastAccessed = time.Now()
		session.mu.Unlock()
	}

	return session, exists
}

// CloseSession closes a specific session
func (m *Manager) CloseSession(conversationID uuid.UUID, serverName string) error {
	sessionKey := m.getSessionKey(conversationID, serverName)

	m.mu.Lock()
	session, exists := m.sessions[sessionKey]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.sessions, sessionKey)
	m.mu.Unlock()

	// No database updates needed - sessions are in-memory only

	// Close client
	if err := session.Client.Close(); err != nil {
		logging.LogErrorf(err, "Failed to close MCP client")
		return err
	}

	logging.LogDebugf("Closed MCP session: conversation=%s server=%s", conversationID, serverName)
	return nil
}

// CloseAllSessionsForConversation closes all sessions for a conversation
func (m *Manager) CloseAllSessionsForConversation(conversationID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for key, session := range m.sessions {
		if session.ConversationID == conversationID {
			delete(m.sessions, key)

			// No database updates needed - sessions are in-memory only

			// Close client
			if err := session.Client.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing sessions: %v", errs)
	}

	logging.LogDebugf("Closed all MCP sessions for conversation: %s", conversationID)
	return nil
}

// GetAllTools returns all tools from all active sessions for a conversation
func (m *Manager) GetAllTools(ctx context.Context, conversationID uuid.UUID) ([]ToolWithServer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allTools []ToolWithServer

	for _, session := range m.sessions {
		if session.ConversationID == conversationID {
			session.mu.RLock()
			for _, tool := range session.Tools {
				allTools = append(allTools, ToolWithServer{
					Tool:       tool,
					ServerName: session.ServerName,
				})
			}
			session.mu.RUnlock()
		}
	}

	return allTools, nil
}

// CallTool calls a tool on the appropriate server
func (m *Manager) CallTool(ctx context.Context, conversationID uuid.UUID, serverName, toolName string, arguments map[string]interface{}) (*protocol.CallToolResult, error) {
	session, exists := m.GetSession(conversationID, serverName)
	if !exists {
		return nil, errors.Errorf("no active session for server %s", serverName)
	}

	// Update last accessed
	session.mu.Lock()
	session.LastAccessed = time.Now()
	session.mu.Unlock()

	// Call tool
	result, err := session.Client.CallTool(ctx, toolName, arguments)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call tool %s on server %s", toolName, serverName)
	}

	// Update last accessed time (in-memory only)
	session.mu.Lock()
	session.LastAccessed = time.Now()
	session.mu.Unlock()

	return result, nil
}

// GetAllResources returns all resources from all active sessions for a conversation
func (m *Manager) GetAllResources(ctx context.Context, conversationID uuid.UUID) ([]ResourceWithServer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allResources []ResourceWithServer

	for _, session := range m.sessions {
		if session.ConversationID == conversationID {
			session.mu.RLock()
			for _, resource := range session.Resources {
				allResources = append(allResources, ResourceWithServer{
					Resource:   resource,
					ServerName: session.ServerName,
				})
			}
			session.mu.RUnlock()
		}
	}

	return allResources, nil
}

// ReadResource reads a resource from the appropriate server
func (m *Manager) ReadResource(ctx context.Context, conversationID uuid.UUID, serverName, resourceURI string) (*protocol.ReadResourceResult, error) {
	session, exists := m.GetSession(conversationID, serverName)
	if !exists {
		return nil, errors.Errorf("no active session for server %s", serverName)
	}

	// Update last accessed
	session.mu.Lock()
	session.LastAccessed = time.Now()
	session.mu.Unlock()

	// Read resource
	result, err := session.Client.ReadResource(ctx, resourceURI)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read resource %s from server %s", resourceURI, serverName)
	}

	// Update last accessed time (in-memory only)
	session.mu.Lock()
	session.LastAccessed = time.Now()
	session.mu.Unlock()

	return result, nil
}

// refreshTools refreshes the tools list for a session
func (m *Manager) refreshTools(ctx context.Context, session *SessionInfo) {
	tools, err := session.Client.ListTools(ctx)
	if err != nil {
		logging.LogErrorf(err, "Failed to refresh tools for server %s", session.ServerName)
		return
	}

	session.mu.Lock()
	session.Tools = tools
	session.mu.Unlock()

	logging.LogDebugf("Refreshed tools for server %s: %d tools", session.ServerName, len(tools))
}

// refreshResources refreshes the resources list for a session
func (m *Manager) refreshResources(ctx context.Context, session *SessionInfo) {
	resources, err := session.Client.ListResources(ctx)
	if err != nil {
		logging.LogErrorf(err, "Failed to refresh resources for server %s", session.ServerName)
		return
	}

	session.mu.Lock()
	session.Resources = resources
	session.mu.Unlock()

	logging.LogDebugf("Refreshed resources for server %s: %d resources", session.ServerName, len(resources))
}

// cleanupLoop periodically cleans up inactive sessions
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupInactiveSessions()
	}
}

// cleanupInactiveSessions closes sessions that have been inactive for too long
func (m *Manager) cleanupInactiveSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, session := range m.sessions {
		session.mu.RLock()
		lastAccessed := session.LastAccessed
		session.mu.RUnlock()

		if now.Sub(lastAccessed) > m.sessionTimeout {
			delete(m.sessions, key)

			// Close client
			session.Client.Close()

			logging.LogDebugf("Cleaned up inactive MCP session: conversation=%s server=%s", session.ConversationID, session.ServerName)
		}
	}
}

// Helper methods

func (m *Manager) getSessionKey(conversationID uuid.UUID, serverName string) string {
	return fmt.Sprintf("%s:%s", conversationID.String(), serverName)
}

// Helper methods removed - no longer needed for in-memory cache

// Helper types

// ToolWithServer associates a tool with its server
type ToolWithServer struct {
	Tool       protocol.Tool
	ServerName string
}

// ResourceWithServer associates a resource with its server
type ResourceWithServer struct {
	Resource   protocol.Resource
	ServerName string
}
