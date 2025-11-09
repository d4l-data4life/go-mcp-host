package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	"github.com/d4l-data4life/go-svc/pkg/logging"
)

const (
	defaultAPIBaseURL = "https://api.openai.com/v1"
	defaultModel      = "gpt-4o-mini"
)

// Client implements the llm.Client interface using the official OpenAI Go SDK.
type Client struct {
	model  string
	openai *openai.Client
}

// Config defines the settings for the OpenAI client wrapper.
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// NewClient builds a new llm.Client backed by OpenAI's official SDK.
func NewClient(cfg Config) *Client {
	if cfg.APIKey == "" {
		cfg.APIKey = viper.GetString("OPENAI_API_KEY")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = viper.GetString("OPENAI_BASE_URL")
	}
	baseURL := normalizeBaseURL(cfg.BaseURL)
	if cfg.Model == "" {
		model := viper.GetString("OPENAI_DEFAULT_MODEL")
		if model == "" {
			model = defaultModel
		}
		cfg.Model = model
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	httpClient := &http.Client{Timeout: cfg.Timeout}
	opts := []option.RequestOption{
		option.WithHTTPClient(httpClient),
	}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	opts = append(opts, option.WithBaseURL(baseURL))

	openaiClient := openai.NewClient(opts...)

	logging.LogDebugf("Initialized OpenAI client (model=%s, base=%s, timeout=%s)",
		cfg.Model, baseURL, cfg.Timeout)

	return &Client{
		model:  cfg.Model,
		openai: &openaiClient,
	}
}

// Chat sends a non-streaming chat request to OpenAI.
func (c *Client) Chat(ctx context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	if request.Model == "" {
		request.Model = c.model
	}

	params, err := c.buildChatParams(request)
	if err != nil {
		return nil, err
	}

	resp, err := c.openai.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, errors.Wrap(err, "openai chat completion failed")
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, errors.New("openai returned an empty response")
	}

	message := convertFromAPIMessage(resp.Choices[0].Message)

	return &llm.ChatResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Message: message,
		Usage:   convertUsage(resp.Usage),
	}, nil
}

// ChatStream starts a streaming chat completion and returns incremental chunks.
func (c *Client) ChatStream(ctx context.Context, request llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	if request.Model == "" {
		request.Model = c.model
	}

	params, err := c.buildChatParams(request)
	if err != nil {
		return nil, err
	}

	stream := c.openai.Chat.Completions.NewStreaming(ctx, params)
	chunkChan := make(chan llm.StreamChunk, 10)

	go func() {
		defer close(chunkChan)
		defer stream.Close()

		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) == 0 {
				if chunk.Usage.TotalTokens > 0 {
					chunkChan <- llm.StreamChunk{
						ID:    chunk.ID,
						Model: chunk.Model,
						Usage: convertUsage(chunk.Usage),
						Done:  true,
					}
				}
				continue
			}
			for _, choice := range chunk.Choices {
				chunkChan <- llm.StreamChunk{
					ID:    chunk.ID,
					Model: chunk.Model,
					Delta: convertChunkDelta(choice.Delta),
					Usage: convertUsage(chunk.Usage),
					Done:  choice.FinishReason != "",
				}
			}
		}

		if err := stream.Err(); err != nil {
			chunkChan <- llm.StreamChunk{
				Error: errors.Wrap(err, "openai streaming error"),
				Done:  true,
			}
		}
	}()

	return chunkChan, nil
}

// ListModels lists the models visible to the configured API key.
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	resp, err := c.openai.Models.List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list OpenAI models")
	}

	if resp == nil || len(resp.Data) == 0 {
		return nil, errors.New("openai returned no models")
	}

	models := make([]llm.Model, len(resp.Data))
	for i, m := range resp.Data {
		models[i] = llm.Model{
			ID:          m.ID,
			Name:        m.ID,
			Description: string(m.Object),
		}
	}
	return models, nil
}

