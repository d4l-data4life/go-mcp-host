package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/agent"
	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"

	"github.com/d4l-data4life/go-svc/pkg/logging"
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
	Content   string     `json:"content"`
	MessageID *uuid.UUID `json:"messageId,omitempty"` // If present, edit/retry existing message
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

	// Convert message history to agent format
	agentMessages := h.convertToAgentMessages(messages)

	// Call agent
	response, err := h.agent.Chat(r.Context(), agent.ChatRequest{
		ConversationID: convID,
		UserID:         userID,
		BearerToken:    GetBearerTokenFromContext(r.Context()),
		UserMessage:    req.Content,
		Messages:       agentMessages,
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
	// Attach tool execution metadata for frontend display
	toolExecs := make([]map[string]interface{}, 0, len(response.ToolsUsed))
	for _, te := range response.ToolsUsed {
		entry := map[string]interface{}{
			"serverName": te.ServerName,
			"toolName":   te.ToolName,
			"arguments":  te.Arguments,
			"result":     te.Result,
			"durationMs": te.Duration.Milliseconds(),
		}
		if te.Error != nil {
			entry["error"] = te.Error.Error()
		}
		toolExecs = append(toolExecs, entry)
	}
	metaJSON, _ := json.Marshal(map[string]interface{}{
		"toolExecutions": toolExecs,
	})
	assistantMessage := models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           models.MessageRoleAssistant,
		Content:        response.Message.Content,
		ToolCalls:      datatypes.JSON(toolCallsJSON),
		Metadata:       datatypes.JSON(metaJSON),
	}

	if err := h.db.Create(&assistantMessage).Error; err != nil {
		logging.LogErrorf(err, "Failed to save assistant message")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to save response"})
		return
	}

	// Auto-generate conversation title if this is the first message and title is still default
	if conversation.Title == "New Chat" || conversation.Title == "New Conversation" {
		// Count total messages (should be 2: user + assistant)
		var messageCount int64
		h.db.Model(&models.Message{}).Where("conversation_id = ?", convID).Count(&messageCount)

		if messageCount == 2 {
			// Generate title based on user's first message
			go func() {
				title := h.agent.GenerateChatTitle(context.Background(), req.Content)
				if title != "" {
					// Update conversation title
					if err := h.db.Model(&models.Conversation{}).
						Where("id = ?", convID).
						Update("title", title).Error; err != nil {
						logging.LogErrorf(err, "Failed to update conversation title")
					} else {
						logging.LogDebugf("Auto-generated title for conversation %s: %s", convID, title)
					}
				}
			}()
		}
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
			h.handleWebSocketReadError(err)
			break
		}

		// Validate input
		if !h.validateStreamRequest(conn, &req) {
			continue
		}

		// Process user message (edit/retry or new)
		userMessage, currentContent, ok := h.processUserMessage(conn, convID, &req)
		if !ok {
			continue
		}

		// Build and send agent response
		h.streamAgentResponse(r.Context(), conn, convID, userID, &conversation, userMessage, currentContent, &req)
	}
}

// handleWebSocketReadError logs WebSocket read errors
func (h *MessagesHandler) handleWebSocketReadError(err error) {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		logging.LogDebugf("WebSocket closed normally")
	} else {
		logging.LogErrorf(err, "WebSocket read error")
	}
}

// validateStreamRequest validates the stream request and sends error if invalid
func (h *MessagesHandler) validateStreamRequest(conn *websocket.Conn, req *SendMessageRequest) bool {
	if req.Content == "" && req.MessageID == nil {
		if err := conn.WriteJSON(map[string]interface{}{"type": "error", "error": "Message content is required"}); err != nil {
			logging.LogErrorf(err, "Failed to write error to WebSocket")
		}
		_ = conn.WriteJSON(map[string]interface{}{"type": "done", "error": "Message content is required"})
		return false
	}
	return true
}

// processUserMessage handles user message creation or editing
func (h *MessagesHandler) processUserMessage(
	conn *websocket.Conn,
	convID uuid.UUID,
	req *SendMessageRequest,
) (models.Message, string, bool) {
	if req.MessageID != nil {
		return h.handleEditOrRetryMessage(conn, convID, req)
	}
	return h.handleNewMessage(conn, convID, req)
}

