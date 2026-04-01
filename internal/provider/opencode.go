package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

const defaultOpenCodeBaseURL = "https://opencode.ai/zen"

// OpenCode is an LLM provider that routes requests through the OpenCode Zen API gateway.
// It supports Claude (Anthropic Messages format), GPT (OpenAI Responses format), and
// all other models (OpenAI Chat Completions format).
type OpenCode struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// OpenCodeOption is a functional option for configuring an OpenCode provider.
type OpenCodeOption func(*OpenCode)

// WithOpenCodeBaseURL returns an option that overrides the default Zen gateway base URL.
func WithOpenCodeBaseURL(url string) OpenCodeOption {
	return func(o *OpenCode) {
		o.baseURL = url
	}
}

// NewOpenCode creates a new OpenCode provider.
// Returns an error if apiKey is empty.
func NewOpenCode(apiKey string, opts ...OpenCodeOption) (*OpenCode, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	o := &OpenCode{
		apiKey:  apiKey,
		baseURL: defaultOpenCodeBaseURL,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o, nil
}

// Send dispatches the request to the appropriate Zen endpoint based on the model name prefix.
func (o *OpenCode) Send(ctx context.Context, req *Request) (*Response, error) {
	switch {
	case strings.HasPrefix(req.Model, "claude-"):
		return o.sendClaude(ctx, req)
	case strings.HasPrefix(req.Model, "gpt-"):
		return o.sendGPT(ctx, req)
	default:
		return o.sendChatCompletions(ctx, req)
	}
}

// ---------------------------------------------------------------------------
// Phase 4: Anthropic Messages Format (claude-* models)
// ---------------------------------------------------------------------------

type ocAnthropicContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   string                 `json:"content,omitempty"`
	IsError   *bool                  `json:"is_error,omitempty"`
}

type ocAnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ocAnthropicContentBlock
}

type ocAnthropicInputSchema struct {
	Type       string                      `json:"type"`
	Properties map[string]ocJSONSchemaProp `json:"properties"`
	Required   []string                    `json:"required,omitempty"`
}

type ocJSONSchemaProp struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type ocAnthropicToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema ocAnthropicInputSchema `json:"input_schema"`
}

type ocAnthropicRequest struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	Messages    []ocAnthropicMessage `json:"messages"`
	System      string               `json:"system,omitempty"`
	Temperature *float64             `json:"temperature,omitempty"`
	Tools       []ocAnthropicToolDef `json:"tools,omitempty"`
}

