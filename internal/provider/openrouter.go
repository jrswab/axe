package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

const (
	// defaultOpenRouterBaseURL is the default OpenRouter API base URL.
	defaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"
)

// OpenRouterOption is a functional option for configuring the OpenRouter provider.
type OpenRouterOption func(*OpenRouter)

// WithOpenRouterBaseURL sets a custom base URL for the OpenRouter provider.
func WithOpenRouterBaseURL(url string) OpenRouterOption {
	return func(o *OpenRouter) {
		o.baseURL = url
	}
}

// WithReferer sets the HTTP-Referer header for OpenRouter app attribution.
func WithReferer(referer string) OpenRouterOption {
	return func(o *OpenRouter) {
		o.referer = referer
	}
}

// WithTitle sets the X-OpenRouter-Title header for OpenRouter app attribution.
func WithTitle(title string) OpenRouterOption {
	return func(o *OpenRouter) {
		o.title = title
	}
}

// WithCategories sets the X-OpenRouter-Categories header for OpenRouter app attribution.
func WithCategories(categories string) OpenRouterOption {
	return func(o *OpenRouter) {
		o.categories = categories
	}
}

// OpenRouter implements the Provider interface for the OpenRouter API.
type OpenRouter struct {
	apiKey     string
	baseURL    string
	referer    string
	title      string
	categories string
	client     *http.Client
}

