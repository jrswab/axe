package provider

import (
	"encoding/json"
	"testing"
)

func TestMapStreamErrorType(t *testing.T) {
	tests := []struct {
		errorType string
		want      ErrorCategory
	}{
		{"overloaded_error", ErrCategoryOverloaded},
		{"rate_limit_error", ErrCategoryRateLimit},
		{"api_error", ErrCategoryServer},
		{"authentication_error", ErrCategoryAuth},
		{"unknown_error", ErrCategoryServer},
	}

	for _, tt := range tests {
		t.Run(tt.errorType, func(t *testing.T) {
			got := mapStreamErrorType(tt.errorType)
			if got != tt.want {
				t.Errorf("mapStreamErrorType(%q) = %q, want %q", tt.errorType, got, tt.want)
			}
		})
	}
}

func TestAnthropicStreamEvent_Unmarshal(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		assert func(t *testing.T, e anthropicStreamEvent)
	}{
		{
			name: "message_start",
			json: `{"type":"message_start","message":{"id":"msg_abc","usage":{"input_tokens":25,"output_tokens":1}}}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "message_start" {
					t.Errorf("Type = %q, want %q", e.Type, "message_start")
				}
				if e.Message == nil {
					t.Fatal("Message is nil")
				}
				if e.Message.ID != "msg_abc" {
					t.Errorf("Message.ID = %q, want %q", e.Message.ID, "msg_abc")
				}
				if e.Message.Usage == nil || e.Message.Usage.InputTokens != 25 {
					t.Errorf("Message.Usage.InputTokens = %v, want 25", e.Message.Usage)
				}
			},
		},
		{
			name: "content_block_start_text",
			json: `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "content_block_start" {
					t.Errorf("Type = %q, want %q", e.Type, "content_block_start")
				}
				if e.Index != 0 {
					t.Errorf("Index = %d, want 0", e.Index)
				}
				if e.ContentBlock == nil || e.ContentBlock.Type != "text" {
					t.Errorf("ContentBlock.Type = %v, want text", e.ContentBlock)
				}
			},
		},
		{
			name: "content_block_start_tool_use",
			json: `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_abc","name":"read_file"}}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "content_block_start" {
					t.Errorf("Type = %q, want %q", e.Type, "content_block_start")
				}
				if e.Index != 1 {
					t.Errorf("Index = %d, want 1", e.Index)
				}
				if e.ContentBlock == nil {
					t.Fatal("ContentBlock is nil")
				}
				if e.ContentBlock.Type != "tool_use" {
					t.Errorf("ContentBlock.Type = %q, want %q", e.ContentBlock.Type, "tool_use")
				}
				if e.ContentBlock.ID != "toolu_abc" {
					t.Errorf("ContentBlock.ID = %q, want %q", e.ContentBlock.ID, "toolu_abc")
				}
				if e.ContentBlock.Name != "read_file" {
					t.Errorf("ContentBlock.Name = %q, want %q", e.ContentBlock.Name, "read_file")
				}
			},
		},
		{
			name: "content_block_delta_text",
			json: `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "content_block_delta" {
					t.Errorf("Type = %q, want %q", e.Type, "content_block_delta")
				}
				if e.Index != 0 {
					t.Errorf("Index = %d, want 0", e.Index)
				}
				if e.Delta == nil || e.Delta.Type != "text_delta" || e.Delta.Text != "Hello" {
					t.Errorf("Delta = %+v, want type=text_delta text=Hello", e.Delta)
				}
			},
		},
		{
			name: "content_block_delta_input_json",
			json: `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "content_block_delta" {
					t.Errorf("Type = %q, want %q", e.Type, "content_block_delta")
				}
				if e.Delta == nil || e.Delta.Type != "input_json_delta" || e.Delta.PartialJSON != `{"path":` {
					t.Errorf("Delta = %+v, want type=input_json_delta partial_json={\"path\":", e.Delta)
				}
			},
		},
		{
			name: "content_block_stop",
			json: `{"type":"content_block_stop","index":0}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "content_block_stop" {
					t.Errorf("Type = %q, want %q", e.Type, "content_block_stop")
				}
				if e.Index != 0 {
					t.Errorf("Index = %d, want 0", e.Index)
				}
			},
		},
		{
			name: "message_delta",
			json: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "message_delta" {
					t.Errorf("Type = %q, want %q", e.Type, "message_delta")
				}
				if e.Delta == nil || e.Delta.StopReason != "end_turn" {
					t.Errorf("Delta.StopReason = %v, want end_turn", e.Delta)
				}
				if e.Usage == nil || e.Usage.OutputTokens != 15 {
					t.Errorf("Usage.OutputTokens = %v, want 15", e.Usage)
				}
			},
		},
		{
			name: "message_stop",
			json: `{"type":"message_stop"}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "message_stop" {
					t.Errorf("Type = %q, want %q", e.Type, "message_stop")
				}
			},
		},
		{
			name: "ping",
			json: `{"type":"ping"}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "ping" {
					t.Errorf("Type = %q, want %q", e.Type, "ping")
				}
			},
		},
		{
			name: "error",
			json: `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
			assert: func(t *testing.T, e anthropicStreamEvent) {
				if e.Type != "error" {
					t.Errorf("Type = %q, want %q", e.Type, "error")
				}
				if e.Error == nil {
					t.Fatal("Error is nil")
				}
				if e.Error.Type != "overloaded_error" {
					t.Errorf("Error.Type = %q, want %q", e.Error.Type, "overloaded_error")
				}
				if e.Error.Message != "Overloaded" {
					t.Errorf("Error.Message = %q, want %q", e.Error.Message, "Overloaded")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(tt.json), &event); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			tt.assert(t, event)
		})
	}
}