type ocAnthropicResponse struct {
	Content []struct {
		Type  string                 `json:"type"`
		Text  string                 `json:"text"`
		ID    string                 `json:"id"`
		Name  string                 `json:"name"`
		Input map[string]interface{} `json:"input"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type ocAnthropicError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func convertToOCAnthropicMessages(msgs []Message) []ocAnthropicMessage {
	result := make([]ocAnthropicMessage, 0, len(msgs))
	for _, msg := range msgs {
		switch {
		case msg.Role == "tool" && len(msg.ToolResults) > 0:
			blocks := make([]ocAnthropicContentBlock, len(msg.ToolResults))
			for i, tr := range msg.ToolResults {
				isErr := tr.IsError
				blocks[i] = ocAnthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: tr.CallID,
					Content:   tr.Content,
					IsError:   &isErr,
				}
			}
			result = append(result, ocAnthropicMessage{Role: "user", Content: blocks})

		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			var blocks []ocAnthropicContentBlock
			if msg.Content != "" {
				blocks = append(blocks, ocAnthropicContentBlock{Type: "text", Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				input := make(map[string]interface{}, len(tc.Arguments))
				for k, v := range tc.Arguments {
					input[k] = v
				}
				blocks = append(blocks, ocAnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			result = append(result, ocAnthropicMessage{Role: msg.Role, Content: blocks})

		default:
			result = append(result, ocAnthropicMessage{Role: msg.Role, Content: msg.Content})
		}
	}
	return result
}

func convertToOCAnthropicTools(tools []Tool) []ocAnthropicToolDef {
	defs := make([]ocAnthropicToolDef, len(tools))
	for i, t := range tools {
		props := make(map[string]ocJSONSchemaProp, len(t.Parameters))
		var required []string
		for name, param := range t.Parameters {
			props[name] = ocJSONSchemaProp{Type: param.Type, Description: param.Description}
			if param.Required {
				required = append(required, name)
			}
		}
		defs[i] = ocAnthropicToolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: ocAnthropicInputSchema{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		}
	}
	return defs
}

func mapOCAnthropicHTTPError(status int, body []byte) *ProviderError {
	var cat ErrorCategory
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		cat = ErrCategoryAuth
	case http.StatusBadRequest:
		cat = ErrCategoryBadRequest
	case http.StatusTooManyRequests:
		cat = ErrCategoryRateLimit
	case 529:
		cat = ErrCategoryOverloaded
	default:
		cat = ErrCategoryServer
	}

	msg := http.StatusText(status)
	var errBody ocAnthropicError
	if json.Unmarshal(body, &errBody) == nil && errBody.Error.Message != "" {
		msg = errBody.Error.Message
	}

	return &ProviderError{Category: cat, Status: status, Message: msg}
}

func (o *OpenCode) sendClaude(ctx context.Context, req *Request) (*Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	body := ocAnthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  convertToOCAnthropicMessages(req.Messages),
		System:    req.System,
	}

	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOCAnthropicTools(req.Tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to marshal request: %s", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to create request: %s", err)}
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error()}
		}
		return nil, &ProviderError{Category: ErrCategoryServer, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to read response: %s", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapOCAnthropicHTTPError(resp.StatusCode, respBody)
	}

	var parsed ocAnthropicResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse response: %s", err)}
	}

	if len(parsed.Content) == 0 {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: "response contains no content"}
	}

	result := &Response{
		Model:        parsed.Model,
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
		StopReason:   parsed.StopReason,
	}

	for _, block := range parsed.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			args := make(map[string]string, len(block.Input))
			for k, v := range block.Input {
				args[k] = fmt.Sprintf("%v", v)
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Streaming
// ---------------------------------------------------------------------------

// SendStream dispatches the streaming request by model prefix.
func (o *OpenCode) SendStream(ctx context.Context, req *Request) (*EventStream, error) {
	switch {
	case strings.HasPrefix(req.Model, "claude-"):
		return o.streamClaude(ctx, req)
	case strings.HasPrefix(req.Model, "gpt-"):
		return o.streamGPT(ctx, req)
	default:
		return o.streamChatCompletions(ctx, req)
	}
}

// --- Claude (Anthropic Messages) streaming ---

type ocAnthropicStreamRequest struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	Messages    []ocAnthropicMessage `json:"messages"`
	System      string               `json:"system,omitempty"`
	Temperature *float64             `json:"temperature,omitempty"`
	Tools       []ocAnthropicToolDef `json:"tools,omitempty"`
	Stream      bool                 `json:"stream"`
}

func (o *OpenCode) streamClaude(ctx context.Context, req *Request) (*EventStream, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	body := ocAnthropicStreamRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  convertToOCAnthropicMessages(req.Messages),
		System:    req.System,
		Stream:    true,
	}

	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOCAnthropicTools(req.Tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to marshal request: %s", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to create request: %s", err)}
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error()}
		}
		return nil, &ProviderError{Category: ErrCategoryServer, Message: err.Error()}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to read error response: %s", err)}
		}
		return nil, mapOCAnthropicHTTPError(resp.StatusCode, respBody)
	}

	parser := NewSSEParser(resp.Body)
	var inputTokens int
	blocks := make(map[int]anthropicBlockInfo)

	nextFunc := func() (StreamEvent, error) {
		for {
			sseEvent, err := parser.Next()
			if err != nil {
				if ctx.Err() != nil {
					return StreamEvent{}, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error(), Err: ctx.Err()}
				}
				if err == io.EOF {
					return StreamEvent{}, io.EOF
				}
				return StreamEvent{}, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("stream read error: %s", err), Err: err}
			}

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(sseEvent.Data), &event); err != nil {
				return StreamEvent{}, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse streaming event: %s", err), Err: err}
			}

			switch event.Type {
			case "message_start":
				if event.Message != nil && event.Message.Usage != nil {
					inputTokens = event.Message.Usage.InputTokens
				}
				continue
			case "content_block_start":
				if event.ContentBlock != nil {
					blocks[event.Index] = anthropicBlockInfo{typ: event.ContentBlock.Type, id: event.ContentBlock.ID, name: event.ContentBlock.Name}
					if event.ContentBlock.Type == "tool_use" {
						return StreamEvent{Type: StreamEventToolStart, ToolCallID: event.ContentBlock.ID, ToolName: event.ContentBlock.Name}, nil
					}
				}
				continue
			case "content_block_delta":
				if event.Delta == nil {
					continue
				}
				block, ok := blocks[event.Index]
				if !ok {
					continue
				}
				switch event.Delta.Type {
				case "text_delta":
					if event.Delta.Text == "" {
						continue
					}
					return StreamEvent{Type: StreamEventText, Text: event.Delta.Text}, nil
				case "input_json_delta":
					if event.Delta.PartialJSON == "" {
						continue
					}
					return StreamEvent{Type: StreamEventToolDelta, ToolCallID: block.id, ToolInput: event.Delta.PartialJSON}, nil
				}
				continue
			case "content_block_stop":
				block, ok := blocks[event.Index]
				if ok && block.typ == "tool_use" {
					return StreamEvent{Type: StreamEventToolEnd, ToolCallID: block.id}, nil
				}
				continue
			case "message_delta":
				var stopReason string
				if event.Delta != nil {
					stopReason = event.Delta.StopReason
				}
				var outputTokens int
				if event.Usage != nil {
					outputTokens = event.Usage.OutputTokens
				}
				return StreamEvent{Type: StreamEventDone, StopReason: stopReason, InputTokens: inputTokens, OutputTokens: outputTokens}, nil
			case "message_stop":
				return StreamEvent{}, io.EOF
			case "ping":
				continue
			case "error":
				if event.Error != nil {
					return StreamEvent{}, &ProviderError{Category: mapStreamErrorType(event.Error.Type), Message: event.Error.Message}
				}
				return StreamEvent{}, &ProviderError{Category: ErrCategoryServer, Message: "unknown stream error"}
			default:
				continue
			}
		}
	}

	return NewEventStream(resp.Body, nextFunc), nil
}

// --- GPT (OpenAI Responses API) streaming ---

type ocResponsesStreamRequest struct {
	Model           string            `json:"model"`
	Input           []interface{}     `json:"input"`
	Temperature     *float64          `json:"temperature,omitempty"`
	MaxOutputTokens *int              `json:"max_output_tokens,omitempty"`
	Tools           []ocOpenAIToolDef `json:"tools,omitempty"`
	Stream          bool              `json:"stream"`
}

type ocRespStreamEvent struct {
	Type   string          `json:"type"`
	ItemID string          `json:"item_id,omitempty"`
	Item   json.RawMessage `json:"item,omitempty"`
	Delta  string          `json:"delta,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}

type ocRespStreamItem struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ocRespStreamResponse struct {
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (o *OpenCode) streamGPT(ctx context.Context, req *Request) (*EventStream, error) {
	body := ocResponsesStreamRequest{
		Model:  req.Model,
		Input:  convertToOCResponsesMessages(req),
		Stream: true,
	}

	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		body.MaxOutputTokens = &mt
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOCOpenAITools(req.Tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to marshal request: %s", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/responses", bytes.NewReader(data))
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to create request: %s", err)}
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error()}
		}
		return nil, &ProviderError{Category: ErrCategoryServer, Message: err.Error()}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to read error response: %s", err)}
		}
		return nil, mapOCOpenAIHTTPError(resp.StatusCode, respBody)
	}

	parser := NewSSEParser(resp.Body)

	nextFunc := func() (StreamEvent, error) {
		for {
			sseEvent, err := parser.Next()
			if err != nil {
				if ctx.Err() != nil {
					return StreamEvent{}, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error(), Err: ctx.Err()}
				}
				if err == io.EOF {
					return StreamEvent{}, io.EOF
				}
				return StreamEvent{}, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("stream read error: %s", err), Err: err}
			}

			if sseEvent.Data == "[DONE]" {
				return StreamEvent{}, io.EOF
			}

			var event ocRespStreamEvent
			if err := json.Unmarshal([]byte(sseEvent.Data), &event); err != nil {
				return StreamEvent{}, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse streaming event: %s", err), Err: err}
			}

			switch event.Type {
			case "response.content_part.delta":
				if event.Delta == "" {
					continue
				}
				return StreamEvent{Type: StreamEventText, Text: event.Delta}, nil

			case "response.output_item.added":
				var item ocRespStreamItem
				if err := json.Unmarshal(event.Item, &item); err != nil {
					continue
				}
				if item.Type == "function_call" {
					return StreamEvent{Type: StreamEventToolStart, ToolCallID: item.ID, ToolName: item.Name}, nil
				}
				continue

			case "response.function_call_arguments.delta":
				return StreamEvent{Type: StreamEventToolDelta, ToolCallID: event.ItemID, ToolInput: event.Delta}, nil

			case "response.output_item.done":
				var item ocRespStreamItem
				if err := json.Unmarshal(event.Item, &item); err != nil {
					continue
				}
				if item.Type == "function_call" {
					return StreamEvent{Type: StreamEventToolEnd, ToolCallID: item.ID}, nil
				}
				continue

			case "response.completed":
				var respObj ocRespStreamResponse
				if err := json.Unmarshal(event.Response, &respObj); err == nil {
					return StreamEvent{
						Type:         StreamEventDone,
						InputTokens:  respObj.Usage.InputTokens,
						OutputTokens: respObj.Usage.OutputTokens,
					}, nil
				}
				return StreamEvent{Type: StreamEventDone}, nil

			default:
				continue
			}
		}
	}

	return NewEventStream(resp.Body, nextFunc), nil
}

