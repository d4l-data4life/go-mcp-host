package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/models"

	"github.com/d4l-data4life/go-svc/pkg/logging"
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

	// Validate user ID from context
	if userID == uuid.Nil {
		logging.LogErrorf(nil, "User ID is nil in context")
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "Unauthorized - invalid user ID"})
		return
	}

	// Verify user exists
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logging.LogErrorf(nil, "User not found in database: %s", userID)
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{
				"error": "User not found - please log in again",
				"code":  "USER_NOT_FOUND",
			})
		} else {
			logging.LogErrorf(err, "Failed to verify user: %s", userID)
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{"error": "Failed to verify user"})
		}
		return
	}

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
		req.Model = viper.GetString("OPENAI_DEFAULT_MODEL")
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
		logging.LogErrorf(err, "Failed to create conversation for user: %s", userID)
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

// DeleteConversation deletes a conversation and all associated messages (immediate delete with CASCADE)
func (h *ConversationsHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "id")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	// Verify conversation exists and belongs to user
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

	// Immediate delete with CASCADE (messages will be deleted automatically due to foreign key constraint)
	err = h.db.Unscoped().Where("id = ? AND user_id = ?", convID, userID).
		Delete(&models.Conversation{}).Error

	if err != nil {
		logging.LogErrorf(err, "Failed to delete conversation")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to delete conversation"})
		return
	}

	logging.LogDebugf("Deleted conversation and associated messages: %s", convID)

	render.Status(r, http.StatusNoContent)
	_, _ = w.Write([]byte{})
}
