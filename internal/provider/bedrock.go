package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
)

// BedrockOption is a functional option for configuring the Bedrock provider.
type BedrockOption func(*Bedrock)

// WithBedrockRegion sets a custom region.
func WithBedrockRegion(region string) BedrockOption {
	return func(b *Bedrock) {
		b.region = region
	}
}

// Bedrock implements the Provider interface for AWS Bedrock.
type Bedrock struct {
	client *bedrockruntime.Client
	region string
}

// NewBedrock creates a new Bedrock provider. Returns an error if region is empty.
func NewBedrock(region string, opts ...BedrockOption) (*Bedrock, error) {
	b := &Bedrock{
		region: region,
	}

	for _, opt := range opts {
		opt(b)
	}

	if b.region == "" {
		return nil, fmt.Errorf("region is required (set AWS_REGION environment variable or configure in config.toml)")
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(b.region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	b.client = bedrockruntime.NewFromConfig(cfg)

	return b, nil
}

func convertToBedrockSystemPrompts(system string) []types.SystemContentBlock {
	if system == "" {
		return nil
	}
	return []types.SystemContentBlock{
		&types.SystemContentBlockMemberText{Value: system},
	}
}

func convertToBedrockTools(tools []Tool) []types.Tool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]types.Tool, len(tools))
	for i, tool := range tools {
		inputSchema := map[string]interface{}{
			"type":       "object",
			"properties": make(map[string]interface{}),
		}

		required := []interface{}{}
		props := inputSchema["properties"].(map[string]interface{})

		for name, param := range tool.Parameters {
			props[name] = map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}
			if param.Required {
				required = append(required, name)
			}
		}

		if len(required) > 0 {
			inputSchema["required"] = required
		}

		result[i] = &types.ToolMemberToolSpec{
			Value: types.ToolSpecification{
				Name:        &tool.Name,
				Description: &tool.Description,
				InputSchema: &types.ToolInputSchemaMemberJson{
					Value: document.NewLazyDocument(inputSchema),
				},
			},
		}
	}
	return result
}

func convertToBedrockMessages(messages []Message) []types.Message {
	result := make([]types.Message, 0, len(messages))

	for _, msg := range messages {
		var role types.ConversationRole
		if msg.Role == "user" {
			role = types.ConversationRoleUser
		} else {
			role = types.ConversationRoleAssistant
		}

		content := make([]types.ContentBlock, 0)

		// Add text content
		if msg.Content != "" {
			content = append(content, &types.ContentBlockMemberText{
				Value: msg.Content,
			})
		}

		// Add tool calls (assistant messages)
		for _, tc := range msg.ToolCalls {
			// Convert string map to interface map for document
			input := make(map[string]interface{})
			for k, v := range tc.Arguments {
				input[k] = v
			}
			
			content = append(content, &types.ContentBlockMemberToolUse{
				Value: types.ToolUseBlock{
					ToolUseId: &tc.ID,
					Name:      &tc.Name,
					Input:     document.NewLazyDocument(input),
				},
			})
		}

		// Add tool results (user messages with tool results)
		for _, tr := range msg.ToolResults {
			status := types.ToolResultStatusSuccess
			if tr.IsError {
				status = types.ToolResultStatusError
			}

			content = append(content, &types.ContentBlockMemberToolResult{
				Value: types.ToolResultBlock{
					ToolUseId: &tr.CallID,
					Content: []types.ToolResultContentBlock{
						&types.ToolResultContentBlockMemberText{Value: tr.Content},
					},
					Status: status,
				},
			})
		}

		result = append(result, types.Message{
			Role:    role,
			Content: content,
		})
	}

	return result
}