// handleEditOrRetryMessage handles editing or retrying an existing message
func (h *MessagesHandler) handleEditOrRetryMessage(
	conn *websocket.Conn,
	convID uuid.UUID,
	req *SendMessageRequest,
) (models.Message, string, bool) {
	var userMessage models.Message
	// Load target user message and verify ownership
	if err := h.db.Where("id = ? AND conversation_id = ? AND role = ?", *req.MessageID, convID, models.MessageRoleUser).First(&userMessage).Error; err != nil {
		logging.LogErrorf(err, "Failed to load user message for edit/retry")
		_ = conn.WriteJSON(map[string]interface{}{"type": "error", "error": "Message not found"})
		_ = conn.WriteJSON(map[string]interface{}{"type": "done", "error": "Message not found"})
		return models.Message{}, "", false
	}

	// Update content if provided and different (edit case)
	if req.Content != "" && req.Content != userMessage.Content {
		if err := h.db.Model(&userMessage).Update("content", req.Content).Error; err != nil {
			logging.LogErrorf(err, "Failed to update message content")
			_ = conn.WriteJSON(map[string]interface{}{"type": "error", "error": "Failed to update message"})
			_ = conn.WriteJSON(map[string]interface{}{"type": "done", "error": "Failed to update message"})
			return models.Message{}, "", false
		}
		userMessage.Content = req.Content
	}

	// Clear persisted error in metadata if present
	clearMessageError(h.db, &userMessage)

	// Delete all subsequent messages (continue conversation from here)
	if err := h.db.Where("conversation_id = ? AND created_at > ?", convID, userMessage.CreatedAt).Delete(&models.Message{}).Error; err != nil {
		logging.LogErrorf(err, "Failed to delete subsequent messages")
	}

	// Acknowledge with current user message
	if err := conn.WriteJSON(map[string]interface{}{
		"type":    "user_message",
		"message": userMessage,
	}); err != nil {
		logging.LogErrorf(err, "Failed to send user message confirmation (edit/retry)")
		return models.Message{}, "", false
	}

	return userMessage, userMessage.Content, true
}

// handleNewMessage creates and sends a new user message
func (h *MessagesHandler) handleNewMessage(conn *websocket.Conn, convID uuid.UUID, req *SendMessageRequest) (models.Message, string, bool) {
	userMessage := models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           models.MessageRoleUser,
		Content:        req.Content,
	}
	h.db.Create(&userMessage)

	// Send user message confirmation
	if err := conn.WriteJSON(map[string]interface{}{
		"type":    "user_message",
		"message": userMessage,
	}); err != nil {
		logging.LogErrorf(err, "Failed to send user message confirmation")
		return models.Message{}, "", false
	}

	return userMessage, req.Content, true
}

// streamAgentResponse streams the agent's response through WebSocket
func (h *MessagesHandler) streamAgentResponse(
	ctx context.Context,
	conn *websocket.Conn,
	convID, userID uuid.UUID,
	conversation *models.Conversation,
	userMessage models.Message,
	currentContent string,
	req *SendMessageRequest,
) {
	// Build message history up to (but not including) the current user message
	var messages []models.Message
	h.db.Where("conversation_id = ? AND created_at < ?", convID, userMessage.CreatedAt).
		Order("created_at ASC").
		Find(&messages)

	// Convert message history to agent format
	agentMessages := h.convertToAgentMessages(messages)

	// Stream agent response
	streamChan, err := h.agent.ChatStream(context.Background(), agent.ChatRequest{
		ConversationID: convID,
		UserID:         userID,
		BearerToken:    GetBearerTokenFromContext(ctx),
		UserMessage:    currentContent,
		Messages:       agentMessages,
		Model:          conversation.Model,
	})

	if err != nil {
		h.handleStreamError(conn, &userMessage, err)
		return
	}

	h.processStreamEvents(conn, convID, conversation, userMessage, req, streamChan)
}