// --- Chat Completions streaming ---

type ocChatStreamRequest struct {
	Model         string             `json:"model"`
	Messages      []ocChatMessage    `json:"messages"`
	Temperature   *float64           `json:"temperature,omitempty"`
	MaxTokens     *int               `json:"max_tokens,omitempty"`
	Tools         []ocOpenAIToolDef  `json:"tools,omitempty"`
	Stream        bool               `json:"stream"`
	StreamOptions *ocStreamOptions   `json:"stream_options,omitempty"`
}

type ocStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

func (o *OpenCode) streamChatCompletions(ctx context.Context, req *Request) (*EventStream, error) {
	body := ocChatStreamRequest{
		Model:         req.Model,
		Messages:      convertToOCChatMessages(req),
		Stream:        true,
		StreamOptions: &ocStreamOptions{IncludeUsage: true},
	}

	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		body.MaxTokens = &mt
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOCOpenAITools(req.Tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to marshal request: %s", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to create request: %s", err)}
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error()}
		}
		return nil, &ProviderError{Category: ErrCategoryServer, Message: err.Error()}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to read error response: %s", err)}
		}
		return nil, mapOCOpenAIHTTPError(resp.StatusCode, respBody)
	}

	parser := NewSSEParser(resp.Body)
	toolCalls := make(map[int]struct{ id, name string })
	var finishReason string
	var gotUsage bool
	var pendingToolEnds []StreamEvent

	nextFunc := func() (StreamEvent, error) {
		for {
			if len(pendingToolEnds) > 0 {
				ev := pendingToolEnds[0]
				pendingToolEnds = pendingToolEnds[1:]
				return ev, nil
			}

			sseEvent, err := parser.Next()
			if err != nil {
				if ctx.Err() != nil {
					return StreamEvent{}, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error(), Err: ctx.Err()}
				}
				if err == io.EOF {
					return StreamEvent{}, io.EOF
				}
				return StreamEvent{}, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("stream read error: %s", err), Err: err}
			}

			if sseEvent.Data == "[DONE]" {
				if !gotUsage {
					return StreamEvent{Type: StreamEventDone, StopReason: finishReason}, nil
				}
				return StreamEvent{}, io.EOF
			}

			var chunk openaiStreamChunk
			if err := json.Unmarshal([]byte(sseEvent.Data), &chunk); err != nil {
				return StreamEvent{}, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse streaming chunk: %s", err), Err: err}
			}

			if len(chunk.Choices) == 0 {
				if chunk.Usage != nil {
					gotUsage = true
					return StreamEvent{
						Type:         StreamEventDone,
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
						StopReason:   finishReason,
					}, nil
				}
				continue
			}

			choice := chunk.Choices[0]

			if choice.Delta.Content != nil && *choice.Delta.Content != "" {
				return StreamEvent{Type: StreamEventText, Text: *choice.Delta.Content}, nil
			}

			if len(choice.Delta.ToolCalls) > 0 {
				tc := choice.Delta.ToolCalls[0]
				if tc.ID != "" {
					toolCalls[tc.Index] = struct{ id, name string }{id: tc.ID, name: tc.Function.Name}
					return StreamEvent{Type: StreamEventToolStart, ToolCallID: tc.ID, ToolName: tc.Function.Name}, nil
				}
				info := toolCalls[tc.Index]
				args := ""
				if tc.Function != nil {
					args = tc.Function.Arguments
				}
				return StreamEvent{Type: StreamEventToolDelta, ToolCallID: info.id, ToolInput: args}, nil
			}

			if choice.FinishReason != nil {
				fr := *choice.FinishReason
				finishReason = fr
				if fr == "tool_calls" {
					indices := make([]int, 0, len(toolCalls))
					for idx := range toolCalls {
						indices = append(indices, idx)
					}
					sort.Ints(indices)
					for _, idx := range indices {
						info := toolCalls[idx]
						pendingToolEnds = append(pendingToolEnds, StreamEvent{Type: StreamEventToolEnd, ToolCallID: info.id})
					}
				}
				continue
			}

			continue
		}
	}

	return NewEventStream(resp.Body, nextFunc), nil
}

