package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MessageRole defines the possible roles for a message
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// Message represents a single message in a conversation
type Message struct {
	ID             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ConversationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversationId"`
	Role           MessageRole    `gorm:"size:20;not null;check:role IN ('user','assistant','system','tool')" json:"role"`
	Content        string         `gorm:"type:text" json:"content"`
	ToolCalls      datatypes.JSON `gorm:"type:jsonb" json:"toolCalls,omitempty"`
	ToolCallID     string         `gorm:"size:255" json:"toolCallId,omitempty"`
	Name           string         `gorm:"size:255" json:"name,omitempty"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`

	// Associations
	Conversation Conversation `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
}

// TableName specifies the table name for Message model
func (Message) TableName() string {
	return "messages"
}

// BeforeCreate hook to ensure ID is set
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function ToolCallFunction       `json:"function"`
}

// ToolCallFunction represents the function part of a tool call
type ToolCallFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