// NewOpenRouter creates a new OpenRouter provider. Returns an error if apiKey is empty.
func NewOpenRouter(apiKey string, opts ...OpenRouterOption) (*OpenRouter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	o := &OpenRouter{
		apiKey:  apiKey,
		baseURL: defaultOpenRouterBaseURL,
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

// openrouterResponse represents the JSON response from the OpenRouter API.
// It extends the OpenAI response shape with OpenRouter-specific fields.
type openrouterResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   *string              `json:"content"`
			ToolCalls []openaiToolCallWire `json:"tool_calls"`
		} `json:"message"`
		FinishReason       string `json:"finish_reason"`
		NativeFinishReason string `json:"native_finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
}

// openrouterErrorResponse represents an OpenRouter API error response.
type openrouterErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// openrouterStreamUsage holds token usage info from the streaming usage chunk,
// including OpenRouter-specific cost.
type openrouterStreamUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}

// openrouterStreamChunk represents a single SSE chunk from OpenRouter streaming API.
type openrouterStreamChunk struct {
	Model   string               `json:"model"`
	Choices []openaiStreamChoice `json:"choices"`
	Usage   *openrouterStreamUsage `json:"usage,omitempty"`
}

// Send makes a completion request to the OpenRouter API.
func (o *OpenRouter) Send(ctx context.Context, req *Request) (*Response, error) {
	var messages []Message
	if req.System != "" {
		messages = append(messages, Message{Role: "system", Content: req.System})
	}
	messages = append(messages, req.Messages...)

	body := openaiRequest{
		Model:    req.Model,
		Messages: convertToOpenAIMessages(messages),
	}

	if req.Temperature != 0 {
		temp := req.Temperature
		body.Temperature = &temp
	}

	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		body.MaxTokens = &mt
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOpenAITools(req.Tools)
	}

	if req.ResponseFormat.IsSet() {
		rf, rfErr := buildOpenAIResponseFormat(req.ResponseFormat)
		if rfErr != nil {
			return nil, rfErr
		}
		body.ResponseFormat = rf
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if o.referer != "" {
		httpReq.Header.Set("HTTP-Referer", o.referer)
	}
	if o.title != "" {
		httpReq.Header.Set("X-OpenRouter-Title", o.title)
	}
	if o.categories != "" {
		httpReq.Header.Set("X-OpenRouter-Categories", o.categories)
	}

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{
				Category: ErrCategoryTimeout,
				Message:  ctx.Err().Error(),
				Err:      ctx.Err(),
			}
		}
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  err.Error(),
			Err:      err,
		}
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, o.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	var apiResp openrouterResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	if len(apiResp.Choices) == 0 {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  "response contains no choices",
		}
	}

	// Parse content (may be null for tool-call responses)
	var content string
	if apiResp.Choices[0].Message.Content != nil {
		content = *apiResp.Choices[0].Message.Content
	}

	// Parse tool calls from response
	var toolCalls []ToolCall
	for _, tc := range apiResp.Choices[0].Message.ToolCalls {
		args := make(map[string]string)
		var rawArgs map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &rawArgs); err == nil {
			for k, v := range rawArgs {
				args[k] = fmt.Sprintf("%v", v)
			}
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	// Determine stop reason: prefer native if available, else normalized
	stopReason := apiResp.Choices[0].FinishReason
	if apiResp.Choices[0].NativeFinishReason != "" {
		stopReason = apiResp.Choices[0].NativeFinishReason
	}

	cacheStatus := httpResp.Header.Get("X-OpenRouter-Cache-Status")

	return &Response{
		Content:      content,
		Model:        apiResp.Model,
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
		Cost:         apiResp.Usage.Cost,
		StopReason:   stopReason,
		CacheStatus:  cacheStatus,
		ToolCalls:    toolCalls,
	}, nil
}

// SendStream makes a streaming completion request to the OpenRouter API.
func (o *OpenRouter) SendStream(ctx context.Context, req *Request) (*EventStream, error) {
	var messages []Message
	if req.System != "" {
		messages = append(messages, Message{Role: "system", Content: req.System})
	}
	messages = append(messages, req.Messages...)

	body := openaiRequest{
		Model:         req.Model,
		Messages:      convertToOpenAIMessages(messages),
		Stream:        true,
		StreamOptions: &openaiStreamOptions{IncludeUsage: true},
	}

	if req.Temperature != 0 {
		temp := req.Temperature
		body.Temperature = &temp
	}

	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		body.MaxTokens = &mt
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToOpenAITools(req.Tools)
	}

	if req.ResponseFormat.IsSet() {
		rf, rfErr := buildOpenAIResponseFormat(req.ResponseFormat)
		if rfErr != nil {
			return nil, rfErr
		}
		body.ResponseFormat = rf
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if o.referer != "" {
		httpReq.Header.Set("HTTP-Referer", o.referer)
	}
	if o.title != "" {
		httpReq.Header.Set("X-OpenRouter-Title", o.title)
	}
	if o.categories != "" {
		httpReq.Header.Set("X-OpenRouter-Categories", o.categories)
	}

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{
				Category: ErrCategoryTimeout,
				Message:  ctx.Err().Error(),
				Err:      ctx.Err(),
			}
		}
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  err.Error(),
			Err:      err,
		}
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		respBody, err := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		if err != nil {
			return nil, &ProviderError{
				Category: ErrCategoryServer,
				Message:  fmt.Sprintf("failed to read error response: %s", err),
				Err:      err,
			}
		}
		return nil, o.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	cacheStatus := httpResp.Header.Get("X-OpenRouter-Cache-Status")

	parser := NewSSEParser(httpResp.Body)
	toolCalls := make(map[int]struct{ id, name string })
	var finishReason string
	var gotUsage bool
	var pendingToolEvents []StreamEvent
	var pendingToolEnds []StreamEvent

	nextFunc := func() (StreamEvent, error) {
		for {
			if len(pendingToolEvents) > 0 {
				ev := pendingToolEvents[0]
				pendingToolEvents = pendingToolEvents[1:]
				return ev, nil
			}

			if len(pendingToolEnds) > 0 {
				ev := pendingToolEnds[0]
				pendingToolEnds = pendingToolEnds[1:]
				return ev, nil
			}

			sseEvent, err := parser.Next()
			if err != nil {
				if ctx.Err() != nil {
					return StreamEvent{}, &ProviderError{
						Category: ErrCategoryTimeout,
						Message:  ctx.Err().Error(),
						Err:      ctx.Err(),
					}
				}
				if err == io.EOF {
					return StreamEvent{}, io.EOF
				}
				return StreamEvent{}, &ProviderError{
					Category: ErrCategoryServer,
					Message:  fmt.Sprintf("stream read error: %s", err),
					Err:      err,
				}
			}

			if sseEvent.Data == "[DONE]" {
				if !gotUsage {
					return StreamEvent{
						Type:        StreamEventDone,
						StopReason:  finishReason,
						CacheStatus: cacheStatus,
					}, nil
				}
				return StreamEvent{}, io.EOF
			}

			var chunk openrouterStreamChunk
			if err := json.Unmarshal([]byte(sseEvent.Data), &chunk); err != nil {
				return StreamEvent{}, &ProviderError{
					Category: ErrCategoryServer,
					Message:  fmt.Sprintf("failed to parse streaming chunk: %s", err),
					Err:      err,
				}
			}

			if len(chunk.Choices) == 0 {
				if chunk.Usage != nil {
					gotUsage = true
					return StreamEvent{
						Type:         StreamEventDone,
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
						Cost:         chunk.Usage.Cost,
						StopReason:   finishReason,
						CacheStatus:  cacheStatus,
					}, nil
				}
				continue
			}

			choice := chunk.Choices[0]

			if choice.Delta.Content != nil && *choice.Delta.Content != "" {
				return StreamEvent{
					Type: StreamEventText,
					Text: *choice.Delta.Content,
				}, nil
			}

			if len(choice.Delta.ToolCalls) > 0 {
				for _, tc := range choice.Delta.ToolCalls {
					if tc.ID != "" {
						toolCalls[tc.Index] = struct{ id, name string }{id: tc.ID, name: tc.Function.Name}
						pendingToolEvents = append(pendingToolEvents, StreamEvent{
							Type:       StreamEventToolStart,
							ToolCallID: tc.ID,
							ToolName:   tc.Function.Name,
						})
					} else {
						info := toolCalls[tc.Index]
						args := ""
						if tc.Function != nil {
							args = tc.Function.Arguments
						}
						pendingToolEvents = append(pendingToolEvents, StreamEvent{
							Type:       StreamEventToolDelta,
							ToolCallID: info.id,
							ToolInput:  args,
						})
					}
				}
				continue
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
						pendingToolEnds = append(pendingToolEnds, StreamEvent{
							Type:       StreamEventToolEnd,
							ToolCallID: info.id,
						})
					}
				}
				continue
			}

			continue
		}
	}

	return NewEventStream(httpResp.Body, nextFunc), nil
}

// handleErrorResponse maps HTTP error responses to ProviderError.
func (o *OpenRouter) handleErrorResponse(status int, body []byte) *ProviderError {
	message := http.StatusText(status)
	var errResp openrouterErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	return &ProviderError{
		Category: o.mapStatusToCategory(status),
		Status:   status,
		Message:  message,
	}
}

// mapStatusToCategory maps HTTP status codes to error categories.
func (o *OpenRouter) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 401, 403:
		return ErrCategoryAuth
	case 400, 404:
		return ErrCategoryBadRequest
	case 429:
		return ErrCategoryRateLimit
	case 500, 502, 503:
		return ErrCategoryServer
	default:
		return ErrCategoryServer
	}
}