// ---------------------------------------------------------------------------
// Phase 5: OpenAI Responses Format (gpt-* models)
// ---------------------------------------------------------------------------

type ocOpenAIFunctionSchema struct {
	Type       string                      `json:"type"`
	Properties map[string]ocJSONSchemaProp `json:"properties"`
	Required   []string                    `json:"required,omitempty"`
}

type ocOpenAIToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  ocOpenAIFunctionSchema `json:"parameters"`
	} `json:"function"`
}

type ocToolCallWire struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ocResponsesAssistantMsg struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []ocToolCallWire `json:"tool_calls,omitempty"`
}

type ocResponsesRequest struct {
	Model           string            `json:"model"`
	Input           []interface{}     `json:"input"`
	Temperature     *float64          `json:"temperature,omitempty"`
	MaxOutputTokens *int              `json:"max_output_tokens,omitempty"`
	Tools           []ocOpenAIToolDef `json:"tools,omitempty"`
}

type ocResponsesResponse struct {
	Model  string `json:"model"`
	Status string `json:"status"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type ocOpenAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func convertToOCOpenAITools(tools []Tool) []ocOpenAIToolDef {
	defs := make([]ocOpenAIToolDef, len(tools))
	for i, t := range tools {
		props := make(map[string]ocJSONSchemaProp, len(t.Parameters))
		var required []string
		for name, param := range t.Parameters {
			props[name] = ocJSONSchemaProp{Type: param.Type, Description: param.Description}
			if param.Required {
				required = append(required, name)
			}
		}
		var def ocOpenAIToolDef
		def.Type = "function"
		def.Function.Name = t.Name
		def.Function.Description = t.Description
		def.Function.Parameters = ocOpenAIFunctionSchema{
			Type:       "object",
			Properties: props,
			Required:   required,
		}
		defs[i] = def
	}
	return defs
}

func mapOCOpenAIHTTPError(status int, body []byte) *ProviderError {
	var cat ErrorCategory
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		cat = ErrCategoryAuth
	case http.StatusBadRequest, http.StatusNotFound:
		cat = ErrCategoryBadRequest
	case http.StatusTooManyRequests:
		cat = ErrCategoryRateLimit
	default:
		cat = ErrCategoryServer
	}

	msg := http.StatusText(status)
	var errBody ocOpenAIError
	if json.Unmarshal(body, &errBody) == nil && errBody.Error.Message != "" {
		msg = errBody.Error.Message
	}

	return &ProviderError{Category: cat, Status: status, Message: msg}
}

func convertToOCResponsesMessages(req *Request) []interface{} {
	var msgs []interface{}

	if req.System != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": req.System})
	}

	for _, msg := range req.Messages {
		switch {
		case msg.Role == "tool" && len(msg.ToolResults) > 0:
			for _, tr := range msg.ToolResults {
				msgs = append(msgs, map[string]string{
					"role":         "tool",
					"tool_call_id": tr.CallID,
					"content":      tr.Content,
				})
			}
		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			toolCalls := make([]ocToolCallWire, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Arguments)
				toolCalls[i] = ocToolCallWire{ID: tc.ID, Type: "function"}
				toolCalls[i].Function.Name = tc.Name
				toolCalls[i].Function.Arguments = string(argsJSON)
			}
			msgs = append(msgs, ocResponsesAssistantMsg{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: toolCalls,
			})
		default:
			msgs = append(msgs, map[string]string{"role": msg.Role, "content": msg.Content})
		}
	}

	return msgs
}

func (o *OpenCode) sendGPT(ctx context.Context, req *Request) (*Response, error) {
	body := ocResponsesRequest{
		Model: req.Model,
		Input: convertToOCResponsesMessages(req),
	}

	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		body.MaxOutputTokens = &mt
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOCOpenAITools(req.Tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to marshal request: %s", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/responses", bytes.NewReader(data))
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to create request: %s", err)}
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error()}
		}
		return nil, &ProviderError{Category: ErrCategoryServer, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to read response: %s", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapOCOpenAIHTTPError(resp.StatusCode, respBody)
	}

	var parsed ocResponsesResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse response: %s", err)}
	}

	if len(parsed.Output) == 0 {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: "response contains no output"}
	}

	result := &Response{
		Model:        parsed.Model,
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
		StopReason:   parsed.Status,
	}

	for _, out := range parsed.Output {
		switch out.Type {
		case "message":
			for _, c := range out.Content {
				if c.Type == "output_text" {
					result.Content += c.Text
				}
			}
		case "function_call":
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(out.Arguments), &argsMap); err != nil {
				return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse tool call arguments: %s", err)}
			}
			args := make(map[string]string, len(argsMap))
			for k, v := range argsMap {
				args[k] = fmt.Sprintf("%v", v)
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        out.ID,
				Name:      out.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Phase 6: OpenAI Chat Completions Format (all other models)
// ---------------------------------------------------------------------------

type ocChatMessage struct {
	Role       string           `json:"role"`
	Content    *string          `json:"content"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []ocToolCallWire `json:"tool_calls,omitempty"`
}

