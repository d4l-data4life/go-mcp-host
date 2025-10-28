package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/weese/go-mcp-host/pkg/mcp/manager"
	"gorm.io/gorm"

	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// MCPServersHandler handles MCP server endpoints
type MCPServersHandler struct {
	db         *gorm.DB
	mcpManager *manager.Manager
}

// NewMCPServersHandler creates a new MCP servers handler
func NewMCPServersHandler(db *gorm.DB, mcpManager *manager.Manager) *MCPServersHandler {
	return &MCPServersHandler{
		db:         db,
		mcpManager: mcpManager,
	}
}

// Routes returns MCP server routes
func (h *MCPServersHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/{conversationId}/servers", h.ListServers)
	r.Get("/{conversationId}/tools", h.ListTools)
	r.Get("/{conversationId}/resources", h.ListResources)

	return r
}

// ServerInfo represents information about an MCP server
type ServerInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	Enabled      bool     `json:"enabled"`
	Capabilities []string `json:"capabilities"`
	Connected    bool     `json:"connected"`
}

// ToolInfo represents information about an MCP tool
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Server      string                 `json:"server"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ResourceInfo represents information about an MCP resource
type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
	Server      string `json:"server"`
}

// ListServers returns all configured MCP servers and their status
func (h *MCPServersHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "conversationId")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	// Verify conversation belongs to user
	var count int64
	h.db.Table("conversations").
		Where("id = ? AND user_id = ?", convID, userID).
		Count(&count)

	if count == 0 {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "Conversation not found"})
		return
	}

	// Get configured servers
	servers := h.mcpManager.GetConfiguredServers()

	var serverInfos []ServerInfo
	for _, cfg := range servers {
		info := ServerInfo{
			Name:         cfg.Name,
			Description:  cfg.Description,
			Type:         cfg.Type,
			Enabled:      cfg.Enabled,
			Capabilities: []string{},
			Connected:    false,
		}

		// Try to get session info for this server
		session, exists := h.mcpManager.GetSession(convID, cfg.Name)
		if exists && session != nil && session.Client != nil {
			info.Connected = true
			// Get capabilities
			caps := session.Client.GetCapabilities()
			if caps.Tools != nil {
				info.Capabilities = append(info.Capabilities, "tools")
			}
			if caps.Resources != nil {
				info.Capabilities = append(info.Capabilities, "resources")
			}
			if caps.Prompts != nil {
				info.Capabilities = append(info.Capabilities, "prompts")
			}
		}

		serverInfos = append(serverInfos, info)
	}

	render.JSON(w, r, serverInfos)
}

// ListTools returns all available tools from MCP servers
func (h *MCPServersHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "conversationId")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	// Verify conversation belongs to user
	var count int64
	h.db.Table("conversations").
		Where("id = ? AND user_id = ?", convID, userID).
		Count(&count)

	if count == 0 {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "Conversation not found"})
		return
	}

	// Get tools from MCP manager
	tools, err := h.mcpManager.ListAllTools(r.Context(), convID, "")
	if err != nil {
		logging.LogErrorf(err, "Failed to list tools")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to list tools"})
		return
	}

	var toolInfos []ToolInfo
	for _, tool := range tools {
		toolInfos = append(toolInfos, ToolInfo{
			Name:        tool.Tool.Name,
			Description: tool.Tool.Description,
			Server:      tool.ServerName,
			InputSchema: tool.Tool.InputSchema,
		})
	}

	render.JSON(w, r, toolInfos)
}

// ListResources returns all available resources from MCP servers
func (h *MCPServersHandler) ListResources(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "conversationId")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	// Verify conversation belongs to user
	var count int64
	h.db.Table("conversations").
		Where("id = ? AND user_id = ?", convID, userID).
		Count(&count)

	if count == 0 {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "Conversation not found"})
		return
	}

	// Get resources from MCP manager
	resources, err := h.mcpManager.ListAllResources(r.Context(), convID, "")
	if err != nil {
		logging.LogErrorf(err, "Failed to list resources")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to list resources"})
		return
	}

	var resourceInfos []ResourceInfo
	for _, resource := range resources {
		resourceInfos = append(resourceInfos, ResourceInfo{
			URI:         resource.Resource.URI,
			Name:        resource.Resource.Name,
			Description: resource.Resource.Description,
			MimeType:    resource.Resource.MimeType,
			Server:      resource.ServerName,
		})
	}

	render.JSON(w, r, resourceInfos)
}
