package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MCPResource represents a cached resource from an MCP server
type MCPResource struct {
	ID                  uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SessionID           uuid.UUID `gorm:"type:uuid;not null;index" json:"sessionId"`
	ResourceURI         string    `gorm:"size:1000;not null" json:"resourceUri"`
	ResourceName        string    `gorm:"size:500" json:"resourceName,omitempty"`
	ResourceDescription string    `gorm:"type:text" json:"resourceDescription,omitempty"`
	MimeType            string    `gorm:"size:255" json:"mimeType,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`

	// Associations
	Session MCPSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

// TableName specifies the table name for MCPResource model
func (MCPResource) TableName() string {
	return "mcp_resources"
}

// BeforeCreate hook to ensure ID is set
func (m *MCPResource) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// ResourceDefinition represents a resource as returned by resources/list
type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContent represents the content of a resource as returned by resources/read
type ResourceContent struct {
	URI      string                   `json:"uri"`
	MimeType string                   `json:"mimeType,omitempty"`
	Contents []ResourceContentItem    `json:"contents"`
}

// ResourceContentItem represents a single content item in a resource
type ResourceContentItem struct {
	Type     string `json:"type"` // "text", "image", etc.
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`     // base64 encoded for images
	MimeType string `json:"mimeType,omitempty"` // for embedded content
}