type ocChatRequest struct {
	Model       string            `json:"model"`
	Messages    []ocChatMessage   `json:"messages"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
	Tools       []ocOpenAIToolDef `json:"tools,omitempty"`
}

type ocChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   *string          `json:"content"`
			ToolCalls []ocToolCallWire `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func convertToOCChatMessages(req *Request) []ocChatMessage {
	var msgs []ocChatMessage

	if req.System != "" {
		content := req.System
		msgs = append(msgs, ocChatMessage{Role: "system", Content: &content})
	}

	for _, msg := range req.Messages {
		switch {
		case msg.Role == "tool" && len(msg.ToolResults) > 0:
			for _, tr := range msg.ToolResults {
				content := tr.Content
				msgs = append(msgs, ocChatMessage{
					Role:       "tool",
					Content:    &content,
					ToolCallID: tr.CallID,
				})
			}
		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			toolCalls := make([]ocToolCallWire, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Arguments)
				toolCalls[i] = ocToolCallWire{ID: tc.ID, Type: "function"}
				toolCalls[i].Function.Name = tc.Name
				toolCalls[i].Function.Arguments = string(argsJSON)
			}
			msgs = append(msgs, ocChatMessage{
				Role:      msg.Role,
				Content:   nil,
				ToolCalls: toolCalls,
			})
		default:
			content := msg.Content
			msgs = append(msgs, ocChatMessage{Role: msg.Role, Content: &content})
		}
	}

	return msgs
}

