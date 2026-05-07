package runner

import (
	"encoding/json"
	"testing"
)

func TestResultMarshalJSON_TableDriven(t *testing.T) {
	cases := []struct {
		name  string
		input Result
		want  struct {
			Cost     bool
			CostVal  float64
			Cache    bool
			CacheVal string
		}
	}{
		{
			name:  "cost and cache status present",
			input: Result{Content: "Hello", Cost: 0.00014, CacheStatus: "HIT", InputTokens: 10, OutputTokens: 5, Model: "openrouter/test", StopReason: "stop"},
			want:  struct{ Cost bool; CostVal float64; Cache bool; CacheVal string }{Cost: true, CostVal: 0.00014, Cache: true, CacheVal: "HIT"},
		},
		{
			name:  "zero cost and empty cache omitted",
			input: Result{Content: "Hello", InputTokens: 10, OutputTokens: 5, Model: "openai/gpt-4", StopReason: "stop"},
			want:  struct{ Cost bool; CostVal float64; Cache bool; CacheVal string }{Cost: false, Cache: false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if tc.want.Cost {
				if cost, ok := parsed["cost"].(float64); !ok || cost != tc.want.CostVal {
					t.Errorf("expected cost=%v, got %v", tc.want.CostVal, parsed["cost"])
				}
			} else {
				if _, ok := parsed["cost"]; ok {
					t.Error("expected cost to be omitted")
				}
			}
			if tc.want.Cache {
				if cs, ok := parsed["cache_status"].(string); !ok || cs != tc.want.CacheVal {
					t.Errorf("expected cache_status=%q, got %v", tc.want.CacheVal, parsed["cache_status"])
				}
			} else {
				if _, ok := parsed["cache_status"]; ok {
					t.Error("expected cache_status to be omitted")
				}
			}
		})
	}
}

func TestResultMarshalJSON_WithMessages(t *testing.T) {
	result := Result{
		Content:     "Hello",
		InputTokens: 10,
		OutputTokens: 5,
		Model:       "openai/gpt-4",
		StopReason:  "stop",
		Messages: []Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "Hello"},
		},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	msgs, ok := parsed["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array, got %T", parsed["messages"])
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	first := msgs[0].(map[string]interface{})
	if first["role"] != "user" || first["content"] != "hi" {
		t.Errorf("first message = %v", first)
	}
	second := msgs[1].(map[string]interface{})
	if second["role"] != "assistant" || second["content"] != "Hello" {
		t.Errorf("second message = %v", second)
	}
	// tool_calls and tool_results must be present as empty arrays, not omitted
	if second["tool_calls"] == nil {
		t.Error("tool_calls should be present, not null/omitted")
	}
	if second["tool_results"] == nil {
		t.Error("tool_results should be present, not null/omitted")
	}
}

func TestResultMarshalJSON_WithMessagesAndToolCalls(t *testing.T) {
	result := Result{
		Content:     "Done",
		InputTokens: 10,
		OutputTokens: 5,
		Model:       "openai/gpt-4",
		StopReason:  "stop",
		Messages: []Message{
			{Role: "user", Content: "list"},
			{
				Role:    "assistant",
				Content: "listing",
				ToolCalls: []ToolCall{
					{ID: "tc1", Name: "list_directory", Arguments: map[string]string{"path": "/tmp"}},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{CallID: "tc1", Content: "a\nb", IsError: false},
				},
			},
		},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	msgs, ok := parsed["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array, got %T", parsed["messages"])
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	second := msgs[1].(map[string]interface{})
	tcs, ok := second["tool_calls"].([]interface{})
	if !ok || len(tcs) != 1 {
		t.Fatalf("expected 1 tool_call, got %v", second["tool_calls"])
	}
	tc := tcs[0].(map[string]interface{})
	if tc["id"] != "tc1" || tc["name"] != "list_directory" {
		t.Errorf("tool_call = %v", tc)
	}
	third := msgs[2].(map[string]interface{})
	trs, ok := third["tool_results"].([]interface{})
	if !ok || len(trs) != 1 {
		t.Fatalf("expected 1 tool_result, got %v", third["tool_results"])
	}
	tr := trs[0].(map[string]interface{})
	if tr["call_id"] != "tc1" || tr["content"] != "a\nb" || tr["is_error"] != false {
		t.Errorf("tool_result = %v", tr)
	}
}

func TestToolCallDetailJSON(t *testing.T) {
	detail := ToolCallDetail{
		Name:    "read_file",
		Input:   map[string]string{},
		Output:  "content",
		IsError: false,
	}

	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, key := range []string{"name", "input", "output", "is_error"} {
		if _, ok := parsed[key]; !ok {
			t.Fatalf("missing key %q in %s", key, string(data))
		}
	}

	if isErr, ok := parsed["is_error"].(bool); !ok || isErr {
		t.Fatalf("expected is_error=false, got %v", parsed["is_error"])
	}

	if inputRaw := parsed["input"]; inputRaw == nil {
		t.Fatalf("expected input to be object, got null")
	} else if inputMap, ok := inputRaw.(map[string]interface{}); !ok {
		t.Fatalf("expected input to be object, got %T", inputRaw)
	} else if len(inputMap) != 0 {
		t.Fatalf("expected empty input map, got %v", inputMap)
	}

	withInput := ToolCallDetail{
		Name:    "read_file",
		Input:   map[string]string{"path": "hello.txt", "mode": "full"},
		Output:  "ok",
		IsError: false,
	}
	data2, err := json.Marshal(withInput)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed2 map[string]interface{}
	if err := json.Unmarshal(data2, &parsed2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	inputMap2, ok := parsed2["input"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected input object, got %T", parsed2["input"])
	}
	if inputMap2["path"] != "hello.txt" {
		t.Fatalf("expected input.path=hello.txt, got %v", inputMap2["path"])
	}
	if inputMap2["mode"] != "full" {
		t.Fatalf("expected input.mode=full, got %v", inputMap2["mode"])
	}
}
