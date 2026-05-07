package runner

import (
	"fmt"
	"strings"

	"github.com/jrswab/axe/internal/provider"
)

// validRoles defines the allowed message roles.
var validRoles = map[string]bool{
	"user":      true,
	"assistant": true,
	"tool":      true,
}

// validateMessages validates a pre-built message slice according to the
// rules in REQ-4 through REQ-6.
func validateMessages(msgs []Message) error {
	var activeCallIDs map[string]bool

	for i, m := range msgs {
		if !validRoles[m.Role] {
			return &ConfigError{Msg: fmt.Sprintf("invalid message role %q at index %d", m.Role, i)}
		}

		switch m.Role {
		case "user":
			if len(m.ToolCalls) > 0 {
				return &ConfigError{Msg: fmt.Sprintf("user message at index %d must not have tool_calls", i)}
			}
			if len(m.ToolResults) > 0 {
				return &ConfigError{Msg: fmt.Sprintf("user message at index %d must not have tool_results", i)}
			}

		case "assistant":
			if len(m.ToolResults) > 0 {
				return &ConfigError{Msg: fmt.Sprintf("assistant message at index %d must not have tool_results", i)}
			}
			// Collect CallIDs from this assistant message for subsequent tool messages
			activeCallIDs = make(map[string]bool, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				activeCallIDs[tc.ID] = true
			}

		case "tool":
			if strings.TrimSpace(m.Content) != "" {
				return &ConfigError{Msg: fmt.Sprintf("tool message at index %d must have empty content", i)}
			}
			if len(m.ToolCalls) > 0 {
				return &ConfigError{Msg: fmt.Sprintf("tool message at index %d must not have tool_calls", i)}
			}
			if activeCallIDs == nil {
				return &ConfigError{Msg: fmt.Sprintf("tool message at index %d has no preceding assistant message with tool calls", i)}
			}
			for _, tr := range m.ToolResults {
				if !activeCallIDs[tr.CallID] {
					return &ConfigError{Msg: fmt.Sprintf("tool result at index %d has unmatched call_id %q", i, tr.CallID)}
				}
			}
		}
	}

	return nil
}

// firstUserMessageContent returns the Content of the first user message
// in the slice, or empty string if none exists.
func firstUserMessageContent(msgs []Message) string {
	for _, m := range msgs {
		if m.Role == "user" {
			return m.Content
		}
	}
	return ""
}

// fromProviderMessages converts a slice of provider messages to runner messages.
func fromProviderMessages(msgs []provider.Message) []Message {
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[i] = fromProviderMessage(m)
	}
	return out
}

// Message represents a single message in a conversation history.
// This is the public-facing type for callers who need to seed or
// receive full message histories through runner.Options/runner.Result.
type Message struct {
	Role        string
	Content     string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]string
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	CallID  string
	Content string
	IsError bool
}

// toProviderMessages converts a slice of runner messages to provider messages.
func toProviderMessages(msgs []Message) []provider.Message {
	out := make([]provider.Message, len(msgs))
	for i, m := range msgs {
		out[i] = provider.Message{
			Role:        m.Role,
			Content:     m.Content,
			ToolCalls:   toProviderToolCalls(m.ToolCalls),
			ToolResults: toProviderToolResults(m.ToolResults),
		}
	}
	return out
}

// fromProviderMessage converts a single provider message to a runner message.
func fromProviderMessage(pm provider.Message) Message {
	return Message{
		Role:        pm.Role,
		Content:     pm.Content,
		ToolCalls:   fromProviderToolCalls(pm.ToolCalls),
		ToolResults: fromProviderToolResults(pm.ToolResults),
	}
}

// toProviderToolCalls converts runner ToolCalls to provider ToolCalls.
func toProviderToolCalls(tcs []ToolCall) []provider.ToolCall {
	if tcs == nil {
		return nil
	}
	out := make([]provider.ToolCall, len(tcs))
	for i, tc := range tcs {
		args := make(map[string]string, len(tc.Arguments))
		for k, v := range tc.Arguments {
			args[k] = v
		}
		out[i] = provider.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: args,
		}
	}
	return out
}

// fromProviderToolCalls converts provider ToolCalls to runner ToolCalls.
func fromProviderToolCalls(tcs []provider.ToolCall) []ToolCall {
	if tcs == nil {
		return nil
	}
	out := make([]ToolCall, len(tcs))
	for i, tc := range tcs {
		args := make(map[string]string, len(tc.Arguments))
		for k, v := range tc.Arguments {
			args[k] = v
		}
		out[i] = ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: args,
		}
	}
	return out
}

// toProviderToolResults converts runner ToolResults to provider ToolResults.
func toProviderToolResults(trs []ToolResult) []provider.ToolResult {
	if trs == nil {
		return nil
	}
	out := make([]provider.ToolResult, len(trs))
	for i, tr := range trs {
		out[i] = provider.ToolResult{
			CallID:  tr.CallID,
			Content: tr.Content,
			IsError: tr.IsError,
		}
	}
	return out
}

// fromProviderToolResults converts provider ToolResults to runner ToolResults.
func fromProviderToolResults(trs []provider.ToolResult) []ToolResult {
	if trs == nil {
		return nil
	}
	out := make([]ToolResult, len(trs))
	for i, tr := range trs {
		out[i] = ToolResult{
			CallID:  tr.CallID,
			Content: tr.Content,
			IsError: tr.IsError,
		}
	}
	return out
}
