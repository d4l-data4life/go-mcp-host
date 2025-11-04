package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/d4l-data4life/go-mcp-host/pkg/llm"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// Client implements the LLM client interface for Ollama
type Client struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// Config holds configuration for the Ollama client
type Config struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

// NewClient creates a new Ollama client
func NewClient(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if config.Model == "" {
		config.Model = "llama3.2"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}

	logging.LogDebugf("Initialized Ollama client with URL: %s (model: %s, timeout: %s)",
		config.BaseURL, config.Model, config.Timeout)

	return &Client{
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Chat sends a chat request and returns the complete response
func (c *Client) Chat(ctx context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	// Set model if not specified
	if request.Model == "" {
		request.Model = c.model
	}

	// Convert to Ollama format
	ollamaReq := c.convertToOllamaRequest(request, false)

	// Marshal request
	reqData, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	logging.LogDebugf("Sending Ollama chat request: model=%s messages=%d tools=%d",
		request.Model, len(request.Messages), len(request.Tools))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	// Parse Ollama response
	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respData, &ollamaResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}

	// Convert to standard format
	response := c.convertFromOllamaResponse(&ollamaResp)

	logging.LogDebugf("Received Ollama response: role=%s content_len=%d tool_calls=%d",
		response.Message.Role, len(response.Message.Content), len(response.Message.ToolCalls))

	return response, nil
}

// ChatStream sends a chat request and returns a channel for streaming responses
func (c *Client) ChatStream(ctx context.Context, request llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	// Set model if not specified
	if request.Model == "" {
		request.Model = c.model
	}

	// Convert to Ollama format with streaming enabled
	ollamaReq := c.convertToOllamaRequest(request, true)

	// Marshal request
	reqData, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	logging.LogDebugf("Starting Ollama streaming chat: model=%s messages=%d tools=%d",
		request.Model, len(request.Messages), len(request.Tools))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	// Create channel for streaming chunks
	chunkChan := make(chan llm.StreamChunk, 10)

	// Start streaming goroutine
	go c.streamResponse(resp.Body, chunkChan)

	return chunkChan, nil
}

// streamResponse reads streaming responses and sends them to the channel
func (c *Client) streamResponse(body io.ReadCloser, chunkChan chan<- llm.StreamChunk) {
	defer close(chunkChan)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse chunk
		var ollamaChunk ollamaChatResponse
		if err := json.Unmarshal(line, &ollamaChunk); err != nil {
			chunkChan <- llm.StreamChunk{
				Error: errors.Wrap(err, "failed to unmarshal chunk"),
				Done:  true,
			}
			return
		}

		// Convert to standard format
		chunk := llm.StreamChunk{
			ID:    ollamaChunk.Model,
			Model: ollamaChunk.Model,
			Delta: llm.Delta{
				Role:    ollamaChunk.Message.Role,
				Content: ollamaChunk.Message.Content,
			},
			Done: ollamaChunk.Done,
		}

		// Parse tool calls if present
		if len(ollamaChunk.Message.ToolCalls) > 0 {
			chunk.Delta.ToolCalls = make([]llm.ToolCall, len(ollamaChunk.Message.ToolCalls))
			for i, tc := range ollamaChunk.Message.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Function.Arguments)
				chunk.Delta.ToolCalls[i] = llm.ToolCall{
					ID:   fmt.Sprintf("call_%d", i),
					Type: llm.ToolTypeFunction,
					Function: llm.ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: string(argsJSON),
					},
				}
			}
		}

		chunkChan <- chunk

		if ollamaChunk.Done {
			logging.LogDebugf("Ollama streaming complete")
			return
		}
	}

	if err := scanner.Err(); err != nil {
		chunkChan <- llm.StreamChunk{
			Error: errors.Wrap(err, "error reading stream"),
			Done:  true,
		}
	}
}

// ListModels returns available models from Ollama
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	models := make([]llm.Model, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = llm.Model{
			ID:         m.Name,
			Name:       m.Name,
			Size:       m.Size,
			ModifiedAt: m.ModifiedAt,
		}
	}

	return models, nil
}

// Helper types for Ollama API

type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Tools    []ollamaTool           `json:"tools,omitempty"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaChatResponse struct {
	Model   string        `json:"model"`
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

// convertToOllamaRequest converts standard request to Ollama format
func (c *Client) convertToOllamaRequest(req llm.ChatRequest, stream bool) ollamaChatRequest {
	ollamaReq := ollamaChatRequest{
		Model:    req.Model,
		Messages: make([]ollamaMessage, len(req.Messages)),
		Stream:   stream,
		Options:  make(map[string]interface{}),
	}

	// Convert messages
	for i, msg := range req.Messages {
		ollamaReq.Messages[i] = ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls
		if len(msg.ToolCalls) > 0 {
			ollamaReq.Messages[i].ToolCalls = make([]ollamaToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				var args map[string]interface{}
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args) // Best effort unmarshaling
				ollamaReq.Messages[i].ToolCalls[j] = ollamaToolCall{
					Function: ollamaToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: args,
					},
				}
			}
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		ollamaReq.Tools = make([]ollamaTool, len(req.Tools))
		for i, tool := range req.Tools {
			ollamaReq.Tools[i] = ollamaTool{
				Type: tool.Type,
				Function: ollamaToolFunction{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
	}

	// Set options
	if req.Temperature != nil {
		ollamaReq.Options["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		ollamaReq.Options["num_predict"] = *req.MaxTokens
	}
	if req.TopP != nil {
		ollamaReq.Options["top_p"] = *req.TopP
	}
	if len(req.Stop) > 0 {
		ollamaReq.Options["stop"] = req.Stop
	}

	return ollamaReq
}

// convertFromOllamaResponse converts Ollama response to standard format
func (c *Client) convertFromOllamaResponse(resp *ollamaChatResponse) *llm.ChatResponse {
	response := &llm.ChatResponse{
		ID:    resp.Model,
		Model: resp.Model,
		Message: llm.Message{
			Role:    resp.Message.Role,
			Content: resp.Message.Content,
		},
	}

	// Convert tool calls
	if len(resp.Message.ToolCalls) > 0 {
		response.Message.ToolCalls = make([]llm.ToolCall, len(resp.Message.ToolCalls))
		for i, tc := range resp.Message.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			response.Message.ToolCalls[i] = llm.ToolCall{
				ID:   fmt.Sprintf("call_%d", i),
				Type: llm.ToolTypeFunction,
				Function: llm.ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: string(argsJSON),
				},
			}
		}
	}

	return response
}