func (c *Client) buildChatParams(req llm.ChatRequest) (openai.ChatCompletionNewParams, error) {
	messages, err := convertMessages(req.Messages)
	if err != nil {
		return openai.ChatCompletionNewParams{}, err
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(req.Model),
		Messages: messages,
	}

	if len(req.Tools) > 0 {
		params.Tools = convertTools(req.Tools)
	}

	if req.Temperature != nil {
		params.Temperature = param.NewOpt(*req.Temperature)
	}
	if req.MaxTokens != nil {
		params.MaxTokens = param.NewOpt(int64(*req.MaxTokens))
	}
	if req.TopP != nil {
		params.TopP = param.NewOpt(*req.TopP)
	}
	if len(req.Stop) == 1 {
		params.Stop.OfString = param.NewOpt(req.Stop[0])
	} else if len(req.Stop) > 1 {
		params.Stop.OfStringArray = req.Stop
	}

	return params, nil
}

func convertMessages(messages []llm.Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleSystem:
			union := openai.SystemMessage(msg.Content)
			if msg.Name != "" && union.OfSystem != nil {
				union.OfSystem.Name = param.NewOpt(msg.Name)
			}
			result = append(result, union)
		case llm.RoleUser:
			union := openai.UserMessage(msg.Content)
			if msg.Name != "" && union.OfUser != nil {
				union.OfUser.Name = param.NewOpt(msg.Name)
			}
			result = append(result, union)
		case llm.RoleAssistant:
			union := openai.ChatCompletionMessageParamOfAssistant(msg.Content)
			if union.OfAssistant != nil {
				if msg.Name != "" {
					union.OfAssistant.Name = param.NewOpt(msg.Name)
				}
				union.OfAssistant.ToolCalls = convertToolCallsToParams(msg.ToolCalls)
			}
			result = append(result, union)
		case llm.RoleTool:
			if msg.ToolCallID == "" {
				return nil, errors.New("tool messages require a tool_call_id")
			}
			union := openai.ToolMessage(msg.Content, msg.ToolCallID)
			result = append(result, union)
		default:
			union := openai.UserMessage(msg.Content)
			result = append(result, union)
		}
	}
	return result, nil
}

func convertTools(tools []llm.Tool) []openai.ChatCompletionToolParam {
	result := make([]openai.ChatCompletionToolParam, len(tools))
	for i, tool := range tools {
		function := shared.FunctionDefinitionParam{
			Name: tool.Function.Name,
		}
		if tool.Function.Description != "" {
			function.Description = param.NewOpt(tool.Function.Description)
		}
		if tool.Function.Parameters != nil {
			function.Parameters = shared.FunctionParameters(tool.Function.Parameters)
		}

		result[i] = openai.ChatCompletionToolParam{
			Type:     constant.ValueOf[constant.Function](),
			Function: function,
		}
	}
	return result
}

func convertToolCallsToParams(toolCalls []llm.ToolCall) []openai.ChatCompletionMessageToolCallParam {
	if len(toolCalls) == 0 {
		return nil
	}

	result := make([]openai.ChatCompletionMessageToolCallParam, len(toolCalls))
	for i, tc := range toolCalls {
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		result[i] = openai.ChatCompletionMessageToolCallParam{
			ID:   id,
			Type: constant.ValueOf[constant.Function](),
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

func convertFromAPIMessage(msg openai.ChatCompletionMessage) llm.Message {
	return llm.Message{
		Role:      strings.ToLower(string(msg.Role)),
		Content:   msg.Content,
		ToolCalls: convertAPIToolCalls(msg.ToolCalls),
	}
}

func convertChunkDelta(delta openai.ChatCompletionChunkChoiceDelta) llm.Delta {
	return llm.Delta{
		Role:      delta.Role,
		Content:   delta.Content,
		ToolCalls: convertChunkToolCalls(delta.ToolCalls),
	}
}

func convertAPIToolCalls(toolCalls []openai.ChatCompletionMessageToolCall) []llm.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	result := make([]llm.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		result[i] = llm.ToolCall{
			ID:   id,
			Type: llm.ToolTypeFunction,
			Function: llm.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

func convertChunkToolCalls(toolCalls []openai.ChatCompletionChunkChoiceDeltaToolCall) []llm.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	result := make([]llm.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		result[i] = llm.ToolCall{
			ID:   id,
			Type: llm.ToolTypeFunction,
			Function: llm.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

func convertUsage(usage openai.CompletionUsage) llm.Usage {
	return llm.Usage{
		PromptTokens:     int(usage.PromptTokens),
		CompletionTokens: int(usage.CompletionTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = defaultAPIBaseURL
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if !strings.HasSuffix(trimmed, "/v1") {
		trimmed += "/v1"
	}
	return trimmed
}
