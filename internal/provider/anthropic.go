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
	// defaultBaseURL is the default Anthropic API base URL.
	defaultBaseURL = "https://api.anthropic.com"

	// defaultMaxTokens is the default max_tokens value when none is specified.
	// The Anthropic API requires max_tokens to be present and greater than zero.
	defaultMaxTokens = 4096

	// anthropicVersion is the required API version header value.
	anthropicVersion = "2023-06-01"
)

// AnthropicOption is a functional option for configuring the Anthropic provider.
type AnthropicOption func(*Anthropic)

// WithBaseURL sets a custom base URL (used for testing with httptest).
func WithBaseURL(url string) AnthropicOption {
	return func(a *Anthropic) {
		a.baseURL = url
	}
}

// Anthropic implements the Provider interface for the Anthropic Messages API.
type Anthropic struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewAnthropic creates a new Anthropic provider. Returns an error if apiKey is empty.
func NewAnthropic(apiKey string, opts ...AnthropicOption) (*Anthropic, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	a := &Anthropic{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

// anthropicRequest is the JSON body sent to the Anthropic Messages API.
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	Tools       []anthropicToolDef `json:"tools,omitempty"`
}

// anthropicMessage is the wire format for a message in the Anthropic API.
type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []anthropicContentBlock
}

// anthropicContentBlock is a content block in an Anthropic message.
// Note: We use pointers/interfaces for optional fields so that omitempty works
// correctly. For tool_result blocks, is_error must always be present.
type anthropicContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   string                 `json:"content,omitempty"`
	IsError   *bool                  `json:"is_error,omitempty"`
}

// anthropicToolDef is the wire format for a tool definition in the Anthropic API.
type anthropicToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicResponse represents the JSON response from the Anthropic Messages API.
type anthropicResponse struct {
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

// anthropicErrorResponse represents an Anthropic API error response.
type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// convertToAnthropicMessages converts provider Messages to the Anthropic wire format.
func convertToAnthropicMessages(msgs []Message) []anthropicMessage {
	var result []anthropicMessage
	for _, msg := range msgs {
		am := anthropicMessage{Role: msg.Role}

		if msg.Role == "tool" && len(msg.ToolResults) > 0 {
			// Tool result messages are sent as role "user" with tool_result content blocks
			am.Role = "user"
			var blocks []anthropicContentBlock
			for _, tr := range msg.ToolResults {
				isErr := tr.IsError
				blocks = append(blocks, anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: tr.CallID,
					Content:   tr.Content,
					IsError:   &isErr,
				})
			}
			am.Content = blocks
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Assistant messages with tool calls need content blocks
			var blocks []anthropicContentBlock
			if msg.Content != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				input := make(map[string]interface{})
				for k, v := range tc.Arguments {
					input[k] = v
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			am.Content = blocks
		} else {
			// Standard text message
			am.Content = msg.Content
		}

		result = append(result, am)
	}
	return result
}

// convertToAnthropicTools converts provider Tools to the Anthropic wire format.
func convertToAnthropicTools(tools []Tool) []anthropicToolDef {
	var result []anthropicToolDef
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

		schema := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			schema["required"] = required
		}

		result = append(result, anthropicToolDef{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		})
	}
	return result
}

// Send makes a completion request to the Anthropic Messages API.
func (a *Anthropic) Send(ctx context.Context, req *Request) (*Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  convertToAnthropicMessages(req.Messages),
		System:    req.System,
	}

	if req.Temperature != 0 {
		temp := req.Temperature
		body.Temperature = &temp
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToAnthropicTools(req.Tools)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		// Check for context cancellation/deadline exceeded
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

	// Handle non-2xx responses
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, a.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	// Parse success response
	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	// Check for empty content array
	if len(apiResp.Content) == 0 {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  "response contains no content",
		}
	}

	// Parse response content blocks
	var textContent string
	var toolCalls []ToolCall
	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			args := make(map[string]string)
			for k, v := range block.Input {
				args[k] = fmt.Sprintf("%v", v)
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return &Response{
		Content:      textContent,
		Model:        apiResp.Model,
		InputTokens:  apiResp.Usage.InputTokens,
		OutputTokens: apiResp.Usage.OutputTokens,
		StopReason:   apiResp.StopReason,
		ToolCalls:    toolCalls,
	}, nil
}

// handleErrorResponse maps HTTP error responses to ProviderError.
func (a *Anthropic) handleErrorResponse(status int, body []byte) *ProviderError {
	// Attempt to parse Anthropic error response
	message := http.StatusText(status)
	var errResp anthropicErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	category := a.mapStatusToCategory(status)

	return &ProviderError{
		Category: category,
		Status:   status,
		Message:  message,
	}
}

// mapStatusToCategory maps HTTP status codes to error categories.
func (a *Anthropic) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 401:
		return ErrCategoryAuth
	case 400:
		return ErrCategoryBadRequest
	case 429:
		return ErrCategoryRateLimit
	case 529:
		return ErrCategoryOverloaded
	case 500, 502, 503:
		return ErrCategoryServer
	default:
		return ErrCategoryServer
	}
}
