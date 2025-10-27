package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/weese/go-mcp-host/pkg/agent"
	"github.com/weese/go-mcp-host/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MessagesHandler handles message endpoints
type MessagesHandler struct {
	db       *gorm.DB
	agent    *agent.Agent
	upgrader websocket.Upgrader
}

// NewMessagesHandler creates a new messages handler
func NewMessagesHandler(db *gorm.DB, agent *agent.Agent) *MessagesHandler {
	return &MessagesHandler{
		db:    db,
		agent: agent,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
	}
}

// Routes returns message routes
func (h *MessagesHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.ListMessages)
	r.Post("/", h.SendMessage)
	r.Get("/stream", h.StreamMessages)

	return r
}

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	Content string `json:"content"`
}

// SendMessageResponse represents the response to sending a message
type SendMessageResponse struct {
	UserMessage      models.Message `json:"userMessage"`
	AssistantMessage models.Message `json:"assistantMessage"`
	Iterations       int            `json:"iterations"`
	ToolsUsed        int            `json:"toolsUsed"`
}

// ListMessages returns all messages in a conversation
func (h *MessagesHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "id")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	// Verify conversation belongs to user
	var conversation models.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conversation).Error; err != nil {
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

	// Get messages
	var messages []models.Message
	err = h.db.Where("conversation_id = ?", convID).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		logging.LogErrorf(err, "Failed to list messages")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to list messages"})
		return
	}

	render.JSON(w, r, messages)
}

// SendMessage sends a message and gets agent response
func (h *MessagesHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "id")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid conversation ID"})
		return
	}

	// Verify conversation belongs to user
	var conversation models.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conversation).Error; err != nil {
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

	// Parse request
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Content == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Message content is required"})
		return
	}

	// Save user message
	userMessage := models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           models.MessageRoleUser,
		Content:        req.Content,
	}

	if err := h.db.Create(&userMessage).Error; err != nil {
		logging.LogErrorf(err, "Failed to save user message")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to save message"})
		return
	}

	// Get message history
	var messages []models.Message
	h.db.Where("conversation_id = ?", convID).
		Order("created_at ASC").
		Find(&messages)

	// TODO: Convert message history to agent format
	// agentMessages := h.convertToAgentMessages(messages)

	// Call agent
	response, err := h.agent.Chat(r.Context(), agent.ChatRequest{
		ConversationID: convID,
		UserMessage:    req.Content,
		Messages:       nil, // Use nil instead of agentMessages for now
		Model:          conversation.Model,
	})

	if err != nil {
		logging.LogErrorf(err, "Agent failed")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Agent failed to process message"})
		return
	}

	// Save assistant message
	toolCallsJSON, _ := json.Marshal(response.Message.ToolCalls)
	assistantMessage := models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           models.MessageRoleAssistant,
		Content:        response.Message.Content,
		ToolCalls:      datatypes.JSON(toolCallsJSON),
	}

	if err := h.db.Create(&assistantMessage).Error; err != nil {
		logging.LogErrorf(err, "Failed to save assistant message")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to save response"})
		return
	}

	logging.LogDebugf("Message processed: conversation=%s iterations=%d tools=%d",
		convID, response.Iterations, len(response.ToolsUsed))

	render.JSON(w, r, SendMessageResponse{
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		Iterations:       response.Iterations,
		ToolsUsed:        len(response.ToolsUsed),
	})
}

// StreamMessages handles streaming message responses via WebSocket
func (h *MessagesHandler) StreamMessages(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "id")

	convID, err := uuid.Parse(conversationID)
	if err != nil {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	// Verify conversation belongs to user
	var conversation models.Conversation
	if err := h.db.Where("id = ? AND user_id = ?", convID, userID).First(&conversation).Error; err != nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.LogErrorf(err, "Failed to upgrade to WebSocket")
		return
	}
	defer conn.Close()

	logging.LogDebugf("WebSocket connection established: conversation=%s user=%s", convID, userID)

	// Handle WebSocket messages
	for {
		// Read message from client
		var req SendMessageRequest
		err := conn.ReadJSON(&req)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logging.LogDebugf("WebSocket closed normally")
			} else {
				logging.LogErrorf(err, "WebSocket read error")
			}
			break
		}

		if req.Content == "" {
			conn.WriteJSON(map[string]string{"error": "Message content is required"})
			continue
		}

		// Save user message
		userMessage := models.Message{
			ID:             uuid.New(),
			ConversationID: convID,
			Role:           models.MessageRoleUser,
			Content:        req.Content,
		}
		h.db.Create(&userMessage)

		// Send user message confirmation
		conn.WriteJSON(map[string]interface{}{
			"type":    "user_message",
			"message": userMessage,
		})

		// Get message history
		var messages []models.Message
		h.db.Where("conversation_id = ?", convID).
			Order("created_at ASC").
			Find(&messages)

		// TODO: Convert message history to agent format
		// agentMessages := h.convertToAgentMessages(messages)

		// Stream agent response
		streamChan, err := h.agent.ChatStream(context.Background(), agent.ChatRequest{
			ConversationID: convID,
			UserMessage:    req.Content,
			Messages:       nil, // Use nil instead of agentMessages for now
			Model:          conversation.Model,
		})

		if err != nil {
			conn.WriteJSON(map[string]string{"error": "Failed to start agent"})
			continue
		}

		var fullContent string
		for event := range streamChan {
			switch event.Type {
			case agent.StreamEventTypeContent:
				fullContent += event.Content
				conn.WriteJSON(map[string]interface{}{
					"type":    "content",
					"content": event.Content,
				})
			case agent.StreamEventTypeToolStart:
				conn.WriteJSON(map[string]interface{}{
					"type": "tool_start",
					"tool": event.Tool,
				})
			case agent.StreamEventTypeToolComplete:
				conn.WriteJSON(map[string]interface{}{
					"type": "tool_complete",
					"tool": event.Tool,
				})
			case agent.StreamEventTypeDone:
				// Save assistant message
				assistantMessage := models.Message{
					ID:             uuid.New(),
					ConversationID: convID,
					Role:           models.MessageRoleAssistant,
					Content:        fullContent,
				}
				h.db.Create(&assistantMessage)

				conn.WriteJSON(map[string]interface{}{
					"type":    "done",
					"message": assistantMessage,
				})
			case agent.StreamEventTypeError:
				conn.WriteJSON(map[string]interface{}{
					"type":  "error",
					"error": event.Error.Error(),
				})
			}
		}
	}
}

// convertToAgentMessages converts database messages to agent messages
func (h *MessagesHandler) convertToAgentMessages(dbMessages []models.Message) []map[string]interface{} {
	// Return empty for now - agent will use last message
	// In production, convert message history to proper format
	return nil
}

