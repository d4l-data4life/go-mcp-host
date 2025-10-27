package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MCPTool represents a cached tool from an MCP server
type MCPTool struct {
	ID              uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SessionID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"sessionId"`
	ToolName        string         `gorm:"size:255;not null" json:"toolName"`
	ToolDescription string         `gorm:"type:text" json:"toolDescription,omitempty"`
	InputSchema     datatypes.JSON `gorm:"type:jsonb;not null" json:"inputSchema"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`

	// Associations
	Session MCPSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

// TableName specifies the table name for MCPTool model
func (MCPTool) TableName() string {
	return "mcp_tools"
}

// BeforeCreate hook to ensure ID is set
func (m *MCPTool) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// ToolDefinition represents a tool as returned by tools/list
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

