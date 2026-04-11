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
	// defaultGeminiBaseURL is the default Google Gemini API base URL.
	defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"
)

// GeminiOption is a functional option for configuring the Gemini provider.
type GeminiOption func(*Gemini)

// WithGeminiBaseURL sets a custom base URL for the Gemini provider.
func WithGeminiBaseURL(url string) GeminiOption {
	return func(g *Gemini) {
		g.baseURL = url
	}
}

// Gemini implements the Provider interface for the Google Gemini API.
type Gemini struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewGemini creates a new Gemini provider. An API key is required.
func NewGemini(apiKey string, opts ...GeminiOption) (*Gemini, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	g := &Gemini{
		apiKey:  apiKey,
		baseURL: defaultGeminiBaseURL,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(g)
	}

	return g, nil
}

// --- Gemini wire types ---

type geminiRequest struct {
	SystemInstruction *geminiSystemInstruction `json:"system_instruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	Tools             []geminiToolDef          `json:"tools,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                          `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall             `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponseEnvelope `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type geminiFunctionResponseEnvelope struct {
	Name     string                        `json:"name"`
	Response geminiFunctionResponsePayload `json:"response"`
}

type geminiFunctionResponsePayload struct {
	Result string `json:"result"`
}

type geminiToolDef struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type geminiGenerationConfig struct {
	Temperature      *float64               `json:"temperature,omitempty"`
	MaxOutputTokens  *int                   `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string                 `json:"response_mime_type,omitempty"`
	ResponseSchema   map[string]interface{} `json:"response_schema,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content      *geminiContent `json:"content"`
	FinishReason string         `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiErrorResponse struct {
	Error geminiErrorDetail `json:"error"`
}

type geminiErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// --- Message conversion ---

// convertToGeminiContents converts provider Messages to the Gemini wire format.
func convertToGeminiContents(msgs []Message) []geminiContent {
	// Build a callID→toolName map from assistant tool calls for resolving tool results.
	callIDToName := make(map[string]string)
	for _, msg := range msgs {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				callIDToName[tc.ID] = tc.Name
			}
		}
	}

	var result []geminiContent
	for _, msg := range msgs {
		switch {
		case msg.Role == "tool" && len(msg.ToolResults) > 0:
			// Tool results become user messages with functionResponse parts.
			var parts []geminiPart
			for _, tr := range msg.ToolResults {
				name := callIDToName[tr.CallID]
				if name == "" {
					name = tr.CallID // fallback
				}
				parts = append(parts, geminiPart{
					FunctionResponse: &geminiFunctionResponseEnvelope{
						Name: name,
						Response: geminiFunctionResponsePayload{
							Result: tr.Content,
						},
					},
				})
			}
			result = append(result, geminiContent{
				Role:  "user",
				Parts: parts,
			})

		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			// Assistant messages with tool calls become model messages with functionCall parts.
			var parts []geminiPart
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				args := make(map[string]interface{})
				for k, v := range tc.Arguments {
					args[k] = v
				}
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: args,
					},
				})
			}
			result = append(result, geminiContent{
				Role:  "model",
				Parts: parts,
			})

		default:
			// Standard text message.
			role := msg.Role
			if role == "assistant" {
				role = "model"
			}
			result = append(result, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: msg.Content}},
			})
		}
	}
	return result
}

// convertToGeminiTools converts provider Tools to the Gemini wire format.
func convertToGeminiTools(tools []Tool) []geminiToolDef {
	var declarations []geminiFunctionDeclaration
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

		declarations = append(declarations, geminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
		})
	}
	return []geminiToolDef{{FunctionDeclarations: declarations}}
}

// --- Send method ---

// SupportsFormat returns true as Gemini supports both JSON mode and JSON Schema.
func (g *Gemini) SupportsFormat(format *ResponseFormat) bool {
	return true
}

// Send makes a completion request to the Google Gemini API.
func (g *Gemini) Send(ctx context.Context, req *Request) (*Response, error) {
	body := geminiRequest{
		Contents: convertToGeminiContents(req.Messages),
	}

	if req.System != "" {
		body.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: req.System}},
		}
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToGeminiTools(req.Tools)
	}

	// Build generationConfig only if needed.
	var genCfg geminiGenerationConfig
	hasGenCfg := false
	if req.Temperature != 0 {
		temp := req.Temperature
		genCfg.Temperature = &temp
		hasGenCfg = true
	}
	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		genCfg.MaxOutputTokens = &mt
		hasGenCfg = true
	}

	if req.Format != nil {
		if req.Format.Type == FormatJSON {
			genCfg.ResponseMimeType = "application/json"
			hasGenCfg = true
		} else if req.Format.Type == FormatSchema {
			genCfg.ResponseMimeType = "application/json"
			genCfg.ResponseSchema = req.Format.Schema
			hasGenCfg = true
		}
	}

	if hasGenCfg {
		body.GenerationConfig = &genCfg
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", g.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
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
		return nil, g.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	var apiResp geminiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	if len(apiResp.Candidates) == 0 {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  "response contains no candidates",
		}
	}

	candidate := apiResp.Candidates[0]

	var content string
	var toolCalls []ToolCall
	toolCallIdx := 0

	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
			if part.FunctionCall != nil {
				args := make(map[string]string)
				for k, v := range part.FunctionCall.Args {
					args[k] = fmt.Sprintf("%v", v)
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:        fmt.Sprintf("gemini_%d", toolCallIdx),
					Name:      part.FunctionCall.Name,
					Arguments: args,
				})
				toolCallIdx++
			}
		}
	}

	resp := &Response{
		Content:    content,
		Model:      req.Model,
		StopReason: candidate.FinishReason,
	}

	if apiResp.UsageMetadata != nil {
		resp.InputTokens = apiResp.UsageMetadata.PromptTokenCount
		resp.OutputTokens = apiResp.UsageMetadata.CandidatesTokenCount
	}

	if len(toolCalls) > 0 {
		resp.ToolCalls = toolCalls
	}

	return resp, nil
}

// SendStream makes a streaming completion request to the Google Gemini API.
func (g *Gemini) SendStream(ctx context.Context, req *Request) (*EventStream, error) {
	body := geminiRequest{
		Contents: convertToGeminiContents(req.Messages),
	}

	if req.System != "" {
		body.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: req.System}},
		}
	}

	if len(req.Tools) > 0 {
		body.Tools = convertToGeminiTools(req.Tools)
	}

	var genCfg geminiGenerationConfig
	hasGenCfg := false
	if req.Temperature != 0 {
		temp := req.Temperature
		genCfg.Temperature = &temp
		hasGenCfg = true
	}
	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		genCfg.MaxOutputTokens = &mt
		hasGenCfg = true
	}

	if req.Format != nil {
		if req.Format.Type == FormatJSON {
			genCfg.ResponseMimeType = "application/json"
			hasGenCfg = true
		} else if req.Format.Type == FormatSchema {
			genCfg.ResponseMimeType = "application/json"
			genCfg.ResponseSchema = req.Format.Schema
			hasGenCfg = true
		}
	}

	if hasGenCfg {
		body.GenerationConfig = &genCfg
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", g.baseURL, req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", g.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
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
		return nil, g.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	parser := NewSSEParser(httpResp.Body)
	toolCallIdx := 0
	var finishReason string
	var inputTokens, outputTokens int
	doneEmitted := false
	var pending []StreamEvent

	nextFunc := func() (StreamEvent, error) {
		for {
			if len(pending) > 0 {
				ev := pending[0]
				pending = pending[1:]
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
					if !doneEmitted {
						doneEmitted = true
						return StreamEvent{
							Type:         StreamEventDone,
							StopReason:   finishReason,
							InputTokens:  inputTokens,
							OutputTokens: outputTokens,
						}, nil
					}
					return StreamEvent{}, io.EOF
				}
				return StreamEvent{}, &ProviderError{
					Category: ErrCategoryServer,
					Message:  fmt.Sprintf("stream read error: %s", err),
					Err:      err,
				}
			}

			var chunk geminiResponse
			if err := json.Unmarshal([]byte(sseEvent.Data), &chunk); err != nil {
				return StreamEvent{}, &ProviderError{
					Category: ErrCategoryServer,
					Message:  fmt.Sprintf("failed to parse streaming chunk: %s", err),
					Err:      err,
				}
			}

			if chunk.UsageMetadata != nil {
				inputTokens = chunk.UsageMetadata.PromptTokenCount
				outputTokens = chunk.UsageMetadata.CandidatesTokenCount
			}

			if len(chunk.Candidates) == 0 {
				continue
			}

			candidate := chunk.Candidates[0]
			if candidate.FinishReason != "" {
				finishReason = candidate.FinishReason
			}

			if candidate.Content == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					args := make(map[string]string)
					for k, v := range part.FunctionCall.Args {
						args[k] = fmt.Sprintf("%v", v)
					}
					id := fmt.Sprintf("gemini_%d", toolCallIdx)
					toolCallIdx++
					argsJSON, _ := json.Marshal(args)
					pending = append(pending, StreamEvent{
						Type:       StreamEventToolStart,
						ToolCallID: id,
						ToolName:   part.FunctionCall.Name,
						ToolInput:  string(argsJSON),
					})
					pending = append(pending, StreamEvent{
						Type:       StreamEventToolEnd,
						ToolCallID: id,
					})
				} else if part.Text != "" {
					pending = append(pending, StreamEvent{
						Type: StreamEventText,
						Text: part.Text,
					})
				}
			}

			continue
		}
	}

	return NewEventStream(httpResp.Body, nextFunc), nil
}

// handleErrorResponse maps HTTP error responses to ProviderError.
func (g *Gemini) handleErrorResponse(status int, body []byte) *ProviderError {
	message := http.StatusText(status)
	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	return &ProviderError{
		Category: g.mapStatusToCategory(status),
		Status:   status,
		Message:  message,
	}
}

// mapStatusToCategory maps HTTP status codes to error categories.
func (g *Gemini) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 401, 403:
		return ErrCategoryAuth
	case 400, 404:
		return ErrCategoryBadRequest
	case 429:
		return ErrCategoryRateLimit
	case 500, 502, 503:
		return ErrCategoryServer
	case 529:
		return ErrCategoryOverloaded
	default:
		return ErrCategoryServer
	}
}