func (o *OpenCode) sendChatCompletions(ctx context.Context, req *Request) (*Response, error) {
	body := ocChatRequest{
		Model:    req.Model,
		Messages: convertToOCChatMessages(req),
	}

	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		body.MaxTokens = &mt
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOCOpenAITools(req.Tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to marshal request: %s", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to create request: %s", err)}
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error()}
		}
		return nil, &ProviderError{Category: ErrCategoryServer, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to read response: %s", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapOCOpenAIHTTPError(resp.StatusCode, respBody)
	}

	var parsed ocChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse response: %s", err)}
	}

	if len(parsed.Choices) == 0 {
		return nil, &ProviderError{Category: ErrCategoryServer, Message: "response contains no choices"}
	}

	choice := parsed.Choices[0]
	result := &Response{
		Model:        parsed.Model,
		InputTokens:  parsed.Usage.PromptTokens,
		OutputTokens: parsed.Usage.CompletionTokens,
		StopReason:   choice.FinishReason,
	}

	if choice.Message.Content != nil {
		result.Content = *choice.Message.Content
	}

	for _, tc := range choice.Message.ToolCalls {
		var argsMap map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsMap); err != nil {
			return nil, &ProviderError{Category: ErrCategoryServer, Message: fmt.Sprintf("failed to parse tool call arguments: %s", err)}
		}
		args := make(map[string]string, len(argsMap))
		for k, v := range argsMap {
			args[k] = fmt.Sprintf("%v", v)
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return result, nil
}
