package runner

import (
	"encoding/json"
	"testing"
)

func TestResultMarshalJSON_CostAndCacheStatus(t *testing.T) {
	r := Result{
		Content:     "Hello",
		Cost:        0.00014,
		CacheStatus: "HIT",
		InputTokens: 10,
		OutputTokens: 5,
		Model:       "openrouter/test",
		StopReason:  "stop",
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if cost, ok := parsed["cost"].(float64); !ok || cost != 0.00014 {
		t.Errorf("expected cost=0.00014, got %v", parsed["cost"])
	}
	if cs, ok := parsed["cache_status"].(string); !ok || cs != "HIT" {
		t.Errorf("expected cache_status=HIT, got %v", parsed["cache_status"])
	}
}

func TestResultMarshalJSON_OmitsZeroCostAndEmptyCache(t *testing.T) {
	r := Result{
		Content:     "Hello",
		InputTokens: 10,
		OutputTokens: 5,
		Model:       "openai/gpt-4",
		StopReason:  "stop",
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := parsed["cost"]; ok {
		t.Error("expected cost to be omitted when zero")
	}
	if _, ok := parsed["cache_status"]; ok {
		t.Error("expected cache_status to be omitted when empty")
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