func convertFromBedrockResponse(output types.ConverseOutput, usage *types.TokenUsage, stopReason types.StopReason, modelID string) *Response {
	resp := &Response{
		Model:      modelID,
		StopReason: string(stopReason),
	}

	if usage != nil {
		if usage.InputTokens != nil {
			resp.InputTokens = int(*usage.InputTokens)
		}
		if usage.OutputTokens != nil {
			resp.OutputTokens = int(*usage.OutputTokens)
		}
	}

	msg, ok := output.(*types.ConverseOutputMemberMessage)
	if !ok || msg.Value.Content == nil {
		return resp
	}

	var textContent string
	var toolCalls []ToolCall

	for _, block := range msg.Value.Content {
		switch v := block.(type) {
		case *types.ContentBlockMemberText:
			textContent += v.Value
		case *types.ContentBlockMemberToolUse:
			if v.Value.ToolUseId == nil || v.Value.Name == nil {
				continue // Skip malformed tool use blocks
			}
			args := make(map[string]string)
			var inputMap map[string]interface{}
			if err := v.Value.Input.UnmarshalSmithyDocument(&inputMap); err != nil {
				fmt.Fprintf(os.Stderr, "bedrock: failed to unmarshal tool input for %s: %v\n", *v.Value.Name, err)
			} else {
				for k, val := range inputMap {
					args[k] = fmt.Sprintf("%v", val)
				}
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:        *v.Value.ToolUseId,
				Name:      *v.Value.Name,
				Arguments: args,
			})
		}
	}

	resp.Content = textContent
	resp.ToolCalls = toolCalls

	return resp
}

func mapBedrockError(err error) error {
	if err == nil {
		return nil
	}

	// Check for context errors first
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &ProviderError{
			Category: ErrCategoryTimeout,
			Message:  "request timeout",
			Err:      err,
		}
	}

	// Use AWS SDK v2 structured error types
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		
		switch code {
		case "AccessDeniedException", "UnauthorizedException":
			return &ProviderError{
				Category: ErrCategoryAuth,
				Message:  "authentication failed",
				Err:      err,
			}
		case "ThrottlingException", "TooManyRequestsException":
			return &ProviderError{
				Category: ErrCategoryRateLimit,
				Message:  "rate limit exceeded",
				Err:      err,
			}
		case "ValidationException", "InvalidRequestException":
			return &ProviderError{
				Category: ErrCategoryBadRequest,
				Message:  "invalid request",
				Err:      err,
			}
		case "ServiceUnavailableException", "InternalServerException":
			return &ProviderError{
				Category: ErrCategoryServer,
				Message:  "server error",
				Err:      err,
			}
		}
	}

	// Network errors - check for timeouts first
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return &ProviderError{
				Category: ErrCategoryTimeout,
				Message:  "request timeout",
				Err:      err,
			}
		}
		return &ProviderError{
			Category: ErrCategoryServer,
			Message:  "network error",
			Err:      err,
		}
	}

	// Default to server error for unknown AWS errors
	return &ProviderError{
		Category: ErrCategoryServer,
		Message:  err.Error(),
		Err:      err,
	}
}

// Send implements the Provider interface.
func (b *Bedrock) Send(ctx context.Context, req *Request) (*Response, error) {
	input := &bedrockruntime.ConverseInput{
		ModelId:  &req.Model,
		Messages: convertToBedrockMessages(req.Messages),
	}

	if req.System != "" {
		input.System = convertToBedrockSystemPrompts(req.System)
	}

	if len(req.Tools) > 0 {
		toolConfig := &types.ToolConfiguration{
			Tools: convertToBedrockTools(req.Tools),
		}
		input.ToolConfig = toolConfig
	}

	inferenceConfig := &types.InferenceConfiguration{}
	if req.Temperature != 0 {
		temp := float32(req.Temperature)
		inferenceConfig.Temperature = &temp
	}
	if req.MaxTokens > 0 {
		maxTokens := int32(req.MaxTokens)
		inferenceConfig.MaxTokens = &maxTokens
	}
	if req.Temperature != 0 || req.MaxTokens > 0 {
		input.InferenceConfig = inferenceConfig
	}

	output, err := b.client.Converse(ctx, input)
	if err != nil {
		return nil, mapBedrockError(err)
	}

	return convertFromBedrockResponse(output.Output, output.Usage, output.StopReason, req.Model), nil
}
