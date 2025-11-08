package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

const defaultListenRetryInterval = time.Second

// Option configures optional Manager behavior.
type Option func(*Manager)

// WithReconnectPolicy sets the reconnect attempt/delay behavior for MCP sessions.
func WithReconnectPolicy(attempts int, delay time.Duration) Option {
	return func(m *Manager) {
		if attempts < 0 {
			attempts = 0
		}
		m.maxReconnectAttempts = attempts
		if delay > 0 {
			m.reconnectDelay = delay
		}
	}
}

// Manager manages MCP client sessions for conversations
type Manager struct {
	serverConfigs      []config.MCPServerConfig
	sessions           map[string]*SessionInfo // key: conversationID:serverName
	mu                 sync.RWMutex
	sessionTimeout     time.Duration
	userToolsCache     *cache.Cache // key: userID:server -> []mcp.Tool
	userResourcesCache *cache.Cache // key: userID:server -> []mcp.Resource
	userCacheTTL       time.Duration
	serverCapsCache    *cache.Cache // key: serverName -> *mcp.ServerCapabilities
	serverLocks        map[string]*sync.Mutex

	userServerLocks map[string]*sync.Mutex // key: userID:serverName

	clientName           string
	clientVersion        string
	clientCapabilities   mcp.ClientCapabilities
	maxReconnectAttempts int
	reconnectDelay       time.Duration
}

// SessionInfo holds information about an active MCP session
type SessionInfo struct {
	Client           *mcpclient.Client
	ConversationID   uuid.UUID
	UserID           uuid.UUID
	ServerName       string
	ServerConfig     config.MCPServerConfig
	SessionID        uuid.UUID
	BearerTokenHash  string // Hash of bearer token used to create this session
	LastAccessed     time.Time
	mu               sync.RWMutex
	reconnectTracker *reconnectTracker
}

// NewMCPManager creates a new MCP manager
func NewMCPManager(serverConfigs []config.MCPServerConfig, opts ...Option) *Manager {
	clientCaps := mcp.ClientCapabilities{
		Roots: &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			ListChanged: true,
		},
		Sampling: &struct{}{},
	}

	m := &Manager{
		serverConfigs:        serverConfigs,
		sessions:             make(map[string]*SessionInfo),
		sessionTimeout:       30 * time.Minute,
		userToolsCache:       cache.New(30*time.Minute, 10*time.Minute),
		userResourcesCache:   cache.New(30*time.Minute, 10*time.Minute),
		userCacheTTL:         30 * time.Minute,
		serverCapsCache:      cache.New(10*time.Minute, 5*time.Minute),
		serverLocks:          make(map[string]*sync.Mutex),
		userServerLocks:      make(map[string]*sync.Mutex),
		clientName:           "go-mcp-host",
		clientVersion:        config.Version,
		clientCapabilities:   clientCaps,
		maxReconnectAttempts: 0,
		reconnectDelay:       defaultListenRetryInterval,
	}

	for _, opt := range opts {
		opt(m)
	}

	go m.cleanupLoop()

	return m
}

// GetConfiguredServers returns the list of configured MCP servers
func (m *Manager) GetConfiguredServers() []config.MCPServerConfig {
	return m.serverConfigs
}

