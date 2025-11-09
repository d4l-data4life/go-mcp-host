package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/client"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/protocol"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// Manager manages MCP client sessions for conversations
type Manager struct {
	factory        *client.Factory
	serverConfigs  []config.MCPServerConfig
	sessions       map[string]*SessionInfo // key: conversationID:serverName
	mu             sync.RWMutex
	sessionTimeout time.Duration

	// Per-user ephemeral cache (no long-lived connections)
	userToolsCache     *cache.Cache // key: userID:server -> []protocol.Tool
	userResourcesCache *cache.Cache // key: userID:server -> []protocol.Resource
	userCacheTTL       time.Duration

	// Cache for server capability probes to avoid repeated initializes
	serverCapsCache *cache.Cache // key: serverName -> *protocol.ServerCapabilities

	// Prevent concurrent short-lived initializations to the same server (avoids session thrash)
	serverLocks map[string]*sync.Mutex
}

// SessionInfo holds information about an active MCP session
type SessionInfo struct {
	Client         *client.Client
	ConversationID uuid.UUID
	UserID         uuid.UUID
	ServerName     string
	ServerConfig   config.MCPServerConfig
	SessionID      uuid.UUID
	BearerToken    string // Bearer token used to create this session (for HTTP servers with forwardBearer)
	LastAccessed   time.Time
	mu             sync.RWMutex
}

