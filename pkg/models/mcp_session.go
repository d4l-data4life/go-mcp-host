package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MCPSessionStatus defines the possible statuses for an MCP session
type MCPSessionStatus string

const (
	MCPSessionStatusInitializing MCPSessionStatus = "initializing"
	MCPSessionStatusConnected    MCPSessionStatus = "connected"
	MCPSessionStatusDisconnected MCPSessionStatus = "disconnected"
	MCPSessionStatusError        MCPSessionStatus = "error"
)

// MCPSessionType defines the transport type for an MCP session
type MCPSessionType string

const (
	MCPSessionTypeStdio MCPSessionType = "stdio"
	MCPSessionTypeHTTP  MCPSessionType = "http"
)

// MCPSession represents an active connection to an MCP server
type MCPSession struct {
	ID             uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ConversationID uuid.UUID        `gorm:"type:uuid;not null;index" json:"conversationId"`
	ServerName     string           `gorm:"size:255;not null" json:"serverName"`
	ServerType     MCPSessionType   `gorm:"size:50;not null;check:server_type IN ('stdio','http')" json:"serverType"`
	ConnectionInfo datatypes.JSON   `gorm:"type:jsonb;not null" json:"connectionInfo"`
	Capabilities   datatypes.JSON   `gorm:"type:jsonb" json:"capabilities,omitempty"`
	Status         MCPSessionStatus `gorm:"size:50;not null;default:'initializing';check:status IN ('initializing','connected','disconnected','error')" json:"status"`
	LastActiveAt   time.Time        `gorm:"not null;default:NOW()" json:"lastActiveAt"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`

	// Associations
	Conversation Conversation  `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
	Tools        []MCPTool     `gorm:"foreignKey:SessionID" json:"tools,omitempty"`
	Resources    []MCPResource `gorm:"foreignKey:SessionID" json:"resources,omitempty"`
}

// TableName specifies the table name for MCPSession model
func (MCPSession) TableName() string {
	return "mcp_sessions"
}

// BeforeCreate hook to ensure ID is set
func (m *MCPSession) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// StdioConnectionInfo represents connection info for stdio transport
type StdioConnectionInfo struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env,omitempty"`
}

// HTTPConnectionInfo represents connection info for HTTP transport
type HTTPConnectionInfo struct {
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	BearerToken string            `json:"bearerToken,omitempty"`
}

// MCPCapabilities represents the capabilities of an MCP server
type MCPCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// ToolsCapability represents tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents resources capability
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability represents prompts capability
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability represents logging capability
type LoggingCapability struct {
	// Empty struct means capability is supported
}

