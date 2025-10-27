package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/weese/go-mcp-host/pkg/config"
	"github.com/weese/go-mcp-host/pkg/mcp/client"
	"github.com/weese/go-mcp-host/pkg/mcp/protocol"
	"github.com/weese/go-mcp-host/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// Manager manages MCP client sessions for conversations
type Manager struct {
	db             *gorm.DB
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
func NewManager(serverConfigs []config.MCPServerConfig, db *gorm.DB) *Manager {
	factory := client.NewFactory("go-mcp-host", config.Version)

	m := &Manager{
		db:             db,
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

	// Create session record in database
	connectionInfo, err := m.getConnectionInfo(serverConfig)
	if err != nil {
		mcpClient.Close()
		return nil, errors.Wrap(err, "failed to get connection info")
	}

	capabilities, err := m.getCapabilitiesJSON(mcpClient.GetServerCapabilities())
	if err != nil {
		mcpClient.Close()
		return nil, errors.Wrap(err, "failed to serialize capabilities")
	}

	dbSession := &models.MCPSession{
		ConversationID: conversationID,
		ServerName:     serverConfig.Name,
		ServerType:     models.MCPSessionType(serverConfig.Type),
		ConnectionInfo: connectionInfo,
		Capabilities:   capabilities,
		Status:         models.MCPSessionStatusConnected,
		LastActiveAt:   time.Now(),
	}

	if err := m.db.Create(dbSession).Error; err != nil {
		mcpClient.Close()
		return nil, errors.Wrap(err, "failed to create session in database")
	}

	// Create session info
	session := &SessionInfo{
		Client:         mcpClient,
		ConversationID: conversationID,
		ServerName:     serverConfig.Name,
		ServerConfig:   serverConfig,
		SessionID:      dbSession.ID,
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

	logging.LogDebugf("Created MCP session: id=%s server=%s", dbSession.ID, serverConfig.Name)

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

	// Update database
	if err := m.db.Model(&models.MCPSession{}).
		Where("id = ?", session.SessionID).
		Updates(map[string]interface{}{
			"status":         models.MCPSessionStatusDisconnected,
			"last_active_at": time.Now(),
		}).Error; err != nil {
		logging.LogErrorf(err, "Failed to update session status in database")
	}

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

			// Update database
			if err := m.db.Model(&models.MCPSession{}).
				Where("id = ?", session.SessionID).
				Updates(map[string]interface{}{
					"status":         models.MCPSessionStatusDisconnected,
					"last_active_at": time.Now(),
				}).Error; err != nil {
				errs = append(errs, err)
			}

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

	// Update last active in database
	m.db.Model(&models.MCPSession{}).
		Where("id = ?", session.SessionID).
		Update("last_active_at", time.Now())

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

	// Update last active in database
	m.db.Model(&models.MCPSession{}).
		Where("id = ?", session.SessionID).
		Update("last_active_at", time.Now())

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

	// Update database cache
	m.updateToolsCache(session.SessionID, tools)

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

	// Update database cache
	m.updateResourcesCache(session.SessionID, resources)

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

			// Update database
			m.db.Model(&models.MCPSession{}).
				Where("id = ?", session.SessionID).
				Updates(map[string]interface{}{
					"status":         models.MCPSessionStatusDisconnected,
					"last_active_at": time.Now(),
				})

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

func (m *Manager) getConnectionInfo(serverConfig config.MCPServerConfig) ([]byte, error) {
	var info interface{}

	switch serverConfig.Type {
	case "stdio":
		info = models.StdioConnectionInfo{
			Command: serverConfig.Command,
			Args:    serverConfig.Args,
			Env:     m.mapToSlice(serverConfig.Env),
		}
	case "http":
		info = models.HTTPConnectionInfo{
			URL:     serverConfig.URL,
			Headers: serverConfig.Headers,
		}
	default:
		return nil, errors.Errorf("unsupported transport type: %s", serverConfig.Type)
	}

	return json.Marshal(info)
}

func (m *Manager) getCapabilitiesJSON(caps *protocol.ServerCapabilities) ([]byte, error) {
	if caps == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(caps)
}

func (m *Manager) mapToSlice(envMap map[string]string) []string {
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

func (m *Manager) updateToolsCache(sessionID uuid.UUID, tools []protocol.Tool) {
	// Delete existing tools
	m.db.Where("session_id = ?", sessionID).Delete(&models.MCPTool{})

	// Insert new tools
	for _, tool := range tools {
		schemaJSON, _ := json.Marshal(tool.InputSchema)
		dbTool := &models.MCPTool{
			SessionID:       sessionID,
			ToolName:        tool.Name,
			ToolDescription: tool.Description,
			InputSchema:     schemaJSON,
		}
		m.db.Create(dbTool)
	}
}

func (m *Manager) updateResourcesCache(sessionID uuid.UUID, resources []protocol.Resource) {
	// Delete existing resources
	m.db.Where("session_id = ?", sessionID).Delete(&models.MCPResource{})

	// Insert new resources
	for _, resource := range resources {
		dbResource := &models.MCPResource{
			SessionID:           sessionID,
			ResourceURI:         resource.URI,
			ResourceName:        resource.Name,
			ResourceDescription: resource.Description,
			MimeType:            resource.MimeType,
		}
		m.db.Create(dbResource)
	}
}

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