// ListAllToolsForUser returns all tools for all enabled servers, scoped by user (short-lived clients + cache)
func (m *Manager) ListAllToolsForUser(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ToolWithServer, error) {
	var (
		results []ToolWithServer
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for _, server := range m.serverConfigs {
		if !server.Enabled {
			continue
		}

		server := server
		wg.Add(1)
		go func() {
			defer wg.Done()

			key := m.getUserKey(userID, server.Name)

			if tools, found := m.userToolsCache.Get(key); found {
				if toolList, ok := tools.([]mcp.Tool); ok {
					logging.LogDebugf("Using cached tools for user %s server %s: %d tools", userID, server.Name, len(toolList))
					mu.Lock()
					for _, t := range toolList {
						results = append(results, ToolWithServer{Tool: t, ServerName: server.Name})
					}
					mu.Unlock()
					return
				}
			}

			logging.LogDebugf("Fetching fresh tools for user %s server %s", userID, server.Name)
			fetched, err := m.fetchToolsForUser(ctx, userID, server, bearerToken)
			if err != nil {
				logging.LogWarningf(err, "Failed to fetch tools for server %s", server.Name)
				return
			}

			m.userToolsCache.Set(key, fetched, m.userCacheTTL)
			logging.LogDebugf("Cached tools for user %s server %s: %d tools", userID, server.Name, len(fetched))

			mu.Lock()
			for _, t := range fetched {
				results = append(results, ToolWithServer{Tool: t, ServerName: server.Name})
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results, nil
}

// ListAllResourcesForUser returns all resources for all enabled servers, scoped by user (short-lived clients + cache)
func (m *Manager) ListAllResourcesForUser(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ResourceWithServer, error) {
	var (
		results []ResourceWithServer
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for _, server := range m.serverConfigs {
		if !server.Enabled {
			continue
		}

		server := server
		wg.Add(1)
		go func() {
			defer wg.Done()

			key := m.getUserKey(userID, server.Name)

			if resources, found := m.userResourcesCache.Get(key); found {
				if resourceList, ok := resources.([]mcp.Resource); ok {
					logging.LogDebugf("Using cached resources for user %s server %s: %d resources", userID, server.Name, len(resourceList))
					mu.Lock()
					for _, r := range resourceList {
						results = append(results, ResourceWithServer{Resource: r, ServerName: server.Name})
					}
					mu.Unlock()
					return
				}
			}

			logging.LogDebugf("Fetching fresh resources for user %s server %s", userID, server.Name)
			fetched, err := m.fetchResourcesForUser(ctx, userID, server, bearerToken)
			if err != nil {
				logging.LogWarningf(err, "Failed to fetch resources for server %s", server.Name)
				return
			}

			m.userResourcesCache.Set(key, fetched, m.userCacheTTL)
			logging.LogDebugf("Cached resources for user %s server %s: %d resources", userID, server.Name, len(fetched))

			mu.Lock()
			for _, r := range fetched {
				results = append(results, ResourceWithServer{Resource: r, ServerName: server.Name})
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results, nil
}

// ProbeServer performs a short-lived initialize to detect capabilities
func (m *Manager) ProbeServer(
	ctx context.Context,
	serverCfg config.MCPServerConfig,
	bearerToken string,
) (*mcp.ServerCapabilities, error) {
	if caps, found := m.serverCapsCache.Get(serverCfg.Name); found {
		if c, ok := caps.(*mcp.ServerCapabilities); ok {
			logging.LogDebugf("Using cached capabilities for server %s", serverCfg.Name)
			return c, nil
		}
	}

	lock := m.getServerLock(serverCfg.Name)
	lock.Lock()
	defer lock.Unlock()

	if caps, found := m.serverCapsCache.Get(serverCfg.Name); found {
		if c, ok := caps.(*mcp.ServerCapabilities); ok {
			logging.LogDebugf("Using cached capabilities for server %s", serverCfg.Name)
			return c, nil
		}
	}

	cli, initResult, err := m.newInitializedClient(ctx, serverCfg, bearerToken, nil)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	capsCopy := initResult.Capabilities
	m.serverCapsCache.Set(serverCfg.Name, &capsCopy, cache.DefaultExpiration)
	return &capsCopy, nil
}

func (m *Manager) fetchToolsForUser(ctx context.Context, userID uuid.UUID, serverCfg config.MCPServerConfig, bearerToken string) ([]mcp.Tool, error) {
	lock := m.getUserServerLock(userID, serverCfg.Name)
	lock.Lock()
	defer lock.Unlock()

	cli, _, err := m.newInitializedClient(ctx, serverCfg, bearerToken, nil)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	result, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (m *Manager) fetchResourcesForUser(
	ctx context.Context,
	userID uuid.UUID,
	serverCfg config.MCPServerConfig,
	bearerToken string,
) ([]mcp.Resource, error) {
	lock := m.getUserServerLock(userID, serverCfg.Name)
	lock.Lock()
	defer lock.Unlock()

	cli, _, err := m.newInitializedClient(ctx, serverCfg, bearerToken, nil)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	result, err := cli.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		return nil, err
	}
	return result.Resources, nil
}

func (m *Manager) newInitializedClient(
	ctx context.Context,
	serverCfg config.MCPServerConfig,
	bearerToken string,
	tracker *reconnectTracker,
) (*mcpclient.Client, *mcp.InitializeResult, error) {
	trans, err := m.createTransport(serverCfg, bearerToken, tracker)
	if err != nil {
		return nil, nil, err
	}

	client := mcpclient.NewClient(trans, mcpclient.WithClientCapabilities(m.clientCapabilities))
	if err := client.Start(ctx); err != nil {
		return nil, nil, err
	}

	initResult, err := client.Initialize(ctx, m.buildInitializeRequest())
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, initResult, nil
}

// GetOrCreateSession gets or creates an MCP session for a conversation and server
func (m *Manager) GetOrCreateSession(
	ctx context.Context,
	conversationID uuid.UUID,
	serverConfig config.MCPServerConfig,
	bearerToken string,
	userID uuid.UUID,
) (*SessionInfo, error) {
	if serverConfig.Type == "http" && serverConfig.ForwardBearer {
		sessionKey := m.getSessionKey(conversationID, serverConfig.Name)
		incomingHash := hashToken(bearerToken)

		m.mu.RLock()
		if session, exists := m.sessions[sessionKey]; exists {
			session.mu.RLock()
			sessionToken := session.BearerTokenHash
			session.mu.RUnlock()

			if sessionToken == incomingHash {
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

			m.mu.RUnlock()
			logging.LogDebugf("Bearer token changed, recreating MCP session: conversation=%s server=%s", conversationID, serverConfig.Name)

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

	m.mu.Lock()
	defer m.mu.Unlock()

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

	var tracker *reconnectTracker
	if serverConfig.Type == "http" && (m.maxReconnectAttempts > 0 || m.reconnectDelay != defaultListenRetryInterval) {
		tracker = newReconnectTracker(m, conversationID, serverConfig.Name)
	}

	mcpClient, initResult, err := m.newInitializedClient(ctx, serverConfig, bearerToken, tracker)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create MCP client for server %s", serverConfig.Name)
	}

	tokenHash := hashToken(bearerToken)
	session := &SessionInfo{
		Client:           mcpClient,
		ConversationID:   conversationID,
		UserID:           userID,
		ServerName:       serverConfig.Name,
		ServerConfig:     serverConfig,
		SessionID:        uuid.New(),
		BearerTokenHash:  tokenHash,
		LastAccessed:     time.Now(),
		reconnectTracker: tracker,
	}

	m.registerNotificationHandlers(session)

	caps := initResult.Capabilities
	if caps.Tools != nil {
		go m.refreshTools(context.Background(), session)
	}
	if caps.Resources != nil {
		go m.refreshResources(context.Background(), session)
	}

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

	if session.reconnectTracker != nil {
		session.reconnectTracker.markClosed()
	}

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

			if session.reconnectTracker != nil {
				session.reconnectTracker.markClosed()
			}

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
) (*mcp.CallToolResult, error) {
	session, exists := m.GetSession(conversationID, serverName)
	if !exists {
		return nil, errors.Errorf("no active session for server %s", serverName)
	}

	session.mu.Lock()
	session.LastAccessed = time.Now()
	session.mu.Unlock()

	result, err := session.Client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call tool %s on server %s", toolName, serverName)
	}

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
) (*mcp.ReadResourceResult, error) {
	session, exists := m.GetSession(conversationID, serverName)
	if !exists {
		return nil, errors.Errorf("no active session for server %s", serverName)
	}

	session.mu.Lock()
	session.LastAccessed = time.Now()
	session.mu.Unlock()

	result, err := session.Client.ReadResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: resourceURI,
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read resource %s from server %s", resourceURI, serverName)
	}

	session.mu.Lock()
	session.LastAccessed = time.Now()
	session.mu.Unlock()

	return result, nil
}

func (m *Manager) refreshTools(ctx context.Context, session *SessionInfo) {
	if ctx == nil {
		ctx = context.Background()
	}

	logging.LogDebugf("Refreshing tools for server %s", session.ServerName)
	result, err := session.Client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		logging.LogErrorf(err, "Failed to refresh tools for server %s", session.ServerName)
		return
	}

	if session.UserID != uuid.Nil {
		cacheKey := m.getUserKey(session.UserID, session.ServerName)
		var tools []mcp.Tool
		if result != nil {
			tools = result.Tools
		}
		m.userToolsCache.Set(cacheKey, tools, m.userCacheTTL)
		logging.LogDebugf(
			"Updated cached tools for user %s server %s: %d tools",
			session.UserID,
			session.ServerName,
			len(tools),
		)
	}

	toolCount := 0
	if result != nil {
		toolCount = len(result.Tools)
	}
	logging.LogDebugf("Refreshed tools for server %s: %d tools", session.ServerName, toolCount)
}

func (m *Manager) refreshResources(ctx context.Context, session *SessionInfo) {
	if ctx == nil {
		ctx = context.Background()
	}

	logging.LogDebugf("Refreshing resources for server %s", session.ServerName)
	result, err := session.Client.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		logging.LogErrorf(err, "Failed to refresh resources for server %s", session.ServerName)
		return
	}

	if session.UserID != uuid.Nil {
		cacheKey := m.getUserKey(session.UserID, session.ServerName)
		var resources []mcp.Resource
		if result != nil {
			resources = result.Resources
		}
		m.userResourcesCache.Set(cacheKey, resources, m.userCacheTTL)
		logging.LogDebugf(
			"Updated cached resources for user %s server %s: %d resources",
			session.UserID,
			session.ServerName,
			len(resources),
		)
	}

	resourceCount := 0
	if result != nil {
		resourceCount = len(result.Resources)
	}
	logging.LogDebugf("Refreshed resources for server %s: %d resources", session.ServerName, resourceCount)
}

func (m *Manager) registerNotificationHandlers(session *SessionInfo) {
	session.Client.OnNotification(func(notification mcp.JSONRPCNotification) {
		switch notification.Notification.Method {
		case string(mcp.MethodNotificationToolsListChanged):
			go m.refreshTools(context.Background(), session)
		case string(mcp.MethodNotificationResourcesListChanged), string(mcp.MethodNotificationResourceUpdated):
			go m.refreshResources(context.Background(), session)
		}
	})
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupInactiveSessions()
	}
}

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
			if session.reconnectTracker != nil {
				session.reconnectTracker.markClosed()
			}
			session.Client.Close()
			logging.LogDebugf("Cleaned up inactive MCP session: conversation=%s server=%s", session.ConversationID, session.ServerName)
		}
	}
}

func (m *Manager) getSessionKey(conversationID uuid.UUID, serverName string) string {
	return fmt.Sprintf("%s:%s", conversationID.String(), serverName)
}

func (m *Manager) getUserKey(userID uuid.UUID, serverName string) string {
	key := fmt.Sprintf("%s:%s", userID.String(), serverName)
	// logging.LogDebugf("Generated cache key: %s", key)
	return key
}

func (m *Manager) getServerLock(serverName string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.serverLocks[serverName] == nil {
		m.serverLocks[serverName] = &sync.Mutex{}
	}
	return m.serverLocks[serverName]
}

func (m *Manager) getUserServerLock(userID uuid.UUID, serverName string) *sync.Mutex {
	key := fmt.Sprintf("%s:%s", userID.String(), serverName)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.userServerLocks[key] == nil {
		m.userServerLocks[key] = &sync.Mutex{}
	}
	return m.userServerLocks[key]
}

// GetServerConfig returns the configured MCP server entry by name.
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

// ToolWithServer associates a tool with its server
type ToolWithServer struct {
	Tool       mcp.Tool
	ServerName string
}

// ResourceWithServer associates a resource with its server
type ResourceWithServer struct {
	Resource   mcp.Resource
	ServerName string
}

func (m *Manager) buildInitializeRequest() mcp.InitializeRequest {
	return mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    m.clientName,
				Version: m.clientVersion,
			},
			Capabilities: m.clientCapabilities,
		},
	}
}