// handleStreamError handles errors when starting the stream
func (h *MessagesHandler) handleStreamError(conn *websocket.Conn, userMessage *models.Message, err error) {
	short := shortenUserError(err)
	persistMessageError(h.db, userMessage, short)
	if writeErr := conn.WriteJSON(map[string]interface{}{
		"type":  "error",
		"error": short,
	}); writeErr != nil {
		logging.LogErrorf(writeErr, "Failed to write error to WebSocket")
	}
	_ = conn.WriteJSON(map[string]interface{}{
		"type":  "done",
		"error": short,
	})
}

// processStreamEvents processes events from the agent stream
func (h *MessagesHandler) processStreamEvents(
	conn *websocket.Conn,
	convID uuid.UUID,
	conversation *models.Conversation,
	userMessage models.Message,
	req *SendMessageRequest,
	streamChan <-chan agent.StreamEvent,
) {
	var fullContent string
	var streamedToolExecs []map[string]interface{}

	for event := range streamChan {
		switch event.Type {
		case agent.StreamEventTypeContent:
			fullContent += event.Content
			if err := conn.WriteJSON(map[string]interface{}{
				"type":    "content",
				"content": event.Content,
			}); err != nil {
				logging.LogErrorf(err, "Failed to send content stream")
				return
			}
		case agent.StreamEventTypeToolStart:
			if err := conn.WriteJSON(map[string]interface{}{
				"type": "tool_start",
				"tool": event.Tool,
			}); err != nil {
				logging.LogErrorf(err, "Failed to send tool start event")
				return
			}
		case agent.StreamEventTypeToolComplete:
			streamedToolExecs = h.handleToolComplete(conn, event, streamedToolExecs)
		case agent.StreamEventTypeDone:
			h.handleStreamDone(conn, convID, conversation, fullContent, streamedToolExecs, req)
		case agent.StreamEventTypeError:
			h.handleStreamEventError(conn, &userMessage, event.Error)
		}
	}
}

// handleToolComplete handles tool completion events
func (h *MessagesHandler) handleToolComplete(
	conn *websocket.Conn,
	event agent.StreamEvent,
	streamedToolExecs []map[string]interface{},
) []map[string]interface{} {
	// Collect tool execution for metadata and forward to client
	if event.Tool != nil {
		entry := map[string]interface{}{
			"serverName": event.Tool.ServerName,
			"toolName":   event.Tool.ToolName,
			"arguments":  event.Tool.Arguments,
			"result":     event.Tool.Result,
			"durationMs": event.Tool.Duration.Milliseconds(),
		}
		if event.Tool.Error != nil {
			entry["error"] = event.Tool.Error.Error()
		}
		streamedToolExecs = append(streamedToolExecs, entry)
	}
	if err := conn.WriteJSON(map[string]interface{}{
		"type": "tool_complete",
		"tool": event.Tool,
	}); err != nil {
		logging.LogErrorf(err, "Failed to send tool complete event")
	}
	return streamedToolExecs
}

// handleStreamDone handles the stream completion event
func (h *MessagesHandler) handleStreamDone(
	conn *websocket.Conn,
	convID uuid.UUID,
	conversation *models.Conversation,
	fullContent string,
	streamedToolExecs []map[string]interface{},
	req *SendMessageRequest,
) {
	// Save assistant message
	logging.LogDebugf("Saving assistant message: content=%s", fullContent)
	metaJSON, _ := json.Marshal(map[string]interface{}{
		"toolExecutions": streamedToolExecs,
	})
	assistantMessage := models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           models.MessageRoleAssistant,
		Content:        fullContent,
		Metadata:       datatypes.JSON(metaJSON),
	}
	h.db.Create(&assistantMessage)

	// Auto-generate conversation title if this is the first message
	h.maybeGenerateTitle(convID, conversation, req.Content)

	if err := conn.WriteJSON(map[string]interface{}{
		"type":    "done",
		"message": assistantMessage,
	}); err != nil {
		logging.LogErrorf(err, "Failed to send done event")
	}
}

// handleStreamEventError handles error events from the stream
func (h *MessagesHandler) handleStreamEventError(conn *websocket.Conn, userMessage *models.Message, err error) {
	short := shortenUserError(err)
	persistMessageError(h.db, userMessage, short)
	if writeErr := conn.WriteJSON(map[string]interface{}{
		"type":  "error",
		"error": short,
	}); writeErr != nil {
		logging.LogErrorf(writeErr, "Failed to send error event")
	}
	_ = conn.WriteJSON(map[string]interface{}{
		"type":  "done",
		"error": short,
	})
}

