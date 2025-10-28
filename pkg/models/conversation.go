package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Conversation represents a chat conversation
type Conversation struct {
	ID           uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID       uuid.UUID      `gorm:"type:uuid;not null;index;constraint:OnDelete:CASCADE" json:"userId"`
	Title        string         `gorm:"size:500;not null;default:'New Conversation'" json:"title"`
	Model        string         `gorm:"size:100;not null;default:'llama3.2'" json:"model"`
	SystemPrompt string         `gorm:"type:text" json:"systemPrompt,omitempty"`
	Metadata     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`

	// Associations
	User     User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Messages []Message `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
	// MCP sessions are now kept in-memory only
}

// TableName specifies the table name for Conversation model
func (Conversation) TableName() string {
	return "conversations"
}

// BeforeCreate hook to ensure ID is set
func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// ConversationSummary represents a lightweight conversation for listing
type ConversationSummary struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"userId"`
	Title         string    `json:"title"`
	Model         string    `json:"model"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	MessageCount  int       `json:"messageCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
