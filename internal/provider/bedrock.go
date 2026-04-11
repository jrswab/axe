package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// BedrockOption is a functional option for configuring the Bedrock provider.
type BedrockOption func(*Bedrock)

// WithBedrockRegion sets a custom region.
func WithBedrockRegion(region string) BedrockOption {
	return func(b *Bedrock) { b.region = region }
}

// WithBedrockBaseURL sets a custom base URL (used for testing with httptest).
func WithBedrockBaseURL(url string) BedrockOption {
	return func(b *Bedrock) { b.baseURL = url }
}

// withBedrockCreds sets credentials directly (used for testing).
func withBedrockCreds(creds *awsCredentials) BedrockOption {
	return func(b *Bedrock) { b.creds = creds }
}

// Bedrock implements the Provider interface for AWS Bedrock.
type Bedrock struct {
	region  string
	baseURL string
	creds   *awsCredentials
	client  *http.Client
}

// NewBedrock creates a new Bedrock provider.
func NewBedrock(region string, opts ...BedrockOption) (*Bedrock, error) {
	b := &Bedrock{region: region}
	for _, opt := range opts {
		opt(b)
	}
	if b.region == "" {
		return nil, fmt.Errorf("region is required (set AWS_REGION environment variable or configure in config.toml)")
	}
	if b.baseURL == "" {
		b.baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", b.region)
	}
	if b.creds == nil {
		creds, err := resolveCredentials("")
		if err != nil {
			return nil, err
		}
		b.creds = creds
	}
	b.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return b, nil
}

// bedrockRequest is the JSON body sent to the Bedrock Converse API.
type bedrockRequest struct {
	Messages        []bedrockMessage        `json:"messages"`
	System          []bedrockSystemBlock     `json:"system,omitempty"`
	InferenceConfig *bedrockInferenceConfig  `json:"inferenceConfig,omitempty"`
	ToolConfig      *bedrockToolConfig       `json:"toolConfig,omitempty"`
}

// bedrockMessage is the wire format for a message in the Bedrock API.
type bedrockMessage struct {
	Role    string         `json:"role"`
	Content []bedrockBlock `json:"content"`
}

// bedrockBlock is a content block in a Bedrock message.
type bedrockBlock struct {
	Text       string              `json:"text,omitempty"`
	ToolUse    *bedrockToolUse     `json:"toolUse,omitempty"`
	ToolResult *bedrockToolResult  `json:"toolResult,omitempty"`
}

// bedrockToolUse represents a tool invocation in a Bedrock response.
type bedrockToolUse struct {
	ToolUseID string                 `json:"toolUseId"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
}

// bedrockToolResult represents a tool result sent back to Bedrock.
type bedrockToolResult struct {
	ToolUseID string                    `json:"toolUseId"`
	Content   []bedrockToolResultContent `json:"content"`
	Status    string                    `json:"status"`
}

// bedrockToolResultContent is the text content inside a tool result.
type bedrockToolResultContent struct {
	Text string `json:"text,omitempty"`
}

// bedrockSystemBlock is a system prompt block in the Bedrock API.
type bedrockSystemBlock struct {
	Text string `json:"text"`
}

// bedrockInferenceConfig holds temperature and max token settings.
type bedrockInferenceConfig struct {
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"maxTokens,omitempty"`
}

// bedrockToolConfig wraps the tool definitions sent to Bedrock.
type bedrockToolConfig struct {
	Tools []bedrockToolDef `json:"tools"`
}

// bedrockToolDef is the wire format for a tool definition in the Bedrock API.
type bedrockToolDef struct {
	ToolSpec bedrockToolSpec `json:"toolSpec"`
}

// bedrockToolSpec is the tool specification inside a bedrockToolDef.
type bedrockToolSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// bedrockResponse represents the JSON response from the Bedrock Converse API.
type bedrockResponse struct {
	Output     bedrockOutput `json:"output"`
	StopReason string        `json:"stopReason"`
	Usage      bedrockUsage  `json:"usage"`
}

// bedrockOutput wraps the message in a Bedrock response.
type bedrockOutput struct {
	Message *bedrockMessage `json:"message,omitempty"`
}

// bedrockUsage contains token usage information from the Bedrock response.
type bedrockUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// bedrockErrorResponse represents a Bedrock API error response.
type bedrockErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// buildBedrockRequest converts a provider Request to the Bedrock wire format.
func buildBedrockRequest(req *Request) bedrockRequest {
	br := bedrockRequest{
		Messages: convertMessages(req.Messages),
	}
	if req.System != "" {
		br.System = []bedrockSystemBlock{{Text: req.System}}
	}
	if req.Temperature != 0 || req.MaxTokens > 0 {
		br.InferenceConfig = &bedrockInferenceConfig{}
		if req.Temperature != 0 {
			t := req.Temperature
			br.InferenceConfig.Temperature = &t
		}
		if req.MaxTokens > 0 {
			m := req.MaxTokens
			br.InferenceConfig.MaxTokens = &m
		}
	}
	if len(req.Tools) > 0 {
		br.ToolConfig = &bedrockToolConfig{Tools: convertTools(req.Tools)}
	}
	return br
}