// maybeGenerateTitle auto-generates a conversation title if needed
func (h *MessagesHandler) maybeGenerateTitle(convID uuid.UUID, conversation *models.Conversation, content string) {
	if conversation.Title == "New Chat" || conversation.Title == "New Conversation" {
		var messageCount int64
		h.db.Model(&models.Message{}).Where("conversation_id = ?", convID).Count(&messageCount)

		if messageCount == 2 {
			go func() {
				title := h.agent.GenerateChatTitle(context.Background(), content)
				if title != "" {
					if err := h.db.Model(&models.Conversation{}).
						Where("id = ?", convID).
						Update("title", title).Error; err != nil {
						logging.LogErrorf(err, "Failed to update conversation title")
					} else {
						logging.LogDebugf("Auto-generated title for conversation %s: %s", convID, title)
					}
				}
			}()
		}
	}
}

// convertToAgentMessages converts database messages to agent messages
func (h *MessagesHandler) convertToAgentMessages(dbMessages []models.Message) []llm.Message {
	// Convert all messages except the last one (which is the current user message that will be added by orchestrator)
	// Also skip system messages as the orchestrator adds its own system prompt
	var agentMessages []llm.Message

	for i := 0; i < len(dbMessages)-1; i++ {
		msg := dbMessages[i]

		// Skip system messages
		if msg.Role == models.MessageRoleSystem {
			continue
		}

		agentMsg := llm.Message{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
		}

		// Parse tool calls if present
		if len(msg.ToolCalls) > 0 {
			var toolCalls []llm.ToolCall
			if err := json.Unmarshal(msg.ToolCalls, &toolCalls); err == nil {
				agentMsg.ToolCalls = toolCalls
			}
		}

		agentMessages = append(agentMessages, agentMsg)
	}

	return agentMessages
}

// persistMessageError stores a short error message in the user message metadata
func persistMessageError(db *gorm.DB, msg *models.Message, shortError string) {
	if msg == nil {
		return
	}
	var meta map[string]interface{}
	if len(msg.Metadata) > 0 {
		_ = json.Unmarshal(msg.Metadata, &meta)
	}
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["errorMessage"] = shortError
	b, _ := json.Marshal(meta)
	_ = db.Model(msg).Update("metadata", datatypes.JSON(b)).Error
	msg.Metadata = datatypes.JSON(b)
}

// clearMessageError removes any persisted error from the user message metadata
func clearMessageError(db *gorm.DB, msg *models.Message) {
	if msg == nil {
		return
	}
	var meta map[string]interface{}
	if len(msg.Metadata) > 0 {
		_ = json.Unmarshal(msg.Metadata, &meta)
	}
	if meta == nil {
		return
	}
	delete(meta, "errorMessage")
	b, _ := json.Marshal(meta)
	_ = db.Model(msg).Update("metadata", datatypes.JSON(b)).Error
	msg.Metadata = datatypes.JSON(b)
}

// shortenUserError maps raw errors to concise, user-friendly messages using proper error unwrapping
func shortenUserError(err error) string {
	if err == nil {
		return "Unexpected error"
	}

	// Check for known sentinel errors using errors.Is
	switch {
	case errors.Is(err, agent.ErrLLMUnavailable):
		return "LLM unavailable. Please check your model service."
	case errors.Is(err, agent.ErrMaxIterations):
		return "Maximum iterations reached. Please try rephrasing your question."
	case errors.Is(err, agent.ErrInvalidToolName):
		return "Invalid tool configuration. Please contact support."
	case errors.Is(err, agent.ErrToolExecutionFailed):
		return "Tool execution failed. Please try again."
	case errors.Is(err, llm.ErrConnectionFailed):
		return "LLM connection failed. Please check your model service."
	case errors.Is(err, llm.ErrRequestFailed):
		return "LLM request failed. Please try again."
	}

	// Fallback: truncate long error messages
	s := err.Error()
	if len(s) > 140 {
		return s[:140] + "…"
	}
	return s
}