// NewMCPManager creates a new MCP manager
func NewMCPManager(serverConfigs []config.MCPServerConfig) *Manager {
	factory := client.NewFactory("go-mcp-host", config.Version)

	m := &Manager{
		factory:            factory,
		serverConfigs:      serverConfigs,
		sessions:           make(map[string]*SessionInfo),
		sessionTimeout:     30 * time.Minute,
		userToolsCache:     cache.New(30*time.Minute, 10*time.Minute), // 30min TTL, 10min cleanup
		userResourcesCache: cache.New(30*time.Minute, 10*time.Minute), // 30min TTL, 10min cleanup
		userCacheTTL:       30 * time.Minute,
		serverCapsCache:    cache.New(10*time.Minute, 5*time.Minute), // probe results cached 10min
		serverLocks:        make(map[string]*sync.Mutex),
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m
}

// GetConfiguredServers returns the list of configured MCP servers
func (m *Manager) GetConfiguredServers() []config.MCPServerConfig {
	return m.serverConfigs
}

// ListAllToolsForUser returns all tools for all enabled servers, scoped by user (short-lived clients + cache)
func (m *Manager) ListAllToolsForUser(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ToolWithServer, error) {
	var results []ToolWithServer
	for _, server := range m.serverConfigs {
		if !server.Enabled {
			continue
		}
		key := m.getUserKey(userID, server.Name)

		// Check cache first
		if tools, found := m.userToolsCache.Get(key); found {
			if toolList, ok := tools.([]protocol.Tool); ok {
				logging.LogDebugf("Using cached tools for user %s server %s: %d tools", userID, server.Name, len(toolList))
				for _, t := range toolList {
					results = append(results, ToolWithServer{Tool: t, ServerName: server.Name})
				}
				continue
			}
		}

		// Fetch fresh via short-lived client
		logging.LogDebugf("Fetching fresh tools for user %s server %s", userID, server.Name)
		fetched, err := m.fetchToolsForUser(ctx, server, bearerToken)
		if err != nil {
			logging.LogWarningf(err, "Failed to fetch tools for server %s", server.Name)
			continue
		}

		// Cache the results
		m.userToolsCache.Set(key, fetched, m.userCacheTTL)
		logging.LogDebugf("Cached tools for user %s server %s: %d tools", userID, server.Name, len(fetched))

		for _, t := range fetched {
			results = append(results, ToolWithServer{Tool: t, ServerName: server.Name})
		}
	}
	return results, nil
}

// ListAllResourcesForUser returns all resources for all enabled servers, scoped by user (short-lived clients + cache)
func (m *Manager) ListAllResourcesForUser(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ResourceWithServer, error) {
	var results []ResourceWithServer
	for _, server := range m.serverConfigs {
		if !server.Enabled {
			continue
		}
		key := m.getUserKey(userID, server.Name)

		// Check cache first
		if resources, found := m.userResourcesCache.Get(key); found {
			if resourceList, ok := resources.([]protocol.Resource); ok {
				logging.LogDebugf("Using cached resources for user %s server %s: %d resources", userID, server.Name, len(resourceList))
				for _, r := range resourceList {
					results = append(results, ResourceWithServer{Resource: r, ServerName: server.Name})
				}
				continue
			}
		}

		// Fetch fresh via short-lived client
		logging.LogDebugf("Fetching fresh resources for user %s server %s", userID, server.Name)
		fetched, err := m.fetchResourcesForUser(ctx, server, bearerToken)
		if err != nil {
			logging.LogWarningf(err, "Failed to fetch resources for server %s", server.Name)
			continue
		}

		// Cache the results
		m.userResourcesCache.Set(key, fetched, m.userCacheTTL)
		logging.LogDebugf("Cached resources for user %s server %s: %d resources", userID, server.Name, len(fetched))

		for _, r := range fetched {
			results = append(results, ResourceWithServer{Resource: r, ServerName: server.Name})
		}
	}
	return results, nil
}

// ProbeServer performs a short-lived initialize to detect capabilities
func (m *Manager) ProbeServer(
	ctx context.Context,
	serverCfg config.MCPServerConfig,
	bearerToken string,
) (*protocol.ServerCapabilities, error) {
	// First, check cache
	if caps, found := m.serverCapsCache.Get(serverCfg.Name); found {
		if c, ok := caps.(*protocol.ServerCapabilities); ok {
			logging.LogDebugf("Using cached capabilities for server %s", serverCfg.Name)
			return c, nil
		}
	}

	lock := m.getServerLock(serverCfg.Name)
	lock.Lock()
	defer lock.Unlock()

	// Re-check after acquiring lock to avoid duplicate probe
	if caps, found := m.serverCapsCache.Get(serverCfg.Name); found {
		if c, ok := caps.(*protocol.ServerCapabilities); ok {
			logging.LogDebugf("Using cached capabilities for server %s", serverCfg.Name)
			return c, nil
		}
	}

	cli, err := m.createClientForUser(serverCfg, bearerToken)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	if err := cli.Initialize(ctx, client.ClientConfig{ClientName: "go-mcp-host", ClientVersion: config.Version, Capabilities: protocol.ClientCapabilities{Roots: &protocol.RootsCapability{ListChanged: true}, Sampling: map[string]interface{}{}}}); err != nil {
		return nil, err
	}
	caps := cli.GetCapabilities()
	// Cache probe result
	m.serverCapsCache.Set(serverCfg.Name, caps, cache.DefaultExpiration)
	return caps, nil
}

// Internal helpers for per-user fetches
func (m *Manager) fetchToolsForUser(ctx context.Context, serverCfg config.MCPServerConfig, bearerToken string) ([]protocol.Tool, error) {
	lock := m.getServerLock(serverCfg.Name)
	lock.Lock()
	defer lock.Unlock()

	cli, err := m.createClientForUser(serverCfg, bearerToken)
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	if err := cli.Initialize(ctx, client.ClientConfig{ClientName: "go-mcp-host", ClientVersion: config.Version, Capabilities: protocol.ClientCapabilities{Roots: &protocol.RootsCapability{ListChanged: true}, Sampling: map[string]interface{}{}}}); err != nil {
		return nil, err
	}
	return cli.ListTools(ctx)
}

func (m *Manager) fetchResourcesForUser(
	ctx context.Context,
	serverCfg config.MCPServerConfig,
	bearerToken string,
) ([]protocol.Resource, error) {
	lock := m.getServerLock(serverCfg.Name)
	lock.Lock()
	defer lock.Unlock()

	cli, err := m.createClientForUser(serverCfg, bearerToken)
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	if err := cli.Initialize(ctx, client.ClientConfig{ClientName: "go-mcp-host", ClientVersion: config.Version, Capabilities: protocol.ClientCapabilities{Roots: &protocol.RootsCapability{ListChanged: true}, Sampling: map[string]interface{}{}}}); err != nil {
		return nil, err
	}
	return cli.ListResources(ctx)
}

func (m *Manager) createClientForUser(serverCfg config.MCPServerConfig, bearerToken string) (*client.Client, error) {
	// Clone config and inject Authorization header when requested
	sc := serverCfg
	if sc.Type == "http" && sc.ForwardBearer && bearerToken != "" {
		if sc.Headers == nil {
			sc.Headers = map[string]string{}
		}
		sc.Headers["Authorization"] = "Bearer " + bearerToken
	}
	return m.factory.CreateClient(sc)
}

// GetOrCreateSession gets or creates an MCP session for a conversation and server
func (m *Manager) GetOrCreateSession(
	ctx context.Context,
	conversationID uuid.UUID,
	serverConfig config.MCPServerConfig,
	bearerToken string,
	userID uuid.UUID,
) (*SessionInfo, error) {
	// For HTTP servers with forwardBearer, don't cache sessions since bearer token may change
	// Instead, check if session exists AND verify it has the same bearer token
	if serverConfig.Type == "http" && serverConfig.ForwardBearer {
		sessionKey := m.getSessionKey(conversationID, serverConfig.Name)

		m.mu.RLock()
		if session, exists := m.sessions[sessionKey]; exists {
			// Check if bearer token matches
			session.mu.RLock()
			sessionToken := session.BearerToken
			session.mu.RUnlock()

			if sessionToken == bearerToken {
				// Token matches, reuse session
				session.mu.Lock()
				session.LastAccessed = time.Now()
				session.mu.Unlock()
				m.mu.RUnlock()
				logging.LogDebugf(
					"Reusing MCP session with same bearer token: conversation=%s server=%s",
					conversationID,
					serverConfig.Name,
				)
				return session, nil
			}

			// Token changed, need to recreate session
			m.mu.RUnlock()
			logging.LogDebugf("Bearer token changed, recreating MCP session: conversation=%s server=%s", conversationID, serverConfig.Name)

			// Close old session
			m.mu.Lock()
			if session, exists := m.sessions[sessionKey]; exists {
				delete(m.sessions, sessionKey)
				session.Client.Close()
			}
			m.mu.Unlock()
		} else {
			m.mu.RUnlock()
		}
	} else {
		// For non-bearer-forwarding servers, use normal caching
		sessionKey := m.getSessionKey(conversationID, serverConfig.Name)

		m.mu.RLock()
		if session, exists := m.sessions[sessionKey]; exists {
			session.mu.Lock()
			session.LastAccessed = time.Now()
			session.mu.Unlock()
			m.mu.RUnlock()
			return session, nil
		}
		m.mu.RUnlock()
	}

	sessionKey := m.getSessionKey(conversationID, serverConfig.Name)

	// Create new session
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock (only for non-bearer servers)
	if serverConfig.Type != "http" || !serverConfig.ForwardBearer {
		if session, exists := m.sessions[sessionKey]; exists {
			session.mu.Lock()
			session.LastAccessed = time.Now()
			session.mu.Unlock()
			return session, nil
		}
	}

	logging.LogDebugf(
		"Creating new MCP session: conversation=%s server=%s bearerToken=%v",
		conversationID,
		serverConfig.Name,
		bearerToken != "",
	)

	// Create MCP client with bearer if configured
	mcpClient, err := m.createClientForUser(serverConfig, bearerToken)
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
		UserID:         userID,
		ServerName:     serverConfig.Name,
		ServerConfig:   serverConfig,
		SessionID:      uuid.New(),  // Generate UUID for internal tracking
		BearerToken:    bearerToken, // Store bearer token for comparison
		LastAccessed:   time.Now(),
	}

	// Set up capability-gated notification handlers (no initial refresh to avoid extra list calls)
	caps := mcpClient.GetServerCapabilities()
	if caps != nil && caps.Tools != nil {
		mcpClient.SetOnToolsListChanged(func() {
			m.refreshTools(ctx, session)
		})
	}
	if caps != nil && caps.Resources != nil {
		mcpClient.SetOnResourcesListChanged(func() {
			m.refreshResources(ctx, session)
		})
	}

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

// CallTool calls a tool on the appropriate server
func (m *Manager) CallTool(
	ctx context.Context,
	conversationID uuid.UUID,
	serverName, toolName string,
	arguments map[string]interface{},
) (*protocol.CallToolResult, error) {
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

// ReadResource reads a resource from the appropriate server
func (m *Manager) ReadResource(
	ctx context.Context,
	conversationID uuid.UUID,
	serverName, resourceURI string,
) (*protocol.ReadResourceResult, error) {
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
	logging.LogDebugf("Refreshing tools for server %s", session.ServerName)
	tools, err := session.Client.ListTools(ctx)
	if err != nil {
		logging.LogErrorf(err, "Failed to refresh tools for server %s", session.ServerName)
		return
	}

	if session.UserID != uuid.Nil {
		cacheKey := m.getUserKey(session.UserID, session.ServerName)
		m.userToolsCache.Set(cacheKey, tools, m.userCacheTTL)
		logging.LogDebugf("Updated cached tools for user %s server %s: %d tools", session.UserID, session.ServerName, len(tools))
	}

	logging.LogDebugf("Refreshed tools for server %s: %d tools", session.ServerName, len(tools))
}

// refreshResources refreshes the resources list for a session
func (m *Manager) refreshResources(ctx context.Context, session *SessionInfo) {
	logging.LogDebugf("Refreshing resources for server %s", session.ServerName)
	resources, err := session.Client.ListResources(ctx)
	if err != nil {
		logging.LogErrorf(err, "Failed to refresh resources for server %s", session.ServerName)
		return
	}

	if session.UserID != uuid.Nil {
		cacheKey := m.getUserKey(session.UserID, session.ServerName)
		m.userResourcesCache.Set(cacheKey, resources, m.userCacheTTL)
		logging.LogDebugf(
			"Updated cached resources for user %s server %s: %d resources",
			session.UserID,
			session.ServerName,
			len(resources),
		)
	}

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

func (m *Manager) getUserKey(userID uuid.UUID, serverName string) string {
	key := fmt.Sprintf("%s:%s", userID.String(), serverName)
	logging.LogDebugf("Generated cache key: %s", key)
	return key
}

// getServerLock returns a per-server mutex to serialize short-lived client operations
func (m *Manager) getServerLock(serverName string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.serverLocks[serverName] == nil {
		m.serverLocks[serverName] = &sync.Mutex{}
	}
	return m.serverLocks[serverName]
}

// GetServerConfig returns the enabled configuration for the given server name.
func (m *Manager) GetServerConfig(serverName string) (config.MCPServerConfig, bool) {
	for _, server := range m.serverConfigs {
		if server.Enabled && server.Name == serverName {
			return server, true
		}
	}
	return config.MCPServerConfig{}, false
}

// GetCacheStats returns cache statistics for debugging
func (m *Manager) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"tools_cache_items":     m.userToolsCache.ItemCount(),
		"resources_cache_items": m.userResourcesCache.ItemCount(),
		"tools_cache_keys":      m.userToolsCache.Items(),
		"resources_cache_keys":  m.userResourcesCache.Items(),
	}
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