// convertMessages converts provider Messages to the Bedrock wire format.
func convertMessages(msgs []Message) []bedrockMessage {
	result := make([]bedrockMessage, 0, len(msgs))
	for _, msg := range msgs {
		bm := bedrockMessage{Role: msg.Role}
		if msg.Role != "user" && msg.Role != "assistant" {
			bm.Role = "assistant"
		}
		if msg.Content != "" {
			bm.Content = append(bm.Content, bedrockBlock{Text: msg.Content})
		}
		for _, tc := range msg.ToolCalls {
			input := make(map[string]interface{}, len(tc.Arguments))
			for k, v := range tc.Arguments {
				input[k] = v
			}
			bm.Content = append(bm.Content, bedrockBlock{ToolUse: &bedrockToolUse{
				ToolUseID: tc.ID, Name: tc.Name, Input: input,
			}})
		}
		for _, tr := range msg.ToolResults {
			status := "success"
			if tr.IsError {
				status = "error"
			}
			bm.Content = append(bm.Content, bedrockBlock{ToolResult: &bedrockToolResult{
				ToolUseID: tr.CallID,
				Content:   []bedrockToolResultContent{{Text: tr.Content}},
				Status:    status,
			}})
		}
		result = append(result, bm)
	}
	return result
}

// convertTools converts provider Tools to the Bedrock wire format.
func convertTools(tools []Tool) []bedrockToolDef {
	result := make([]bedrockToolDef, len(tools))
	for i, tool := range tools {
		props := make(map[string]interface{}, len(tool.Parameters))
		var required []string
		for name, param := range tool.Parameters {
			props[name] = map[string]interface{}{"type": param.Type, "description": param.Description}
			if param.Required {
				required = append(required, name)
			}
		}
		schema := map[string]interface{}{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		result[i] = bedrockToolDef{ToolSpec: bedrockToolSpec{
			Name: tool.Name, Description: tool.Description, InputSchema: schema,
		}}
	}
	return result
}

// parseBedrockResponse converts a Bedrock API response to a provider Response.
func parseBedrockResponse(resp *bedrockResponse, model string) *Response {
	r := &Response{
		Model:        model,
		StopReason:   resp.StopReason,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}
	if resp.Output.Message == nil {
		return r
	}
	for _, block := range resp.Output.Message.Content {
		if block.Text != "" {
			r.Content += block.Text
		}
		if block.ToolUse != nil {
			args := make(map[string]string, len(block.ToolUse.Input))
			for k, v := range block.ToolUse.Input {
				args[k] = fmt.Sprintf("%v", v)
			}
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID: block.ToolUse.ToolUseID, Name: block.ToolUse.Name, Arguments: args,
			})
		}
	}
	return r
}

// mapStatusToCategory maps HTTP status codes to error categories.
func (b *Bedrock) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 401, 403:
		return ErrCategoryAuth
	case 429:
		return ErrCategoryRateLimit
	case 400:
		return ErrCategoryBadRequest
	default:
		return ErrCategoryServer
	}
}

// SupportsFormat returns false as Bedrock structured output is not yet implemented.
func (b *Bedrock) SupportsFormat(format *ResponseFormat) bool {
	return false
}

// Send makes a completion request to the AWS Bedrock Converse API.
func (b *Bedrock) Send(ctx context.Context, req *Request) (*Response, error) {
	body, err := json.Marshal(buildBedrockRequest(req))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/model/%s/converse", b.baseURL, req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	signRequest(httpReq, b.creds, b.region, "bedrock", body, time.Now())

	httpResp, err := b.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{Category: ErrCategoryTimeout, Message: ctx.Err().Error(), Err: ctx.Err()}
		}
		return nil, &ProviderError{Category: ErrCategoryServer, Message: err.Error(), Err: err}
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		msg := http.StatusText(httpResp.StatusCode)
		var errResp bedrockErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			msg = errResp.Message
		}
		return nil, &ProviderError{
			Category: b.mapStatusToCategory(httpResp.StatusCode),
			Status:   httpResp.StatusCode,
			Message:  msg,
		}
	}

	var apiResp bedrockResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	if apiResp.Output.Message == nil {
		fmt.Fprintf(os.Stderr, "bedrock: response contains no message content\n")
	}

	return parseBedrockResponse(&apiResp, req.Model), nil
}
