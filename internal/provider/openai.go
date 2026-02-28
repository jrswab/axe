package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// defaultOpenAIBaseURL is the default OpenAI API base URL.
	defaultOpenAIBaseURL = "https://api.openai.com"
)

// OpenAIOption is a functional option for configuring the OpenAI provider.
type OpenAIOption func(*OpenAI)

// WithOpenAIBaseURL sets a custom base URL for the OpenAI provider.
func WithOpenAIBaseURL(url string) OpenAIOption {
	return func(o *OpenAI) {
		o.baseURL = url
	}
}

// OpenAI implements the Provider interface for the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAI creates a new OpenAI provider. Returns an error if apiKey is empty.
func NewOpenAI(apiKey string, opts ...OpenAIOption) (*OpenAI, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	o := &OpenAI{
		apiKey:  apiKey,
		baseURL: defaultOpenAIBaseURL,
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

// openaiRequest is the JSON body sent to the OpenAI Chat Completions API.
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Tools       []openaiToolDef `json:"tools,omitempty"`
}

// openaiMessage is the wire format for a message in the OpenAI API.
type openaiMessage struct {
	Role       string               `json:"role"`
	Content    *string              `json:"content"`                // nullable for assistant tool-call messages
	ToolCallID string               `json:"tool_call_id,omitempty"` // for role "tool" messages
	ToolCalls  []openaiToolCallWire `json:"tool_calls,omitempty"`   // for assistant messages with tool calls
}

// openaiToolDef is the wire format for a tool definition in the OpenAI API.
type openaiToolDef struct {
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

// openaiToolFunction is the function definition inside an OpenAI tool.
type openaiToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// openaiToolCallWire is the wire format for a tool call in OpenAI request/response.
type openaiToolCallWire struct {
	ID       string                     `json:"id"`
	Type     string                     `json:"type"`
	Function openaiToolCallFunctionWire `json:"function"`
}

// openaiToolCallFunctionWire is the function info inside a tool call.
type openaiToolCallFunctionWire struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openaiResponse represents the JSON response from the OpenAI Chat Completions API.
type openaiResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   *string              `json:"content"`
			ToolCalls []openaiToolCallWire `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// openaiErrorResponse represents an OpenAI API error response.
type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// convertToOpenAIMessages converts provider Messages to the OpenAI wire format.
func convertToOpenAIMessages(msgs []Message) []openaiMessage {
	var result []openaiMessage
	for _, msg := range msgs {
		if msg.Role == "tool" && len(msg.ToolResults) > 0 {
			// Each ToolResult becomes a separate "tool" message
			for _, tr := range msg.ToolResults {
				content := tr.Content
				result = append(result, openaiMessage{
					Role:       "tool",
					Content:    &content,
					ToolCallID: tr.CallID,
				})
			}
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Assistant message with tool calls
			var toolCalls []openaiToolCallWire
			for _, tc := range msg.ToolCalls {
				// Encode arguments as JSON string
				argsMap := make(map[string]interface{})
				for k, v := range tc.Arguments {
					argsMap[k] = v
				}
				argsJSON, _ := json.Marshal(argsMap)
				toolCalls = append(toolCalls, openaiToolCallWire{
					ID:   tc.ID,
					Type: "function",
					Function: openaiToolCallFunctionWire{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
			var contentPtr *string
			if msg.Content != "" {
				contentPtr = &msg.Content
			}
			result = append(result, openaiMessage{
				Role:      "assistant",
				Content:   contentPtr,
				ToolCalls: toolCalls,
			})
		} else {
			// Standard text message
			content := msg.Content
			result = append(result, openaiMessage{
				Role:    msg.Role,
				Content: &content,
			})
		}
	}
	return result
}

// convertToOpenAITools converts provider Tools to the OpenAI wire format.
func convertToOpenAITools(tools []Tool) []openaiToolDef {
	var result []openaiToolDef
	for _, tool := range tools {
		properties := make(map[string]interface{})
		var required []string
		for name, param := range tool.Parameters {
			properties[name] = map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}
			if param.Required {
				required = append(required, name)
			}
		}

		params := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			params["required"] = required
		}

		result = append(result, openaiToolDef{
			Type: "function",
			Function: openaiToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}
	return result
}

// Send makes a completion request to the OpenAI Chat Completions API.
func (o *OpenAI) Send(ctx context.Context, req *Request) (*Response, error) {
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

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

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
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, o.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	var apiResp openaiResponse
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

	return &Response{
		Content:      content,
		Model:        apiResp.Model,
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
		StopReason:   apiResp.Choices[0].FinishReason,
		ToolCalls:    toolCalls,
	}, nil
}

// handleErrorResponse maps HTTP error responses to ProviderError.
func (o *OpenAI) handleErrorResponse(status int, body []byte) *ProviderError {
	message := http.StatusText(status)
	var errResp openaiErrorResponse
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
func (o *OpenAI) mapStatusToCategory(status int) ErrorCategory {
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
