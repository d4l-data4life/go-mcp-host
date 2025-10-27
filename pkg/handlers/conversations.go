package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/weese/go-mcp-host/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConversationsHandler handles conversation endpoints
type ConversationsHandler struct {
	db *gorm.DB
}

// NewConversationsHandler creates a new conversations handler
func NewConversationsHandler(db *gorm.DB) *ConversationsHandler {
	return &ConversationsHandler{
		db: db,
	}
}

// Routes returns conversation routes
func (h *ConversationsHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.ListConversations)
	r.Post("/", h.CreateConversation)
	r.Get("/{id}", h.GetConversation)
	r.Put("/{id}", h.UpdateConversation)
	r.Delete("/{id}", h.DeleteConversation)

	return r
}

// CreateConversationRequest represents a request to create a conversation
type CreateConversationRequest struct {
	Title        string `json:"title"`
	Model        string `json:"model"`
	SystemPrompt string `json:"systemPrompt"`
}

// UpdateConversationRequest represents a request to update a conversation
type UpdateConversationRequest struct {
	Title        string `json:"title"`
	SystemPrompt string `json:"systemPrompt"`
}

// ListConversations returns all conversations for the current user
func (h *ConversationsHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())

	var conversations []models.Conversation
	err := h.db.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&conversations).Error

	if err != nil {
		logging.LogErrorf(err, "Failed to list conversations")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to list conversations"})
		return
	}

	render.JSON(w, r, conversations)
}

// CreateConversation creates a new conversation
func (h *ConversationsHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())

	var req CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	// Set defaults
	if req.Title == "" {
		req.Title = "New Conversation"
	}
	if req.Model == "" {
		req.Model = "llama3.2"
	}

	// Create conversation
	conversation := models.Conversation{
		ID:           uuid.New(),
		UserID:       userID,
		Title:        req.Title,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
	}

	if err := h.db.Create(&conversation).Error; err != nil {
		logging.LogErrorf(err, "Failed to create conversation")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to create conversation"})
		return
	}

	logging.LogDebugf("Created conversation: %s for user: %s", conversation.ID, userID)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, conversation)
}

// GetConversation returns a specific conversation
func (h *ConversationsHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "id")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	var conversation models.Conversation
	err = h.db.Where("id = ? AND user_id = ?", convID, userID).
		First(&conversation).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "Conversation not found"})
		} else {
			logging.LogErrorf(err, "Failed to get conversation")
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{"error": "Failed to get conversation"})
		}
		return
	}

	render.JSON(w, r, conversation)
}

// UpdateConversation updates a conversation
func (h *ConversationsHandler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "id")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	var req UpdateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	// Get conversation
	var conversation models.Conversation
	err = h.db.Where("id = ? AND user_id = ?", convID, userID).
		First(&conversation).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "Conversation not found"})
		} else {
			logging.LogErrorf(err, "Failed to get conversation")
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{"error": "Failed to get conversation"})
		}
		return
	}

	// Update fields
	if req.Title != "" {
		conversation.Title = req.Title
	}
	if req.SystemPrompt != "" {
		conversation.SystemPrompt = req.SystemPrompt
	}

	if err := h.db.Save(&conversation).Error; err != nil {
		logging.LogErrorf(err, "Failed to update conversation")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to update conversation"})
		return
	}

	logging.LogDebugf("Updated conversation: %s", convID)

	render.JSON(w, r, conversation)
}

// DeleteConversation deletes a conversation (soft delete)
func (h *ConversationsHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "id")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	// Soft delete
	err = h.db.Where("id = ? AND user_id = ?", convID, userID).
		Delete(&models.Conversation{}).Error

	if err != nil {
		logging.LogErrorf(err, "Failed to delete conversation")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to delete conversation"})
		return
	}

	logging.LogDebugf("Deleted conversation: %s", convID)

	render.Status(r, http.StatusNoContent)
	w.Write([]byte{})
}