func (m *Manager) createTransport(
	serverCfg config.MCPServerConfig,
	bearerToken string,
	tracker *reconnectTracker,
) (mcptransport.Interface, error) {
	switch serverCfg.Type {
	case "stdio":
		if serverCfg.Command == "" {
			return nil, errors.New("stdio transport requires command")
		}
		envSlice := envMapToSlice(serverCfg.Env)
		args := append([]string(nil), serverCfg.Args...)
		return mcptransport.NewStdioWithOptions(serverCfg.Command, envSlice, args), nil
	case "http":
		if serverCfg.URL == "" {
			return nil, errors.New("http transport requires URL")
		}
		mode := serverCfg.Mode
		if mode == "" {
			mode = config.HTTPServerModeBatch
		}
		headers := cloneHeaders(serverCfg.Headers)
		if serverCfg.ForwardBearer && bearerToken != "" {
			headers["Authorization"] = "Bearer " + bearerToken
		}
		logger := m.newTransportLogger(serverCfg.Name, tracker)

		switch mode {
		case config.HTTPServerModeBatch:
			opts := []mcptransport.StreamableHTTPCOption{
				mcptransport.WithHTTPHeaders(headers),
				mcptransport.WithContinuousListening(),
			}
			if logger != nil {
				opts = append(opts, mcptransport.WithHTTPLogger(logger))
			}
			return mcptransport.NewStreamableHTTP(serverCfg.URL, opts...)
		case config.HTTPServerModeStream:
			var opts []mcptransport.ClientOption
			if len(headers) > 0 {
				opts = append(opts, mcptransport.WithHeaders(headers))
			}
			if logger != nil {
				opts = append(opts, mcptransport.WithSSELogger(logger))
			}
			return mcptransport.NewSSE(serverCfg.URL, opts...)
		default:
			return nil, errors.Errorf("unsupported http mode %q for server %s", serverCfg.Mode, serverCfg.Name)
		}
	default:
		return nil, errors.Errorf("unsupported transport type: %s", serverCfg.Type)
	}
}

func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	result := make([]string, 0, len(env))
	for key, value := range env {
		result = append(result, key+"="+value)
	}
	return result
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for k, v := range headers {
		cloned[k] = v
	}
	return cloned
}

func hashToken(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) newTransportLogger(serverName string, tracker *reconnectTracker) *transportLogger {
	return &transportLogger{
		serverName: serverName,
		tracker:    tracker,
	}
}
