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
	// defaultRequestyBaseURL is the default Requesty API base URL.
	defaultRequestyBaseURL = "https://router.requesty.ai/v1"
)

// RequestyOption is a functional option for configuring the Requesty provider.
type RequestyOption func(*Requesty)

// WithRequestyBaseURL sets a custom base URL for the Requesty provider.
func WithRequestyBaseURL(url string) RequestyOption {
	return func(o *Requesty) {
		o.baseURL = url
	}
}

// WithRequestyReferer sets the HTTP-Referer header for Requesty app attribution.
func WithRequestyReferer(referer string) RequestyOption {
	return func(o *Requesty) {
		o.referer = referer
	}
}

// WithRequestyTitle sets the X-Title header for Requesty app attribution.
func WithRequestyTitle(title string) RequestyOption {
	return func(o *Requesty) {
		o.title = title
	}
}

// Requesty implements the Provider interface for the Requesty API.
type Requesty struct {
	apiKey  string
	baseURL string
	referer string
	title   string
	client  *http.Client
}

// NewRequesty creates a new Requesty provider. Returns an error if apiKey is empty.
func NewRequesty(apiKey string, opts ...RequestyOption) (*Requesty, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	o := &Requesty{
		apiKey:  apiKey,
		baseURL: defaultRequestyBaseURL,
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

// requestyResponse represents the JSON response from the Requesty API.
// It extends the OpenAI response shape with Requesty-specific fields.
type requestyResponse struct {
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

// requestyErrorResponse represents a Requesty API error response.
type requestyErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// requestyStreamUsage holds token usage info from the streaming usage chunk,
// including Requesty-specific cost.
type requestyStreamUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}

// requestyStreamChunk represents a single SSE chunk from Requesty streaming API.
type requestyStreamChunk struct {
	Model   string               `json:"model"`
	Choices []openaiStreamChoice `json:"choices"`
	Usage   *requestyStreamUsage `json:"usage,omitempty"`
}

// Send makes a completion request to the Requesty API.
func (o *Requesty) Send(ctx context.Context, req *Request) (*Response, error) {
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
		httpReq.Header.Set("X-Title", o.title)
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

	var apiResp requestyResponse
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

// SendStream makes a streaming completion request to the Requesty API.
func (o *Requesty) SendStream(ctx context.Context, req *Request) (*EventStream, error) {
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
		httpReq.Header.Set("X-Title", o.title)
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

			var chunk requestyStreamChunk
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
func (o *Requesty) handleErrorResponse(status int, body []byte) *ProviderError {
	message := http.StatusText(status)
	var errResp requestyErrorResponse
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
func (o *Requesty) mapStatusToCategory(status int) ErrorCategory {
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
